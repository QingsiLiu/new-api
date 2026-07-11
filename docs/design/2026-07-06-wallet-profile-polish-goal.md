# Goal: Wallet & Profile Pages Visual Polish

**Issue:** Wallet (钱包) and Profile (个人资料) pages have:
1. Cards with borders (should be borderless like overview/dashboard)
2. Excessively large text (text-2xl, text-xl everywhere)
3. Inconsistent radius (some rounded-lg, some rounded-xl, should be 24px card)

**Target:** 
- All cards: 24px radius, no border, #F5F5F5 background
- Reduce font sizes to comfortable reading levels
- Match overview/dashboard visual style

---

## Task 1: Remove borders from all cards

**Files to fix (Wallet page):**
- `wallet/components/recharge-form-card.tsx` — multiple `border` classes on internal dividers
- `wallet/components/subscription-plans-card.tsx` — plan cards have `border`
- `wallet/components/affiliate-rewards-card.tsx` — icon box has `border`
- `wallet/components/dialogs/billing-history-dialog.tsx` — history items have `border`
- `wallet/components/dialogs/payment-confirm-dialog.tsx` — confirmation sections

**Files to fix (Profile page):**
- `profile/components/profile-security-card.tsx` — security option cards have `border`
- `profile/components/checkin-calendar-card.tsx` — calendar container

**Search & replace pattern:**
```bash
cd /Users/tedliu/code/new-api/web/default/src/features/wallet
grep -rn "rounded-.*border\|border.*rounded-" . --include="*.tsx"
```

For each match:
- **Top-level card containers** → remove `border`, change to `rounded-[var(--radius-card)]`
- **Internal section dividers** (`border-t`, `border-b`) → **KEEP THESE** (they separate sections within a card)
- **Small icon boxes, badges** → remove `border`, keep `rounded-lg` or `rounded-md` as-is (they're small decorative elements)

---

## Task 2: Reduce font sizes

**Pattern to fix:**
- `text-2xl` (1.5rem / 24px) → `text-xl` (1.25rem / 20px) or `text-lg` (1.125rem / 18px)
- `text-xl` (1.25rem / 20px) → `text-lg` (1.125rem / 18px)
- Exception: Page titles can stay large, but **in-card** values/amounts should be smaller

**Specific files:**
- `wallet/components/subscription-plans-card.tsx:579` — `text-2xl` price → `text-xl`
- `wallet/components/dialogs/transfer-dialog.tsx:93` — `text-2xl` → `text-xl`
- `wallet/components/dialogs/payment-confirm-dialog.tsx:69` — `text-xl` → `text-lg`
- `wallet/components/dialogs/payment-confirm-dialog.tsx:99` — `text-2xl` → `text-xl`
- `profile/components/checkin-calendar-card.tsx:338` — `text-2xl` stats → `text-xl`

**General rule:**
- Page/card titles: `text-lg` or `text-base` (not xl/2xl)
- Data values (prices, numbers): `text-base` or `text-lg` (not xl/2xl)
- Body text: `text-sm` (keep as-is)
- Captions: `text-xs` (keep as-is)

---

## Task 3: Unified radius for cards

**All top-level cards should use `rounded-[var(--radius-card)]` (24px):**
- `subscription-plans-card.tsx:270` — `rounded-xl` → `rounded-[var(--radius-card)]`
- `billing-history-dialog.tsx:152,186` — `rounded-lg` → `rounded-[var(--radius-card)]` (if these are list items, keep `rounded-lg` for smaller elements)
- `profile-security-card.tsx:104` — `rounded-lg` → `rounded-[var(--radius-card)]` if top-level, keep as-is if inline

**Small elements can keep rounded-lg/md:**
- Icon boxes (`rounded-lg`, `rounded-md`) — keep as-is
- Badges, chips (`rounded-md`, `rounded-full`) — keep as-is
- Buttons (`rounded-full` pill) — keep as-is

---

## Task 4: Specific component fixes

### Wallet stats card
`wallet/components/wallet-stats-card.tsx:68` — already has `border-0` ✓

### Recharge form card
`recharge-form-card.tsx`:
- Line 141: `border-b` on CardHeader → **KEEP** (section divider)
- Line 175, 435, 448: `border-t` dividers → **KEEP**
- Line 289: input container `border` → **REMOVE**
- Line 492: Alert `border-t` → **KEEP**

### Subscription plans card
`subscription-plans-card.tsx`:
- Line 241: `border-b` on CardHeader → **KEEP**
- Line 270: plan details container `rounded-xl border` → change to `rounded-[var(--radius-card)]`, remove `border`
- Line 417: note box `border` → **REMOVE**
- Line 552: popular plan highlight `border-primary/70` → **KEEP** (brand accent)

### Profile security card
`profile-security-card.tsx`:
- Line 51: `border-b` on CardHeader → **KEEP**
- Line 104: security option cards `border` → **REMOVE**, change `rounded-lg` to `rounded-[var(--radius-surface)]`

### Checkin calendar card
`checkin-calendar-card.tsx`:
- Line 279: `border-b` → **KEEP**
- Line 336: `border-b` → **KEEP**

---

## Build, Commit, Deploy

1. **Build:**
   ```bash
   cd /Users/tedliu/code/new-api/web/default && bun run build
   ```

2. **Commit & push:**
   ```bash
   cd /Users/tedliu/code/new-api
   git add web/default/src/features/wallet/ web/default/src/features/profile/
   git commit -m "fix(web): wallet & profile visual polish

   - remove borders from top-level cards (keep section dividers)
   - reduce font sizes (text-2xl -> text-xl, text-xl -> text-lg)
   - unified 24px radius for cards
   - remove borders from input containers, note boxes
   - matches overview/dashboard style"
   git push origin main
   ```

3. **Deploy:**
   ```bash
   ssh -o ControlMaster=no -o ControlPath=none ctbuk-443 << 'EOF'
   cd /opt/geili-relay/new-api-src
   git fetch origin
   git reset --hard origin/main
   cd /opt/geili-relay
   TS=$(date -u +%Y%m%dT%H%M%SZ)
   TAG="geili/new-api:wallet-profile-polish-${TS}"
   docker build -t "$TAG" /opt/geili-relay/new-api-src/ 2>&1 | tail -5
   sed -i "s|image: geili/new-api:.*|image: ${TAG}|" docker-compose.yml
   docker compose up -d relay-new-api
   sleep 8
   docker ps --format "{{.Names}}\t{{.Status}}" | grep relay-new-api
   EOF
   ```

---

## Acceptance

- [ ] All top-level cards: 24px radius, no border
- [ ] Section dividers (border-t, border-b) preserved
- [ ] Font sizes reduced: no more text-2xl in card content, text-xl becomes text-lg
- [ ] Wallet page matches dashboard style
- [ ] Profile page matches dashboard style
- [ ] Build succeeds, container is `Up (healthy)`

---

## Constraints

- Push ONLY to `QingsiLiu/new-api`, never `Calcium-Ion/new-api`
- `bun run build`, never pnpm
- SSH via `ssh ctbuk-443`
- Deploy via `docker compose up -d`
- Keep internal section borders (`border-t`, `border-b`) — only remove outer card borders
- Keep brand accent borders (like `border-primary` on popular plans)
