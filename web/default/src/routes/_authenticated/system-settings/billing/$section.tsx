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
import { createFileRoute, redirect } from '@tanstack/react-router'

import { BillingSettings } from '@/features/system-settings/billing'
import {
  BILLING_DEFAULT_SECTION,
  BILLING_SECTION_IDS,
} from '@/features/system-settings/billing/section-registry.tsx'

const REMOVED_BILLING_SECTION_REDIRECTS: Record<
  string,
  | { to: '/system-settings/billing/$section'; section: 'quota' }
  | { to: '/models/$section'; section: 'metadata' }
  | { to: '/users' }
> = {
  currency: { to: '/system-settings/billing/$section', section: 'quota' },
  'model-pricing': { to: '/models/$section', section: 'metadata' },
  'spec-pricing': { to: '/models/$section', section: 'metadata' },
  'group-pricing': { to: '/users' },
}

export const Route = createFileRoute(
  '/_authenticated/system-settings/billing/$section'
)({
  beforeLoad: ({ params }) => {
    const removedSection = REMOVED_BILLING_SECTION_REDIRECTS[params.section]
    if (removedSection) {
      if ('section' in removedSection) {
        throw redirect({
          to: removedSection.to,
          params: { section: removedSection.section },
        })
      }
      throw redirect({ to: removedSection.to })
    }

    const validSections = BILLING_SECTION_IDS as unknown as string[]
    if (!validSections.includes(params.section)) {
      throw redirect({
        to: '/system-settings/billing/$section',
        params: { section: BILLING_DEFAULT_SECTION },
      })
    }
  },
  component: BillingSettings,
})
