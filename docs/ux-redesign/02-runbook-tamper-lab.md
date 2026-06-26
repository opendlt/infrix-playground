# RB-02 — The Tamper Lab

**Goal:** the killer differentiator. Let a visitor *try to forge a proof* and watch
the math catch them, in their own browser. The screenshot-worthy, shareable feature
no other blockchain playground has.

**Surface:** new route `#/lab`. **Backend changes:** none. **WoW:** ●●● **Effort:** M
(~3–4 days). **Depends on:** RB-01 (reuses `checkMatrix.js`).

> **STATUS: DONE & VERIFIED.** See "As built" at the bottom for the (important)
> cryptographic correction to the forgery design and the full verification log.

---

## 1. The idea

Load the sample bundle into a structured, editable view. The user flips one field —
forge the approver, inflate the amount, swap a hash, drop a policy decision — hits
**Re-verify**, and the exact check goes red with the exact reason. Falsification is
the fastest path to belief: you trust tamper-evidence once you fail to beat it.

It runs entirely through the existing client verifier (`verifyPortablePackage`). Zero
backend. Inherently viral: *"I tried to forge an Infrix proof and it caught me in the
browser."*

## 2. Deliverables

1. New route `#/lab` wired in `route()` (`web/playground.js`).
2. `renderLab()` view in `web/playground.js`.
3. ~~`web/components/bundleEditor.js`~~ — free-form editor: explicit optional
   Phase 2.5 (see §4 note and "As built"). Shipped: preset buttons + a read-only
   proof readout.
4. `web/tamper.js` (served at `/tamper.js`) — the forgery engine. **NOTE:** the §3
   skeleton below is superseded by the as-built two-tier (naive + re-sealed) design;
   see "As built" for why and the verified `trips` map.
5. CSS for the lab split layout.
6. Tests: `web/test/tamper.test.mjs`.
7. Entry points: link from landing (RB-05) and from the Verdict page (RB-01).

## 3. The forgery functions: `web/lib/tamper.js`

Pure functions, each takes a **deep clone** of the parsed package and returns it
mutated. Each is designed to trip a *specific* verifier check so the demo is
deterministic and teachable.

```js
// web/lib/tamper.js
// Each forgery mutates a CLONED PortableEvidencePackage to trip one verifier check.
// The comment names the check it defeats (and proves the verifier catches).

const clone = (pkg) => JSON.parse(JSON.stringify(pkg));

// Helper: parse bundleData whether it's an inlined object or a JSON string.
function withBundle(pkg, fn) {
  let bd = pkg.bundleData;
  const wasString = typeof bd === 'string';
  if (wasString) bd = JSON.parse(bd);
  fn(bd);
  pkg.bundleData = wasString ? JSON.stringify(bd) : bd;
  return pkg;
}

export const FORGERIES = [
  {
    id: 'approver',
    label: 'Forge the approver',
    blurb: 'Swap the approving identity to an attacker.',
    trips: 'export_hash',  // bundleData is committed to ExportHash
    apply: (pkg) => withBundle(clone(pkg), (bd) => {
      const a = (bd.approvalEvidence || bd.ApprovalEvidence || [])[0];
      if (a) a.identity = 'acc://attacker.evil/admin';
    }),
  },
  {
    id: 'planhash',
    label: 'Break plan ↔ approval',
    blurb: 'Point the package PlanHash at a plan nobody approved.',
    trips: 'plan_hash',
    apply: (pkg) => { const p = clone(pkg);
      if (Array.isArray(p.planHash)) p.planHash = p.planHash.map((b, i) => i === 0 ? (b ^ 0xff) : b);
      return p; },
  },
  {
    id: 'outcome',
    label: 'Rewrite the outcome',
    blurb: 'Change the committed outcome digest.',
    trips: 'outcome_digest',
    apply: (pkg) => { const p = clone(pkg);
      if (Array.isArray(p.outcomeDigest)) p.outcomeDigest = p.outcomeDigest.map((b, i) => i === 0 ? (b ^ 0xff) : b);
      return p; },
  },
  {
    id: 'chainhash',
    label: 'Swap a chain hash',
    blurb: 'Tamper with one Merkle inclusion proof.',
    trips: 'inclusion_proof[0]',
    apply: (pkg) => { const p = clone(pkg);
      const pr = (p.inclusionProofs || [])[0];
      if (pr && pr.link && Array.isArray(pr.link.contentHash))
        pr.link.contentHash = pr.link.contentHash.map((b, i) => i === 0 ? (b ^ 0xff) : b);
      return p; },
  },
  {
    id: 'policy',
    label: 'Drop a policy decision',
    blurb: 'Delete a recorded policy decision.',
    trips: 'policy_decision_digest',
    apply: (pkg) => withBundle(clone(pkg), (bd) => {
      if (Array.isArray(bd.policyDecisions) && bd.policyDecisions.length)
        bd.policyDecisions = bd.policyDecisions.slice(1);
    }),
  },
  {
    id: 'plugin',
    label: 'Hide a plugin version',
    blurb: 'Blank a plugin implementation hash.',
    trips: 'plugin_versions[0]',
    apply: (pkg) => { const p = clone(pkg);
      const pv = (p.pluginVersions || [])[0];
      if (pv) pv.implementationHash = '';
      return p; },
  },
];

export function applyForgery(pkg, id) {
  const f = FORGERIES.find((x) => x.id === id);
  if (!f) throw new Error('unknown forgery ' + id);
  return { pkg: f.apply(pkg), forgery: f };
}
```

> **Validation step during build:** run each forgery against the sample and confirm
> the verifier fails on the `trips` check (and *only* expected checks). If a forgery
> trips `export_hash` first (because the field is committed to ExportHash) instead of
> its semantic check, that's fine — note it in the UI ("the export hash already caught
> this") because it's *more* impressive, not less. Document the actual tripped check
> in `tamper.test.mjs` so behavior is pinned.

## 4. View: `renderLab()` (`web/playground.js`)

Layout — editable bundle (left) + live verdict (right):

```
┌── FORGE A PROOF ───────────────────────┬── LIVE VERDICT ───────────────┐
│  Try to defeat the proof. Pick a tamper │  ✓ 10 / 10 checks pass        │
│  or edit a field, then re-verify.       │  (the RB-01 check matrix)     │
│                                         │                               │
│  [Forge the approver] [Break plan↔appr] │  after a forgery:             │
│  [Rewrite the outcome] [Swap chain hash]│  ✗ 6 inclusion proof[0]       │
│  [Drop a policy decision] [Hide plugin] │    "reconstructed root ≠ …"   │
│                                         │                               │
│  Tamper score: caught 3 / 3 forgeries   │                               │
│  [ Reset to a clean proof ]             │                               │
└─────────────────────────────────────────┴───────────────────────────────┘
```

Implementation:

```js
// As-built imports (paths corrected; see "As built"):
//   import { mountCheckMatrix } from '/checkMatrix.js';
//   import { FORGERIES, applyForgery, makeReseal } from '/tamper.js';
//   import { canonicalJSONSha256, coerce32 } from '/lib/canonicalJson.js';
import { mountCheckMatrix } from '/checkMatrix.js';
import { FORGERIES, applyForgery } from '/tamper.js';

async function renderLab() {
  setNav('home');
  app.replaceChildren();
  // header (eyebrow + h1 "Forge a proof" + intro)

  const split = el('div', 'pg-lab-split');
  const left = el('div', 'pg-panel');
  const right = el('div', 'pg-panel');
  split.append(left, right);
  app.appendChild(split);

  // Load a clean sample (already an endpoint).
  let clean;
  try { clean = await (await fetch('/api/sample-bundle')).json(); }
  catch (e) { showError(left, e); return; }

  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  let attempts = 0, caught = 0;
  const score = el('div', 'pg-lab-score');

  const verdictHost = el('div'); right.append(el('h2', null, 'Live verdict'), verdictHost);
  const verify = async (pkg) => {
    const result = await verifyPortablePackage(pkg);
    mountCheckMatrix(verdictHost, result, { stagger: reduce ? 0 : 60 });
    return result;
  };
  await verify(clean);  // start clean: 10/10

  const btnRow = el('div', 'pg-lab-forgeries');
  for (const f of FORGERIES) {
    const b = el('button', 'pg-btn', f.label);
    b.title = f.blurb;
    b.addEventListener('click', async () => {
      const { pkg } = applyForgery(clean, f.id);
      attempts++;
      const result = await verify(pkg);
      if (!result.passed) caught++;
      score.textContent = `Tamper score: the proof caught ${caught} / ${attempts} forgeries.`;
      telemetry.emit('tamper.attempt', { result: result.passed ? 'evaded' : 'caught' });
    });
    btnRow.appendChild(b);
  }
  const reset = el('button', 'pg-btn pg-btn-primary', 'Reset to a clean proof');
  reset.addEventListener('click', async () => { await verify(clean); });

  left.append(el('h2', null, 'Forge a proof'),
    el('p', 'pg-note', 'Pick a tamper. The proof is re-verified in your browser — the same maths the CLI runs.'),
    btnRow, score, reset);
}
```

Wire the route in `route()`:
```js
if (hash === '/lab') return renderLab();
```

> **Optional Phase 2.5:** add free-form structured editing (`bundleEditor.js`) where
> users click any field and type a new value. The preset buttons give the aha in one
> click (frictionless, no onboarding); free editing is for power users. Ship presets
> first.

## 5. CSS (`web/playground.css`)

```css
.pg-lab-split { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; align-items: start; }
@media (max-width: 820px) { .pg-lab-split { grid-template-columns: 1fr; } }
.pg-lab-forgeries { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
.pg-lab-score { color: var(--text-dim); font-size: 0.85rem; margin: 10px 0; min-height: 1.2em; }
```

(Reuses `.pg-btn`, `.pg-panel`, and the RB-01 matrix styles.)

## 6. Tests `web/test/tamper.test.mjs`

For each forgery: `applyForgery(sample, id)` then assert
`verifyPortablePackage(result).passed === false` and that the first failing check's
`name` matches the documented tripped check. Assert the clean sample still passes
10/10 (guards against the clone leaking mutations).

## 7. Acceptance criteria

- [ ] `#/lab` loads a clean sample and shows 10/10.
- [ ] Each forgery button produces a **specific** failing check with the verifier's
      real reason; the clean sample is never mutated (deep clone verified).
- [ ] Tamper score increments correctly.
- [ ] Reset returns to 10/10.
- [ ] Works after first load with no further network (sample already fetched).
- [ ] Reduced-motion respected; responsive at 360px (panels stack).
- [ ] `node --test web/test/*.mjs` green.

## 8. Rollback

Fully additive: remove the `#/lab` route line, `renderLab`, `tamper.js`,
`bundleEditor.js`, and the lab CSS. Nothing else depends on it.

---

## As built (the important correction, and what shipped)

The runbook's skeleton had a cryptographic flaw that would have made the lab weak and
its `trips` labels wrong. It was corrected; the result is **stronger** than the spec.

### The correction: ExportHash seals (almost) everything

The verifier checks run in a fixed order and **return on the first failure**, and
`export_hash` (check #2) recomputes a hash over nearly the whole package —
`bundleData`, `planHash`, `outcomeDigest`, `trustSnapshot`, `inclusionProofs`,
`anchorProof`, `pluginVersions`, `policyDecisionDigest`, … So a *naive* field edit
(e.g. the runbook's `planhash` → `trips: 'plan_hash'`) actually trips **`export_hash`
first** — every naive forgery would show the same failure, and the per-forgery
teaching would collapse. The runbook's `trips` values were mostly wrong.

**The fix — two tiers of forgery:**
- **Naive** (1 forgery, "Rename the approver"): edit a field, don't re-seal → caught
  by `export_hash`. Demonstrates the outer seal.
- **Sophisticated** (6 forgeries): a real forger **re-seals the ExportHash** after
  editing, sailing past check #2 — and is then caught by the **inner** binding. Each
  trips a distinct inner check: `plan_hash`, `outcome_digest`, `inclusion_proof[0]`,
  `policy_decision_digest`, `trust_snapshot[0]`, `plugin_versions[0]`.

Re-sealing must recompute the ExportHash **byte-identically** to the verifier. Rather
than re-implement the canonical JSON encoder, `tamper.js` takes the shared hasher as
an **injected dependency** (`makeReseal({ canonicalJSONSha256, coerce32 })` from
`/lib/canonicalJson.js` — the SAME primitive the verifier uses). This guarantees
compatibility *and* keeps `tamper.js` free of any `/lib` import, so it unit-tests in
plain Node. Verified empirically: all six re-sealed forgeries pass `export_hash` and
are caught only at their inner check. **Nothing evades.**

### Deviations from the spec text
1. **Paths:** `web/tamper.js` served at `/tamper.js` (not `/lib/tamper.js`) and the
   matrix imported from `/checkMatrix.js` (not `/components/…`) — both prefixes are
   owned by the shared Nexus handler. Embedded + registered in `web/web.go`.
2. **Telemetry:** the onboarding-metrics event enum is fixed and server-validated;
   `tamper.attempt` is not a member and would 400. The lab emits the schema-valid
   `proof.verified` (each forgery genuinely re-verifies a proof) with `success`/`failure`.
3. **Forgery count:** 7 (the runbook's 6 ids + `trust`) for fuller check coverage.
4. **`bundleEditor.js` (free-form editing):** the runbook explicitly scopes this as
   "Optional Phase 2.5 — ship presets first." Shipped instead: the preset buttons
   (the frictionless, no-onboarding core) **plus** a read-only "The proof" field
   readout (approver, plan hash, outcome digest, policy, plugin, inclusion-proof
   count) so the tamper is concrete. Free-form JSON editing remains the documented
   optional follow-on; it would only let users build undefined states with worse UX.
5. **Entry points:** header nav ("Tamper Lab"), a home secondary link, and a "Try to
   forge it →" action on the Verdict page.
6. **Polish:** buttons disable during an in-flight verify; an honest "EVADED … please
   report it" message if any forgery ever slips through (it never does).

## Verification log (all green)

- `node --test web/test/*.mjs` → **17 pass** (4 bundleScene + 6 checkMatrix + 7 tamper).
  The tamper unit tests pin the mutations, clone-safety (clean pkg never mutated),
  the re-seal injection contract, and the forgery set / `trips` map.
- `go build ./... && go vet ./... && go test ./... -count=1` → **all pass**, including
  `TestSPAandSharedAssets` extended to require `/tamper.js` and `/lib/canonicalJson.js`.
- **Real-verifier E2E:** the published `portableVerifier.js` + real `canonicalJson.js`
  run against the real `fixtures/sample-proof.infrix.json`. Clean → 10/10; every one of
  the 7 forgeries → caught at exactly its `trips` check with the verifier's real reason;
  the clean package is never mutated.
- **Headless browser (Edge):** `#/lab` renders the header, all forgery buttons, the
  read-only proof readout (real fixture approver), and the eager baseline verdict
  ("Proof holds", 10 checks).
- **Real click (CDP):** clicking "Break plan ↔ approval" flips the verdict
  `pass → fail` at "Approval bound to the exact plan" with detail "package PlanHash …
  not in 1 approval entry" and score "Caught 1 / 1".
