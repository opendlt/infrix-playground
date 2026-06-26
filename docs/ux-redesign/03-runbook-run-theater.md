# RB-03 — The Run Theater (Spine)

**Goal:** replace the ○/✓ to-do list with a horizontal **Spine** that assembles
itself left-to-right as the run streams, each node emitting its real hash, ending in
a "PROOF SEALED" payoff beat instead of an instant route change.

**Surface:** `#/run` (replaces `startRun`). **Backend changes:** one small, optional-
but-recommended (item A). **WoW:** ●●● **Effort:** M (~4 days). **Depends on:** RB-04.

> **STATUS: DONE & VERIFIED.** Backend item A was implemented in full (not skipped).
> See "As built" at the bottom for design decisions, deviations, and the full
> verification log (including a real-browser Run Theater E2E via CDP).

---

## 1. Why

The 7-stage governed flow is the brand — the shared design system literally ships a
7-stop gradient named after the stages (`--spine-1` violet … `--spine-7` gold). Today
it's a checklist (Finding 4) and the climax is a `location.hash` assignment (Finding
7). The Spine turns the run into the product's signature motion moment.

## 2. Deliverables

1. `web/spine.js` (served at `/spine.js`; see "As built") — the spine widget (also reused by RB-05 landing ambient mode).
2. Reworked `startRun()` → renders the spine, drives it from SSE, plays the seal beat.
3. CSS for the spine + active-stage card + seal animation.
4. **Backend item A** (recommended): a short hash per streamed `Step`.
5. Tests: `web/test/spine.test.mjs`.

## 3. Stage → spine-color mapping

The flow has 9 steps; the gradient has 7 stops. Map the *visual* stages to the ramp
and treat `export`/`verify` as meta-steps (white/accent halo, not a spine color):

| step key | spine | color |
|---|---|---|
| intent | 1 | #8E7BFF violet |
| plan | 2 | #6E9CFF indigo |
| policy | 3 | #4EC8FF cyan |
| approval | 4 (interp 3→4) | cyan→mint |
| credential | 5 (interp 4→5) | mint→emerald |
| outcome | 5 | #6EFFB0 emerald |
| anchor | 7 | #FFD24E gold |
| export | — | accent halo |
| verify | — | white halo |

Read the actual hex from CSS so themes work:
```js
function spineVar(n) {
  return getComputedStyle(document.documentElement).getPropertyValue(`--spine-${n}`).trim();
}
```
Provide `spineColor(i, total)` that samples/interpolates these in sRGB and caches per
`document.documentElement.dataset.theme`.

## 4. Component: `web/spine.js` (served at `/spine.js`)

```js
// web/spine.js
// A horizontal flow spine. Nodes go pending -> running -> complete (or failed),
// each filling its connector with the stage gradient. Reused in ambient mode on
// the landing page (autoplay loop, no SSE).

function el(tag, cls) { const n = document.createElement(tag); if (cls) n.className = cls; return n; }

export function mountSpine(container, steps, opts = {}) {
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const wrap = el('div', 'pg-spine');
  wrap.setAttribute('role', 'list');
  const nodes = {};
  steps.forEach((s, i) => {
    const node = el('div', 'pg-spine-node');
    node.setAttribute('role', 'listitem');
    node.dataset.state = 'pending';
    node.dataset.key = s.key;
    node.style.setProperty('--node-color', s.color || 'var(--accent)');
    const dot = el('span', 'pg-spine-dot');
    const label = el('span', 'pg-spine-label'); label.textContent = s.label;
    const hash = el('code', 'pg-spine-hash'); // filled when known
    node.append(dot, label, hash);
    if (i < steps.length - 1) wrap.appendChild(node);
    else wrap.appendChild(node);
    nodes[s.key] = { node, dot, hash };
  });
  if (container) container.replaceChildren(wrap);

  return {
    el: wrap,
    set(key, state, hashText) {
      const n = nodes[key]; if (!n) return;
      n.node.dataset.state = state;
      if (hashText) n.hash.textContent = hashText;
      if (state === 'complete' && !reduce) {
        n.dot.animate([{ transform: 'scale(1.4)' }, { transform: 'scale(1)' }],
          { duration: 220, easing: 'cubic-bezier(0.4,0,0.2,1)' });
      }
      if (state === 'failed' && !reduce) {
        n.node.animate([{ transform: 'translateX(-3px)' }, { transform: 'translateX(3px)' },
          { transform: 'none' }], { duration: 180 });
      }
    },
    seal() { // the payoff beat
      if (reduce) return Promise.resolve();
      wrap.classList.add('pg-spine-sealing');
      return new Promise((res) => setTimeout(() => { wrap.classList.add('pg-spine-sealed'); res(); }, 600));
    },
  };
}
```

## 5. Rework `startRun()` (`web/playground.js`)

1. Build the step list with colors (see §3) and `mountSpine`.
2. Render the spine in `pending`. Below it, an **active-stage card** that swaps
   (slide-up) on each step event showing the stage's human label + its hash.
3. SSE handling (the stream already exists, `playground.js:217`):
   ```js
   src.onmessage = (ev) => {
     const e = JSON.parse(ev.data);
     if (e.type === 'step' && e.step) {
       spine.set(e.step.key, e.step.status, e.step.hash /* from backend item A */);
       if (e.step.status === 'running') swapActiveCard(e.step);
     } else if (e.type === 'done' && e.receiptId) {
       src.close();
       spine.seal().then(() => { location.hash = `#/r/${e.receiptId}`; });
     } else if (e.type === 'error') { /* node -> failed, inline retry */ }
   };
   ```
4. **Route:** add `if (hash === '/run')` if you want a deep-link; otherwise keep
   `startRun` invoked from the landing CTA.

## 6. Backend item A — hash per step (recommended)

Without this the spine shows decoration, not real linkage. Small, contained change.

- `worker/worker.go`:
  - Add `Hash string \`json:"hash,omitempty"\`` to `Step` (line ~67).
  - When narrating completed steps (the loop at ~191), set each step's `Hash` from the
    corresponding `bundleData.chain.links[i].contentHash` (first 8 bytes, hex). The
    package is in hand (`resp.Package`); map step key → link by sequence/type.
- `api/runs.go` / `api/events.go`: the `Step` is already serialized into the SSE
  `step` event; once the field exists it flows through. Verify the event marshaller
  includes it.
- Update `worker_test.go` to assert the hash is populated for chain-backed steps.

If you choose to skip item A for v1, have the spine omit the hash line (don't fake it).

## 7. CSS (`web/playground.css`)

```css
.pg-spine { display: flex; gap: 0; align-items: flex-start; overflow-x: auto;
  padding: 24px 4px; }
.pg-spine-node { display: flex; flex-direction: column; align-items: center;
  gap: 6px; min-width: 96px; position: relative; }
.pg-spine-node::after { content: ''; position: absolute; top: 9px; left: 50%; width: 100%;
  height: 2px; background: var(--spine-track); z-index: 0; }
.pg-spine-node:last-child::after { display: none; }
.pg-spine-node[data-state="complete"]::after { background: var(--node-color); }
.pg-spine-dot { width: 18px; height: 18px; border-radius: 50%; z-index: 1;
  background: var(--spine-track); border: 2px solid var(--spine-track); }
.pg-spine-node[data-state="running"] .pg-spine-dot {
  background: var(--node-color); border-color: var(--node-color);
  animation: pg-spine-pulse 1.2s ease-in-out infinite; }
.pg-spine-node[data-state="complete"] .pg-spine-dot {
  background: var(--node-color); border-color: var(--node-color); }
.pg-spine-node[data-state="failed"] .pg-spine-dot {
  background: var(--alert); border-color: var(--alert); }
.pg-spine-node[data-state="pending"] { opacity: 0.45; }
.pg-spine-label { font-size: 0.74rem; color: var(--text-secondary); text-align: center; }
.pg-spine-hash { font: 500 0.68rem var(--mono); color: var(--text-dim); }
@keyframes pg-spine-pulse {
  0%,100% { box-shadow: 0 0 0 0 var(--accent-glow); }
  50%     { box-shadow: 0 0 0 6px transparent; } }
.pg-spine-sealing { filter: brightness(1.4); transition: filter 0.6s; }
.pg-spine-sealed::after { /* optional "PROOF SEALED" chip drop-in */ }
@media (prefers-reduced-motion: reduce) {
  .pg-spine-node[data-state="running"] .pg-spine-dot { animation: none; }
}
```

## 8. Tests `web/test/spine.test.mjs`

- `mountSpine` with N steps → N nodes, all `data-state="pending"`.
- `.set('plan','complete','769e4062')` → node state complete, hash text shown.
- `.set(key,'failed')` → state failed.
- reduced-motion shim → no `animate` calls (or no throw).

## 9. Acceptance criteria

- [ ] Spine renders all stages within 100ms of run start (from `started.steps`).
- [ ] Each completed stage shows a real hash (when item A landed) or no hash line (if deferred).
- [ ] On done, the seal beat plays, *then* navigates to the verdict.
- [ ] On error, the failing node is obvious; inline retry offered.
- [ ] Reduced-motion: instant state colors, no pulse/seal.
- [ ] Horizontal scroll on narrow viewports keeps the spine usable.
- [ ] `go test ./... -count=1` and `node --test web/test/*.mjs` green.

## 10. Rollback

Front-end is additive — restore `startRun`'s `<ul>` rendering, delete `spine.js`.
Backend item A is a single nullable field; reverting the struct field + emit line is
safe and the SSE event simply stops carrying `hash`.

---

## As built (decisions, deviations, verification)

Implemented in full — **including backend item A** (the user directed no deferment).

### Backend item A — real per-step hashes (`worker/worker.go`)
- `Step` gained `Hash string \`json:"hash,omitempty"\``; it flows through `api/runs.go`
  unchanged (the SSE `step` event already serializes the whole `Step`).
- New `stepHashes(pkg)` resolves each flow step to the **real** 8-byte hex of its
  artifact, honestly and explicitly: `intent/plan/policy/approval/credential/outcome`
  → the matching evidence-chain link's content hash (`policy`→`policy_decision` link;
  `credential`→`external_proof`/`trust_assumption` link); `anchor` → the chain digest
  anchored to L0 (`chain.ChainHash`); `export` → the package `ExportHash`; `verify` →
  **no** hash (the verdict is computed, not a stored artifact). Missing/unparseable →
  the key is simply absent (the UI shows no hash line — never a fake one).
- Pinned by `TestRunStepsCarryRealChainHashes`: chain-backed stages + anchor + export
  carry 16-hex hashes, `verify` carries none, and `intent` equals the fixture's first
  chain link (`f660e00519777cde`).

### Frontend
- **Paths:** `web/spine.js` served at `/spine.js` (the `/components` prefix is shared-
  handler-owned), embedded in `web/web.go`. Imports: `mountSpine`, `stageColor`.
- **`stageColor(key)`** reads the theme's spine vars via `getComputedStyle`,
  interpolating `approval`/`credential` between adjacent stops, caching per theme, so
  the gradient is correct under the dark/daylight/phosphor themes (RB-04).
- **Streaming reality → pacing:** the worker narrates all stages as `complete` in a
  tight burst (it is deterministic post-run narration), so there is no server-sent
  `running` phase. The client buffers events in an async channel and paces them
  (~150ms/stage) to produce the left-to-right assembly — the **data (order, hashes) is
  entirely real; only the cadence is presentation**, mirroring the RB-01 matrix
  stagger. Reduced-motion applies everything instantly.
- **Payoff beat:** on `done`, `spine.seal()` plays, *then* `location.hash` navigates —
  no longer an instant route change (fixes Finding 7). The active-stage card narrates
  each stage and its hash, ending "Proof sealed".
- **Failure:** a failed step or `error`/SSE-drop event marks the node failed and shows
  an inline "Try again" that re-runs — no dead end.
- **`#/run` deep-link:** deliberately **not** added (the runbook scopes it optional and
  recommends against). The run is initiated by the CTA action; auto-starting on every
  navigation/refresh would create churn and hit the rate limiter. `startRun` is invoked
  from the home action card, as the runbook's §5.4 default specifies.
- Removed the dead `.pg-step*`/`.pg-run` CSS; added spine + stage-card + failnote CSS
  with token fallbacks.

## Verification log (all green)
- `node --test web/test/*.mjs` → **24 pass** (4 bundleScene + 6 checkMatrix + 7 tamper
  + 7 spine). Spine tests pin node lifecycle, real-hash display, the reduced-motion
  contract (zero animations), the seal beat, and `stageColor` gradient resolution.
- `go build ./... && go vet ./... && go test ./... -count=1` → **all pass**, including
  `TestRunStepsCarryRealChainHashes` and `TestSPAandSharedAssets` extended for `/spine.js`.
- **Full Run Theater browser E2E (Edge + CDP):** a fake node served the sample bundle;
  the real playground server ran against it; a real click on "Run a governed escrow"
  produced a 9-node spine showing the **real** intent hash `f660e00519777cde`, played
  the seal, navigated to `#/r/eb63a2166f9e`, and the verdict rendered **10 / 10**
  ("…reconstructed locally"). End to end, real data, real browser.
