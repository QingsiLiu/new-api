import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { getPricingGroupDisplayName } from './group-display'

describe('pricing group display', () => {
  test('uses pricing API display names instead of opaque group ids', () => {
    assert.equal(
      getPricingGroupDisplayName('grp_papezrbk', {
        grp_papezrbk: 'Claude【官转】',
      }),
      'Claude【官转】'
    )
  })

  test('falls back to the original group code when no display name exists', () => {
    assert.equal(getPricingGroupDisplayName('Claude', {}), 'Claude')
  })
})
