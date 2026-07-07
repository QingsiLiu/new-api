import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { PricingModel } from '../types'
import {
  getImageSpecPriceDisplayItems,
  getDefaultImageSpecPrice,
  getImageSpecPriceRows,
  isImageSpecPricingModel,
} from './spec-pricing'

const imageSpecModel: PricingModel = {
  id: 1,
  model_name: 'gemini-3-pro-image-preview',
  quota_type: 1,
  model_ratio: 0,
  model_price: 0.15,
  completion_ratio: 0,
  enable_groups: ['default'],
  pricing_mode: 'image_spec',
  pricing_config: JSON.stringify({
    mode: 'image_spec',
    unit: 'per_image',
    resolutions: {
      '1k': { cny_per_image: 0.32 },
      '2k': { cny_per_image: 0.32 },
      '4k': { cny_per_image: 0.49 },
    },
    default_cny_per_image: 0.32,
  }),
}

describe('image spec pricing', () => {
  test('detects image spec pricing models from pricing_config', () => {
    assert.equal(isImageSpecPricingModel(imageSpecModel), true)
  })

  test('returns sorted resolution prices instead of legacy model_price', () => {
    assert.deepEqual(getImageSpecPriceRows(imageSpecModel), [
      { resolution: '1K', cnyPerImage: 0.32 },
      { resolution: '2K', cnyPerImage: 0.32 },
      { resolution: '4K', cnyPerImage: 0.49 },
    ])
  })

  test('returns the configured default image price', () => {
    assert.equal(getDefaultImageSpecPrice(imageSpecModel), 0.32)
  })

  test('formats display items for marketplace cards and tables', () => {
    assert.deepEqual(getImageSpecPriceDisplayItems(imageSpecModel), [
      { label: '1K', cnyPerImage: 0.32, formatted: '¥0.32' },
      { label: '2K', cnyPerImage: 0.32, formatted: '¥0.32' },
      { label: '4K', cnyPerImage: 0.49, formatted: '¥0.49' },
    ])
  })
})
