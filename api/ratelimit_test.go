// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"testing"
	"time"
)

// TestRateLimiterBurstThenReject pins the limiter contract: a client gets up to
// Burst requests immediately, then is rejected until tokens refill.
func TestRateLimiterBurstThenReject(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	rl := NewRateLimiter(RateLimitConfig{Burst: 3, PerMinute: 60}) // 1 token/sec
	rl.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if d := rl.Allow("client-a"); !d.Allowed {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
	d := rl.Allow("client-a")
	if d.Allowed {
		t.Fatal("4th request should be rejected (burst exhausted)")
	}
	if d.RetryAfter <= 0 {
		t.Error("rejected request should report a positive RetryAfter")
	}

	// A different client has its own bucket.
	if !rl.Allow("client-b").Allowed {
		t.Error("a separate client must not be limited by client-a's usage")
	}

	// After enough time passes, tokens refill.
	now = now.Add(2 * time.Second)
	if !rl.Allow("client-a").Allowed {
		t.Error("client-a should be allowed again after refill")
	}
}

// TestRateLimiterDisabled ensures a non-positive burst disables limiting.
func TestRateLimiterDisabled(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{Burst: 0})
	for i := 0; i < 100; i++ {
		if !rl.Allow("x").Allowed {
			t.Fatal("disabled limiter must always allow")
		}
	}
}

// TestRateLimiterPrunesIdleBuckets confirms idle buckets are eventually pruned
// so the map cannot grow without bound.
func TestRateLimiterPrunesIdleBuckets(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	rl := NewRateLimiter(RateLimitConfig{Burst: 2, PerMinute: 60})
	rl.now = func() time.Time { return now }

	rl.Allow("ephemeral")
	if len(rl.buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(rl.buckets))
	}
	now = now.Add(11 * time.Minute) // beyond ttl
	rl.Allow("fresh")
	if _, ok := rl.buckets["ephemeral"]; ok {
		t.Error("idle bucket should have been pruned")
	}
}
