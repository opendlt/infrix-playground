// RB-03 — Run Theater spine smokes.
//
// Pins the spine component: node lifecycle (pending→running→complete/failed),
// real-hash display, the reduced-motion contract (no animations), the seal beat,
// and stage-colour resolution from the Spine Aurora gradient. Dependency-free:
// a tiny DOM shim (the repo's jsdom-free convention) provides exactly the surface
// spine.js touches, including style.setProperty, classList, animate, and
// getComputedStyle.

import { test } from 'node:test';
import assert from 'node:assert/strict';

let animateCalls = 0;
class El {
  constructor(tag) {
    this.tagName = String(tag).toUpperCase();
    this.className = '';
    this._text = null;
    this.children = [];
    this.dataset = {};
    this.attributes = {};
    this._classes = new Set();
    this.style = { _p: {}, setProperty(k, v) { this._p[k] = v; }, getPropertyValue(k) { return this._p[k] || ''; } };
  }
  set textContent(v) { this._text = v == null ? '' : String(v); this.children = []; }
  get textContent() { return this._text != null ? this._text : this.children.map((c) => c.textContent).join(''); }
  appendChild(c) { this.children.push(c); return c; }
  replaceChildren(...n) { this.children = []; for (const x of n) this.appendChild(x); }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  getAttribute(k) { return k in this.attributes ? this.attributes[k] : null; }
  animate() { animateCalls += 1; return { finished: Promise.resolve() }; }
  get classList() {
    const s = this._classes;
    return { add: (c) => s.add(c), remove: (c) => s.delete(c), contains: (c) => s.has(c) };
  }
  _matches(sel) { return sel.startsWith('.') ? this.className.split(/\s+/).includes(sel.slice(1)) : this.tagName === sel.toUpperCase(); }
  querySelectorAll(sel) { const o = []; const w = (n) => { for (const c of n.children) { if (c._matches(sel)) o.push(c); w(c); } }; w(this); return o; }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
}

const VARS = {
  '--spine-1': '#8E7BFF', '--spine-2': '#6E9CFF', '--spine-3': '#4EC8FF',
  '--spine-4': '#4DE3B5', '--spine-5': '#6EFFB0', '--spine-7': '#FFD24E',
  '--accent': '#8F82FF', '--text': '#ECEFFA',
};
const docEl = new El('html');
docEl.dataset.theme = 'dark';
globalThis.document = { createElement: (t) => new El(t), documentElement: docEl };
globalThis.window = { matchMedia: () => ({ matches: false }) };
globalThis.getComputedStyle = () => ({ getPropertyValue: (n) => VARS[n] || '' });

const { mountSpine, stageColor } = await import('../spine.js');

const STEPS = [
  { key: 'intent', label: 'Intent', color: 'rgb(1,1,1)' },
  { key: 'plan', label: 'Plan', color: 'rgb(2,2,2)' },
  { key: 'outcome', label: 'Outcome', color: 'rgb(3,3,3)' },
];

test('mountSpine renders one pending node per step with its label and colour', () => {
  const host = document.createElement('div');
  const { el } = mountSpine(host, STEPS, { reduce: true });
  const nodes = el.querySelectorAll('.pg-spine-node');
  assert.equal(nodes.length, 3);
  assert.ok(nodes.every((n) => n.dataset.state === 'pending'));
  assert.equal(nodes[0].dataset.key, 'intent');
  assert.equal(nodes[0].querySelector('.pg-spine-label').textContent, 'Intent');
  assert.equal(nodes[0].style.getPropertyValue('--node-color'), 'rgb(1,1,1)');
});

test('set() drives state and shows the real hash', () => {
  const host = document.createElement('div');
  const s = mountSpine(host, STEPS, { reduce: true });
  s.set('plan', 'complete', '769e4062f1339d82');
  const plan = s.el.querySelectorAll('.pg-spine-node').find((n) => n.dataset.key === 'plan');
  assert.equal(plan.dataset.state, 'complete');
  assert.equal(plan.querySelector('.pg-spine-hash').textContent, '769e4062f1339d82');
  s.set('outcome', 'failed');
  const out = s.el.querySelectorAll('.pg-spine-node').find((n) => n.dataset.key === 'outcome');
  assert.equal(out.dataset.state, 'failed');
});

test('reduced motion suppresses all animation', () => {
  animateCalls = 0;
  const host = document.createElement('div');
  const s = mountSpine(host, STEPS, { reduce: true });
  s.set('intent', 'complete', 'abc');
  s.set('plan', 'failed');
  assert.equal(animateCalls, 0, 'no animate() calls under reduced motion');
});

test('with motion, completing a node animates it', () => {
  animateCalls = 0;
  const host = document.createElement('div');
  const s = mountSpine(host, STEPS, { reduce: false });
  s.set('intent', 'complete', 'abc');
  assert.ok(animateCalls > 0, 'complete animates the dot when motion is allowed');
});

test('seal() under reduced motion resolves and marks the spine sealed', async () => {
  const host = document.createElement('div');
  const s = mountSpine(host, STEPS, { reduce: true });
  await s.seal();
  assert.ok(s.el.classList.contains('pg-spine-sealed'));
});

test('stageColor resolves the gradient stops to concrete colours', () => {
  assert.equal(stageColor('intent'), 'rgb(142,123,255)');      // --spine-1 #8E7BFF
  assert.equal(stageColor('export'), 'rgb(143,130,255)');      // --accent  #8F82FF
  assert.match(stageColor('approval'), /^rgb\(\d+,\d+,\d+\)$/); // interpolated spine-3→4
});

test('stageColor falls back to var(--accent) for an unknown key', () => {
  assert.equal(stageColor('mystery-stage'), 'var(--accent)');
});
