import {
  getImageSpecPriceRows,
  getModelSpecPricingSummary,
  getVideoMatrixPriceRows,
} from './spec-pricing'
import type { PricingModel } from '../types'

function assertEqual<T>(actual: T, expected: T, label: string) {
  const actualJSON = JSON.stringify(actual)
  const expectedJSON = JSON.stringify(expected)
  if (actualJSON !== expectedJSON) {
    throw new Error(`${label}: expected ${expectedJSON}, got ${actualJSON}`)
  }
}

const imageModel: PricingModel = {
  id: 1,
  model_name: 'nano-banana-2',
  quota_type: 1,
  model_ratio: 0,
  completion_ratio: 0,
  model_price: 0.18,
  amount_cny: 0.18,
  pricing_mode: 'image_spec',
  spec_pricing: {
    '2k': { cny_per_image: 0.28 },
    '4k': { cny_per_image: 0.42 },
    '1k': { cny_per_image: 0.18 },
  },
  enable_groups: ['default'],
}

assertEqual(
  getImageSpecPriceRows(imageModel).map((row) => [
    row.resolution,
    row.label,
    row.formattedPrice,
  ]),
  [
    ['1k', '1K', '¥0.18'],
    ['2k', '2K', '¥0.28'],
    ['4k', '4K', '¥0.42'],
  ],
  'image rows are ordered and formatted'
)

assertEqual(
  getModelSpecPricingSummary(imageModel),
  {
    mode: 'image_spec',
    labelKey: 'Image generation',
    entries: [
      { label: '1K', formattedPrice: '¥0.18' },
      { label: '2K', formattedPrice: '¥0.28' },
      { label: '4K', formattedPrice: '¥0.42' },
    ],
  },
  'image card summary'
)

const videoModel: PricingModel = {
  id: 2,
  model_name: 'seedance-pro',
  quota_type: 1,
  model_ratio: 0,
  completion_ratio: 0,
  model_price: 0.6,
  amount_cny: 0.6,
  pricing_mode: 'video_matrix',
  spec_pricing: {
    '1080p': {
      '16:9': {
        no_video_input: { cny_per_second: 0.9 },
      },
    },
    '720p': {
      '16:9': {
        no_video_input: { cny_per_second: 0.6 },
        with_video_input: { unsupported: true },
      },
    },
  },
  enable_groups: ['default'],
}

assertEqual(
  getVideoMatrixPriceRows(videoModel).map((row) => [
    row.resolution,
    row.ratio,
    row.mode,
    row.supported,
    row.supported ? row.formattedPrice : row.labelKey,
  ]),
  [
    ['720p', '16:9', 'no_video_input', true, '¥0.6'],
    ['720p', '16:9', 'with_video_input', false, 'Unsupported'],
    ['1080p', '16:9', 'no_video_input', true, '¥0.9'],
  ],
  'video matrix rows include unsupported cells'
)

assertEqual(
  getModelSpecPricingSummary(videoModel),
  {
    mode: 'video_matrix',
    labelKey: 'Video generation',
    startPrice: '¥0.6',
    unitKey: 'second',
  },
  'video card summary'
)
