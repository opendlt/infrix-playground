// Infrix Playground — the verdict matrix (RB-01).
//
// Renders the browser-side verifier's {passed, checks[]} result as the product's
// hero: all ten cryptographic checks, each pairing the verifier's own machine
// `detail` with a plain-language line that teaches what the check proves. This is
// the "trust microscope" — the visitor watches the maths reconstruct locally,
// with no node trusted.
//
// Served as a top-level playground asset at /checkMatrix.js (embedded in web.go),
// the same way /bundleScene.js is. (The /components/ URL prefix is owned by the
// shared Nexus asset handler, so playground-owned modules live at the root.)

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

// Normalize an indexed check name ("inclusion_proof[3]", "trust_snapshot[0]",
// "plugin_versions[1]") to its base key so TITLES/EXPLAIN resolve. The verifier
// reports the collection name on success and an indexed element name on the
// failing entry, so both forms must map to the same human copy.
function baseName(name) {
  const i = name.indexOf('[');
  if (i === -1) return name;
  const root = name.slice(0, i);
  if (root === 'inclusion_proof' || root === 'inclusion_proofs') return 'inclusion_proofs';
  if (root === 'trust_snapshot') return 'trust_snapshot';
  if (root === 'plugin_versions') return 'plugin_versions';
  return root;
}

function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = String(text);
  return n;
}

/**
 * mountCheckMatrix(container, result, opts) renders the verdict and returns a
 * handle. Pure DOM — environment-agnostic except for `document` and `setTimeout`.
 *
 * @param {HTMLElement|null} container  replaced with the matrix (null = build only)
 * @param {{passed:boolean, checks:Array<{name:string,passed:boolean,detail?:string,error?:string}>}} result
 * @param {{stagger?:number, assuranceLabel?:string}} [opts]
 *   stagger: ms between rows revealing (0 = instant). The caller gates this on
 *            prefers-reduced-motion (pass 0 when reduced motion is requested).
 *   assuranceLabel: optional chip text, e.g. "L3 · G2".
 * @returns {{ el: HTMLElement, replay: () => void }}
 */
export function mountCheckMatrix(container, result, opts = {}) {
  const stagger = Number(opts.stagger) || 0;
  const assuranceLabel = opts.assuranceLabel || '';
  const checks = (result && Array.isArray(result.checks)) ? result.checks : [];
  const passedCount = checks.filter((c) => c && c.passed).length;
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
  head.appendChild(el('p', 'pg-matrix-count',
    `${passedCount} / ${total} cryptographic check${total === 1 ? '' : 's'} reconstructed locally`));
  wrap.appendChild(head);

  // Rows.
  const list = el('ol', 'pg-matrix-list');
  const rows = [];
  let firstFail = null;
  checks.forEach((c, i) => {
    const base = baseName(c.name || '');
    const li = el('li', 'pg-matrix-row');
    li.dataset.pass = String(!!c.passed);
    if (!c.passed) { li.dataset.fail = '1'; if (!firstFail) firstFail = li; }
    li.appendChild(el('span', 'pg-matrix-rowglyph', c.passed ? '✓' : '✗'));
    li.appendChild(el('span', 'pg-matrix-idx', String(i + 1)));
    const body = el('div', 'pg-matrix-body');
    body.appendChild(el('span', 'pg-matrix-name', TITLES[base] || c.name || 'check'));
    const explain = EXPLAIN[base];
    if (explain) body.appendChild(el('span', 'pg-matrix-explain', explain));
    const detail = c.error || c.detail;
    if (detail) body.appendChild(el('code', 'pg-matrix-detail', detail));
    li.appendChild(body);
    list.appendChild(li);
    rows.push(li);
  });
  wrap.appendChild(list);

  // On failure, name the moment and lead the eye to the broken link.
  if (!allPass) {
    wrap.appendChild(el('div', 'pg-matrix-failnote', "Here's exactly what didn't add up."));
  }

  if (container) container.replaceChildren(wrap);

  // Reveal the rows. With stagger they turn on one at a time so a skeptic
  // literally watches each check land; the checks themselves already ran.
  function reveal() {
    if (stagger <= 0) {
      rows.forEach((r) => { r.dataset.revealed = '1'; });
      return;
    }
    rows.forEach((r) => { r.dataset.revealed = '0'; });
    rows.forEach((r, i) => { setTimeout(() => { r.dataset.revealed = '1'; }, i * stagger); });
  }
  reveal();

  // Lead the eye to the first failing check (after layout settles).
  if (firstFail && typeof firstFail.scrollIntoView === 'function') {
    setTimeout(() => {
      try { firstFail.scrollIntoView({ block: 'nearest' }); } catch { /* non-fatal */ }
    }, 0);
  }

  return { el: wrap, replay: reveal };
}

// Exported for tests so the indexed→base mapping is pinned behavior.
export { baseName };
