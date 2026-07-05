import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { VCHART_OPTION } from './vchart'

describe('VCHART_OPTION', () => {
  test('uses geili chart CSS variables as the default data scheme', () => {
    assert.deepEqual(VCHART_OPTION.theme.colorScheme.default.dataScheme, [
      'var(--chart-1)',
      'var(--chart-2)',
      'var(--chart-3)',
      'var(--chart-4)',
      'var(--chart-5)',
    ])
  })
})
