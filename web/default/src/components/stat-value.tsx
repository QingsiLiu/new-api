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
import { splitStatValueText } from '@/lib/stat-value-parts'
import { cn } from '@/lib/utils'

type StatValueProps = React.HTMLAttributes<HTMLSpanElement> & {
  value: React.ReactNode
}

export function StatValue({ value, className, ...props }: StatValueProps) {
  if (typeof value !== 'string' && typeof value !== 'number') {
    return (
      <span data-slot='stat-value' className={className} {...props}>
        {value}
      </span>
    )
  }

  const parts = splitStatValueText(String(value))

  return (
    <span
      data-slot='stat-value'
      className={cn('inline', className)}
      {...props}
    >
      {parts.currency && (
        <span className='stat-value-affix' data-stat-value-part='currency'>
          {parts.currency}
        </span>
      )}
      <span data-stat-value-part='main'>{parts.main}</span>
      {parts.unit && (
        <span className='stat-value-unit' data-stat-value-part='unit'>
          {parts.unit}
        </span>
      )}
    </span>
  )
}
