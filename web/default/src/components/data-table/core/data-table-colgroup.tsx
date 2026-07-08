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
import type { Table as TanstackTable } from '@tanstack/react-table'

export function DataTableColgroup<TData>({
  table,
}: {
  table: TanstackTable<TData>
}) {
  const columns = table.getVisibleLeafColumns()

  return (
    <colgroup>
      {columns.map((column) => {
        // A `flex` column has no fixed width: under table-layout:fixed the
        // browser distributes the table's leftover width to auto-width cols,
        // so it stretches to fill the container. Its size becomes a min width.
        const isFlex = column.columnDef.meta?.flex
        return (
          <col
            key={column.id}
            style={
              isFlex
                ? { minWidth: `${column.getSize()}px` }
                : { width: `${column.getSize()}px` }
            }
          />
        )
      })}
    </colgroup>
  )
}
