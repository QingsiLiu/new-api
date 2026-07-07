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
import { cn } from '@/lib/utils'
import type { DataTableColumnClassName, DataTablePinnedColumn } from './types'

export function getResolvedColumnClassName(
  getColumnClassName?: DataTableColumnClassName,
  pinnedColumns?: DataTablePinnedColumn[]
): DataTableColumnClassName {
  return getResolvedColumnClassNameFromMap(
    getColumnClassName,
    getPinnedColumnMap(pinnedColumns)
  )
}

export function getResolvedColumnClassNameFromMap(
  getColumnClassName?: DataTableColumnClassName,
  pinnedColumnById?: Map<string, DataTablePinnedColumn>
): DataTableColumnClassName {
  return (columnId, kind) => {
    const customClassName = getColumnClassName?.(columnId, kind)
    const pinnedColumn = pinnedColumnById?.get(columnId)

    if (!pinnedColumn) return customClassName

    return cn(customClassName, getPinnedColumnClassName(pinnedColumn, kind))
  }
}

export function getPinnedColumnMap(pinnedColumns?: DataTablePinnedColumn[]) {
  if (!pinnedColumns?.length) return undefined

  return new Map(pinnedColumns.map((column) => [column.columnId, column]))
}

function getPinnedColumnClassName(
  pinnedColumn: DataTablePinnedColumn,
  kind: 'header' | 'cell'
) {
  return cn(
    'sticky whitespace-nowrap',
    pinnedColumn.side === 'left' ? 'left-0' : 'right-0',
    pinnedColumn.side === 'left'
      ? 'border-r border-border/70'
      : 'border-l border-border/70',
    kind === 'header'
      ? 'z-30 !bg-card'
      : // Pinned cells sit above scrolled content, so their hover/selected
        // backgrounds must be OPAQUE — a translucent bg-muted/50 lets the
        // content scrolling underneath show through the sticky column.
        'z-20 bg-card group-hover:[background-color:color-mix(in_oklch,var(--muted)_50%,var(--card))] group-data-[state=selected]:bg-muted',
    pinnedColumn.className,
    kind === 'header'
      ? pinnedColumn.headerClassName
      : pinnedColumn.cellClassName
  )
}
