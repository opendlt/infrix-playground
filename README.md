# Infrix Hosted Playground (adoption-09)

> **▶ Live: https://play.infrix.opendlt.org** — run a governed flow and
> re-verify its proof in your browser, no install. The default (anonymous)
> experience is **fixture-backed and caps at L3** (deterministic, no live-L0
> claim); when the **Kermit sandbox** is enabled on the instance, runs go live
> against the testnet and can reach L4. The footer and the Status page report
> which mode this instance is in.

A no-install, browser-first way to feel Infrix:

1. Open the playground.
2. Pick a golden flow.
3. Run it.
4. Watch it in Nexus/Cinema.
5. Export the proof.
6. Verify it **in your browser** — without trusting the playground node.
7. Share a proof receipt.

This is the easiest way to understand Infrix's value before installing anything.

## Why this exists

Every system that produces "verifiable" results faces the same hard question:
*why should I believe the proof?* Most explorers answer "because our node says
so." Infrix's answer is different, and this playground exists to let a skeptic
**feel that difference in 30 seconds, with nothing installed.**

Infrix runs **governed flows** (e.g. an escrow: intent → plan → policy →
approval → credential → outcome → anchor) and emits a **portable evidence
package** — a self-contained cryptographic proof that the flow ran correctly
under its governance rules. The playground's whole job is to prove that package
is trustworthy **without asking you to trust the playground.**

It does that by being a **thin client**: the node *runs* the flow, but your
browser **re-verifies the returned proof locally**, with the same published
verifier the CLI uses. The node is a runner, never a trusted verifier — so even
this demo's own server cannot fake a passing result. The verdict is maths you
re-run yourself.

### The value proposition

> **Run a real governed flow and verify its proof in your own browser — against a
> server you don't have to trust — with no install, no wallet, and no funding.**

### What you can actually do here

- **Watch the proof build itself.** The governed flow assembles stage by stage,
  each step emitting its real hash, hash-linked to the one before it.
- **See the verdict, not a verdict.** Verification runs *in your browser* and
  shows all ten cryptographic checks — each with a plain-language "what this
  proves" — reconstructed locally, no node trusted.
- **Try to break it.** The **Tamper Lab** lets you forge a field and watch the
  maths catch you at the exact broken link. Failing to beat tamper-evidence is
  the fastest way to believe in it.
- **Bring your own proof.** Drop or paste any `*.infrix.json` bundle and verify
  it entirely client-side — no login, nothing uploaded to us.
- **Share the receipt.** Send a passing proof to someone who re-verifies it
  themselves.

### Who it's for, and the job it does

It's for a developer or technical evaluator deciding whether Infrix's
"verifiable governance" claim is real. The job it does is to convert *"sounds
like marketing"* into *"I personally watched it prove itself, and I couldn't
forge it"* — **before** they install anything. It is a **trust-demonstration
tool**, not a flow runner with a UI bolted on.

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

## Deploy

The playground ships as a container (pure-Go, distroless; `web/` and `fixtures/`
are embedded, so the image is just the static binary). CI builds it on every PR
and publishes `ghcr.io/opendlt/infrix-playground` on `main` and `v*` tags.

```sh
docker run --rm -p 8086:8086 \
  -e INFRIX_PLAYGROUND_NODE_ENDPOINT=https://devnet.infrix.opendlt.org \
  ghcr.io/opendlt/infrix-playground:latest
# open http://localhost:8086/
```

Configure via env (CLI flags still override):

| Env var | Default | Purpose |
|---|---|---|
| `INFRIX_PLAYGROUND_ADDR` | `0.0.0.0:8086` (in image) | listen address |
| `INFRIX_PLAYGROUND_NODE_ENDPOINT` | `http://127.0.0.1:8080` | the Infrix node that runs flows |
| `INFRIX_PLAYGROUND_KERMIT` | unset | `1` to advertise live Kermit Sandbox (node must offer it; testnet only) |
| `INFRIX_PLAYGROUND_RECEIPT_DIR` | empty (in-memory) | persist share-linked receipts to disk |

Receipts are in-memory by default, so the container needs no writable volume and
runs read-only/non-root as shipped. "Bring Your Own Proof" verifies offline even
with no node configured; "Anonymous Demo" / "Kermit Sandbox" need a reachable node.

## Modes

| Mode | Backing | Wallet / funding | Assurance | Notes |
|------|---------|------------------|-----------|-------|
| **Anonymous Demo** | deterministic flow on the node | none | caps at **L3** (no live-L0 claim) | default; shareable receipt |
| **Kermit Sandbox** | live Kermit testnet (node-side) | short-lived test identity, faucet-backed | can reach **L4** | opt-in (`--kermit`); rate-limited |
| **Bring Your Own Proof** | your uploaded bundle | none, no login | offline cryptographic verdict | verification runs entirely in the browser |

## Agent API — one-call run → prove (DX P4-4)

Agents (not just browsers) get a **single, synchronous** endpoint that runs an
allowlisted governed flow to completion and returns the portable proof — no
create → poll → fetch dance:

```
POST /api/agent/run-flow
  { "flow": "golden-escrow", "mode": "anonymous" }
→ 200 { "ok": true, "flow": "...", "mode": "...", "runId": "...",
        "receiptId": "...", "proof": { ...portable evidence package... },
        "verifyHint": "verify `proof` offline with @infrix/verify" }
```

The same abuse guard, mode gating, and rate limiter as `/api/runs` apply — an
agent cannot run an arbitrary flow. Pass the returned `proof` straight to
`@infrix/verify` (or `POST /api/verify`) for an independent verdict.

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
