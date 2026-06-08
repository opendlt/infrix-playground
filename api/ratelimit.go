// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"sync"
	"time"
)

// RateLimiter is a per-client token-bucket admission limiter (adoption-09). It
// bounds how many runs a single IP/session can start, so a hosted instance
// cannot be drained by one visitor. It is intentionally small and dependency-
// free: the playground's needs are coarse (a few runs per minute per client),
// not the IU-metered throughput control pkg/billing provides for the protocol.
type RateLimiter struct {
	capacity   float64       // bucket size (burst)
	refillRate float64       // tokens added per second
	ttl        time.Duration // idle buckets older than this are pruned
	now        func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// RateLimitConfig configures a RateLimiter.
type RateLimitConfig struct {
	// Burst is the maximum number of requests allowed instantaneously.
	Burst float64
	// PerMinute is the sustained refill rate (requests per minute).
	PerMinute float64
}

// DefaultRateLimitConfig is the hosted default: a small burst with a steady
// per-minute refill — generous enough for a curious visitor, bounded enough to
// resist abuse.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{Burst: 5, PerMinute: 10}
}

// NewRateLimiter builds a limiter from cfg. A non-positive Burst disables
// limiting (Allow always returns true) — used by tests that exercise other
// paths.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		capacity:   cfg.Burst,
		refillRate: cfg.PerMinute / 60.0,
		ttl:        10 * time.Minute,
		now:        time.Now,
		buckets:    make(map[string]*bucket),
	}
	return rl
}

// Decision is the outcome of an admission check.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Allow consumes one token for key and reports whether the request is admitted.
// When rejected, RetryAfter says how long until a token is available.
func (rl *RateLimiter) Allow(key string) Decision {
	if rl == nil || rl.capacity <= 0 {
		return Decision{Allowed: true}
	}
	now := rl.now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.pruneLocked(now)

	b := rl.buckets[key]
	if b == nil {
		b = &bucket{tokens: rl.capacity, last: now}
		rl.buckets[key] = b
	}
	// Refill since last seen.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = minF(rl.capacity, b.tokens+elapsed*rl.refillRate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens -= 1
		return Decision{Allowed: true}
	}
	// Time until one token accrues.
	deficit := 1 - b.tokens
	var retry time.Duration
	if rl.refillRate > 0 {
		retry = time.Duration(deficit/rl.refillRate*float64(time.Second)) + time.Second
	}
	return Decision{Allowed: false, RetryAfter: retry}
}

// pruneLocked drops idle buckets so the map cannot grow unbounded. Caller holds
// the lock.
func (rl *RateLimiter) pruneLocked(now time.Time) {
	for k, b := range rl.buckets {
		if now.Sub(b.last) > rl.ttl {
			delete(rl.buckets, k)
		}
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
