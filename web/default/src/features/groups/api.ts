/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import type {
  GroupRegistryDetailResponse,
  GroupRegistryItem,
  GroupRegistryResponse,
} from './types'

export async function getGroupRegistry(): Promise<GroupRegistryResponse> {
  const res = await api.get('/api/group/registry')
  return res.data
}

export async function getUserGroupRegistry(): Promise<GroupRegistryResponse> {
  const res = await api.get('/api/user/self/groups')
  return res.data
}

export async function createGroupRegistry(data: {
  display_name: string
  description?: string
  ratio?: number
  user_usable?: boolean
  sort?: number
}): Promise<GroupRegistryDetailResponse> {
  const res = await api.post('/api/group/registry', data)
  return res.data
}

export async function updateGroupRegistry(
  code: string,
  data: {
    display_name?: string
    description?: string
    ratio?: number
    user_usable?: boolean
    sort?: number
  }
): Promise<GroupRegistryDetailResponse> {
  const res = await api.put(`/api/group/registry/${code}`, data)
  return res.data
}

export async function deleteGroupRegistry(code: string): Promise<{
  success: boolean
  message?: string
  data?: Record<string, number>
}> {
  const res = await api.delete(`/api/group/registry/${code}`)
  return res.data
}

export type { GroupRegistryItem }
