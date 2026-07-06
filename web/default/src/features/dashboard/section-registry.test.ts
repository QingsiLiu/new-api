import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { canAccessDashboardSection } from './section-registry'

describe('canAccessDashboardSection', () => {
  test('allows regular dashboard sections for signed-in users', () => {
    assert.equal(canAccessDashboardSection('overview', 1), true)
    assert.equal(canAccessDashboardSection('models', 1), true)
  })

  test('blocks the user analytics section for non-admin users', () => {
    assert.equal(canAccessDashboardSection('users', undefined), false)
    assert.equal(canAccessDashboardSection('users', 1), false)
  })

  test('allows the user analytics section for admins', () => {
    assert.equal(canAccessDashboardSection('users', 10), true)
    assert.equal(canAccessDashboardSection('users', 100), true)
  })

  test('rejects unknown dashboard sections', () => {
    assert.equal(canAccessDashboardSection('unknown', 100), false)
  })
})
