# Goal: Model Analytics Dashboard — Complete Visual Audit

**Issue:** Previous fix only covered top cards. Bottom cards still have borders, and buttons/tabs across the page have inconsistent radius. Need a **complete page-wide audit and fix**.

**Target:** Every card, button, tab, and control on the model analytics dashboard page must follow the design system:
- Cards → 24px radius, no border
- Buttons → 9999px pill
- Tabs → 9999px pill
- Badge/chip → 9999px pill

---

## Comprehensive Audit & Fix

### Step 1: Find all remaining cards with borders

**Search command:**
```bash
cd /Users/tedliu/code/new-api/web/default/src/features/dashboard/components/models
grep -rn "border\|rounded-\[" . --include="*.tsx" | grep -v "border-b\|border-t\|border-l\|border-r\|border-x\|border-y" | grep "className"
```

This finds all `border` (without directional suffix) and `rounded-[` classes.

**For each match:**
- If it's a **card/panel container** → remove `border` class, change radius to `rounded-[var(--radius-card)]` (24px)
- If it's a **button** → change to `rounded-[var(--radius-pill)]` or `rounded-full`
- If it's a **tab** → change to `rounded-[var(--radius-pill)]` or `rounded-full`
- If it's a **badge/chip** → change to `rounded-[var(--radius-pill)]` or `rounded-full`

### Step 2: Check model-charts.tsx specifically

**File:** `model-charts.tsx`

This file renders the bottom "模型调用分析" card with trend/distribution/ranking tabs and chart type toggle buttons.

**Expected issues:**
- Main chart card container likely has `border` → remove
- Main chart card likely uses `rounded-[var(--radius-surface)]` → change to `rounded-[var(--radius-card)]`
- Tab buttons (调用趋势/调用次数分布/调用次数排行) might use `rounded-md` or `rounded-lg` → change to `rounded-[var(--radius-pill)]` or `rounded-full`
- Chart type toggle (柱状图/面积图) might use `rounded-md` → change to `rounded-[var(--radius-pill)]`

**Search for:**
```bash
grep -n "border\|rounded-" model-charts.tsx
```

Fix every non-pill button and every card container.

### Step 3: Check tab navigation ("模型调用分析/用户统计")

These tabs are likely in the **main page layout** or a wrapper component. They should be pill-shaped.

**Search:**
```bash
cd /Users/tedliu/code/new-api/web/default/src/features/dashboard
find . -name "*.tsx" -exec grep -l "模型调用分析\|用户统计" {} \;
```

Find the tab component rendering those labels, ensure it uses `rounded-[var(--radius-pill)]` or `rounded-full`.

### Step 4: Check filter/settings buttons ("偏好设置/筛选")

These are in `models-filter-dialog.tsx` or `models-chart-preferences.tsx` trigger buttons.

**Files:**
- `models-filter-dialog.tsx`
- `models-chart-preferences.tsx`

**Search for trigger buttons:**
```bash
grep -n "Button\|rounded-" models-filter-dialog.tsx models-chart-preferences.tsx
```

Ensure all buttons use `rounded-[var(--radius-pill)]` or inherit from `Button` component (which should already be pill).

If they override with `className="... rounded-md ..."`, remove the override.

### Step 5: Audit consumption-distribution-chart.tsx deeper

**Line 145** has a tab container:
```tsx
<div className='bg-muted/60 inline-flex h-7 w-full overflow-x-auto rounded-[var(--radius-pill)] border p-0.5 sm:h-8 sm:w-auto'>
```

This container should be pill ✓ but it has `border` — remove the `border` class.

**Line 153** has tab items that should be pill — verify they use `rounded-[var(--radius-pill)]`.

### Step 6: Build, commit, deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/dashboard/components/models/
   git commit -m "fix(web): model analytics dashboard complete visual audit

   - all remaining cards: 24px radius, no border
   - all buttons: 9999px pill
   - all tabs: 9999px pill
   - chart type toggles: 9999px pill
   - remove borders from tab containers
   - comprehensive page-wide consistency"
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
     TAG="geili/new-api:dashboard-complete-${TS}"
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tail -5
     sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml
     docker compose up -d relay-new-api
     sleep 8
     docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api
   '
   ```

---

## Acceptance

- [ ] All cards on page: 24px radius, no border (top stats, Cost Distribution, 模型调用分析)
- [ ] "模型调用分析/用户统计" tabs: 9999px pill
- [ ] "偏好设置/筛选" buttons: 9999px pill
- [ ] "调用趋势/调用次数分布/调用次数排行" tabs: 9999px pill
- [ ] "柱状图/面积图" toggle: 9999px pill
- [ ] Chart legend chips: 9999px pill (if applicable)
- [ ] No `rounded-md`, `rounded-lg`, `rounded-xl` orphan classes on interactive elements
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Be exhaustive — check EVERY file in `features/dashboard/components/models/`
