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
import { useTranslation } from 'react-i18next'
import { formatCNYAmount } from '@/lib/currency'
import { Skeleton } from '@/components/ui/skeleton'
import { EditorialStat, EditorialStatGroup } from '@/components/editorial'
import type { UserWalletData } from '../types'

interface WalletStatsCardProps {
  user: UserWalletData | null
  loading?: boolean
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='editorial-panel overflow-hidden p-4'>
        <div className='grid gap-4 md:grid-cols-3'>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className='px-1 py-2'>
              <Skeleton className='h-3.5 w-20' />
              <Skeleton className='mt-3 h-9 w-28' />
              <Skeleton className='mt-1.5 h-3.5 w-24' />
            </div>
          ))}
        </div>
      </div>
    )
  }

  const stats = [
    {
      label: t('Current Balance'),
      value: formatCNYAmount(props.user?.balance_cny ?? 0),
      description: t('Available balance'),
    },
    {
      label: t('Total Usage'),
      value: formatCNYAmount(props.user?.used_cny ?? 0),
      description: t('Total spending'),
    },
    {
      label: t('API Requests'),
      value: (props.user?.request_count ?? 0).toLocaleString(),
      description: t('Total requests made'),
    },
  ]

  return (
    <div className='editorial-panel overflow-hidden p-4 sm:p-5'>
      <EditorialStatGroup className='border-0 py-0'>
        {stats.map((item, index) => (
          <EditorialStat
            key={item.label}
            label={item.label}
            value={item.value}
            accent={index === 0}
            description={item.description}
          />
        ))}
      </EditorialStatGroup>
    </div>
  )
}
