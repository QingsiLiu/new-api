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
/* eslint-disable react-refresh/only-export-components */
import * as React from 'react'
import { type LucideIcon } from 'lucide-react'
import { stringToColor } from '@/lib/colors'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

export const dotColorMap = {
  success: 'bg-calm-green-fg',
  warning: 'bg-calm-amber-fg',
  danger: 'bg-calm-rose-fg',
  info: 'bg-calm-blue-fg',
  neutral: 'bg-calm-gray-fg',
  purple: 'bg-calm-violet-fg',
  amber: 'bg-calm-amber-fg',
  blue: 'bg-calm-blue-fg',
  cyan: 'bg-calm-teal-fg',
  green: 'bg-calm-green-fg',
  grey: 'bg-calm-gray-fg',
  indigo: 'bg-calm-violet-fg',
  'light-blue': 'bg-calm-blue-fg',
  'light-green': 'bg-calm-green-fg',
  lime: 'bg-calm-green-fg',
  orange: 'bg-calm-amber-fg',
  pink: 'bg-calm-rose-fg',
  red: 'bg-calm-rose-fg',
  teal: 'bg-calm-teal-fg',
  violet: 'bg-calm-violet-fg',
  yellow: 'bg-calm-amber-fg',
} as const

export const textColorMap = {
  success: 'text-calm-green-fg',
  warning: 'text-calm-amber-fg',
  danger: 'text-calm-rose-fg',
  info: 'text-calm-blue-fg',
  neutral: 'text-calm-gray-fg',
  purple: 'text-calm-violet-fg',
  amber: 'text-calm-amber-fg',
  blue: 'text-calm-blue-fg',
  cyan: 'text-calm-teal-fg',
  green: 'text-calm-green-fg',
  grey: 'text-calm-gray-fg',
  indigo: 'text-calm-violet-fg',
  'light-blue': 'text-calm-blue-fg',
  'light-green': 'text-calm-green-fg',
  lime: 'text-calm-green-fg',
  orange: 'text-calm-amber-fg',
  pink: 'text-calm-rose-fg',
  red: 'text-calm-rose-fg',
  teal: 'text-calm-teal-fg',
  violet: 'text-calm-violet-fg',
  yellow: 'text-calm-amber-fg',
} as const

export type StatusVariant = keyof typeof dotColorMap

export const badgeSurfaceMap: Record<StatusVariant, string> = {
  success: 'bg-calm-green-bg',
  warning: 'bg-calm-amber-bg',
  danger: 'bg-calm-rose-bg',
  info: 'bg-calm-blue-bg',
  neutral: 'bg-calm-gray-bg',
  purple: 'bg-calm-violet-bg',
  amber: 'bg-calm-amber-bg',
  blue: 'bg-calm-blue-bg',
  cyan: 'bg-calm-teal-bg',
  green: 'bg-calm-green-bg',
  grey: 'bg-calm-gray-bg',
  indigo: 'bg-calm-violet-bg',
  'light-blue': 'bg-calm-blue-bg',
  'light-green': 'bg-calm-green-bg',
  lime: 'bg-calm-green-bg',
  orange: 'bg-calm-amber-bg',
  pink: 'bg-calm-rose-bg',
  red: 'bg-calm-rose-bg',
  teal: 'bg-calm-teal-bg',
  violet: 'bg-calm-violet-bg',
  yellow: 'bg-calm-amber-bg',
}

/** Controls the visual style of the badge.
 * - `badge`    — default editorial chip with dot and mono text (default)
 * - `text`     — plain text, no background or padding, only color
 * - `underline`— plain text with a bottom border underline
 */
export type StatusBadgeType = 'badge' | 'text' | 'underline'

/** Context that lets ancestor components (e.g. MobileCardList field area)
 *  override the badge type without modifying every call site. */
export const StatusBadgeTypeContext =
  React.createContext<StatusBadgeType>('badge')

const sizeMap = {
  sm: 'h-5 gap-1.5 px-2.5 text-[0.6875rem] leading-none',
  md: 'h-5 gap-1.5 px-2.5 text-[0.6875rem] leading-none',
  lg: 'h-6 gap-1.5 px-3 text-xs leading-none',
} as const

const textSizeMap = {
  sm: 'gap-1.5 text-[0.6875rem] leading-none',
  md: 'gap-1.5 text-[0.6875rem] leading-none',
  lg: 'gap-1.5 text-xs leading-none',
} as const

export interface StatusBadgeProps extends Omit<
  React.HTMLAttributes<HTMLSpanElement>,
  'children'
> {
  label?: string
  children?: React.ReactNode
  icon?: LucideIcon
  pulse?: boolean
  /** Kept for compatibility; when enabled, renders a small color swatch. */
  showDot?: boolean
  variant?: StatusVariant | null
  size?: 'sm' | 'md' | 'lg' | null
  copyable?: boolean
  copyText?: string
  autoColor?: string
  /** Visual style. Defaults to 'badge'. Can be overridden via StatusBadgeTypeContext. */
  type?: StatusBadgeType
}

export function StatusBadge({
  label,
  children,
  icon: Icon,
  variant,
  size = 'sm',
  pulse = false,
  showDot = false,
  copyable = true,
  copyText,
  autoColor,
  type: typeProp,
  className,
  onClick,
  ...props
}: StatusBadgeProps) {
  const { copyToClipboard } = useCopyToClipboard()
  const contextType = React.useContext(StatusBadgeTypeContext)
  const type = typeProp ?? contextType

  const computedVariant: StatusVariant = autoColor
    ? (stringToColor(autoColor) as StatusVariant)
    : (variant ?? 'neutral')

  const handleClick = (e: React.MouseEvent<HTMLSpanElement>) => {
    if (copyable) {
      e.stopPropagation()
      copyToClipboard(copyText || label || '')
    }
    onClick?.(e)
  }

  const content =
    children ??
    (label ? (
      <span className='min-w-0 truncate leading-normal'>{label}</span>
    ) : null)

  const isBadge = type === 'badge'
  const title = copyable
    ? `Click to copy: ${copyText || label || ''}`
    : label || undefined

  return (
    <span
      data-slot='status-badge'
      className={cn(
        'inline-flex w-fit max-w-full min-w-0 shrink items-center font-medium tracking-normal whitespace-nowrap transition-colors',
        isBadge
          ? cn(
              'rounded-[var(--radius-pill)]',
              badgeSurfaceMap[computedVariant],
              sizeMap[size ?? 'sm']
            )
          : cn(
              textSizeMap[size ?? 'sm'],
              type === 'underline' && 'border-b border-current pb-px'
            ),
        textColorMap[computedVariant],
        pulse && 'animate-pulse',
        copyable &&
          'cursor-copy hover:brightness-95 active:scale-95 dark:hover:brightness-110',
        className
      )}
      onClick={handleClick}
      title={title}
      {...props}
    >
      {showDot && (
        <span
          className={cn(
            'inline-block size-1.5 shrink-0 rounded-full',
            dotColorMap[computedVariant]
          )}
          aria-hidden='true'
        />
      )}
      {Icon && <Icon className='size-3.5 shrink-0' />}
      {content}
    </span>
  )
}

export interface StatusBadgeListProps<T> extends Omit<
  React.HTMLAttributes<HTMLDivElement>,
  'children'
> {
  empty?: React.ReactNode
  getKey?: (item: T, index: number) => React.Key
  items: T[]
  max?: number
  moreLabel?: (remaining: number) => string
  renderItem: (item: T, index: number) => React.ReactNode
}

export function StatusBadgeList<T>(props: StatusBadgeListProps<T>) {
  const {
    className,
    empty = <span className='text-muted-foreground text-xs'>-</span>,
    getKey,
    items,
    max = 2,
    moreLabel,
    renderItem,
    ...domProps
  } = props

  if (items.length === 0) {
    return empty
  }

  const displayed = items.slice(0, max)
  const remaining = items.length - max

  return (
    <div
      className={cn(
        'flex max-w-full min-w-0 items-center gap-1 overflow-hidden',
        className
      )}
      {...domProps}
    >
      {displayed.map((item, index) => (
        <React.Fragment key={getKey?.(item, index) ?? index}>
          {renderItem(item, index)}
        </React.Fragment>
      ))}
      {remaining > 0 && (
        <StatusBadge
          label={moreLabel?.(remaining) ?? `+${remaining}`}
          variant='neutral'
          size='sm'
          copyable={false}
          className='shrink-0'
        />
      )}
    </div>
  )
}

export const statusPresets = {
  active: {
    variant: 'success' as const,
    label: 'Active',
  },
  inactive: {
    variant: 'neutral' as const,
    label: 'Inactive',
  },
  invited: {
    variant: 'info' as const,
    label: 'Invited',
  },
  suspended: {
    variant: 'danger' as const,
    label: 'Suspended',
  },
  pending: {
    variant: 'warning' as const,
    label: 'Pending',
    pulse: true,
  },
} as const

export type StatusPreset = keyof typeof statusPresets
