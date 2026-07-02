// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opendlt/infrix-playground/worker"
)

// agentRunFlowResponse is the STABLE, machine-readable shape an agent gets back
// from POST /api/agent/run-flow (DX P4-4): one call runs an allowlisted governed
// flow to completion and returns the portable proof, so an agent never has to
// create → poll → fetch across three endpoints.
//
//	{
//	  "ok": true,
//	  "flow": "golden-escrow",
//	  "mode": "anonymous",
//	  "runId": "...",
//	  "receiptId": "...",
//	  "proof": { ...portable evidence package... },
//	  "verifyHint": "verify `proof` with @infrix/verify, or POST it to /api/verify"
//	}
//
// The abuse guard, mode gating, and rate limiter are the SAME as /api/runs — an
// agent cannot run an arbitrary flow.
type agentRunFlowResponse struct {
	OK         bool            `json:"ok"`
	Flow       string          `json:"flow"`
	Mode       string          `json:"mode"`
	RunID      string          `json:"runId"`
	ReceiptID  string          `json:"receiptId"`
	Proof      json.RawMessage `json:"proof"`
	VerifyHint string          `json:"verifyHint"`
}

// handleAgentRunFlow runs an allowlisted governed flow to completion and returns
// the portable proof in one response. Synchronous by design (agents want a
// single deterministic call); bounded by a timeout.
func (s *Server) handleAgentRunFlow(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Flow == "" {
		req.Flow = worker.FlowGoldenEscrow
	}
	// Abuse guard: only allowlisted flows — identical to /api/runs.
	if err := s.guard.CheckFlow(req.Flow); err != nil {
		s.metrics.Inc(MetricAbuseRejections)
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	mode := worker.Mode(req.Mode)
	if mode == "" {
		mode = worker.ModeAnonymous
	}
	if mode != worker.ModeAnonymous && mode != worker.ModeKermit {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("unknown mode %q", req.Mode))
		return
	}
	if mode == worker.ModeKermit && !s.cfg.KermitEnabled() {
		writeJSONError(w, http.StatusConflict,
			"Kermit Sandbox mode is disabled on this instance. Use Anonymous Demo mode — it runs the same governed flow deterministically.")
		return
	}
	if d := s.limiter.Allow(clientKey(r)); !d.Allowed {
		s.metrics.Inc(MetricRateLimited)
		if d.RetryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(d.RetryAfter.Seconds())))
		}
		writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded — please wait a moment and try again")
		return
	}

	// The run outlives this request; detach it from the request context (which is
	// canceled on return) so the node call is not aborted.
	run := s.runs.Start(context.Background(), mode, req.Flow)

	deadline := time.Now().Add(90 * time.Second)
	for {
		snap := run.snapshot()
		if snap.State.Terminal() {
			if snap.State == StateFailed {
				writeJSONError(w, http.StatusBadGateway, "run failed: "+snap.Error)
				return
			}
			stored, ok := s.store.Get(snap.ReceiptID)
			if !ok {
				writeJSONError(w, http.StatusInternalServerError, "run completed but no proof was stored")
				return
			}
			writeJSON(w, http.StatusOK, agentRunFlowResponse{
				OK:         true,
				Flow:       req.Flow,
				Mode:       string(mode),
				RunID:      run.ID,
				ReceiptID:  stored.ID,
				Proof:      json.RawMessage(stored.BundleJSON),
				VerifyHint: "verify `proof` offline with @infrix/verify, or POST it to /api/verify",
			})
			return
		}
		if time.Now().After(deadline) {
			writeJSONError(w, http.StatusGatewayTimeout, "run did not complete within the timeout")
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
}
