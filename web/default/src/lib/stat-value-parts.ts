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

const CURRENCY_SYMBOL_PATTERN = /^([¥$€£])\s*/
const UNIT_PATTERN = /\s*(t\/s|ms|s|%)$/i

export type StatValueParts = {
  currency?: string
  main: string
  unit?: string
}

export function splitStatValueText(value: string): StatValueParts {
  let text = value.trim()
  if (!text) {
    return { main: value }
  }

  const currencyMatch = text.match(CURRENCY_SYMBOL_PATTERN)
  const currency = currencyMatch?.[1]
  if (currencyMatch) {
    text = text.slice(currencyMatch[0].length)
  }

  const unitMatch = text.match(UNIT_PATTERN)
  const unit = unitMatch?.[1]
  if (unitMatch) {
    text = text.slice(0, -unitMatch[0].length)
  }

  return {
    currency,
    main: text || value,
    unit,
  }
}
