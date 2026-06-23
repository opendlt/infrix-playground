// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"time"
)

// Config configures a hosted-playground Server (adoption-09). The zero value is
// not usable; build from DefaultConfig and override.
type Config struct {
	// Addr is the listen address (e.g. "127.0.0.1:8086").
	Addr string

	// RunFlowEndpoint is the base URL of the Infrix node that runs golden flows
	// (POST /v4/playground/runFlow). The playground is a thin client: it never
	// runs flows in-process. Empty means runs cannot execute.
	RunFlowEndpoint string

	// ReceiptDir, when non-empty, persists share-linked runs to disk so they
	// survive a restart. Empty keeps runs in memory only.
	ReceiptDir string

	// KermitAvailable advertises that the configured node offers live Kermit
	// Sandbox runs. false disables Kermit mode, and run requests for it fail
	// closed with a clear message — the spec's "Kermit disabled fallback".
	KermitAvailable bool

	// RateLimit bounds runs per client.
	RateLimit RateLimitConfig

	// Retention is how long a stored run is kept before the cleanup job
	// removes it.
	Retention time.Duration

	// CleanupInterval is how often the cleanup job runs.
	CleanupInterval time.Duration
}

// DefaultConfig returns the hosted defaults: anonymous-only (Kermit disabled),
// a local node endpoint, in-memory receipts, a small rate limit, and a daily
// cleanup of runs older than 24h.
func DefaultConfig() Config {
	return Config{
		Addr:            "127.0.0.1:8086",
		RunFlowEndpoint: "http://127.0.0.1:8080",
		RateLimit:       DefaultRateLimitConfig(),
		Retention:       24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
}

// KermitEnabled reports whether the instance can run live Kermit flows.
func (c Config) KermitEnabled() bool { return c.KermitAvailable }
