# Goal: API Keys, Usage Logs, Task Logs — Unified Table Visual

**Issue:** API密钥、使用日志、任务日志三个页面的表格视觉不一致，与概览/数据看板页风格不统一。表格容器有边框、圆角是6.08px surface而不是24px card。

**Target:** 所有三个页面的表格、卡片、筛选工具栏完全统一为：
- 表格容器 → 24px圆角，无边框，#F5F5F5背景
- 空状态容器 → 24px圆角，无边框
- 筛选工具栏 → 如果是独立容器则24px圆角，无边框；如果是内嵌则保持现状
- 所有按钮 → 9999px pill
- 移动端卡片 → 24px圆角，无边框

---

## Files to Fix

### API Keys Page

**File 1: `keys/components/api-keys-table.tsx`**

**Line 67** (table container with data):
```tsx
// Before
<div className='divide-border border-border overflow-hidden rounded-[var(--radius-surface)] border'>

// After
<div className='divide-border overflow-hidden rounded-[var(--radius-card)]'>
```
Remove `border-border border`, change to card radius.

**Line 102** (empty state container):
```tsx
// Before
<div className='border-border rounded-[var(--radius-surface)] border p-8'>

// After
<div className='rounded-[var(--radius-card)] p-8'>
```
Remove border, change to card radius.

**Line 121** (skeleton loading container):
```tsx
// Before
<div className='divide-border border-border overflow-hidden rounded-[var(--radius-surface)] border'>

// After
<div className='divide-border overflow-hidden rounded-[var(--radius-card)]'>
```
Remove border, change to card radius.

---

### Usage Logs Page

**File 2: `usage-logs/components/logs-filter-toolbar.tsx`**

**Line 110** (filter toolbar container):
```tsx
// Before
className={cn('bg-card/50 rounded-lg border p-2.5', props.className)}

// After
className={cn('bg-card/50 rounded-[var(--radius-card)] p-2.5', props.className)}
```
Remove `border`, change `rounded-lg` to card radius. **Note:** If this toolbar is meant to be a floating panel, 24px is correct. If it's meant to be a subtle inline control bar, consider keeping it surface (6.08px) without border. **Decision:** Make it 24px card to match the table it filters.

**File 3: `usage-logs/components/usage-logs-mobile-card.tsx`**

**Line 56** (mobile card container with data):
```tsx
// Before
<div className='border-border/50 bg-card overflow-hidden rounded-[var(--radius-surface)] border'>

// After
<div className='bg-card overflow-hidden rounded-[var(--radius-card)]'>
```
Remove `border-border/50 border`, change to card radius.

**Line 341** (empty state):
```tsx
// Before
<div className='rounded-lg border p-6'>

// After
<div className='rounded-[var(--radius-card)] p-6'>
```
Remove `border`, change to card radius.

**Line 356** (loading skeleton):
```tsx
// Before
<div className='border-border/50 bg-card overflow-hidden rounded-lg border'>

// After
<div className='bg-card overflow-hidden rounded-[var(--radius-card)]'>
```
Remove border, change to card radius.

**File 4: `usage-logs/components/dialogs/audio-preview-dialog.tsx`**

**Line 75** (audio preview panel):
```tsx
// Before
<div className='bg-card flex gap-4 rounded-lg border p-4'>

// After
<div className='bg-card flex gap-4 rounded-[var(--radius-surface)] p-4'>
```
Remove `border`. **Note:** This is a small inline panel inside a dialog, not a top-level card → use surface (6.08px) not card (24px).

---

### Task Logs Page

**Search for task logs table/card:**
```bash
cd /Users/tedliu/code/new-api/web/default/src/features/usage-logs
grep -rn "task.*table\|TaskLog" . --include="*.tsx" | head -10
```

Task logs likely share the same `usage-logs-table.tsx` or have a dedicated component. **Check:**
- If task logs use `usage-logs-table.tsx` → fixing that file covers both
- If task logs have a separate table component → apply the same fixes (remove border, change to card radius)

**Search command to find task-specific components:**
```bash
find /Users/tedliu/code/new-api/web/default/src/features/usage-logs -name "*task*" -type f
```

For each task-specific table/card file found, apply the same pattern:
- Table container → `rounded-[var(--radius-card)]`, no `border`
- Empty state → `rounded-[var(--radius-card)]`, no `border`
- Mobile cards → `rounded-[var(--radius-card)]`, no `border`

---

### Additional: Check usage-logs-table.tsx desktop table

**File: `usage-logs/components/usage-logs-table.tsx`**

**Search:**
```bash
grep -n "border\|rounded-\[" usage-logs-table.tsx | head -20
```

Look for the main `<DataTableView>` or table wrapper. If it has `border` or uses `rounded-[var(--radius-surface)]`, fix it to:
```tsx
rounded-[var(--radius-card)]  // no border class
```

---

## Build, Commit, Deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/keys/ web/default/src/features/usage-logs/
   git commit -m "fix(web): unified table visual for keys/usage-logs/task-logs

   - all table containers: 24px radius, no border
   - empty states: 24px radius, no border
   - mobile cards: 24px radius, no border
   - filter toolbar: 24px radius, no border
   - matches overview/dashboard style"
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
     TAG="geili/new-api:tables-unified-${TS}"
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

- [ ] API密钥页表格：24px圆角，无边框
- [ ] 使用日志页表格（桌面+移动）：24px圆角，无边框
- [ ] 任务日志页表格：24px圆角，无边框
- [ ] 所有空状态容器：24px圆角，无边框
- [ ] 筛选工具栏：24px圆角，无边框
- [ ] 所有三个页面视觉与概览/数据看板完全一致
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Keep internal table borders (row dividers) — only remove outer container borders
- Small inline panels in dialogs can use surface (6.08px) — only top-level cards/tables use card (24px)
