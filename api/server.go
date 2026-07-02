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
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opendlt/infrix-playground/fixtures"
	"github.com/opendlt/infrix-playground/web"
	"github.com/opendlt/infrix-playground/worker"
	nexusweb "github.com/opendlt/infrix-nexus-web"
	schemaev "github.com/opendlt/infrix-schema/evidence"
	schemaom "github.com/opendlt/infrix-schema/onboardingmetrics"
	verifypr "github.com/opendlt/infrix-verify/proofreceipt"
	"github.com/opendlt/infrix-verify/verifykit"
)

// Server is the hosted-playground HTTP service (adoption-09). It serves the
// playground SPA, the shared Nexus modules the SPA imports, and a small JSON
// API for runs, receipts, sharing, verification, readiness, and metrics. It
// writes nothing to mainnet, stores no private keys, and gates every expensive
// operation behind a rate limiter and an abuse guard.
type Server struct {
	cfg     Config
	mux     *http.ServeMux
	runner  *worker.Runner
	store   *ReceiptStore
	runs    *RunManager
	metrics *Metrics
	guard   *AbuseGuard
	limiter *RateLimiter
	nexusH  http.Handler

	// Onboarding analytics (adoption-12): redacted, schema-validated events the
	// browser posts. In-memory; privacy-preserving (no sensitive fields admitted).
	eventsMu sync.Mutex
	events   []schemaom.Event
}

// NewServer builds a Server from cfg.
func NewServer(cfg Config) (*Server, error) {
	store, err := NewReceiptStore(cfg.ReceiptDir)
	if err != nil {
		return nil, err
	}
	metrics := NewMetrics()
	runner := worker.New(cfg.RunFlowEndpoint, cfg.KermitAvailable)
	s := &Server{
		cfg:     cfg,
		runner:  runner,
		store:   store,
		runs:    NewRunManager(runner, store, metrics),
		metrics: metrics,
		guard:   NewAbuseGuard(),
		limiter: NewRateLimiter(cfg.RateLimit),
		nexusH:  nexusweb.StaticHandler(),
	}
	s.routes()
	return s, nil
}

// Handler exposes the mux (for tests using httptest).
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	mux := http.NewServeMux()

	// Operational surface.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	// Playground API.
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/readiness", s.handleReadiness)
	mux.HandleFunc("GET /api/sample-bundle", s.handleSampleBundle)
	mux.HandleFunc("POST /api/runs", s.handleCreateRun)
	mux.HandleFunc("POST /api/agent/run-flow", s.handleAgentRunFlow)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /api/runs/{id}/events", s.handleRunEvents)
	mux.HandleFunc("GET /api/receipts/{id}", s.handleGetReceipt)
	mux.HandleFunc("GET /api/receipts/{id}/bundle", s.handleGetBundle)
	mux.HandleFunc("POST /api/verify", s.handleVerify)

	// Onboarding analytics (adoption-12).
	mux.HandleFunc("POST /api/events", s.handlePostEvent)
	mux.HandleFunc("GET /api/events/summary", s.handleEventsSummary)

	// Shared Nexus modules the SPA imports by absolute path. Served from the
	// embedded Nexus asset tree so there is ONE canonical implementation.
	mux.HandleFunc("GET /lib/", s.handleShared)
	mux.HandleFunc("GET /components/", s.handleShared)
	mux.HandleFunc("GET /cinema-core/", s.handleShared)
	mux.HandleFunc("GET /styles.css", s.handleShared)

	// Playground's own embedded assets.
	for path, file := range web.Assets() {
		f := file
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", f.ContentType)
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(f.Body)
		})
	}

	// SPA shell for "/" and all client-routed paths (#/r/<id>, #/verify, …).
	mux.HandleFunc("GET /", s.handleIndex)

	s.mux = mux
}

// Run starts the listener and the cleanup job, blocking until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("playground: listen %s: %w", s.cfg.Addr, err)
	}
	srv := &http.Server{Handler: s.mux, ReadHeaderTimeout: 10 * time.Second}

	go s.cleanupLoop(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case e := <-errCh:
		if e == http.ErrServerClosed {
			return nil
		}
		return e
	}
}

// cleanupLoop runs the daily-style cleanup job: it prunes runs older than the
// retention window on a fixed interval (the spec's "daily cleanup job").
func (s *Server) cleanupLoop(ctx context.Context) {
	if s.cfg.CleanupInterval <= 0 {
		return
	}
	t := time.NewTicker(s.cfg.CleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.store.Cleanup(s.cfg.Retention)
		}
	}
}

// --- handlers ---

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(web.Index())
}

// handleShared serves a shared module from the Nexus asset tree. Path traversal
// is rejected; only the SPA's import prefixes reach here.
func (s *Server) handleShared(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" || strings.Contains(p, "..") {
		http.NotFound(w, r)
		return
	}
	data, err := nexusweb.Asset(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentTypeFor(p))
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":         true,
		"kermitEnabled": s.cfg.KermitEnabled(),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.metrics.Snapshot())
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	modes := []string{string(worker.ModeAnonymous)}
	if s.cfg.KermitEnabled() {
		modes = append(modes, string(worker.ModeKermit))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kermitEnabled": s.cfg.KermitEnabled(),
		"allowedFlows":  s.guard.AllowedFlows(),
		"modes":         modes,
	})
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"anonymous": true,
		"kermit":    s.cfg.KermitEnabled(),
		"verifier":  true,
		"fixture":   len(fixtures.SampleProof) > 0,
		"mainnet":   false,
	})
}

func (s *Server) handleSampleBundle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(fixtures.SampleProof)
}

type createRunRequest struct {
	Mode string `json:"mode"`
	Flow string `json:"flow"`
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Flow == "" {
		req.Flow = worker.FlowGoldenEscrow
	}
	// Abuse guard: only allowlisted flows — no arbitrary contract execution.
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
	// Kermit disabled fallback: a clear, labeled refusal — not a crash.
	if mode == worker.ModeKermit && !s.cfg.KermitEnabled() {
		writeJSONError(w, http.StatusConflict,
			"Kermit Sandbox mode is disabled on this instance. Use Anonymous Demo mode — it runs the same governed flow deterministically, with no wallet or funding.")
		return
	}

	// Rate limit the expensive run path per client.
	if d := s.limiter.Allow(clientKey(r)); !d.Allowed {
		s.metrics.Inc(MetricRateLimited)
		if d.RetryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(d.RetryAfter.Seconds())))
		}
		writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded — please wait a moment and try again")
		return
	}

	// The run is a fire-and-forget background job that outlives this request
	// (we return 202 immediately and stream progress over SSE). It must NOT use
	// the request context, which is canceled the moment this handler returns —
	// the run-flow call to the node would be aborted. Detach to a background
	// context.
	run := s.runs.Start(context.Background(), mode)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":    run.ID,
		"mode":  mode,
		"state": StateRunning,
		"steps": worker.FlowSteps(),
	})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run, ok := s.runs.Get(r.PathValue("id"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, run.snapshot())
}

func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	run, ok := s.runs.Get(r.PathValue("id"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, "run not found")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub := run.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				return
			}
			data, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleGetReceipt(w http.ResponseWriter, r *http.Request) {
	run, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, "receipt not found")
		return
	}
	s.metrics.Inc(MetricReceiptViews)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         run.ID,
		"mode":       run.Mode,
		"network":    run.Network,
		"proofLabel": run.ProofLabel,
		"receipt":    run.Receipt,
	})
}

func (s *Server) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	run, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, "bundle not found")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", run.ID+".infrix.json"))
	_, _ = w.Write(run.BundleJSON)
}

// handleVerify is the server-side cross-check: it runs the SAME pkg/verifykit
// the CLI uses, OFFLINE (no L0 confirmer), against a browser-supplied bundle.
// The browser path (client-side portableVerifier) is the no-trust default; this
// endpoint is a convenience that never claims more than the offline verdict.
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if d := s.limiter.Allow(clientKey(r)); !d.Allowed {
		s.metrics.Inc(MetricRateLimited)
		writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded — please wait a moment and try again")
		return
	}
	body := http.MaxBytesReader(w, r.Body, s.guard.MaxUploadBytes())
	var pkg schemaev.PortableEvidencePackage
	if err := json.NewDecoder(body).Decode(&pkg); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid portable evidence package JSON")
		return
	}
	s.metrics.Inc(MetricVerifications)

	rep := verifykit.Verify(r.Context(), &pkg, verifykit.Options{}) // offline — no node trust, caps at L3
	receipt := verifypr.FromVerifyReport(rep, verifypr.VerifyConvertOptions{
		Verifier: "infrix verify (playground, offline)",
		Command:  "infrix verify <bundle>.infrix.json",
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"verified": rep.Verified,
		"receipt":  receipt,
	})
}

// --- small helpers ---

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"message": msg}})
}

// clientKey identifies a caller for rate limiting: the X-Forwarded-For client
// (first hop) when present, else the connection's remote host.
func clientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func contentTypeFor(p string) string {
	switch {
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(p, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(p, ".js"), strings.HasSuffix(p, ".mjs"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".woff2"):
		return "font/woff2"
	}
	return "application/octet-stream"
}
