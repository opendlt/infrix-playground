// Infrix Playground SPA (adoption-09).
//
// A no-install browser experience: pick a golden flow, run it, watch it in
// Cinema, export the proof, verify it IN THE BROWSER (no node trust), and share
// a receipt link. It reuses the canonical shared modules — the proof-receipt
// card, the client-side portable verifier, and the Cinema core — so what a
// visitor sees here is the same trust answer the CLI and Nexus give.

import { mountProofReceipt } from '/components/proofReceiptView.js';
import { mountUserError } from '/components/userErrorCard.js';
import { verifyPortablePackage } from '/lib/portableVerifier.js';
import { canonicalJSONSha256, coerce32 } from '/lib/canonicalJson.js';
import { bundleToCinemaProof } from '/bundleScene.js';
import { mountCheckMatrix } from '/checkMatrix.js';
import { FORGERIES, applyForgery, makeReseal } from '/tamper.js';
import { mountSpine, stageColor } from '/spine.js';

const app = document.getElementById('app');
const modeBadge = document.getElementById('pg-mode-badge');

let config = { kermitEnabled: false, allowedFlows: ['golden-escrow'], modes: ['anonymous'] };
let selectedMode = 'anonymous';

// ---- onboarding analytics (adoption-12): OPT-IN, privacy-preserving ----
// Disabled by default (hosted products require opt-in). When opted in, only
// safe, redacted fields are sent (event, mode, result, proofLevel) — never an
// account URL, key, proof bundle, or business data. The session id is a random
// per-browser-session token (not tied to identity).
const telemetry = {
  key: 'pg:analytics:optin',
  optedIn() {
    try { return localStorage.getItem(this.key) === '1'; } catch { return false; }
  },
  setOptIn(on) {
    try { localStorage.setItem(this.key, on ? '1' : '0'); } catch { /* ignore */ }
  },
  sessionId() {
    try {
      let s = sessionStorage.getItem('pg:analytics:sid');
      if (!s) { s = 's_' + Math.random().toString(36).slice(2, 14); sessionStorage.setItem('pg:analytics:sid', s); }
      return s;
    } catch { return 's_anon'; }
  },
  emit(event, extra = {}) {
    if (!this.optedIn()) return;
    const body = {
      version: '1',
      time: new Date().toISOString(),
      source: 'hosted',
      sessionId: this.sessionId(),
      event,
      redacted: true,
      ...extra,
    };
    // Fire-and-forget; never block or surface errors to the user.
    try { fetch('/api/events', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).catch(() => {}); } catch { /* ignore */ }
  },
};

// ---- helpers ----
function el(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text != null) n.textContent = String(text);
  return n;
}

// prefersReducedMotion reports whether the visitor asked for reduced motion, so
// the verdict matrix (and other animations) can present instantly instead of
// staggering. Defaults to false when matchMedia is unavailable.
function prefersReducedMotion() {
  try { return window.matchMedia('(prefers-reduced-motion: reduce)').matches; } catch { return false; }
}

async function api(path, opts) {
  const res = await fetch(path, opts);
  let body = null;
  try { body = await res.json(); } catch { body = null; }
  if (!res.ok) {
    const err = new Error((body && body.error && body.error.message) || `${path}: ${res.status}`);
    if (body && body.error) err.userError = body.error;
    err.status = res.status;
    throw err;
  }
  return body;
}

function showError(container, e) {
  const box = el('div', 'pg-error');
  if (e && e.userError) {
    mountUserError(box, e.userError);
  } else {
    box.appendChild(el('div', 'pg-action-title', 'Something went wrong'));
    box.appendChild(el('div', 'pg-note', (e && e.message) || String(e)));
  }
  container.appendChild(box);
}

function setNav(name) {
  document.querySelectorAll('.pg-nav a').forEach((a) => {
    if (a.getAttribute('data-nav') === name) a.setAttribute('aria-current', 'page');
    else a.removeAttribute('aria-current');
  });
}

function updateModeBadge() {
  modeBadge.hidden = false;
  modeBadge.dataset.mode = selectedMode;
  modeBadge.textContent = selectedMode === 'kermit' ? 'Kermit Sandbox (live testnet)' : 'Anonymous Demo (fixture-backed)';
}

// ---- home (the landing) ----
// Leads with the live spine — the brand and the thesis in one image — then a
// single primary CTA and quiet side-doors. The spine self-assembles on a loop
// (ambient), or shows a static completed flow under reduced motion.
function renderHome() {
  setNav('home');
  updateModeBadge();
  app.replaceChildren();

  const hero = el('section', 'pg-hero pg-home-hero');

  // Ambient spine — the signature, looping quietly above the headline.
  const spineHost = el('div', 'pg-home-spine');
  hero.appendChild(spineHost);
  const reduce = prefersReducedMotion();
  const ambKeys = skeletonSteps().map((s) => s.key);
  const ambSteps = ambKeys.map((k) => ({ key: k, label: RUN_SHORT_LABEL[k] || k, color: stageColor(k) }));
  const ambSpine = mountSpine(spineHost, ambSteps, { reduce });
  runAmbientSpine(ambSpine, ambKeys, reduce);

  hero.appendChild(el('h1', null, 'See a governed deal prove itself.'));
  hero.appendChild(el('p', null,
    'Run it, then verify every cryptographic link in your own browser. No install, no wallet, no trust in us.'));

  // One primary CTA + quiet side-doors.
  const cta = el('div', 'pg-home-cta');
  const run = el('button', 'pg-btn pg-btn-primary pg-cta', 'Run the escrow →');
  run.type = 'button';
  run.addEventListener('click', () => startRun(selectedMode));
  cta.appendChild(run);
  const verifyLink = el('a', 'pg-home-door', 'Already have a proof? Verify it →');
  verifyLink.href = '#/verify';
  cta.appendChild(verifyLink);
  const labLink = el('a', 'pg-home-door', 'or break one in the Tamper Lab →');
  labLink.href = '#/lab';
  cta.appendChild(labLink);
  hero.appendChild(cta);

  // Compact inline mode selector right by the CTA.
  const modes = el('div', 'pg-home-modes');
  modes.appendChild(el('span', 'pg-home-modes-label', 'Mode'));
  modes.appendChild(modeButton('anonymous', 'Anonymous demo', true));
  modes.appendChild(modeButton('kermit', 'Kermit sandbox', config.kermitEnabled));
  hero.appendChild(modes);
  const note = el('div', 'pg-mode-note');
  note.textContent = config.kermitEnabled
    ? 'Anonymous is fixture-backed and caps at L3 (no live-L0 claim). Kermit runs live against the testnet and can reach L4.'
    : 'Anonymous is fixture-backed and caps at L3 (no live-L0 claim). Kermit Sandbox is disabled on this instance.';
  hero.appendChild(note);

  app.appendChild(hero);

  // Slim footer row — secondary tools + the opt-in analytics toggle live here,
  // out of the hero's way.
  const foot = el('div', 'pg-home-footer');
  const mm = el('a', 'pg-home-foot-link', 'MetaMask signing →');
  mm.href = '#/metamask';
  foot.appendChild(mm);
  const st = el('a', 'pg-home-foot-link', 'What it can do →');
  st.href = '#/status';
  foot.appendChild(st);

  const optWrap = el('label', 'pg-analytics-optin');
  const optBox = el('input');
  optBox.type = 'checkbox';
  optBox.checked = telemetry.optedIn();
  optBox.addEventListener('change', () => telemetry.setOptIn(optBox.checked));
  optWrap.appendChild(optBox);
  optWrap.appendChild(el('span', null, ' Share anonymous onboarding analytics (opt-in; no keys, no proof contents, never your data)'));
  foot.appendChild(optWrap);
  app.appendChild(foot);
}

// runAmbientSpine animates the landing spine: a quiet left-to-right fill on a
// loop. Self-halts when the spine leaves the DOM (view change), so no timer
// leaks. Under reduced motion it shows a static completed flow instead.
function runAmbientSpine(spine, keys, reduce) {
  if (reduce) { keys.forEach((k) => spine.set(k, 'complete')); return; }
  let i = 0;
  const tick = () => {
    if (!spine.el || !spine.el.isConnected) return; // view changed → stop
    if (i === 0) keys.forEach((k) => spine.set(k, 'pending'));
    if (i < keys.length) {
      spine.set(keys[i], 'complete');
      i += 1;
      setTimeout(tick, 240);
    } else {
      i = 0;
      setTimeout(tick, 1600); // hold the finished flow, then loop
    }
  };
  tick();
}

function modeButton(mode, label, enabled) {
  const b = el('button', 'pg-mode-btn', label);
  b.type = 'button';
  b.setAttribute('aria-pressed', String(selectedMode === mode));
  if (!enabled) { b.disabled = true; b.title = 'Disabled on this instance'; }
  b.addEventListener('click', () => {
    if (b.disabled) return;
    selectedMode = mode;
    updateModeBadge();
    renderHome();
  });
  return b;
}

// ---- run (the Run Theater) ----
// The governed flow assembles left-to-right as a Spine: each stage lights in its
// gradient colour and emits its REAL artifact hash (hash-linked to the previous
// stage), ending in a "seal" payoff beat before the verdict opens. The completion
// events stream in fast; the client paces them so the assembly is legible — the
// data (order, hashes) is entirely real, only the cadence is presentation.
const RUN_SHORT_LABEL = Object.freeze({
  intent: 'Intent', plan: 'Plan', policy: 'Policy', approval: 'Approval',
  credential: 'Credential', outcome: 'Outcome', anchor: 'Anchored',
  export: 'Exported', verify: 'Verified',
});

// makeChannel is a tiny async queue: SSE callbacks push events; the consumer
// awaits them in order. This lets us process a burst of completion events with
// deliberate pacing without dropping or reordering any.
function makeChannel() {
  const buf = [];
  let waiter = null;
  return {
    push(x) { buf.push(x); if (waiter) { const w = waiter; waiter = null; w(); } },
    next() {
      if (buf.length) return Promise.resolve(buf.shift());
      return new Promise((resolve) => { waiter = () => resolve(buf.shift()); });
    },
  };
}

async function startRun(mode) {
  setNav('home');
  app.replaceChildren();

  const head = el('section', 'pg-hero');
  head.appendChild(el('p', 'pg-eyebrow', mode === 'kermit'
    ? 'KERMIT SANDBOX · LIVE TESTNET' : 'ANONYMOUS DEMO · FIXTURE-BACKED'));
  head.appendChild(el('h1', null, 'Building a tamper-evident proof'));
  head.appendChild(el('p', null, mode === 'kermit'
    ? 'Live against the Kermit testnet — each stage hash-linked to the last.'
    : 'A governed escrow, hash-linked stage by stage — no wallet, no funding.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  const spineHost = el('div', 'pg-spine-host');
  panel.appendChild(spineHost);
  const card = el('div', 'pg-stage-card');
  panel.appendChild(card);
  app.appendChild(panel);

  let started;
  try {
    started = await api('/api/runs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode, flow: 'golden-escrow' }),
    });
  } catch (e) {
    showError(app, e);
    return;
  }

  const steps = (started.steps && started.steps.length) ? started.steps : skeletonSteps();
  const labelByKey = {};
  for (const s of steps) labelByKey[s.key] = s.label;
  const reduce = prefersReducedMotion();
  const spineSteps = steps.map((s) => ({
    key: s.key,
    label: RUN_SHORT_LABEL[s.key] || s.label,
    color: stageColor(s.key),
  }));
  const spine = mountSpine(spineHost, spineSteps, { reduce });

  // The active-stage card narrates the stage the spine is on.
  const cardTitle = el('div', 'pg-stage-title', 'Starting the governed run…');
  const cardHash = el('code', 'pg-stage-hash');
  const cardDesc = el('div', 'pg-stage-desc', 'Asking the node to run the flow; the proof is verified in your browser.');
  card.appendChild(cardTitle);
  card.appendChild(cardHash);
  card.appendChild(cardDesc);

  const swapCard = (step, phase) => {
    cardTitle.textContent = labelByKey[step.key] || step.label || step.key;
    cardHash.textContent = step.hash ? `hash ${step.hash}…` : '';
    cardDesc.textContent = phase === 'complete'
      ? (step.hash ? 'Hash-linked into the tamper-evident chain.' : 'Recorded in the proof.')
      : 'Running…';
    if (!reduce && typeof card.animate === 'function') {
      card.animate(
        [{ opacity: 0.45, transform: 'translateY(6px)' }, { opacity: 1, transform: 'none' }],
        { duration: 180, easing: 'cubic-bezier(0.4,0,0.2,1)' },
      );
    }
  };

  const failRun = (msg) => {
    const box = el('div', 'pg-run-failnote');
    box.appendChild(el('div', 'pg-action-title', 'The run did not complete'));
    box.appendChild(el('div', 'pg-note', msg || 'Something interrupted the run.'));
    const retry = el('button', 'pg-btn pg-btn-primary', 'Try again');
    retry.type = 'button';
    retry.addEventListener('click', () => startRun(mode));
    box.appendChild(retry);
    card.replaceChildren(box);
  };

  const ch = makeChannel();
  const src = new EventSource(`/api/runs/${encodeURIComponent(started.id)}/events`);
  let terminated = false;
  src.onmessage = (ev) => { try { ch.push(JSON.parse(ev.data)); } catch { /* ignore malformed frame */ } };
  src.onerror = () => { if (!terminated) ch.push({ type: '__sse_error' }); };

  const delay = (ms) => new Promise((r) => setTimeout(r, ms));

  (async () => {
    for (;;) {
      const e = await ch.next();
      if (terminated) return;
      if (!e) continue;
      if (e.type === 'step' && e.step) {
        const st = e.step;
        if (st.status === 'failed') {
          terminated = true; src.close();
          spine.set(st.key, 'failed');
          failRun(`Stage “${labelByKey[st.key] || st.key}” failed.`);
          return;
        }
        spine.set(st.key, 'running');
        swapCard(st, 'running');
        await delay(reduce ? 0 : 150);
        if (terminated) return;
        spine.set(st.key, 'complete', st.hash);
        swapCard(st, 'complete');
        await delay(reduce ? 0 : 40);
      } else if (e.type === 'done' && e.receiptId) {
        terminated = true; src.close();
        cardTitle.textContent = 'Proof sealed';
        cardHash.textContent = '';
        cardDesc.textContent = 'Verified independently — opening the verdict.';
        telemetry.emit('demo.completed', { mode: mode === 'kermit' ? 'kermit' : 'local', result: 'success' });
        await spine.seal();
        location.hash = `#/r/${e.receiptId}`;
        return;
      } else if (e.type === 'error') {
        terminated = true; src.close();
        telemetry.emit('error.step', { mode: mode === 'kermit' ? 'kermit' : 'local', result: 'failure' });
        failRun(e.error);
        return;
      } else if (e.type === '__sse_error') {
        terminated = true; src.close();
        failRun('The connection to the run was lost.');
        return;
      }
      // "state" and any unknown event types are ignored.
    }
  })();
}

function skeletonSteps() {
  return [
    { key: 'intent', label: 'Buyer submits escrow intent' },
    { key: 'plan', label: 'Infrix compiles a governed plan' },
    { key: 'policy', label: 'Policy authorizes a regulated release' },
    { key: 'approval', label: 'Operator approval bound to the plan hash' },
    { key: 'credential', label: 'Delivery credential verified' },
    { key: 'outcome', label: 'Funds released to the seller' },
    { key: 'anchor', label: 'Evidence anchored' },
    { key: 'export', label: 'Portable proof exported' },
    { key: 'verify', label: 'Verified independently (no node trust)' },
  ];
}

// ---- receipt / share view ----
// The verdict matrix is the hero: the ten cryptographic checks run eagerly in the
// browser (no click), and the receipt card + Cinema replay are demoted into an
// "Inspect the full proof" disclosure below it.
async function renderReceipt(id) {
  setNav('home');
  app.replaceChildren();
  const loading = el('div', 'pg-loading', 'Loading proof receipt…');
  app.appendChild(loading);

  let data;
  try {
    data = await api(`/api/receipts/${encodeURIComponent(id)}`);
  } catch (e) {
    app.replaceChildren();
    showError(app, e);
    return;
  }
  app.replaceChildren();

  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'Proof receipt'));
  head.appendChild(el('p', null, data.mode === 'kermit'
    ? 'Live Kermit run — confirmed against the testnet.'
    : 'Fixture-backed run — verified offline, caps at L3 (no live-L0 claim).'));
  app.appendChild(head);

  // The verdict matrix is the hero. Start with a loading line; the matrix
  // replaces it once the browser-side verification completes.
  const matrixHost = el('div', 'pg-matrix-host');
  matrixHost.appendChild(el('div', 'pg-loading', 'Verifying in your browser (no node trusted)…'));
  app.appendChild(matrixHost);

  // Actions under the verdict.
  const actions = el('div', 'pg-receipt-actions');
  const dl = el('a', 'pg-btn', 'Download proof bundle');
  dl.href = `/api/receipts/${encodeURIComponent(id)}/bundle`;
  dl.setAttribute('download', `${id}.infrix.json`);
  const reverifyBtn = el('button', 'pg-btn pg-btn-primary', 'Re-verify line by line');
  reverifyBtn.type = 'button';
  reverifyBtn.disabled = true; // enabled once the matrix exists
  const copyBtn = el('button', 'pg-btn', 'Copy share link');
  copyBtn.type = 'button';
  const forgeLink = el('a', 'pg-btn', 'Try to forge it →');
  forgeLink.href = '#/lab';
  actions.appendChild(dl);
  actions.appendChild(reverifyBtn);
  actions.appendChild(copyBtn);
  actions.appendChild(forgeLink);
  app.appendChild(actions);

  copyBtn.addEventListener('click', async () => {
    const url = location.origin + '/#/r/' + id;
    let copied = false;
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(url);
        copied = true;
      }
    } catch { /* fall through to manual reveal */ }
    copyBtn.textContent = copied ? 'Copied ✓' : url;
    setTimeout(() => { copyBtn.textContent = 'Copy share link'; }, 1600);
  });

  // Disclosure: the full proof (receipt card + Cinema replay), demoted below the
  // verdict so it never competes with the trust answer.
  const details = el('details', 'pg-disclosure');
  details.appendChild(el('summary', null, 'Inspect the full proof'));
  const split = el('div', 'pg-split');
  const left = el('div', 'pg-panel');
  left.appendChild(el('h2', null, 'Receipt'));
  const card = el('div');
  mountProofReceipt(card, data.receipt);
  left.appendChild(card);
  const right = el('div', 'pg-panel');
  right.appendChild(el('h2', null, 'Cinema replay'));
  const cinemaHost = el('div', 'pg-cinema-host');
  right.appendChild(cinemaHost);
  split.appendChild(left);
  split.appendChild(right);
  details.appendChild(split);
  app.appendChild(details);

  // Load the bundle once, shared by the eager verifier and the replay.
  const pkgPromise = fetch(`/api/receipts/${encodeURIComponent(id)}/bundle`)
    .then((res) => (res.ok ? res.json() : null))
    .catch(() => null);

  // Eagerly verify in the browser and render the matrix — no click required.
  let matrix = null;
  const pkg = await pkgPromise;
  if (!pkg) {
    matrixHost.replaceChildren();
    showError(matrixHost, { message: 'Could not load the proof bundle to verify.' });
  } else {
    try {
      const result = await verifyPortablePackage(pkg);
      telemetry.emit('proof.verified', { result: result.passed ? 'success' : 'failure' });
      const a = (data.receipt && data.receipt.assurance) || {};
      const label = [a.proofLevel, a.governanceLevel].filter(Boolean).join(' · ');
      matrix = mountCheckMatrix(matrixHost, result, {
        stagger: prefersReducedMotion() ? 0 : 80,
        assuranceLabel: label,
      });
      reverifyBtn.disabled = false;
    } catch (e) {
      matrixHost.replaceChildren();
      showError(matrixHost, e);
    }
  }

  reverifyBtn.addEventListener('click', () => { if (matrix) matrix.replay(); });

  // Lazy-mount the Cinema replay only when the disclosure first opens. While the
  // <details> is collapsed the host has no size, so deferring avoids a zero-size
  // canvas and the heavy cinema-core import until the visitor asks for it.
  let cinemaMounted = false;
  details.addEventListener('toggle', async () => {
    if (!details.open || cinemaMounted) return;
    cinemaMounted = true;
    try {
      const proof = bundleToCinemaProof(data.receipt, pkg);
      const { mountCinema } = await import('/cinema-core/loader.js');
      await mountCinema({ mode: 'cinema.proof', root: cinemaHost, proof });
      telemetry.emit('cinema.opened', { result: 'success' });
    } catch (e) {
      cinemaHost.appendChild(el('div', 'pg-note', 'Replay unavailable in this browser.'));
    }
  });
}

// ---- verify (bring your own proof) ----
// Drop a file, pick a file, paste JSON, or load the sample — every door flows
// into the same RB-01 verdict matrix. Verification runs entirely in the browser.
function renderVerify() {
  setNav('verify');
  app.replaceChildren();
  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'Verify a proof'));
  head.appendChild(el('p', null, 'Drop, choose, or paste a portable proof bundle. Verification runs entirely in your browser — this server is never trusted.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  const out = el('div', 'pg-verify-out');

  // Shared sink: parse → matrix. Used by drop, file-pick, paste, and sample.
  const handle = async (text) => {
    let pkg;
    try { pkg = JSON.parse(text); }
    catch { showError(out, { message: 'That is not valid JSON. Drop or paste a portable proof bundle.' }); return; }
    out.replaceChildren(el('div', 'pg-loading', 'Verifying in your browser…'));
    try {
      const result = await verifyPortablePackage(pkg);
      mountCheckMatrix(out, result, { stagger: prefersReducedMotion() ? 0 : 80 });
    } catch (e) {
      out.replaceChildren();
      showError(out, e);
    }
  };

  // Primary: the dropzone.
  const dz = el('div', 'pg-dropzone');
  dz.tabIndex = 0;
  dz.setAttribute('role', 'button');
  dz.setAttribute('aria-label', 'Drop or choose a proof file to verify');
  dz.appendChild(el('div', 'pg-dropzone-icon', '⬇'));
  dz.appendChild(el('div', 'pg-dropzone-title', 'Drop a .infrix.json proof'));
  dz.appendChild(el('div', 'pg-note', 'or click to choose · no login · verification runs on your machine'));
  panel.appendChild(dz);

  const file = document.createElement('input');
  file.type = 'file';
  file.accept = '.json,application/json';
  file.hidden = true;
  panel.appendChild(file);

  dz.addEventListener('click', () => file.click());
  dz.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); file.click(); } });
  dz.addEventListener('dragover', (e) => { e.preventDefault(); dz.dataset.over = '1'; });
  dz.addEventListener('dragleave', () => { delete dz.dataset.over; });
  dz.addEventListener('drop', async (e) => {
    e.preventDefault();
    delete dz.dataset.over;
    const f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
    if (f) handle(await f.text());
  });
  file.addEventListener('change', async () => { if (file.files[0]) handle(await file.files[0].text()); });

  // Secondary: paste-instead toggle + load-sample.
  const row = el('div', 'pg-verify-row');
  const pasteToggle = el('button', 'pg-btn', 'Paste JSON instead');
  pasteToggle.type = 'button';
  const sampleBtn = el('button', 'pg-btn', 'Load the sample');
  sampleBtn.type = 'button';
  row.appendChild(pasteToggle);
  row.appendChild(sampleBtn);
  row.appendChild(el('span', 'pg-note', 'No login. No upload to us — verification runs entirely on your machine.'));
  panel.appendChild(row);

  // Paste area, hidden until requested.
  const pasteWrap = el('div', 'pg-paste-wrap');
  pasteWrap.hidden = true;
  const ta = el('textarea', 'pg-verify-input');
  ta.setAttribute('aria-label', 'Portable proof bundle JSON');
  ta.placeholder = 'Paste a *.infrix.json portable evidence package here…';
  const verifyBtn = el('button', 'pg-btn pg-btn-primary', 'Verify in browser');
  verifyBtn.type = 'button';
  pasteWrap.appendChild(ta);
  pasteWrap.appendChild(verifyBtn);
  panel.appendChild(pasteWrap);

  panel.appendChild(out);
  app.appendChild(panel);

  pasteToggle.addEventListener('click', () => {
    pasteWrap.hidden = !pasteWrap.hidden;
    pasteToggle.textContent = pasteWrap.hidden ? 'Paste JSON instead' : 'Hide paste box';
    if (!pasteWrap.hidden) ta.focus();
  });
  verifyBtn.addEventListener('click', () => handle(ta.value));
  sampleBtn.addEventListener('click', async () => {
    try {
      const res = await fetch('/api/sample-bundle');
      if (!res.ok) throw new Error('sample bundle unavailable');
      await handle(await res.text());
    } catch (e) { showError(out, e); }
  });
}

// ---- status ("what it can do") ----
// User-facing capability statements, not ops-speak. "Mainnet writes — never" is
// framed as a safety guarantee, not a missing feature. /api/readiness is unchanged.
async function renderStatus() {
  setNav('status');
  app.replaceChildren();
  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'What this playground can do'));
  head.appendChild(el('p', null, 'Right here, right now — no install, no account.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  app.appendChild(panel);

  const row = (on, title, desc) => {
    const d = el('div', 'pg-cap');
    d.dataset.on = on ? '1' : '0';
    d.appendChild(el('span', 'pg-cap-dot'));
    const body = el('div');
    body.appendChild(el('div', 'pg-cap-title', title));
    body.appendChild(el('div', 'pg-cap-desc', desc));
    d.appendChild(body);
    return d;
  };

  try {
    const r = await api('/api/readiness');
    panel.appendChild(row(r.anonymous, 'Run a demo', 'Yes — right now, no wallet, no funding.'));
    panel.appendChild(row(r.kermit, 'Live testnet (Kermit)', r.kermit ? 'Enabled on this instance — can reach L4.' : 'Off on this instance; the demo runs the same flow deterministically.'));
    panel.appendChild(row(r.verifier, 'Verify your own proof', 'Yes — entirely in your browser, no node trusted.'));
    panel.appendChild(row(!!r.fixture, 'Sample proof', r.fixture ? 'Ready to load in Verify and the Tamper Lab.' : 'Unavailable on this instance.'));
    // Safety guarantee, not a gap.
    panel.appendChild(row(true, 'Mainnet writes', 'Never. This playground can’t touch real funds.'));
  } catch (e) {
    showError(panel, e);
  }
}

// ---- metamask ----
async function renderMetaMask() {
  setNav('home');
  app.replaceChildren();
  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'MetaMask typed-data signing'));
  head.appendChild(el('p', null, 'Infrix admits intents signed via MetaMask EIP-712 typed data, bound to an Accumulate key page. Try a signing smoke test here.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  app.appendChild(panel);

  const provider = (typeof window !== 'undefined') ? window.ethereum : null;
  if (!provider) {
    const box = el('div', 'pg-error');
    mountUserError(box, {
      code: 'METAMASK_PROVIDER_MISSING',
      title: 'No MetaMask provider was found',
      message: 'Install or unlock MetaMask to try typed-data signing.',
      impact: 'The SDK cannot request a signature without a wallet provider.',
      fixes: [{ label: 'Install/unlock MetaMask, then reload', safeToRun: false }],
      docs: 'docs/errors/metamask.md',
    });
    panel.appendChild(box);
    panel.appendChild(el('div', 'pg-note', 'The full MetaMask escrow example lives at examples/metamask-golden-escrow.'));
    return;
  }

  const btn = el('button', 'pg-btn pg-btn-primary', 'Connect & sign sample typed data');
  btn.type = 'button';
  panel.appendChild(btn);
  const out = el('div');
  panel.appendChild(out);

  btn.addEventListener('click', async () => {
    out.replaceChildren(el('div', 'pg-loading', 'Awaiting MetaMask…'));
    try {
      const accounts = await provider.request({ method: 'eth_requestAccounts' });
      const from = accounts && accounts[0];
      const typed = sampleTypedData();
      const sig = await provider.request({
        method: 'eth_signTypedData_v4',
        params: [from, JSON.stringify(typed)],
      });
      const dl = el('dl', 'pg-kv');
      dl.appendChild(el('dt', null, 'Account'));
      dl.appendChild(el('dd', null, from));
      dl.appendChild(el('dt', null, 'Signature'));
      dl.appendChild(el('dd', null, sig));
      out.replaceChildren(el('div', 'pg-meta', 'Signed. In a real flow this signature is verified against your Accumulate key page before the intent is admitted.'), dl);
    } catch (e) {
      out.replaceChildren();
      const code = (e && e.code === 4001) ? 'METAMASK_USER_REJECTED' : 'METAMASK_PUBLIC_KEY_RECOVERY_FAILED';
      showError(out, { userError: {
        code,
        title: code === 'METAMASK_USER_REJECTED' ? 'The signature was rejected in MetaMask' : 'Signing failed',
        message: (e && e.message) || 'Signing did not complete.',
        fixes: [{ label: 'Re-run and approve the MetaMask prompt', safeToRun: false }],
        docs: 'docs/errors/metamask.md',
        retryable: true,
      } });
    }
  });
}

function sampleTypedData() {
  return {
    types: {
      EIP712Domain: [
        { name: 'name', type: 'string' },
        { name: 'version', type: 'string' },
      ],
      InfrixIntent: [
        { name: 'goalType', type: 'string' },
        { name: 'signer', type: 'string' },
        { name: 'nonce', type: 'string' },
      ],
    },
    primaryType: 'InfrixIntent',
    domain: { name: 'Infrix', version: '1' },
    message: { goalType: 'TOKEN_TRANSFER', signer: 'acc://playground.acme/book/1', nonce: 'playground-demo' },
  };
}

// ---- tamper lab ----
// Let a visitor try to forge the proof and watch the verifier catch them — the
// same client-side maths the receipt page runs. No backend, no node trust. Some
// forgeries re-seal the export hash (a "sophisticated" forger) so the proof is
// caught by an inner cryptographic binding instead of the outer seal.
async function renderLab() {
  setNav('lab');
  app.replaceChildren();

  const head = el('section', 'pg-hero');
  head.appendChild(el('p', 'pg-eyebrow', 'TAMPER LAB · TRY TO BEAT THE MATHS'));
  head.appendChild(el('h1', null, 'Forge a proof'));
  head.appendChild(el('p', null,
    'Pick a tamper. Each one re-verifies the bundle in your browser — the same checks the CLI runs — and the proof catches it. Nothing is uploaded; nothing is trusted.'));
  app.appendChild(head);

  const split = el('div', 'pg-lab-split');
  const left = el('div', 'pg-panel');
  const right = el('div', 'pg-panel');
  split.appendChild(left);
  split.appendChild(right);
  app.appendChild(split);

  // Load a clean sample (already an endpoint); keep it pristine as the baseline.
  let clean;
  try {
    const res = await fetch('/api/sample-bundle');
    if (!res.ok) throw new Error('sample bundle unavailable');
    clean = await res.json();
  } catch (e) { showError(left, e); return; }

  // The re-seal primitive uses the SAME canonical hasher the verifier uses, so a
  // "sophisticated" forgery genuinely passes the export-hash seal and is caught
  // by the inner binding instead.
  const reseal = makeReseal({ canonicalJSONSha256, coerce32 });
  const reduce = prefersReducedMotion();

  // Right: the live verdict (the RB-01 matrix).
  right.appendChild(el('h2', null, 'Live verdict'));
  const verdictHost = el('div', 'pg-matrix-host');
  right.appendChild(verdictHost);
  const verify = async (pkg) => {
    verdictHost.replaceChildren(el('div', 'pg-loading', 'Verifying in your browser…'));
    const result = await verifyPortablePackage(pkg);
    mountCheckMatrix(verdictHost, result, { stagger: reduce ? 0 : 60 });
    telemetry.emit('proof.verified', { result: result.passed ? 'success' : 'failure' });
    return result;
  };

  // Left: a readout of the clean proof, the forgery buttons, score, reset.
  left.appendChild(el('h2', null, 'The proof'));
  left.appendChild(buildBundleReadout(clean));

  let attempts = 0;
  let caught = 0;
  const score = el('div', 'pg-lab-score', 'Pick a tamper below to attack the proof.');

  const btnRow = el('div', 'pg-lab-forgeries');
  for (const f of FORGERIES) {
    const b = el('button', 'pg-btn', f.label);
    b.type = 'button';
    b.title = f.blurb;
    b.addEventListener('click', async () => {
      setBusy(btnRow, true);
      reset.disabled = true;
      try {
        const { pkg } = await applyForgery(clean, f.id, reseal);
        const result = await verify(pkg);
        attempts += 1;
        if (!result.passed) caught += 1;
        score.textContent = result.passed
          ? `The proof was EVADED (${caught} / ${attempts} caught). That should not happen — please report it.`
          : `Caught ${caught} / ${attempts}. ${f.blurb}`;
      } catch (e) {
        showError(verdictHost, e);
      } finally {
        setBusy(btnRow, false);
        reset.disabled = false;
      }
    });
    btnRow.appendChild(b);
  }

  const reset = el('button', 'pg-btn pg-btn-primary', 'Reset to a clean proof');
  reset.type = 'button';
  reset.addEventListener('click', async () => {
    await verify(clean);
    score.textContent = 'Clean proof restored. Pick a tamper to attack it.';
  });

  left.appendChild(el('div', 'pg-lab-prompt', 'Try to defeat it:'));
  left.appendChild(btnRow);
  left.appendChild(score);
  left.appendChild(reset);

  // Start from the clean baseline: 10/10.
  await verify(clean);
}

// buildBundleReadout shows the salient fields a visitor is about to attack, so
// the tamper is concrete (not an opaque JSON edit). Read-only; derived from the
// clean bundle. Accepts bundleData as an inlined object or a JSON string.
function buildBundleReadout(pkg) {
  const dl = el('dl', 'pg-kv');
  const add = (k, v) => { if (v == null || v === '') return; dl.appendChild(el('dt', null, k)); dl.appendChild(el('dd', null, v)); };
  let bd = pkg.bundleData;
  if (typeof bd === 'string') { try { bd = JSON.parse(bd); } catch { bd = {}; } }
  bd = bd || {};
  const approval = (bd.approvalEvidence || bd.ApprovalEvidence || [])[0] || {};
  const policy = (bd.policyDecisions || bd.PolicyDecisions || [])[0] || {};
  const plugin = (pkg.pluginVersions || [])[0] || {};
  add('Approver', approval.identity);
  add('Plan hash', shortHex(pkg.planHash));
  add('Outcome digest', shortHex(pkg.outcomeDigest));
  add('Policy', policy.decision ? `${policy.decision} (${policy.ruleId || policy.policyType || 'rule'})` : '');
  add('Plugin', plugin.pluginId ? `${plugin.pluginId}@${plugin.version || '?'}` : '');
  add('Inclusion proofs', Array.isArray(pkg.inclusionProofs) ? String(pkg.inclusionProofs.length) : '');
  return dl;
}

// shortHex renders the first n bytes of a [N]byte array as hex with an ellipsis.
function shortHex(arr, n = 6) {
  if (!Array.isArray(arr)) return '';
  return arr.slice(0, n).map((b) => (b & 0xff).toString(16).padStart(2, '0')).join('') + '…';
}

function setBusy(container, on) {
  container.querySelectorAll('button').forEach((b) => { b.disabled = on; });
}

// ---- theme (Spine Aurora) ----
// The shared /styles.css defines three themes via :root[data-theme]; the
// attribute VALUES are dark / light / contrast (the design-system display names
// are Aurora-Dark / Daylight / Phosphor — we show those as labels). We set the
// attribute and remember the choice, wired before first render to avoid a flash.
const THEMES = ['dark', 'light', 'contrast'];
const THEME_GLYPH = { dark: '☾', light: '☀', contrast: '▮' };
const THEME_LABEL = { dark: 'Dark', light: 'Daylight', contrast: 'Phosphor' };

function applyTheme(t) {
  document.documentElement.dataset.theme = t;
  const btn = document.getElementById('pg-theme');
  if (btn) {
    btn.textContent = THEME_GLYPH[t];
    btn.title = `Theme: ${THEME_LABEL[t]} (click to switch)`;
    btn.setAttribute('aria-label', `Theme: ${THEME_LABEL[t]}. Switch theme.`);
  }
  try { localStorage.setItem('pg:theme', t); } catch { /* storage blocked */ }
}

function initTheme() {
  let t;
  try { t = localStorage.getItem('pg:theme'); } catch { /* storage blocked */ }
  if (!THEMES.includes(t)) t = 'dark';
  applyTheme(t);
  const btn = document.getElementById('pg-theme');
  if (btn) {
    btn.addEventListener('click', () => {
      const cur = document.documentElement.dataset.theme;
      const next = THEMES[(THEMES.indexOf(cur) + 1) % THEMES.length];
      applyTheme(next);
    });
  }
}

// ---- boot ----
async function boot() {
  initTheme();
  try {
    config = await api('/api/config');
    if (!config.kermitEnabled && selectedMode === 'kermit') selectedMode = 'anonymous';
  } catch { /* defaults stand */ }
  telemetry.emit('page.loaded');
  window.addEventListener('hashchange', route);
  route();
}

function route() {
  const hash = (location.hash || '#/').replace(/^#/, '');
  if (hash.startsWith('/r/')) return renderReceipt(decodeURIComponent(hash.slice(3)));
  if (hash === '/verify') return renderVerify();
  if (hash === '/lab') return renderLab();
  if (hash === '/readiness') { location.replace('#/status'); return; } // legacy link → status
  if (hash === '/status') return renderStatus();
  if (hash === '/metamask') return renderMetaMask();
  return renderHome();
}

boot();
