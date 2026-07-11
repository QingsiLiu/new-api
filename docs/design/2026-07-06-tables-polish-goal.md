# Goal: Table Filters & Row Dividers Polish

**Issue 1:** Filter toolbars above tables still have borders (should be borderless 24px cards)
**Issue 2:** Table row dividers (horizontal lines) go edge-to-edge, should have horizontal padding to "breathe"

---

## Fix 1: Remove borders from filter toolbars

**Files to check:**
- `usage-logs/components/common-logs-filter-bar.tsx`
- `usage-logs/components/task-logs-filter-bar.tsx`
- Any other filter/toolbar components that wrap the date picker and action buttons

**Search:**
```bash
cd /Users/tedliu/code/new-api/web/default/src/features/usage-logs/components
grep -rn "border" common-logs-filter-bar.tsx task-logs-filter-bar.tsx
```

**For each filter bar container:**
```tsx
// Before
<div className="... border ...">

// After
<div className="...">  // remove border class entirely
```

If the container uses `rounded-lg`, also change to `rounded-[var(--radius-card)]` for consistency.

---

## Fix 2: Add horizontal padding to row dividers

**Approach:** Table rows currently use `divide-y` or `border-b` which creates full-width lines. We need horizontal inset.

**Option A: Add padding to table container, negative margin to rows**
This is complex and fragile.

**Option B: Use `divide-x-0` + custom row borders with padding**
Also complex.

**Option C (Recommended): Add horizontal padding to the table body via CSS**

**File:** `/Users/tedliu/code/new-api/web/default/src/styles/index.css`

Add a scoped rule for `geili-minimal` theme:

```css
[data-theme-preset='geili-minimal'] [data-slot='table-body'] [data-slot='table-row'] {
  border-left: 1rem solid transparent;
  border-right: 1rem solid transparent;
}

[data-theme-preset='geili-minimal'] [data-slot='table-body'] [data-slot='table-row']:not(:last-child) {
  border-bottom: 1px solid var(--border);
}

/* Remove default divide-y from parent if present */
[data-theme-preset='geili-minimal'] [data-slot='table-body'].divide-y > [data-slot='table-row'] {
  border-top: 0;
}
```

This creates:
- 16px (1rem) transparent "border" on left/right → pushes content inset
- 1px bottom border between rows (except last)
- Overrides any `divide-y` utility from Tailwind

**Alternative approach if rows don't have `data-slot='table-row'`:**

Search for the actual class structure:
```bash
cd /Users/tedliu/code/new-api/web/default/src/components/data-table/core
grep -A10 "TableRow\|<tr" data-table-view.tsx
```

If rows are plain `<tr>` elements, use:
```css
[data-theme-preset='geili-minimal'] [data-slot='table'] tbody tr {
  padding-left: 1rem;
  padding-right: 1rem;
}

[data-theme-preset='geili-minimal'] [data-slot='table'] tbody tr:not(:last-child) {
  border-bottom: 1px solid var(--border);
}
```

**Or, if using a wrapper div with `divide-y`:**

Find the parent container that applies `divide-y`. If it's on `<tbody>` or a wrapper `<div>`, replace `divide-y` with custom CSS:

```tsx
// Before
<tbody className="divide-y divide-border">

// After
<tbody className="[&>tr]:border-b [&>tr]:border-border [&>tr]:px-4 [&>tr:last-child]:border-b-0">
```

This uses Tailwind arbitrary variants to:
- Apply `border-b border-border px-4` to all `<tr>`
- Remove bottom border from last `<tr>`

**Decision:** Use the Tailwind arbitrary variant approach in the component file for precision.

---

## Specific File Fixes

### Fix filter bars

**File 1: `common-logs-filter-bar.tsx`**

**Search:**
```bash
grep -n "className.*border" common-logs-filter-bar.tsx
```

Remove any `border` class from the main container. If the container has `rounded-lg`, change to `rounded-[var(--radius-card)]`.

**File 2: `task-logs-filter-bar.tsx`**

Same as above.

---

### Fix table row padding

**File: `data-table-view.tsx`**

Find the `<tbody>` or row container. Currently likely:
```tsx
<tbody className="divide-y divide-border">
  {table.getRowModel().rows.map((row) => (
    <tr ...>...</tr>
  ))}
</tbody>
```

**Change to:**
```tsx
<tbody className="[&>tr]:border-b [&>tr]:border-border [&>tr]:px-4 [&>tr:last-child]:border-b-0">
  {table.getRowModel().rows.map((row) => (
    <tr ...>...</tr>
  ))}
</tbody>
```

This adds 16px (px-4) horizontal padding to each row and uses bottom borders instead of `divide-y`.

**Note:** If rows are wrapped in a custom component (like `<TableRow>`), adjust the selector accordingly. The goal is to add `px-4` (or `px-6` for more breathing room — 24px) to each row.

---

## Build, Commit, Deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/usage-logs/ web/default/src/components/data-table/ web/default/src/styles/
   git commit -m "fix(web): remove filter bar borders, add table row inset

   - common-logs-filter-bar, task-logs-filter-bar: remove borders
   - table rows: add horizontal padding (px-4 or px-6) so dividers don't touch edges
   - cleaner table breathing room"
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
     TAG="geili/new-api:tables-polish-${TS}"
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tail -5
     sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml
     docker compose up -d relay-new-api
     sleep 8
     docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api
   '
   ```

---

## Acceptance

- [ ] Filter toolbars (common logs, task logs): no border
- [ ] Table row dividers: do NOT touch left/right edges, have 16-24px inset
- [ ] Visual breathing room in tables
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Test on both usage logs and task logs pages
