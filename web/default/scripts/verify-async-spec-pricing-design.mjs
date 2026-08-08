import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '..')

const files = {
  registry: path.join(
    root,
    'src/features/system-settings/billing/section-registry.tsx'
  ),
  billingIndex: path.join(
    root,
    'src/features/system-settings/billing/index.tsx'
  ),
  billingRoute: path.join(
    root,
    'src/routes/_authenticated/system-settings/billing/$section.tsx'
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
  pricingFiltersHook: path.join(
    root,
    'src/features/pricing/hooks/use-filters.ts'
  ),
}

const localeFiles = ['en', 'zh', 'zh-TW', 'fr', 'ru', 'ja', 'vi'].map(
  (locale) => path.join(root, 'src/i18n/locales', `${locale}.json`)
)

const specPricingI18nKeys = [
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

const registry = read(files.registry)
const billingIndex = read(files.billingIndex)
const billingRoute = read(files.billingRoute)
const types = read(files.types)
const pricingTypes = read(files.pricingTypes)
const pricingSpecHelper = read(files.pricingSpecHelper)
const pricingModelCard = read(files.pricingModelCard)
const pricingModelDetails = read(files.pricingModelDetails)
const pricingSidebar = read(files.pricingSidebar)
const pricingToolbar = read(files.pricingToolbar)
const pricingFiltersHook = read(files.pricingFiltersHook)
const billingTypes = types.slice(
  types.indexOf('export type BillingSettings = {'),
  types.indexOf('export type OperationsSettings = {')
)

for (const section of [
  'currency',
  'model-pricing',
  'spec-pricing',
  'group-pricing',
  'checkin',
]) {
  assertNotContains(
    registry,
    `id: '${section}'`,
    `retired billing section ${section}`
  )
}
assertContains(registry, "id: 'quota'", 'quota billing section')
assertContains(registry, "id: 'payment'", 'payment gateway section')
assertContains(registry, "id: 'advanced'", 'advanced billing section')
assertContains(registry, 'ToolPriceSettings', 'advanced tool pricing editor')
assertContains(
  registry,
  "settings['tool_price_setting.prices']",
  'tool pricing option wiring'
)
assertNotContains(
  billingIndex,
  'AsyncSpecPricing',
  'legacy spec pricing default'
)
assertNotContains(billingIndex, 'ModelRatio', 'legacy model pricing default')
assertNotContains(billingIndex, 'GroupRatio', 'legacy group pricing default')
assertNotContains(billingIndex, 'checkin_setting', 'legacy check-in defaults')
assertNotContains(
  billingTypes,
  'AsyncSpecPricing: string',
  'legacy spec pricing type'
)
assertNotContains(
  billingTypes,
  'ModelRatio: string',
  'legacy model pricing type'
)
assertNotContains(
  billingTypes,
  'GroupRatio: string',
  'legacy group pricing type'
)
assertNotContains(
  billingTypes,
  'checkin_setting',
  'legacy check-in settings type'
)
assertContains(
  billingRoute,
  "'model-pricing': { to: '/models/$section', section: 'metadata' }",
  'legacy model pricing redirect'
)
assertContains(
  billingRoute,
  "'spec-pricing': { to: '/models/$section', section: 'metadata' }",
  'legacy spec pricing redirect'
)
assertContains(
  billingRoute,
  "'group-pricing': { to: '/users' }",
  'legacy group pricing redirect'
)
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
      throw new Error(
        `Missing media pricing locale key in ${relativePath}: ${key}`
      )
    }
  }
}

console.log('model pricing center cleanup verification passed')
