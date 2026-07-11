# Goal: Model Analytics Dashboard — Borderless 24px Cards

**Current issue:** Model analytics dashboard page (`/dashboard` 数据看板) has inconsistent styling:
- Top stat cards have borders and small radius (6.08px surface)
- Chart cards have borders and small radius
- Performance overview cards have borders
- Not matching the borderless 24px style of the overview page

**Target:** All cards use **24px radius, no border, #F5F5F5 background** floating on #FAFAFA page, matching overview page style.

---

## Files to Fix

### File 1: `log-stat-cards.tsx`
**Path:** `/Users/tedliu/code/new-api/web/default/src/features/dashboard/components/models/log-stat-cards.tsx`

**Line 115:**
```tsx
// Before
<div className='overflow-hidden rounded-[var(--radius-surface)] border'>

// After
<div className='overflow-hidden rounded-[var(--radius-card)]'>
```

Remove `border` class, change `--radius-surface` (6.08px) to `--radius-card` (24px).

---

### File 2: `consumption-distribution-chart.tsx`
**Path:** `/Users/tedliu/code/new-api/web/default/src/features/dashboard/components/models/consumption-distribution-chart.tsx`

**Line 135:**
```tsx
// Before
<div className='overflow-hidden rounded-[var(--radius-surface)] border'>

// After
<div className='overflow-hidden rounded-[var(--radius-card)]'>
```

Remove `border` class, change to card radius.

**Also check line 136:** if there's a `border-b` on the header row, keep it (internal divider between header and chart content is OK, only remove the outer border).

---

### File 3: `performance-overview.tsx`
**Path:** `/Users/tedliu/code/new-api/web/default/src/features/dashboard/components/models/performance-overview.tsx`

**Line 116 (empty state):**
```tsx
// Before
<div className='text-muted-foreground overflow-hidden rounded-[var(--radius-surface)] border px-4 py-3 text-center text-xs'>

// After
<div className='text-muted-foreground overflow-hidden rounded-[var(--radius-card)] px-4 py-3 text-center text-xs'>
```

**Line 123 (performance card):**
```tsx
// Before
<div className='overflow-hidden rounded-[var(--radius-surface)] border'>

// After
<div className='overflow-hidden rounded-[var(--radius-card)]'>
```

Remove both `border` classes, change to card radius.

---

## Build, Commit, Deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/dashboard/components/models/
   git commit -m "fix(web): model analytics dashboard borderless 24px cards

   - log-stat-cards: 24px radius, no border
   - consumption-distribution-chart: 24px radius, no border
   - performance-overview: 24px radius, no border
   - matches overview page style (borderless cards floating on page)"
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
     TAG="geili/new-api:dashboard-borderless-${TS}"
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tail -5
     sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml
     docker compose up -d relay-new-api
     sleep 8
     docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api
   '
   ```

4. **Verify:**
   ```bash
   ssh ctbuk-443 'docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api'
   ```

---

## Acceptance

- [ ] Top 5 stat cards: 24px radius, no border
- [ ] Cost Distribution chart card: 24px radius, no border
- [ ] Performance overview cards: 24px radius, no border
- [ ] "暂无性能数据" empty state: 24px radius, no border
- [ ] All cards have #F5F5F5 background floating on #FAFAFA page
- [ ] Model analytics dashboard visually matches overview page style
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Keep internal borders (like `border-b` between header and chart) — only remove outer card borders
