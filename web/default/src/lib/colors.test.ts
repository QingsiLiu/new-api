import assert from 'node:assert/strict'
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

describe('semantic data colors', () => {
  test('maps color names to the calm harmonized token family', () => {
    assert.equal(colorToBgClass.blue, 'bg-calm-blue-bg')
    assert.equal(colorToBgClass.purple, 'bg-calm-violet-bg')
    assert.equal(colorToBgClass.violet, 'bg-calm-violet-bg')
    assert.equal(colorToBgClass.indigo, 'bg-calm-violet-bg')
    assert.equal(colorToBgClass.cyan, 'bg-calm-teal-bg')
    assert.equal(colorToBgClass.teal, 'bg-calm-teal-bg')
    assert.equal(colorToBgClass.pink, 'bg-calm-rose-bg')
    assert.equal(colorToBgClass.lime, 'bg-calm-green-bg')
  })

  test('uses matching soft surfaces for avatars and badges', () => {
    assert.equal(avatarColorMap.blue, 'bg-calm-blue-bg text-calm-blue-fg')
    assert.equal(
      avatarColorMap.violet,
      'bg-calm-violet-bg text-calm-violet-fg'
    )
    assert.equal(avatarColorMap.teal, 'bg-calm-teal-bg text-calm-teal-fg')
    assert.equal(avatarColorMap.pink, 'bg-calm-rose-bg text-calm-rose-fg')

    assert.equal(dotColorMap.blue, 'bg-calm-blue-fg')
    assert.equal(textColorMap.violet, 'text-calm-violet-fg')
    assert.equal(badgeSurfaceMap.teal, 'bg-calm-teal-bg')
    assert.equal(badgeSurfaceMap.pink, 'bg-calm-rose-bg')
  })

  test('keeps badge color maps out of vivid semantic colors', () => {
    const vividColorPattern = /\b(chart|success|warning|destructive|info)\b/

    for (const value of Object.values(dotColorMap)) {
      assert.doesNotMatch(value, vividColorPattern)
    }
    for (const value of Object.values(textColorMap)) {
      assert.doesNotMatch(value, vividColorPattern)
    }
    for (const value of Object.values(badgeSurfaceMap)) {
      assert.doesNotMatch(value, vividColorPattern)
    }
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
