import { AlertTriangle, CheckCircle2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getEffectiveTextPricingSummary,
  getModelModalLabel,
  summarizeModelPricing,
} from '../lib'
import type { Model } from '../types'

export function ModelClassificationCell(props: { model: Model }) {
  const { t } = useTranslation()

  return (
    <div className='flex flex-wrap gap-1'>
      <StatusBadge
        label={getModelModalLabel(t, props.model.modal)}
        autoColor={props.model.modal || 'unspecified'}
        size='sm'
      />
      {props.model.modal === 'text' && props.model.text_category ? (
        <StatusBadge
          label={
            props.model.text_category === 'unclassified'
              ? t('Unclassified')
              : props.model.text_category.toUpperCase()
          }
          variant={
            props.model.text_category === 'unclassified' ? 'warning' : 'info'
          }
          size='sm'
        />
      ) : null}
    </div>
  )
}

export function OfficialPriceProfileCell(props: { model: Model }) {
  const { t } = useTranslation()

  if (props.model.modal !== 'text') {
    return (
      <span className='text-muted-foreground text-xs'>
        {t('Specification pricing')}
      </span>
    )
  }

  const profile = props.model.official_price_profile
  if (!profile) {
    return <StatusBadge variant='warning' size='sm' label={t('Missing')} />
  }

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={<div className='min-w-0 cursor-help' />}>
          <div className='max-w-[220px] truncate text-sm font-medium'>
            {profile.display_name}
          </div>
          <div className='text-muted-foreground max-w-[220px] truncate font-mono text-xs'>
            {profile.key}
          </div>
        </TooltipTrigger>
        <TooltipContent side='top'>
          <div className='max-w-80 space-y-1'>
            <div>{profile.display_name}</div>
            <div className='font-mono text-xs'>{profile.key}</div>
            {profile.tiers?.length ? (
              <div className='text-xs'>
                {t('{{count}} context tiers', {
                  count: profile.tiers.length,
                })}
              </div>
            ) : null}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function CategoryMultiplierCell(props: { model: Model }) {
  const multiplier = props.model.effective_text_pricing?.category_multiplier
  if (!Number.isFinite(multiplier)) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  return (
    <span className='font-mono text-sm'>{Number(multiplier).toFixed(4)}x</span>
  )
}

export function PricingAvailabilityCell(props: { model: Model }) {
  const { t } = useTranslation()

  if (props.model.modal !== 'text') {
    const configured =
      props.model.pricing_mode === 'image_spec' ||
      props.model.pricing_mode === 'video_matrix' ||
      props.model.pricing_mode === 'free'
    return (
      <StatusBadge
        variant={configured ? 'success' : 'warning'}
        size='sm'
        copyable={false}
      >
        {configured ? t('Configured') : t('Legacy fallback')}
      </StatusBadge>
    )
  }

  const badge = (
    <StatusBadge
      variant={props.model.pricing_ready ? 'success' : 'danger'}
      size='sm'
      copyable={false}
    >
      {props.model.pricing_ready ? (
        <CheckCircle2 aria-hidden='true' />
      ) : (
        <AlertTriangle aria-hidden='true' />
      )}
      {props.model.pricing_ready ? t('Ready') : t('Blocked')}
    </StatusBadge>
  )
  if (!props.model.pricing_error) return badge

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={<div className='cursor-help' />}>
          {badge}
        </TooltipTrigger>
        <TooltipContent className='max-w-80'>
          {props.model.pricing_error}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function PricingSummaryCell(props: { model: Model }) {
  const { t } = useTranslation()

  if (props.model.modal !== 'text') {
    return (
      <span className='text-muted-foreground text-xs'>
        {summarizeModelPricing(props.model, t)}
      </span>
    )
  }

  const summary = getEffectiveTextPricingSummary(
    props.model.effective_text_pricing
  )
  if (summary.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  return (
    <div className='space-y-0.5 font-mono text-xs'>
      {summary.slice(0, 3).map((item) => (
        <div key={item.key} className='whitespace-nowrap'>
          <span className='text-muted-foreground'>
            {getPricingDimensionLabel(item.key, t)}
          </span>{' '}
          {item.credits.toFixed(6)} C
        </div>
      ))}
    </div>
  )
}

function getPricingDimensionLabel(
  key: string,
  t: ReturnType<typeof useTranslation>['t']
): string {
  switch (key) {
    case 'input':
      return t('Input')
    case 'output':
      return t('Output')
    case 'cached_input':
      return t('Cache read')
    case 'cache_write':
      return t('Cache write')
    case 'cache_write_5m':
      return t('Cache write 5m')
    case 'cache_write_1h':
      return t('Cache write 1h')
    default:
      return key
  }
}
