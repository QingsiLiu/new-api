# Goal: Minimal Design — Final Geometry Polish

**Context:** Production (`f3127b9 minimal-recon`) is deployed and healthy, but visual inspection reveals remaining geometry inconsistencies in:
1. **Config drawer** (`config-drawer.tsx`) — theme customization panel uses `rounded-md` (resolves to ~4.6px, not the semantic surface 6.08px or pill 9999px) on all option cards/buttons. Visual chaos in the density/sidebar/layout selector.
2. **Hero terminal traffic-light dots** container might still have non-surface radius around the red/yellow/green dots.
3. **Empty-state panels** in dashboard/pricing might still use inflated `rounded-xl`.

These are **low-leverage** (not on critical path), but they break visual cohesion when users open settings or land on empty states. Fix them to complete the geometry contract.

---

## Tasks

### T1: Config drawer option cards → surface or pill

**File:** `web/default/src/components/config-drawer.tsx`

The theme customization Sheet contains selector cards for density, sidebar, layout, content-layout, direction. Every option card currently uses `rounded-md` (`~4.6px` under the de-inflated `--radius`) which is neither pill nor surface. Lines affected: 177, 269, 277, 356, 432, 517, 658.

**Decision logic:**
- **Clickable option cards** (density tiles, sidebar mode tiles, layout tiles) → these are **interactive buttons**, so apply **pill** (`rounded-full` or `rounded-[var(--radius-pill)]`).
- **Inner content indicators** (the miniature layout preview boxes inside each tile, lines 277 `rounded-[calc(var(--radius)*0.75)]`) → these are **non-interactive decoration**, use **surface** (`rounded-[var(--radius-surface)]`).
- **Progress/separator bars** (lines 473, 690, 697-698 `rounded-sm`) → keep as-is (tiny decorative bars, `rounded-sm` is fine).

**Changes:**
```tsx
// Line 177 (density option outer ring) — interactive tile
'ring-border relative rounded-full ring-[1px]',

// Line 269 (sidebar mode option outer ring)
'ring-border relative h-12 rounded-full ring-[1px] transition',

// Line 277 (inner layout preview box — non-interactive decoration)
className='absolute inset-1 flex overflow-hidden rounded-[var(--radius-surface)]'

// Repeat for lines 356, 432, 517, 658 — all option outer rings → `rounded-full`
```

**Result:** Density/sidebar/layout tiles become pill buttons (consistent with all other interactive UI); inner preview decorations use surface radius.

---

### T2: Hero terminal traffic-light dots — verify or fix container

**File:** `web/default/src/features/home/components/hero-terminal-demo.tsx`

The traffic-light dots (`red-dot`, `yellow-dot`, `green-dot`) at line ~214-222 are wrapped in a container. The outer terminal panel already uses `rounded-[var(--radius-surface)]` (line 208 ✓), but if the dots sit inside a `<div className="...rounded-md...">`, that inner box creates a mismatch.

**Check:** Read lines 210-225. If there's a `<div>` wrapping the dots with `rounded-md` or `rounded-lg`, change to `rounded-[var(--radius-surface)]` or remove (dots themselves are `rounded-full`, so no wrapper radius is needed).

If no wrapper exists or it's already surface/transparent, skip this task.

---

### T3: Empty-state panels

**Files:**
- `web/default/src/components/empty-state.tsx`
- `web/default/src/features/pricing/components/empty-state.tsx`
- Any `features/dashboard/**/empty-state*.tsx` (search for them)

Empty-state panels are non-interactive content surfaces → should use **surface** (`6.08px`) or **card** (`24px`) if they're top-level.

**Search & fix:**
```bash
cd /Users/tedliu/code/new-api/web/default/src
grep -rn "rounded-\(lg\|xl\|2xl\)" components/empty-state.tsx features/*/components/empty-state*.tsx features/dashboard/components/**/empty-state*.tsx
```

For each match:
- If it's a top-level container meant to be a standalone card → `rounded-[var(--radius-card)]` (24px).
- If it's a content box inside another card → `rounded-[var(--radius-surface)]` (6.08px).
- If it's a small icon box → `rounded-[var(--radius-surface)]` (icon-square per design skill).

---

### T4: Calendar/date-picker cells (optional — only if visually broken)

The `Calendar` component from `components/ui/calendar.tsx` renders day cells. If those cells currently use `rounded-md` and look inconsistent, change to `rounded-[var(--radius-surface)]` for the cell button. This is **low-priority** — only fix if you see it's visually broken in screenshots or if grep shows `rounded-md` in `calendar.tsx`.

---

### T5: Build, commit, deploy

1. **Build** (must pass):
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/components/config-drawer.tsx web/default/src/features/home/components/hero-terminal-demo.tsx web/default/src/components/empty-state.tsx web/default/src/features/pricing/components/empty-state.tsx web/default/src/components/ui/calendar.tsx
   git commit -m "fix(web): final geometry polish for minimal design

   - config-drawer option tiles -> pill (rounded-full)
   - config-drawer inner previews -> surface (6.08px)
   - hero terminal traffic-light container -> surface (if needed)
   - empty-state panels -> surface or card
   - calendar cells -> surface (if touched)"
   git push origin main
   ```

3. **Deploy:**
   ```bash
   rsync -az --delete --exclude='.git' --exclude='node_modules' --exclude='web/default/dist' \
     /Users/tedliu/code/new-api/ ctbuk-443:/opt/geili-relay/new-api-src/

   ssh ctbuk-443 '
     set -e
     cd /opt/geili-relay
     TS=$(date -u +%Y%m%dT%H%M%SZ)
     TAG="geili/new-api:minimal-polish-${TS}"
     cp docker-compose.yml docker-compose.yml.before-minimal-polish-${TS}
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tee build-minimal-polish-${TS}.log
     sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml
     docker compose up -d relay-new-api
     sleep 8
     docker ps --format "{{.Names}} {{.Status}}" | grep relay-new-api
   '
   ```

4. **Verify:**
   ```bash
   ssh ctbuk-443 'docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api'
   curl -sI https://all.geiliapi.com | head -5
   ```
   Container must be `Up (healthy)`, not `Restarting`.

---

## Acceptance

- [ ] Theme config drawer option tiles are pill-shaped (`rounded-full`), inner previews are surface (`6.08px`).
- [ ] No visible `rounded-md` (~4.6px) orphan radii in settings or empty states.
- [ ] Traffic-light dot container (if any) uses surface radius or none.
- [ ] Empty-state panels use surface (6.08px) or card (24px) depending on hierarchy.
- [ ] Build succeeds, deploy completes, container is `Up`.

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`.
- `bun run build`, never pnpm.
- SSH via `ssh ctbuk-443` (port 443 tunnel).
- Deploy via `docker compose up -d`, never standalone `docker run`.
- Keep all data/status/brand colors intact.
