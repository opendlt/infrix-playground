// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

// Package fixtures holds the hosted-playground's committed, known-good demo
// artifacts (adoption-09). The sample proof is the SAME portable evidence
// package the Nexus browser verifier and the Go verifier both already accept
// (pkg/nexus/web/testdata/portable-fixture.valid.json), so the playground's
// "Bring Your Own Proof" path has a real bundle to verify out of the box — no
// run required, no node trusted.
package fixtures

import _ "embed"

// SampleProof is a known-good portable evidence package (version 4). It
// verifies offline; it is fixture-backed and makes no live-L0 (L4) claim.
//
//go:embed sample-proof.infrix.json
var SampleProof []byte

// SampleProofManifest is the fixture's provenance manifest (sha256, generator,
// verifier results).
//
//go:embed sample-proof.manifest.json
var SampleProofManifest []byte
