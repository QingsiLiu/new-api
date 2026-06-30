# Geili Editorial Rebrand Progress

Date: 2026-06-30
Branch: `codex/geili-editorial-ui`
Worktree: `/Users/tedliu/.config/superpowers/worktrees/new-api/codex-geili-editorial-ui`
Target: `web/default` only

## Baseline

- Read the Goal and parent design spec from `/Users/tedliu/Documents/GeiliAPI/docs/superpowers/specs/`.
- Created an isolated worktree from `origin/main`, separate from the existing `codex/newapi-async-gateway` checkout.
- Ran `bun install` from `web/`.
- Baseline verification:
  - `bun run typecheck` in `web/default`: passed.
  - `bun run build` in `web/default`: passed.
  - `bun run lint` in `web/default`: failed before any visual edits with existing React hooks/query lint errors (101 errors, 4 warnings), mainly `react-hooks/set-state-in-effect`, `react-hooks/refs`, `react-hooks/purity`, and `@tanstack/query/exhaustive-deps`.

## Stage 1 - Fonts

- Added self-hosted font files under `web/default/public/fonts`:
  - Fraunces variable normal/italic, latin and latin-ext.
  - Inter variable normal/italic, latin and latin-ext.
  - IBM Plex Mono 400/500/600 normal, latin and latin-ext.
- Added local `@font-face` declarations in `web/default/src/styles/theme.css`.
- Set `--font-sans` to Inter, `--font-serif` to Fraunces + CJK serif fallbacks, and `--font-mono` to IBM Plex Mono.
- Removed old `@fontsource-variable/public-sans` and `@fontsource-variable/lora` imports and dependencies.
- Updated font preference docs/types to describe the new self-hosted Geili font stack.
- Verification after Stage 1:
  - `bun run typecheck` in `web/default`: passed.
  - `bun run build` in `web/default`: passed.
  - Build output includes bundled `inter-*`, `fraunces-*`, and `ibm-plex-mono-*` font assets.

## Stage 2 - Geili Editorial Preset

- Added `geili-editorial` to `THEME_PRESETS` and set `DEFAULT_THEME_CUSTOMIZATION.preset` to it.
- Updated the theme customization provider so the `geili-editorial` default still writes `data-theme-preset="geili-editorial"` to `<body>`.
- Added complete light and dark token blocks in `web/default/src/styles/theme-presets.css` for:
  - warm paper / ink paper backgrounds,
  - cinnabar `--primary`,
  - muted semantic status colors,
  - chart, sidebar, skeleton, border, input, and radius tokens.
- Kept the old neutral `default` preset available for manual selection.
- Added localized preset labels for `preset.geili-editorial`.
- Added `web/default/scripts/verify-geili-editorial-theme.mjs` to statically verify preset registration, default selection, and full light/dark token coverage.
- Verification after Stage 2:
  - `bun scripts/verify-geili-editorial-theme.mjs` in `web/default`: passed.
  - `bun run typecheck` in `web/default`: passed.
  - `bun run build` in `web/default`: passed.

## Stage 3 - Editorial Base Typography And Motion

- Added global editorial typography rules in `web/default/src/styles/index.css`:
  - Fraunces for headings, dialog/sheet/drawer titles, card titles, display text, and stat numbers.
  - IBM Plex Mono for table headers, badges, labels, status text, and IDs.
  - Warm selection color, balanced headings, tabular stat numerals, and base font rendering features.
- Added reusable editorial utility classes for constrained containers, section stacks, mono labels, display titles, stat values, hairlines, focal text, and hairline panels.
- Changed card hover styling away from shadow stacking toward subtle border/background changes.
- Added shared hairline table header treatment and removed default shadows from cards and overlay surfaces so component layers can rely on hairline structure.
- Tightened shared motion in `web/default/src/lib/motion.ts` to 150-220ms fade/slide transitions without blur or scale-heavy motion.
- Verification after Stage 3:
  - `bunx prettier --write src/styles/index.css src/lib/motion.ts` in `web/default`: passed.
  - `bun run typecheck` in `web/default`: passed.
  - `bun run build` in `web/default`: passed.

## Next

- Stage 4: restyle shared UI, layout, data table, status, and overlay components without changing business logic.
