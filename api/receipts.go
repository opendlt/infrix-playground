// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	schemapr "github.com/opendlt/infrix-schema/proofreceipt"
)

// StoredRun is one completed playground run, keyed by a content-derived ID and
// addressable via a share link (/r/<id>). It holds ONLY public material: the
// proof receipt and the portable bundle. No private keys, no wallet data, no
// session identifiers are ever stored.
type StoredRun struct {
	ID         string            `json:"id"`
	Mode       string            `json:"mode"`
	Flow       string            `json:"flow"`
	Network    string            `json:"network"`
	ProofLabel string            `json:"proofLabel"`
	CreatedAt  time.Time         `json:"createdAt"`
	Receipt    *schemapr.Receipt `json:"receipt"`
	BundleJSON json.RawMessage   `json:"bundle"`
}

// ReceiptStore persists completed runs in memory and, when a directory is
// configured, on disk so share links survive a restart. It is safe for
// concurrent use and prunes runs older than the retention window.
type ReceiptStore struct {
	dir string
	now func() time.Time

	mu   sync.RWMutex
	runs map[string]*StoredRun
}

// NewReceiptStore builds a store. dir == "" keeps runs in memory only; a
// non-empty dir is created if needed and any existing runs are loaded so share
// links persist across restarts.
func NewReceiptStore(dir string) (*ReceiptStore, error) {
	s := &ReceiptStore{dir: dir, now: time.Now, runs: make(map[string]*StoredRun)}
	if dir == "" {
		return s, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("playground: create receipt dir: %w", err)
	}
	if err := s.loadFromDisk(); err != nil {
		return nil, err
	}
	return s, nil
}

// DeriveID returns the stable, content-addressed ID for a run: a short hex
// prefix of the SHA-256 of its receipt. Deterministic and free of secrets.
func DeriveID(receiptJSON []byte) string {
	sum := sha256.Sum256(receiptJSON)
	return hex.EncodeToString(sum[:])[:12]
}

// Put stores a run, deriving its ID from the receipt when ID is empty, and
// returns the ID. When a disk directory is configured the run is also persisted.
func (s *ReceiptStore) Put(run *StoredRun) (string, error) {
	if run == nil || run.Receipt == nil {
		return "", fmt.Errorf("playground: cannot store a nil run/receipt")
	}
	receiptJSON, err := run.Receipt.MarshalJSONIndent()
	if err != nil {
		return "", fmt.Errorf("playground: encode receipt: %w", err)
	}
	if run.ID == "" {
		run.ID = DeriveID(receiptJSON)
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = s.now().UTC()
	}

	s.mu.Lock()
	s.runs[run.ID] = run
	s.mu.Unlock()

	if s.dir != "" {
		if err := s.persist(run); err != nil {
			return run.ID, err
		}
	}
	return run.ID, nil
}

// Get returns a stored run by ID.
func (s *ReceiptStore) Get(id string) (*StoredRun, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	return r, ok
}

// Count returns the number of stored runs.
func (s *ReceiptStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runs)
}

// Cleanup removes runs older than maxAge (both in memory and on disk) and
// returns how many were removed. This is the cleanup-policy primitive the daily
// job calls.
func (s *ReceiptStore) Cleanup(maxAge time.Duration) int {
	cutoff := s.now().Add(-maxAge)
	s.mu.Lock()
	var removed []string
	for id, r := range s.runs {
		if r.CreatedAt.Before(cutoff) {
			delete(s.runs, id)
			removed = append(removed, id)
		}
	}
	s.mu.Unlock()

	if s.dir != "" {
		for _, id := range removed {
			_ = os.Remove(s.diskPath(id))
		}
	}
	return len(removed)
}

func (s *ReceiptStore) diskPath(id string) string {
	return filepath.Join(s.dir, id+".run.json")
}

func (s *ReceiptStore) persist(run *StoredRun) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("playground: encode stored run: %w", err)
	}
	if err := os.WriteFile(s.diskPath(run.ID), data, 0o644); err != nil {
		return fmt.Errorf("playground: persist run: %w", err)
	}
	return nil
}

func (s *ReceiptStore) loadFromDisk() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("playground: read receipt dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".run.json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		data, rerr := os.ReadFile(filepath.Join(s.dir, name))
		if rerr != nil {
			continue
		}
		var run StoredRun
		if json.Unmarshal(data, &run) == nil && run.ID != "" {
			s.runs[run.ID] = &run
		}
	}
	return nil
}
