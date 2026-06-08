// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package worker

import (
	"context"
	"encoding/json"
	"testing"
)

// TestFixtureRunnerProducesVerifiedL3Receipt is the fixture-runner unit test:
// the anonymous path runs deterministically, produces a verified receipt, a
// parseable portable bundle, and — critically — NEVER claims L4 (no live L0).
func TestFixtureRunnerProducesVerifiedL3Receipt(t *testing.T) {
	r := &Runner{} // no LiveAnchor → anonymous only
	if r.KermitEnabled() {
		t.Fatal("Kermit must be disabled without a LiveAnchor")
	}

	var steps []Step
	res, err := r.Run(context.Background(), ModeAnonymous, func(s Step) { steps = append(steps, s) })
	if err != nil {
		t.Fatalf("anonymous run: %v", err)
	}
	if res.Receipt == nil {
		t.Fatal("run produced no receipt")
	}
	if res.Receipt.Status != "verified" {
		t.Errorf("status = %q, want verified", res.Receipt.Status)
	}
	// SECURITY: fixture mode must never claim L4.
	if res.Receipt.Assurance.L0Verified {
		t.Error("anonymous receipt must not be L0Verified")
	}
	if res.Receipt.ClaimsL4() {
		t.Error("anonymous receipt must NOT claim L4")
	}
	if nt := res.Receipt.NodeTrusted(); nt {
		t.Error("receipt must not require node trust")
	}

	// Bundle must be valid JSON the verifier can re-read.
	var anyPkg map[string]any
	if err := json.Unmarshal(res.BundleJSON, &anyPkg); err != nil {
		t.Fatalf("bundle JSON invalid: %v", err)
	}
	if anyPkg["version"] == nil {
		t.Error("bundle JSON missing version")
	}

	// All canonical flow steps must have been emitted as completed.
	completed := map[string]bool{}
	for _, s := range steps {
		if s.Status == StepComplete {
			completed[s.Key] = true
		}
	}
	for _, want := range []string{"intent", "plan", "policy", "approval", "outcome", "verify"} {
		if !completed[want] {
			t.Errorf("step %q was not emitted as complete", want)
		}
	}
}

// TestKermitDisabledFailsClosed verifies the spec's "Kermit disabled fallback":
// requesting Kermit mode without live wiring fails with a clear message, not a
// crash or a fake result.
func TestKermitDisabledFailsClosed(t *testing.T) {
	r := &Runner{}
	_, err := r.Run(context.Background(), ModeKermit, nil)
	if err == nil {
		t.Fatal("Kermit run with no live anchor must fail")
	}
	if got := err.Error(); !contains(got, "disabled") {
		t.Errorf("error should explain Kermit is disabled, got: %q", got)
	}
}

func TestUnknownModeRejected(t *testing.T) {
	r := &Runner{}
	if _, err := r.Run(context.Background(), Mode("bogus"), nil); err == nil {
		t.Fatal("unknown mode must be rejected")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
