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

const localesDir = path.resolve('src/i18n/locales')
const sourceDir = path.resolve('src')
const localeFiles = ['en.json', 'zh.json', 'fr.json', 'ru.json', 'ja.json', 'vi.json']
const dynamicKeyPrefixes = ['pricingPage.compare.']
const mustTranslateKeys = new Set([
  'Balance Settings',
  'Set balance amount and limits',
  'Unlimited Balance',
  'Enable unlimited balance for this API key',
])

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walk(fullPath, out)
    } else if (/\.(tsx?|jsx?|mjs)$/.test(entry.name)) {
      out.push(fullPath)
    }
  }
  return out
}

function literalI18nKeys() {
  const keys = new Set()
  const callPattern = /\bt\(\s*(['"`])((?:\\.|(?!\1)[\s\S])*?)\1/g

  for (const file of walk(sourceDir)) {
    const text = fs.readFileSync(file, 'utf8')
    let match
    while ((match = callPattern.exec(text))) {
      const rawKey = match[2]
      if (rawKey.includes('${')) continue
      const key = rawKey.replace(/\\'/g, "'").replace(/\\"/g, '"')
      if (dynamicKeyPrefixes.some((prefix) => key.startsWith(prefix))) continue
      keys.add(key)
    }
  }

  return [...keys].sort()
}

const failures = []
const requiredKeys = literalI18nKeys()

for (const file of localeFiles) {
  const locale = file.replace(/\.json$/, '')
  const json = JSON.parse(fs.readFileSync(path.join(localesDir, file), 'utf8'))
  const translations = json.translation ?? {}

  for (const key of requiredKeys) {
    const value = translations[key]
    if (typeof value !== 'string' || value.trim() === '') {
      failures.push(`${file}: missing translation for "${key}"`)
      continue
    }
    if (locale !== 'en' && mustTranslateKeys.has(key) && value === key) {
      failures.push(`${file}: untranslated value for "${key}"`)
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('localization verification passed')
