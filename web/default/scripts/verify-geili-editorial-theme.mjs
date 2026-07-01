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
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const presetsCss = readFileSync(
  resolve(root, 'src/styles/theme-presets.css'),
  'utf8'
)
const customizationTs = readFileSync(
  resolve(root, 'src/lib/theme-customization.ts'),
  'utf8'
)

const requiredTokens = [
  '--background',
  '--foreground',
  '--card',
  '--card-foreground',
  '--popover',
  '--popover-foreground',
  '--primary',
  '--primary-foreground',
  '--secondary',
  '--secondary-foreground',
  '--muted',
  '--muted-foreground',
  '--accent',
  '--accent-foreground',
  '--border',
  '--input',
  '--ring',
  '--destructive',
  '--destructive-foreground',
  '--success',
  '--success-foreground',
  '--warning',
  '--warning-foreground',
  '--info',
  '--info-foreground',
  '--neutral',
  '--neutral-foreground',
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
  '--sidebar',
  '--sidebar-foreground',
  '--sidebar-primary',
  '--sidebar-primary-foreground',
  '--sidebar-accent',
  '--sidebar-accent-foreground',
  '--sidebar-border',
  '--sidebar-ring',
  '--skeleton-base',
  '--skeleton-highlight',
  '--radius',
]

function getBlock(selector) {
  const start = presetsCss.indexOf(`${selector} {`)
  if (start === -1) {
    throw new Error(`Missing selector: ${selector}`)
  }

  const bodyStart = presetsCss.indexOf('{', start) + 1
  let depth = 1
  for (let index = bodyStart; index < presetsCss.length; index += 1) {
    const char = presetsCss[index]
    if (char === '{') depth += 1
    if (char === '}') depth -= 1
    if (depth === 0) {
      return presetsCss.slice(bodyStart, index)
    }
  }

  throw new Error(`Unclosed selector: ${selector}`)
}

function assertTokens(selector, tokens) {
  const block = getBlock(selector)
  const missing = tokens.filter((token) => !block.includes(`${token}:`))
  if (missing.length > 0) {
    throw new Error(`${selector} missing tokens: ${missing.join(', ')}`)
  }
}

assertTokens("[data-theme-preset='geili-editorial']", requiredTokens)
assertTokens(".dark [data-theme-preset='geili-editorial']", requiredTokens)

if (!customizationTs.includes("preset: 'geili-editorial'")) {
  throw new Error('DEFAULT_THEME_CUSTOMIZATION must default to geili-editorial')
}

if (!customizationTs.includes("value: 'geili-editorial'")) {
  throw new Error('THEME_PRESETS must register geili-editorial')
}

console.log('geili-editorial theme preset verified')
