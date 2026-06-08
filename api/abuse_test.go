// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import "testing"

// TestAbuseGuardFlowAllowlist is the security check that the anonymous surface
// can only run allowlisted flows — never an arbitrary contract or command.
func TestAbuseGuardFlowAllowlist(t *testing.T) {
	g := NewAbuseGuard()

	if !g.AllowedFlow("golden-escrow") {
		t.Error("golden-escrow must be allowed")
	}
	for _, bad := range []string{"", "arbitrary-contract", "rm -rf /", "../../etc/passwd", "exec:/bin/sh"} {
		if g.AllowedFlow(bad) {
			t.Errorf("flow %q must NOT be allowed", bad)
		}
		if err := g.CheckFlow(bad); err == nil {
			t.Errorf("CheckFlow(%q) must error", bad)
		}
	}
}

// TestLooksLikePath is the security check that nothing path-like reaches a
// loader: the playground accepts proofs by value only.
func TestLooksLikePath(t *testing.T) {
	paths := []string{
		"/etc/passwd",
		"..\\..\\secret",
		"C:\\Windows\\System32",
		"./relative",
		"a/b/c",
		"../escape",
	}
	for _, p := range paths {
		if !LooksLikePath(p) {
			t.Errorf("%q should be detected as a path", p)
		}
	}
	notPaths := []string{"golden-escrow", "anonymous", "a-bundle-id", ""}
	for _, p := range notPaths {
		if LooksLikePath(p) {
			t.Errorf("%q should NOT be detected as a path", p)
		}
	}
}

func TestAbuseGuardUploadBound(t *testing.T) {
	g := NewAbuseGuard()
	if g.MaxUploadBytes() <= 0 {
		t.Error("upload bound must be positive")
	}
	if g.MaxUploadBytes() > 16<<20 {
		t.Error("upload bound is implausibly large")
	}
}
