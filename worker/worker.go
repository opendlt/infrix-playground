// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

// Package worker is the hosted-playground run executor (adoption-09). It runs a
// golden governed flow — deterministically for the anonymous demo, or live
// against Kermit when an operator has enabled it — and turns the result into a
// downloadable portable proof bundle plus a canonical proof receipt.
//
// It invents no proof logic of its own: the anonymous run reuses pkg/demo's
// deterministic golden-escrow path, the live run reuses pkg/demo's Kermit path,
// and verification goes through the same pkg/verifykit the `infrix verify` CLI
// uses. The receipt is built by pkg/proofreceipt. So a playground run produces
// exactly the artifact a CLI user would, with no node trust required to check
// it.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AccumulateNetwork/infrix/pkg/demo"
	"github.com/AccumulateNetwork/infrix/pkg/evidence"
	"github.com/AccumulateNetwork/infrix/pkg/proofreceipt"
)

// proofVerifiedAt returns the UTC timestamp the verification completed. Live
// (L0-confirmed) receipts require a non-empty VerifiedAt to validate; the
// anonymous path records it too for an honest audit trail.
func proofVerifiedAt(_ *demo.FlowResult) string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Mode selects which backend a run uses.
type Mode string

const (
	// ModeAnonymous is the fixture-backed, deterministic, no-wallet,
	// no-funding, no-live-L0 path. It caps at L3 and never claims L4.
	ModeAnonymous Mode = "anonymous"
	// ModeKermit is the live Kermit-sandbox path: a real L0 anchor confirmed
	// against the Kermit testnet, reaching L4. Available only when the
	// operator has wired a live anchor provider.
	ModeKermit Mode = "kermit"
)

// FlowGoldenEscrow is the one flow the anonymous playground exposes. The
// allowlist (see api.AbuseGuard) rejects anything else, so the anonymous
// surface can never run arbitrary contracts.
const FlowGoldenEscrow = "golden-escrow"

// StepStatus is the lifecycle of a single progress step.
type StepStatus string

const (
	StepRunning  StepStatus = "running"
	StepComplete StepStatus = "complete"
	StepFailed   StepStatus = "failed"
)

// Step is one progress event a run emits as it executes. The web UI streams
// these so a visitor watches the governed flow advance.
type Step struct {
	Key    string     `json:"key"`
	Label  string     `json:"label"`
	Status StepStatus `json:"status"`
}

// flowSteps is the canonical, ordered narration of the golden-escrow governed
// flow. Each step maps to a real component of the produced evidence bundle, so
// the narration is the proof's own structure — not decoration.
var flowSteps = []Step{
	{Key: "intent", Label: "Buyer submits escrow intent"},
	{Key: "plan", Label: "Infrix compiles a governed plan"},
	{Key: "policy", Label: "Policy authorizes a regulated release"},
	{Key: "approval", Label: "Operator approval bound to the plan hash"},
	{Key: "credential", Label: "Delivery credential verified"},
	{Key: "outcome", Label: "Funds released to the seller"},
	{Key: "anchor", Label: "Evidence anchored"},
	{Key: "export", Label: "Portable proof exported"},
	{Key: "verify", Label: "Verified independently (no node trust)"},
}

// FlowSteps returns the ordered step narration (exposed for tests + the UI's
// initial skeleton).
func FlowSteps() []Step { return append([]Step(nil), flowSteps...) }

// RunResult is the complete output of a playground run: the canonical receipt,
// the downloadable portable bundle bytes, and the parsed package (for the
// browser replay + client-side verification).
type RunResult struct {
	Mode         Mode
	NetworkLabel string
	ProofLabel   string
	Receipt      *proofreceipt.Receipt
	Package      *evidence.PortableEvidencePackage
	BundleJSON   []byte
}

// Runner executes playground runs. LiveAnchor is nil unless the operator has
// enabled Kermit mode; when nil, ModeKermit runs fail closed with a clear
// message (the spec's "Kermit disabled fallback").
type Runner struct {
	// LiveAnchor supplies the live-L0 wiring for ModeKermit. nil disables
	// Kermit mode.
	LiveAnchor *demo.LiveAnchor
}

// KermitEnabled reports whether live Kermit runs are available on this
// instance.
func (r *Runner) KermitEnabled() bool { return r != nil && r.LiveAnchor != nil }

// Run executes a governed flow in the requested mode, emitting progress steps
// through emit (which may be nil). It returns the run result or an error; on
// error the run is reported failed and no receipt is produced.
func (r *Runner) Run(ctx context.Context, mode Mode, emit func(Step)) (*RunResult, error) {
	send := func(s Step) {
		if emit != nil {
			emit(s)
		}
	}

	var (
		fr  *demo.FlowResult
		err error
	)
	switch mode {
	case ModeAnonymous:
		fr, err = demo.RunLocal()
	case ModeKermit:
		if !r.KermitEnabled() {
			return nil, fmt.Errorf("Kermit Sandbox mode is disabled on this instance — use Anonymous Demo mode, which runs the same governed flow deterministically without a wallet or funding")
		}
		fr, err = demo.RunKermit(ctx, r.LiveAnchor)
	default:
		return nil, fmt.Errorf("unknown run mode %q", mode)
	}
	if err != nil {
		send(Step{Key: "run", Label: "Run failed", Status: StepFailed})
		return nil, err
	}

	// Narrate the produced proof's structure as completed steps.
	for _, s := range flowSteps {
		s.Status = StepComplete
		send(s)
	}

	bundleJSON, err := json.MarshalIndent(fr.Package, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("playground: marshal portable bundle: %w", err)
	}

	verifyCmd := "infrix verify <bundle>.infrix.json"
	if mode == ModeKermit {
		verifyCmd = "infrix verify <bundle>.infrix.json --l0 " + fr.NetworkLabel
	}
	receipt := proofreceipt.FromVerifyReport(fr.Report.VerifyReport, proofreceipt.VerifyConvertOptions{
		SubjectType: proofreceipt.SubjectEvidence,
		IntentID:    fr.Report.IntentID,
		PlanID:      fr.Report.PlanID,
		OutcomeID:   fr.Report.OutcomeID,
		EvidenceID:  fr.Report.BundleID,
		AnchorTx:    fr.Package.AnchorTxHash,
		Verifier:    "infrix verify",
		Command:     verifyCmd,
		Network:     fr.NetworkLabel,
		VerifiedAt:  proofVerifiedAt(fr),
	})

	return &RunResult{
		Mode:         mode,
		NetworkLabel: fr.NetworkLabel,
		ProofLabel:   fr.Report.ProofLabel,
		Receipt:      receipt,
		Package:      fr.Package,
		BundleJSON:   bundleJSON,
	}, nil
}
