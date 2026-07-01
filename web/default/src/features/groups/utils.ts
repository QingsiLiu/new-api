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
import type { GroupRegistryItem, GroupRegistryResponse } from './types'

export function normalizeGroupRegistryItems(
  response?: GroupRegistryResponse | null
): GroupRegistryItem[] {
  const rawGroups =
    response?.groups && response.groups.length > 0
      ? response.groups
      : response?.data

  if (!Array.isArray(rawGroups)) return []

  return rawGroups
    .map((group) => {
      if (typeof group === 'string') {
        return {
          code: group,
          display_name: group,
          description: '',
          ratio: 1,
          user_usable: false,
          is_reserved: false,
          sort: 0,
        }
      }

      return {
        code: group.code,
        display_name: group.display_name || group.code,
        description: group.description || '',
        ratio: group.ratio ?? 1,
        user_usable: group.user_usable ?? false,
        is_reserved: group.is_reserved ?? false,
        sort: group.sort ?? 0,
      }
    })
    .filter((group) => group.code)
}
