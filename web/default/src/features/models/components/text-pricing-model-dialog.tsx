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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, Loader2, RotateCcw, SlidersHorizontal } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

import { previewTextPricingModel, updateTextPricingModel } from '../api'
import { modelsQueryKeys } from '../lib'
import { parseTextPricingMultiplierInput } from '../lib/model-pricing'
import type { Model, TextPricingModelPreview } from '../types'

type TextPricingModelDialogProps = {
  model: Model
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TextPricingModelDialog(props: TextPricingModelDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<'inherit' | 'custom'>('inherit')
  const [multiplierInput, setMultiplierInput] = useState('')
  const [preview, setPreview] = useState<TextPricingModelPreview | null>(null)

  useEffect(() => {
    if (!props.open) return
    setMode(
      props.model.text_multiplier_override === undefined ? 'inherit' : 'custom'
    )
    setMultiplierInput(
      props.model.text_multiplier_override === undefined
        ? ''
        : String(props.model.text_multiplier_override)
    )
    setPreview(null)
  }, [props.open, props.model])

  const parsed =
    mode === 'custom'
      ? parseTextPricingMultiplierInput(multiplierInput)
      : { valid: true as const, multiplier: 0 }
  const multiplierErrorId = `model-multiplier-error-${props.model.id}`
  let targetMultiplier: number | null = null
  if (mode === 'custom' && parsed.valid) {
    targetMultiplier = parsed.multiplier
  }
  const currentMultiplier = props.model.text_multiplier_override ?? null
  const unchanged =
    targetMultiplier === null
      ? currentMultiplier === null
      : currentMultiplier === targetMultiplier
  const canPreview =
    !unchanged && (mode === 'inherit' || parsed.valid) && !preview

  const previewMutation = useMutation({
    mutationFn: previewTextPricingModel,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Unable to preview model pricing'))
        return
      }
      setPreview(response.data)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Unable to preview model pricing'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: updateTextPricingModel,
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Unable to update model pricing'))
        return
      }
      toast.success(
        mode === 'inherit'
          ? t('Model now inherits its group multiplier')
          : t('Model multiplier updated')
      )
      props.onOpenChange(false)
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: modelsQueryKeys.textPricing(),
        }),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({
          queryKey: modelsQueryKeys.detail(props.model.id),
        }),
      ])
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Unable to update model pricing'))
    },
  })

  const handleModeChange = (value: string) => {
    if (value !== 'inherit' && value !== 'custom') return
    setMode(value)
    setPreview(null)
    if (value === 'inherit') setMultiplierInput('')
  }

  const handlePreview = () => {
    if (!canPreview) return
    previewMutation.mutate({
      model_id: props.model.id,
      multiplier: targetMultiplier,
    })
  }

  const handleApply = () => {
    if (!preview || updateMutation.isPending) return
    updateMutation.mutate({
      model_id: props.model.id,
      multiplier: targetMultiplier,
    })
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[min(760px,calc(100vh-2rem))] overflow-y-auto sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 pr-6'>
            <SlidersHorizontal className='size-4' aria-hidden='true' />
            {t('Set model multiplier')}
          </DialogTitle>
          <DialogDescription className='font-mono break-all'>
            {props.model.model_name}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <RadioGroup
            value={mode}
            onValueChange={handleModeChange}
            aria-label={t('Model multiplier mode')}
            className='gap-2'
          >
            <label className='border-border hover:bg-muted/40 flex cursor-pointer items-start gap-3 rounded-lg border p-3'>
              <RadioGroupItem
                value='inherit'
                aria-label={t('Inherit group multiplier')}
              />
              <span className='min-w-0'>
                <span className='block font-medium'>
                  {t('Inherit group multiplier')}
                </span>
                <span className='text-muted-foreground block text-xs'>
                  {t('Use the selected vendor group multiplier.')}
                </span>
              </span>
            </label>
            <label className='border-border hover:bg-muted/40 flex cursor-pointer items-start gap-3 rounded-lg border p-3'>
              <RadioGroupItem
                value='custom'
                aria-label={t('Custom multiplier')}
              />
              <span className='min-w-0 flex-1 space-y-2'>
                <span className='block font-medium'>
                  {t('Custom multiplier')}
                </span>
                {mode === 'custom' ? (
                  <span className='block space-y-1'>
                    <Label htmlFor={`model-multiplier-${props.model.id}`}>
                      {t('Multiplier')}
                    </Label>
                    <Input
                      id={`model-multiplier-${props.model.id}`}
                      inputMode='decimal'
                      value={multiplierInput}
                      onChange={(event) => {
                        setMultiplierInput(event.target.value)
                        setPreview(null)
                      }}
                      aria-invalid={!parsed.valid}
                      aria-describedby={multiplierErrorId}
                      placeholder='0.1000'
                    />
                    <span
                      id={multiplierErrorId}
                      role={parsed.valid ? undefined : 'alert'}
                      className={`block text-xs ${parsed.valid ? 'text-muted-foreground' : 'text-destructive'}`}
                    >
                      {t('Greater than 0, at most 1, up to 4 decimals.')}
                    </span>
                  </span>
                ) : null}
              </span>
            </label>
          </RadioGroup>

          {preview ? (
            <ModelPricingPreview preview={preview} />
          ) : (
            <Alert>
              <RotateCcw aria-hidden='true' />
              <AlertTitle>{t('Preview required')}</AlertTitle>
              <AlertDescription>
                {t('Review the pricing change before applying it.')}
              </AlertDescription>
            </Alert>
          )}
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={previewMutation.isPending || updateMutation.isPending}
          >
            {t('Cancel')}
          </Button>
          {preview ? (
            <Button
              type='button'
              onClick={handleApply}
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? (
                <Loader2 className='size-4 animate-spin' aria-hidden='true' />
              ) : (
                <Check className='size-4' aria-hidden='true' />
              )}
              {t('Apply change')}
            </Button>
          ) : (
            <Button
              type='button'
              onClick={handlePreview}
              disabled={!canPreview || previewMutation.isPending}
            >
              {previewMutation.isPending ? (
                <Loader2 className='size-4 animate-spin' aria-hidden='true' />
              ) : (
                <SlidersHorizontal className='size-4' aria-hidden='true' />
              )}
              {t('Preview change')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ModelPricingPreview(props: { preview: TextPricingModelPreview }) {
  const { t } = useTranslation()
  return (
    <Alert>
      <Check aria-hidden='true' />
      <AlertTitle>{t('Pricing preview')}</AlertTitle>
      <AlertDescription>
        <div className='mt-2 grid gap-2 sm:grid-cols-2'>
          <PreviewValue title={t('Before')} impact={props.preview.before} />
          <PreviewValue title={t('After')} impact={props.preview.after} />
        </div>
      </AlertDescription>
    </Alert>
  )
}

function PreviewValue(props: {
  title: string
  impact: TextPricingModelPreview['before']
}) {
  const { t } = useTranslation()
  return (
    <div className='border-border/70 min-w-0 rounded-md border p-2'>
      <div className='font-medium'>{props.title}</div>
      {props.impact.pricing_ready ? (
        <div className='mt-1 space-y-0.5 font-mono text-xs'>
          <div>
            {t('Multiplier')}:{' '}
            {formatMultiplier(props.impact.effective_multiplier)}
          </div>
          <div>
            {t('Input')}:{' '}
            {props.impact.input_quota_per_million.toLocaleString()}
          </div>
          <div>
            {t('Output')}:{' '}
            {props.impact.output_quota_per_million.toLocaleString()}
          </div>
        </div>
      ) : (
        <div className='text-destructive mt-1 text-xs'>
          {props.impact.pricing_error || t('Unavailable')}
        </div>
      )}
    </div>
  )
}

function formatMultiplier(value?: number): string {
  return Number.isFinite(value) ? `${Number(value).toFixed(4)}x` : '-'
}
