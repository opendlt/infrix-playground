# RB-04 — Adopt the Spine Aurora Design System

**Goal:** stop looking templated. Replace GitHub-blue fallbacks with the real shared
tokens, set an intentional type scale, and wire the theme toggle that already exists
in the shared sheet.

**Surface:** global (`web/index.html`, `web/playground.css`, small `web/playground.js`).
**Backend changes:** none. **WoW:** ●●○ **Effort:** S (~1–2 days). **Depends on:** none.
**Do this first** — it's the foundation every other runbook renders on.

> **STATUS: DONE & VERIFIED.** Note the theme-name correction in "As built" — the
> real `data-theme` values are `dark`/`light`/`contrast`, not `daylight`/`phosphor`.

---

## 1. Why

`web/playground.css` ships on a full design system and ignores it (Finding 3):
GitHub-blue `var(--accent, #58a6ff)` fallbacks on 9 lines, a 1.7rem hero, flat cards,
no theme control despite three themes existing. The shared `/styles.css`
("Spine Aurora") is loaded first by `index.html`, so the tokens are always present.

## 2. Token reference (from shared `/styles.css`)

```
Surfaces   --bg #08090F  --surface #131724  --surface-alt #1B2031  --surface-hover #232940
Borders    --border #252B40  --border-bold #353D58  --border-strong #4A5478
Text       --text #ECEFFA  --text-secondary #9BA4C2  --text-dim #8B94BD
Accent     --accent #8F82FF  --accent-soft #2D2755  --accent-glow rgba(110,91,255,.32)
Status     --ok #4DE3B5/--ok-soft  --warn #FFC960/--warn-soft  --alert #FF6E92/--alert-soft  --info #6EAFFF/--info-soft
Spine      --spine-1..7 (violet→gold)  --spine-track #1F2438
Shadow     --shadow-sm/-md/-lg  --shadow-glow
Type       --font (Inter)  --mono (JetBrains Mono)
Radius     --radius 10px  --radius-sm 6px  --radius-xs 4px
Motion     --motion-fast 120ms cubic-bezier(.4,0,.2,1)
Themes     :root[data-theme="dark"|"light"|"contrast"]   ← ATTRIBUTE VALUES
           (display names: Aurora-Dark / Daylight / Phosphor — see "As built")
```

## 3. Tasks

### 3.1 Strip the fallbacks (`web/playground.css`)

Replace every `var(--token, #hex)` with `var(--token)`. The shared sheet always loads
first (`index.html:9`). Specifically remove the `#58a6ff`, `#0e1116`, `#161b22`,
`#21262d`, `#e6edf3`, `#8b949e`, `#0d1117`, `#46a758`, `#e5484d`, `#f5a623` fallbacks
and map any that don't have a direct token:
- `--success` → `--ok`; `--danger` → `--alert`; `--warning` → `--warn`;
  `--surface-alt` stays; `--text-dim` stays.

### 3.2 Type scale

```css
.pg-hero h1 { font-size: clamp(2.4rem, 5vw, 4rem); font-weight: 800;
  letter-spacing: -0.02em; line-height: 1.05; margin: 0 0 10px; }
.pg-hero p { font-size: 1.05rem; line-height: 1.6; color: var(--text-secondary);
  max-width: 60ch; }
.pg-eyebrow { font-size: 0.72rem; font-weight: 700; letter-spacing: 0.12em;
  text-transform: uppercase; color: var(--text-dim); }
/* hashes & ids already use var(--mono) in places — make it consistent */
```

### 3.3 Card + button polish

```css
.pg-action, .pg-panel { background: var(--surface); border: 1px solid var(--border);
  border-radius: var(--radius); }
.pg-action { transition: border-color var(--motion-fast), transform var(--motion-fast),
  box-shadow var(--motion-fast); }
.pg-action:hover { border-color: var(--border-bold); transform: translateY(-2px);
  box-shadow: var(--shadow-glow); }
.pg-btn-primary { background: var(--accent); color: var(--bg); border-color: var(--accent);
  font-weight: 700; }
.pg-btn-primary:hover { box-shadow: var(--shadow-glow); }
@media (prefers-reduced-motion: reduce) {
  .pg-action, .pg-action:hover { transition: none; transform: none; }
}
```

### 3.4 Brand mark

The header brand uses `▰` (`index.html:15`). Replace with a tiny inline SVG spine
(7 dots in the gradient) so the brand *is* the signature. Keep it crisp at 20px.

### 3.5 Theme toggle

Add to the header (`index.html`, in `.pg-header` after nav):
```html
<button id="pg-theme" class="pg-theme-btn" type="button" aria-label="Switch theme" title="Theme">☾</button>
```
In `web/playground.js`, before first render (in `boot()` before `route()`).
NOTE (as built): the real attribute values are `dark`/`light`/`contrast` — the
labels Dark/Daylight/Phosphor are display-only:
```js
const THEMES = ['dark', 'light', 'contrast'];          // real data-theme values
const GLYPH = { dark: '☾', light: '☀', contrast: '▮' };
function applyTheme(t) {
  document.documentElement.dataset.theme = t;
  const btn = document.getElementById('pg-theme');
  if (btn) btn.textContent = GLYPH[t];
  try { localStorage.setItem('pg:theme', t); } catch {}
}
function initTheme() {
  let t; try { t = localStorage.getItem('pg:theme'); } catch {}
  if (!THEMES.includes(t)) t = 'dark';
  applyTheme(t);
  const btn = document.getElementById('pg-theme');
  if (btn) btn.addEventListener('click', () => {
    const next = THEMES[(THEMES.indexOf(document.documentElement.dataset.theme) + 1) % THEMES.length];
    applyTheme(next);
  });
}
```
Call `initTheme()` at the very top of `boot()` (before any await) to avoid a flash.

```css
.pg-theme-btn { background: none; border: 1px solid var(--border); border-radius: var(--radius-sm);
  color: var(--text-dim); width: 30px; height: 30px; cursor: pointer; }
.pg-theme-btn:hover { color: var(--text); border-color: var(--border-bold); }
```

### 3.6 Focus + motion floor (global)

```css
:where(button, a, input, textarea, [tabindex]):focus-visible {
  outline: 2px solid var(--accent); outline-offset: 2px; border-radius: var(--radius-xs);
}
```
Audit: ensure existing hover transforms (`.pg-action`) are gated (done in 3.3).

## 4. Acceptance criteria

- [ ] No hard-coded color fallbacks remain in `playground.css` (`grep '#'` shows only
      comments / rgba in tokens). Verify: `grep -nE '#[0-9a-fA-F]{3,6}' web/playground.css`.
- [ ] Hero uses the clamp scale; eyebrows present where specified.
- [ ] Theme toggle cycles dark→daylight→phosphor, persists across reload, no flash.
- [ ] All three themes are legible (text contrast holds — the shared tokens are WCAG-AA tuned).
- [ ] Keyboard focus visible on every control.
- [ ] Reduced-motion honored.
- [ ] `node --test web/test/*.mjs` green (no behavioral change to logic).

## 5. Rollback

Pure CSS/markup. `git checkout web/playground.css web/index.html` and revert the
`initTheme` addition in `playground.js`.

---

## As built (the theme correction, and what shipped)

Implemented in full. One material correction surfaced during verification.

### The theme-name correction (caught by the contrast E2E)
The runbook's token reference (and §3.5) said the themes were
`data-theme="dark"|"daylight"|"phosphor"`. That was wrong — those are the design
system's **display names**. The shared `/styles.css` actually defines the attribute
**values** `dark`, **`light`**, and **`contrast`**:
- `dark` — Aurora-Dark (default).
- `light` — "Daylight" (a real light theme: `--bg #FAFBFD`, `--text #15192B`).
- `contrast` — "Phosphor" (terminal-green: `--bg #000`, `--text #C8FFD9`).

The first headless run exposed this: all three "themes" reported the *identical*
contrast (17.32) because `data-theme="daylight"`/`"phosphor"` matched nothing and
fell back to dark — the toggle was a visual no-op. Fixed: `playground.js` cycles
`['dark','light','contrast']` while showing the friendly labels Dark / Daylight /
Phosphor (and glyphs ☾ / ☀ / ▮). `initTheme` self-heals a stored value not in the
set (falls back to dark), so the old `'daylight'` key from a prior build is harmless.

### What shipped
1. **All colour fallbacks stripped** from `playground.css` — pure `var(--token)`
   throughout (verified: `grep -nE '#[0-9a-fA-F]{3,6}'` is empty; no `rgba(` or
   `var(--x, #…)` either). Legacy names mapped: `--success`→`--ok`,
   `--danger`→`--alert`, `--warning`→`--warn`. The one remaining nested var,
   `var(--node-color, var(--accent))`, is a dynamic per-element prop fallback, not a
   hard-coded colour.
2. **Type scale:** `.pg-hero h1` → `clamp(2.4rem, 5vw, 4rem)`/800/-0.02em;
   `.pg-hero p` 1.05rem/1.6; `.pg-eyebrow` defined once and reused.
3. **Card + button polish:** `.pg-action` hover = `--border-bold` + `--shadow-glow`
   + lift; `.pg-btn-primary` is now a solid accent button (`bg:--accent;
   color:--bg`).
4. **Brand mark:** the `▰` glyph is replaced by an inline SVG of **7 dots filled
   with `--spine-1…7`** (themed via `nth-child`) — the brand is the Spine signature.
5. **Theme toggle:** a header button cycles the three real themes, persists to
   `localStorage('pg:theme')`, and is wired at the top of `boot()` (before render).
6. **Global a11y/motion floor:** a `:where(...):focus-visible` ring on every
   control, and a single `prefers-reduced-motion` block gating all
   transitions/animations added across RB-01…RB-04.

## Verification log (all green)
- `grep -nE '#[0-9a-fA-F]{3,6}' web/playground.css` → **empty** (no hex fallbacks);
  no `rgba(` fallbacks either.
- `node --test web/test/*.mjs` → **24 pass** (no behavioural change).
- `go build ./... && go vet ./... && go test ./... -count=1` → **all pass**.
- **Headless browser (Edge + CDP):** brand mark renders **7 circles**; hero h1 uses
  the clamp scale (38.4px floor, vs the old 27.2px); the theme button cycles
  `dark → light → contrast` with glyphs `☾/☀/▮`; the choice **persists across a
  reload**; and the **`--text`-vs-`--bg` contrast is distinct and WCAG-AAA in every
  theme — dark 17.32, light 16.82, contrast 18.76** (proving the themes truly apply,
  the bug the first run caught).
