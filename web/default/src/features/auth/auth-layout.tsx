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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()

  return (
    <div className='bg-background relative min-h-svh overflow-hidden'>
      <Link
        to='/'
        className='absolute top-5 left-5 z-10 flex items-center gap-2 transition-opacity hover:opacity-80 sm:top-8 sm:left-8'
      >
        <div className='border-border bg-card relative h-8 w-8 overflow-hidden rounded-md border'>
          {loading ? (
            <Skeleton className='absolute inset-0 rounded-none' />
          ) : (
            <img
              src={logo}
              alt={t('Logo')}
              className='h-full w-full object-cover'
            />
          )}
        </div>
        {loading ? (
          <Skeleton className='h-6 w-24' />
        ) : (
          <h1 className='text-foreground text-sm font-semibold'>{systemName}</h1>
        )}
      </Link>

      <div className='mx-auto grid min-h-svh w-full max-w-6xl px-5 pt-24 pb-8 sm:px-8 lg:grid-cols-[minmax(0,0.9fr)_minmax(26rem,0.62fr)] lg:items-center lg:gap-16 lg:pt-8'>
        <aside className='hidden border-l pl-6 lg:block'>
          <p className='text-muted-foreground mb-5 text-xs font-medium'>
            {t('Access Console')}
          </p>
          <h2 className='editorial-display max-w-xl text-5xl'>{systemName}</h2>
          <p className='text-muted-foreground mt-6 max-w-sm text-sm leading-7'>
            {t(
              'A quiet control surface for routing, billing, keys, and model operations.'
            )}
          </p>
        </aside>

        <div className='flex w-full items-center justify-center lg:justify-end'>
          <div className='editorial-panel w-full max-w-[30rem] px-5 py-6 sm:px-8 sm:py-8'>
            {children}
          </div>
        </div>
      </div>
    </div>
  )
}
