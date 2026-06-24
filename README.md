# Infrix Hosted Playground (adoption-09)

A no-install, browser-first way to feel Infrix:

1. Open the playground.
2. Pick a golden flow.
3. Run it.
4. Watch it in Nexus/Cinema.
5. Export the proof.
6. Verify it **in your browser** — without trusting the playground node.
7. Share a proof receipt.

This is the easiest way to understand Infrix's value before installing anything.

## Architecture — a thin client

The playground is a **thin client**. It does **not** run flows in-process. It
asks an Infrix node to run a golden governed flow over the node's
`POST /v4/playground/runFlow` endpoint, receives the portable evidence package,
and then **re-verifies that package OFFLINE** with the published verifier
(`github.com/opendlt/infrix-verify/verifykit`) — the same verifier the
`infrix verify` CLI uses. The node is a flow **runner**, never a trusted
verifier: the playground's verdict comes from its own re-verification, so the
no-node-trust promise holds even for its own demo.

It depends only on the published Infrix modules — `infrix-schema`,
`infrix-verify`, and `infrix-nexus-web` (the shared SPA modules) — plus the Go
standard library. No monorepo `pkg/*`, no live-node compile-time dependency.

## Run it

The playground needs an Infrix node serving `/v4/playground/runFlow`:

```sh
go run ./cmd/infrix-playground --addr 127.0.0.1:8086 --node-endpoint http://127.0.0.1:8080
# then open http://127.0.0.1:8086/
```

Advertise the live Kermit sandbox (the configured node must actually offer it;
testnet only, mainnet is refused):

```sh
go run ./cmd/infrix-playground --kermit --kermit-l0 kermit
```

## Modes

| Mode | Backing | Wallet / funding | Assurance | Notes |
|------|---------|------------------|-----------|-------|
| **Anonymous Demo** | deterministic flow on the node | none | caps at **L3** (no live-L0 claim) | default; shareable receipt |
| **Kermit Sandbox** | live Kermit testnet (node-side) | short-lived test identity, faucet-backed | can reach **L4** | opt-in (`--kermit`); rate-limited |
| **Bring Your Own Proof** | your uploaded bundle | none, no login | offline cryptographic verdict | verification runs entirely in the browser |

## Layout

```
cmd/infrix-playground/   the server binary
web/                     the browser SPA (embedded; reuses the shared Nexus modules)
api/                     the HTTP server, run state machine, receipt store, rate limiter, abuse guard, metrics
worker/                  the run client (calls /v4/playground/runFlow, re-verifies offline)
fixtures/                a known-good sample proof for "Bring Your Own Proof"
receipts/                runtime share-linked receipts (gitignored)
```

The playground invents no proof logic: the node runs the deterministic golden
flow, the playground re-verifies with the published `verifykit`, and builds
receipts with the published `infrix-verify/proofreceipt` converter. The browser
verifier and the Cinema replay are the canonical Nexus modules, served from the
embedded `infrix-nexus-web` asset tree.

## Security & abuse controls

- Rate limit per client (token bucket).
- Fixed flow allowlist — the anonymous surface cannot run an arbitrary contract.
- Bounded upload size for browser-supplied proofs; proofs accepted **by value only** (never a path).
- No mainnet writes. No private-key storage. Short-lived Kermit test identities (node-side).
- Daily cleanup job prunes old share-linked runs.
- Health (`/healthz`), readiness (`/readyz`), and metrics (`/metrics`) endpoints.

## Honest claims

The hosted playground **demonstrates Infrix flows with fixture-backed and
Kermit-backed modes.** It is **not production infrastructure** and **does not
prove mainnet readiness** — the UI says so in its footer, and the anonymous mode
never claims L4.

## Tests

```sh
go test ./... -count=1          # Go: api + worker (thin client, offline verify)
node --test web/test/*.mjs      # web: bundle-scene smokes
```
