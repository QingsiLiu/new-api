# Goal: Remove Chrome Dividers — Pure Surface Separation

**Philosophy:** The design skill says "depth from surface contrast only" — not from borders. The current sidebar right border and header bottom border are visual noise. Remove them. Separation comes purely from the #FFFFFF sidebar against a subtly tinted main-content background.

---

## Tasks

### T1: Remove sidebar right border

**File:** `web/default/src/components/ui/sidebar.tsx`

Find the `Sidebar` component (likely the root container with `data-slot='sidebar'`). It currently has:
- `border-right: 1px solid var(--sidebar-border)` or similar

**Change to:**
```tsx
// Remove border-right entirely
className="... [remove any border-r-* or border-right classes] ..."
```

If there's an inline style or CSS rule in `index.css` that forces `[data-slot='sidebar'] { border-right: 1px ... }`, remove that line.

**Also check:** `src/styles/index.css` — if there's a scoped rule like:
```css
[data-theme-preset='geili-minimal'] [data-slot='sidebar'] {
  border-right: 1px solid #e5e7eb !important;
}
```
→ Delete the `border-right` line entirely (keep other properties like `box-shadow: none`).

---

### T2: Ensure main content has subtle background tint

**Goal:** The sidebar (#FFFFFF) needs to contrast against the main content area. Main content should be `bg-background` which should resolve to a barely-tinted white (not pure #FFFFFF).

**File:** `web/default/src/styles/theme-presets.css` — in `[data-theme-preset='geili-minimal']`:

Check the `--background` token. Currently it's likely `#ffffff`. Change to:
```css
--background: #fafafa; /* barely gray, creates micro-contrast with sidebar */
```

**Alternatively:** Keep `--background: #ffffff` and set the **sidebar** to `#f9f9f9` (slightly darker than content). But the design skill says sidebar is `neutral-primary-soft (#FFFFFF)`, so better to tint the content area instead.

**Dark mode:** Keep `--background: #000000` (sidebar also #000000 in dark, separated by content being #0a0a0a or similar — check if this is already set).

---

### T3: Remove header bottom border

**File:** Find the top navigation header component. Likely in:
- `web/default/src/components/layout/header.tsx`
- or `web/default/src/components/layout/nav.tsx`  
- or `web/default/src/app/layout.tsx` (if header is inline)

Search for:
```bash
cd /Users/tedliu/code/new-api/web/default/src
grep -rn "border-b\|border-bottom" components/layout/*.tsx app/layout.tsx features/*/components/layout*.tsx
```

Look for the `<header>` or top nav container that has `border-b`, `border-bottom`, or `shadow-sm`. **Remove it entirely.**

**Example:**
```tsx
// Before
<header className="border-b border-border bg-background">

// After
<header className="bg-background">
```

If the border is in a CSS file (e.g., `@layer base { header { border-bottom: ... } }`), remove that rule.

---

### T4: Verify page background is the content tone

The `<body>` or outermost wrapper (`<SidebarProvider>` or `<main>`) should have `bg-background` so the tinted background applies to the whole content area, not just cards.

**File:** `web/default/src/app/layout.tsx` or wherever the root layout wrapper is.

Ensure the content area (the `<SidebarInset>` or main wrapper) has `className="bg-background"` so it picks up the `#fafafa` tone.

---

### T5: Build, commit, deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src
   git commit -m "fix(web): remove chrome dividers, pure surface separation

   - sidebar: remove right border (depth from bg contrast only)
   - header: remove bottom border (no visual noise)
   - main content: subtle #fafafa tint for sidebar contrast
   - aligns with 'radically minimal' design philosophy"
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
     TAG="geili/new-api:minimal-borderless-${TS}"
     cp docker-compose.yml docker-compose.yml.before-minimal-borderless-${TS}
     docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tee build-minimal-borderless-${TS}.log
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

- [ ] No vertical divider between sidebar and main content
- [ ] No horizontal divider below top navigation
- [ ] Sidebar (#FFFFFF) visually separates from main content (#FAFAFA) via subtle background tone only
- [ ] Dark mode: sidebar (#000000) separates from content (#0A0A0A or similar darker tone)
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Do NOT remove table borders, card borders, or input borders — only the sidebar/header chrome dividers
