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
  pricingTypes: path.join(root, 'src/features/pricing/types.ts'),
  pricingSpecHelper: path.join(
    root,
    'src/features/pricing/lib/spec-pricing.ts'
  ),
  pricingModelCard: path.join(
    root,
    'src/features/pricing/components/model-card.tsx'
  ),
  pricingModelDetails: path.join(
    root,
    'src/features/pricing/components/model-details.tsx'
  ),
  pricingSidebar: path.join(
    root,
    'src/features/pricing/components/pricing-sidebar.tsx'
  ),
  pricingToolbar: path.join(
    root,
    'src/features/pricing/components/pricing-toolbar.tsx'
  ),
  pricingFiltersHook: path.join(root, 'src/features/pricing/hooks/use-filters.ts'),
}

const localeFiles = ['en', 'zh', 'fr', 'ru', 'ja', 'vi'].map((locale) =>
  path.join(root, 'src/i18n/locales', `${locale}.json`)
)

const specPricingI18nKeys = [
  'Spec Pricing',
  'Save spec pricing',
  'Video prices',
  'Video matrix prices',
  'Add video price',
  'No video prices configured',
  'Resolution',
  'Ratio',
  'Mode',
  'Status',
  'Supported',
  'Unsupported',
  'No video input',
  'With video input',
  'Text with audio',
  'Text without audio',
  'Image with audio',
  'Image without audio',
  'CNY / second',
  'Min / max',
  'Image prices',
  'Add image price',
  'No image prices configured',
  'CNY / image',
  'Image generation',
  'Video generation',
  'Starting at',
  'second',
  'Default price',
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
const pricingTypes = read(files.pricingTypes)
const pricingSpecHelper = read(files.pricingSpecHelper)
const pricingModelCard = read(files.pricingModelCard)
const pricingModelDetails = read(files.pricingModelDetails)
const pricingSidebar = read(files.pricingSidebar)
const pricingToolbar = read(files.pricingToolbar)
const pricingFiltersHook = read(files.pricingFiltersHook)

for (const [needle, label] of [
  ['Video matrix prices', 'video matrix section label'],
  ['CNY / second', 'video per-second price column'],
  ['VIDEO_RATIO_OPTIONS', 'video ratio option set'],
  ['VIDEO_MODE_OPTIONS', 'video mode option set'],
  ['VIDEO_STATUS_OPTIONS', 'video support status option set'],
  ['prices?: Record', 'video matrix JSON type'],
  ['spec.prices[resolution]', 'video matrix JSON writer'],
  ['unsupported: !row.supported', 'unsupported video matrix writer'],
  ['row.ratio', 'video ratio row field'],
  ['row.mode', 'video mode row field'],
  ['row.supported', 'video status row field'],
  ['CNY / image', 'image per-image price column'],
  ['IMAGE_RESOLUTION_OPTIONS', 'image resolution option set'],
  ['spec.resolutions[resolution]', 'image resolution JSON writer'],
  ['Switch to JSON', 'JSON editor toggle'],
  ['Switch to Visual', 'visual editor toggle'],
  ['StaticDataTable', 'dense table editor'],
  ['NativeSelect', 'bounded option controls'],
  ['minCNY', 'video minimum price field'],
  ['maxCNY', 'video maximum price field'],
]) {
  assertContains(component, needle, label)
}

for (const [needle, label] of [
  ['quotaPerCNY', 'QuotaPerCNY prop/state in spec pricing editor'],
  ['QuotaPerCNY', 'QuotaPerCNY option payload in spec pricing editor'],
  ['useEffect', 'effect-based state initialization'],
  ['rounded-full', 'pill styling in pricing editor'],
  ['shadow-lg', 'large decorative shadow'],
  ['bg-gradient', 'decorative gradient background'],
  ['IMAGE_QUALITY_OPTIONS', 'old image quality option set'],
  ['spec.qualities[quality]', 'old image quality JSON writer'],
  ['AlertDescription', 'developer-facing pricing explanation banner'],
  ['PreviewLine', 'quota preview cards'],
  ['Quota per CNY', 'raw quota-per-CNY label'],
  ['Video preview', 'video quota preview'],
  ['Image preview', 'image quota preview'],
  [
    'Specification prices are absolute CNY prices',
    'developer-facing pricing explanation text',
  ],
]) {
  assertNotContains(component, needle, label)
}

assertContains(registry, "id: 'spec-pricing'", 'billing section registration')
assertContains(registry, 'AsyncSpecPricingSettings', 'settings component import')
assertNotContains(
  registry,
  'quotaPerCNYDefault',
  'QuotaPerCNY prop wiring in billing registry'
)
assertContains(billingIndex, 'QuotaPerCNY', 'QuotaPerCNY default value')
assertContains(billingIndex, 'AsyncSpecPricing', 'AsyncSpecPricing default value')
assertContains(types, 'QuotaPerCNY: number', 'QuotaPerCNY settings type')
assertContains(types, 'AsyncSpecPricing: string', 'AsyncSpecPricing settings type')
assertContains(pricingTypes, 'pricing_mode?', 'pricing mode API field')
assertContains(pricingTypes, 'spec_pricing?', 'spec pricing API field')
assertContains(pricingTypes, 'amount_cny?', 'CNY amount API field')
assertContains(
  pricingSpecHelper,
  'getImageSpecPriceRows',
  'image spec pricing helper'
)
assertContains(
  pricingSpecHelper,
  'getVideoMatrixPriceRows',
  'video matrix pricing helper'
)
assertContains(
  pricingSpecHelper,
  'formatCNYAmount',
  'CNY-native spec pricing formatter'
)
assertContains(
  pricingModelCard,
  'SpecPricingInlineSummary',
  'pricing card spec summary'
)
assertContains(
  pricingModelCard,
  'getModelSpecPricingSummary',
  'pricing card spec pricing branch'
)
assertContains(
  pricingModelDetails,
  'ImageSpecPricingSection',
  'image spec detail table'
)
assertContains(
  pricingModelDetails,
  'VideoMatrixPricingSection',
  'video matrix detail table'
)
assertContains(
  pricingModelDetails,
  'CNY / image',
  'image spec detail CNY column'
)
assertContains(
  pricingModelDetails,
  'CNY / second',
  'video matrix detail CNY column'
)
assertNotContains(
  pricingSpecHelper,
  'formatCurrencyFromUSD',
  'legacy USD formatter in spec pricing helper'
)
assertNotContains(
  pricingSidebar,
  "title={t('Pricing Type')}",
  'low-value pricing type sidebar filter'
)
assertNotContains(
  pricingSidebar,
  "title={t('Endpoint Type')}",
  'low-value endpoint type sidebar filter'
)
assertNotContains(
  pricingSidebar,
  'type, and tags',
  'sidebar copy that references hidden type filters'
)
assertContains(
  pricingSidebar,
  'groupDisplay',
  'pricing sidebar public group display mapping'
)
assertNotContains(
  pricingSidebar,
  'useGroupRegistry',
  'authenticated group registry lookup in public pricing sidebar'
)
assertNotContains(
  pricingToolbar,
  'type, endpoint, and tags',
  'mobile filter copy that references hidden type filters'
)
assertNotContains(
  pricingFiltersHook,
  'quotaType: search.quotaType',
  'hidden pricing type URL filter initialization'
)
assertNotContains(
  pricingFiltersHook,
  'endpointType: search.endpointType',
  'hidden endpoint type URL filter initialization'
)

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
