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
import { EditorialLabel } from './editorial-label'

type EditorialStatProps = React.HTMLAttributes<HTMLDivElement> & {
  label: React.ReactNode
  value: React.ReactNode
  accent?: boolean
  description?: React.ReactNode
}

type EditorialStatGroupProps = React.HTMLAttributes<HTMLDivElement>

export function EditorialStat({
  accent = false,
  className,
  description,
  label,
  value,
  ...props
}: EditorialStatProps) {
  return (
    <div
      className={cn(
        'border-border flex min-w-0 flex-col gap-2 py-3 first:pl-0 md:border-l md:pl-5 md:first:border-l-0',
        className
      )}
      {...props}
    >
      <EditorialLabel>{label}</EditorialLabel>
      <div
        className={cn(
          'editorial-stat-value min-w-0 truncate',
          accent && 'text-primary'
        )}
      >
        {value}
      </div>
      {description != null && (
        <div className='text-muted-foreground text-sm leading-relaxed'>
          {description}
        </div>
      )}
    </div>
  )
}

export function EditorialStatGroup({
  className,
  ...props
}: EditorialStatGroupProps) {
  return (
    <div
      className={cn(
        'border-border grid gap-4 border-y py-4 md:grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] md:gap-0',
        className
      )}
      {...props}
    />
  )
}
