/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getAuthenticatedSignInRedirect,
  isGeiliAdminHostname,
} from './sign-in-redirect'

describe('getAuthenticatedSignInRedirect', () => {
  test('recognizes the production admin hostname case-insensitively', () => {
    assert.equal(isGeiliAdminHostname('admin.geiliapi.com'), true)
    assert.equal(isGeiliAdminHostname('admin.auapi.ai'), true)
    assert.equal(isGeiliAdminHostname(' ADMIN.AUAPI.AI '), true)
    assert.equal(isGeiliAdminHostname(' ADMIN.GEILIAPI.COM '), true)
    assert.equal(isGeiliAdminHostname('geiliapi.com'), false)
  })

  test('keeps the sign-in page available without cached user state', () => {
    assert.equal(
      getAuthenticatedSignInRedirect(null, undefined, 'admin.geiliapi.com'),
      null
    )
  })

  test('keeps ordinary cached users on the admin sign-in page', () => {
    assert.equal(
      getAuthenticatedSignInRedirect(
        { role: 1 },
        '/dashboard',
        'admin.geiliapi.com'
      ),
      null
    )
  })

  test('redirects verified administrators into the admin console', () => {
    assert.equal(
      getAuthenticatedSignInRedirect(
        { role: 10 },
        undefined,
        'ADMIN.GEILIAPI.COM'
      ),
      '/dashboard'
    )
    assert.equal(
      getAuthenticatedSignInRedirect(
        { role: 100 },
        '/system-settings',
        'admin.geiliapi.com'
      ),
      '/system-settings'
    )
  })

  test('preserves the existing behavior on non-admin hosts', () => {
    assert.equal(
      getAuthenticatedSignInRedirect({ role: 1 }, '/keys', 'example.com'),
      '/keys'
    )
  })
})
