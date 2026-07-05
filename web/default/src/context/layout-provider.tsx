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
import { createContext, useContext, useEffect, useState } from 'react'
import { getCookie, removeCookie, setCookie } from '@/lib/cookies'
import {
  LAYOUT_DEFAULTS_VERSION,
  LAYOUT_DEFAULTS_VERSION_COOKIE_NAME,
  resolveVersionedPreferenceDefault,
  shouldMigratePreferenceDefaults,
} from '@/lib/preference-defaults'

export type Collapsible = 'offcanvas' | 'icon' | 'none'
export type Variant = 'inset' | 'sidebar' | 'floating'

// Cookie constants following the pattern from sidebar.tsx
const LAYOUT_COLLAPSIBLE_COOKIE_NAME = 'layout_collapsible'
const LAYOUT_VARIANT_COOKIE_NAME = 'layout_variant'
const LAYOUT_COOKIE_MAX_AGE = 60 * 60 * 24 * 7 // 7 days

// Default values
const DEFAULT_VARIANT = 'sidebar'
const DEFAULT_COLLAPSIBLE = 'icon'

const LAYOUT_COLLAPSIBLE_VALUES: ReadonlySet<Collapsible> = new Set([
  'offcanvas',
  'icon',
  'none',
])

const LAYOUT_VARIANT_VALUES: ReadonlySet<Variant> = new Set([
  'inset',
  'sidebar',
  'floating',
])

type LayoutContextType = {
  resetLayout: () => void

  defaultCollapsible: Collapsible
  collapsible: Collapsible
  setCollapsible: (collapsible: Collapsible) => void

  defaultVariant: Variant
  variant: Variant
  setVariant: (variant: Variant) => void
}

const LayoutContext = createContext<LayoutContextType | null>(null)

function shouldMigrateLayoutDefaults() {
  return shouldMigratePreferenceDefaults(
    getCookie(LAYOUT_DEFAULTS_VERSION_COOKIE_NAME),
    LAYOUT_DEFAULTS_VERSION
  )
}

function readLayoutCookie<T extends string>(
  name: string,
  allowed: ReadonlySet<T>,
  fallback: T,
  migration?: {
    legacyDefault: T
    shouldMigrateLegacyDefault: boolean
  }
): T {
  return resolveVersionedPreferenceDefault({
    savedValue: getCookie(name),
    allowedValues: allowed,
    fallback,
    legacyDefault: migration?.legacyDefault,
    shouldMigrateLegacyDefault:
      migration?.shouldMigrateLegacyDefault ?? false,
  })
}

type LayoutProviderProps = {
  children: React.ReactNode
}

export function LayoutProvider({ children }: LayoutProviderProps) {
  const [collapsible, _setCollapsible] = useState<Collapsible>(() =>
    readLayoutCookie(
      LAYOUT_COLLAPSIBLE_COOKIE_NAME,
      LAYOUT_COLLAPSIBLE_VALUES,
      DEFAULT_COLLAPSIBLE
    )
  )

  const [variant, _setVariant] = useState<Variant>(() =>
    readLayoutCookie(
      LAYOUT_VARIANT_COOKIE_NAME,
      LAYOUT_VARIANT_VALUES,
      DEFAULT_VARIANT,
      {
        legacyDefault: 'inset',
        shouldMigrateLegacyDefault: shouldMigrateLayoutDefaults(),
      }
    )
  )

  useEffect(() => {
    if (shouldMigrateLayoutDefaults()) {
      if (getCookie(LAYOUT_VARIANT_COOKIE_NAME) === 'inset') {
        removeCookie(LAYOUT_VARIANT_COOKIE_NAME)
      }
    }
    setCookie(
      LAYOUT_DEFAULTS_VERSION_COOKIE_NAME,
      LAYOUT_DEFAULTS_VERSION,
      LAYOUT_COOKIE_MAX_AGE
    )
  }, [])

  const setCollapsible = (newCollapsible: Collapsible) => {
    _setCollapsible(newCollapsible)
    setCookie(
      LAYOUT_COLLAPSIBLE_COOKIE_NAME,
      newCollapsible,
      LAYOUT_COOKIE_MAX_AGE
    )
  }

  const setVariant = (newVariant: Variant) => {
    _setVariant(newVariant)
    setCookie(LAYOUT_VARIANT_COOKIE_NAME, newVariant, LAYOUT_COOKIE_MAX_AGE)
  }

  const resetLayout = () => {
    setCollapsible(DEFAULT_COLLAPSIBLE)
    setVariant(DEFAULT_VARIANT)
  }

  const contextValue: LayoutContextType = {
    resetLayout,
    defaultCollapsible: DEFAULT_COLLAPSIBLE,
    collapsible,
    setCollapsible,
    defaultVariant: DEFAULT_VARIANT,
    variant,
    setVariant,
  }

  return <LayoutContext value={contextValue}>{children}</LayoutContext>
}

// Define the hook for the provider
// eslint-disable-next-line react-refresh/only-export-components
export function useLayout() {
  const context = useContext(LayoutContext)
  if (!context) {
    throw new Error('useLayout must be used within a LayoutProvider')
  }
  return context
}
