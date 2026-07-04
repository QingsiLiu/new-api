import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'
import {
  DEFAULT_THEME_CUSTOMIZATION,
  PRESET_DEFAULT_FONT,
  THEME_PRESETS,
} from './theme-customization'

const themePresetsCss = readFileSync('src/styles/theme-presets.css', 'utf8')
const themeCss = readFileSync('src/styles/theme.css', 'utf8')

function readBlock(selector: string) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = themePresetsCss.match(
    new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\n\\}`, 'm')
  )
  assert.ok(match, `missing CSS block for ${selector}`)
  return match[1]
}

function readVars(selector: string) {
  const block = readBlock(selector)
  const vars = new Map<string, string>()
  for (const [, name, value] of block.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)) {
    vars.set(name, value.trim())
  }
  return vars
}

function expectVars(vars: Map<string, string>, expected: Record<string, string>) {
  for (const [name, value] of Object.entries(expected)) {
    assert.equal(vars.get(name), value, `--${name}`)
  }
}

describe('geili-modern borderless preset', () => {
  test('is the default modern sans preset registered for users', () => {
    assert.equal(DEFAULT_THEME_CUSTOMIZATION.preset, 'geili-modern')
    assert.equal(PRESET_DEFAULT_FONT['geili-modern'], 'sans')
    assert.deepEqual(THEME_PRESETS[0], {
      value: 'geili-modern',
      name: '现代 / Modern',
      description:
        'Borderless neutral chrome with solid cinnabar accents and quiet Vercel-style surfaces.',
      swatches: ['#FFFFFF', '#CF4520', '#FAFAFA'],
    })
  })

  test('uses the approved light and dark token values without gradient tokens', () => {
    const light = readVars("[data-theme-preset='geili-modern']")
    const dark = readVars(".dark [data-theme-preset='geili-modern']")

    expectVars(light, {
      background: '#ffffff',
      foreground: '#0a0a0a',
      card: '#ffffff',
      primary: '#cf4520',
      'primary-foreground': '#ffffff',
      secondary: '#f4f4f4',
      muted: '#f4f4f4',
      'muted-foreground': '#666666',
      accent: '#f4f4f4',
      border: 'rgba(0,0,0,.07)',
      input: '#f4f4f4',
      ring: 'rgba(0,0,0,.15)',
      success: '#00a67e',
      sidebar: '#fafafa',
      'sidebar-border': 'transparent',
      radius: '0.5rem',
      'shadow-card': '0 0 0 1px rgba(0,0,0,.06), 0 4px 16px rgba(0,0,0,.05)',
      'grain-opacity': '0.025',
    })

    expectVars(dark, {
      background: '#0a0a0a',
      foreground: '#ededed',
      card: '#111111',
      primary: '#f0603a',
      'primary-foreground': '#0a0a0a',
      secondary: '#1a1a1a',
      muted: '#1a1a1a',
      'muted-foreground': '#888888',
      border: 'rgba(255,255,255,.08)',
      input: '#1a1a1a',
      ring: 'rgba(255,255,255,.15)',
      success: '#3ecf8e',
      sidebar: '#111111',
      'sidebar-border': 'transparent',
      'shadow-card': '0 0 0 1px rgba(255,255,255,.07), 0 4px 16px rgba(0,0,0,.5)',
      'grain-opacity': '0.045',
    })

    const tokenCss = `${readBlock("[data-theme-preset='geili-modern']")}\n${readBlock(".dark [data-theme-preset='geili-modern']")}`
    assert.doesNotMatch(tokenCss, /\b(?:linear|radial|conic)-gradient\b/)
    assert.equal(light.has('grad'), false)
    assert.equal(light.has('grad-soft'), false)
    assert.equal(dark.has('grad'), false)
    assert.equal(dark.has('grad-soft'), false)
  })

  test('exposes the borderless shadow token to Tailwind theme variables', () => {
    assert.match(themeCss, /--color-shadow-card:\s*var\(--shadow-card\);/)
  })
})
