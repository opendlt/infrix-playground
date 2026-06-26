# Infrix Playground — UX/UI Redesign Spec ("Trust Microscope")

Status: proposal • Audience: front-end + a thin slice of back-end • Scope: `web/`, with 4 small `api/` additions

This is an implementation-grade spec. Every section names files, data sources, DOM
structure, tokens, and acceptance criteria. A dev team should be able to build it
without further design rounds.

---

## 0. The one-sentence product

> **A trust microscope: run a governed flow, then watch ten cryptographic checks
> verify it in *your* browser — against a server you don't trust — each link of
> the chain laid bare and explained in plain language.**

Every decision below serves that sentence. If a feature doesn't make the math more
visible, more local, or more legible, it's cut.

---

## 1. What's wrong today (the teardown)

| Area | Current state | Problem |
|---|---|---|
| **Verifier output** | `verifyPortablePackage()` returns `{passed, checks[]}` with 10 richly-detailed checks; UI renders the boolean and discards `checks[]` (`playground.js:336`, `:410`). | The single most valuable, most differentiated asset in the product is invisible. |
| **Design tokens** | `playground.css` uses `var(--accent, #58a6ff)` GitHub-blue fallbacks, flat gray cards. | The shared "Spine Aurora" system (7-stage gradient, glows, Inter/JetBrains Mono, 3 themes) is unused. Looks templated. |
| **The flow** | Rendered as a `<ul>` checklist with ○/◔/✓ glyphs (`startRun`). | A governed cryptographic flow shown as a grocery list. No hash chain, no linkage, no spectacle. |
| **Hash chain** | `bundleData.chain.links[]` (6 links, each with `contentHash`/`prevHash`/`sequence`) never shown. | The "tamper-evident chain" claim is asserted, never demonstrated. |
| **Merkle proofs** | 6 `inclusionProofs[]` reconstructed by the verifier; never visualized. | The most "wow"-able cryptographic object in the bundle is silent. |
| **Home page** | Hero + 3 nearly-identical text cards, two of which (`Run`, `Watch a replay`) do the *exact same thing* (`startRun`). | Redundant, low-signal, zero spectacle. No "feel it in 5 seconds" moment. |
| **Hero copy** | "Feel Infrix in your browser." | Generic. Says nothing only Infrix can say. |
| **Run → receipt** | Hard hash navigation (`location.hash = '#/r/...'`) wipes the run animation instantly. | No payoff moment; the climax is a route change. |
| **Verify view** | Bare `<textarea>`; paste JSON. | Friction wall. No drag-drop, no file picker, no "what is this" affordance. |
| **Receipt view** | Two equal gray panels (receipt card + Cinema). | No hierarchy. The verdict — the whole point — competes with a replay widget. |
| **Readiness view** | `<dl>` of "ready/unavailable". | Internal-ops language leaked to a marketing surface. |
| **Empty/idle states** | none | Verify view with no input is a dead box. |

**Net:** the product *tells* a trust story in prose and *hides* the cryptographic
proof that would let a skeptic *feel* it.

---

## 2. Design language: actually use Spine Aurora

All tokens already exist in the shared `/styles.css` (`infrix-nexus-web`). Stop
using GitHub-blue fallbacks. Concretely:

### 2.1 Token adoption (replace in `playground.css`)

- `--accent #58a6ff` fallbacks → `--accent` (#8F82FF violet) with **no** hex fallback (the shared sheet always loads first; if it fails, that's a hard error worth surfacing).
- Cards: `var(--surface)` + `var(--border)`; hover → `var(--border-bold)` + `box-shadow: var(--shadow-glow)`.
- Mono data (hashes, ids): `var(--mono)` (JetBrains Mono), already partly used.
- Body/display: `var(--font)` (Inter).

### 2.2 The Spine — the signature element

The 7-stage gradient is the brand. Map flow stages to spine colors **everywhere**:

```
intent      → --spine-1  #8E7BFF  violet
plan        → --spine-2  #6E9CFF  indigo
policy      → --spine-3  #4EC8FF  cyan      (your flow inserts policy/approval/credential —
approval    → --spine-3→4 blend             interpolate across the 7-stop ramp; see §4.2)
credential  → --spine-4  #4DE3B5  mint
outcome     → --spine-5  #6EFFB0  emerald
anchor      → --spine-7  #FFD24E  gold
verify      → render as WHITE/accent halo, not a spine color — it's the meta-step
```

**Type scale** (override the timid 1.7rem hero):

| Role | Size / weight | Face |
|---|---|---|
| Hero display | `clamp(2.4rem, 5vw, 4rem)` / 800, `letter-spacing: -0.02em` | Inter |
| Section h2 | 1.25rem / 700 | Inter |
| Body | 1rem / 400, `line-height 1.6` | Inter |
| Data / hash | 0.8rem / 500 | JetBrains Mono |
| Eyebrow/label | 0.72rem / 700, `text-transform: uppercase`, `letter-spacing: 0.12em`, `color: var(--text-dim)` | Inter |

**Motion:** respect `prefers-reduced-motion` (gate every animation). Use
`--motion-fast` / cubic-beziers from tokens. One orchestrated moment beats ten
scattered ones (see §4 and §5).

### 2.3 Theme switch

Shared sheet ships `dark` (default) / `daylight` / `phosphor` via
`:root[data-theme=...]`. Add a 3-state toggle in the header (☾ / ☀ / terminal).
Persist to `localStorage('pg:theme')`; set `document.documentElement.dataset.theme`
on boot before first paint to avoid flash.

---

## 3. Information architecture (new)

Collapse from "5 flat routes + redundant cards" to a **single guided spine** with
power-user side-doors.

```
#/            Landing — the live spine hero + ONE primary CTA + BYO-proof door
#/run         The Run Theater  (was startRun, fully rebuilt — §4)
#/r/<id>      The Verdict       (was renderReceipt, rebuilt — §5)
#/verify      Drop a proof      (rebuilt with dropzone — §6)
#/lab         "Tamper Lab"      (NEW killer feature — §7)
#/metamask    Sign an intent    (keep, restyle)
#/status      Readiness         (rename, re-voice — §8)
```

Kill the duplicate "Watch a replay" card (it calls `startRun`, identical to "Run").

---

## 4. The Run Theater (`#/run`) — replaces `startRun`

The current checklist is the biggest spectacle miss. Rebuild as a **horizontal
spine that builds itself left-to-right as SSE events arrive**, with each stage
emitting its real hash.

### 4.1 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  EYEBROW: GOVERNED ESCROW · ANONYMOUS DEMO (fixture-backed)            │
│  H1: Building a tamper-evident proof                                   │
│                                                                        │
│   ●───────●───────●───────◍ . . . ○ . . . ○ . . . ○                    │  ← the spine
│  intent  plan   policy  approval cred  outcome anchor                  │
│   violet indigo  cyan    cyan→mint ...           gold                  │
│                                                                        │
│  ┌── active stage card (slides up as each completes) ──────────────┐   │
│  │  APPROVAL  ·  acc://…/admin  ·  role: approver                   │   │
│  │  contentHash 76 9e 40 62 …  ← links to plan's hash               │   │
│  │  "Operator approval is cryptographically bound to the plan."     │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────┘
```

### 4.2 The spine widget (`web/components/spine.js`, NEW)

- A flex row of N nodes (N = `flowSteps.length`, minus `anchor` when unanchored — mirror `bundleScene.js` logic).
- Each node: a 14px dot + connector line to the next.
- **Color ramp:** precompute a function `spineColor(i, total)` that samples the 7
  CSS spine vars by interpolating in sRGB (read via
  `getComputedStyle(document.documentElement).getPropertyValue('--spine-N')`). Cache per theme.
- **States** (`data-state` on each node): `pending` (track color, 40% opacity),
  `running` (pulsing ring, `@keyframes spine-pulse` 1.2s), `complete` (filled +
  brief `scale(1.4)→1` pop), `failed` (`--alert`, shake once).
- Connector fills with a gradient sweep from prev color → this color over 280ms
  when the node completes (`transition: --fill 0.28s`; use a width-animated overlay
  if CSS custom-prop transitions aren't available).
- `prefers-reduced-motion`: drop pulse/pop/shake; keep instant state colors.

### 4.3 Wiring to SSE (rework `startRun`)

The SSE stream already emits `{type:'step', step:{key,status}}` and `{type:'done', receiptId}`.

1. On `POST /api/runs`, render the full spine in `pending` from `started.steps`.
2. On each `step` event: set node `data-state`, animate connector, and **swap the
   active-stage card** (slide-up) showing that stage's human label + its hash.
   - **Problem:** the SSE `step` event today carries no hash (`worker/worker.go:67`
     `Step{Key,Label,Status}`). **Back-end change required** (§9, item A): include
     a short hash per step so the theater shows *real* linkage, not decoration.
3. On `done`: **do not hard-navigate.** Play a 600ms "seal" animation (the whole
   spine glows gold→white, a `✓ PROOF SEALED` chip drops in), *then*
   `location.hash = '#/r/'+receiptId`. This is the payoff beat.
4. On `error`: the running node turns `--alert`, card shows the error via the
   existing `mountUserError`. Offer "Try again" inline (no full reset).

### 4.4 Acceptance criteria
- Spine renders all stages within 100ms of run start (skeleton from `started.steps`).
- Each completed stage shows a non-empty hash from the backend.
- Reduced-motion users get instant color states, no payoff animation jank.
- Failure leaves the spine readable (which stage failed is obvious at a glance).

---

## 5. The Verdict (`#/r/<id>`) — rebuild `renderReceipt`

This is where the WoW lives. Current code shows verdict + Cinema as co-equal gray
panels and **only runs the 10 checks when you click a button, then discards them.**
Invert everything.

### 5.1 Run the verifier eagerly, render the matrix

On entering the view, after `pkgPromise` resolves, **immediately** call
`verifyPortablePackage(pkg)` (don't wait for a click) and render the full
`checks[]` array as the hero of the page.

```
┌─────────────────────────────────────────────────────────────────────┐
│   EYEBROW: VERIFIED IN YOUR BROWSER · THIS SERVER WAS NOT TRUSTED     │
│                                                                       │
│   ✓  PROOF HOLDS              L3  ·  G2                               │
│   10 / 10 cryptographic checks reconstructed locally                  │
│   ▓▓▓▓▓▓▓▓▓▓  (animated fill, one tick per check, ~80ms stagger)      │
│                                                                       │
│   ┌─ THE TEN CHECKS ───────────────────────────────────────────┐     │
│   │ ✓ 1  Version            v4                                  │     │
│   │ ✓ 2  Export hash        recomputed 1a2b3c… = stored ✓       │     │
│   │ ✓ 3  Bundle data        embedded bundle id=ev-…             │     │
│   │ ✓ 4  Plan ↔ approval    matches an ApprovalEvidence.PlanHash│     │
│   │ ✓ 5  Outcome digest     matches embedded                    │     │
│   │ ✓ 6  Inclusion proofs   6 proofs reconstruct cleanly  [view]│     │
│   │ ✓ 7  Anchor binding     bundle not anchored; skipped        │     │
│   │ ✓ 8  Trust snapshot     1 entry, block 987                  │     │
│   │ ✓ 9  Policy digest      1 decision: allow                   │     │
│   │ ✓10  Plugin provenance  pkg.nexus.fixture.demo @1.0.0       │     │
│   └────────────────────────────────────────────────────────────┘     │
│                                                                       │
│   [ Download .infrix.json ]  [ Re-verify line by line ]  [ Tamper Lab ]│
└─────────────────────────────────────────────────────────────────────┘
```

### 5.2 The check matrix component (`web/components/checkMatrix.js`, NEW)

- Input: the `{passed, checks[]}` object verbatim.
- Each row: status glyph (`✓`/`✗`), index, a **friendly title** (map check `name`→
  title via a lookup table, below), and the verifier's own `detail`/`error` in mono.
- **Plain-language layer:** each check `name` maps to a one-line "what this proves"
  tooltip/expander. This is the no-onboarding teaching moment — the spec text is
  already documented at the top of `portableVerifier.js`; lift it:

  ```
  version          → "The proof uses a format this verifier understands."
  export_hash      → "Nothing in the file was altered after it was exported."
  bundle_data      → "The evidence bundle is well-formed."
  plan_hash        → "The approval is bound to the exact plan that ran — not a different one."
  outcome_digest   → "The recorded outcome matches what the bundle commits to."
  inclusion_proofs → "Every step is provably part of the same tamper-evident chain."
  anchor_proof     → "If anchored, the on-ledger anchor matches this bundle exactly."
  trust_snapshot   → "The trust profile in force was captured at a real block height."
  policy_decision_digest → "The policy decisions are exactly the ones recorded — none added or removed."
  plugin_versions  → "Every plugin that touched this is identified by id, version, and code hash."
  ```

- **"Re-verify line by line"** button: re-runs with an artificial 120ms stagger so a
  skeptic literally watches each check turn green one at a time. (Pure theater over
  real computation — the checks genuinely run.) Gate stagger on reduced-motion.
- On `passed:false`: the failing check is the only red row, expanded by default,
  with its `detail` and the plain-language line "Here's exactly what didn't add up."

### 5.3 Demote, don't delete, Cinema + receipt card

- Cinema replay and the existing `mountProofReceipt` card move **below** the matrix,
  in a `<details>`-style "Inspect the full proof" disclosure (collapsed by default).
- Keep the share line, but restyle as a one-click **Copy link** button with a
  "Copied ✓" micro-confirm (current `share.textContent = 'Share link: '+url` is not
  even clickable — fix that).

### 5.4 Acceptance criteria
- The 10 checks render and animate within ~300ms of bundle load, no click required.
- Every check shows the verifier's real `detail`, plus a plain-language line.
- A tampered/failing bundle makes the failing check unmissable and explains it.
- Cinema still works, just demoted.

---

## 6. Drop a proof (`#/verify`) — kill the friction wall

Replace the bare textarea-first layout.

- **Primary affordance:** a large dashed **dropzone** ("Drop a `.infrix.json` proof,
  or click to choose") with drag-over highlight (`--accent-glow`).
- **Secondary:** "Paste JSON" toggles the textarea (keep the existing path).
- **Tertiary:** "Load the sample" (existing `/api/sample-bundle`).
- On a valid drop/paste: jump straight into the §5 check matrix (reuse
  `checkMatrix.js`) — same verdict surface as a hosted run. One component, two doors.
- Empty state copy: "No login. No upload to us — verification runs entirely on your
  machine." (Reinforce the thesis on the most skeptical-user surface.)

---

## 7. 🔥 The Tamper Lab (`#/lab`) — the killer differentiator (NEW)

**This is the feature no blockchain project has and the one that will get shared.**

Premise: *let the user try to cheat the proof and watch it fail.* Load the sample
bundle into an editable, structured view. Let them flip one byte — change an
approver identity, bump an amount, swap a hash, delete a policy decision — then hit
**Re-verify** and watch the exact check go red with the exact reason.

### 7.1 Why this works
- It turns a passive demo into an **interactive falsification game** — the fastest
  way to *believe* tamper-evidence is to fail to beat it.
- It needs **zero backend**: everything runs through the existing client
  `verifyPortablePackage`. Pure front-end.
- It's inherently viral / screenshot-friendly ("I tried to forge an Infrix proof and
  it caught me in the browser").

### 7.2 UX

```
┌── EDITABLE BUNDLE (structured, not raw JSON) ──┬── LIVE VERDICT ──────────┐
│  Approver   acc://nexus-fixture.acme/admin  ✎  │  ✓ 10/10 checks pass     │
│  Amount     [ editable ]                     ✎  │                          │
│  Plan hash  29 fd 77 08 …                    ✎  │  ↓ user edits approver   │
│  Policy     allow  (must-allow)              ✎  │                          │
│  ...                                            │  ✗ 4 Plan ↔ approval     │
│                                                 │    "package PlanHash …   │
│  [ Tamper a field ▾ ]   [ Reset ]   [Re-verify] │     not in approval entry"│
└─────────────────────────────────────────────────┴──────────────────────────┘
```

- Provide **preset tamper buttons** ("Forge the approver", "Inflate the amount",
  "Swap a chain hash", "Drop a policy decision") so users with no idea what to edit
  get the aha in one click — frictionless, no onboarding.
- Each preset mutates the in-memory `pkg`, re-runs the verifier, and the §5 matrix
  re-renders with the now-failing check highlighted and explained.
- A running "tamper score": "You've tried 3 forgeries. The proof caught all 3."

### 7.3 Implementation
- New view `renderLab()` in `playground.js` + `web/components/bundleEditor.js`.
- Deep-clone the sample bundle; mutate clone; call `verifyPortablePackage(clone)`.
- Preset mutations are tiny pure functions (e.g. `forgeApprover(pkg)` sets
  `bundleData.approvalEvidence[0].identity = 'acc://attacker.evil/admin'`).
- No persistence, no network. Reset reloads `/api/sample-bundle`.

### 7.4 Acceptance criteria
- Every preset tamper produces a **specific** failing check with the real reason.
- Reset returns to 10/10.
- Works offline after first load (the sample is already fetched).

---

## 8. Readiness → "What this instance can do" (`#/status`)

Re-voice from ops-speak to user-facing. Replace `ready/unavailable/never` `<dl>` with
plain capability statements + a status dot:

- ● **Run a demo** — yes, right now, no wallet.
- ● **Live testnet (Kermit)** — enabled / "off on this instance."
- ● **Verify your own proof** — yes, in your browser.
- ● **Mainnet writes** — never. (frame as a *safety guarantee*, not a missing feature.)

Keep `/api/readiness` as-is; only the render changes.

---

## 9. Landing (`#/`) — lead with the spine, not three cards

- **Hero = a live, looping, muted micro-animation of the spine self-assembling**
  (reuse `spine.js` in an autoplay/ambient mode), behind one headline:

  > **See a governed deal prove itself.**
  > Run it, then verify every cryptographic link in your own browser. No install,
  > no wallet, no trust in us.

- **One** primary CTA: `Run the escrow →` (goes to `#/run`).
- **One** secondary door: `Already have a proof? Verify it →` (`#/verify`).
- A thin "or break one in the Tamper Lab →" tertiary link.
- Mode selector stays but moves into a compact inline control near the CTA (not a
  separate section); keep the existing anonymous/kermit logic.
- Move analytics opt-in + MetaMask + status into a slim footer row. They're not
  hero material.

---

## 10. Required back-end changes (small, listed for completeness)

| # | Change | File | Why |
|---|---|---|---|
| A | Add a short hash (e.g. first 8 bytes of each link's `contentHash`, hex) to each streamed `Step`, or add a `hash` field to the SSE `step` event. | `worker/worker.go` (`Step` struct + emit), `api/runs.go`/`events.go` | The Run Theater (§4) shows *real* linkage per stage instead of decoration. Optional but high-impact. |
| B | (Optional) expose `chain.links[]` summary on the receipt endpoint so the Verdict page can draw the linked chain without re-parsing the whole bundle client-side. | `api/server.go handleGetReceipt` | Convenience; client can also derive from the bundle it already fetches. Skip if minimizing backend churn. |

Everything else (§5, §6, §7) is **front-end only** — the client already fetches the
full bundle and already has the verifier.

---

## 11. Build order (so value lands early)

1. **§2 token adoption + §5 check matrix** — biggest WoW-per-hour. The matrix alone
   transforms the product. (Front-end only.)
2. **§7 Tamper Lab** — the differentiator and the shareable. (Front-end only.)
3. **§4 Run Theater spine** + backend item A. (The spectacle.)
4. **§6 dropzone, §9 landing, §8 status, theme toggle.** (Polish + cohesion.)
5. **§5.4 / §4.4 / §7.4 acceptance passes** + reduced-motion + keyboard-focus audit.

---

## 12. Quality floor (non-negotiable, no announcement)

- Responsive to 360px (the §5 matrix and §7 lab become single-column stacks).
- Visible keyboard focus on every interactive element (`:focus-visible` with
  `--accent` outline — already partly present, extend it).
- `prefers-reduced-motion`: every animation in §4/§5/§7 gated.
- `prefers-color-scheme` → default theme; user toggle overrides.
- All copy in interface voice: active, specific, no apologies in errors (the existing
  `mountUserError` is good — keep using it).
- No new dependencies. Everything composes from the shared Nexus modules already
  served by `handleShared`.

---

## 13. Copy rewrite cheatsheet (drop-in)

| Where | From | To |
|---|---|---|
| Landing H1 | "Feel Infrix in your browser" | "See a governed deal prove itself." |
| Landing sub | "Run a real governed flow, watch it in Cinema…" | "Run it, then verify every cryptographic link in your own browser. No install, no wallet, no trust in us." |
| Run H1 | "Running governed escrow" | "Building a tamper-evident proof" |
| Verdict eyebrow | (none) | "VERIFIED IN YOUR BROWSER · THIS SERVER WAS NOT TRUSTED" |
| Verdict headline | "Browser verification PASSED — verified without trusting this server." | "Proof holds. 10/10 checks reconstructed locally." |
| Verify empty | (none) | "No login. No upload to us — verification runs entirely on your machine." |
| Status mainnet | "Mainnet writes: never" | "Mainnet writes — never. This playground can't touch real funds." |

---

## 14. The one risk worth taking

Spend the boldness on the **Tamper Lab + line-by-line re-verify**. Everything else
stays disciplined and quiet (Aurora-dark, generous whitespace, one accent). The
memorable thing — the thing people screenshot and send to a colleague — is *"I tried
to forge it and the math caught me, in my own browser."* That is a claim no other
blockchain playground can make, and it's already 90% built in your verifier. Finish it.
