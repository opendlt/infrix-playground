// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"sort"
	"sync"
	"time"
)

// Metrics is a small, dependency-free counter set for the hosted-playground's
// operational surface (adoption-09): runs started/completed/failed, rate-limit
// rejections, verifications, and share-link views. Exposed at /metrics.
type Metrics struct {
	startedAt time.Time
	now       func() time.Time

	mu       sync.Mutex
	counters map[string]int64
}

// Metric names (stable; the /metrics surface and tests key on them).
const (
	MetricRunsStarted     = "runs_started"
	MetricRunsCompleted   = "runs_completed"
	MetricRunsFailed      = "runs_failed"
	MetricRateLimited     = "rate_limited"
	MetricVerifications   = "verifications"
	MetricReceiptViews    = "receipt_views"
	MetricAbuseRejections = "abuse_rejections"
	MetricEventsReceived  = "events_received"
)

// NewMetrics returns an initialized metric set.
func NewMetrics() *Metrics {
	m := &Metrics{now: time.Now, counters: make(map[string]int64)}
	m.startedAt = m.now()
	return m
}

// Inc increments a counter by one.
func (m *Metrics) Inc(name string) { m.Add(name, 1) }

// Add increments a counter by n.
func (m *Metrics) Add(name string, n int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.counters[name] += n
	m.mu.Unlock()
}

// Snapshot returns a copy of all counters plus uptime_seconds.
func (m *Metrics) Snapshot() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int64, len(m.counters)+1)
	for k, v := range m.counters {
		out[k] = v
	}
	out["uptime_seconds"] = int64(m.now().Sub(m.startedAt).Seconds())
	return out
}

// SortedKeys returns the metric names in a stable order (for rendering).
func (m *Metrics) SortedKeys(snap map[string]int64) []string {
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
