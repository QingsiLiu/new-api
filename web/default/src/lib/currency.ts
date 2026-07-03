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
/**
 * ============================================================================
 * Currency Formatting Library
 * ============================================================================
 *
 * This module provides CNY-native formatting utilities for balances, charges,
 * topups, and billing previews. Legacy function names are kept so older call
 * sites can migrate without a broad rename.
 *
 * ## Key Concepts
 *
 * 1. **CNY**: The public money unit shown to users and admins.
 * 2. **Internal billing units**: Database integer units, where ¥1 = 100,000.
 * 3. **Legacy config fields**: Old exchange-rate settings may still be present
 *    in API payloads, but display code ignores them.
 *
 * ## When to Use Each Function
 *
 * - `formatCNYAmount()`: Use for CNY amounts.
 * - `formatCurrencyFromUSD()`: Legacy name; formats the input as CNY.
 * - `formatBillingCurrencyFromUSD()`: Legacy name; formats the input as CNY.
 * - `formatLocalCurrencyAmount()`: Formats the input as CNY.
 * - `formatQuotaWithCurrency()`: Converts internal billing units to CNY.
 *
 * ## Quick Reference Guide
 *
 * | Scenario | Input Type | Function to Use | Why |
 * |----------|-----------|-----------------|-----|
 * | User balance display | CNY units or CNY amount | `formatQuotaWithCurrency()` / `formatCNYAmount()` |
 * | Recharge option button | CNY amount | `formatCNYAmount()` |
 * | Payment confirmation | CNY amount | `formatLocalCurrencyAmount()` |
 * | Billing history Amount | CNY amount | `formatCNYAmount()` |
 * | Model pricing | CNY amount | `formatBillingCurrencyFromUSD()` |
 * | Internal billing units | Integer units | `formatQuotaWithCurrency()` |
 *
 * ## Critical Rules
 *
 * 1. **Never apply exchange rates**: CNY amounts are already final money values.
 * 2. **Internal billing units**: Convert with ¥1 = 100,000 units.
 * 3. **Billing displays**: Always show `¥`.
 */
import {
  useSystemConfigStore,
  DEFAULT_CURRENCY_CONFIG,
  type CurrencyConfig,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

export interface CurrencyFormatOptions {
  /** Fraction digits to use when |value| >= 1 */
  digitsLarge?: number
  /** Fraction digits to use when |value| < 1 */
  digitsSmall?: number
  /** Whether to abbreviate thousands with k suffix */
  abbreviate?: boolean
  /** Minimal absolute value to display when rounding would produce zero */
  minimumNonZero?: number
}

export const CNY_QUOTA_UNIT = 100000

type DisplayMeta =
  | {
      kind: 'currency'
      symbol: string
      currencyCode: string
      exchangeRate: number
    }
  | {
      kind: 'custom'
      symbol: string
      exchangeRate: number
    }
  | {
      kind: 'tokens'
      /** Number of tokens per USD */
      quotaPerUnit: number
    }

const DEFAULT_FORMAT_OPTIONS: Required<CurrencyFormatOptions> = {
  digitsLarge: 2,
  digitsSmall: 4,
  abbreviate: true,
  minimumNonZero: 0,
}

const DISPLAY_TYPE_VALUES = ['USD', 'CNY', 'TOKENS', 'CUSTOM'] as const
type DisplayTypeLiteral = (typeof DISPLAY_TYPE_VALUES)[number]

export function isCurrencyDisplayType(
  value: unknown
): value is CurrencyDisplayType {
  return (
    typeof value === 'string' &&
    DISPLAY_TYPE_VALUES.includes(value as DisplayTypeLiteral)
  )
}

export function parseCurrencyDisplayType(
  value: unknown,
  fallback: CurrencyDisplayType = 'CNY'
): CurrencyDisplayType {
  return isCurrencyDisplayType(value) ? value : fallback
}

function getConfig(): CurrencyConfig {
  const { config } = useSystemConfigStore.getState()
  const currency = config?.currency ?? DEFAULT_CURRENCY_CONFIG
  return {
    ...DEFAULT_CURRENCY_CONFIG,
    ...currency,
    quotaDisplayType: 'CNY',
    quotaPerUnit: CNY_QUOTA_UNIT,
    usdExchangeRate: 1,
    customCurrencyExchangeRate: 1,
    customCurrencySymbol:
      currency?.customCurrencySymbol?.trim() ||
      DEFAULT_CURRENCY_CONFIG.customCurrencySymbol,
  }
}

function getDisplayMeta(config: CurrencyConfig): DisplayMeta {
  void config
  return {
    kind: 'currency',
    symbol: '¥',
    currencyCode: 'CNY',
    exchangeRate: 1,
  }
}

function getBillingDisplayMeta(config: CurrencyConfig): DisplayMeta {
  return getDisplayMeta(config)
}

function mergeOptions(
  options?: CurrencyFormatOptions
): Required<CurrencyFormatOptions> {
  if (!options) return DEFAULT_FORMAT_OPTIONS
  return {
    digitsLarge: options.digitsLarge ?? DEFAULT_FORMAT_OPTIONS.digitsLarge,
    digitsSmall: options.digitsSmall ?? DEFAULT_FORMAT_OPTIONS.digitsSmall,
    abbreviate: options.abbreviate ?? DEFAULT_FORMAT_OPTIONS.abbreviate,
    minimumNonZero:
      options.minimumNonZero ?? DEFAULT_FORMAT_OPTIONS.minimumNonZero,
  }
}

function removeTrailingZeros(str: string): string {
  if (!str.includes('.')) return str
  return str.replace(/(\.[0-9]*?)0+$/, '$1').replace(/\.$/, '')
}

function formatNumberWithSuffix(
  value: number,
  digitsLarge: number,
  digitsSmall: number,
  abbreviate: boolean
): string {
  const abs = Math.abs(value)
  if (abbreviate && abs >= 1000) {
    const result = value / 1000
    return removeTrailingZeros(result.toFixed(1)) + 'k'
  }

  const digits = abs >= 1 ? digitsLarge : digitsSmall
  return removeTrailingZeros(value.toFixed(digits))
}

function adjustForMinimum(
  value: number,
  digits: number,
  minimumNonZero: number
): number {
  if (value === 0) return value

  const threshold = minimumNonZero > 0 ? minimumNonZero : Math.pow(10, -digits)
  const abs = Math.abs(value)
  if (abs > 0 && abs < threshold) {
    return value > 0 ? threshold : -threshold
  }
  return value
}

function formatCurrencyValue(
  value: number,
  options: Required<CurrencyFormatOptions>,
  meta: DisplayMeta
): string {
  if (meta.kind === 'tokens') {
    return formatNumberWithSuffix(
      value,
      options.digitsLarge,
      options.digitsSmall,
      options.abbreviate
    )
  }

  const digits =
    Math.abs(value) >= 1 ? options.digitsLarge : options.digitsSmall
  const adjustedValue = adjustForMinimum(value, digits, options.minimumNonZero)

  if (meta.kind === 'currency') {
    const formatted = new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: meta.currencyCode,
      currencyDisplay: 'narrowSymbol',
      minimumFractionDigits: 0,
      maximumFractionDigits: digits,
    }).format(adjustedValue)
    return formatted
  }

  const decimal = new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: digits,
  }).format(adjustedValue)

  return `${meta.symbol} ${decimal}`
}

/**
 * Get the current currency configuration and display metadata.
 *
 * @returns Object containing config and display metadata
 *
 * @internal
 * This is primarily for internal use. Most consumers should use the
 * higher-level formatting functions instead.
 */
export function getCurrencyDisplay() {
  const config = getConfig()
  const meta = getDisplayMeta(config)
  return { config, meta }
}

/**
 * Format a CNY amount. This is the preferred formatter for public money.
 */
export function formatCNYAmount(
  amountCNY: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (amountCNY == null || Number.isNaN(amountCNY)) return '-'

  const { meta } = getCurrencyDisplay()
  const merged = mergeOptions(options)

  return formatCurrencyValue(amountCNY, merged, meta)
}

/**
 * Legacy name retained for older call sites. The input is now treated as CNY.
 */
export function formatCurrencyFromUSD(
  amountUSD: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  return formatCNYAmount(amountUSD, options)
}

/**
 * Legacy name retained for older call sites. The input is now treated as CNY.
 */
export function formatBillingCurrencyFromUSD(
  amountUSD: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  return formatCNYAmount(amountUSD, options)
}

/**
 * Format internal billing units as CNY. ¥1 = 100,000 units.
 */
export function formatQuotaWithCurrency(
  quota: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (quota == null || Number.isNaN(quota)) return '-'

  const { config } = getCurrencyDisplay()
  const amountCNY = quota / config.quotaPerUnit
  return formatCNYAmount(amountCNY, options)
}

/**
 * Get the current currency label for UI display.
 *
 * Returns the public money label used by wallet and billing UI.
 */
export function getCurrencyLabel(): string {
  return 'CNY'
}

/**
 * Check if currency display is enabled (not in token-only mode).
 *
 * @returns True if displaying in actual currency (USD/CNY/etc), false if tokens only
 *
 * @example
 * // With quotaDisplayType: 'USD' or 'CNY'
 * isCurrencyDisplayEnabled() → true
 *
 * // With quotaDisplayType: 'TOKENS'
 * isCurrencyDisplayEnabled() → false
 *
 * @remarks
 * Use this to conditionally show currency-specific UI elements
 */
export function isCurrencyDisplayEnabled(): boolean {
  return true
}

/**
 * Format an amount that is already in CNY.
 */
export function formatLocalCurrencyAmount(
  amount: number | null | undefined,
  options?: CurrencyFormatOptions
): string {
  if (amount == null || Number.isNaN(amount)) return '-'

  const { config } = getCurrencyDisplay()
  const meta = getBillingDisplayMeta(config)
  const merged = mergeOptions(options)

  return formatCurrencyValue(amount, merged, meta)
}
