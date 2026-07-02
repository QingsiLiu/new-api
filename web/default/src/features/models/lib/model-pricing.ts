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
import type { TFunction } from 'i18next'
import type { Model } from '../types'

export const MODEL_MODAL_VALUES = ['text', 'image', 'video', 'audio'] as const
export const MODEL_PRICING_MODE_VALUES = [
  'ratio',
  'image_spec',
  'video_matrix',
  'free',
  'inherit',
] as const

export type ModelModal = (typeof MODEL_MODAL_VALUES)[number]
export type ModelPricingMode = (typeof MODEL_PRICING_MODE_VALUES)[number]

export type ModelSpecResolutionPrice = {
  cny_per_image?: number
  cny_per_second?: number
}

export type AsyncVideoModePrice = {
  cny_per_second?: number
  unsupported?: boolean
}

export type ModelPricingConfig = {
  mode?: ModelPricingMode
  base_ratio?: number
  completion_ratio?: number
  cache_ratio?: number
  create_cache_ratio?: number
  model_price?: number
  use_price?: boolean
  image_ratio?: number
  audio_ratio?: number
  audio_completion_ratio?: number
  unit?: string
  resolutions?: Record<string, ModelSpecResolutionPrice>
  qualities?: Record<string, { cny_per_image?: number }>
  default_cny_per_image?: number
  prices?: Record<string, Record<string, Record<string, AsyncVideoModePrice>>>
  default_cny_per_second?: number
  min_cny?: number
  max_cny?: number
}

export type ImageSpecRow = {
  id: number
  resolution: string
  cnyPerImage: string
}

export type VideoMatrixRow = {
  id: number
  resolution: string
  ratio: string
  mode: string
  supported: boolean
  cnyPerSecond: string
}

export const VIDEO_RESOLUTION_OPTIONS = ['480p', '720p', '1080p', '2k', '4k']
export const VIDEO_RATIO_OPTIONS = ['16:9', '9:16', '4:3', '3:4', '1:1', '21:9']
export const VIDEO_MODE_OPTIONS = [
  'no_video_input',
  'with_video_input',
  'text_audio',
  'text_no_audio',
  'image_audio',
  'image_no_audio',
]
export const IMAGE_RESOLUTION_OPTIONS = ['1k', '2k', '4k']

export function parseModelPricingConfig(raw?: string): ModelPricingConfig {
  if (!raw?.trim()) return { mode: 'inherit' }
  try {
    const parsed = JSON.parse(raw) as ModelPricingConfig
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return { mode: 'inherit' }
    }
    return {
      ...parsed,
      mode: parsed.mode || 'inherit',
    }
  } catch {
    return { mode: 'inherit' }
  }
}

export function stringifyModelPricingConfig(
  config: ModelPricingConfig
): string {
  return JSON.stringify(config, null, 2)
}

export function getModelModalLabel(t: TFunction, modal?: string): string {
  switch (modal) {
    case 'text':
      return t('Text')
    case 'image':
      return t('Image')
    case 'video':
      return t('Video')
    case 'audio':
      return t('Audio')
    default:
      return t('Unspecified')
  }
}

export function getPricingModeLabel(t: TFunction, mode?: string): string {
  switch (mode) {
    case 'ratio':
      return t('Ratio')
    case 'image_spec':
      return t('Image spec')
    case 'video_matrix':
      return t('Video matrix')
    case 'free':
      return t('Free')
    case 'inherit':
      return t('Legacy fallback')
    default:
      return t('Legacy fallback')
  }
}

export function summarizeModelPricing(model: Model, t: TFunction): string {
  const config = parseModelPricingConfig(model.pricing_config)
  const mode = model.pricing_mode || config.mode || 'inherit'
  if (mode === 'ratio') {
    if (config.use_price) {
      return t('Fixed {{price}}', {
        price: formatNumber(config.model_price),
      })
    }
    return t('Ratio {{ratio}}x', {
      ratio: formatNumber(config.base_ratio),
    })
  }
  if (mode === 'image_spec') {
    return t('Spec {{count}} tiers', {
      count: Object.keys(config.resolutions || {}).length,
    })
  }
  if (mode === 'video_matrix') {
    return t('Matrix {{count}} cells', {
      count: countVideoMatrixCells(config),
    })
  }
  return getPricingModeLabel(t, mode)
}

export function countVideoMatrixCells(config: ModelPricingConfig): number {
  let count = 0
  for (const ratioPrices of Object.values(config.prices || {})) {
    for (const modePrices of Object.values(ratioPrices || {})) {
      count += Object.keys(modePrices || {}).length
    }
  }
  return count
}

export function imageRowsFromConfig(
  config: ModelPricingConfig
): ImageSpecRow[] {
  const rows: ImageSpecRow[] = []
  let nextId = 1
  for (const [resolution, price] of Object.entries(config.resolutions || {})) {
    rows.push({
      id: nextId,
      resolution,
      cnyPerImage: toInputNumber(price.cny_per_image),
    })
    nextId += 1
  }
  if (rows.length === 0) {
    rows.push({ id: 1, resolution: '1k', cnyPerImage: '' })
  }
  return rows
}

export function videoRowsFromConfig(
  config: ModelPricingConfig
): VideoMatrixRow[] {
  const rows: VideoMatrixRow[] = []
  let nextId = 1
  for (const [resolution, ratioPrices] of Object.entries(config.prices || {})) {
    for (const [ratio, modePrices] of Object.entries(ratioPrices || {})) {
      for (const [mode, price] of Object.entries(modePrices || {})) {
        rows.push({
          id: nextId,
          resolution,
          ratio,
          mode,
          supported: !price.unsupported,
          cnyPerSecond: toInputNumber(price.cny_per_second),
        })
        nextId += 1
      }
    }
  }
  if (rows.length === 0) {
    rows.push({
      id: 1,
      resolution: '720p',
      ratio: '16:9',
      mode: 'no_video_input',
      supported: true,
      cnyPerSecond: '',
    })
  }
  return rows
}

export function imageRowsToResolutions(
  rows: ImageSpecRow[]
): Record<string, ModelSpecResolutionPrice> {
  const resolutions: Record<string, ModelSpecResolutionPrice> = {}
  for (const row of rows) {
    const resolution = row.resolution.trim()
    if (!resolution) continue
    resolutions[resolution] = {
      cny_per_image: parseNonNegativeNumber(row.cnyPerImage),
    }
  }
  return resolutions
}

export function videoRowsToPrices(
  rows: VideoMatrixRow[]
): NonNullable<ModelPricingConfig['prices']> {
  const prices: NonNullable<ModelPricingConfig['prices']> = {}
  for (const row of rows) {
    const resolution = row.resolution.trim()
    const ratio = row.ratio.trim()
    const mode = row.mode.trim()
    if (!resolution || !ratio || !mode) continue
    prices[resolution] = prices[resolution] || {}
    prices[resolution][ratio] = prices[resolution][ratio] || {}
    prices[resolution][ratio][mode] = row.supported
      ? { cny_per_second: parseNonNegativeNumber(row.cnyPerSecond) }
      : { unsupported: true }
  }
  return prices
}

export function parseNonNegativeNumber(value?: string | number): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed < 0) return 0
  return parsed
}

export function quotaPreview(cny: number, quotaPerCNY: number): number {
  if (!Number.isFinite(cny) || !Number.isFinite(quotaPerCNY)) return 0
  return Math.round(Math.max(0, cny) * Math.max(0, quotaPerCNY))
}

export function formatNumber(value?: number): string {
  if (!Number.isFinite(value)) return '0'
  return Number(value).toLocaleString(undefined, {
    maximumFractionDigits: 6,
  })
}

function toInputNumber(value?: number): string {
  return Number.isFinite(value) ? String(value) : ''
}
