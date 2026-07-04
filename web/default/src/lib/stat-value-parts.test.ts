import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { splitStatValueText } from './stat-value-parts'

describe('splitStatValueText', () => {
  test('separates leading currency symbols', () => {
    assert.deepEqual(splitStatValueText('¥1.23k'), {
      currency: '¥',
      main: '1.23k',
      unit: undefined,
    })
  })

  test('separates compact metric units', () => {
    assert.deepEqual(splitStatValueText('42t/s'), {
      currency: undefined,
      main: '42',
      unit: 't/s',
    })
    assert.deepEqual(splitStatValueText('98%'), {
      currency: undefined,
      main: '98',
      unit: '%',
    })
    assert.deepEqual(splitStatValueText('1.2s'), {
      currency: undefined,
      main: '1.2',
      unit: 's',
    })
  })

  test('keeps plain values intact', () => {
    assert.deepEqual(splitStatValueText('--'), {
      currency: undefined,
      main: '--',
      unit: undefined,
    })
    assert.deepEqual(splitStatValueText('1,234'), {
      currency: undefined,
      main: '1,234',
      unit: undefined,
    })
  })
})
