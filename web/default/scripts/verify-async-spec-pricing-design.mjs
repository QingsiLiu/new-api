import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..')

const files = {
  component: path.join(
    root,
    'src/features/system-settings/models/async-spec-pricing-settings.tsx'
  ),
  registry: path.join(
    root,
    'src/features/system-settings/billing/section-registry.tsx'
  ),
  billingIndex: path.join(
    root,
    'src/features/system-settings/billing/index.tsx'
  ),
  types: path.join(root, 'src/features/system-settings/types.ts'),
}

const localeFiles = ['en', 'zh', 'fr', 'ru', 'ja', 'vi'].map((locale) =>
  path.join(root, 'src/i18n/locales', `${locale}.json`)
)

const specPricingI18nKeys = [
  'Spec Pricing',
  'Quota per CNY',
  'Save spec pricing',
  'Video preview',
  'Image preview',
  'Video prices',
  'Add video price',
  'No video prices configured',
  'Resolution',
  'CNY / second',
  'Min / max',
  'Image prices',
  'Add image price',
  'No image prices configured',
  'CNY / image',
  'No rows',
  'Add a row to preview quota conversion.',
  'Specification prices are absolute CNY prices. When a model and specification match, the calculated quota replaces the per-model price; unmatched models keep the existing per-model pricing.',
]

function read(file) {
  if (!fs.existsSync(file)) {
    throw new Error(`Missing required file: ${path.relative(root, file)}`)
  }
  return fs.readFileSync(file, 'utf8')
}

function assertContains(source, needle, label) {
  if (!source.includes(needle)) {
    throw new Error(`Missing ${label}: ${needle}`)
  }
}

function assertNotContains(source, needle, label) {
  if (source.includes(needle)) {
    throw new Error(`Unexpected ${label}: ${needle}`)
  }
}

const component = read(files.component)
const registry = read(files.registry)
const billingIndex = read(files.billingIndex)
const types = read(files.types)

for (const [needle, label] of [
  ['Quota per CNY', 'QuotaPerCNY editor label'],
  ['CNY / second', 'video per-second price column'],
  ['CNY / image', 'image per-image price column'],
  ['IMAGE_RESOLUTION_OPTIONS', 'image resolution option set'],
  ['spec.resolutions[resolution]', 'image resolution JSON writer'],
  ['Video preview', 'video quota preview'],
  ['Image preview', 'image quota preview'],
  ['Switch to JSON', 'JSON editor toggle'],
  ['Switch to Visual', 'visual editor toggle'],
  ['StaticDataTable', 'dense table editor'],
  ['NativeSelect', 'bounded option controls'],
  ['minCNY', 'video minimum price field'],
  ['maxCNY', 'video maximum price field'],
  ['value: String(quotaPerCNY)', 'string option payload for QuotaPerCNY'],
]) {
  assertContains(component, needle, label)
}

for (const [needle, label] of [
  ['useEffect', 'effect-based state initialization'],
  ['rounded-full', 'pill styling in pricing editor'],
  ['shadow-lg', 'large decorative shadow'],
  ['bg-gradient', 'decorative gradient background'],
  ['IMAGE_QUALITY_OPTIONS', 'old image quality option set'],
  ['spec.qualities[quality]', 'old image quality JSON writer'],
]) {
  assertNotContains(component, needle, label)
}

assertContains(registry, "id: 'spec-pricing'", 'billing section registration')
assertContains(registry, 'AsyncSpecPricingSettings', 'settings component import')
assertContains(billingIndex, 'QuotaPerCNY', 'QuotaPerCNY default value')
assertContains(billingIndex, 'AsyncSpecPricing', 'AsyncSpecPricing default value')
assertContains(types, 'QuotaPerCNY: number', 'QuotaPerCNY settings type')
assertContains(types, 'AsyncSpecPricing: string', 'AsyncSpecPricing settings type')

for (const file of localeFiles) {
  const relativePath = path.relative(root, file)
  const data = JSON.parse(read(file))
  if (!data.translation || typeof data.translation !== 'object') {
    throw new Error(`Missing translation object in ${relativePath}`)
  }

  for (const key of specPricingI18nKeys) {
    const value = data.translation[key]
    if (typeof value !== 'string' || value.trim() === '') {
      throw new Error(`Missing Spec Pricing locale key in ${relativePath}: ${key}`)
    }
  }
}

console.log('async spec pricing design verification passed')
