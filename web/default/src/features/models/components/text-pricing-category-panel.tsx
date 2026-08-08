import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Calculator, Loader2, ShieldCheck } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import {
  getTextPricingConfig,
  previewTextPricingCategory,
  updateTextPricingCategory,
} from '../api'
import {
  getTextCategoryMultiplier,
  modelsQueryKeys,
  normalizeOfficialPriceProfiles,
  normalizeTextPricingCategories,
} from '../lib'
import type { TextPricingImpact, TextPricingPreviewSummary } from '../types'

type TextPricingCategoryPanelProps = {
  category?: string
}

export function TextPricingCategoryPanel(props: TextPricingCategoryPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [multiplierInput, setMultiplierInput] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)

  const configQuery = useQuery({
    queryKey: modelsQueryKeys.textPricing(),
    queryFn: getTextPricingConfig,
  })

  const categories = useMemo(
    () => normalizeTextPricingCategories(configQuery.data?.data?.categories),
    [configQuery.data?.data?.categories]
  )
  const profiles = useMemo(
    () => normalizeOfficialPriceProfiles(configQuery.data?.data?.profiles),
    [configQuery.data?.data?.profiles]
  )
  const currentMultiplier = getTextCategoryMultiplier(
    categories,
    props.category
  )
  const profileCount = profiles.filter(
    (profile) => profile.category === props.category
  ).length

  useEffect(() => {
    setMultiplierInput(
      currentMultiplier === undefined ? '' : String(currentMultiplier)
    )
    setConfirmOpen(false)
  }, [currentMultiplier, props.category])

  const parsedMultiplier = Number(multiplierInput)
  const multiplierIsValid =
    multiplierInput.trim() !== '' &&
    Number.isFinite(parsedMultiplier) &&
    parsedMultiplier > 0 &&
    parsedMultiplier <= 1 &&
    /^\d+(?:\.\d{1,4})?$/.test(multiplierInput)
  const multiplierChanged =
    currentMultiplier === undefined || parsedMultiplier !== currentMultiplier

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

  if (configQuery.isLoading) {
    return (
      <div className='border-border flex min-h-16 items-center gap-2 border-y px-3 text-sm'>
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading text pricing configuration')}
      </div>
    )
  }

  if (!configQuery.data?.success || !configQuery.data.data) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Text pricing configuration unavailable')}</AlertTitle>
        <AlertDescription>
          {configQuery.data?.message ||
            configQuery.error?.message ||
            t('Check the pricing service and try again.')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <>
      <section className='border-border bg-muted/25 grid gap-3 border-y px-3 py-3 lg:grid-cols-[minmax(0,1fr)_minmax(280px,420px)]'>
        <div className='flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 text-sm'>
          <span className='font-medium'>{t('Text pricing')}</span>
          <Badge variant='outline'>
            {t('Mode')}: {configQuery.data.data.mode}
          </Badge>
          <Badge variant='outline'>
            {t('Catalog')}: {configQuery.data.data.catalog_version}
          </Badge>
          {props.category ? (
            <Badge variant='secondary'>
              {profileCount} {t('official profiles')}
            </Badge>
          ) : null}
          <span className='text-muted-foreground basis-full text-xs'>
            {t(
              'Official prices are read-only. The category multiplier applies to every model in the selected group.'
            )}
          </span>
        </div>

        {props.category ? (
          <div className='flex min-w-0 items-end gap-2'>
            <div className='min-w-0 flex-1 space-y-1'>
              <Label htmlFor='text-category-multiplier'>
                {t('{{category}} multiplier', {
                  category: props.category.toUpperCase(),
                })}
              </Label>
              <Input
                id='text-category-multiplier'
                inputMode='decimal'
                value={multiplierInput}
                onChange={(event) => setMultiplierInput(event.target.value)}
                aria-invalid={multiplierInput.length > 0 && !multiplierIsValid}
                placeholder='0.0500'
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Enter a value greater than 0 and at most 1, up to 4 decimals.'
                )}
              </p>
            </div>
            <Button
              type='button'
              variant='outline'
              disabled={
                !multiplierIsValid ||
                !multiplierChanged ||
                previewMutation.isPending
              }
              onClick={() =>
                previewMutation.mutate({
                  category: props.category as string,
                  multiplier: parsedMultiplier,
                })
              }
            >
              {previewMutation.isPending ? (
                <Loader2 className='size-4 animate-spin' aria-hidden='true' />
              ) : (
                <Calculator className='size-4' aria-hidden='true' />
              )}
              {t('Preview impact')}
            </Button>
          </div>
        ) : null}
      </section>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Apply category multiplier?')}
        desc={t(
          'The change is atomic and takes effect immediately for every model in this category.'
        )}
        confirmText={t('Apply multiplier')}
        isLoading={updateMutation.isPending}
        handleConfirm={() => {
          if (!props.category || !multiplierIsValid) return
          updateMutation.mutate({
            category: props.category,
            multiplier: parsedMultiplier,
          })
        }}
      >
        <div className='space-y-3'>
          <Alert>
            <ShieldCheck aria-hidden='true' />
            <AlertTitle>
              {t('{{count}} models affected', {
                count: previewMutation.data?.data?.affected_count || 0,
              })}
            </AlertTitle>
            <AlertDescription>
              {t('Category')}: {props.category?.toUpperCase()} ·{' '}
              {t('Multiplier')}: {parsedMultiplier}
            </AlertDescription>
          </Alert>
          <div className='grid gap-3 sm:grid-cols-2'>
            <PricingPreviewSummary
              title={t('Before')}
              summary={previewMutation.data?.data?.before}
            />
            <PricingPreviewSummary
              title={t('After')}
              summary={previewMutation.data?.data?.after}
            />
          </div>
        </div>
      </ConfirmDialog>
    </>
  )
}

function PricingPreviewSummary(props: {
  title: string
  summary?: TextPricingPreviewSummary
}) {
  return (
    <div className='border-border min-w-0 rounded-lg border p-3'>
      <div className='mb-2 flex items-center justify-between gap-2 text-sm'>
        <span className='font-medium'>{props.title}</span>
        <span className='font-mono'>{props.summary?.multiplier ?? '-'}x</span>
      </div>
      {!props.summary?.models.length ? (
        <span className='text-muted-foreground text-xs'>-</span>
      ) : (
        <div className='space-y-2'>
          {props.summary.models.slice(0, 5).map((model) => (
            <PricingImpactRow key={model.id} model={model} />
          ))}
          {props.summary.models.length > 5 ? (
            <p className='text-muted-foreground text-xs'>
              +{props.summary.models.length - 5}
            </p>
          ) : null}
        </div>
      )}
    </div>
  )
}

function PricingImpactRow(props: { model: TextPricingImpact }) {
  const { t } = useTranslation()

  return (
    <div className='border-border/70 min-w-0 rounded-md border px-2 py-1.5 text-xs'>
      <div className='truncate font-medium'>{props.model.model_name}</div>
      {props.model.pricing_ready ? (
        <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-3 font-mono'>
          <span>
            {t('Input')} {props.model.input_quota_per_million}
          </span>
          <span>
            {t('Output')} {props.model.output_quota_per_million}
          </span>
        </div>
      ) : (
        <div className='text-destructive mt-1 truncate'>
          {props.model.pricing_error || t('Unavailable')}
        </div>
      )}
    </div>
  )
}
