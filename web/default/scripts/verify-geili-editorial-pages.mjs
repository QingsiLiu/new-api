/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const files = {
  authLayout: 'src/features/auth/auth-layout.tsx',
  authSignIn: 'src/features/auth/sign-in/index.tsx',
  authSignUp: 'src/features/auth/sign-up/index.tsx',
  authForgot: 'src/features/auth/forgot-password/index.tsx',
  authOtp: 'src/features/auth/otp/index.tsx',
  authReset: 'src/features/auth/reset-password-confirm/index.tsx',
  homeHero: 'src/features/home/components/sections/hero.tsx',
  homeFeatures: 'src/features/home/components/sections/features.tsx',
  homeStats: 'src/features/home/components/sections/stats.tsx',
  homeCta: 'src/features/home/components/sections/cta.tsx',
  homeGatewayCard: 'src/features/home/components/gateway-card.tsx',
  homeFeatureItem: 'src/features/home/components/feature-item.tsx',
  homeTerminal: 'src/features/home/components/hero-terminal-demo.tsx',
  pricing: 'src/features/pricing/index.tsx',
  wallet: 'src/features/wallet/index.tsx',
  walletStats: 'src/features/wallet/components/wallet-stats-card.tsx',
  walletRecharge: 'src/features/wallet/components/recharge-form-card.tsx',
  dashboardSummary: 'src/features/dashboard/components/overview/summary-cards.tsx',
  dashboardStatCard: 'src/features/dashboard/components/ui/stat-card.tsx',
  dashboardOverview: 'src/features/dashboard/components/overview/overview-dashboard.tsx',
  dashboardPanelWrapper: 'src/features/dashboard/components/ui/panel-wrapper.tsx',
  errorFrame: 'src/features/errors/error-frame.tsx',
  errorsNotFound: 'src/features/errors/not-found-error.tsx',
  errorsGeneral: 'src/features/errors/general-error.tsx',
  errorsForbidden: 'src/features/errors/forbidden.tsx',
  errorsUnauthorized: 'src/features/errors/unauthorized-error.tsx',
  errorsMaintenance: 'src/features/errors/maintenance-error.tsx',
}

const source = Object.fromEntries(
  Object.entries(files).map(([key, file]) => [
    key,
    readFileSync(resolve(root, file), 'utf8'),
  ])
)

function assertIncludes(key, needle, message) {
  if (!source[key].includes(needle)) {
    throw new Error(`${files[key]}: ${message}`)
  }
}

function assertNotIncludes(key, needle, message) {
  if (source[key].includes(needle)) {
    throw new Error(`${files[key]}: ${message}`)
  }
}

function assertNoPattern(key, pattern, message) {
  if (pattern.test(source[key])) {
    throw new Error(`${files[key]}: ${message}`)
  }
}

for (const key of [
  'authLayout',
  'authSignIn',
  'authSignUp',
  'authForgot',
  'authOtp',
  'authReset',
]) {
  assertIncludes(key, 'editorial-', 'auth pages must use editorial typography')
}

assertIncludes(
  'authLayout',
  'useSystemConfig',
  'auth layout must keep using configured logo and system name'
)
assertIncludes(
  'authLayout',
  'grid-cols-[minmax(0,0.9fr)_minmax',
  'auth layout must use an asymmetric editorial grid'
)

for (const key of [
  'homeHero',
  'homeFeatures',
  'homeStats',
  'homeCta',
  'pricing',
  'walletStats',
  'dashboardSummary',
  'dashboardStatCard',
]) {
  assertIncludes(key, 'editorial-', 'page must use editorial primitives/classes')
}

for (const key of [
  'homeHero',
  'homeFeatures',
  'homeCta',
  'homeGatewayCard',
  'homeFeatureItem',
  'homeTerminal',
  'pricing',
]) {
  assertNoPattern(
    key,
    /(?:blue|violet|purple|emerald|amber|sky|rose|teal|slate|zinc|stone|neutral|gray)-(?:50|100|200|300|400|500|600|700|800|900|950)/,
    'public/editorial pages must not use palette utility colors'
  )
  assertNoPattern(
    key,
    /radial-gradient|bg-gradient|bg-linear|from-|via-|to-|glass-|backdrop-blur|shadow-(?:xs|sm|md|lg|xl|2xl|\[)/,
    'public/editorial pages must not use old gradient, glass, or shadow styling'
  )
  assertNoPattern(
    key,
    /oklch\(|rgba\(|#[0-9a-fA-F]{3,8}/,
    'public/editorial pages must use semantic tokens, not hardcoded colors'
  )
}

for (const key of [
  'errorsNotFound',
  'errorsGeneral',
  'errorsForbidden',
  'errorsUnauthorized',
  'errorsMaintenance',
]) {
  assertIncludes(key, 'ErrorFrame', 'error pages must use shared editorial frame')
}

assertIncludes(
  'errorFrame',
  'editorial-display',
  'error frame must render error codes with display serif'
)

assertIncludes(
  'walletStats',
  'EditorialStatGroup',
  'wallet stats must use editorial stat group'
)
assertIncludes(
  'walletStats',
  'accent={index === 0}',
  'wallet balance must be the single cinnabar stat'
)

for (const key of ['walletRecharge', 'dashboardSummary', 'dashboardStatCard']) {
  assertNoPattern(
    key,
    /(?:rose|teal|emerald|green|red|amber|blue|violet|purple|sky)-(?:50|100|200|300|400|500|600|700|800|900|950)/,
    'dashboard/wallet pages must avoid palette utility colors'
  )
}

assertNotIncludes(
  'dashboardStatCard',
  'bg-linear-to-t',
  'stat cards must use semantic chart/token colors instead of gradients'
)
assertIncludes(
  'dashboardStatCard',
  'editorial-stat-value',
  'stat values must use editorial serif numerals'
)

console.log('geili-editorial page coverage verified')
