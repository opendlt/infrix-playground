// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

// Package worker is the hosted-playground run executor (adoption-09). It is a
// THIN CLIENT: it asks an Infrix node to run a golden governed flow over the
// /v4/playground/runFlow endpoint, then — critically — re-verifies the returned
// portable evidence package OFFLINE with the published verifier (the SAME
// verifykit the `infrix verify` CLI uses). The receipt is built from the
// client's OWN verdict, so a playground run produces exactly the artifact a CLI
// user would, with no trust in the node required to check it.
//
// It imports no monorepo pkg/*: the flow runs server-side (the node owns the
// 50-package golden-escrow stack), and everything the client needs — the
// portable-package type, the verifier, and the receipt converter — ships from
// the published infrix-schema / infrix-verify modules.
package worker

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	schemaev "github.com/opendlt/infrix-schema/evidence"
	schemapr "github.com/opendlt/infrix-schema/proofreceipt"
	verifypr "github.com/opendlt/infrix-verify/proofreceipt"
	"github.com/opendlt/infrix-verify/verifykit"
)

// Mode selects which backend a run uses.
type Mode string

const (
	// ModeAnonymous is the fixture-backed, deterministic, no-wallet,
	// no-funding, no-live-L0 path. It caps at L3 and never claims L4.
	ModeAnonymous Mode = "anonymous"
	// ModeKermit is the live Kermit-sandbox path: a real L0 anchor confirmed
	// against the Kermit testnet, reaching L4. Available only when the
	// operator has wired a live anchor provider on the node AND supplied this
	// client an L0 confirmer so it can confirm the anchor independently.
	ModeKermit Mode = "kermit"
)

// The allowlisted playground flows (DX P3-3). Each is a FIXED, parameter-free
// governed flow the node runs to completion, returning a portable proof the
// client re-verifies offline. The allowlist (see api.AbuseGuard) rejects
// anything else, so the anonymous surface can never run an arbitrary contract.
const (
	FlowGoldenEscrow    = "golden-escrow"    // escrow release with regulated approval + delivery credential
	FlowCreateDID       = "create-did"       // create a did:infrix
	FlowIssueCredential = "issue-credential" // issue a verifiable credential
)

// PlaygroundFlows is the canonical flow allowlist, in display order.
func PlaygroundFlows() []string {
	return []string{FlowGoldenEscrow, FlowCreateDID, FlowIssueCredential}
}

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
	// Hash is a short (8-byte) hex of the REAL artifact this stage corresponds
	// to in the produced evidence package — the chain link's content hash, the
	// anchored chain digest, or the export seal. It lets the UI's spine show the
	// proof's own cryptographic structure rather than decoration. Empty for
	// stages with no stored artifact (e.g. the final independent verify), or when
	// the package cannot be parsed. omitempty so failure/skeleton steps stay lean.
	Hash string `json:"hash,omitempty"`
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
	Receipt      *schemapr.Receipt
	Package      *schemaev.PortableEvidencePackage
	BundleJSON   []byte
}

// Runner executes playground runs against a node's /v4/playground/runFlow
// endpoint and verifies the result offline. It holds NO live-L0 wiring of its
// own (that lives server-side); KermitAvailable advertises whether the node
// offers live Kermit runs, and L0Confirmer — when supplied — lets this client
// confirm a Kermit anchor against L0 INDEPENDENTLY (trusting the ledger, not the
// Infrix node).
type Runner struct {
	// Endpoint is the base URL of the Infrix node serving
	// /v4/playground/runFlow (e.g. "http://127.0.0.1:8080").
	Endpoint string
	// HTTPClient overrides the default client (tests inject httptest). nil
	// uses http.DefaultClient.
	HTTPClient *http.Client
	// KermitAvailable advertises that ModeKermit runs are offered. When false,
	// ModeKermit fails closed with a clear message.
	KermitAvailable bool
	// L0Confirmer, when non-nil, confirms a Kermit run's L0 anchor
	// independently so the client's offline verdict can reach L4. nil keeps
	// every verdict capped at L3 (no node trust, no L0 reach).
	L0Confirmer verifykit.L0AnchorConfirmer
}

// New builds a Runner pointed at a node endpoint. kermitAvailable advertises
// whether the node offers live Kermit runs.
func New(endpoint string, kermitAvailable bool) *Runner {
	return &Runner{Endpoint: endpoint, KermitAvailable: kermitAvailable}
}

// KermitEnabled reports whether live Kermit runs are available on this instance.
func (r *Runner) KermitEnabled() bool { return r != nil && r.KermitAvailable }

// runFlowRequest is the /v4/playground/runFlow request body. It mirrors
// playgroundrpc.RunFlowParams (kept in lockstep by the parity fence in
// pkg/playgroundrpc).
type runFlowRequest struct {
	Mode string `json:"mode"`
	Flow string `json:"flow"`
}

// runFlowResponse is the /v4/playground/runFlow response body. It mirrors
// playgroundrpc.RunFlowResult. The labels are informational; the client trusts
// only its OWN re-verification of Package for the verdict.
type runFlowResponse struct {
	NetworkLabel string                            `json:"networkLabel"`
	ProofLabel   string                            `json:"proofLabel"`
	L0Verified   bool                              `json:"l0Verified"`
	IntentID     string                            `json:"intentId"`
	PlanID       string                            `json:"planId"`
	OutcomeID    string                            `json:"outcomeId"`
	BundleID     string                            `json:"bundleId"`
	AnchorTx     string                            `json:"anchorTx"`
	Package      *schemaev.PortableEvidencePackage `json:"package"`
}

// Run executes a governed flow in the requested mode by calling the node's
// run-flow endpoint, then re-verifying the returned package offline. Progress
// steps are emitted through emit (which may be nil). On error the run is
// reported failed and no receipt is produced.
func (r *Runner) Run(ctx context.Context, mode Mode, flow string, emit func(Step)) (*RunResult, error) {
	send := func(s Step) {
		if emit != nil {
			emit(s)
		}
	}
	if strings.TrimSpace(flow) == "" {
		flow = FlowGoldenEscrow
	}

	switch mode {
	case ModeAnonymous:
		// always available
	case ModeKermit:
		if !r.KermitEnabled() {
			return nil, fmt.Errorf("Kermit Sandbox mode is disabled on this instance — use Anonymous Demo mode, which runs the same governed flow deterministically without a wallet or funding")
		}
	default:
		return nil, fmt.Errorf("unknown run mode %q", mode)
	}

	resp, err := r.callRunFlow(ctx, mode, flow)
	if err != nil {
		send(Step{Key: "run", Label: "Run failed", Status: StepFailed})
		return nil, err
	}
	if resp.Package == nil {
		send(Step{Key: "run", Label: "Run failed", Status: StepFailed})
		return nil, fmt.Errorf("playground: node returned no portable package")
	}

	// Narrate the produced proof's structure as completed steps, each carrying
	// the REAL hash of the artifact it corresponds to in the package — so the
	// UI's spine renders the proof's own structure, not decoration.
	hashes := stepHashes(resp.Package)
	for _, s := range flowSteps {
		s.Status = StepComplete
		s.Hash = hashes[s.Key]
		send(s)
	}

	// The trust-bearing step: re-verify the returned package OFFLINE with the
	// canonical verifier. The node's labels are NEVER trusted for the verdict.
	// Anonymous caps at L3; Kermit reaches L4 only when an L0 confirmer lets
	// this client confirm the anchor against the ledger independently.
	opts := verifykit.Options{}
	if mode == ModeKermit {
		opts.L0Confirmer = r.L0Confirmer
	}
	rep := verifykit.Verify(ctx, resp.Package, opts)

	bundleJSON, err := json.MarshalIndent(resp.Package, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("playground: marshal portable bundle: %w", err)
	}

	verifyCmd := "infrix verify <bundle>.infrix.json"
	if mode == ModeKermit {
		verifyCmd = "infrix verify <bundle>.infrix.json --l0 " + resp.NetworkLabel
	}
	receipt := verifypr.FromVerifyReport(rep, verifypr.VerifyConvertOptions{
		SubjectType: schemapr.SubjectEvidence,
		IntentID:    resp.IntentID,
		PlanID:      resp.PlanID,
		OutcomeID:   resp.OutcomeID,
		EvidenceID:  resp.BundleID,
		AnchorTx:    resp.AnchorTx,
		Verifier:    "infrix verify",
		Command:     verifyCmd,
		Network:     resp.NetworkLabel,
		VerifiedAt:  time.Now().UTC().Format(time.RFC3339),
	})

	return &RunResult{
		Mode:         mode,
		NetworkLabel: resp.NetworkLabel,
		ProofLabel:   resp.ProofLabel,
		Receipt:      receipt,
		Package:      resp.Package,
		BundleJSON:   bundleJSON,
	}, nil
}

// stepHashes maps each flow step key to a short (8-byte) hex of the real
// artifact hash it corresponds to in the produced package, so the UI shows the
// proof's own cryptographic structure. The mapping is honest and explicit:
//
//	intent/plan/policy/approval/credential/outcome → the matching evidence-chain
//	    link's content hash (policy via the "policy_decision" link; credential via
//	    the external-proof / trust-assumption link that records the delivery
//	    condition);
//	anchor → the chain digest that is anchored to L0 (chain.ChainHash);
//	export → the package's own ExportHash seal;
//	verify → no stored artifact (the verdict is computed, not stored) → no hash.
//
// Returns a map that is simply missing keys it cannot resolve (e.g. a step whose
// link is absent, or when BundleData fails to parse) — callers treat a missing
// hash as "no hash line", never a fake one.
func stepHashes(pkg *schemaev.PortableEvidencePackage) map[string]string {
	out := map[string]string{}
	if pkg == nil {
		return out
	}
	short := func(b []byte) string {
		if len(b) == 0 {
			return ""
		}
		return hex.EncodeToString(b[:min(len(b), 8)])
	}
	set := func(key, h string) {
		if h != "" {
			out[key] = h
		}
	}

	// The export seal lives on the package itself.
	eh := pkg.ExportHash
	set("export", short(eh[:]))

	var bundle schemaev.EvidenceBundle
	if err := json.Unmarshal(pkg.BundleData, &bundle); err != nil {
		return out
	}
	if bundle.Chain != nil {
		byType := map[string]string{}
		for i := range bundle.Chain.Links {
			ch := bundle.Chain.Links[i].ContentHash
			byType[bundle.Chain.Links[i].Type] = short(ch[:])
		}
		link := func(types ...string) string {
			for _, t := range types {
				if h, ok := byType[t]; ok {
					return h
				}
			}
			return ""
		}
		set("intent", link("intent"))
		set("plan", link("plan"))
		set("policy", link("policy_decision", "policy"))
		set("approval", link("approval"))
		set("credential", link("external_proof", "credential", "trust_assumption"))
		set("outcome", link("outcome"))
		// The anchor stage shows the chain digest that is anchored to L0.
		cdigest := bundle.Chain.ChainHash
		set("anchor", short(cdigest[:]))
	}
	return out
}

// callRunFlow POSTs the run request to the node's run-flow endpoint and decodes
// the response. The node's error message (if any) is surfaced.
func (r *Runner) callRunFlow(ctx context.Context, mode Mode, flow string) (*runFlowResponse, error) {
	if strings.TrimSpace(r.Endpoint) == "" {
		return nil, fmt.Errorf("playground: no run-flow endpoint configured")
	}
	if strings.TrimSpace(flow) == "" {
		flow = FlowGoldenEscrow
	}
	reqBody, err := json.Marshal(runFlowRequest{Mode: string(mode), Flow: flow})
	if err != nil {
		return nil, fmt.Errorf("playground: encode run-flow request: %w", err)
	}
	url := strings.TrimRight(r.Endpoint, "/") + "/v4/playground/runFlow"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("playground: build run-flow request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("playground: call run-flow endpoint: %w", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 8<<20))
	if httpResp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
			return nil, fmt.Errorf("playground: run-flow failed: %s", e.Error.Message)
		}
		return nil, fmt.Errorf("playground: run-flow endpoint returned status %d", httpResp.StatusCode)
	}

	var resp runFlowResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("playground: decode run-flow response: %w", err)
	}
	return &resp, nil
}
