# RB-01 — The 10-Check Verdict Matrix

**Goal:** make the browser-side cryptographic verification the hero of the product.
Render all ten checks the verifier already computes, eagerly, with plain-language
explanations and a "re-verify line by line" reveal.

**Surface:** `#/r/<id>` (hosted receipt) and `#/verify` (bring-your-own).
**Backend changes:** none. **WoW:** ●●● **Effort:** M (~3–4 days).
**Depends on:** RB-04 soft (works without it, looks better with it).

> **STATUS: DONE & VERIFIED.** See "As built" at the bottom for the (small) deviations
> from this spec that were required to make it actually work, and the verification log.

---

## 1. Why this first

The verifier (`/lib/portableVerifier.js`) already returns `{passed, checks[]}` with
ten richly-detailed checks. Today the UI discards `checks[]` and prints one word
(see `00-FINDINGS.md` Finding 1). This is the single highest-leverage change in the
product and is pure front-end.

## 2. Deliverables

1. `web/checkMatrix.js` — new reusable component (served at `/checkMatrix.js`; see "As built").
2. CSS for the matrix in `web/playground.css`.
3. `renderReceipt` (in `web/playground.js`) reworked to run the verifier eagerly and
   mount the matrix as the page hero; Cinema + receipt card demoted into a disclosure.
4. `renderVerify` reworked to mount the same matrix on success.
5. A copyable share control (fixes Finding 9).
6. A web smoke test `web/test/checkMatrix.test.mjs`.

## 3. Data contract (verbatim from the verifier)

```js
verifyPortablePackage(pkg) -> Promise<{
  passed: boolean,
  checks: Array<{ name: string, passed: boolean, detail?: string, error?: string }>
}>
```

The ten `name` values (stable, lifted from the verifier source):
`version`, `export_hash`, `bundle_data`, `plan_hash`, `outcome_digest`,
`inclusion_proofs`, `anchor_proof`, `trust_snapshot`, `policy_decision_digest`,
`plugin_versions`. (Inclusion proofs may also appear as `inclusion_proof[i]` on the
failing element — handle the `[` prefix in the title lookup, see §4.)

## 4. Component: `web/checkMatrix.js` (served at `/checkMatrix.js`)

Self-contained ES module, no new imports. Skeleton:

```js
// web/checkMatrix.js
// Renders the verifier's {passed, checks[]} result as the product's hero verdict.
// Each row pairs the verifier's own machine detail with a plain-language line that
// teaches what the check proves — the no-onboarding moment.

const TITLES = Object.freeze({
  version:                'Format understood',
  export_hash:            'Nothing altered after export',
  bundle_data:            'Evidence bundle well-formed',
  plan_hash:              'Approval bound to the exact plan',
  outcome_digest:         'Outcome matches what was committed',
  inclusion_proofs:       'Every step is in the tamper-evident chain',
  anchor_proof:           'On-ledger anchor matches this bundle',
  trust_snapshot:         'Trust profile captured at a real block',
  policy_decision_digest: 'Policy decisions exactly as recorded',
  plugin_versions:        'Every plugin identified by id, version, code hash',
});

const EXPLAIN = Object.freeze({
  version:                'The proof uses a format this verifier understands.',
  export_hash:            'Nothing in the file was altered after it was exported.',
  bundle_data:            'The evidence bundle is structurally valid.',
  plan_hash:              'The approval is bound to the exact plan that ran — not a substituted one.',
  outcome_digest:         'The recorded outcome matches what the bundle commits to.',
  inclusion_proofs:       'Every step is provably part of one tamper-evident chain.',
  anchor_proof:           'If anchored, the on-ledger anchor matches this bundle exactly.',
  trust_snapshot:         'The trust profile in force was captured at a real block height.',
  policy_decision_digest: 'The policy decisions are exactly those recorded — none added or removed.',
  plugin_versions:        'Every plugin that touched this is identified by id, version, and code hash.',
});

// Normalize "inclusion_proof[3]" -> "inclusion_proofs" for title/explain lookup.
function baseName(name) {
  const i = name.indexOf('[');
  if (i === -1) return name;
  const root = name.slice(0, i);
  return root === 'inclusion_proof' ? 'inclusion_proofs'
    : root === 'trust_snapshot' ? 'trust_snapshot'
    : root === 'plugin_versions' ? 'plugin_versions'
    : root;
}

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = String(text);
  return n;
}

/**
 * mountCheckMatrix(container, result, opts)
 * @param result  the {passed, checks[]} from verifyPortablePackage
 * @param opts.stagger  ms between rows turning green (0 = instant). Gated by caller
 *                      on prefers-reduced-motion.
 * @param opts.assuranceLabel  e.g. "L3 · G2" for the headline chip (optional)
 * @returns {{ el: HTMLElement, replay: () => void }}
 */
export function mountCheckMatrix(container, result, opts = {}) {
  const { stagger = 0, assuranceLabel = '' } = opts;
  const checks = (result && result.checks) || [];
  const passedCount = checks.filter((c) => c.passed).length;
  const total = checks.length;
  const allPass = !!(result && result.passed);

  const wrap = el('section', 'pg-matrix');
  wrap.dataset.state = allPass ? 'pass' : 'fail';
  wrap.setAttribute('role', 'group');
  wrap.setAttribute('aria-label', allPass ? 'Proof holds' : 'Proof did not verify');

  // Headline.
  const head = el('header', 'pg-matrix-head');
  head.appendChild(el('p', 'pg-eyebrow', 'VERIFIED IN YOUR BROWSER · THIS SERVER WAS NOT TRUSTED'));
  const verdict = el('div', 'pg-matrix-verdict');
  verdict.appendChild(el('span', 'pg-matrix-glyph', allPass ? '✓' : '✗'));
  verdict.appendChild(el('span', 'pg-matrix-title', allPass ? 'Proof holds' : 'Proof did not verify'));
  if (assuranceLabel) verdict.appendChild(el('span', 'pg-matrix-assurance', assuranceLabel));
  head.appendChild(verdict);
  head.appendChild(el('p', 'pg-matrix-count', `${passedCount} / ${total} cryptographic checks reconstructed locally`));
  wrap.appendChild(head);

  // Rows.
  const list = el('ol', 'pg-matrix-list');
  const rows = [];
  checks.forEach((c, i) => {
    const base = baseName(c.name);
    const li = el('li', 'pg-matrix-row');
    li.dataset.pass = String(c.passed);
    if (!c.passed) li.dataset.fail = '1';
    li.appendChild(el('span', 'pg-matrix-rowglyph', c.passed ? '✓' : '✗'));
    li.appendChild(el('span', 'pg-matrix-idx', String(i + 1)));
    const body = el('div', 'pg-matrix-body');
    body.appendChild(el('span', 'pg-matrix-name', TITLES[base] || c.name));
    body.appendChild(el('span', 'pg-matrix-explain', EXPLAIN[base] || ''));
    const detail = c.error || c.detail;
    if (detail) body.appendChild(el('code', 'pg-matrix-detail', detail));
    li.appendChild(body);
    list.appendChild(li);
    rows.push(li);
  });
  wrap.appendChild(list);

  if (container) container.replaceChildren(wrap);

  // Stagger reveal (theater over real, already-computed results).
  function reveal() {
    if (!stagger) { rows.forEach((r) => (r.dataset.revealed = '1')); return; }
    rows.forEach((r) => (r.dataset.revealed = '0'));
    rows.forEach((r, i) => setTimeout(() => (r.dataset.revealed = '1'), i * stagger));
  }
  reveal();

  return { el: wrap, replay: reveal };
}
```

**Failing-check behavior:** rows with `data-fail` get a left accent bar in
`--alert` and stay fully visible; if `result.passed === false`, scroll the first
failing row into view and add a `pg-matrix-failnote` line: "Here's exactly what
didn't add up."

## 5. CSS (`web/playground.css`)

Use Spine Aurora tokens (no GitHub-blue fallbacks). Key rules:

```css
.pg-matrix { border: 1px solid var(--border); border-radius: var(--radius);
  background: var(--surface); padding: 22px; }
.pg-matrix[data-state="pass"] { box-shadow: var(--shadow-glow); }
.pg-eyebrow { font-size: 0.72rem; font-weight: 700; letter-spacing: 0.12em;
  text-transform: uppercase; color: var(--text-dim); margin: 0 0 10px; }
.pg-matrix-verdict { display: flex; align-items: baseline; gap: 12px; }
.pg-matrix-glyph { font-size: 2rem; }
.pg-matrix[data-state="pass"] .pg-matrix-glyph { color: var(--ok); }
.pg-matrix[data-state="fail"] .pg-matrix-glyph { color: var(--alert); }
.pg-matrix-title { font-size: clamp(1.5rem, 3vw, 2.2rem); font-weight: 800;
  letter-spacing: -0.02em; }
.pg-matrix-assurance { margin-left: auto; font: 600 0.8rem var(--mono);
  border: 1px solid var(--border-bold); border-radius: 999px; padding: 2px 10px;
  color: var(--text-dim); }
.pg-matrix-count { color: var(--text-dim); font-size: 0.9rem; margin: 4px 0 18px; }

.pg-matrix-list { list-style: none; margin: 0; padding: 0;
  display: flex; flex-direction: column; gap: 2px; }
.pg-matrix-row { display: grid; grid-template-columns: 22px 22px 1fr; gap: 10px;
  align-items: start; padding: 10px 8px; border-radius: var(--radius-sm);
  opacity: 0; transform: translateY(4px);
  transition: opacity var(--motion-fast), transform var(--motion-fast); }
.pg-matrix-row[data-revealed="1"] { opacity: 1; transform: none; }
.pg-matrix-row[data-fail] { background: var(--alert-soft);
  box-shadow: inset 3px 0 0 var(--alert); }
.pg-matrix-rowglyph { font-weight: 800; }
.pg-matrix-row[data-pass="true"]  .pg-matrix-rowglyph { color: var(--ok); }
.pg-matrix-row[data-pass="false"] .pg-matrix-rowglyph { color: var(--alert); }
.pg-matrix-idx { color: var(--text-dim); font: 500 0.8rem var(--mono); }
.pg-matrix-body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.pg-matrix-name { font-weight: 700; font-size: 0.95rem; }
.pg-matrix-explain { color: var(--text-secondary); font-size: 0.85rem; }
.pg-matrix-detail { color: var(--text-dim); font: 500 0.75rem var(--mono);
  overflow-wrap: anywhere; }

@media (prefers-reduced-motion: reduce) {
  .pg-matrix-row { transition: none; opacity: 1; transform: none; }
}
@media (max-width: 560px) {
  .pg-matrix-row { grid-template-columns: 20px 1fr; }
  .pg-matrix-idx { display: none; }
}
```

## 6. Rework `renderReceipt` (`web/playground.js`)

1. Import the new component:
   ```js
   import { mountCheckMatrix } from '/checkMatrix.js';
   ```
2. After `pkgPromise` resolves (it already exists, `playground.js:318`), **run the
   verifier eagerly** instead of waiting for the button:
   ```js
   const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
   const matrixHost = el('div'); left.prepend(matrixHost); // matrix is the hero
   const pkg = await pkgPromise;
   if (pkg) {
     const result = await verifyPortablePackage(pkg);
     const label = (data.receipt && data.receipt.assurance)
       ? [data.receipt.assurance.proofLevel, data.receipt.assurance.governanceLevel].filter(Boolean).join(' · ')
       : '';
     const m = mountCheckMatrix(matrixHost, result, { stagger: reduce ? 0 : 80, assuranceLabel: label });
     telemetry.emit('proof.verified', { result: result.passed ? 'success' : 'failure' });
     // Re-verify button now replays the staggered reveal of the real results.
     verifyBtn.textContent = 'Re-verify line by line';
     verifyBtn.addEventListener('click', () => m.replay());
   }
   ```
3. **Demote** the existing `mountProofReceipt` card and the Cinema panel into a
   collapsed `<details class="pg-disclosure"><summary>Inspect the full proof</summary>…`
   block below the matrix. (Keep both — just not competing with the verdict.)
4. **Fix the share control** (Finding 9): replace the plain-text share line with:
   ```js
   const copyBtn = el('button', 'pg-btn', 'Copy share link');
   copyBtn.addEventListener('click', async () => {
     await navigator.clipboard.writeText(location.origin + '/#/r/' + id);
     copyBtn.textContent = 'Copied ✓';
     setTimeout(() => (copyBtn.textContent = 'Copy share link'), 1500);
   });
   ```

## 7. Rework `renderVerify` (`web/playground.js`)

In the existing `verifyBtn` click handler (`playground.js:397`), replace the
boolean-only rendering with the matrix:

```js
const result = await verifyPortablePackage(pkg);
const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
mountCheckMatrix(out, result, { stagger: reduce ? 0 : 80 });
```

(The dropzone itself is RB-05; this runbook just swaps the result rendering.)

## 8. Tests

`web/test/checkMatrix.test.mjs` (Node `--test`, jsdom-free DOM shim or the existing
test harness pattern in `web/test/`):

- Given a 10/10 pass result → 10 rows, `data-state="pass"`, count text "10 / 10".
- Given a result with one failing `plan_hash` → that row has `data-fail`, glyph ✗,
  shows the `detail` string.
- `inclusion_proof[3]` name → row title resolves to "Every step is in the
  tamper-evident chain" (baseName normalization).
- `replay()` re-applies `data-revealed` to all rows.

Run: `node --test web/test/*.mjs`.

## 9. Acceptance criteria

- [ ] Entering `#/r/<id>` runs the verifier and renders all 10 checks within ~300ms,
      **no click required**.
- [ ] Each row shows the verifier's real `detail`/`error` plus a plain-language line.
- [ ] A failing bundle makes the failing check unmissable and scrolls it into view.
- [ ] "Re-verify line by line" replays the staggered reveal.
- [ ] Cinema + receipt card still reachable in the disclosure.
- [ ] Share link is one-click copyable with confirm.
- [ ] Reduced-motion: rows appear instantly, no stagger.
- [ ] Responsive at 360px (index column hidden, rows stack).
- [ ] `node --test web/test/*.mjs` and `go test ./... -count=1` green.

## 10. Rollback

The component is additive. Revert `renderReceipt`/`renderVerify` diffs and delete
`checkMatrix.js`; the prior boolean rendering returns. No data or API changes to undo.

---

## As built (deviations from the spec above, and why)

The runbook's intent was implemented in full — nothing demoted, deferred, or stubbed.
A handful of details changed during implementation because the spec was written
before the serving/routing reality was confirmed:

1. **Path: `/checkMatrix.js`, not `/components/checkMatrix.js`.** The `/components/`
   URL prefix is routed to the shared Nexus asset handler (`api/server.go:102` →
   `handleShared`), which only knows the `infrix-nexus-web` asset tree — a
   playground-owned file there would 404. Playground modules are served from the
   root and embedded individually in `web/web.go` (exactly like `bundleScene.js`).
   So the component lives at `web/checkMatrix.js`, is embedded + registered in
   `web/web.go` (`/checkMatrix.js`), and imported as `from '/checkMatrix.js'`.
2. **`web/web.go`** gained a `//go:embed checkMatrix.js` + an `Assets()` entry.
3. **`api/server_test.go`** — extended `TestSPAandSharedAssets` to require
   `/checkMatrix.js` (200, `application/javascript`, body contains `mountCheckMatrix`).
   This pins the embed+route wiring so the integration can't silently regress.
4. **`web/package.json`** test script broadened to `test/*.test.mjs` so all web
   smokes run locally (CI already globbed `web/test/*.mjs`).
5. **Refinements beyond the skeleton** (all in the spirit of "do it perfect"):
   - The component leads the eye to the first failing check via guarded
     `scrollIntoView` and appends the `pg-matrix-failnote` line on failure.
   - `baseName` is exported so the indexed→base mapping is unit-pinned.
   - In `renderReceipt`: the "Re-verify line by line" button is **disabled until the
     matrix exists** (no click can race the async verify); the share **Copy** button
     falls back to revealing the URL when the Clipboard API is unavailable; the
     Cinema replay is **lazy-mounted on first disclosure open** (a collapsed
     `<details>` has no size, which would otherwise give Cinema a zero-size canvas).
   - The matrix CSS keeps defensive token fallbacks so it stays legible even if the
     shared `/styles.css` fails to load; when present, the Spine Aurora tokens win.
   - Removed the now-orphaned `buildReceiptFromVerifier` import and `.pg-share` rule.

## Verification log (all green)

- `node --test web/test/*.mjs` → **10 pass** (4 bundleScene + 6 checkMatrix).
- `go build ./... && go vet ./... && go test ./... -count=1` → **all pass**,
  including the extended `TestSPAandSharedAssets`.
- **Real-verifier E2E:** the published `portableVerifier.js` run against the real
  `fixtures/sample-proof.infrix.json`, piped into the real `checkMatrix.js`, yields
  `passed=true`, 10 checks, 10 rendered rows, `state=pass`.
- **Live binary:** `/checkMatrix.js` serves 200 (`application/javascript`, body has
  `mountCheckMatrix`); `/playground.js` imports `/checkMatrix.js`; the full import
  graph (`/lib/portableVerifier.js`, `/lib/canonicalJson.js`,
  `/components/proofReceiptView.js`, `/cinema-core/loader.js`, `/styles.css`) and
  `/api/sample-bundle` (valid v4) all serve 200.
