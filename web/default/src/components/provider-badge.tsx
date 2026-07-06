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
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { StatusBadge, type StatusBadgeProps } from './status-badge'

type ProviderBadgeProps = Omit<StatusBadgeProps, 'children' | 'label'> & {
  iconKey?: string | null
  iconSize?: number
  label: string
}

function getProviderVariant(
  label: string,
  iconKey?: string | null
): StatusBadgeProps['variant'] {
  const identity = `${iconKey ?? ''} ${label}`.toLowerCase()

  if (identity.includes('openai')) return 'green'
  if (identity.includes('anthropic') || identity.includes('claude')) {
    return 'violet'
  }
  if (
    identity.includes('gemini') ||
    identity.includes('google') ||
    identity.includes('vertex') ||
    identity.includes('azure')
  ) {
    return 'blue'
  }
  if (identity.includes('kie')) return 'teal'

  return undefined
}

export function ProviderBadge({
  className,
  iconKey,
  iconSize = 14,
  label,
  variant,
  ...badgeProps
}: ProviderBadgeProps) {
  const icon = iconKey ? getLobeIcon(iconKey, iconSize) : null
  const providerVariant = variant ?? getProviderVariant(label, iconKey)

  return (
    <StatusBadge
      data-slot='provider-badge'
      label={label}
      autoColor={providerVariant ? undefined : label}
      variant={providerVariant}
      size='sm'
      className={cn('min-w-0 shrink overflow-hidden', className)}
      {...badgeProps}
    >
      {icon && <span className='flex shrink-0 items-center'>{icon}</span>}
      <span className='min-w-0 truncate leading-normal'>{label}</span>
    </StatusBadge>
  )
}
