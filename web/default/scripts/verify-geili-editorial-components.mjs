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
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const editorialDir = resolve(root, 'src/components/editorial')
const indexPath = resolve(editorialDir, 'index.ts')
const statPath = resolve(editorialDir, 'editorial-stat.tsx')
const labelPath = resolve(editorialDir, 'editorial-label.tsx')
const statusPath = resolve(editorialDir, 'editorial-status.tsx')

const requiredFiles = [indexPath, statPath, labelPath, statusPath]
const missingFiles = requiredFiles.filter((file) => !existsSync(file))

if (missingFiles.length > 0) {
  throw new Error(
    `Missing editorial component files: ${missingFiles
      .map((file) => file.replace(`${root}/`, ''))
      .join(', ')}`
  )
}

const index = readFileSync(indexPath, 'utf8')
const stat = readFileSync(statPath, 'utf8')
const label = readFileSync(labelPath, 'utf8')
const status = readFileSync(statusPath, 'utf8')

for (const exported of [
  'EditorialStat',
  'EditorialStatGroup',
  'EditorialLabel',
  'EditorialStatus',
]) {
  if (!index.includes(exported)) {
    throw new Error(`src/components/editorial/index.ts must export ${exported}`)
  }
}

for (const tokenClass of [
  'editorial-label',
  'editorial-stat-value',
  'border-border',
  'text-primary',
]) {
  if (!stat.includes(tokenClass)) {
    throw new Error(`EditorialStat must use tokenized class: ${tokenClass}`)
  }
}

if (!label.includes('editorial-label')) {
  throw new Error('EditorialLabel must use the shared editorial-label utility')
}

for (const tokenClass of [
  'font-mono',
  'bg-success',
  'bg-primary',
  'bg-destructive',
  'bg-neutral',
]) {
  if (!status.includes(tokenClass)) {
    throw new Error(`EditorialStatus must use tokenized class: ${tokenClass}`)
  }
}

console.log('geili-editorial shared components verified')
