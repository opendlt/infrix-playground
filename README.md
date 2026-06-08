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

## Run it

```sh
go run ./cmd/infrix playground serve --addr 127.0.0.1:8086
# then open http://127.0.0.1:8086/
```

Enable the live Kermit sandbox (testnet only; mainnet is refused):

```sh
go run ./cmd/infrix playground serve --kermit --kermit-l0 kermit
```

## Modes

| Mode | Backing | Wallet / funding | Assurance | Notes |
|------|---------|------------------|-----------|-------|
| **Anonymous Demo** | deterministic fixture | none | caps at **L3** (no live-L0 claim) | default; shareable receipt |
| **Kermit Sandbox** | live Kermit testnet | short-lived test identity, faucet-backed | can reach **L4** | opt-in (`--kermit`); rate-limited |
| **Bring Your Own Proof** | your uploaded bundle | none, no login | offline cryptographic verdict | verification runs entirely in the browser |

## Layout

```
hosted-playground/
  web/        the browser SPA (embedded; reuses the shared Nexus modules)
  api/        the HTTP server, run state machine, receipt store, rate limiter, abuse guard, metrics
  worker/     the run executor (reuses pkg/demo + pkg/verifykit + pkg/proofreceipt)
  fixtures/   a known-good sample proof for "Bring Your Own Proof"
  receipts/   runtime share-linked receipts (gitignored)
```

The playground invents no proof logic: it runs the same deterministic golden
flow `pkg/demo` runs, verifies with the same `pkg/verifykit` the `infrix verify`
CLI uses, and builds receipts with `pkg/proofreceipt`. The browser verifier and
the Cinema replay are the canonical Nexus modules, served from the embedded
Nexus asset tree.

## Security & abuse controls

- Rate limit per client (token bucket).
- Fixed flow allowlist — the anonymous surface cannot run an arbitrary contract.
- Bounded upload size for browser-supplied proofs; proofs accepted **by value only** (never a path).
- No mainnet writes. No private-key storage. Short-lived Kermit test identities.
- Daily cleanup job prunes old share-linked runs.
- Health (`/healthz`), readiness (`/readyz`), and metrics (`/metrics`) endpoints.

## Honest claims

The hosted playground **demonstrates Infrix flows with fixture-backed and
Kermit-backed modes.** It is **not production infrastructure** and **does not
prove mainnet readiness** — the UI says so in its footer, and the anonymous mode
never claims L4.

## Tests

```sh
go test ./hosted-playground/... ./pkg/verifykit ./pkg/proofreceipt
npm test --prefix hosted-playground/web
npm run test:hosted --prefix tools/ux-gate     # Playwright end-to-end gate
go run ./cmd/infrix claims lint
```
