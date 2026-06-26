// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opendlt/infrix-playground/fixtures"
)

// runFlowNode is a stand-in for the node's /v4/playground/runFlow endpoint: it
// returns the known-good sample portable package so the thin client can be
// exercised end-to-end (HTTP → offline verify → receipt) without running the
// real flow stack.
func runFlowNode(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/playground/runFlow" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		// Embed the sample package raw so the client decodes + verifies it.
		body := []byte(`{"networkLabel":"local deterministic demo","proofLabel":"L3","l0Verified":false,"package":` + string(fixtures.SampleProof) + `}`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

// TestThinClientRunProducesVerifiedL3Receipt is the thin-client integration
// test: the anonymous path calls the node, re-verifies the returned package
// OFFLINE, produces a verified receipt, a parseable portable bundle, and —
// critically — NEVER claims L4 (no live L0) and never requires node trust.
func TestThinClientRunProducesVerifiedL3Receipt(t *testing.T) {
	ts := runFlowNode(t)
	defer ts.Close()

	r := New(ts.URL, false) // anonymous only
	r.HTTPClient = ts.Client()
	if r.KermitEnabled() {
		t.Fatal("Kermit must be disabled without availability")
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

// TestRunStepsCarryRealChainHashes is the RB-03 (Run Theater) backend contract:
// each chain-backed step carries the REAL 8-byte hex of its artifact hash so the
// UI's spine shows the proof's own structure, the "verify" meta-step carries no
// hash, and the values match the fixture exactly (no decoration).
func TestRunStepsCarryRealChainHashes(t *testing.T) {
	ts := runFlowNode(t)
	defer ts.Close()
	r := New(ts.URL, false)
	r.HTTPClient = ts.Client()

	var steps []Step
	if _, err := r.Run(context.Background(), ModeAnonymous, func(s Step) { steps = append(steps, s) }); err != nil {
		t.Fatalf("run: %v", err)
	}

	hashOf := map[string]string{}
	for _, s := range steps {
		if s.Status == StepComplete {
			hashOf[s.Key] = s.Hash
		}
	}

	// Every chain-backed stage (plus the anchor digest and the export seal) must
	// carry a non-empty 16-hex-char (8-byte) hash.
	for _, key := range []string{"intent", "plan", "policy", "approval", "credential", "outcome", "anchor", "export"} {
		h := hashOf[key]
		if len(h) != 16 {
			t.Errorf("step %q hash = %q, want 16 hex chars (8 bytes)", key, h)
		}
	}
	// The "verify" meta-step records no stored artifact.
	if hashOf["verify"] != "" {
		t.Errorf("verify step must carry no hash, got %q", hashOf["verify"])
	}
	// The intent hash must match the fixture's first chain link content hash
	// (first 8 bytes), proving the value is real and not decoration.
	const wantIntent = "f660e00519777cde"
	if hashOf["intent"] != wantIntent {
		t.Errorf("intent hash = %q, want %q (fixture chain link[0])", hashOf["intent"], wantIntent)
	}
}

// TestKermitDisabledFailsClosed verifies the spec's "Kermit disabled fallback":
// requesting Kermit mode without availability fails with a clear message, before
// any node call — not a crash or a fake result.
func TestKermitDisabledFailsClosed(t *testing.T) {
	r := New("http://127.0.0.1:0", false)
	_, err := r.Run(context.Background(), ModeKermit, nil)
	if err == nil {
		t.Fatal("Kermit run with no availability must fail")
	}
	if got := err.Error(); !contains(got, "disabled") {
		t.Errorf("error should explain Kermit is disabled, got: %q", got)
	}
}

func TestUnknownModeRejected(t *testing.T) {
	r := New("http://127.0.0.1:0", false)
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
