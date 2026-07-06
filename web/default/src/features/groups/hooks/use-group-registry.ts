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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useLocation } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { getGroupRegistry, getUserGroupRegistry } from '../api'
import {
  groupRegistryScopeForRole,
  normalizeGroupRegistryItems,
} from '../utils'

export function useGroupRegistry() {
  const userRole = useAuthStore((s) => s.auth.user?.role)
  const pathname = useLocation({ select: (location) => location.pathname })
  const scope = groupRegistryScopeForRole(userRole, pathname)
  const { data } = useQuery({
    queryKey: ['group-registry', scope],
    queryFn: scope === 'admin' ? getGroupRegistry : getUserGroupRegistry,
    enabled: scope !== 'anonymous',
    staleTime: 5 * 60 * 1000,
  })

  return useMemo(() => {
    const items = normalizeGroupRegistryItems(data)
    const byCode = new Map<string, (typeof items)[number]>()
    for (const item of items) {
      byCode.set(item.code, item)
    }
    const getDisplayName = (code?: string | null) => {
      const trimmed = code?.trim()
      if (!trimmed) return ''
      return byCode.get(trimmed)?.display_name || trimmed
    }
    const getItem = (code?: string | null) => {
      const trimmed = code?.trim()
      if (!trimmed) return undefined
      return byCode.get(trimmed)
    }
    return { items, byCode, getDisplayName, getItem }
  }, [data])
}
