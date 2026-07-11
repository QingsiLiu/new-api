import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  LAYOUT_DEFAULTS_VERSION,
  resolveVersionedPreferenceDefault,
  THEME_DEFAULTS_VERSION,
} from './preference-defaults'
import { DEFAULT_THEME_CUSTOMIZATION } from './theme-customization'

const densityValues = new Set(['default', 'sm', 'lg', 'xl'])

describe('resolveVersionedPreferenceDefault', () => {
  test('uses a fresh theme defaults version for geili-minimal migration', () => {
    assert.equal(THEME_DEFAULTS_VERSION, 'geili-minimal-v1')
  })

  test('uses floating sidebar as the current layout defaults version', () => {
    assert.equal(LAYOUT_DEFAULTS_VERSION, 'floating-v1')
  })

  test('keeps theme drawer defaults aligned to the Geili compact document-like preset', () => {
    assert.equal(DEFAULT_THEME_CUSTOMIZATION.scale, 'sm')
    assert.equal(DEFAULT_THEME_CUSTOMIZATION.contentLayout, 'full')
  })

  test('moves a stale legacy default cookie to the new default', () => {
    assert.equal(
      resolveVersionedPreferenceDefault({
        savedValue: 'default',
        allowedValues: densityValues,
        fallback: 'sm',
        legacyDefault: 'default',
        shouldMigrateLegacyDefault: true,
      }),
      'sm'
    )
  })

  test('keeps explicit non-default choices during migration', () => {
    assert.equal(
      resolveVersionedPreferenceDefault({
        savedValue: 'lg',
        allowedValues: densityValues,
        fallback: 'sm',
        legacyDefault: 'default',
        shouldMigrateLegacyDefault: true,
      }),
      'lg'
    )
  })

  test('keeps the legacy value after the defaults version has been recorded', () => {
    assert.equal(
      resolveVersionedPreferenceDefault({
        savedValue: 'default',
        allowedValues: densityValues,
        fallback: 'sm',
        legacyDefault: 'default',
        shouldMigrateLegacyDefault: false,
      }),
      'default'
    )
  })

  test('falls back when the saved value is missing or invalid', () => {
    assert.equal(
      resolveVersionedPreferenceDefault({
        savedValue: undefined,
        allowedValues: densityValues,
        fallback: 'sm',
        legacyDefault: 'default',
        shouldMigrateLegacyDefault: true,
      }),
      'sm'
    )
    assert.equal(
      resolveVersionedPreferenceDefault({
        savedValue: 'tiny',
        allowedValues: densityValues,
        fallback: 'sm',
        legacyDefault: 'default',
        shouldMigrateLegacyDefault: true,
      }),
      'sm'
    )
  })
})
