import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  cnyInputToCredits,
  cnyToCredits,
  creditsInputToCNY,
  creditsToCNY,
  buildTextPricingModelQuery,
  formatCNYInput,
  formatCreditsInput,
  getEffectiveTextPricingSummary,
  imageRowsFromConfig,
  imageRowsToResolutions,
  normalizeOfficialPriceProfiles,
  normalizeTextPricingCategories,
  normalizeTextPricingGroups,
  parseTextPricingMultiplierInput,
  videoRowsFromConfig,
  videoRowsToPrices,
} from './model-pricing'

describe('model pricing conversions', () => {
  test('maps CNY and Credits through quota without changing settlement truth', () => {
    assert.equal(cnyToCredits(1), 100_000 / 3_600)
    assert.equal(creditsToCNY(100_000 / 3_600), 1)
    assert.equal(cnyInputToCredits('1'), '27.777778')
    assert.equal(creditsInputToCNY('27.777778'), '1.0000')
  })

  test('keeps empty and invalid price inputs empty', () => {
    assert.equal(cnyInputToCredits(''), '')
    assert.equal(cnyInputToCredits('-1'), '')
    assert.equal(creditsInputToCNY('invalid'), '')
  })

  test('uses fixed display precision for each unit', () => {
    assert.equal(formatCNYInput(1), '1.0000')
    assert.equal(formatCNYInput(0.125), '0.1250')
    assert.equal(formatCreditsInput(1), '1.000000')
    assert.equal(formatCreditsInput(0.125), '0.125000')
  })
})

describe('media pricing configuration', () => {
  test('round-trips image resolution prices through editable rows', () => {
    const rows = imageRowsFromConfig({
      resolutions: {
        '1k': { cny_per_image: 0.125 },
        '4k': { cny_per_image: 1 },
      },
    })

    assert.deepEqual(rows, [
      { id: 1, resolution: '1k', cnyPerImage: '0.1250' },
      { id: 2, resolution: '4k', cnyPerImage: '1.0000' },
    ])
    assert.deepEqual(imageRowsToResolutions(rows), {
      '1k': { cny_per_image: 0.125 },
      '4k': { cny_per_image: 1 },
    })
  })

  test('preserves video matrix cells and unsupported combinations', () => {
    const rows = videoRowsFromConfig({
      prices: {
        '720p': {
          '16:9': {
            no_video_input: { cny_per_second: 0.125 },
            with_video_input: { unsupported: true },
          },
        },
      },
    })

    assert.deepEqual(videoRowsToPrices(rows), {
      '720p': {
        '16:9': {
          no_video_input: { cny_per_second: 0.125 },
          with_video_input: { unsupported: true },
        },
      },
    })
  })
})

describe('text pricing contract normalization', () => {
  test('accepts category maps and profile maps', () => {
    assert.deepEqual(
      normalizeTextPricingCategories({
        gpt: 0.05,
        claude: { category: 'claude', multiplier: 0.22 },
      }),
      [
        { category: 'gpt', multiplier: 0.05 },
        { category: 'claude', multiplier: 0.22 },
      ]
    )

    assert.deepEqual(
      normalizeOfficialPriceProfiles({
        'openai.gpt-5': {
          key: '',
          category: 'gpt',
          display_name: 'GPT-5',
        },
      }),
      [
        {
          key: 'openai.gpt-5',
          category: 'gpt',
          display_name: 'GPT-5',
        },
      ]
    )
  })

  test('always exposes the four supported pricing groups in order', () => {
    assert.deepEqual(
      normalizeTextPricingGroups({
        claude: { category: 'claude', multiplier: 0.22 },
        unclassified: { category: 'unclassified', model_count: 9 },
      }).map((group) => [group.category, group.multiplier]),
      [
        ['gpt', undefined],
        ['claude', 0.22],
        ['gemini', undefined],
        ['grok', undefined],
      ]
    )
  })

  test('validates multiplier bounds and four-decimal precision', () => {
    assert.deepEqual(parseTextPricingMultiplierInput('0.125'), {
      valid: true,
      multiplier: 0.125,
    })
    assert.deepEqual(parseTextPricingMultiplierInput('1'), {
      valid: true,
      multiplier: 1,
    })
    assert.equal(parseTextPricingMultiplierInput('0').valid, false)
    assert.equal(parseTextPricingMultiplierInput('1.0001').valid, false)
    assert.equal(parseTextPricingMultiplierInput('0.12345').valid, false)
  })

  test('does not build a model-list request before a group is expanded', () => {
    assert.equal(buildTextPricingModelQuery(false, 'gpt', '', 1, 10), undefined)
    assert.deepEqual(buildTextPricingModelQuery(true, 'gpt', ' 5.5 ', 2, 10), {
      useSearch: true,
      params: {
        modal: 'text',
        text_category: 'gpt',
        keyword: '5.5',
        p: 2,
        page_size: 10,
      },
    })
  })

  test('summarizes effective quota fields as Credits', () => {
    assert.deepEqual(
      getEffectiveTextPricingSummary({
        input_quota_per_million: 3_600,
        output_quota_per_million: 7_200,
      }),
      [
        { key: 'input', quota: 3_600, credits: 1 },
        { key: 'output', quota: 7_200, credits: 2 },
      ]
    )
  })
})
