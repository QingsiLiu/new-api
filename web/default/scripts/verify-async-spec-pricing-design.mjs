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

console.log('async spec pricing design verification passed')
