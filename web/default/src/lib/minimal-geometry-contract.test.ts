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
})
