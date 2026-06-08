// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import "testing"

// TestRunStateMachine pins the forward-only transition rules: created → running
// → complete|failed, and nothing else.
func TestRunStateMachine(t *testing.T) {
	legal := []struct{ from, to RunState }{
		{StateCreated, StateRunning},
		{StateRunning, StateComplete},
		{StateRunning, StateFailed},
	}
	for _, c := range legal {
		if !c.from.CanTransition(c.to) {
			t.Errorf("%s → %s should be legal", c.from, c.to)
		}
	}

	illegal := []struct{ from, to RunState }{
		{StateCreated, StateComplete},  // must run first
		{StateCreated, StateFailed},    // must run first
		{StateRunning, StateCreated},   // no going back
		{StateComplete, StateRunning},  // terminal
		{StateComplete, StateFailed},   // terminal
		{StateFailed, StateRunning},    // terminal
		{StateComplete, StateComplete}, // terminal
	}
	for _, c := range illegal {
		if c.from.CanTransition(c.to) {
			t.Errorf("%s → %s should be illegal", c.from, c.to)
		}
	}

	if !StateComplete.Terminal() || !StateFailed.Terminal() {
		t.Error("complete and failed must be terminal")
	}
	if StateRunning.Terminal() || StateCreated.Terminal() {
		t.Error("created and running must not be terminal")
	}
}
