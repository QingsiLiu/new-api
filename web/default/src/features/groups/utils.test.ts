import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  groupRegistryScopeForRole,
  normalizeGroupRegistryItems,
} from './utils'

describe('normalizeGroupRegistryItems', () => {
  test('normalizes current-user group map responses without the admin registry endpoint', () => {
    const items = normalizeGroupRegistryItems({
      success: true,
      data: {
        default: {
          desc: 'Default group',
          ratio: 1,
          display_name: 'GPT',
        },
      },
    })

    assert.deepEqual(items, [
      {
        code: 'default',
        display_name: 'GPT',
        description: 'Default group',
        ratio: 1,
        user_usable: true,
        is_reserved: false,
        sort: 0,
      },
    ])
  })

  test('uses the current-user group endpoint for non-admin roles', () => {
    assert.equal(groupRegistryScopeForRole(undefined), 'anonymous')
    assert.equal(groupRegistryScopeForRole(1), 'self')
    assert.equal(groupRegistryScopeForRole(10), 'admin')
    assert.equal(groupRegistryScopeForRole(100), 'admin')
  })
})
