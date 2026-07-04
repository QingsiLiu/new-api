import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { getSafePostLoginRedirect } from './use-auth-redirect'

describe('getSafePostLoginRedirect', () => {
  test('keeps ordinary user redirects on user-accessible pages', () => {
    assert.equal(getSafePostLoginRedirect('/keys', 1), '/keys')
    assert.equal(getSafePostLoginRedirect('/dashboard', 1), '/dashboard')
    assert.equal(getSafePostLoginRedirect(undefined, 1), '/dashboard')
  })

  test('sends non-admin users away from admin-only redirects', () => {
    assert.equal(getSafePostLoginRedirect('/channels', 1), '/dashboard')
    assert.equal(getSafePostLoginRedirect('/models/metadata', 1), '/dashboard')
    assert.equal(getSafePostLoginRedirect('/users?page=1', 1), '/dashboard')
    assert.equal(
      getSafePostLoginRedirect('/system-settings/site', 1),
      '/dashboard'
    )
  })

  test('sends non-admin users away from stale forbidden redirects', () => {
    assert.equal(getSafePostLoginRedirect('/403', 1), '/dashboard')
  })

  test('preserves admin redirects for admins', () => {
    assert.equal(getSafePostLoginRedirect('/channels', 10), '/channels')
    assert.equal(
      getSafePostLoginRedirect('/system-settings/site', 100),
      '/system-settings/site'
    )
  })
})
