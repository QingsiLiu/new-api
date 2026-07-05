import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const themeCss = readFileSync(
  new URL('../styles/theme.css', import.meta.url),
  'utf8'
)

const calmNames = [
  'green',
  'teal',
  'blue',
  'violet',
  'amber',
  'rose',
  'gray',
] as const

function cssBlock(selector: string): string {
  const match = themeCss.match(new RegExp(String.raw`${selector}\s*{([\s\S]*?)\n}`))
  assert.ok(match, `missing ${selector} block`)
  return match[1]
}

function cssVar(block: string, name: string): string {
  const match = block.match(new RegExp(String.raw`--${name}:\s*([^;]+);`))
  assert.ok(match, `missing --${name}`)
  return match[1].trim()
}

describe('calm palette css tokens', () => {
  test('exposes calm palette colors through theme inline mappings', () => {
    for (const name of calmNames) {
      assert.match(
        themeCss,
        new RegExp(String.raw`--color-calm-${name}-bg:\s*var\(--calm-${name}-bg\);`)
      )
      assert.match(
        themeCss,
        new RegExp(String.raw`--color-calm-${name}-fg:\s*var\(--calm-${name}-fg\);`)
      )
    }
  })

  test('uses one shared lightness and chroma for light calm backgrounds', () => {
    const root = cssBlock(':root')

    for (const name of ['green', 'teal', 'blue', 'violet', 'rose'] as const) {
      assert.match(cssVar(root, `calm-${name}-bg`), /^oklch\(0\.955 0\.024 /)
    }

    assert.match(cssVar(root, 'calm-amber-bg'), /^oklch\(0\.955 0\.028 /)
    assert.match(cssVar(root, 'calm-gray-bg'), /^oklch\(0\.945 0\.004 /)
  })

  test('keeps dark calm backgrounds in the same low-chroma family', () => {
    const dark = cssBlock('\\.dark')

    for (const name of ['green', 'teal', 'blue', 'violet', 'rose'] as const) {
      assert.match(cssVar(dark, `calm-${name}-bg`), /^oklch\(0\.26 0\.03 /)
    }

    assert.match(cssVar(dark, 'calm-amber-bg'), /^oklch\(0\.27 0\.035 /)
    assert.match(cssVar(dark, 'calm-gray-bg'), /^oklch\(0\.27 0\.006 /)
  })
})
