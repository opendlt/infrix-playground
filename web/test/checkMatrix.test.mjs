// RB-01 — verdict matrix smokes.
//
// The verifier already computes {passed, checks[]}; this asserts the matrix
// renders every check (not just the boolean), maps indexed check names to their
// human title, surfaces the verifier's detail on failures, and that replay()
// re-reveals the rows. Uses a tiny self-contained DOM shim so the test stays
// dependency-free, matching the repo's jsdom-free convention.

import { test } from 'node:test';
import assert from 'node:assert/strict';

// ---- minimal DOM shim (only what checkMatrix.js touches) ----
class El {
  constructor(tag) {
    this.tagName = String(tag).toUpperCase();
    this.className = '';
    this._text = null;
    this.children = [];
    this.dataset = {};
    this.attributes = {};
    this.parent = null;
  }
  set textContent(v) { this._text = v == null ? '' : String(v); this.children = []; }
  get textContent() {
    if (this._text != null) return this._text;
    return this.children.map((c) => c.textContent).join('');
  }
  appendChild(c) { c.parent = this; this.children.push(c); return c; }
  replaceChildren(...nodes) { this.children = []; for (const n of nodes) this.appendChild(n); }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  getAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attributes, k) ? this.attributes[k] : null; }
  scrollIntoView() { /* no-op in the shim */ }
  _matches(sel) {
    if (sel.startsWith('.')) return this.className.split(/\s+/).includes(sel.slice(1));
    return this.tagName === sel.toUpperCase();
  }
  querySelectorAll(sel) {
    const out = [];
    const walk = (n) => { for (const c of n.children) { if (c._matches(sel)) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
}
globalThis.document = { createElement: (tag) => new El(tag) };

const { mountCheckMatrix, baseName } = await import('../checkMatrix.js');

function passResult(n = 10) {
  const names = ['version', 'export_hash', 'bundle_data', 'plan_hash', 'outcome_digest',
    'inclusion_proofs', 'anchor_proof', 'trust_snapshot', 'policy_decision_digest', 'plugin_versions'];
  return { passed: true, checks: names.slice(0, n).map((name) => ({ name, passed: true, detail: 'ok' })) };
}

test('10/10 pass renders all rows, pass state, and the count', () => {
  const host = document.createElement('div');
  const { el } = mountCheckMatrix(host, passResult(10), { stagger: 0 });
  assert.equal(el.dataset.state, 'pass');
  const rows = el.querySelectorAll('.pg-matrix-row');
  assert.equal(rows.length, 10, 'all ten checks render as rows');
  assert.match(el.querySelector('.pg-matrix-count').textContent, /10 \/ 10/);
  assert.equal(el.querySelector('.pg-matrix-failnote'), null, 'no failnote when passing');
  assert.ok(rows.every((r) => r.dataset.revealed === '1'), 'rows reveal at stagger 0');
  assert.ok(rows.every((r) => r.dataset.pass === 'true'));
});

test('a failing plan_hash is flagged, glyphed, and shows the verifier detail', () => {
  const detail = 'package PlanHash 1a2b… not in 1 approval entry';
  const result = {
    passed: false,
    checks: [
      { name: 'version', passed: true, detail: 'v4' },
      { name: 'export_hash', passed: true, detail: 'recomputed' },
      { name: 'plan_hash', passed: false, detail },
    ],
  };
  const host = document.createElement('div');
  const { el } = mountCheckMatrix(host, result, { stagger: 0 });
  assert.equal(el.dataset.state, 'fail');
  assert.ok(el.querySelector('.pg-matrix-failnote'), 'failnote present on failure');
  const rows = el.querySelectorAll('.pg-matrix-row');
  const failRow = rows.find((r) => r.dataset.fail === '1');
  assert.ok(failRow, 'the failing check has data-fail');
  assert.equal(failRow.dataset.pass, 'false');
  assert.equal(failRow.querySelector('.pg-matrix-rowglyph').textContent, '✗');
  assert.equal(failRow.querySelector('.pg-matrix-name').textContent, 'Approval bound to the exact plan');
  assert.equal(failRow.querySelector('.pg-matrix-detail').textContent, detail);
});

test('an indexed inclusion_proof name resolves to the collection title', () => {
  const result = {
    passed: false,
    checks: [{ name: 'inclusion_proof[3]', passed: false, detail: 'reconstructed root ≠ ChainHash' }],
  };
  const host = document.createElement('div');
  const { el } = mountCheckMatrix(host, result, { stagger: 0 });
  const name = el.querySelector('.pg-matrix-name').textContent;
  assert.equal(name, 'Every step is in the tamper-evident chain');
});

test('baseName normalizes every indexed form', () => {
  assert.equal(baseName('inclusion_proof[3]'), 'inclusion_proofs');
  assert.equal(baseName('inclusion_proofs'), 'inclusion_proofs');
  assert.equal(baseName('trust_snapshot[0]'), 'trust_snapshot');
  assert.equal(baseName('plugin_versions[1]'), 'plugin_versions');
  assert.equal(baseName('plan_hash'), 'plan_hash');
});

test('replay() re-reveals all rows', () => {
  const host = document.createElement('div');
  const { el, replay } = mountCheckMatrix(host, passResult(10), { stagger: 0 });
  const rows = el.querySelectorAll('.pg-matrix-row');
  rows.forEach((r) => { r.dataset.revealed = '0'; });
  replay();
  assert.ok(rows.every((r) => r.dataset.revealed === '1'), 'replay re-reveals every row');
});

test('error string is preferred over detail and still renders', () => {
  const result = { passed: false, checks: [{ name: 'export_hash', passed: false, error: 'failed to recompute: boom' }] };
  const host = document.createElement('div');
  const { el } = mountCheckMatrix(host, result, { stagger: 0 });
  assert.equal(el.querySelector('.pg-matrix-detail').textContent, 'failed to recompute: boom');
});
