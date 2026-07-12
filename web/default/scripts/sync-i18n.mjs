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
import fs from 'node:fs/promises'
import path from 'node:path'

// This script is executed from the web/ package root (see package.json script).
const LOCALES_DIR = path.resolve('src/i18n/locales')
const SRC_DIR = path.resolve('src')
const FALLBACK_COMPARE_LOCALE = 'en' // used for "still English" detection only
const CHECK_MODE = process.argv.includes('--check')
const OBFUSCATED_KEYS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

const BRAND_AND_LITERAL_KEYS = new Set([
  'AI Proxy',
  'AIGC2D',
  'Alipay',
  'Anthropic',
  'Anthropic (/v1/messages)',
  'API URL',
  'API2GPT',
  'AccessKey / SecretAccessKey',
  'AZURE_OPENAI_ENDPOINT *',
  'Baidu V2',
  'BillingMode',
  'ChatGPT',
  'ChatGPT Subscription (Codex)',
  'Claude',
  'Claude Opus 4.6',
  'Claude Opus 4.6 Fast',
  'Claude Sonnet 4.5',
  'Client ID',
  'Client Secret',
  'Cloudflare',
  'Cohere',
  'CNY (¥)',
  'DeepSeek',
  'Discord',
  'Discord ID',
  'DoubaoVideo',
  'Doubao',
  'Doubao Seed 1.8',
  'FastGPT',
  'Gemini',
  'Gemini Image 4K',
  'Gemini 2.5 Flash',
  'Gemini 3 Pro Image',
  'Gemini (/v1beta/models/{model}:generateContent)',
  'GitHub',
  'GitHub ID',
  'Gotify',
  'GLM-4.5 Air',
  'GPT Image 1 Mini',
  'Jimeng',
  'Jimeng Video (OpenAI Video)',
  'JustSong',
  'LingYiWanWu',
  'LinuxDO',
  'Midjourney',
  'MidjourneyPlus',
  'Midjourney-Proxy',
  'MiniMax',
  'Mistral',
  'MokaAI',
  'Moonshot',
  'Nano Banana',
  'New API',
  'New API &lt;noreply@example.com&gt;',
  'NewAPI',
  'OAuth Client Secret',
  'OhMyGPT',
  'Ollama',
  'One API',
  'OpenAI',
  'OpenAI (/v1/chat/completions)',
  'OpenAI Response Compaction (/v1/responses/compact)',
  'OpenAI Responses (/v1/responses)',
  'OpenAIMax',
  'OpenRouter',
  'Pancake',
  'Passkey',
  'Perplexity',
  'PostgreSQL',
  'QuantumNous',
  'Qwen3 Max',
  'Qwen3 Omni Flash',
  'Quota:',
  'Replicate',
  'Responses',
  'SiliconFlow',
  'SQLite',
  'Stripe',
  'Submodel',
  'SunoAPI',
  'Telegram',
  'Telegram ID',
  'Tencent',
  'TTFT P50',
  'TTFT P95',
  'TTFT P99',
  'Uptime Kuma',
  'Uptime Kuma URL',
  'Vertex AI',
  'VolcEngine',
  'Waffo Pancake Dashboard',
  'Waffo Pancake MoR',
  'Waffo Pancake',
  'WeChat',
  'WeChat ID',
  'WeChat Pay',
  'Webhook URL',
  'Webhook URL:',
  'Webhook',
  'Well-Known URL',
  'Worker URL',
  'Xinference',
  'Xunfei',
  'Zhipu V4',
  'CC Switch',
  'Creem Product ID',
  'CREDITCARD,DEBITCARD',
  'MER_xxx',
  'ModelPrice',
  'endpoint_type',
  'bash -lc',
  'bg-destructive',
  'forced_bad_request',
  'header:session_id -> json:prompt_cache_key',
  'invalid_request_error',
  'json:prompt_cache_key -> header:session_id',
  'ollama/ollama:latest',
  'prefer-by-conversation-id',
  'price_...',
  'prod_...',
  'prompt_cache_key',
  'ratio_config',
  'redacted_thinking',
  'runway',
  'seedance-2.0',
  'session_id',
  'tier("base", p * 3 + c * 15)',
  'viggle',
  'web_search_preview:gpt-4o*',
  '--foo bar',
  'preset.geili-editorial',
  'preset.geili-minimal',
  'preset.geili-modern',
  'resolution,ratio,mode,cny_per_second',
  'stderr',
  'stdout',
  '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"',
  'edit_this',
  'footer.columns.related.links.midjourney',
  'footer.columns.related.links.newApiKeyTool',
  'my-status',
  'new-api-key-tool',
  'price_xxx',
  'whsec_xxx',
])

function isPlainObject(v) {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function stableStringify(obj) {
  let text = JSON.stringify(obj, null, 2)
  for (const key of OBFUSCATED_KEYS) {
    text = text.replaceAll(`"${key.runtime}":`, `"${key.serialized}":`)
  }
  return text + '\n'
}

function countLeafKeys(obj) {
  if (Array.isArray(obj)) return obj.length
  if (!isPlainObject(obj)) return 0
  let count = 0
  for (const k of Object.keys(obj)) {
    const v = obj[k]
    if (isPlainObject(v) || Array.isArray(v)) count += countLeafKeys(v)
    else count += 1
  }
  return count
}

async function collectSourceFiles(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (
        ['assets', 'i18n', 'styles'].includes(entry.name) ||
        entry.name.startsWith('.')
      ) {
        continue
      }
      files.push(...(await collectSourceFiles(full)))
      continue
    }
    if (!/\.(tsx?|jsx?)$/i.test(entry.name)) continue
    if (/\.(test|spec)\.(tsx?|jsx?)$/i.test(entry.name)) continue
    files.push(full)
  }
  return files
}

function decodeSourceString(raw) {
  return raw
    .replaceAll("\\'", "'")
    .replaceAll('\\"', '"')
    .replaceAll('\\`', '`')
    .replaceAll('\\n', '\n')
    .replaceAll('\\t', '\t')
}

function stripSourceComments(source) {
  let out = ''
  let quote = null
  let escaped = false

  for (let i = 0; i < source.length; i += 1) {
    const char = source[i]
    const next = source[i + 1]

    if (quote) {
      out += char
      if (escaped) {
        escaped = false
        continue
      }
      if (char === '\\') {
        escaped = true
        continue
      }
      if (char === quote) quote = null
      continue
    }

    if (char === "'" || char === '"' || char === '`') {
      quote = char
      out += char
      continue
    }

    if (char === '/' && next === '/') {
      while (i < source.length && source[i] !== '\n') i += 1
      out += '\n'
      continue
    }

    if (char === '/' && next === '*') {
      i += 2
      while (i < source.length && !(source[i] === '*' && source[i + 1] === '/')) {
        out += source[i] === '\n' ? '\n' : ' '
        i += 1
      }
      i += 1
      continue
    }

    out += char
  }

  return out
}

function shouldCollectI18nKey(key) {
  const value = key.trim()
  if (!value) return false
  if (value.includes('${')) return false
  if (value.includes('\n')) return false
  if (/^https?:\/\//i.test(value)) return false
  if (/^\/[\w./:-]+$/.test(value)) return false
  if (/^[.#]?[a-z0-9_-]+$/i.test(value) && value.length < 3) return false
  return true
}

function collectStringMatches(source, regex, keys) {
  let match
  while ((match = regex.exec(source)) !== null) {
    const value = decodeSourceString(match[2] ?? '')
    if (shouldCollectI18nKey(value)) keys.add(value)
  }
}

async function collectSourceI18nKeys() {
  const keys = new Set()
  const files = await collectSourceFiles(SRC_DIR)
  const callRegex = /\b(?:t|i18next\.t)\(\s*(['"`])((?:\\.|(?!\1)[\s\S])*?)\1/g
  const dynamicPropRegex =
    /\b(?:titleKey|descriptionKey|saveLabel|savingLabel|resetLabel|submitLabel|entityName|label|description|placeholder|error|message|emptyTitle|emptyDescription)\s*[:=]\s*(['"`])((?:\\.|(?!\1)[\s\S])*?)\1/g

  for (const file of files) {
    const source = stripSourceComments(await fs.readFile(file, 'utf8'))
    collectStringMatches(source, callRegex, keys)
    collectStringMatches(source, dynamicPropRegex, keys)
  }

  const staticKeysPath = path.resolve('src/i18n/static-keys.ts')
  const staticKeysSource = stripSourceComments(
    await fs.readFile(staticKeysPath, 'utf8').catch(() => '')
  )
  collectStringMatches(
    staticKeysSource,
    /(['"`])((?:\\.|(?!\1)[\s\S])*?)\1/g,
    keys
  )

  return [...keys].sort((a, b) => a.localeCompare(b))
}

function reorderLikeBase(base, target, fill, extras, missing, currentPath = []) {
  // If base is an object, we keep base's key order and recurse.
  if (isPlainObject(base)) {
    const out = {}
    const t = isPlainObject(target) ? target : {}
    const f = isPlainObject(fill) ? fill : {}

    for (const key of Object.keys(base)) {
      const nextPath = [...currentPath, key]
      if (Object.prototype.hasOwnProperty.call(t, key)) {
        out[key] = reorderLikeBase(base[key], t[key], f[key], extras, missing, nextPath)
      } else {
        missing.push(nextPath.join('.'))
        out[key] = reorderLikeBase(base[key], undefined, f[key], extras, missing, nextPath)
      }
    }

    for (const key of Object.keys(t)) {
      if (!Object.prototype.hasOwnProperty.call(base, key)) {
        const nextPath = [...currentPath, key].join('.')
        extras[nextPath] = t[key]
      }
    }

    return out
  }

  // For arrays: prefer target if it's also an array; otherwise use base.
  if (Array.isArray(base)) {
    if (Array.isArray(target)) return target
    if (Array.isArray(fill)) return fill
    return base
  }

  // For primitives: prefer target if defined, else base.
  return target === undefined ? (fill ?? base) : target
}

function isLikelyUntranslated({ locale, baseValue, value }) {
  if (typeof value !== 'string' || typeof baseValue !== 'string') return false
  if (value !== baseValue) return false

  // Skip short tokens / acronyms / ids
  const s = baseValue.trim()
  if (BRAND_AND_LITERAL_KEYS.has(s)) return false
  if (
    /^https?:\/\//.test(s) ||
    /^\/[\w/-]+/.test(s) ||
    /^[\w.-]+@[\w.-]+$/.test(s) ||
    /^smtp\./i.test(s) ||
    /^socks5:/i.test(s) ||
    /^org-/.test(s) ||
    /^gpt-/i.test(s) ||
    /^checkout\./.test(s) ||
    /^footer\./.test(s) ||
    /^[A-Z0-9_ *./:-]+$/.test(s) ||
    s.startsWith('{') ||
    s.startsWith('[') ||
    s.includes('&#10;')
  ) {
    return false
  }
  if (s.length < 6) return false
  if (!/[A-Za-z]{3,}/.test(s)) return false

  // For locales with non-latin scripts, equality with EN is a strong signal.
  if (locale === 'ja' || locale === 'zh') return true
  if (locale === 'ru') return true

  // For fr/vi: still useful but noisier; keep it conservative.
  if (locale === 'fr' || locale === 'vi') return /\b(the|and|or|to|with|please)\b/i.test(s)

  return false
}

async function main() {
  const entries = await fs.readdir(LOCALES_DIR, { withFileTypes: true })
  const localeFiles = entries
    .filter((e) => e.isFile() && e.name.endsWith('.json'))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b))

  // Auto-pick base locale as the one with the most leaf keys under translation (most "rich").
  const parsedByLocale = {}
  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const raw = await fs.readFile(path.join(LOCALES_DIR, filename), 'utf8')
    parsedByLocale[locale] = JSON.parse(raw)
  }

  const baseLocale = Object.keys(parsedByLocale)
    .map((locale) => {
      const json = parsedByLocale[locale]
      const trans = json?.translation ?? {}
      return { locale, score: countLeafKeys(trans) }
    })
    .sort((a, b) => b.score - a.score || a.locale.localeCompare(b.locale))[0]?.locale

  if (!baseLocale) throw new Error('No locale files found.')

  const baseFile = `${baseLocale}.json`
  const baseJson = parsedByLocale[baseLocale]
  baseJson.translation ??= {}

  const sourceKeys = await collectSourceI18nKeys()
  const addedSourceKeys = []
  for (const key of sourceKeys) {
    if (!Object.prototype.hasOwnProperty.call(baseJson.translation, key)) {
      baseJson.translation[key] = key
      addedSourceKeys.push(key)
    }
  }

  const compareJson = parsedByLocale[FALLBACK_COMPARE_LOCALE] ?? baseJson

  const report = {
    base: baseFile,
    sourceKeys: sourceKeys.length,
    addedSourceKeys,
    locales: {},
  }

  const extrasDir = path.join(LOCALES_DIR, '_extras')
  const reportsDir = path.join(LOCALES_DIR, '_reports')
  await fs.mkdir(extrasDir, { recursive: true })
  await fs.mkdir(reportsDir, { recursive: true })

  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const full = path.join(LOCALES_DIR, filename)
    const json = parsedByLocale[locale]

    const extras = {}
    const missing = []
    const fixed = reorderLikeBase(baseJson, json, compareJson, extras, missing)

    // Untranslated scan (translation namespace only)
    const untranslated = {}
    const compareTrans = compareJson?.translation ?? {}
    const trans = fixed?.translation ?? {}
    if (
      isPlainObject(compareTrans) &&
      isPlainObject(trans) &&
      locale !== FALLBACK_COMPARE_LOCALE &&
      locale !== baseLocale
    ) {
      for (const k of Object.keys(compareTrans)) {
        const baseValue = compareTrans[k]
        const value = trans[k]
        if (isLikelyUntranslated({ locale, baseValue, value })) {
          untranslated[k] = value
        }
      }
    }

    report.locales[locale] = {
      file: filename,
      missingCount: missing.length,
      extrasCount: Object.keys(extras).length,
      untranslatedCount: Object.keys(untranslated).length,
    }

    if (Object.keys(extras).length > 0) {
      await writeLocaleFile(path.join(extrasDir, `${locale}.extras.json`), stableStringify(extras))
    } else {
      await removeLocaleFile(path.join(extrasDir, `${locale}.extras.json`))
    }
    if (Object.keys(untranslated).length > 0) {
      await writeLocaleFile(
        path.join(reportsDir, `${locale}.untranslated.json`),
        stableStringify(untranslated)
      )
    } else {
      await removeLocaleFile(path.join(reportsDir, `${locale}.untranslated.json`))
    }

    // Rewrite locale file in base order (even for en to normalize formatting)
    await writeLocaleFile(full, stableStringify(fixed))
  }

  await writeLocaleFile(path.join(reportsDir, '_sync-report.json'), stableStringify(report))

  if (CHECK_MODE && writeLocaleFile.changed.length > 0) {
    console.error('i18n check failed. Files need sync:')
    for (const file of writeLocaleFile.changed) console.error(`- ${file}`)
    process.exitCode = 1
    return
  }

  console.log(
    `${CHECK_MODE ? 'i18n check done' : 'i18n sync done'}. Report: ${path.join(reportsDir, '_sync-report.json')}`
  )
}

async function writeLocaleFile(file, content) {
  const previous = await fs.readFile(file, 'utf8').catch(() => undefined)
  if (previous === content) return
  writeLocaleFile.changed.push(file)
  if (!CHECK_MODE) {
    await fs.mkdir(path.dirname(file), { recursive: true })
    await fs.writeFile(file, content, 'utf8')
  }
}
writeLocaleFile.changed = []

async function removeLocaleFile(file) {
  const exists = await fs.stat(file).then(() => true).catch(() => false)
  if (!exists) return
  writeLocaleFile.changed.push(file)
  if (!CHECK_MODE) await fs.rm(file, { force: true })
}

main().catch((err) => {
   
  console.error(err)
  process.exitCode = 1
})
