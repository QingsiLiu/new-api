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
import type * as React from 'react'
import type { Table as TanstackTable } from '@tanstack/react-table'

export function getTableSizeStyle<TData>(
  table: TanstackTable<TData>
): React.CSSProperties {
  const width = table
    .getVisibleLeafColumns()
    .reduce((total, column) => total + column.getSize(), 0)

  // Use table-layout: fixed to prevent columns from expanding beyond their
  // defined sizes. Set min-width to enable horizontal scroll when needed,
  // and max-width to prevent the table from stretching beyond the total
  // column widths (which would cause the last column to absorb extra space).
  // width: 100% ensures the table fills its container when columns are narrower.
  return {
    minWidth: width,
    maxWidth: width,
    tableLayout: 'fixed',
    width: '100%',
  }
}
