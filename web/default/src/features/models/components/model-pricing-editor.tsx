import type { TFunction } from 'i18next'
import { AlertTriangle, CirclePlus, ExternalLink, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonEditor } from '@/components/json-editor'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  cnyInputToCredits,
  creditsInputToCNY,
  formatCNYInput,
  getEffectiveTextPricingSummary,
  getOfficialDimensionsSummary,
  getProfilesForCategory,
  IMAGE_RESOLUTION_OPTIONS,
  type ImageSpecRow,
  type ModelPricingMode,
  VIDEO_MODE_OPTIONS,
  VIDEO_RATIO_OPTIONS,
  VIDEO_RESOLUTION_OPTIONS,
  type VideoMatrixRow,
} from '../lib'
import type {
  Model,
  OfficialPriceProfile,
  TextPricingCategoryConfig,
} from '../types'

export type MediaPricingDraft = {
  imageRows: ImageSpecRow[]
  videoRows: VideoMatrixRow[]
  imageDefaultCNY: string
  videoDefaultCNY: string
}

type ModelPricingEditorProps = {
  modal: string
  textCategory: string
  officialPriceKey: string
  pricingMode: string
  pricingConfig: string
  categories: TextPricingCategoryConfig[]
  profiles: OfficialPriceProfile[]
  currentModel?: Model | null
  mediaDraft: MediaPricingDraft
  onTextCategoryChange: (category: string) => void
  onOfficialPriceKeyChange: (key: string) => void
  onPricingModeChange: (mode: ModelPricingMode) => void
  onPricingConfigChange: (config: string) => void
  onMediaDraftChange: (draft: MediaPricingDraft) => void
}

export function ModelPricingEditor(props: ModelPricingEditorProps) {
  if (props.modal === 'text') return <TextPricingEditor {...props} />
  if (props.modal === 'image') return <ImagePricingEditor {...props} />
  if (props.modal === 'video') return <VideoPricingEditor {...props} />
  return <AudioPricingEditor {...props} />
}

function TextPricingEditor(props: ModelPricingEditorProps) {
  const { t } = useTranslation()
  const profiles = useMemo(
    () => getProfilesForCategory(props.profiles, props.textCategory),
    [props.profiles, props.textCategory]
  )
  const selectedProfile = profiles.find(
    (profile) => profile.key === props.officialPriceKey
  )
  const categoryConfig = props.categories.find(
    (entry) => entry.category === props.textCategory
  )
  const currentPricingMatches =
    props.currentModel?.text_category === props.textCategory &&
    props.currentModel?.official_price_key === props.officialPriceKey
  const effectiveSummary = currentPricingMatches
    ? getEffectiveTextPricingSummary(props.currentModel?.effective_text_pricing)
    : []

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label>{t('Text category')}</Label>
          <Select
            value={props.textCategory}
            onValueChange={(value) => {
              if (!value) return
              props.onTextCategoryChange(value)
              props.onOfficialPriceKeyChange('')
            }}
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Select text category')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {['gpt', 'claude', 'gemini', 'grok', 'unclassified'].map(
                  (category) => (
                    <SelectItem key={category} value={category}>
                      {category === 'unclassified'
                        ? t('Unclassified')
                        : category.toUpperCase()}
                    </SelectItem>
                  )
                )}
              </SelectGroup>
            </SelectContent>
          </Select>
          <p className='text-muted-foreground text-xs'>
            {categoryConfig?.multiplier === undefined
              ? t('No category multiplier configured')
              : t('Current category multiplier: {{multiplier}}x', {
                  multiplier: categoryConfig.multiplier.toFixed(4),
                })}
          </p>
        </div>

        <div className='space-y-2'>
          <Label>{t('Official price profile')}</Label>
          <Select
            value={props.officialPriceKey || undefined}
            onValueChange={(value) =>
              props.onOfficialPriceKeyChange(value || '')
            }
            disabled={props.textCategory === 'unclassified'}
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Select official price profile')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {profiles.map((profile) => (
                  <SelectItem key={profile.key} value={profile.key}>
                    {profile.display_name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <p className='text-muted-foreground text-xs'>
            {t('Official prices are read-only and versioned by the server.')}
          </p>
        </div>
      </div>

      {props.textCategory === 'unclassified' ? (
        <Alert variant='destructive'>
          <AlertTriangle aria-hidden='true' />
          <AlertTitle>
            {t('Unclassified text models cannot be enabled')}
          </AlertTitle>
          <AlertDescription>
            {t('Choose a supported category and official price profile first.')}
          </AlertDescription>
        </Alert>
      ) : null}

      {selectedProfile ? (
        <OfficialProfileDetails profile={selectedProfile} />
      ) : null}

      {currentPricingMatches && props.currentModel ? (
        <div className='border-border rounded-lg border p-3'>
          <div className='mb-2 flex flex-wrap items-center gap-2'>
            <span className='text-sm font-medium'>
              {t('Effective pricing')}
            </span>
            <Badge
              variant={
                props.currentModel.pricing_ready ? 'secondary' : 'destructive'
              }
            >
              {props.currentModel.pricing_ready ? t('Ready') : t('Blocked')}
            </Badge>
          </div>
          {props.currentModel.pricing_error ? (
            <p className='text-destructive mb-2 text-xs'>
              {props.currentModel.pricing_error}
            </p>
          ) : null}
          <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
            {effectiveSummary.map((item) => (
              <div
                key={item.key}
                className='bg-muted/50 rounded-md px-2.5 py-2 text-xs'
              >
                <div className='text-muted-foreground'>{item.key}</div>
                <div className='font-mono'>
                  {item.credits.toFixed(6)} Credits
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}

function OfficialProfileDetails(props: { profile: OfficialPriceProfile }) {
  const { t } = useTranslation()
  const dimensions = getOfficialDimensionsSummary(props.profile.dimensions)

  return (
    <div className='border-border rounded-lg border p-3'>
      <div className='flex flex-wrap items-start justify-between gap-2'>
        <div>
          <div className='font-medium'>{props.profile.display_name}</div>
          <div className='text-muted-foreground font-mono text-xs'>
            {props.profile.key}
          </div>
        </div>
        {props.profile.source_url ? (
          <Button
            variant='outline'
            size='sm'
            render={
              <a
                href={props.profile.source_url}
                target='_blank'
                rel='noreferrer'
              />
            }
          >
            {t('Official source')}
            <ExternalLink aria-hidden='true' />
          </Button>
        ) : null}
      </div>

      <div className='mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
        {dimensions.map((dimension) => (
          <div
            key={dimension.key}
            className='bg-muted/50 rounded-md px-2.5 py-2 text-xs'
          >
            <div className='text-muted-foreground'>{dimension.key}</div>
            <div className='font-mono'>
              {props.profile.currency || '$'}
              {dimension.value} / {props.profile.unit || t('1M tokens')}
            </div>
          </div>
        ))}
      </div>

      {props.profile.tiers?.length ? (
        <div className='mt-3 space-y-2'>
          <div className='text-sm font-medium'>{t('Context tiers')}</div>
          {props.profile.tiers.map((tier) => (
            <div
              key={`${tier.label}-${tier.min_prompt_tokens || 0}`}
              className='border-border/70 rounded-md border px-2.5 py-2 text-xs'
            >
              <div className='font-medium'>{tier.label}</div>
              <div className='text-muted-foreground mt-1'>
                {formatTierRange(
                  tier.min_prompt_tokens,
                  tier.max_prompt_tokens,
                  t
                )}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}

function ImagePricingEditor(props: ModelPricingEditorProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-4'>
      <MediaPricingModeSelect
        value={props.pricingMode}
        specMode='image_spec'
        specLabel={t('Image specification pricing')}
        onChange={props.onPricingModeChange}
      />

      {props.pricingMode === 'image_spec' ? (
        <>
          <div className='space-y-3'>
            {props.mediaDraft.imageRows.map((row) => (
              <div
                key={row.id}
                className='border-border grid gap-3 rounded-lg border p-3 lg:grid-cols-[150px_minmax(0,1fr)_auto]'
              >
                <div className='space-y-2'>
                  <Label>{t('Resolution')}</Label>
                  <Select
                    value={row.resolution}
                    onValueChange={(value) => {
                      if (!value) return
                      props.onMediaDraftChange({
                        ...props.mediaDraft,
                        imageRows: props.mediaDraft.imageRows.map((entry) =>
                          entry.id === row.id
                            ? { ...entry, resolution: value }
                            : entry
                        ),
                      })
                    }}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {IMAGE_RESOLUTION_OPTIONS.map((resolution) => (
                          <SelectItem key={resolution} value={resolution}>
                            {resolution.toUpperCase()}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>

                <DualPriceInput
                  cnyValue={row.cnyPerImage}
                  onCNYChange={(value) =>
                    props.onMediaDraftChange({
                      ...props.mediaDraft,
                      imageRows: props.mediaDraft.imageRows.map((entry) =>
                        entry.id === row.id
                          ? { ...entry, cnyPerImage: value }
                          : entry
                      ),
                    })
                  }
                />

                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  className='text-destructive self-end'
                  disabled={props.mediaDraft.imageRows.length === 1}
                  onClick={() =>
                    props.onMediaDraftChange({
                      ...props.mediaDraft,
                      imageRows: props.mediaDraft.imageRows.filter(
                        (entry) => entry.id !== row.id
                      ),
                    })
                  }
                  aria-label={t('Remove price tier')}
                >
                  <Trash2 aria-hidden='true' />
                </Button>
              </div>
            ))}
          </div>

          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() =>
              props.onMediaDraftChange({
                ...props.mediaDraft,
                imageRows: [
                  ...props.mediaDraft.imageRows,
                  {
                    id: nextRowId(props.mediaDraft.imageRows),
                    resolution: '1k',
                    cnyPerImage: '',
                  },
                ],
              })
            }
          >
            <CirclePlus aria-hidden='true' />
            {t('Add resolution price')}
          </Button>

          <div className='border-border rounded-lg border p-3'>
            <div className='mb-3'>
              <div className='text-sm font-medium'>{t('Default price')}</div>
              <div className='text-muted-foreground text-xs'>
                {t('Used when no resolution-specific price matches.')}
              </div>
            </div>
            <DualPriceInput
              cnyValue={props.mediaDraft.imageDefaultCNY}
              onCNYChange={(value) =>
                props.onMediaDraftChange({
                  ...props.mediaDraft,
                  imageDefaultCNY: value,
                })
              }
            />
          </div>
        </>
      ) : (
        <PricingModeNotice mode={props.pricingMode} />
      )}
    </div>
  )
}

function VideoPricingEditor(props: ModelPricingEditorProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-4'>
      <MediaPricingModeSelect
        value={props.pricingMode}
        specMode='video_matrix'
        specLabel={t('Video matrix pricing')}
        onChange={props.onPricingModeChange}
      />

      {props.pricingMode === 'video_matrix' ? (
        <>
          <div className='space-y-3'>
            {props.mediaDraft.videoRows.map((row) => (
              <div
                key={row.id}
                className='border-border space-y-3 rounded-lg border p-3'
              >
                <div className='grid gap-3 sm:grid-cols-3'>
                  <CompactSelect
                    label={t('Resolution')}
                    value={row.resolution}
                    options={VIDEO_RESOLUTION_OPTIONS}
                    onChange={(value) =>
                      updateVideoRow(props, row.id, { resolution: value })
                    }
                  />
                  <CompactSelect
                    label={t('Aspect ratio')}
                    value={row.ratio}
                    options={VIDEO_RATIO_OPTIONS}
                    onChange={(value) =>
                      updateVideoRow(props, row.id, { ratio: value })
                    }
                  />
                  <CompactSelect
                    label={t('Mode')}
                    value={row.mode}
                    options={VIDEO_MODE_OPTIONS}
                    onChange={(value) =>
                      updateVideoRow(props, row.id, { mode: value })
                    }
                  />
                </div>

                <div className='grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px_auto]'>
                  <DualPriceInput
                    cnyValue={row.cnyPerSecond}
                    disabled={!row.supported}
                    onCNYChange={(value) =>
                      updateVideoRow(props, row.id, { cnyPerSecond: value })
                    }
                  />
                  <div className='border-border flex items-center justify-between rounded-lg border px-3 py-2'>
                    <Label htmlFor={`video-supported-${row.id}`}>
                      {t('Supported')}
                    </Label>
                    <Switch
                      id={`video-supported-${row.id}`}
                      checked={row.supported}
                      onCheckedChange={(value) =>
                        updateVideoRow(props, row.id, { supported: value })
                      }
                    />
                  </div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    className='text-destructive self-end'
                    disabled={props.mediaDraft.videoRows.length === 1}
                    onClick={() =>
                      props.onMediaDraftChange({
                        ...props.mediaDraft,
                        videoRows: props.mediaDraft.videoRows.filter(
                          (entry) => entry.id !== row.id
                        ),
                      })
                    }
                    aria-label={t('Remove matrix cell')}
                  >
                    <Trash2 aria-hidden='true' />
                  </Button>
                </div>
              </div>
            ))}
          </div>

          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() =>
              props.onMediaDraftChange({
                ...props.mediaDraft,
                videoRows: [
                  ...props.mediaDraft.videoRows,
                  {
                    id: nextRowId(props.mediaDraft.videoRows),
                    resolution: '720p',
                    ratio: '16:9',
                    mode: 'no_video_input',
                    supported: true,
                    cnyPerSecond: '',
                  },
                ],
              })
            }
          >
            <CirclePlus aria-hidden='true' />
            {t('Add matrix cell')}
          </Button>

          <div className='border-border rounded-lg border p-3'>
            <div className='mb-3'>
              <div className='text-sm font-medium'>{t('Default price')}</div>
              <div className='text-muted-foreground text-xs'>
                {t('Used when no matrix cell matches the request.')}
              </div>
            </div>
            <DualPriceInput
              cnyValue={props.mediaDraft.videoDefaultCNY}
              onCNYChange={(value) =>
                props.onMediaDraftChange({
                  ...props.mediaDraft,
                  videoDefaultCNY: value,
                })
              }
            />
          </div>
        </>
      ) : (
        <PricingModeNotice mode={props.pricingMode} />
      )}
    </div>
  )
}

function AudioPricingEditor(props: ModelPricingEditorProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-4'>
      <MediaPricingModeSelect
        value={props.pricingMode}
        specMode='ratio'
        specLabel={t('Compatibility ratio pricing')}
        onChange={props.onPricingModeChange}
      />
      {props.pricingMode === 'ratio' ? (
        <div className='space-y-2'>
          <Label>{t('Compatibility pricing configuration')}</Label>
          <JsonEditor
            value={props.pricingConfig}
            onChange={props.onPricingConfigChange}
            keyPlaceholder='field'
            valuePlaceholder='value'
            keyLabel={t('Field')}
            valueLabel={t('Value')}
            valueType='any'
            emptyMessage={t('No compatibility pricing configured.')}
          />
        </div>
      ) : (
        <PricingModeNotice mode={props.pricingMode} />
      )}
    </div>
  )
}

function MediaPricingModeSelect(props: {
  value: string
  specMode: ModelPricingMode
  specLabel: string
  onChange: (mode: ModelPricingMode) => void
}) {
  const { t } = useTranslation()

  return (
    <div className='space-y-2'>
      <Label>{t('Pricing mode')}</Label>
      <Select
        value={props.value}
        onValueChange={(value) => {
          if (value) props.onChange(value as ModelPricingMode)
        }}
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value={props.specMode}>{props.specLabel}</SelectItem>
            <SelectItem value='free'>{t('Free')}</SelectItem>
            <SelectItem value='inherit'>{t('Legacy fallback')}</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  )
}

function PricingModeNotice(props: { mode: string }) {
  const { t } = useTranslation()

  return (
    <Alert>
      <AlertTitle>
        {props.mode === 'free' ? t('Free model') : t('Legacy fallback')}
      </AlertTitle>
      <AlertDescription>
        {props.mode === 'free'
          ? t('Requests do not consume quota in free mode.')
          : t(
              'The legacy pricing resolver remains authoritative for this model.'
            )}
      </AlertDescription>
    </Alert>
  )
}

function DualPriceInput(props: {
  cnyValue: string
  disabled?: boolean
  onCNYChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const [creditsValue, setCreditsValue] = useState(
    cnyInputToCredits(props.cnyValue)
  )
  const [editingCredits, setEditingCredits] = useState(false)

  useEffect(() => {
    if (!editingCredits) setCreditsValue(cnyInputToCredits(props.cnyValue))
  }, [editingCredits, props.cnyValue])

  return (
    <div className='grid min-w-0 gap-3 sm:grid-cols-2'>
      <div className='space-y-2'>
        <Label>{t('Price (CNY)')}</Label>
        <Input
          inputMode='decimal'
          value={props.cnyValue}
          disabled={props.disabled}
          placeholder='0.0000'
          onChange={(event) => {
            if (!isDecimalInput(event.target.value, 4)) return
            props.onCNYChange(event.target.value)
          }}
          onBlur={() => {
            if (!props.cnyValue) return
            props.onCNYChange(formatCNYInput(Number(props.cnyValue)))
          }}
        />
      </div>
      <div className='space-y-2'>
        <Label>{t('Price (Credits)')}</Label>
        <Input
          inputMode='decimal'
          value={creditsValue}
          disabled={props.disabled}
          placeholder='0.000000'
          onFocus={() => setEditingCredits(true)}
          onChange={(event) => {
            if (!isDecimalInput(event.target.value, 6)) return
            setCreditsValue(event.target.value)
            props.onCNYChange(creditsInputToCNY(event.target.value))
          }}
          onBlur={() => {
            setEditingCredits(false)
            setCreditsValue(cnyInputToCredits(props.cnyValue))
          }}
        />
      </div>
    </div>
  )
}

function CompactSelect(props: {
  label: string
  value: string
  options: string[]
  onChange: (value: string) => void
}) {
  return (
    <div className='space-y-2'>
      <Label>{props.label}</Label>
      <Select
        value={props.value}
        onValueChange={(value) => {
          if (value) props.onChange(value)
        }}
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {props.options.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  )
}

function updateVideoRow(
  props: ModelPricingEditorProps,
  id: number,
  patch: Partial<VideoMatrixRow>
) {
  props.onMediaDraftChange({
    ...props.mediaDraft,
    videoRows: props.mediaDraft.videoRows.map((row) =>
      row.id === id ? { ...row, ...patch } : row
    ),
  })
}

function nextRowId(rows: Array<{ id: number }>): number {
  return Math.max(0, ...rows.map((row) => row.id)) + 1
}

function isDecimalInput(value: string, precision: number): boolean {
  if (value === '') return true
  return new RegExp(`^\\d*(?:\\.\\d{0,${precision}})?$`).test(value)
}

function formatTierRange(
  min: number | undefined,
  max: number | undefined,
  t: TFunction
): string {
  const hasMin = typeof min === 'number' && Number.isFinite(min)
  const hasMax = typeof max === 'number' && Number.isFinite(max)

  if (hasMin && hasMax) {
    return t('{{min}}–{{max}} tokens', {
      min: min.toLocaleString(),
      max: max.toLocaleString(),
    })
  }
  if (hasMin) {
    return t('≥ {{min}} tokens', { min: min.toLocaleString() })
  }
  if (hasMax) {
    return t('≤ {{max}} tokens', { max: max.toLocaleString() })
  }
  return t('All context lengths')
}
