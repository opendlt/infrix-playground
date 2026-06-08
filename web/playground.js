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
import { buildReceiptFromVerifier } from '/lib/proofReceipt.js';
import { bundleToCinemaProof } from '/bundleScene.js';

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

// ---- home ----
function renderHome() {
  setNav('home');
  updateModeBadge();
  app.replaceChildren();

  const hero = el('section', 'pg-hero');
  hero.appendChild(el('h1', null, 'Feel Infrix in your browser'));
  hero.appendChild(el('p', null,
    'Run a real governed flow, watch it in Cinema, export a portable proof, and verify it right here — no install, no wallet, no funding.'));
  app.appendChild(hero);

  const actions = el('div', 'pg-actions');
  actions.appendChild(actionCard('Run a governed escrow', 'Execute the verifiable-escrow flow and get a proof receipt.', () => startRun(selectedMode)));
  actions.appendChild(actionCard('Verify a proof', 'Bring your own proof bundle and verify it in your browser.', () => { location.hash = '#/verify'; }));
  actions.appendChild(actionCard('Watch a replay', 'Run the flow, then watch the Cinema replay of the proof.', () => startRun(selectedMode)));
  app.appendChild(actions);

  // Mode selector.
  const modes = el('section', 'pg-modes');
  modes.appendChild(el('h2', null, 'Mode'));
  const row = el('div', 'pg-mode-row');
  row.appendChild(modeButton('anonymous', 'Anonymous Demo', true));
  row.appendChild(modeButton('kermit', 'Kermit Sandbox', config.kermitEnabled));
  modes.appendChild(row);
  const note = el('div', 'pg-mode-note');
  note.textContent = config.kermitEnabled
    ? 'Anonymous mode is fixture-backed and caps at L3 (no live-L0 claim). Kermit Sandbox runs live against the Kermit testnet and can reach L4.'
    : 'Anonymous mode is fixture-backed and caps at L3 (no live-L0 claim). Kermit Sandbox is disabled on this instance.';
  modes.appendChild(note);
  app.appendChild(modes);

  const secondary = el('div', 'pg-secondary');
  const mm = el('a', null, 'Try MetaMask typed-data signing →');
  mm.href = '#/metamask';
  secondary.appendChild(mm);
  const rd = el('a', null, 'Inspect readiness →');
  rd.href = '#/readiness';
  secondary.appendChild(rd);
  app.appendChild(secondary);

  // adoption-12 — opt-in analytics toggle (off by default).
  const optWrap = el('label', 'pg-analytics-optin');
  const optBox = el('input');
  optBox.type = 'checkbox';
  optBox.checked = telemetry.optedIn();
  optBox.addEventListener('change', () => telemetry.setOptIn(optBox.checked));
  optWrap.appendChild(optBox);
  optWrap.appendChild(el('span', null, ' Share anonymous onboarding analytics (opt-in; no keys, no proof contents, never your data)'));
  app.appendChild(optWrap);
}

function actionCard(title, desc, onClick) {
  const b = el('button', 'pg-action');
  b.type = 'button';
  b.appendChild(el('span', 'pg-action-title', title));
  b.appendChild(el('span', 'pg-action-desc', desc));
  b.addEventListener('click', onClick);
  return b;
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

// ---- run ----
async function startRun(mode) {
  setNav('home');
  app.replaceChildren();
  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'Running governed escrow'));
  head.appendChild(el('p', null, mode === 'kermit'
    ? 'Live against the Kermit testnet.'
    : 'Deterministic fixture-backed run — no wallet, no funding.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  const list = el('ul', 'pg-steps');
  panel.appendChild(list);
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

  // Render the step skeleton.
  const steps = (started.steps && started.steps.length) ? started.steps : skeletonSteps();
  const rowFor = {};
  for (const s of steps) {
    const li = el('li', 'pg-step');
    li.dataset.status = 'pending';
    li.appendChild(el('span', 'pg-step-icon', '○'));
    li.appendChild(el('span', 'pg-step-label', s.label));
    list.appendChild(li);
    rowFor[s.key] = li;
  }

  const src = new EventSource(`/api/runs/${encodeURIComponent(started.id)}/events`);
  src.onmessage = (ev) => {
    let e;
    try { e = JSON.parse(ev.data); } catch { return; }
    if (e.type === 'step' && e.step) {
      const li = rowFor[e.step.key];
      if (li) {
        li.dataset.status = e.step.status;
        li.querySelector('.pg-step-icon').textContent =
          e.step.status === 'complete' ? '✓' : e.step.status === 'failed' ? '✗' : '◔';
      }
    } else if (e.type === 'done' && e.receiptId) {
      src.close();
      telemetry.emit('demo.completed', { mode: mode === 'kermit' ? 'kermit' : 'local', result: 'success' });
      location.hash = `#/r/${e.receiptId}`;
    } else if (e.type === 'error') {
      src.close();
      telemetry.emit('error.step', { mode: mode === 'kermit' ? 'kermit' : 'local', result: 'failure' });
      showError(app, { message: e.error });
    }
  };
  src.onerror = () => { src.close(); };
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

  const split = el('div', 'pg-split');

  // Left: receipt card + actions.
  const left = el('div', 'pg-panel');
  left.appendChild(el('h2', null, 'Receipt'));
  const card = el('div');
  mountProofReceipt(card, data.receipt);
  left.appendChild(card);

  const actions = el('div', 'pg-receipt-actions');
  const dl = el('a', 'pg-btn', 'Download proof bundle');
  dl.href = `/api/receipts/${encodeURIComponent(id)}/bundle`;
  dl.setAttribute('download', `${id}.infrix.json`);
  actions.appendChild(dl);
  const verifyBtn = el('button', 'pg-btn pg-btn-primary', 'Verify it yourself');
  verifyBtn.type = 'button';
  actions.appendChild(verifyBtn);
  left.appendChild(actions);

  const share = el('div', 'pg-share');
  share.textContent = 'Share link: ' + location.origin + '/#/r/' + id;
  left.appendChild(share);

  const verifyOut = el('div');
  left.appendChild(verifyOut);
  split.appendChild(left);

  // Right: Cinema replay of the proof.
  const right = el('div', 'pg-panel');
  right.appendChild(el('h2', null, 'Cinema replay'));
  const cinemaHost = el('div', 'pg-cinema-host');
  right.appendChild(cinemaHost);
  split.appendChild(right);

  app.appendChild(split);

  // Load the bundle once, shared by both the verify button and the replay. The
  // verify handler is wired SYNCHRONOUSLY (no await before addEventListener) so
  // a click never races ahead of the listener.
  const pkgPromise = fetch(`/api/receipts/${encodeURIComponent(id)}/bundle`)
    .then((res) => (res.ok ? res.json() : null))
    .catch(() => null);

  verifyBtn.addEventListener('click', async () => {
    verifyOut.replaceChildren(el('div', 'pg-loading', 'Verifying in your browser (no node trusted)…'));
    const pkg = await pkgPromise;
    if (!pkg) {
      verifyOut.replaceChildren();
      showError(verifyOut, { message: 'Could not load the bundle to verify.' });
      return;
    }
    try {
      const result = await verifyPortablePackage(pkg);
      telemetry.emit('proof.verified', { result: result.passed ? 'success' : 'failure' });
      const browserReceipt = buildReceiptFromVerifier(result, {
        evidenceId: data.receipt && data.receipt.artifacts ? data.receipt.artifacts.evidenceId : '',
      });
      verifyOut.replaceChildren(el('div', 'pg-meta', result.passed
        ? 'Browser verification PASSED — verified without trusting this server.'
        : 'Browser verification did NOT pass.'));
      const c = el('div');
      mountProofReceipt(c, browserReceipt);
      verifyOut.appendChild(c);
    } catch (e) {
      verifyOut.replaceChildren();
      showError(verifyOut, e);
    }
  });

  // Mount the Cinema replay (proof mode) — node-independent, after the verify
  // handler is wired so its async load never blocks interaction.
  try {
    const pkg = await pkgPromise;
    const proof = bundleToCinemaProof(data.receipt, pkg);
    const { mountCinema } = await import('/cinema-core/loader.js');
    await mountCinema({ mode: 'cinema.proof', root: cinemaHost, proof });
    telemetry.emit('cinema.opened', { result: 'success' });
  } catch (e) {
    cinemaHost.appendChild(el('div', 'pg-note', 'Replay unavailable in this browser.'));
  }
}

// ---- verify (bring your own proof) ----
function renderVerify() {
  setNav('verify');
  app.replaceChildren();
  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'Verify a proof'));
  head.appendChild(el('p', null, 'Paste a portable proof bundle (or load the sample). Verification runs entirely in your browser — this server is never trusted.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  const ta = el('textarea', 'pg-verify-input');
  ta.setAttribute('aria-label', 'Portable proof bundle JSON');
  ta.placeholder = 'Paste a *.infrix.json portable evidence package here…';
  panel.appendChild(ta);

  const row = el('div', 'pg-verify-row');
  const verifyBtn = el('button', 'pg-btn pg-btn-primary', 'Verify in browser');
  verifyBtn.type = 'button';
  const sampleBtn = el('button', 'pg-btn', 'Load sample proof');
  sampleBtn.type = 'button';
  row.appendChild(verifyBtn);
  row.appendChild(sampleBtn);
  row.appendChild(el('span', 'pg-note', 'No login. No node trust.'));
  panel.appendChild(row);

  const out = el('div');
  panel.appendChild(out);
  app.appendChild(panel);

  sampleBtn.addEventListener('click', async () => {
    try {
      const res = await fetch('/api/sample-bundle');
      ta.value = await res.text();
    } catch (e) { showError(out, e); }
  });

  verifyBtn.addEventListener('click', async () => {
    out.replaceChildren();
    let pkg;
    try {
      pkg = JSON.parse(ta.value);
    } catch {
      showError(out, { message: 'That is not valid JSON. Paste a portable proof bundle.' });
      return;
    }
    out.appendChild(el('div', 'pg-loading', 'Verifying in your browser…'));
    try {
      const result = await verifyPortablePackage(pkg);
      const receipt = buildReceiptFromVerifier(result, {});
      out.replaceChildren(el('div', 'pg-meta', result.passed
        ? 'Verification PASSED — without trusting this server.'
        : 'Verification did NOT pass.'));
      const c = el('div');
      mountProofReceipt(c, receipt);
      out.appendChild(c);
    } catch (e) {
      out.replaceChildren();
      showError(out, e);
    }
  });
}

// ---- readiness ----
async function renderReadiness() {
  setNav('readiness');
  app.replaceChildren();
  const head = el('section', 'pg-hero');
  head.appendChild(el('h1', null, 'Instance readiness'));
  head.appendChild(el('p', null, 'What this playground instance can do right now.'));
  app.appendChild(head);

  const panel = el('div', 'pg-panel');
  app.appendChild(panel);
  try {
    const r = await api('/api/readiness');
    const dl = el('dl', 'pg-kv');
    const add = (k, v) => { dl.appendChild(el('dt', null, k)); dl.appendChild(el('dd', null, v)); };
    add('Anonymous demo', r.anonymous ? 'ready' : 'unavailable');
    add('Kermit sandbox', r.kermit ? 'enabled (live testnet)' : 'disabled');
    add('Browser verifier', r.verifier ? 'ready' : 'unavailable');
    add('Sample fixture', r.fixture ? 'ready' : 'unavailable');
    add('Mainnet writes', 'never');
    panel.appendChild(dl);
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

// ---- boot ----
async function boot() {
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
  if (hash === '/readiness') return renderReadiness();
  if (hash === '/metamask') return renderMetaMask();
  return renderHome();
}

boot();
