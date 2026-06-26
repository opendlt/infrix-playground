# RB-05 — Frictionless Doors (landing, dropzone verify, status re-voice)

**Goal:** remove every onboarding wall. Rebuild the landing around the live spine and
one CTA, turn the bare paste-textarea into a drag-drop dropzone, and re-voice the
ops-speak readiness page into user-facing capability statements.

**Surface:** `#/` (home), `#/verify`, `#/status` (was `#/readiness`).
**Backend changes:** none. **WoW:** ●●○ **Effort:** M (~3 days).
**Depends on:** RB-04 (tokens), RB-01 (matrix), RB-03 (spine, for ambient hero).

> **STATUS: DONE & VERIFIED.** See "As built" for what shipped and the full
> verification log (including a real-browser doors E2E via CDP).

---

## 1. Landing rebuild (`renderHome`)

Today: hero + three cards (two identical), generic copy, no spectacle (Findings 6).
New: lead with the spine, one primary CTA, one BYO door, a tertiary Tamper Lab link.

```
┌────────────────────────────────────────────────────────────────┐
│  ●───●───●───●───●───●───●   ← ambient spine (autoplay loop)     │
│                                                                  │
│  See a governed deal prove itself.                               │
│  Run it, then verify every cryptographic link in your own        │
│  browser. No install, no wallet, no trust in us.                 │
│                                                                  │
│  [ Run the escrow → ]   Already have a proof? Verify it →        │
│                         or break one in the Tamper Lab →         │
│                                                                  │
│  Mode: (•) Anonymous demo   ( ) Kermit sandbox                   │
└────────────────────────────────────────────────────────────────┘
```

Implementation notes:
- Ambient spine: `mountSpine` (from RB-03) in autoplay mode — loop the stages
  filling every ~2.5s. Gate the loop on `prefers-reduced-motion` (show a static
  completed spine instead).
- **One** primary CTA `Run the escrow →` → `startRun(selectedMode)`.
- **Delete the duplicate** "Watch a replay" card (it called the same `startRun`).
- Secondary text links: `Verify it →` (`#/verify`), `Tamper Lab →` (`#/lab`).
- Mode selector: keep the existing anonymous/kermit logic (`modeButton`), but render
  it compact and inline near the CTA — not a separate `<section>`.
- Move MetaMask link, readiness link, and the analytics opt-in into a slim footer row
  (`.pg-home-footer`) — they're not hero material.

## 2. Dropzone verify (`renderVerify`)

Today: bare `<textarea>` first (Finding 8). New: dropzone primary, paste secondary.

```
┌──────────────────────────────────────────────┐
│   ⬇  Drop a .infrix.json proof                │
│      or click to choose a file                │
│   ----------------------------------------    │
│   No login. No upload to us — verification     │
│   runs entirely on your machine.              │
│                                                │
│   [ Paste JSON instead ]   [ Load the sample ] │
└──────────────────────────────────────────────┘
```

Implementation:
```js
const dz = el('div', 'pg-dropzone');
dz.tabIndex = 0;
dz.setAttribute('role', 'button');
dz.setAttribute('aria-label', 'Drop or choose a proof file to verify');
dz.append(el('div', 'pg-dropzone-icon', '⬇'),
  el('div', 'pg-dropzone-title', 'Drop a .infrix.json proof'),
  el('div', 'pg-note', 'or click to choose · no login · verification runs on your machine'));

const file = document.createElement('input');
file.type = 'file'; file.accept = '.json,application/json'; file.hidden = true;

const handle = async (text) => {
  let pkg; try { pkg = JSON.parse(text); }
  catch { showError(out, { message: 'That is not valid JSON. Drop a portable proof bundle.' }); return; }
  out.replaceChildren(el('div', 'pg-loading', 'Verifying in your browser…'));
  const result = await verifyPortablePackage(pkg);
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  mountCheckMatrix(out, result, { stagger: reduce ? 0 : 80 });   // RB-01 component
};

dz.addEventListener('click', () => file.click());
dz.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); file.click(); } });
dz.addEventListener('dragover', (e) => { e.preventDefault(); dz.dataset.over = '1'; });
dz.addEventListener('dragleave', () => { delete dz.dataset.over; });
dz.addEventListener('drop', async (e) => {
  e.preventDefault(); delete dz.dataset.over;
  const f = e.dataTransfer.files[0]; if (f) handle(await f.text());
});
file.addEventListener('change', async () => { if (file.files[0]) handle(await file.files[0].text()); });
```
- Keep the textarea path behind a "Paste JSON instead" toggle (reuse existing handler).
- Keep "Load the sample" (`/api/sample-bundle`) — it now flows into the same matrix.

```css
.pg-dropzone { border: 2px dashed var(--border-bold); border-radius: var(--radius);
  padding: 40px 20px; text-align: center; cursor: pointer; background: var(--surface);
  transition: border-color var(--motion-fast), background var(--motion-fast); }
.pg-dropzone:hover, .pg-dropzone:focus-visible { border-color: var(--accent); }
.pg-dropzone[data-over="1"] { border-color: var(--accent); background: var(--accent-soft);
  box-shadow: var(--shadow-glow); }
.pg-dropzone-icon { font-size: 2rem; color: var(--accent); }
.pg-dropzone-title { font-weight: 700; font-size: 1.1rem; margin-top: 6px; }
@media (prefers-reduced-motion: reduce) { .pg-dropzone { transition: none; } }
```

## 3. Status re-voice (`renderReadiness` → `#/status`)

Today: `<dl>` of `ready/unavailable/never` ops-speak (Finding 11). Keep
`/api/readiness` unchanged; change only the render to capability statements with a
status dot. Frame "mainnet writes: never" as a **safety guarantee**, not a gap.

```js
const r = await api('/api/readiness');
const row = (on, title, desc) => {
  const d = el('div', 'pg-cap'); d.dataset.on = on ? '1' : '0';
  d.append(el('span', 'pg-cap-dot'), (() => { const b = el('div');
    b.append(el('div', 'pg-cap-title', title), el('div', 'pg-cap-desc', desc)); return b; })());
  return d;
};
panel.append(
  row(r.anonymous, 'Run a demo', 'Yes — right now, no wallet, no funding.'),
  row(r.kermit, 'Live testnet (Kermit)', r.kermit ? 'Enabled on this instance.' : 'Off on this instance.'),
  row(r.verifier, 'Verify your own proof', 'Yes — entirely in your browser.'),
  row(true, 'Mainnet writes', 'Never. This playground cannot touch real funds.'),  // safety, not gap
);
```
- Rename nav label "Readiness" → "What it can do" (`index.html` nav) and the route
  `#/readiness` → `#/status` (keep `#/readiness` as a redirect for old links).

```css
.pg-cap { display: flex; gap: 12px; align-items: flex-start; padding: 12px 0;
  border-bottom: 1px solid var(--border); }
.pg-cap-dot { width: 10px; height: 10px; border-radius: 50%; margin-top: 5px;
  background: var(--text-dim); }
.pg-cap[data-on="1"] .pg-cap-dot { background: var(--ok); box-shadow: 0 0 8px var(--ok); }
.pg-cap-title { font-weight: 700; }
.pg-cap-desc { color: var(--text-secondary); font-size: 0.88rem; }
```

## 4. Copy changes (apply the cheatsheet from `SPEC.md` §13)

| Where | To |
|---|---|
| Landing H1 | "See a governed deal prove itself." |
| Landing sub | "Run it, then verify every cryptographic link in your own browser. No install, no wallet, no trust in us." |
| Run H1 | "Building a tamper-evident proof" |
| Verify empty | "No login. No upload to us — verification runs entirely on your machine." |
| Status mainnet | "Mainnet writes — never. This playground can't touch real funds." |
| Footer | keep the honest "Fixture-backed & Kermit-backed demo…" line — it's good. |

## 5. Acceptance criteria

- [ ] Landing leads with the (ambient) spine + one primary CTA; the duplicate card is gone.
- [ ] Verify accepts drag-drop, file-pick, paste, and sample — all flow into the RB-01 matrix.
- [ ] Dropzone is keyboard-operable (Enter/Space opens picker) with visible focus.
- [ ] Status page reads in user voice; mainnet framed as a guarantee.
- [ ] `#/readiness` still resolves (redirect to `#/status`).
- [ ] Reduced-motion: static spine on landing, no dropzone transition.
- [ ] Responsive at 360px across all three surfaces.
- [ ] `node --test web/test/*.mjs` green.

## 6. Rollback

Front-end only. Revert `renderHome`, `renderVerify`, `renderReadiness`, the route
map, and `index.html` nav. Delete the added CSS blocks.

---

## As built (what shipped, and deviations)

Implemented in full — front-end only, no backend changes.

### Landing (`renderHome`)
- Leads with a **live ambient spine** (the RB-03 `mountSpine` + `stageColor`, 9
  nodes) driven by a new `runAmbientSpine(spine, keys, reduce)`: a quiet
  left-to-right fill on a loop that **self-halts when the spine leaves the DOM**
  (`spine.el.isConnected`) so no timers leak across navigations. Under reduced
  motion it shows a static completed flow (no loop).
- One primary CTA "Run the escrow →" (`startRun(selectedMode)`); the **duplicate
  "Watch a replay" card was deleted** (it called the same `startRun`). Quiet
  side-doors: "Verify it →" and "or break one in the Tamper Lab →".
- Mode selector is now **compact and inline** by the CTA (kept `modeButton`
  logic). MetaMask + "What it can do" links and the analytics opt-in moved into a
  slim `.pg-home-footer` row. Removed the now-dead `actionCard` helper and the
  `.pg-actions`/`.pg-modes`/`.pg-secondary` CSS.

### Verify (`renderVerify`)
- A **dropzone** is the primary affordance (drag-drop + click-to-pick), with a
  hidden `<input type=file accept=".json,application/json">`, keyboard-operable
  (Enter/Space, `role="button"`, `tabindex=0`, visible focus). "Paste JSON
  instead" toggles a textarea; "Load the sample" loads the fixture. **All four
  doors funnel into one `handle(text)` sink → the RB-01 matrix.** Empty-state copy
  reinforces "no upload to us — verification runs entirely on your machine."

### Status (`renderReadiness` → `renderStatus`, `#/status`)
- Re-voiced from `ready/unavailable/never` ops-speak into user-facing **capability
  rows** with a status dot; "Mainnet writes — never" framed as a **safety
  guarantee**. `/api/readiness` is unchanged (render only). Nav label →
  "What it can do"; route `#/readiness` **redirects** to `#/status`
  (`location.replace`, no history entry) so old links still resolve.

### Deviation
- The dropzone's actual **drop/file-pick** are not exercised in the headless E2E
  (triggering them opens a native OS file dialog that hangs headless). Instead the
  E2E proves the dropzone's attributes (role/tabindex/accept) and exercises the
  **same `handle()` sink** via "Load the sample" and the paste toggle — so the
  shared verification path is covered end to end.

## Verification log (all green)
- `grep -nE '#[0-9a-fA-F]{3,6}' web/playground.css` → **empty** (RB-04 token
  purity preserved).
- `node --test web/test/*.mjs` → **24 pass**; `go build/vet/test` → **all pass**.
- **Full doors browser E2E (Edge + CDP):**
  - **Landing:** H1 "See a governed deal prove itself."; 9-node ambient spine;
    **exactly one** "Run the escrow" CTA; "Watch a replay" **gone**; both doors
    present; 2 inline mode buttons; footer present.
  - **Verify:** dropzone present (`role=button`, `tabindex=0`, accept `.json,
    application/json`); "Paste JSON instead" reveals the textarea; "Load the
    sample" → matrix **`pass`, "10 / 10 … reconstructed locally"**.
  - **Status:** 5 capability rows, 4 lit dots, mainnet row reads "Never. This
    playground can’t touch real funds."
  - **Redirect:** `#/readiness` → `location.hash` becomes `#/status` and the
    capability rows render.
