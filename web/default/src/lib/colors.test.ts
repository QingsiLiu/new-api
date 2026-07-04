import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'
import {
  avatarColorMap,
  CHART_COLORS,
  colorToBgClass,
  stringToColor,
} from './colors'
import {
  badgeSurfaceMap,
  dotColorMap,
  textColorMap,
} from '@/components/status-badge'

const statusBadgeSource = readFileSync('src/components/status-badge.tsx', 'utf8')

describe('semantic data colors', () => {
  test('maps color names to the geili-modern vivid token identity', () => {
    assert.equal(colorToBgClass.blue, 'bg-chart-5')
    assert.equal(colorToBgClass.purple, 'bg-chart-2')
    assert.equal(colorToBgClass.violet, 'bg-chart-2')
    assert.equal(colorToBgClass.indigo, 'bg-chart-2')
    assert.equal(colorToBgClass.cyan, 'bg-chart-3')
    assert.equal(colorToBgClass.teal, 'bg-chart-3')
    assert.equal(colorToBgClass.pink, 'bg-chart-1')
    assert.equal(colorToBgClass.lime, 'bg-success')
  })

  test('uses matching soft surfaces for avatars and badges', () => {
    assert.equal(avatarColorMap.blue, 'bg-chart-5/10 text-chart-5')
    assert.equal(avatarColorMap.violet, 'bg-chart-2/10 text-chart-2')
    assert.equal(avatarColorMap.teal, 'bg-chart-3/10 text-chart-3')
    assert.equal(avatarColorMap.pink, 'bg-chart-1/10 text-chart-1')

    assert.equal(dotColorMap.blue, 'bg-chart-5')
    assert.equal(textColorMap.violet, 'text-chart-2')
    assert.equal(badgeSurfaceMap.teal, 'bg-chart-3/15')
    assert.equal(badgeSurfaceMap.pink, 'bg-chart-1/15')
    for (const surfaceClass of Object.values(badgeSurfaceMap)) {
      assert.doesNotMatch(surfaceClass, /\bborder(?:-|$)/)
    }
  })

  test('status badges are soft-filled without default dots or borders', () => {
    assert.match(statusBadgeSource, /showDot = false/)
    assert.doesNotMatch(statusBadgeSource, /'rounded-full border'/)
  })

  test('keeps stable string-to-color assignment inside the semantic palette', () => {
    const first = stringToColor('gpt-5')
    const second = stringToColor('gpt-5')

    assert.equal(first, second)
    assert.ok(Object.hasOwn(colorToBgClass, first))
  })

  test('charts expose the five CSS token colors in order', () => {
    assert.deepEqual(CHART_COLORS, [
      'var(--chart-1)',
      'var(--chart-2)',
      'var(--chart-3)',
      'var(--chart-4)',
      'var(--chart-5)',
    ])
  })
})
