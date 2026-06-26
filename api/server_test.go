// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opendlt/infrix-playground/fixtures"
)

func newTestServer(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// newRunFlowNode stands in for the Infrix node's /v4/playground/runFlow
// endpoint: it returns the known-good sample portable package so a thin-client
// run completes and the client re-verifies it offline. Returns the base URL.
func newRunFlowNode(t *testing.T) string {
	t.Helper()
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/playground/runFlow" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"networkLabel":"local deterministic demo","proofLabel":"L3","l0Verified":false,"package":` + string(fixtures.SampleProof) + `}`))
	}))
	t.Cleanup(node.Close)
	return node.URL
}

func getJSON(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	var body map[string]any
	data, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(data, &body)
	return res.StatusCode, body
}

func postJSON(t *testing.T, url string, payload any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(payload)
	res, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()
	var body map[string]any
	data, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(data, &body)
	return res.StatusCode, body
}

// waitForReceipt polls the run until it completes and returns the share id.
func waitForReceipt(t *testing.T, base, runID string) string {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		code, snap := getJSON(t, base+"/api/runs/"+runID)
		if code != http.StatusOK {
			t.Fatalf("GET run status = %d", code)
		}
		switch snap["state"] {
		case "complete":
			id, _ := snap["receiptId"].(string)
			if id == "" {
				t.Fatal("completed run has no receiptId")
			}
			return id
		case "failed":
			t.Fatalf("run failed: %v", snap["error"])
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("run did not complete in time")
	return ""
}

// TestAnonymousGoldenFlow is the headline integration test: a browser-only user
// runs the golden flow, gets a receipt, downloads the bundle, and the share
// link resolves — with no wallet, no funding, and no L4 claim.
func TestAnonymousGoldenFlow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RunFlowEndpoint = newRunFlowNode(t)
	cfg.RateLimit = RateLimitConfig{Burst: 50, PerMinute: 600}
	ts := newTestServer(t, cfg)

	code, started := postJSON(t, ts.URL+"/api/runs", map[string]string{"mode": "anonymous", "flow": "golden-escrow"})
	if code != http.StatusAccepted {
		t.Fatalf("create run = %d, body=%v", code, started)
	}
	runID, _ := started["id"].(string)
	if runID == "" {
		t.Fatal("no run id returned")
	}

	receiptID := waitForReceipt(t, ts.URL, runID)

	// Share link resolves to the receipt.
	code, rec := getJSON(t, ts.URL+"/api/receipts/"+receiptID)
	if code != http.StatusOK {
		t.Fatalf("GET receipt = %d", code)
	}
	receipt, _ := rec["receipt"].(map[string]any)
	if receipt["status"] != "verified" {
		t.Errorf("receipt status = %v, want verified", receipt["status"])
	}
	// SECURITY: fixture mode must not claim L4.
	assurance, _ := receipt["assurance"].(map[string]any)
	if l0, _ := assurance["l0Verified"].(bool); l0 {
		t.Error("fixture-mode receipt must not be L0Verified")
	}
	level, _ := assurance["proofLevel"].(string)
	if strings.Contains(strings.ToUpper(level), "L4") {
		t.Errorf("fixture-mode proofLevel must not be L4, got %q", level)
	}

	// Bundle downloads as JSON.
	res, err := http.Get(ts.URL + "/api/receipts/" + receiptID + "/bundle")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("bundle download = %d", res.StatusCode)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, ".infrix.json") {
		t.Errorf("bundle missing attachment disposition: %q", cd)
	}
	var pkg map[string]any
	body, _ := io.ReadAll(res.Body)
	if json.Unmarshal(body, &pkg) != nil || pkg["version"] == nil {
		t.Error("bundle is not a valid portable package JSON")
	}
}

// TestProofUploadVerify is the server-side offline cross-check: a known-good
// bundle uploaded to /api/verify verifies.
func TestProofUploadVerify(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	res, err := http.Post(ts.URL+"/api/verify", "application/json", bytes.NewReader(fixtures.SampleProof))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("verify = %d", res.StatusCode)
	}
	var body map[string]any
	data, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(data, &body)
	if v, _ := body["verified"].(bool); !v {
		t.Errorf("known-good sample should verify, body=%v", body)
	}
}

// TestKermitDisabledFallback: requesting Kermit when disabled returns a clear,
// labeled refusal (409), not a crash or a fake run.
func TestKermitDisabledFallback(t *testing.T) {
	ts := newTestServer(t, DefaultConfig()) // kermit unavailable → disabled
	code, body := postJSON(t, ts.URL+"/api/runs", map[string]string{"mode": "kermit"})
	if code != http.StatusConflict {
		t.Fatalf("kermit-disabled = %d, want 409", code)
	}
	errObj, _ := body["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !strings.Contains(strings.ToLower(msg), "disabled") {
		t.Errorf("message should explain Kermit is disabled: %q", msg)
	}

	// Config and readiness must label Kermit as unavailable.
	_, cfgBody := getJSON(t, ts.URL+"/api/config")
	if ke, _ := cfgBody["kermitEnabled"].(bool); ke {
		t.Error("config must report kermit disabled")
	}
}

// TestRateLimitExceeded: a tiny burst rejects the over-limit request with 429.
func TestRateLimitExceeded(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RunFlowEndpoint = newRunFlowNode(t)
	cfg.RateLimit = RateLimitConfig{Burst: 1, PerMinute: 1}
	ts := newTestServer(t, cfg)

	code1, _ := postJSON(t, ts.URL+"/api/runs", map[string]string{"mode": "anonymous"})
	if code1 != http.StatusAccepted {
		t.Fatalf("first run = %d, want 202", code1)
	}
	code2, body2 := postJSON(t, ts.URL+"/api/runs", map[string]string{"mode": "anonymous"})
	if code2 != http.StatusTooManyRequests {
		t.Fatalf("second run = %d, want 429", code2)
	}
	errObj, _ := body2["error"].(map[string]any)
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "rate limit") {
		t.Errorf("429 should mention rate limit: %q", msg)
	}
}

// TestSecurityArbitraryFlowRejected: the anonymous surface cannot run an
// arbitrary flow/contract — only the allowlist.
func TestSecurityArbitraryFlowRejected(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	for _, bad := range []string{"arbitrary-contract", "exec:/bin/sh", "../../etc/passwd"} {
		code, _ := postJSON(t, ts.URL+"/api/runs", map[string]string{"mode": "anonymous", "flow": bad})
		if code != http.StatusBadRequest {
			t.Errorf("flow %q = %d, want 400", bad, code)
		}
	}
}

// TestSecurityVerifyRejectsNonJSON: the verify endpoint accepts proofs by value
// (JSON) only — never a path or arbitrary bytes.
func TestSecurityVerifyRejectsNonJSON(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	res, err := http.Post(ts.URL+"/api/verify", "application/json", strings.NewReader("/etc/passwd"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("non-JSON verify = %d, want 400", res.StatusCode)
	}
}

// TestOperationalEndpoints: health, ready, metrics respond.
func TestOperationalEndpoints(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		code, _ := getJSON(t, ts.URL+path)
		if code != http.StatusOK {
			t.Errorf("%s = %d, want 200", path, code)
		}
	}
}

// TestSPAandSharedAssets: the SPA shell and the shared Nexus modules the SPA
// imports are served.
func TestSPAandSharedAssets(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())

	// SPA shell.
	res, _ := http.Get(ts.URL + "/")
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(body, []byte("Infrix Playground")) {
		t.Error("index did not render the playground shell")
	}

	// A shared module the SPA imports by absolute path, plus the playground's own
	// embedded modules (checkMatrix.js is served from the root, not /components/,
	// because the /components/ prefix is owned by the shared Nexus handler).
	for _, p := range []string{"/lib/portableVerifier.js", "/lib/canonicalJson.js", "/components/proofReceiptView.js", "/cinema-core/loader.js", "/playground.js", "/checkMatrix.js", "/tamper.js", "/spine.js", "/styles.css"} {
		r, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Errorf("asset %s = %d, want 200", p, r.StatusCode)
		}
	}

	// The verdict-matrix module must be served as JavaScript with real content —
	// the SPA imports it by absolute path, so a 404 or wrong type breaks the page.
	r, err := http.Get(ts.URL + "/checkMatrix.js")
	if err != nil {
		t.Fatal(err)
	}
	cmBody, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("/checkMatrix.js Content-Type = %q, want application/javascript", ct)
	}
	if !bytes.Contains(cmBody, []byte("mountCheckMatrix")) {
		t.Error("/checkMatrix.js did not serve the matrix module body")
	}

	// The Tamper Lab forgery engine is a root-served playground module too.
	tr, err := http.Get(ts.URL + "/tamper.js")
	if err != nil {
		t.Fatal(err)
	}
	tBody, _ := io.ReadAll(tr.Body)
	tr.Body.Close()
	if ct := tr.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("/tamper.js Content-Type = %q, want application/javascript", ct)
	}
	if !bytes.Contains(tBody, []byte("applyForgery")) {
		t.Error("/tamper.js did not serve the forgery-engine module body")
	}

	// The Run Theater spine is a root-served playground module too.
	sr, err := http.Get(ts.URL + "/spine.js")
	if err != nil {
		t.Fatal(err)
	}
	sBody, _ := io.ReadAll(sr.Body)
	sr.Body.Close()
	if ct := sr.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("/spine.js Content-Type = %q, want application/javascript", ct)
	}
	if !bytes.Contains(sBody, []byte("mountSpine")) {
		t.Error("/spine.js did not serve the spine module body")
	}
}
