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
import { formatLocalCurrencyAmount, CNY_QUOTA_UNIT } from '@/lib/currency'
import { DEFAULT_DISCOUNT_RATE } from '../constants'

// ============================================================================
// Wallet-specific Formatting Functions
// ============================================================================

/**
 * Format Creem price as public CNY money.
 */
export function formatCreemPrice(
  price: number,
  _currency: 'CNY' | 'USD' | 'EUR'
): string {
  return formatLocalCurrencyAmount(price, {
    digitsLarge: 2,
    digitsSmall: 2,
    abbreviate: false,
  })
}

/**
 * Format internal billing units as a compact CNY amount.
 */
export function formatQuotaShort(quota: number): string {
  return formatLocalCurrencyAmount(quota / CNY_QUOTA_UNIT, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: true,
  })
}

/**
 * Format a CNY amount.
 */
export function formatCurrency(amount: number | string): string {
  const numeric =
    typeof amount === 'number' ? amount : Number.parseFloat(String(amount))
  if (!Number.isFinite(numeric)) return '-'

  return formatLocalCurrencyAmount(numeric, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: false,
  })
}

/**
 * Get discount label for display (e.g., "20% OFF")
 */
export function getDiscountLabel(discount: number): string {
  if (discount >= DEFAULT_DISCOUNT_RATE) {
    return ''
  }
  const off = Math.round((1 - discount) * 100)
  return `${off}% OFF`
}

/**
 * Calculate pricing details for a preset amount
 */
export function calculatePresetPricing(
  presetValue: number,
  _priceRatio: number,
  discount: number,
  _usdExchangeRate: number = 1
) {
  const originalPrice = presetValue
  const actualPrice = originalPrice * discount
  const savedAmount = originalPrice - actualPrice
  const hasDiscount = discount < 1.0
  const displayValue = presetValue

  return {
    displayValue,
    originalPrice,
    actualPrice,
    savedAmount,
    hasDiscount,
  }
}
