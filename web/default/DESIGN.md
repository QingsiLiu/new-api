# READ BEFORE ANY UI CHANGE

This document is the standing visual contract for `web/default`.

Source of truth:
- Runtime tokens: `src/styles/theme-presets.css`, preset `geili-editorial`.
- Theme registration/defaults: `src/lib/theme-customization.ts`.
- Shared editorial utilities: `src/styles/index.css`.
- Original brand spec: `/Users/tedliu/Documents/GeiliAPI/docs/superpowers/specs/2026-06-30-geili-editorial-visual-system-design.md`.

If this document ever disagrees with the live `geili-editorial` preset, the
preset wins. Update the document in the same PR instead of changing tokens by
memory.

Before submitting any UI change, run:

```bash
bun run verify:design
```

## Five Iron Laws

1. One cinnabar focus: each visual region may have at most one primary
   cinnabar focus, such as the primary action, active marker, one key number, or
   a link underline. Everything else stays neutral.
2. Hairline first: use 1px `border-border` dividers and spacing to create
   structure. Do not stack cards with shadows, glass, or gradients.
3. Serif numbers, mono labels: large titles and numbers use Fraunces; eyebrow
   labels, table heads, states, IDs, and small metadata use IBM Plex Mono with
   uppercase tracking.
4. Whitespace before density: prefer fewer, clearer surfaces with generous
   spacing. Let headings and content breathe.
5. Restrained semantic color: success, warning, info, destructive, neutral, and
   in-progress states use tokenized, low-saturation dots or fine outlines. Do
   not use large saturated badges.

## Theme Preset

`geili-editorial` is the default theme preset for `web/default`. It is applied
through `data-theme-preset='geili-editorial'` and supports both light and dark
mode through the `.dark` ancestor selector.

Do not hardcode palette utility colors such as `blue-500`, `emerald-100`,
`slate-900`, or raw hex values in editorial pages. Use semantic tokens:
`bg-background`, `text-foreground`, `border-border`, `text-primary`,
`bg-success`, `bg-neutral`, and the shared editorial utilities.

## Light Tokens: Warm Paper

Current values from `src/styles/theme-presets.css`:

| Token | Value | Role |
| --- | --- | --- |
| `--background` | `#f4f1e8` | Warm paper page canvas |
| `--foreground` | `#18160f` | Ink text |
| `--card` | `#f8f5ec` | Raised panel surface |
| `--card-foreground` | `#18160f` | Text on panels |
| `--popover` | `#faf7ef` | Floating surface |
| `--popover-foreground` | `#18160f` | Text on floating surfaces |
| `--primary` | `#c8432a` | Cinnabar focus |
| `--primary-foreground` | `#fbf3e6` | Text on cinnabar |
| `--secondary` | `#e7e2d5` | Secondary surface/action |
| `--secondary-foreground` | `#2a271e` | Text on secondary |
| `--muted` | `#ece7db` | Quiet surface |
| `--muted-foreground` | `#8c8676` | Secondary text |
| `--accent` | `#ede8dc` | Hover/selected soft surface |
| `--accent-foreground` | `#18160f` | Text on accent |
| `--destructive` | `#9e2b22` | Destructive state |
| `--destructive-foreground` | `#fbf3e6` | Text on destructive |
| `--success` | `#1f7a4d` | Success state dot |
| `--success-foreground` | `#f4f1e8` | Text on success if needed |
| `--warning` | `#9b6a24` | Warning state |
| `--warning-foreground` | `#18160f` | Text on warning |
| `--info` | `#516f83` | Informational state |
| `--info-foreground` | `#f4f1e8` | Text on info |
| `--neutral` | `#a6a096` | Refund/neutral state |
| `--neutral-foreground` | `#18160f` | Text on neutral |
| `--border` | `#dcd6c8` | Hairline divider |
| `--input` | `#d6d0c1` | Input border |
| `--ring` | `#c8432a` | Focus ring |
| `--chart-1` | `#18160f` | Neutral chart series |
| `--chart-2` | `#8c8676` | Muted chart series |
| `--chart-3` | `#c8432a` | Cinnabar highlight series |
| `--chart-4` | `#1f7a4d` | Success chart series |
| `--chart-5` | `#a6a096` | Neutral chart series |
| `--sidebar` | `#f1ede3` | Sidebar surface |
| `--sidebar-foreground` | `#2a271e` | Sidebar text |
| `--sidebar-primary` | `#c8432a` | Sidebar active/focus marker |
| `--sidebar-primary-foreground` | `#fbf3e6` | Text on sidebar primary |
| `--sidebar-accent` | `#e7e2d5` | Sidebar hover/selected soft surface |
| `--sidebar-accent-foreground` | `#18160f` | Text on sidebar accent |
| `--sidebar-border` | `#dcd6c8` | Sidebar hairline |
| `--sidebar-ring` | `#c8432a` | Sidebar focus ring |
| `--skeleton-base` | `#e7e2d5` | Skeleton base |
| `--skeleton-highlight` | `#faf7ef` | Skeleton shimmer highlight |
| `--radius` | `0.4rem` | Base radius |

## Dark Tokens: Ink Paper

Current values from `src/styles/theme-presets.css`:

| Token | Value | Role |
| --- | --- | --- |
| `--background` | `#15130d` | Ink paper canvas |
| `--foreground` | `#ece6d7` | Cream text |
| `--card` | `#1b1812` | Raised panel surface |
| `--card-foreground` | `#ece6d7` | Text on panels |
| `--popover` | `#1e1b14` | Floating surface |
| `--popover-foreground` | `#ece6d7` | Text on floating surfaces |
| `--primary` | `#e1542f` | Brightened cinnabar focus |
| `--primary-foreground` | `#fcefe3` | Text on cinnabar |
| `--secondary` | `#262219` | Secondary surface/action |
| `--secondary-foreground` | `#ece6d7` | Text on secondary |
| `--muted` | `#221f18` | Quiet surface |
| `--muted-foreground` | `#928c7b` | Secondary text |
| `--accent` | `#2a2519` | Hover/selected soft surface |
| `--accent-foreground` | `#ece6d7` | Text on accent |
| `--destructive` | `#d14a33` | Destructive state |
| `--destructive-foreground` | `#fcefe3` | Text on destructive |
| `--success` | `#3fa76b` | Success state dot |
| `--success-foreground` | `#15130d` | Text on success if needed |
| `--warning` | `#d39a45` | Warning state |
| `--warning-foreground` | `#15130d` | Text on warning |
| `--info` | `#87a7b8` | Informational state |
| `--info-foreground` | `#15130d` | Text on info |
| `--neutral` | `#7c7768` | Refund/neutral state |
| `--neutral-foreground` | `#15130d` | Text on neutral |
| `--border` | `#312c22` | Hairline divider |
| `--input` | `#34301f` | Input border |
| `--ring` | `#e1542f` | Focus ring |
| `--chart-1` | `#ece6d7` | Neutral chart series |
| `--chart-2` | `#928c7b` | Muted chart series |
| `--chart-3` | `#e1542f` | Cinnabar highlight series |
| `--chart-4` | `#3fa76b` | Success chart series |
| `--chart-5` | `#7c7768` | Neutral chart series |
| `--sidebar` | `#17150f` | Sidebar surface |
| `--sidebar-foreground` | `#ece6d7` | Sidebar text |
| `--sidebar-primary` | `#e1542f` | Sidebar active/focus marker |
| `--sidebar-primary-foreground` | `#fcefe3` | Text on sidebar primary |
| `--sidebar-accent` | `#262219` | Sidebar hover/selected soft surface |
| `--sidebar-accent-foreground` | `#ece6d7` | Text on sidebar accent |
| `--sidebar-border` | `#312c22` | Sidebar hairline |
| `--sidebar-ring` | `#e1542f` | Sidebar focus ring |
| `--skeleton-base` | `#221f18` | Skeleton base |
| `--skeleton-highlight` | `#2a2519` | Skeleton shimmer highlight |
| `--radius` | `0.4rem` | Base radius |

## Typography

The type system has three voices:

| Role | Font | Usage |
| --- | --- | --- |
| Display serif | Fraunces | `h1`-`h6`, hero/display titles, section titles, large stats, error codes |
| UI sans | Inter | Body text, buttons, forms, table cells, navigation copy |
| Mono label | IBM Plex Mono | Eyebrows, table heads, status labels, IDs, code, compact metadata |

Implementation notes:
- Fonts are self-hosted under `public/fonts` and wired in `src/styles/theme.css`.
- `--font-sans` is Inter; `--font-serif` starts with Fraunces and then CJK
  serif fallbacks; `--font-mono` is IBM Plex Mono.
- `geili-editorial` keeps body/UI on Inter through `PRESET_DEFAULT_FONT`, while
  headings and editorial utilities use Fraunces.
- Use `.editorial-display`, `.editorial-section-title`, `.editorial-stat-value`,
  `.editorial-label`, `.editorial-status`, and `.editorial-id` instead of
  open-coded font stacks.
- Keep letter spacing at `0` for Fraunces display and numeric text. Mono labels
  use the shared uppercase tracking, currently `0.14em`.

Recommended sizes:

| Token | Size / Line | Weight | Usage |
| --- | --- | --- | --- |
| Display XL | `clamp(...) / 0.98` | 500 | Public hero titles |
| Display L | about `40px / 1` | 500 | Page-level titles |
| Section | `22-30px / 1.08` | 500 | Section headings |
| Stat | `30-56px / 0.98` | 500 | Large numbers |
| UI base | `14px / 1.5` | 400-500 | Body and controls |
| Mono label | `10-11px / 1.2` | 500 | Labels, table heads, statuses |

## Components

Button:
- Use the primary cinnabar variant only for the single main action in a region.
- Prefer secondary, outline, ghost, or link treatments for neighboring actions.
- Keep button text compact, medium weight, and free of decorative shadows.

Navigation and shell:
- Sidebar and navigation labels should read as mono editorial labels.
- Active state is foreground text plus a small cinnabar marker, not a large
  filled color block.
- Use `--sidebar-*` tokens for shell surfaces.

Tables:
- Table heads are mono uppercase labels.
- Rows are separated with hairlines.
- IDs, request numbers, and code-like values use mono.
- Amounts and key metrics may use Fraunces, right aligned where useful.
- Hover uses `accent`, not custom palette utilities.

Stats:
- Use `EditorialStat` and `EditorialStatGroup`.
- Structure is mono label above, Fraunces value below.
- Multiple stats are divided by vertical hairlines instead of independent
  shadow cards.
- Only one stat in a region should use the cinnabar accent.

Status:
- Use `EditorialStatus` or the same pattern: small dot plus mono label.
- Success uses `success`; in-progress uses `primary`; failure uses
  `destructive`; refund/neutral uses `neutral`.
- Do not use large high-saturation filled badges.

Cards and panels:
- Use `card` surface, `border-border`, and no shadow.
- Panels may use `.editorial-panel`.
- Cards are for repeated items, dialogs, and genuinely framed tools, not for
  wrapping every page section.

Inputs and controls:
- Use hairline borders, tokenized focus rings, and muted placeholders.
- Keep controls dense enough for repeated work, but do not sacrifice breathing
  room around section titles.

Dialogs, popovers, drawers, and command surfaces:
- Use `popover`/`card` surfaces, hairline borders, tokenized focus, and very
  restrained elevation only where the primitive requires it.
- Titles should use the shared serif rules.

Charts:
- Use neutral ink/cream series for most data.
- Reserve cinnabar for the one highlighted series.
- Gridlines and axes should use hairline/muted tokens; labels should be mono
  when compact metadata is shown.

Motion:
- Keep motion subtle: short fade/translate transitions, 150-220ms where
  possible, and respect `prefers-reduced-motion`.
- The top navigation progress indicator may use cinnabar.

## Page Rules

Public home:
- Use editorial grid, Fraunces display title, mono eyebrow labels, hairline
  feature divisions, and a single cinnabar CTA/focus.
- Do not bring back gradients, glass effects, decorative color clouds, or
  hardcoded palette utilities.

Auth:
- Warm paper / ink paper full-screen layout.
- Asymmetric editorial grid.
- Brand/system name remains configured through runtime system config.
- Form controls use hairline inputs and one cinnabar submit action.

Dashboard:
- Overview surfaces use editorial stat groups and panel wrappers.
- Key balance or one primary metric may be cinnabar; the rest are neutral.
- Summary panels should feel like editorial sections, not marketing cards.

Wallet and billing:
- Balance is the single cinnabar stat.
- Recharge may be the primary cinnabar action.
- Orders, records, and amounts should use table discipline: hairlines, mono
  labels, and tokenized states.

Pricing:
- Pricing pages are editorial, table-forward, and tokenized.
- Avoid raw palette colors, gradients, glass, backdrop blur, and heavy shadows.
- Model cards, detail panels, sidebars, toolbar, search, and pricing tables
  must remain semantic-token based.

Error pages:
- Use the shared editorial error frame.
- Error codes render with Fraunces display typography.

Admin/data-heavy pages:
- Favor structured data tables, compact controls, and predictable scanning.
- Use cinnabar sparingly for active markers, primary creation actions, or one
  in-progress state.

## Cinnabar Restraint

Cinnabar is `--primary`: `#c8432a` in light mode and `#e1542f` in dark mode.

Use it for:
- The one primary button in a visual region.
- The active navigation marker.
- One key number, such as balance.
- Link underline or subtle focus accent.
- In-progress state dot.

Do not use it for:
- Page backgrounds.
- Multiple competing buttons in the same region.
- Large filled badges.
- Gradients.
- Decorative blobs.
- Decorative borders around every card.

When in doubt, make the element neutral and let typography, spacing, and
hairlines do the work.

## Automated Guardrails

The design guard is:

```bash
bun run verify:design
```

It runs:

```bash
node scripts/verify-geili-editorial-theme.mjs
node scripts/verify-geili-editorial-components.mjs
node scripts/verify-geili-editorial-pages.mjs
```

The guard verifies:
- The `geili-editorial` preset includes the required light and dark tokens.
- The preset is registered and defaulted.
- Shared editorial components exist and use tokenized classes.
- Covered auth, home, pricing, wallet, dashboard, and error pages keep editorial
  typography and avoid hardcoded colors, palette utilities, gradients, glass,
  and old shadow styling where those checks apply.

Any UI PR must keep `bun run verify:design` passing.
