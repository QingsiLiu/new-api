# Goal: Minimal Design System — Component-Level Reconstruction

**Date:** 2026-07-06
**Repo:** `/Users/tedliu/code/new-api` (private fork `QingsiLiu/new-api` — NEVER push to `Calcium-Ion/new-api`)
**Frontend:** `web/default` (React 19 + Tailwind v4 + base-ui + CVA)
**Design skill source (read first):** `/Users/tedliu/Documents/GeiliAPI/codex-design-skill-files/` — all 27 `.md` files
**Build:** `bun run build` (NEVER pnpm)
**Deploy:** London server via `ssh ctbuk-443`, `/opt/geili-relay/`, docker compose

---

## Why this goal exists

The `geili-minimal` theme preset was applied as a **skin only** — colors changed, but component geometry was left inconsistent. The result is visually fractured: pill buttons sit next to square-cornered step cards, `rounded-md` rectangular status badges, and inflated `rounded-xl/2xl` panels. Screenshots confirm mixed square / rounded / pill geometry on the same page.

### Root causes (must fix all three)

1. **`--radius` inflation.** `geili-minimal` sets `--radius: 1.5rem` (24px). Every Tailwind derived radius (`rounded-md/lg/xl/2xl`) scales from `--radius`, so across ~200 feature files those tokens balloon to 18px / 24px / 33px / 43px. Plain `<div>`/`<Link>` elements (onboarding step cards, number chips, code panels, inner chips) become oversized blobs clashing with the 9999px pill controls.

2. **`status-badge.tsx` bypasses the badge override.** It emits `data-slot='status-badge'` (not `'badge'`), so the `[data-slot='badge'] → 9999px` CSS never reaches it. It renders `rounded-md`. **58 files** import it (all table status / group / type badges). This is the single highest-leverage fix.

3. **CSS-patch whack-a-mole.** Geometry was forced via `!important` overrides scoped to `[data-theme-preset='geili-minimal']`, which never covers checkboxes, modals, textareas, tooltips, dropdown items, nested feature panels, etc.

### Strategy: semantic radius tokens + source-level primitive rewrite

Stop patching. Introduce a **four-tier semantic radius system**, rewire the UI primitives to reference it directly, de-inflate the base `--radius` so derived feature-file radii collapse to a tasteful content scale, and null every shadow. This makes geometry deterministic and coherent instead of override-dependent.

---

## The geometry contract (from the design skill)

| Tier | Value | Applies to |
|------|-------|-----------|
| **pill** | `9999px` | buttons, badges, status-badges, inputs (single-line), alerts, pill tabs, sidebar nav rows, pagination items, dropdown/select/menu **items**, chips |
| **card** | `24px` | cards only (`[data-slot='card']`), top-level content cards |
| **surface** | `6.08px` | tables, modals/dialogs/sheets, popovers, dropdown/select/command **containers**, tooltips, code panels, textareas (multiline), icon-square containers, inner content chips |
| **control** | `4px` | checkboxes, tiny non-pill toggles |

**Shadows: none, everywhere.** Depth comes only from the `#F5F5F5`-on-`#FFFFFF` surface contrast and the 1px `#E5E7EB` hairline. The only allowed elevation is the modal backdrop scrim.

**Color:** `#000000` is the only filled accent. Status/data colors (`--success/--warning/--destructive/--info`, calm-color tokens, chart tokens) are retained **only** for data cells, sparklines, and provider/brand marks — never for UI chrome (buttons, badge fills, panel backgrounds).

---

## Tasks

Execute in order. After each phase, keep the app buildable.

### P1 — Semantic radius tokens + de-inflate base radius

**File: `web/default/src/styles/theme.css`**

In the `@theme inline` block (near the existing `--radius-sm/md/lg/xl` definitions), add four semantic tokens:

```css
  --radius-pill: 9999px;
  --radius-card: 1.5rem;      /* 24px */
  --radius-surface: 0.38rem;  /* ~6.08px */
  --radius-control: 0.25rem;  /* 4px */
```

**File: `web/default/src/styles/theme-presets.css`** — in the `[data-theme-preset='geili-minimal']` light block:

- Change `--radius: 1.5rem;` → `--radius: 0.38rem;` (6.08px).
  This de-inflates every derived Tailwind radius used in feature files: `rounded-md`≈4.6px, `rounded-lg`=6.08px, `rounded-xl`≈8.5px, `rounded-2xl`≈11px — a coherent content-surface scale instead of 18–43px blobs.
- Keep `--shadow-card: none;`.
- Cards keep 24px via the explicit `[data-slot='card']` rule (P4) and the `--radius-card` token, NOT via `--radius`.

Do the same `--radius: 0.38rem;` in the `.dark [data-theme-preset='geili-minimal']` block (it currently inherits — set it explicitly).

---

### P2 — Rewrite UI primitives to semantic tokens

Edit these files in `web/default/src/components/ui/`. Replace the derived/arbitrary radius classes with explicit semantic-token classes. Use Tailwind arbitrary-value syntax `rounded-[var(--radius-…)]`.

| File | Current | Change to |
|------|---------|-----------|
| `card.tsx` | `rounded-lg`, image `rounded-t-lg`/`rounded-b-lg` | `rounded-[var(--radius-card)]`, `rounded-t-[var(--radius-card)]`/`rounded-b-[var(--radius-card)]` |
| `button.tsx` | base already `rounded-full` ✓ | leave pill; ensure `xs`/`sm`/`icon-*` sizes are `rounded-full` (remove any `in-data-[slot=button-group]:rounded-lg` → `rounded-[var(--radius-surface)]`) |
| `badge.tsx` | `rounded-full` ✓ | leave |
| `input.tsx` | `rounded-full` ✓ | leave (single-line pill) |
| `textarea.tsx` | `rounded-[calc(var(--radius)*0.75)]` | `rounded-[var(--radius-surface)]` (multiline cannot be pill) |
| `alert.tsx` | `rounded-lg` | `rounded-[var(--radius-pill)]` |
| `checkbox.tsx` | `rounded-[calc(var(--radius)*0.45)]` | `rounded-[var(--radius-control)]` |
| `switch.tsx` | `rounded-full` ✓ | leave; remove thumb `shadow-sm` → `shadow-none` |
| `radio-group.tsx` | `rounded-full` ✓ | leave |
| `dialog.tsx` | content `rounded-lg` + `shadow-[var(--shadow-card)]` | `rounded-[var(--radius-surface)]`, `shadow-none` |
| `sheet.tsx` | `shadow-[var(--shadow-card)]` | `shadow-none`; inner corners `rounded-[var(--radius-surface)]` where a corner is exposed |
| `popover.tsx` | content `rounded-lg` + shadow | `rounded-[var(--radius-surface)]`, `shadow-none` |
| `dropdown-menu.tsx` | content `rounded-lg`(+sub) + shadow; item `rounded-md` | container → `rounded-[var(--radius-surface)]`, `shadow-none`; **items → `rounded-[var(--radius-pill)]`** |
| `select.tsx` | trigger `rounded-[calc(var(--radius)*0.75)]`; content `rounded-lg`; item `rounded-md`; shadow | trigger → `rounded-[var(--radius-pill)]`; content → `rounded-[var(--radius-surface)]` + `shadow-none`; item → `rounded-[var(--radius-pill)]` |
| `command.tsx` | root `rounded-xl!`; item `rounded-md` | root → `rounded-[var(--radius-surface)]`; item → `rounded-[var(--radius-pill)]` |
| `tooltip.tsx` | content `rounded-md` + shadow | `rounded-[var(--radius-pill)]` (short) — if it wraps multiline rich content use `rounded-[var(--radius-surface)]`; `shadow-none` |
| `table.tsx` | container has no radius | container/wrapper → `rounded-[var(--radius-surface)]`, `overflow-hidden`, 1px `border-border` |
| `tabs.tsx` | list `rounded-lg`; trigger `rounded-md` + active `shadow-[var(--shadow-card)]` | list → `rounded-[var(--radius-pill)]`; trigger → `rounded-[var(--radius-pill)]`; active `shadow-none` |
| `input-otp.tsx` | container `rounded-lg`; slot `first/last:rounded-l/r-lg` | `rounded-[var(--radius-surface)]` variants |
| `sidebar.tsx` | menu-button/-action/-sub-button `rounded-md`; floating/inset `rounded-lg` + shadow | nav rows (`menu-button`, `menu-action`, `menu-sub-button`, `menu-badge`, `trigger`) → `rounded-[var(--radius-pill)]`; floating/inset containers → `rounded-[var(--radius-card)]`, `shadow-none`; active bar `before:rounded-sm` → keep |
| `pagination.tsx` | via buttonVariants | ensure items resolve to pill (they use Button → already pill) |
| `avatar.tsx` | `rounded-full` ✓ | leave |
| `skeleton.tsx` | `rounded-[calc(var(--radius)*0.75)]` | `rounded-[var(--radius-surface)]` |

**Rule:** wherever you find `shadow-[var(--shadow-card)]`, `shadow-sm`, `shadow-md`, `shadow-lg`, `shadow-xl` on a UI primitive, replace with `shadow-none`. The `--shadow-card` variable is already `none` in the preset, but remove the classes at source so other presets don't reintroduce shadows on these rewritten primitives — EXCEPT leave non-minimal presets visually intact by keeping `--shadow-card` defined in `:root`/other presets (only the geili-minimal preset nulls it). Since the class `shadow-[var(--shadow-card)]` resolves to `none` under geili-minimal automatically, you MAY keep it; but for the components explicitly listed above (dialog, popover, dropdown, select, tooltip, sheet, tabs active, switch thumb, sidebar containers) set `shadow-none` unconditionally so the flat look is guaranteed.

---

### P3 — Fix `status-badge.tsx` (critical, 58 dependents)

**File: `web/default/src/components/status-badge.tsx`**

- Change the badge shell radius `rounded-md` → `rounded-[var(--radius-pill)]` (or `rounded-full`).
- Ensure horizontal padding is pill-appropriate: at least `px-2.5` so short labels (`0.3x`, `GPT`) still read as pills, not circles-with-text.
- The status dot: keep it a circle (`rounded-full`); if it currently uses `rounded-sm`, change to `rounded-full`.
- Do NOT change the color logic — status/group colors (calm-color tokens) are data identity and must be preserved. Only the geometry changes.

**File: `web/default/src/components/data-table/core/badge-cell.tsx`** — verify it inherits geometry from the wrapped `Badge`/`StatusBadge`; no radius should be hardcoded here. If it is, use `rounded-[var(--radius-pill)]`.

---

### P4 — Card & surface guarantees in index.css

**File: `web/default/src/styles/index.css`** — in the existing `geili-minimal` scoped block, ensure/adjust:

- `[data-slot='card']` → `border-radius: var(--radius-card); border: 1px solid var(--border); box-shadow: none;` (light + dark). Remove the hardcoded `24px`/`#e5e7eb`/`#1f1f1f` literals in favor of tokens.
- `[data-slot='sidebar-inset'] [data-slot='card'], .data-table-card` → `border-radius: var(--radius-surface); box-shadow: none;`
- Add a **global shadow sweep** for the preset:
  ```css
  [data-theme-preset='geili-minimal'] [class*='shadow-'] { box-shadow: none !important; }
  ```
  (Safe because the design is intentionally flat. Place it AFTER other rules.)
- Remove now-redundant `!important` radius patches that the P2 source rewrites make unnecessary (button/input/badge/alert/dropdown-item/select radius rules) — keep only what still adds value (card, sidebar rail border, data-table-card surface radius). Leave the badge **color** normalization if present, but note status-badge colors come from calm tokens.

---

### P5 — Flatten nested feature panels

These feature files hardcode inflated radii and nested borders that create the "card-in-card" clutter seen in the onboarding/overview screenshots. With `--radius` de-inflated they shrink automatically, but also remove the redundant nested borders and align to the surface tone.

**File: `web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`**
- Onboarding step list container (`<ol>`): `rounded-2xl border bg-background/45` → `rounded-[var(--radius-card)] border-border bg-transparent` (let the outer card provide the surface; drop the extra tint).
- `StartStepItem` number/status chip (`size-8 rounded-lg border`): → `rounded-[var(--radius-surface)] border-border` (6.08px icon-square, no heavy border) — or make it borderless on `bg-muted`.
- `StartStepItem` row `Link` (`rounded-xl border px-3 py-2.5`): → `rounded-[var(--radius-surface)]`; drop the per-row border, use `hover:bg-muted` for affordance (rows inside a card should not each carry a border).
- Inner icon chip (`size-7 rounded-lg bg-muted`): → `rounded-[var(--radius-surface)]`.
- `QuickActionItem` (recommended action cards) `rounded-xl` + inner `rounded-lg`: → outer `rounded-[var(--radius-surface)]`, inner icon `rounded-[var(--radius-surface)]`; these are list rows in a panel — no individual heavy borders, hover tint only.
- RequestPreview / code panel `rounded-xl` inner block: → `rounded-[var(--radius-surface)]`; traffic-light dots stay `rounded-full`.

**File: `web/default/src/features/dashboard/components/overview/summary-cards.tsx`** — card `rounded-xl border` → `rounded-[var(--radius-card)] border-border`; sub-cells `rounded-lg border` → `rounded-[var(--radius-surface)]` borderless-on-muted.

**File: `web/default/src/features/dashboard/components/overview/performance-health-panel.tsx`** — `rounded-2xl border shadow-xs` → `rounded-[var(--radius-card)] border-border shadow-none`; sub-panel `rounded-xl` → `rounded-[var(--radius-surface)]`.

**File: `web/default/src/features/dashboard/components/ui/panel-wrapper.tsx`** — `rounded-2xl border shadow-xs` → `rounded-[var(--radius-card)] border-border shadow-none`.

**File: `web/default/src/features/dashboard/components/ui/stat-card.tsx`** — detail chips `rounded-lg border` → `rounded-[var(--radius-surface)]` borderless-on-muted.

**File: `web/default/src/features/keys/components/api-keys-table.tsx`** — table wrapper `rounded-lg border` → `rounded-[var(--radius-surface)] border-border`; empty state + skeleton to surface radius.

**File: `web/default/src/features/home/components/hero-terminal-demo.tsx`** — code panel `rounded-xl` → `rounded-[var(--radius-surface)]`; label pill stays pill.

**File: `web/default/src/features/home/components/icon-card.tsx`** and **`sections/how-it-works.tsx`** — `rounded-2xl` icon boxes → `rounded-[var(--radius-surface)]` (6.08px icon-square per skill); step number badge stays `rounded-full`.

General rule for P5: **inside a card/panel, child rows and chips must not each carry their own 1px border** — use the shared surface tone (`bg-muted`) + hover tint for separation. Reserve borders for the outermost card only. This kills the "box-in-box" fracture.

---

### P6 — Typography polish

**File: `web/default/src/styles/index.css`** (`@layer base`)

- Display headings tracking: add `letter-spacing: -0.03em;` to `:is(h1, h2, h3, h4)`, `[data-slot='card-title']`, `[data-slot='dialog-title']`. Do NOT apply below 22px (leave h5/h6, body, labels at 0).
- Table head: confirm `[data-slot='table-head']` renders 12px, weight 500, `text-transform: uppercase`, `letter-spacing: 0.011em`, `color: var(--muted-foreground)`. Add if missing.
- Body/label tracking stays 0.

Font: the skill specifies Open Sans; the app ships Inter locally. **Keep Inter** (humanist sans, functionally equivalent) — do not add a webfont dependency. Note this deviation in the commit message.

---

### P7 — Build, commit, deploy (SAFE procedure)

1. **Build** (must be zero-error before proceeding):
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push** to the private fork only:
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src
   git commit -m "feat(web): component-level minimal design reconstruction

   - semantic radius tokens (pill/card/surface/control); de-inflate --radius to 6.08px
   - rewrite ui primitives to semantic tokens (card/dialog/popover/dropdown/select/
     command/tooltip/table/tabs/sidebar/checkbox/textarea/alert/otp/skeleton)
   - status-badge.tsx -> pill (fixes 58 table badge consumers)
   - flatten nested feature panels (overview/summary/stat/keys/home), remove box-in-box borders
   - global shadow sweep for geili-minimal; heading -0.03em tracking
   - font stays Inter (Open Sans-equivalent), no webfont added"
   git push origin main
   ```
   **NEVER** push to `Calcium-Ion/new-api`.

3. **Deploy to London** — build the image ON the server and restart **via docker compose** (NOT `docker run`; standalone `docker run` misses the compose network and crash-loops on `lookup relay-postgres ... connection refused`):
   ```bash
   # sync source (new-api-src is NOT a git repo on the server)
   rsync -az --delete \
     --exclude='.git' --exclude='node_modules' --exclude='web/default/dist' \
     /Users/tedliu/code/new-api/ ctbuk-443:/opt/geili-relay/new-api-src/

   ssh ctbuk-443 '
     set -e
     cd /opt/geili-relay
     TS=$(date -u +%Y%m%dT%H%M%SZ)
     SHA=$(git -C /Users/tedliu/code/new-api rev-parse --short HEAD 2>/dev/null || echo local)
     TAG="geili/new-api:minimal-recon-${TS}"
     cp docker-compose.yml docker-compose.yml.before-minimal-recon-${TS}
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tee build-minimal-recon-${TS}.log
     sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml
     docker compose up -d relay-new-api
     sleep 8
     docker ps --format "{{.Names}} {{.Status}}" | grep relay-new-api
     docker logs relay-new-api --tail 20
   '
   ```

4. **Verify** the container is `Up` (not `Restarting`) and the site responds:
   ```bash
   ssh ctbuk-443 'docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api'
   curl -sI https://all.geiliapi.com | head -5
   ```
   If the container is `Restarting`, read `docker logs relay-new-api`, and if it is a networking/DB error, confirm it was started via `docker compose` (not `docker run`). Roll back by pointing the compose `image:` back to `geili/new-api:calm-color-20260705T123458Z-dd2ed65` and `docker compose up -d relay-new-api`.

---

## Acceptance criteria

- [ ] Every button, badge, **status-badge**, single-line input, alert, pill tab, sidebar nav row, pagination item, and dropdown/select/menu **item** is a 9999px pill.
- [ ] Every card is 24px; every table / modal / popover / dropdown container / tooltip / code panel / icon-square / textarea is 6.08px; checkboxes are 4px. No other radii appear.
- [ ] No `rounded-md`/`rounded-lg`/`rounded-xl`/`rounded-2xl` visually renders as an oversized blob (base `--radius` is 6.08px; derived tokens are ≤11px).
- [ ] Zero box-shadows on any card, button, input, panel, modal, dropdown, sidebar, or table in geili-minimal.
- [ ] No "box-in-box": rows/chips inside a card use surface tone + hover tint, not individual borders.
- [ ] Status/group badges in tables (Gemini / Claude / 已启用 / 0.3x) render as pills with their data colors intact.
- [ ] Display headings (h1–h4) carry `-0.03em` tracking; table heads are uppercase 12px `#8F8F8F`.
- [ ] Dark mode: `#000000` canvas, `#141414` cards, `#1F1F1F` hairlines, no shadows.
- [ ] `bun run build` succeeds with zero errors.
- [ ] `relay-new-api` container is `Up` (not `Restarting`); `https://all.geiliapi.com` loads and admin login works.

---

## Constraints (hard)

- Push ONLY to `QingsiLiu/new-api`. NEVER `Calcium-Ion/new-api`.
- `bun run build`, never pnpm.
- SSH via `ssh ctbuk-443` (port 22 is blocked outbound; 443 tunnel required).
- Deploy via `docker compose up -d relay-new-api`, never standalone `docker run`.
- Do not print or commit `.env`, tokens, or secrets.
- Preserve all data/status/brand colors (calm-color tokens, chart tokens, provider/payment hex) — only geometry and shadows change on chrome.
- Keep other theme presets (geili-modern, editorial, anthropic, etc.) visually intact; all changes are either token-additive or scoped to `geili-minimal` / shared primitives whose radius now reads from semantic tokens.
