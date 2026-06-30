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

## Next

- Stage 2: add the `geili-editorial` preset tokens, register it in the preset picker, and make it the default theme customization.
