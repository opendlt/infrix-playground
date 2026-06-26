# Infrix Playground — "Trust Microscope" Redesign

This folder is the complete plan to turn the Infrix Playground from a competent
technical demo into a "WoW" product with a defensible super-insight no other
blockchain project offers.

## The thesis

> **A trust microscope: run a governed flow, then watch ten cryptographic checks
> verify it in *your* browser — against a server you don't trust — each link of the
> chain laid bare and explained in plain language.**

Every blockchain explorer shows you *what happened*. Infrix Playground should show
you *why you can believe it happened* — computed locally, with every cryptographic
link revealed and explained. The product is a **trust microscope**, not a flow runner.

The crucial discovery: the WoW is **already 90% built**. The client-side verifier
(`portableVerifier.js`) runs a 10-check cryptographic matrix in the browser and
returns a richly-detailed result — and the current UI throws all of it away and
prints one word. We are not building new cryptography. We are *revealing the
cryptography that already runs.*

## Documents in this folder

| File | What it is |
|---|---|
| `README.md` | This file — master plan, effort map, sequencing, status tracker. |
| `00-FINDINGS.md` | The brutally honest teardown: what's wrong, with file/line evidence. |
| `SPEC.md` | The full design spec (IA, tokens, copy, all surfaces, acceptance criteria). |
| `01-runbook-check-matrix.md` | RB-01 — render the 10-check verifier matrix. **Front-end only.** |
| `02-runbook-tamper-lab.md` | RB-02 — the interactive falsification game. **Front-end only.** |
| `03-runbook-run-theater.md` | RB-03 — the self-assembling Spine run animation. FE + 1 small BE change. |
| `04-runbook-design-system.md` | RB-04 — adopt Spine Aurora tokens, type scale, theme toggle. **Front-end only.** |
| `05-runbook-frictionless-doors.md` | RB-05 — dropzone verify, landing rebuild, status re-voice. **Front-end only.** |

## The major efforts

| ID | Effort | Surface | Backend? | WoW | Effort | Depends on |
|---|---|---|---|---|---|---|
| RB-04 | Spine Aurora design system | global | no | ●●○ | S | — |
| RB-01 | 10-check verdict matrix | `#/r/<id>`, `#/verify` | no | ●●● | M | RB-04 (soft) |
| RB-02 | Tamper Lab | `#/lab` | no | ●●● | M | RB-01 |
| RB-03 | Run Theater (Spine) | `#/run` | 1 small change | ●●● | M | RB-04 |
| RB-05 | Frictionless doors | `#/`, `#/verify`, `#/status` | no | ●●○ | M | RB-04, RB-01 |

`S`≈1–2 days, `M`≈3–5 days for one front-end dev. WoW = differentiation impact.

## Recommended sequencing

The order front-loads pure front-end wins so the "WoW" lands before any Go is touched.

```
Phase 1 — Foundation + biggest win   (RB-04 → RB-01)
   Adopt the design system, then render the check matrix.
   After this phase the product already feels transformed.

Phase 2 — The differentiator         (RB-02)
   Tamper Lab. The shareable, screenshot-worthy feature. Reuses RB-01's matrix.

Phase 3 — The spectacle              (RB-03)
   Run Theater spine + the one small backend change (hash on streamed step).

Phase 4 — Cohesion + polish          (RB-05)
   Dropzone, landing rebuild, status re-voice, theme toggle wiring, a11y pass.
```

Each phase is independently shippable. If you ship only Phase 1, the product is
already dramatically better.

## Non-negotiable quality floor (applies to every runbook)

- Responsive to 360px.
- Visible `:focus-visible` keyboard focus on every interactive element.
- `prefers-reduced-motion` gates every animation.
- No new runtime dependencies — compose from the shared Nexus modules already
  served by `api/server.go handleShared` (`/lib/`, `/components/`, `/cinema-core/`,
  `/styles.css`).
- All copy in interface voice: active, specific, errors never apologize or stay vague.
- Existing Go tests stay green (`go test ./... -count=1`); web smokes stay green
  (`node --test web/test/*.mjs`).

## Status tracker

| ID | Status | Owner | PR | Notes |
|---|---|---|---|---|
| RB-04 | **done** | | | Built + verified (24 web tests, Go suite, headless theme/contrast E2E). All hex fallbacks stripped; 7-dot SVG brand; clamp type scale; theme toggle. **Correction:** real `data-theme` values are `dark`/`light`/`contrast` (display names Dark/Daylight/Phosphor), not `daylight`/`phosphor`. |
| RB-01 | **done** | | | Built + verified (unit + Go asset test + real-verifier E2E + live binary). Component served at `/checkMatrix.js` (not `/components/`, which the shared Nexus handler owns). |
| RB-02 | **done** | | | Built + verified (17 web tests, Go asset test, real-verifier E2E over all 7 forgeries, headless-browser render, real CDP click). Two-tier forgeries: naive→export_hash, six re-sealed→distinct inner checks. Engine served at `/tamper.js`. |
| RB-03 | **done** | | | Built + verified (24 web tests, Go worker hash test + asset test, full Run Theater browser E2E via CDP). **Backend item A implemented** (real per-step chain-link hashes). Spine served at `/spine.js`. |
| RB-05 | **done** | | | Built + verified (24 web tests, Go suite, full doors browser E2E via CDP). Ambient-spine landing + 1 CTA (duplicate card removed); dropzone verify (drop/pick/paste/sample → matrix); status re-voice; `#/readiness`→`#/status` redirect. |

## Key source-of-truth references

- Verifier (the engine): `web/` imports `/lib/portableVerifier.js` from
  `infrix-nexus-web` module cache:
  `~/go/pkg/mod/github.com/opendlt/infrix-nexus-web@v0.1.0/web/lib/portableVerifier.js`
- Design tokens: `…/infrix-nexus-web@v0.1.0/web/styles.css` ("Spine Aurora").
- Current SPA: `web/playground.js`, `web/playground.css`, `web/index.html`.
- Bundle → scene: `web/bundleScene.js`.
- Sample bundle (the demo data): `fixtures/sample-proof.infrix.json`.
- Run executor + step model: `worker/worker.go`.
- HTTP surface: `api/server.go`.
