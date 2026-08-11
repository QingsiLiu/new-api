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
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import {
  Calculator,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Loader2,
  Pencil,
  Search,
  SlidersHorizontal,
  TriangleAlert,
} from 'lucide-react'
import { useDeferredValue, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getModels,
  previewTextPricingCategory,
  searchModels,
  updateTextPricingCategory,
} from '../api'
import {
  buildTextPricingModelQuery,
  getEffectiveTextPricingSummary,
  getOfficialDimensionsSummary,
  modelsQueryKeys,
  parseTextPricingMultiplierInput,
} from '../lib'
import type {
  EffectiveTextPricing,
  EffectiveTextPricingTier,
  Model,
  OfficialPriceDimensions,
  OfficialPriceProfile,
  OfficialPriceTier,
  TextPricingCategoryConfig,
  TextPricingGroup,
} from '../types'
import { useModels } from './models-provider'
import { TextPricingModelDialog } from './text-pricing-model-dialog'

const GROUP_PAGE_SIZE = 10

type TextPricingGroupRowProps = {
  group: TextPricingCategoryConfig & { category: TextPricingGroup }
  catalogVersion: string
}

export function TextPricingGroupRow(props: TextPricingGroupRowProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [multiplierInput, setMultiplierInput] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [editingModel, setEditingModel] = useState<Model | null>(null)

  useEffect(() => {
    setMultiplierInput(
      props.group.multiplier === undefined ? '' : String(props.group.multiplier)
    )
  }, [props.group.multiplier])

  const parsedMultiplier = parseTextPricingMultiplierInput(multiplierInput)
  const multiplierInvalid =
    multiplierInput.length > 0 && !parsedMultiplier.valid
  const multiplierErrorId = `group-multiplier-error-${props.group.category}`
  const multiplierChanged =
    parsedMultiplier.valid &&
    parsedMultiplier.multiplier !== props.group.multiplier

  const previewMutation = useMutation({
    mutationFn: previewTextPricingCategory,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Unable to preview pricing impact'))
        return
      }
      setConfirmOpen(true)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Unable to preview pricing impact'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: updateTextPricingCategory,
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(
          response.message || t('Unable to update category multiplier')
        )
        return
      }
      toast.success(t('Category multiplier updated'))
      setConfirmOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: modelsQueryKeys.textPricing(),
        }),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
      ])
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Unable to update category multiplier'))
    },
  })

  const handlePreview = () => {
    if (!parsedMultiplier.valid || !multiplierChanged) return
    previewMutation.mutate({
      category: props.group.category,
      multiplier: parsedMultiplier.multiplier,
    })
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div className='grid min-h-20 grid-cols-[auto_minmax(0,1fr)] gap-3 px-3 py-3 lg:grid-cols-[auto_minmax(120px,1.1fr)_minmax(110px,0.8fr)_minmax(150px,0.9fr)_minmax(230px,1fr)] lg:items-center'>
        <CollapsibleTrigger
          className='hover:bg-muted focus-visible:ring-ring mt-0.5 flex size-8 items-center justify-center rounded-md outline-none focus-visible:ring-2 lg:mt-0'
          aria-label={
            open
              ? t('Collapse {{group}} models', {
                  group: props.group.category.toUpperCase(),
                })
              : t('Expand {{group}} models', {
                  group: props.group.category.toUpperCase(),
                })
          }
        >
          <ChevronDown
            className={`size-4 transition-transform ${open ? 'rotate-180' : ''}`}
            aria-hidden='true'
          />
        </CollapsibleTrigger>

        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <CollapsibleTrigger className='hover:text-primary focus-visible:ring-ring rounded-sm text-left text-sm font-semibold outline-none focus-visible:ring-2'>
              {props.group.category.toUpperCase()}
            </CollapsibleTrigger>
            {props.group.activation_ready ? (
              <Badge variant='outline'>
                <CheckCircle2 aria-hidden='true' />
                {t('Ready')}
              </Badge>
            ) : (
              <Badge variant='destructive'>
                <TriangleAlert aria-hidden='true' />
                {t('Not ready')}
              </Badge>
            )}
          </div>
          {props.group.activation_error ? (
            <p className='text-destructive mt-1 text-xs break-words'>
              {props.group.activation_error}
            </p>
          ) : null}
        </div>

        <GroupMetric
          label={t('Billable')}
          value={`${props.group.pricing_ready_count || 0} / ${props.group.model_count || 0}`}
        />
        <div className='grid grid-cols-2 gap-3 lg:block'>
          <GroupMetric
            label={t('Overrides')}
            value={props.group.override_count || 0}
          />
          <div className='lg:mt-1'>
            <span className='text-muted-foreground block text-xs'>
              {t('Catalog')}
            </span>
            <span className='block truncate font-mono text-xs'>
              {props.catalogVersion}
            </span>
          </div>
        </div>

        <div className='col-span-2 flex min-w-0 items-end gap-2 lg:col-span-1'>
          <div className='min-w-0 flex-1 space-y-1'>
            <Label htmlFor={`group-multiplier-${props.group.category}`}>
              {t('Group multiplier')}
            </Label>
            <Input
              id={`group-multiplier-${props.group.category}`}
              inputMode='decimal'
              value={multiplierInput}
              onChange={(event) => {
                setMultiplierInput(event.target.value)
                previewMutation.reset()
              }}
              aria-invalid={multiplierInvalid}
              aria-describedby={
                multiplierInvalid ? multiplierErrorId : undefined
              }
              placeholder='0.1000'
            />
            {multiplierInvalid ? (
              <p
                id={multiplierErrorId}
                role='alert'
                className='text-destructive text-xs'
              >
                {t('Greater than 0, at most 1, up to 4 decimals.')}
              </p>
            ) : null}
          </div>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={
              !parsedMultiplier.valid ||
              !multiplierChanged ||
              previewMutation.isPending
            }
            onClick={handlePreview}
          >
            {previewMutation.isPending ? (
              <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            ) : (
              <Calculator className='size-4' aria-hidden='true' />
            )}
            {t('Preview')}
          </Button>
          <CollapsibleTrigger
            render={<Button type='button' size='sm' variant='ghost' />}
            aria-label={
              open
                ? t('Collapse {{group}} models', {
                    group: props.group.category.toUpperCase(),
                  })
                : t('Expand {{group}} models', {
                    group: props.group.category.toUpperCase(),
                  })
            }
          >
            {open ? t('Collapse') : t('View details')}
          </CollapsibleTrigger>
        </div>
      </div>

      <CollapsibleContent className='border-t'>
        <GroupModelList
          category={props.group.category}
          open={open}
          onEditMultiplier={setEditingModel}
        />
      </CollapsibleContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Apply group multiplier?')}
        desc={t('Existing model overrides remain unchanged.')}
        confirmText={t('Save group multiplier')}
        isLoading={updateMutation.isPending}
        handleConfirm={() => {
          if (!parsedMultiplier.valid) return
          updateMutation.mutate({
            category: props.group.category,
            multiplier: parsedMultiplier.multiplier,
          })
        }}
      >
        <div className='bg-muted/40 grid gap-3 rounded-lg border p-3 sm:grid-cols-3'>
          <GroupMetric
            label={t('Inherited models affected')}
            value={previewMutation.data?.data?.affected_count || 0}
          />
          <GroupMetric
            label={t('Overrides preserved')}
            value={previewMutation.data?.data?.override_count || 0}
          />
          <div>
            <span className='text-muted-foreground block text-xs'>
              {t('New multiplier')}
            </span>
            <span className='font-mono text-sm'>
              {parsedMultiplier.valid
                ? `${parsedMultiplier.multiplier.toFixed(4)}x`
                : '-'}
            </span>
          </div>
        </div>
      </ConfirmDialog>

      {editingModel ? (
        <TextPricingModelDialog
          model={editingModel}
          open
          onOpenChange={(dialogOpen) => {
            if (!dialogOpen) setEditingModel(null)
          }}
        />
      ) : null}
    </Collapsible>
  )
}

function GroupMetric(props: { label: string; value: number | string }) {
  return (
    <div className='min-w-0'>
      <span className='text-muted-foreground block text-xs'>{props.label}</span>
      <span className='block text-sm font-medium tabular-nums'>
        {props.value}
      </span>
    </div>
  )
}

function GroupModelList(props: {
  category: TextPricingGroup
  open: boolean
  onEditMultiplier: (model: Model) => void
}) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const deferredSearch = useDeferredValue(search)
  const [page, setPage] = useState(1)
  const queryConfig = buildTextPricingModelQuery(
    props.open,
    props.category,
    deferredSearch,
    page,
    GROUP_PAGE_SIZE
  )

  const modelsQuery = useQuery({
    queryKey: modelsQueryKeys.textPricingModels(
      props.category,
      queryConfig?.params || {
        modal: 'text',
        text_category: props.category,
        p: page,
        page_size: GROUP_PAGE_SIZE,
      }
    ),
    queryFn: () => {
      if (!queryConfig) throw new Error('text pricing group is collapsed')
      return queryConfig.useSearch
        ? searchModels(queryConfig.params)
        : getModels(queryConfig.params)
    },
    enabled: Boolean(queryConfig),
    placeholderData: keepPreviousData,
  })

  const models = modelsQuery.data?.data?.items || []
  const total = modelsQuery.data?.data?.total || 0
  const totalPages = Math.max(1, Math.ceil(total / GROUP_PAGE_SIZE))

  const renderModelsContent = () => {
    if (modelsQuery.isLoading) {
      return (
        <div className='text-muted-foreground flex min-h-24 items-center justify-center gap-2 text-sm'>
          <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          {t('Loading models')}
        </div>
      )
    }
    if (modelsQuery.isError) {
      return (
        <div className='text-destructive min-h-20 py-6 text-center text-sm'>
          {modelsQuery.error.message || t('Unable to load models')}
        </div>
      )
    }
    if (models.length === 0) {
      return (
        <div className='text-muted-foreground min-h-20 py-6 text-center text-sm'>
          {t('No models found')}
        </div>
      )
    }
    return (
      <div className='divide-border bg-background overflow-hidden rounded-lg border'>
        {models.map((model) => (
          <TextPricingModelRow
            key={model.id}
            model={model}
            onEditMultiplier={() => props.onEditMultiplier(model)}
          />
        ))}
      </div>
    )
  }

  return (
    <div className='bg-muted/15 min-w-0 p-3'>
      <div className='mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground text-xs'>
          {t('{{count}} models', { count: total })}
        </div>
        <div className='relative w-full sm:w-64'>
          <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2' />
          <Input
            value={search}
            onChange={(event) => {
              setSearch(event.target.value)
              setPage(1)
            }}
            placeholder={t('Search models...')}
            aria-label={t('Search {{group}} models', {
              group: props.category.toUpperCase(),
            })}
            className='pl-9'
          />
        </div>
      </div>

      {renderModelsContent()}

      {total > GROUP_PAGE_SIZE ? (
        <div className='mt-3 flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-xs'>
            {t('Page {{current}} of {{total}}', {
              current: page,
              total: totalPages,
            })}
          </span>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page <= 1 || modelsQuery.isFetching}
              aria-label={t('Previous page')}
            >
              <ChevronLeft aria-hidden='true' />
            </Button>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              onClick={() =>
                setPage((current) => Math.min(totalPages, current + 1))
              }
              disabled={page >= totalPages || modelsQuery.isFetching}
              aria-label={t('Next page')}
            >
              <ChevronRight aria-hidden='true' />
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function TextPricingModelRow(props: {
  model: Model
  onEditMultiplier: () => void
}) {
  const { t } = useTranslation()
  const { setCurrentRow, setOpen } = useModels()
  const effective = props.model.effective_text_pricing
  const source = effective?.multiplier_source

  return (
    <div className='grid min-w-0 gap-3 border-b p-3 last:border-b-0 xl:grid-cols-[minmax(180px,1.1fr)_minmax(230px,1.4fr)_minmax(130px,0.7fr)_minmax(250px,1.5fr)_auto] xl:items-start'>
      <div className='min-w-0'>
        <div className='font-mono text-sm font-medium break-all'>
          {props.model.model_name}
        </div>
        <div className='mt-1 flex flex-wrap items-center gap-1.5'>
          <Badge
            variant={props.model.pricing_ready ? 'outline' : 'destructive'}
          >
            {props.model.pricing_ready ? (
              <CheckCircle2 aria-hidden='true' />
            ) : (
              <TriangleAlert aria-hidden='true' />
            )}
            {props.model.pricing_ready ? t('Billable') : t('Blocked')}
          </Badge>
          <Badge variant={props.model.status === 1 ? 'secondary' : 'outline'}>
            {props.model.status === 1 ? t('Enabled') : t('Disabled')}
          </Badge>
        </div>
        {props.model.pricing_error ? (
          <p className='text-destructive mt-1 text-xs break-words'>
            {props.model.pricing_error}
          </p>
        ) : null}
      </div>

      <OfficialPriceDetails profile={props.model.official_price_profile} />

      <div className='min-w-0'>
        <span className='text-muted-foreground block text-xs'>
          {t('Effective multiplier')}
        </span>
        <span className='mt-0.5 block font-mono text-sm font-medium'>
          {formatMultiplier(effective?.effective_multiplier)}
        </span>
        <Badge variant={source === 'model_override' ? 'secondary' : 'outline'}>
          {source === 'model_override'
            ? t('Model override')
            : t('Group inherited')}
        </Badge>
      </div>

      <EffectivePriceDetails pricing={effective} />

      <TooltipProvider>
        <div className='flex items-center gap-1 xl:justify-end'>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type='button'
                  variant='outline'
                  size='icon-sm'
                  onClick={props.onEditMultiplier}
                  aria-label={t('Set model multiplier')}
                />
              }
            >
              <SlidersHorizontal aria-hidden='true' />
            </TooltipTrigger>
            <TooltipContent>{t('Set model multiplier')}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  onClick={() => {
                    setCurrentRow(props.model)
                    setOpen('update-model')
                  }}
                  aria-label={t('Edit model metadata')}
                />
              }
            >
              <Pencil aria-hidden='true' />
            </TooltipTrigger>
            <TooltipContent>{t('Edit model metadata')}</TooltipContent>
          </Tooltip>
        </div>
      </TooltipProvider>
    </div>
  )
}

function OfficialPriceDetails(props: {
  profile?: OfficialPriceProfile | null
}) {
  const { t } = useTranslation()
  if (!props.profile) {
    return (
      <div className='text-destructive text-xs'>
        {t('Missing official profile')}
      </div>
    )
  }

  const tiers = props.profile.tiers?.length
    ? props.profile.tiers
    : [
        {
          label: t('Default'),
          dimensions: props.profile.dimensions || {},
        },
      ]

  return (
    <div className='min-w-0'>
      <div className='flex min-w-0 flex-wrap items-baseline gap-x-2'>
        <span className='truncate text-xs font-medium'>
          {props.profile.display_name}
        </span>
        <span className='text-muted-foreground font-mono text-[11px]'>
          {props.profile.version}
        </span>
      </div>
      <div className='mt-1 space-y-1'>
        {tiers.map((tier) => (
          <OfficialTierDetails key={tier.label} tier={tier} />
        ))}
      </div>
    </div>
  )
}

function OfficialTierDetails(props: {
  tier: Pick<OfficialPriceTier, 'label' | 'dimensions'>
}) {
  const { t } = useTranslation()
  const dimensions = getOfficialDimensionsSummary(props.tier.dimensions)
  return (
    <div className='text-xs'>
      <span className='text-muted-foreground'>{props.tier.label}: </span>
      <span className='inline-flex flex-wrap gap-x-2 gap-y-0.5 font-mono'>
        {dimensions.map((dimension) => (
          <span key={dimension.key}>
            {getDimensionLabel(dimension.key, t)} $
            {formatPrice(dimension.value)}
          </span>
        ))}
      </span>
    </div>
  )
}

function EffectivePriceDetails(props: {
  pricing?: EffectiveTextPricing | null
}) {
  const { t } = useTranslation()
  if (!props.pricing) {
    return <div className='text-muted-foreground text-xs'>-</div>
  }

  const tiers = props.pricing.tiers?.length
    ? props.pricing.tiers
    : [baseEffectiveTier(props.pricing, t('Default'))]

  return (
    <div className='min-w-0'>
      <span className='text-muted-foreground block text-xs'>
        {t('Credits / quota per 1M tokens')}
      </span>
      <div className='mt-1 space-y-1'>
        {tiers.map((tier) => (
          <EffectiveTierDetails key={tier.label} tier={tier} />
        ))}
      </div>
    </div>
  )
}

function EffectiveTierDetails(props: { tier: EffectiveTextPricingTier }) {
  const { t } = useTranslation()
  const summary = getEffectiveTextPricingSummary(props.tier)
  return (
    <div className='text-xs'>
      <span className='text-muted-foreground'>{props.tier.label}: </span>
      <span className='inline-flex flex-wrap gap-x-2 gap-y-0.5 font-mono'>
        {summary.map((dimension) => (
          <span key={dimension.key}>
            {getDimensionLabel(dimension.key, t)} {dimension.credits.toFixed(4)}{' '}
            C / {dimension.quota.toLocaleString()}
          </span>
        ))}
      </span>
    </div>
  )
}

function baseEffectiveTier(
  pricing: EffectiveTextPricing,
  label: string
): EffectiveTextPricingTier {
  return {
    label,
    input_quota_per_million: pricing.input_quota_per_million || 0,
    output_quota_per_million: pricing.output_quota_per_million || 0,
    cached_input_quota_per_million: pricing.cached_input_quota_per_million,
    cache_write_quota_per_million: pricing.cache_write_quota_per_million,
    cache_write_5m_quota_per_million: pricing.cache_write_5m_quota_per_million,
    cache_write_1h_quota_per_million: pricing.cache_write_1h_quota_per_million,
  }
}

function getDimensionLabel(
  key: keyof OfficialPriceDimensions | string,
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

function formatPrice(value: number): string {
  return Number.isInteger(value)
    ? String(value)
    : String(Number(value.toFixed(6)))
}

function formatMultiplier(value?: number): string {
  return Number.isFinite(value) ? `${Number(value).toFixed(4)}x` : '-'
}
