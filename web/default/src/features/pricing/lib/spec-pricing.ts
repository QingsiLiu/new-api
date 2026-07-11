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
import { formatCNYAmount } from '@/lib/currency'
import type { PricingModel } from '../types'

const IMAGE_RESOLUTION_ORDER = ['1k', '2k', '4k'] as const

const VIDEO_MODE_LABEL_KEYS: Record<string, string> = {
  no_video_input: 'No video input',
  with_video_input: 'With video input',
  text_audio: 'Text with audio',
  text_no_audio: 'Text without audio',
  image_audio: 'Image with audio',
  image_no_audio: 'Image without audio',
}

export type ImageSpecResolutionPrice = {
  cny_per_image?: number | null
  cny_per_second?: number | null
}

export type VideoMatrixModePrice = {
  cny_per_second?: number | null
  unsupported?: boolean
}

export type ImageSpecPricing = Record<string, ImageSpecResolutionPrice>
export type VideoMatrixPricing = Record<
  string,
  Record<string, Record<string, VideoMatrixModePrice>>
>

export type ImageSpecPriceRow = {
  resolution: string
  label: string
  priceCNY: number
  formattedPrice: string
}

export type ImageSpecDefaultPriceRow = {
  labelKey: string
  priceCNY: number
  formattedPrice: string
}

export type VideoMatrixPriceRow = {
  resolution: string
  ratio: string
  mode: string
  modeLabelKey: string
  supported: boolean
  priceCNY: number | null
  formattedPrice: string
  labelKey: string
}

export type ModelSpecPricingSummary =
  | {
      mode: 'image_spec'
      labelKey: string
      entries: Array<{ label: string; formattedPrice: string }>
    }
  | {
      mode: 'video_matrix'
      labelKey: string
      startPrice: string
      unitKey: string
    }
  | {
      mode: 'free'
      labelKey: string
      formattedPrice: string
    }

export function getModelSpecPricingSummary(
  model: PricingModel
): ModelSpecPricingSummary | null {
  if (model.pricing_mode === 'image_spec') {
    return {
      mode: 'image_spec',
      labelKey: 'Image generation',
      entries: getImageSpecPriceRows(model)
        .slice(0, 3)
        .map((row) => ({
          label: row.label,
          formattedPrice: row.formattedPrice,
        })),
    }
  }

  if (model.pricing_mode === 'video_matrix') {
    return {
      mode: 'video_matrix',
      labelKey: 'Video generation',
      startPrice: formatSpecCNY(getVideoStartPriceCNY(model)),
      unitKey: 'second',
    }
  }

  if (model.pricing_mode === 'free') {
    return {
      mode: 'free',
      labelKey: 'Free',
      formattedPrice: formatSpecCNY(0),
    }
  }

  return null
}

export function getImageSpecPriceRows(model: PricingModel): ImageSpecPriceRow[] {
  const specPricing = asImageSpecPricing(model.spec_pricing)
  if (!specPricing) {
    return []
  }

  return Object.entries(specPricing)
    .filter((entry): entry is [string, ImageSpecResolutionPrice] =>
      hasFiniteNumber(entry[1].cny_per_image)
    )
    .sort(([left], [right]) => compareResolution(left, right))
    .map(([resolution, price]) => ({
      resolution,
      label: formatResolutionLabel(resolution),
      priceCNY: Number(price.cny_per_image),
      formattedPrice: formatSpecCNY(Number(price.cny_per_image)),
    }))
}

export function getImageSpecDefaultPriceRow(
  model: PricingModel
): ImageSpecDefaultPriceRow | null {
  if (!hasFiniteNumber(model.amount_cny)) {
    return null
  }
  return {
    labelKey: 'Default price',
    priceCNY: Number(model.amount_cny),
    formattedPrice: formatSpecCNY(Number(model.amount_cny)),
  }
}

export function getVideoMatrixPriceRows(
  model: PricingModel
): VideoMatrixPriceRow[] {
  const specPricing = asVideoMatrixPricing(model.spec_pricing)
  if (!specPricing) {
    return []
  }

  const rows: VideoMatrixPriceRow[] = []
  for (const [resolution, ratioMap] of Object.entries(specPricing)) {
    for (const [ratio, modeMap] of Object.entries(ratioMap)) {
      for (const [mode, price] of Object.entries(modeMap)) {
        const supported = !price.unsupported && hasFiniteNumber(price.cny_per_second)
        const priceCNY = supported ? Number(price.cny_per_second) : null
        rows.push({
          resolution,
          ratio,
          mode,
          modeLabelKey: VIDEO_MODE_LABEL_KEYS[mode] ?? mode,
          supported,
          priceCNY,
          formattedPrice: supported ? formatSpecCNY(priceCNY) : '-',
          labelKey: supported ? '' : 'Unsupported',
        })
      }
    }
  }

  return rows.sort((left, right) => {
    const resolutionOrder = compareResolution(left.resolution, right.resolution)
    if (resolutionOrder !== 0) return resolutionOrder
    const ratioOrder = left.ratio.localeCompare(right.ratio, undefined, {
      numeric: true,
    })
    if (ratioOrder !== 0) return ratioOrder
    return left.mode.localeCompare(right.mode)
  })
}

function getVideoStartPriceCNY(model: PricingModel): number | null {
  if (hasFiniteNumber(model.amount_cny)) {
    return Number(model.amount_cny)
  }
  const prices = getVideoMatrixPriceRows(model)
    .map((row) => row.priceCNY)
    .filter((value): value is number => hasFiniteNumber(value))
  if (prices.length === 0) {
    return null
  }
  return Math.min(...prices)
}

function asImageSpecPricing(value: unknown): ImageSpecPricing | null {
  if (!isRecord(value)) return null
  return value as ImageSpecPricing
}

function asVideoMatrixPricing(value: unknown): VideoMatrixPricing | null {
  if (!isRecord(value)) return null
  return value as VideoMatrixPricing
}

function compareResolution(left: string, right: string): number {
  const leftIndex = IMAGE_RESOLUTION_ORDER.indexOf(
    left as (typeof IMAGE_RESOLUTION_ORDER)[number]
  )
  const rightIndex = IMAGE_RESOLUTION_ORDER.indexOf(
    right as (typeof IMAGE_RESOLUTION_ORDER)[number]
  )
  if (leftIndex >= 0 || rightIndex >= 0) {
    if (leftIndex < 0) return 1
    if (rightIndex < 0) return -1
    return leftIndex - rightIndex
  }
  return left.localeCompare(right, undefined, { numeric: true })
}

function formatResolutionLabel(resolution: string): string {
  return resolution.toUpperCase()
}

function formatSpecCNY(value: number | null | undefined): string {
  return formatCNYAmount(value, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: false,
  })
}

function hasFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}
