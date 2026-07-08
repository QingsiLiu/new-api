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
  const columns = table.getVisibleLeafColumns()
  const width = columns.reduce((total, column) => total + column.getSize(), 0)
  const hasFlexColumn = columns.some((column) => column.columnDef.meta?.flex)

  // table-layout: fixed keeps columns at their defined sizes. min-width enables
  // horizontal scroll when the columns are wider than the container.
  //
  // Without a flex column, cap the table at the columns' total width (max-width)
  // so a container wider than the table leaves the SLACK OUTSIDE the table
  // rather than inflating the last (pinned actions) column.
  //
  // With a flex column, drop the cap: width:100% lets the table fill the
  // container and the flex column (auto width in the colgroup) absorbs the
  // slack, so the actions column stays flush against the container's right edge
  // with no trailing blank.
  return {
    minWidth: width,
    ...(hasFlexColumn ? {} : { maxWidth: width }),
    tableLayout: 'fixed',
    width: '100%',
  }
}
