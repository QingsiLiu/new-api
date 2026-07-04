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
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-destructive',
  info: 'bg-info',
  neutral: 'bg-neutral',
  purple: 'bg-chart-2',
  amber: 'bg-warning',
  blue: 'bg-chart-5',
  cyan: 'bg-chart-3',
  green: 'bg-success',
  grey: 'bg-neutral',
  indigo: 'bg-chart-2',
  'light-blue': 'bg-info',
  'light-green': 'bg-success',
  lime: 'bg-success',
  orange: 'bg-warning',
  pink: 'bg-chart-1',
  red: 'bg-destructive',
  teal: 'bg-chart-3',
  violet: 'bg-chart-2',
  yellow: 'bg-warning',
} as const

export const textColorMap = {
  success: 'text-success',
  warning: 'text-warning',
  danger: 'text-destructive',
  info: 'text-info',
  neutral: 'text-muted-foreground',
  purple: 'text-chart-2',
  amber: 'text-warning',
  blue: 'text-chart-5',
  cyan: 'text-chart-3',
  green: 'text-success',
  grey: 'text-muted-foreground',
  indigo: 'text-chart-2',
  'light-blue': 'text-info',
  'light-green': 'text-success',
  lime: 'text-success',
  orange: 'text-warning',
  pink: 'text-chart-1',
  red: 'text-destructive',
  teal: 'text-chart-3',
  violet: 'text-chart-2',
  yellow: 'text-warning',
} as const

export type StatusVariant = keyof typeof dotColorMap

export const badgeSurfaceMap: Record<StatusVariant, string> = {
  success: 'border-success/25 bg-success/15',
  warning: 'border-warning/30 bg-warning/15',
  danger: 'border-destructive/30 bg-destructive/15',
  info: 'border-info/25 bg-info/15',
  neutral: 'border-border bg-muted/60',
  purple: 'border-chart-2/25 bg-chart-2/15',
  amber: 'border-warning/30 bg-warning/15',
  blue: 'border-chart-5/25 bg-chart-5/15',
  cyan: 'border-chart-3/25 bg-chart-3/15',
  green: 'border-success/25 bg-success/15',
  grey: 'border-border bg-muted/60',
  indigo: 'border-chart-2/25 bg-chart-2/15',
  'light-blue': 'border-info/25 bg-info/15',
  'light-green': 'border-success/25 bg-success/15',
  lime: 'border-success/25 bg-success/15',
  orange: 'border-warning/30 bg-warning/15',
  pink: 'border-chart-1/25 bg-chart-1/15',
  red: 'border-destructive/30 bg-destructive/15',
  teal: 'border-chart-3/25 bg-chart-3/15',
  violet: 'border-chart-2/25 bg-chart-2/15',
  yellow: 'border-warning/30 bg-warning/15',
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
  sm: 'h-5 gap-1.5 px-1.5 text-[0.6875rem] leading-none',
  md: 'h-5 gap-1.5 px-1.5 text-[0.6875rem] leading-none',
  lg: 'h-6 gap-1.5 px-2 text-xs leading-none',
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
  /** Kept for compatibility; badges render a leading dot by default. */
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
  showDot = true,
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
        'font-mono tracking-[0.12em] uppercase',
        isBadge
          ? cn(
              'rounded-full border',
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
