// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"net/http"
	"testing"
)

// TestAgentRunFlowReturnsProofInOneCall is the P4-4 guarantee: a single POST to
// /api/agent/run-flow runs an allowlisted governed flow to completion and returns
// the portable proof — an agent hands `proof` straight to @infrix/verify, with no
// create → poll → fetch across three endpoints.
func TestAgentRunFlowReturnsProofInOneCall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RunFlowEndpoint = newRunFlowNode(t)
	cfg.RateLimit = RateLimitConfig{Burst: 50, PerMinute: 600}
	ts := newTestServer(t, cfg)

	code, body := postJSON(t, ts.URL+"/api/agent/run-flow", map[string]string{"mode": "anonymous", "flow": "golden-escrow"})
	if code != http.StatusOK {
		t.Fatalf("agent run-flow = %d, body=%v", code, body)
	}
	if body["ok"] != true {
		t.Errorf("ok != true: %v", body["ok"])
	}
	if id, _ := body["receiptId"].(string); id == "" {
		t.Error("response has no receiptId")
	}
	proof, ok := body["proof"].(map[string]any)
	if !ok || len(proof) == 0 {
		t.Fatalf("response has no portable proof an agent can verify: %v", body["proof"])
	}
}

// TestAgentRunFlowRejectsArbitraryFlow proves the abuse guard applies to the
// agent endpoint too — no arbitrary contract execution.
func TestAgentRunFlowRejectsArbitraryFlow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RunFlowEndpoint = newRunFlowNode(t)
	ts := newTestServer(t, cfg)

	code, _ := postJSON(t, ts.URL+"/api/agent/run-flow", map[string]string{"flow": "arbitrary-contract-exec"})
	if code == http.StatusOK {
		t.Fatal("agent endpoint must reject a non-allowlisted flow (abuse guard)")
	}
}
