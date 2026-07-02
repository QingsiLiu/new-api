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
import { useNavigate } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

export function LegacyModelPricingNotice({
  descriptionKey,
}: {
  descriptionKey: string
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return (
    <Alert className='mb-4'>
      <AlertDescription className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='space-y-1'>
          <div className='text-foreground font-medium'>
            {t('Legacy pricing is read-only')}
          </div>
          <div>{t(descriptionKey)}</div>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() =>
            void navigate({
              to: '/models/$section',
              params: { section: 'metadata' },
            })
          }
        >
          {t('Open model center')}
          <ArrowRight className='size-4' />
        </Button>
      </AlertDescription>
    </Alert>
  )
}
