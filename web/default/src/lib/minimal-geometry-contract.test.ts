import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'

const projectRoot = fileURLToPath(new URL('../../', import.meta.url))

function readSource(relativePath: string): string {
  return readFileSync(join(projectRoot, relativePath), 'utf8')
}

describe('geili-minimal geometry contract', () => {
  test('keeps the minimal base radius locked after user radius overrides', () => {
    const css = readSource('src/styles/theme-presets.css')
    const radiusAxisIndex = css.lastIndexOf("[data-theme-radius='xl']")
    const minimalLockIndex = css.lastIndexOf(
      "[data-theme-preset='geili-minimal'][data-theme-radius]"
    )

    assert.ok(radiusAxisIndex > -1, 'expected the user radius axis to exist')
    assert.ok(
      minimalLockIndex > radiusAxisIndex,
      'geili-minimal must re-lock --radius after user radius selectors'
    )
  })

  test('theme drawer option controls stay flat', () => {
    const source = readSource('src/components/config-drawer.tsx')

    assert.equal(source.includes('group-data-checked:shadow-md'), false)
    assert.equal(
      source.includes('group-data-checked:shadow-[var(--shadow-card)]'),
      false
    )
    assert.ok(
      source.includes("customization.preset !== 'geili-minimal'"),
      'geili-minimal should hide the user radius axis'
    )
  })

  test('theme preset labels are translated instead of leaking internal keys', () => {
    const locales = ['zh', 'en', 'fr', 'ja', 'ru', 'vi']

    for (const locale of locales) {
      const source = readSource(`src/i18n/locales/${locale}.json`)
      const messages = JSON.parse(source).translation as Record<string, string>

      assert.ok(messages['preset.geili-minimal'])
      assert.ok(messages['preset.geili-modern'])
      assert.notEqual(messages['preset.geili-minimal'], 'preset.geili-minimal')
      assert.notEqual(messages['preset.geili-modern'], 'preset.geili-modern')
    }
  })

  test('usage-log badges and stat chips use semantic pill geometry', () => {
    const files = [
      'src/components/group-badge.tsx',
      'src/features/usage-logs/components/common-logs-stats.tsx',
      'src/features/usage-logs/components/model-badge.tsx',
      'src/features/usage-logs/components/columns/column-helpers.tsx',
      'src/features/usage-logs/components/columns/common-logs-columns.tsx',
    ]

    for (const file of files) {
      const source = readSource(file)
      assert.equal(
        source.includes('rounded-md'),
        false,
        `${file} should not override badges/chips to rounded-md`
      )
    }
  })

  test('single-line controls keep pill geometry after consumer classes', () => {
    const button = Button({ className: 'rounded-lg', children: 'Save' })
    const input = Input({ className: 'rounded-lg' })

    assert.equal(button.props.className.includes('rounded-lg'), false)
    assert.equal(button.props.className.includes('rounded-full'), true)
    assert.equal(input.props.className.includes('rounded-lg'), false)
    assert.equal(input.props.className.includes('rounded-full'), true)
  })

  test('shared selectors and drawer helpers use semantic geometry', () => {
    const forbiddenByFile: Record<string, string[]> = {
      'src/components/data-table/toolbar/faceted-filter.tsx': [
        'rounded-sm px-1',
        'rounded-[calc(var(--radius)*0.45)]',
      ],
      'src/components/model-group-selector.tsx': [
        'rounded-lg',
        '!shadow-none',
      ],
      'src/components/drawer-layout.ts': ['rounded-md'],
      'src/components/layout/constants.ts': ['rounded-lg', '--shadow-card'],
      'src/components/tag-input.tsx': ['rounded-md', 'shadow-xs'],
      'src/components/json-code-editor.tsx': ['rounded-lg'],
      'src/components/theme-quick-switcher.tsx': ['rounded-lg', 'rounded-md'],
    }

    for (const [file, forbiddenTokens] of Object.entries(forbiddenByFile)) {
      const source = readSource(file)

      for (const token of forbiddenTokens) {
        assert.equal(source.includes(token), false, `${file} still has ${token}`)
      }
    }
  })
})
