// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestEventsAcceptsRedacted: a valid, redacted, browser-sourced event is
// accepted (202).
func TestEventsAcceptsRedacted(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	code, _ := postJSON(t, ts.URL+"/api/events", map[string]any{
		"version":   "1",
		"time":      "2026-06-08T00:00:00Z",
		"source":    "nexus",
		"sessionId": "s_abc123",
		"event":     "demo.completed",
		"mode":      "local",
		"result":    "success",
		"redacted":  true,
	})
	if code != http.StatusAccepted {
		t.Fatalf("valid event = %d, want 202", code)
	}

	// And the funnel summary reflects it.
	scode, body := getJSON(t, ts.URL+"/api/events/summary")
	if scode != http.StatusOK {
		t.Fatalf("summary = %d", scode)
	}
	if fd, _ := body["firstDemoSuccess"].(bool); !fd {
		t.Errorf("summary should reflect the demo completion: %v", body)
	}
}

// TestEventsRejectsSensitiveFields is the privacy fence: an event whose field
// carries an account URL (or key/full hash) is rejected, even if otherwise
// well-formed.
func TestEventsRejectsSensitiveFields(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	code, body := postJSON(t, ts.URL+"/api/events", map[string]any{
		"version":   "1",
		"time":      "2026-06-08T00:00:00Z",
		"source":    "nexus",
		"sessionId": "s_abc",
		"event":     "demo.completed",
		"persona":   "acc://alice.acme/book/1", // sensitive: account URL
		"redacted":  true,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("sensitive event = %d, want 400", code)
	}
	errObj, _ := body["error"].(map[string]any)
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "sensitive") {
		t.Errorf("rejection should mention sensitive data: %q", msg)
	}
}

// TestEventsRejectsUnknownFields: a payload with an extra field (e.g. a proof
// bundle) is rejected by the strict schema — a bundle can never be stored.
func TestEventsRejectsUnknownFields(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	code, _ := postJSON(t, ts.URL+"/api/events", map[string]any{
		"version":   "1",
		"time":      "2026-06-08T00:00:00Z",
		"source":    "nexus",
		"sessionId": "s_abc",
		"event":     "demo.completed",
		"redacted":  true,
		"bundle":    map[string]any{"version": "4", "secret": "stuff"}, // not in schema
	})
	if code != http.StatusBadRequest {
		t.Fatalf("event with unknown field = %d, want 400", code)
	}
}

// TestEventsRejectsBadSource: CLI/SDK sources are not accepted by the hosted
// endpoint (only browser surfaces post here).
func TestEventsRejectsBadSource(t *testing.T) {
	ts := newTestServer(t, DefaultConfig())
	code, _ := postJSON(t, ts.URL+"/api/events", map[string]any{
		"version": "1", "time": "2026-06-08T00:00:00Z", "source": "cli",
		"sessionId": "s_abc", "event": "command.started", "redacted": true,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("cli-source event = %d, want 400", code)
	}
}
