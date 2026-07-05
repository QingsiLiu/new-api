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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SettingsSection } from '../components/settings-section'

type PricingSectionProps = {
  defaultValues: unknown
}

export function PricingSection({ defaultValues: _ }: PricingSectionProps) {
  const { t } = useTranslation()

  return (
    <SettingsSection title={t('Pricing & Display')}>
      <div className='max-w-sm space-y-2'>
        <div className='space-y-2'>
          <Label>{t('Currency')}</Label>
          <Input value='CNY' readOnly />
        </div>
      </div>
    </SettingsSection>
  )
}
