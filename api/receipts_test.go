// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"encoding/json"
	"testing"
	"time"

	proofreceipt "github.com/opendlt/infrix-schema/proofreceipt"
)

func sampleReceipt(id string) *proofreceipt.Receipt {
	r := proofreceipt.New()
	r.Status = proofreceipt.StatusVerified
	r.Summary = "Verified without trusting the Infrix node."
	r.Subject = proofreceipt.Subject{Type: proofreceipt.SubjectEvidence, ID: id}
	r.Artifacts = proofreceipt.Artifacts{EvidenceID: id}
	return r
}

func TestReceiptStorePutGet(t *testing.T) {
	s, err := NewReceiptStore("")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Put(&StoredRun{
		Mode:       "anonymous",
		Network:    "local deterministic demo",
		Receipt:    sampleReceipt("ev-1"),
		BundleJSON: json.RawMessage(`{"version":"4"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("Put returned empty id")
	}
	got, ok := s.Get(id)
	if !ok {
		t.Fatal("Get miss after Put")
	}
	if got.Mode != "anonymous" || got.Receipt.Subject.ID != "ev-1" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if s.Count() != 1 {
		t.Errorf("count = %d, want 1", s.Count())
	}
}

// TestDeriveIDDeterministic: the same receipt always yields the same share id.
func TestDeriveIDDeterministic(t *testing.T) {
	r := sampleReceipt("ev-1")
	j, _ := r.MarshalJSONIndent()
	a := DeriveID(j)
	b := DeriveID(j)
	if a != b {
		t.Errorf("DeriveID not deterministic: %s != %s", a, b)
	}
	if len(a) != 12 {
		t.Errorf("id length = %d, want 12", len(a))
	}
	// A different receipt yields a different id.
	if DeriveID(mustJSON(sampleReceipt("ev-2"))) == a {
		t.Error("distinct receipts must produce distinct ids")
	}
}

func mustJSON(r *proofreceipt.Receipt) []byte {
	j, _ := r.MarshalJSONIndent()
	return j
}

func TestReceiptStoreCleanup(t *testing.T) {
	now := time.Unix(3_000_000, 0)
	s, _ := NewReceiptStore("")
	s.now = func() time.Time { return now }

	id, _ := s.Put(&StoredRun{Receipt: sampleReceipt("old"), BundleJSON: json.RawMessage(`{}`)})

	// Nothing older than 1h yet.
	if removed := s.Cleanup(time.Hour); removed != 0 {
		t.Errorf("nothing should be cleaned yet, removed %d", removed)
	}
	// Advance past retention.
	now = now.Add(2 * time.Hour)
	if removed := s.Cleanup(time.Hour); removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if _, ok := s.Get(id); ok {
		t.Error("cleaned run should be gone")
	}
}

// TestReceiptStoreDiskPersistence: share links survive a restart when a dir is
// configured.
func TestReceiptStoreDiskPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, err := NewReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s1.Put(&StoredRun{
		Mode:       "anonymous",
		Receipt:    sampleReceipt("persist"),
		BundleJSON: json.RawMessage(`{"version":"4"}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Re-open the store from the same dir.
	s2, err := NewReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get(id)
	if !ok {
		t.Fatal("run did not persist across restart")
	}
	if got.Receipt.Subject.ID != "persist" {
		t.Errorf("persisted receipt mismatch: %+v", got.Receipt.Subject)
	}
}
