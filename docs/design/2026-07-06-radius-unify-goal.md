# Goal: Wallet & Profile — Unify Internal Element Radius

**Root cause identified:** Top-level cards are all 24px (consistent). But INTERNAL elements use a chaotic mix of radii — `rounded-lg` (6px), `rounded-md` (4.5px), `rounded-xl` (8.5px), `rounded-2xl` (11px). This inconsistency is what makes the pages look "wrong."

**Design contract:**
- Top-level cards: `rounded-[var(--radius-card)]` (24px) — already correct, DON'T touch
- **Internal surfaces** (icon boxes, inner panels, option rows, amount buttons, tabs, chips): `rounded-[var(--radius-surface)]` (6.08px) — UNIFY ALL to this
- Avatars: keep `rounded-xl`/`rounded-2xl` (avatars are allowed larger rounding) — DON'T touch profile-header avatar
- Small status dots/pills that are meant to be circular: keep `rounded-full`

---

## Rule: Replace ALL internal `rounded-lg`, `rounded-md`, `rounded-xl` (except avatars) with `rounded-[var(--radius-surface)]`

### Wallet files

**`wallet/components/affiliate-rewards-card.tsx`**
- Line 66: icon box `rounded-lg` → `rounded-[var(--radius-surface)]`

**`wallet/components/subscription-plans-card.tsx`**
- Line 417: subscription record box `rounded-md` → `rounded-[var(--radius-surface)]`

**`wallet/components/recharge-form-card.tsx`**
- Line 239: amount preset button `rounded-lg` → `rounded-[var(--radius-surface)]`
- Line 289: payment method container `rounded-md` → `rounded-[var(--radius-surface)]`
- Line 320: payment option button `rounded-lg` → `rounded-[var(--radius-surface)]`
- Line 382: payment option button `rounded-lg` → `rounded-[var(--radius-surface)]`

### Profile files

**`profile/components/passkey-card.tsx`**
- Line 225: icon box `rounded-md` → `rounded-[var(--radius-surface)]`
- Line 329: info panel `rounded-md` → `rounded-[var(--radius-surface)]`

**`profile/components/checkin-calendar-card.tsx`**
- Line 283: option button `rounded-lg` → `rounded-[var(--radius-surface)]`
- Line 286: icon box `rounded-xl` → `rounded-[var(--radius-surface)]`
- Line 298: status chip `rounded-md` → keep `rounded-full` (small status pill, make it pill)
- Line 426: calendar day cell `rounded-lg` → `rounded-[var(--radius-surface)]`
- Line 466: note box `rounded-lg border` → `rounded-[var(--radius-surface)]`, remove `border`

**`profile/components/profile-settings-card.tsx`**
- Line 73: TabsList container `rounded-xl` → `rounded-[var(--radius-pill)]` (tab container should be pill)
- Line 76: tab trigger `rounded-lg` → `rounded-[var(--radius-pill)]` (tab items pill)
- Line 84: tab trigger `rounded-lg` → `rounded-[var(--radius-pill)]` (tab items pill)

**`profile/components/two-fa-card.tsx`**
- Line 83: icon box `rounded-md` → `rounded-[var(--radius-surface)]`

**`profile/components/sidebar-modules-card.tsx`**
- Line 209: icon box `rounded-lg` → `rounded-[var(--radius-surface)]`
- Line 228: module panel `rounded-xl border` → `rounded-[var(--radius-surface)]`, remove `border`
- Line 246: module row `rounded-lg border` → `rounded-[var(--radius-surface)]`, remove `border`

**`profile/components/profile-security-card.tsx`**
- Line 109: icon box `rounded-md` → `rounded-[var(--radius-surface)]`

### DON'T TOUCH (avatars keep larger rounding)
- `profile/components/profile-header.tsx` lines 110, 112 — avatar `rounded-xl`/`rounded-2xl` stays

---

## Also: Remove remaining internal borders

While unifying radius, remove any `border` class that appears alongside these internal elements (they should use background tone, not borders):
- `checkin-calendar-card.tsx:466` — remove `border`
- `sidebar-modules-card.tsx:228,246` — remove `border`

Keep section dividers (`border-b`, `border-t`) that separate card header from content.

**Also fix:** `profile-settings-card.tsx:51` and `recharge-form-card.tsx:141` still have `border-b` on CardHeader — these are the raw `<Card>` skeleton fallbacks (loading states). Change `border-b` to nothing (whitespace separation) to match the TitledCard fix already applied.

---

## Build, Commit, Deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Verify the build CSS** (sanity check radius unified):
   ```bash
   grep -c "radius-surface" dist/static/css/index.*.css
   ```

3. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/wallet/ web/default/src/features/profile/
   git commit -m "fix(web): unify internal element radius on wallet & profile

   - all internal surfaces (icon boxes, panels, option rows, amount buttons)
     -> rounded-[var(--radius-surface)] (6.08px), was chaotic mix of lg/md/xl/2xl
   - tab containers/items -> pill
   - status chips -> pill
   - remove remaining internal borders (module panels, note boxes)
   - avatars keep larger rounding (unchanged)
   - top-level cards stay 24px (unchanged)"
   git push origin main
   ```

4. **Deploy (rsync then build on server):**
   ```bash
   cd /Users/tedliu/code/new-api
   rsync -az --delete --exclude='.git' --exclude='node_modules' --exclude='web/default/dist' \
     ./ ctbuk-443:/opt/geili-relay/new-api-src/

   ssh -o ControlMaster=no ctbuk-443 'cd /opt/geili-relay && TS=$(date -u +%Y%m%dT%H%M%SZ) && TAG="geili/new-api:radius-unify-${TS}" && docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tail -3 && sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml && docker compose up -d relay-new-api && sleep 6 && docker ps --format "{{.Names}} {{.Status}}" | grep relay-new-api'
   ```

---

## Acceptance

- [ ] All internal icon boxes: 6.08px surface radius
- [ ] Amount preset buttons (¥5, ¥10...): 6.08px surface radius
- [ ] Payment option buttons: 6.08px surface radius
- [ ] Tab containers and items: pill
- [ ] Module panels, note boxes: 6.08px, no border
- [ ] Avatars unchanged (larger rounding OK)
- [ ] Top-level cards unchanged (24px)
- [ ] Visual consistency: card 24px, internal elements uniform 6px
- [ ] Build succeeds, container Up (healthy)

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`, deploy via `docker compose up -d`
- Use rsync to sync source (server dir is NOT a git repo)
- DON'T touch top-level card radius (already correct at 24px)
- DON'T touch avatars
