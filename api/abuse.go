// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"fmt"
	"strings"

	"github.com/opendlt/infrix-playground/worker"
)

// AbuseGuard enforces the hosted-playground's safety envelope (adoption-09):
//   - a FIXED allowlist of flows — the anonymous surface can never run an
//     arbitrary contract or command;
//   - bounded upload sizes for browser-supplied proofs;
//   - rejection of any input that looks like a filesystem path, so a request
//     can never reference the host's disk.
//
// The guard is pure policy: it makes no network or disk calls itself.
type AbuseGuard struct {
	allowedFlows  map[string]bool
	maxUploadByte int64
}

// DefaultMaxUploadBytes bounds a browser-uploaded proof bundle. A real portable
// evidence package is tens of KB; 2 MiB is a generous ceiling that still stops
// a memory-exhaustion upload.
const DefaultMaxUploadBytes = 2 << 20

// NewAbuseGuard builds the guard with the fixed allowlist of governed flows the
// anonymous playground exposes (DX P3-3): golden-escrow, create-did, and
// issue-credential. Each is a parameter-free flow the node runs to completion;
// anything else is rejected.
func NewAbuseGuard() *AbuseGuard {
	allowed := map[string]bool{}
	for _, f := range worker.PlaygroundFlows() {
		allowed[f] = true
	}
	return &AbuseGuard{
		allowedFlows:  allowed,
		maxUploadByte: DefaultMaxUploadBytes,
	}
}

// AllowedFlow reports whether flow is on the fixed allowlist.
func (g *AbuseGuard) AllowedFlow(flow string) bool {
	return g != nil && g.allowedFlows[flow]
}

// AllowedFlows returns the allowlist (for the UI + tests).
func (g *AbuseGuard) AllowedFlows() []string {
	out := make([]string, 0, len(g.allowedFlows))
	for f := range g.allowedFlows {
		out = append(out, f)
	}
	return out
}

// MaxUploadBytes is the upper bound on a browser-supplied proof body.
func (g *AbuseGuard) MaxUploadBytes() int64 { return g.maxUploadByte }

// CheckFlow validates a requested flow, returning a clear error when it is not
// allowed. This is the wall that stops arbitrary-contract execution.
func (g *AbuseGuard) CheckFlow(flow string) error {
	if g.AllowedFlow(flow) {
		return nil
	}
	return fmt.Errorf("flow %q is not on the playground allowlist; available flows: %s", flow, strings.Join(worker.PlaygroundFlows(), ", "))
}

// LooksLikePath reports whether s resembles a filesystem path or traversal
// attempt. The playground accepts proof JSON by value only — never a path — so
// any path-like input is rejected before it reaches any loader.
func LooksLikePath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "..") {
		return true
	}
	if strings.ContainsAny(s, "/\\") {
		return true
	}
	// Windows drive prefix (C:...) or an absolute-ish leading colon.
	if len(s) >= 2 && s[1] == ':' {
		return true
	}
	return false
}
