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
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import type { PricingModel } from '../types'

type ImageSpecResolutionPrice = {
  cny_per_image?: number
}

type ImageSpecPricingConfig = {
  mode?: string
  unit?: string
  resolutions?: Record<string, ImageSpecResolutionPrice>
  default_cny_per_image?: number
}

export type ImageSpecPriceRow = {
  resolution: string
  cnyPerImage: number
}

export type ImageSpecPriceDisplayItem = {
  label: string
  cnyPerImage: number
  formatted: string
}

export function parseImageSpecPricingConfig(
  raw?: string
): ImageSpecPricingConfig {
  if (!raw?.trim()) return {}
  try {
    const parsed = JSON.parse(raw) as ImageSpecPricingConfig
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }
    return parsed
  } catch {
    return {}
  }
}

export function isImageSpecPricingModel(model: PricingModel): boolean {
  const config = parseImageSpecPricingConfig(model.pricing_config)
  return (model.pricing_mode || config.mode) === 'image_spec'
}

export function getImageSpecPriceRows(
  model: PricingModel
): ImageSpecPriceRow[] {
  if (!isImageSpecPricingModel(model)) return []
  const config = parseImageSpecPricingConfig(model.pricing_config)
  return Object.entries(config.resolutions || {})
    .map(([resolution, price]) => ({
      resolution: normalizeResolutionLabel(resolution),
      cnyPerImage: Number(price?.cny_per_image),
    }))
    .filter((row) => Number.isFinite(row.cnyPerImage))
    .sort(
      (a, b) =>
        resolutionSortValue(a.resolution) - resolutionSortValue(b.resolution)
    )
}

export function getDefaultImageSpecPrice(model: PricingModel): number | null {
  if (!isImageSpecPricingModel(model)) return null
  const config = parseImageSpecPricingConfig(model.pricing_config)
  const value = Number(config.default_cny_per_image)
  return Number.isFinite(value) ? value : null
}

export function formatImageSpecPrice(value: number): string {
  return formatBillingCurrencyFromUSD(value, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: false,
  })
}

export function getImageSpecPriceDisplayItems(
  model: PricingModel
): ImageSpecPriceDisplayItem[] {
  const rows = getImageSpecPriceRows(model)
  if (rows.length > 0) {
    return rows.map((row) => ({
      label: row.resolution,
      cnyPerImage: row.cnyPerImage,
      formatted: formatImageSpecPrice(row.cnyPerImage),
    }))
  }

  const defaultPrice = getDefaultImageSpecPrice(model)
  if (defaultPrice == null) return []
  return [
    {
      label: 'Default',
      cnyPerImage: defaultPrice,
      formatted: formatImageSpecPrice(defaultPrice),
    },
  ]
}

function normalizeResolutionLabel(value: string): string {
  const trimmed = value.trim()
  if (/^\d+k$/i.test(trimmed)) return trimmed.toUpperCase()
  return trimmed
}

function resolutionSortValue(value: string): number {
  const match = value.match(/^(\d+(?:\.\d+)?)K$/i)
  if (match) return Number(match[1]) * 1000
  return Number.MAX_SAFE_INTEGER
}
