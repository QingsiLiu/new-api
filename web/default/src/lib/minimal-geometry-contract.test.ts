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

  test('sidebar pill navigation rows keep a visible gap', () => {
    const source = readSource('src/components/ui/sidebar.tsx')

    assert.ok(
      source.includes('flex w-full min-w-0 flex-col gap-1'),
      'adjacent sidebar pill rows need a real gap so active/hover backgrounds do not merge'
    )
  })

  test('table badge containers do not use negative offsets that clip pills', () => {
    const badgeCell = readSource('src/components/data-table/core/badge-cell.tsx')
    const badgeListCell = readSource(
      'src/components/data-table/core/badge-list-cell.tsx'
    )
    const channelsColumns = readSource(
      'src/features/channels/components/channels-columns.tsx'
    )
    const channelActions = readSource(
      'src/features/channels/components/data-table-row-actions.tsx'
    )

    assert.equal(badgeCell.includes('-ml-1.5'), false)
    assert.equal(badgeListCell.includes('-ml-1.5'), false)
    assert.equal(channelsColumns.includes('-ml-1.5'), false)
    assert.equal(channelActions.includes('-ml-1.5'), false)
  })

  test('channel actions column has enough width for three pill icon buttons', () => {
    const source = readSource(
      'src/features/channels/components/channels-columns.tsx'
    )
    const actionsColumn = source.match(
      /id: 'actions'[\s\S]*?enableHiding: false/
    )?.[0]

    assert.ok(actionsColumn, 'expected channels actions column definition')
    assert.match(
      actionsColumn,
      /size:\s*(?:1[6-9]\d|[2-9]\d{2})/,
      'sticky actions column must reserve scale-safe width'
    )
  })

  test('pinned table headers use an opaque surface background', () => {
    const source = readSource('src/components/data-table/core/column-pinning.ts')

    assert.ok(
      source.includes("kind === 'header'\n      ? 'z-30 !bg-card"),
      'sticky header cells must mask the columns beneath them'
    )
    assert.equal(source.includes('[background-color:'), false)
  })

  test('theme density axis applies compact default instead of dropping it', () => {
    const source = readSource(
      'src/context/theme-customization-provider.tsx'
    )

    assert.ok(
      source.includes("scale === 'default' ? null : scale"),
      'only the explicit Default density option should remove data-theme-scale'
    )
    assert.equal(
      source.includes(
        'scale === DEFAULT_THEME_CUSTOMIZATION.scale ? null : scale'
      ),
      false
    )
  })
})
