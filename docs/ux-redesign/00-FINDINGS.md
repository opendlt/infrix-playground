# 00 — Findings (the brutally honest teardown)

Evidence-backed review of the playground as it exists. Every claim cites a file/line
so it's auditable, not opinion.

## Summary judgment

The playground is a **competent technical demo wearing a generic developer-tool
skin**, and it **actively hides its single most valuable asset** — the browser-side
cryptographic verifier. The trust architecture (offline re-verification, no node
trust, content-addressed receipts) is genuinely excellent and is the moat. The UI
*narrates* that trust in prose instead of letting a skeptic *feel* it.

## Finding 1 — The WoW is computed, then deleted (CRITICAL)

`/lib/portableVerifier.js::verifyPortablePackage()` returns:

```js
{ passed: boolean, checks: Array<{ name, passed, detail?, error? }> }   // 10 entries
```

The 10 checks are: `version`, `export_hash`, `bundle_data`, `plan_hash`
(↔ ApprovalEvidence binding), `outcome_digest`, `inclusion_proofs` (Merkle
reconstruction), `anchor_proof` (cross-binding), `trust_snapshot`,
`policy_decision_digest`, `plugin_versions`. Each carries a human-readable `detail`
(e.g. `"6 proofs reconstruct cleanly"`, `"matches an ApprovalEvidence.PlanHash"`).

The UI uses **only `result.passed`** and discards `checks[]`:
- `web/playground.js:332-338` (hosted receipt verify) — prints "PASSED"/"did NOT pass".
- `web/playground.js:408-412` (bring-your-own verify) — same.

**Impact:** the one thing no other blockchain playground offers — ten independent
cryptographic checks the visitor can watch run locally — is invisible. This is the
highest-leverage fix in the entire product. → RB-01.

## Finding 2 — Verification is gated behind a click and never eager

`renderReceipt` mounts a "Verify it yourself" button (`playground.js:293`) and only
runs the verifier on click (`:322`). The default state of the most important page is
a receipt card with the verdict *unrun*. The payoff requires a user action that most
visitors won't take. → RB-01 (run eagerly).

## Finding 3 — The design system is unused

The shared `/styles.css` is a complete system ("Spine Aurora"): a 7-stop gradient
`--spine-1`(#8E7BFF violet) … `--spine-7`(#FFD24E gold) **named after the flow
stages**, three themes (`dark`/`daylight`/`phosphor`), `--shadow-glow`, Inter +
JetBrains Mono, motion curves.

`web/playground.css` ignores nearly all of it:
- GitHub-blue fallbacks throughout: `var(--accent, #58a6ff)` (lines 26, 34, 35, 50, 51, 62, 67, 92, 93).
- Timid hero: `.pg-hero h1 { font-size: 1.7rem }` (line 41).
- Flat gray cards, no glow, no spine, no theme switch.

**Impact:** the product looks templated despite shipping on a distinctive system. → RB-04.

## Finding 4 — The governed flow is shown as a to-do list

The 7-stage flow is rendered three times, none of them as a *picture of linkage*:
1. `skeletonSteps()` (`playground.js:241`) — text.
2. `startRun()` streamed `<ul class="pg-steps">` with ○/◔/✓/✗ glyphs (`:206-237`, CSS `:72-79`).
3. `bundleScene.js` — 7 colored rectangles in Cinema.

The hash chain that *makes* it tamper-evident — `bundleData.chain.links[]`, 6 links
each with `contentHash`/`prevHash`/`sequence`/`timestamp` — is never shown. The claim
is asserted, never demonstrated. → RB-03.

## Finding 5 — Merkle inclusion proofs are silent

`inclusionProofs[]` (6 in the sample) are reconstructed by the verifier
(`verifyMerkleInclusionProof`, `portableVerifier.js:352`) and never visualized. The
most inherently "wow" cryptographic object in the bundle produces no pixels. → RB-01
(check 6 detail) + optional deep-dive.

## Finding 6 — Redundant, low-signal home page

`renderHome` (`playground.js:103`) shows three action cards. **Two of them — "Run a
governed escrow" and "Watch a replay" — call the identical function** `startRun(selectedMode)`
(`:115`, `:117`). The hero copy "Feel Infrix in your browser" says nothing only
Infrix can say. No 5-second "feel it" moment. → RB-05.

## Finding 7 — The run climax is a route change

On `done`, `startRun` does `location.hash = '#/r/'+receiptId` (`playground.js:231`),
instantly wiping the run animation. There is no payoff beat — the emotional peak of
"the proof is sealed" is a navigation. → RB-03.

## Finding 8 — Verify view is a friction wall

`renderVerify` (`playground.js:362`) leads with a bare `<textarea>` expecting pasted
JSON. No drag-drop, no file picker, no dropzone, dead empty state. The most
skeptical-user surface (BYO proof) has the highest friction. → RB-05.

## Finding 9 — Share link isn't even a link

`renderReceipt` renders the share URL as plain text:
`share.textContent = 'Share link: ' + location.origin + '/#/r/' + id` (`playground.js:299`).
Not clickable, not copyable, no confirm. → RB-01 (Copy button) / RB-05.

## Finding 10 — Receipt page has no hierarchy

`renderReceipt` lays out the verdict card and the Cinema replay as two equal gray
`pg-panel`s in a 1fr/1fr split (`playground.js:279-313`, CSS `.pg-split` `:82`). The
verdict — the entire point — visually competes with a replay widget. → RB-01 (matrix
is hero, Cinema demoted).

## Finding 11 — Readiness leaks ops language

`renderReadiness` (`playground.js:424`) prints a `<dl>` of `ready`/`unavailable`/
`never`. Internal operational vocabulary on a public marketing surface. → RB-05.

## Finding 12 — No empty/idle states, no theme control, no motion gating audit

- Verify with no input: dead box.
- No theme toggle despite three themes existing.
- Animations present (`.pg-action` transform hover) are not gated on
  `prefers-reduced-motion`. → RB-04 / RB-05.

## What is already good (keep, build on)

- The **trust architecture**: offline re-verification, "node is a runner not a
  verifier", content-addressed receipt IDs (`api/receipts.go DeriveID`).
- **`mountUserError`** structured error cards (`playground.js` uses them well).
- **Security posture**: flow allowlist, rate limit, no mainnet writes, opt-in
  analytics with redaction (`playground.js:26-55`).
- **The verifier itself** — the entire WoW engine. It just needs to be *seen*.
