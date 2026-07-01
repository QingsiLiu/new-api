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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useGroupRegistry } from '../hooks/use-group-registry'

type GroupSelectProps = {
  value?: string | null
  onValueChange: (value: string) => void
  placeholder?: string
  className?: string
  disabled?: boolean
  includeValues?: string[]
}

export function GroupSelect(props: GroupSelectProps) {
  const groupRegistry = useGroupRegistry()
  const selectedValue = props.value?.trim() || ''

  const options = useMemo(() => {
    const byCode = new Map(
      groupRegistry.items.map((group) => [group.code, group])
    )
    for (const value of [selectedValue, ...(props.includeValues ?? [])]) {
      const code = value.trim()
      if (!code || byCode.has(code)) continue
      byCode.set(code, {
        code,
        display_name: groupRegistry.getDisplayName(code),
        description: '',
        ratio: 1,
        user_usable: false,
        is_reserved: false,
        sort: 0,
      })
    }
    return Array.from(byCode.values())
  }, [groupRegistry, props.includeValues, selectedValue])

  const selectedLabel = selectedValue
    ? groupRegistry.getDisplayName(selectedValue)
    : props.placeholder

  return (
    <Select
      items={options.map((group) => ({
        value: group.code,
        label: group.display_name || group.code,
      }))}
      value={selectedValue || null}
      onValueChange={(value) => value !== null && props.onValueChange(value)}
      disabled={props.disabled}
    >
      <SelectTrigger className={props.className}>
        <SelectValue>{selectedLabel}</SelectValue>
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {options.map((group) => (
            <SelectItem key={group.code} value={group.code}>
              {group.display_name || group.code}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
