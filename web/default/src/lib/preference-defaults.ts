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

export const THEME_DEFAULTS_VERSION_COOKIE_NAME = 'theme_defaults_version'
export const THEME_DEFAULTS_VERSION = 'geili-minimal-v1'

export const LAYOUT_DEFAULTS_VERSION_COOKIE_NAME = 'layout_defaults_version'
export const LAYOUT_DEFAULTS_VERSION = 'sidebar-v1'

export function shouldMigratePreferenceDefaults(
  savedVersion: string | undefined,
  currentVersion: string
) {
  return savedVersion !== currentVersion
}

export function resolveVersionedPreferenceDefault<T extends string>(params: {
  savedValue: string | undefined
  allowedValues: ReadonlySet<T>
  fallback: T
  legacyDefault?: T
  shouldMigrateLegacyDefault: boolean
}): T {
  const { savedValue, allowedValues, fallback } = params
  if (!savedValue || !allowedValues.has(savedValue as T)) {
    return fallback
  }

  const value = savedValue as T
  if (
    params.shouldMigrateLegacyDefault &&
    params.legacyDefault !== undefined &&
    value === params.legacyDefault
  ) {
    return fallback
  }

  return value
}
