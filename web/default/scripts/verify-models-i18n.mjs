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
import fs from 'node:fs'
import path from 'node:path'

const ROOT = process.cwd()
const MODELS_DIR = path.join(ROOT, 'src/features/models')
const LOCALES_DIR = path.join(ROOT, 'src/i18n/locales')
const LOCALES = ['en', 'zh', 'fr', 'ja', 'ru', 'vi']

const ALLOWED_LITERAL_TEXT = new Set(['USDC', 'IOCOIN'])
const ALLOWED_LITERAL_PROPS = new Set(['value', 'type', 'variant', 'size', 'side', 'align'])

function walk(dir) {
  const out = []
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      out.push(...walk(full))
    } else if (/\.(ts|tsx)$/.test(entry.name)) {
      out.push(full)
    }
  }
  return out
}

function lineNumber(source, index) {
  return source.slice(0, index).split('\n').length
}

function isTranslatableKey(key) {
  const trimmed = key.trim()
  if (!trimmed) return false
  if (/^[,;.:/\s]+$/.test(trimmed)) return false
  return /[A-Za-z]/.test(trimmed)
}

function collectTranslationKeys(files) {
  const keys = new Map()
  for (const file of files) {
    const source = fs.readFileSync(file, 'utf8')
    for (const match of source.matchAll(/\bt\(\s*(['"`])([\s\S]*?)\1/g)) {
      const key = match[2]
      if (!isTranslatableKey(key)) continue
      const rel = path.relative(ROOT, file)
      if (!keys.has(key)) keys.set(key, [])
      keys.get(key).push(`${rel}:${lineNumber(source, match.index)}`)
    }
  }
  return keys
}

function collectHardcodedText(files) {
  const findings = []
  for (const file of files.filter((f) => f.endsWith('.tsx'))) {
    const source = fs.readFileSync(file, 'utf8')
    const rel = path.relative(ROOT, file)

    for (const match of source.matchAll(/>([^<>{}\n]*[A-Za-z][^<>{}\n]*)</g)) {
      const text = match[1].trim()
      if (!text || ALLOWED_LITERAL_TEXT.has(text)) continue
      findings.push(`${rel}:${lineNumber(source, match.index)} JSX text "${text}"`)
    }

    for (const match of source.matchAll(/\b([a-zA-Z]+(?:Label|Text|Message|Title|Placeholder))=(['"])([^'"]*[A-Za-z][^'"]*)\2/g)) {
      const prop = match[1]
      const text = match[3].trim()
      if (!text || ALLOWED_LITERAL_PROPS.has(prop)) continue
      if (text.startsWith('{') || text.startsWith('[')) continue
      findings.push(`${rel}:${lineNumber(source, match.index)} prop ${prop}="${text}"`)
    }
  }
  return findings
}

function readLocale(locale) {
  const file = path.join(LOCALES_DIR, `${locale}.json`)
  return JSON.parse(fs.readFileSync(file, 'utf8')).translation ?? {}
}

const files = walk(MODELS_DIR)
const keys = collectTranslationKeys(files)
const locales = Object.fromEntries(LOCALES.map((locale) => [locale, readLocale(locale)]))
const missing = []

for (const [key, refs] of [...keys.entries()].sort(([a], [b]) => a.localeCompare(b))) {
  for (const locale of LOCALES) {
    if (!(key in locales[locale])) {
      missing.push(`${locale}: "${key}"\n  ${refs.slice(0, 3).join('\n  ')}`)
    }
  }
}

const hardcoded = collectHardcodedText(files)

if (missing.length > 0 || hardcoded.length > 0) {
  if (missing.length > 0) {
    console.error(`Missing models i18n keys: ${missing.length}`)
    console.error(missing.join('\n'))
  }
  if (hardcoded.length > 0) {
    console.error(`Hardcoded models UI text: ${hardcoded.length}`)
    console.error(hardcoded.join('\n'))
  }
  process.exit(1)
}

console.log(`models i18n verified: ${keys.size} keys across ${LOCALES.length} locales`)
