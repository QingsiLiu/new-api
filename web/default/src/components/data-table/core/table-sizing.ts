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

  // Grow to fill the container when the columns are narrower than the viewport
  // (via `width: 100%`), but never stretch individual fixed columns beyond
  // their defined size — the last column (usually pinned actions) otherwise
  // absorbs all the slack and shows a large empty gap on the right.
  // `min-width` keeps horizontal scroll when columns are wider than the
  // container; `max-width` caps the table at the columns' total intrinsic width
  // so the extra space stays OUTSIDE the table instead of inflating a column.
  return {
    minWidth: width,
    maxWidth: width,
    tableLayout: 'fixed',
    width: '100%',
  }
}
