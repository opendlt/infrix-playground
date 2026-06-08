// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/AccumulateNetwork/infrix/pkg/onboardingmetrics"
)

// handlePostEvent ingests one onboarding analytics event from the browser
// (adoption-12). It is strict and privacy-preserving:
//   - unknown fields are rejected (a proof bundle or payload can never sneak in);
//   - the event must validate against the canonical schema;
//   - any field still carrying sensitive material (an account URL, a key, a
//     full hash) is REJECTED — the hosted endpoint accepts redacted events only;
//   - accepted events are redacted again server-side before being stored.
//
// Analytics are opt-in client-side; this endpoint never receives anything the
// browser did not choose to send, and stores nothing sensitive.
func (s *Server) handlePostEvent(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()

	var e onboardingmetrics.Event
	if err := dec.Decode(&e); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid event: "+err.Error())
		return
	}
	// Hosted events must declare their source as the browser surfaces.
	if e.Source != onboardingmetrics.SourceNexus &&
		e.Source != onboardingmetrics.SourceCinema &&
		e.Source != onboardingmetrics.SourceHosted {
		writeJSONError(w, http.StatusBadRequest, "event source must be nexus|cinema|hosted")
		return
	}
	if err := e.Validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "event failed validation: "+err.Error())
		return
	}
	// Reject non-redacted sensitive submissions outright.
	if field, bad := onboardingmetrics.SensitiveField(&e); bad {
		s.metrics.Inc(MetricAbuseRejections)
		writeJSONError(w, http.StatusBadRequest, "event field "+field+" carries sensitive data; send redacted events only")
		return
	}

	stored := onboardingmetrics.Redact(e)
	s.eventsMu.Lock()
	s.events = append(s.events, stored)
	s.eventsMu.Unlock()
	s.metrics.Inc(MetricEventsReceived)

	w.WriteHeader(http.StatusAccepted)
}

// handleEventsSummary returns the privacy-preserving onboarding funnel computed
// from the events received this session.
func (s *Server) handleEventsSummary(w http.ResponseWriter, r *http.Request) {
	s.eventsMu.Lock()
	events := append([]onboardingmetrics.Event(nil), s.events...)
	s.eventsMu.Unlock()
	writeJSON(w, http.StatusOK, onboardingmetrics.Summarize(events))
}
