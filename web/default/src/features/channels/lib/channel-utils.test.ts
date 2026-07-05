import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { getBalanceVariant, getResponseTimeConfig } from './channel-utils'

describe('channel numeric color policy', () => {
  test('keeps response time neutral until it exceeds 10s', () => {
    assert.equal(getResponseTimeConfig(0).variant, 'neutral')
    assert.equal(getResponseTimeConfig(500).variant, 'neutral')
    assert.equal(getResponseTimeConfig(10_000).variant, 'neutral')
    assert.equal(getResponseTimeConfig(10_001).variant, 'danger')
  })

  test('keeps balances neutral regardless of amount', () => {
    assert.equal(getBalanceVariant(0), 'neutral')
    assert.equal(getBalanceVariant(0.5), 'neutral')
    assert.equal(getBalanceVariant(10), 'neutral')
    assert.equal(getBalanceVariant(100), 'neutral')
  })
})
