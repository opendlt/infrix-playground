// Infrix Playground — the Run Theater spine (RB-03).
//
// A horizontal flow that assembles left-to-right as the run streams: each node
// goes pending → running → complete (or failed), fills its connector with its
// stage colour (the shared 7-stop Spine Aurora gradient), and carries the real
// artifact hash of the stage it represents. On completion the whole spine plays
// a "seal" beat — the payoff moment — before the page hands off to the verdict.
//
// Also usable in an ambient autoplay mode (RB-05 landing). Served at /spine.js
// (the /components URL prefix is owned by the shared Nexus handler, so playground
// modules live at the root, embedded in web.go).

function el(tag, cls) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  return n;
}

function reduceMotion() {
  try { return window.matchMedia('(prefers-reduced-motion: reduce)').matches; } catch { return false; }
}

// --- stage colour: map a step key to a colour from the Spine Aurora gradient ---
// intent→spine-1 … outcome→spine-5, anchor→spine-7; approval/credential are
// interpolated between adjacent stops; export/verify are meta (accent / text).
const STAGE_STOPS = {
  intent: ['--spine-1'],
  plan: ['--spine-2'],
  policy: ['--spine-3'],
  approval: ['--spine-3', '--spine-4', 0.5],
  credential: ['--spine-4', '--spine-5', 0.5],
  outcome: ['--spine-5'],
  anchor: ['--spine-7'],
  export: ['--accent'],
  verify: ['--text'],
};

const colorCache = new Map();

function cssVar(name) {
  try { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); } catch { return ''; }
}
function hexToRgb(h) {
  h = (h || '').replace('#', '').trim();
  if (h.length === 3) h = h.split('').map((c) => c + c).join('');
  if (h.length < 6) return null;
  const n = parseInt(h.slice(0, 6), 16);
  if (Number.isNaN(n)) return null;
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
function mix(a, b, t) { return a.map((v, i) => Math.round(v + (b[i] - v) * t)); }

/**
 * stageColor returns a concrete CSS colour for a step key, sampled (and where
 * needed interpolated) from the theme's spine gradient. Cached per theme so a
 * theme switch re-resolves. Falls back to var(--accent) for unknown keys.
 */
export function stageColor(key) {
  const theme = (document.documentElement && document.documentElement.dataset && document.documentElement.dataset.theme) || 'dark';
  const ck = theme + ':' + key;
  if (colorCache.has(ck)) return colorCache.get(ck);
  const stop = STAGE_STOPS[key];
  let out = 'var(--accent)';
  if (stop) {
    const a = hexToRgb(cssVar(stop[0]));
    if (a && stop.length >= 3) {
      const b = hexToRgb(cssVar(stop[1]));
      out = b ? `rgb(${mix(a, b, stop[2]).join(',')})` : `rgb(${a.join(',')})`;
    } else if (a) {
      out = `rgb(${a.join(',')})`;
    } else {
      out = cssVar(stop[0]) || 'var(--accent)';
    }
  }
  colorCache.set(ck, out);
  return out;
}

/**
 * mountSpine builds the spine into container and returns a handle.
 * @param {HTMLElement|null} container
 * @param {Array<{key:string,label:string,color?:string}>} steps
 * @param {{reduce?:boolean}} [opts]  reduce overrides prefers-reduced-motion (tests)
 * @returns {{ el:HTMLElement, set:Function, seal:Function, node:Function, keys:Function }}
 */
export function mountSpine(container, steps, opts = {}) {
  const reduce = opts.reduce != null ? opts.reduce : reduceMotion();
  const wrap = el('div', 'pg-spine');
  wrap.setAttribute('role', 'list');
  const nodes = {};
  steps.forEach((s) => {
    const node = el('div', 'pg-spine-node');
    node.setAttribute('role', 'listitem');
    node.dataset.state = 'pending';
    node.dataset.key = s.key;
    if (node.style && node.style.setProperty) node.style.setProperty('--node-color', s.color || 'var(--accent)');
    const dot = el('span', 'pg-spine-dot');
    const label = el('span', 'pg-spine-label');
    label.textContent = s.label;
    const hash = el('code', 'pg-spine-hash'); // filled when a real hash arrives
    node.appendChild(dot);
    node.appendChild(label);
    node.appendChild(hash);
    wrap.appendChild(node);
    nodes[s.key] = { node, dot, hash };
  });
  if (container) container.replaceChildren(wrap);

  function set(key, state, hashText) {
    const n = nodes[key];
    if (!n) return;
    n.node.dataset.state = state;
    if (hashText) n.hash.textContent = hashText;
    if (reduce) return;
    if (state === 'complete' && typeof n.dot.animate === 'function') {
      n.dot.animate(
        [{ transform: 'scale(1.4)' }, { transform: 'scale(1)' }],
        { duration: 220, easing: 'cubic-bezier(0.4,0,0.2,1)' },
      );
    } else if (state === 'failed' && typeof n.node.animate === 'function') {
      n.node.animate(
        [{ transform: 'translateX(-3px)' }, { transform: 'translateX(3px)' }, { transform: 'none' }],
        { duration: 180 },
      );
    }
  }

  function seal() {
    if (reduce) {
      if (wrap.classList) wrap.classList.add('pg-spine-sealed');
      return Promise.resolve();
    }
    if (wrap.classList) wrap.classList.add('pg-spine-sealing');
    return new Promise((res) => setTimeout(() => {
      if (wrap.classList) wrap.classList.add('pg-spine-sealed');
      res();
    }, 600));
  }

  return {
    el: wrap,
    set,
    seal,
    node: (k) => (nodes[k] ? nodes[k].node : null),
    keys: () => Object.keys(nodes),
  };
}
