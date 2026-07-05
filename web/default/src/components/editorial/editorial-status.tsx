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
import * as React from 'react'
import { cn } from '@/lib/utils'

const statusToneClassName = {
  success: 'bg-success/15 text-success',
  progress: 'bg-primary/15 text-primary',
  danger: 'bg-destructive/15 text-destructive',
  neutral: 'bg-muted text-muted-foreground',
  warning: 'bg-warning/15 text-warning',
  info: 'bg-info/15 text-info',
} as const

type EditorialStatusTone = keyof typeof statusToneClassName

type EditorialStatusProps = Omit<
  React.HTMLAttributes<HTMLSpanElement>,
  'children'
> & {
  children: React.ReactNode
  tone?: EditorialStatusTone
}

export function EditorialStatus({
  children,
  className,
  tone = 'neutral',
  ...props
}: EditorialStatusProps) {
  return (
    <span
      className={cn(
        'inline-flex min-w-0 items-center rounded-md px-1.5 text-xs font-medium',
        statusToneClassName[tone],
        className
      )}
      {...props}
    >
      <span className='min-w-0 truncate'>{children}</span>
    </span>
  )
}

export type { EditorialStatusTone }
