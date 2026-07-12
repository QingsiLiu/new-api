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
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const scriptPath = fileURLToPath(new URL('./sync-i18n.mjs', import.meta.url))

test('sync-i18n ignores comments while collecting static i18n keys', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'sync-i18n-'))
  fs.mkdirSync(path.join(tmp, 'src/i18n/locales'), { recursive: true })
  fs.writeFileSync(
    path.join(tmp, 'src/i18n/static-keys.ts'),
    `// Static translation keys that don't get picked up by the t('...') regex.
export const STATIC_I18N_KEYS = [
  'Visible Key',
] as const
`,
    'utf8'
  )
  for (const locale of ['en', 'zh']) {
    fs.writeFileSync(
      path.join(tmp, `src/i18n/locales/${locale}.json`),
      JSON.stringify({ translation: {} }, null, 2),
      'utf8'
    )
  }

  execFileSync(process.execPath, [scriptPath], {
    cwd: tmp,
    stdio: 'pipe',
  })

  const en = JSON.parse(
    fs.readFileSync(path.join(tmp, 'src/i18n/locales/en.json'), 'utf8')
  )
  assert.equal(en.translation['Visible Key'], 'Visible Key')
  assert.equal(en.translation['t get picked up by the t('], undefined)
  assert.equal(en.translation[',\n  '], undefined)
})
