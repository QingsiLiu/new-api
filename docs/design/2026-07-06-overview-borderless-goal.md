# Goal: Overview Page — Unified Borderless Cards

**Current issue:** Overview page (`/dashboard` or `/`) has inconsistent borders and radii across floating panels. Some cards have 1px borders (wrong), some don't. Radii vary.

**Target:** All top-level content cards use **24px radius, no border, #F5F5F5 background** floating on #FAFAFA page. Internal rows use 6.08px surface or 9999px pill. Pure surface-contrast depth.

---

## Analysis from Screenshot

### Cards on overview page:
1. **"开始使用" (Getting Started)** — large left card with 3 step items
2. **"推荐操作" (Recommended Actions)** — right sidebar card with action buttons
3. **"用量概览" (Usage Overview)** — 3 stat cards in a row
4. **"性能健康" (Performance Health)** — bottom-left card
5. **"运行时间" (Uptime)** — bottom-right card

### Current state:
- Getting Started: **has border** ❌ (should be borderless)
- Recommended Actions: **has border** ❌
- Usage stat cards: borderless ✓
- Performance/Uptime: **have borders** ❌

All should be **borderless**, relying on #F5F5F5 background on #FAFAFA page for depth.

---

## Tasks

### T1: Remove borders from all overview cards

**File:** `/Users/tedliu/code/new-api/web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`

This file renders the overview page layout. Find all card/panel wrappers and remove `border` classes.

**Search patterns:**
- `border` (any Tailwind border class: `border`, `border-border`, `border-default`)
- `ring-1` or `ring-[1px]` (sometimes used for borders)

**For each top-level card container, change:**
```tsx
// Before
<Card className="border border-border ...">

// After
<Card className="...">  // no border class at all
```

**Or if using a custom div:**
```tsx
// Before
<div className="rounded-2xl border border-border bg-card ...">

// After
<div className="rounded-[1.5rem] bg-card ...">  // 1.5rem = 24px
```

**Files to check:**
- `overview-dashboard.tsx` (main layout)
- `features/dashboard/components/overview/summary-cards.tsx` (usage stat cards)
- `features/dashboard/components/overview/performance-health-panel.tsx`
- `features/dashboard/components/ui/panel-wrapper.tsx` (if this is used as a wrapper)
- Any `QuickActionCard` or `RecommendedActionCard` component

---

### T2: Ensure all top-level cards use 24px radius

**Rule:** Every floating panel on the overview page should use `rounded-[var(--radius-card)]` or `rounded-[1.5rem]` (24px).

**Check each card:** If you see `rounded-xl` (which now resolves to ~8.5px under the de-inflated `--radius`), change to `rounded-[var(--radius-card)]` or `rounded-[1.5rem]`.

**Inner rows/items** (the step 1/2/3 rows, recommended action buttons) should keep `rounded-[var(--radius-surface)]` (6.08px) or `rounded-full` if they're interactive buttons.

---

### T3: Ensure card background is #F5F5F5

All cards should use `bg-card` which resolves to `#F5F5F5` in geili-minimal. If any card uses `bg-background` or `bg-white`, change to `bg-card`.

**Check:** `className="bg-card"` on every top-level card.

---

### T4: Verify page background is #FAFAFA

The main content area (`<SidebarInset>` or the wrapper around the overview grid) should have `bg-background` which now resolves to `#FAFAFA`.

This creates the subtle depth: cards (#F5F5F5) float on page (#FAFAFA).

---

### T5: Dark mode check

In dark mode:
- Page background: `#0a0a0a` (from `--background` in dark theme)
- Cards: `#141414` (from `--card` in dark theme)
- No borders in dark mode either

If the dark theme doesn't have these values set in `theme-presets.css`, add them:
```css
.dark [data-theme-preset='geili-minimal'] {
  --background: #0a0a0a;  /* page */
  --card: #141414;        /* cards */
}
```

---

### T6: Build, commit, deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/dashboard web/default/src/styles/theme-presets.css
   git commit -m "fix(web): overview page borderless cards, unified 24px radius

   - remove borders from all overview panels (getting-started, recommended-actions, usage-stats, performance, uptime)
   - ensure all top-level cards use 24px radius (rounded-[var(--radius-card)])
   - cards float on #FAFAFA page via #F5F5F5 bg only (no borders)
   - dark mode: #141414 cards on #0a0a0a page"
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
     TAG="geili/new-api:overview-borderless-${TS}"
     cp docker-compose.yml docker-compose.yml.before-overview-borderless-${TS}
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tee build-overview-borderless-${TS}.log
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

---

## Acceptance

- [ ] All overview page cards have no borders (no `border`, `ring-1`, or `border-border` classes)
- [ ] All top-level cards use 24px radius (`rounded-[var(--radius-card)]` or `rounded-[1.5rem]`)
- [ ] All cards use `bg-card` (#F5F5F5 in light, #141414 in dark)
- [ ] Page background is `bg-background` (#FAFAFA in light, #0a0a0a in dark)
- [ ] Inner step rows use 6.08px surface radius, buttons use 9999px pill
- [ ] Dark mode: same borderless treatment with darker tones
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Do NOT remove table borders or input borders — only card/panel chrome borders
