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
import { useCallback, useEffect, useMemo, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  ClipboardPaste,
  ExternalLink,
  Loader2,
  Plus,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { deferEffect } from '@/lib/defer-effect'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { GroupBadge } from '@/components/group-badge'
import { JsonEditor } from '@/components/json-editor'
import { StatusBadge } from '@/components/status-badge'
import { TagInput } from '@/components/tag-input'
import { getChannels } from '@/features/channels/api'
import type { Channel } from '@/features/channels/types'
import {
  getOptionValue,
  useSystemOptions,
} from '@/features/system-settings/hooks/use-system-options'
import {
  createModel,
  getModel,
  getVendors,
  updateModel,
  updateModelAccess,
} from '../../api'
import { ENDPOINT_TEMPLATES, getNameRuleOptions } from '../../constants'
import {
  formatNumber,
  imageRowsFromConfig,
  imageRowsToResolutions,
  MODEL_MODAL_VALUES,
  MODEL_PRICING_MODE_VALUES,
  parseModelPricingConfig,
  parseModelTags,
  parseNonNegativeNumber,
  quotaPreview,
  stringifyModelPricingConfig,
  videoRowsFromConfig,
  videoRowsToPrices,
  IMAGE_RESOLUTION_OPTIONS,
  VIDEO_MODE_OPTIONS,
  VIDEO_RATIO_OPTIONS,
  VIDEO_RESOLUTION_OPTIONS,
  modelsQueryKeys,
  vendorsQueryKeys,
  type ImageSpecRow,
  type ModelPricingConfig,
  type ModelPricingMode,
  type VideoMatrixRow,
} from '../../lib'
import type { Model } from '../../types'

const modelFormSchema = z.object({
  id: z.number().optional(),
  model_name: z.string().min(1, 'Model name is required'),
  alias: z.string().max(128, 'Alias must be at most 128 characters'),
  description: z.string(),
  icon: z.string(),
  tags: z.array(z.string()),
  vendor_id: z.number().optional(),
  endpoints: z.string(),
  name_rule: z.number(),
  status: z.boolean(),
  sync_official: z.boolean(),
  modal: z.enum(MODEL_MODAL_VALUES),
})

type ModelFormValues = z.infer<typeof modelFormSchema>

type ModelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Model | null
}

type RatioFormState = {
  usePrice: boolean
  modelPrice: string
  baseRatio: string
  completionRatio: string
  cacheRatio: string
  createCacheRatio: string
  imageRatio: string
  audioRatio: string
  audioCompletionRatio: string
}

const DEFAULT_RATIO_STATE: RatioFormState = {
  usePrice: false,
  modelPrice: '',
  baseRatio: '',
  completionRatio: '',
  cacheRatio: '1',
  createCacheRatio: '1.25',
  imageRatio: '1',
  audioRatio: '1',
  audioCompletionRatio: '1',
}

function nextRowId(rows: Array<{ id: number }>): number {
  return rows.reduce((max, row) => Math.max(max, row.id), 0) + 1
}

function toInput(value?: number, fallback = ''): string {
  return Number.isFinite(value) ? String(value) : fallback
}

function buildRatioState(config: ModelPricingConfig): RatioFormState {
  return {
    usePrice: Boolean(config.use_price),
    modelPrice: toInput(config.model_price),
    baseRatio: toInput(config.base_ratio),
    completionRatio: toInput(config.completion_ratio),
    cacheRatio: toInput(config.cache_ratio, '1'),
    createCacheRatio: toInput(config.create_cache_ratio, '1.25'),
    imageRatio: toInput(config.image_ratio, '1'),
    audioRatio: toInput(config.audio_ratio, '1'),
    audioCompletionRatio: toInput(config.audio_completion_ratio, '1'),
  }
}

function normalizePricingMode(mode?: string): ModelPricingMode {
  return MODEL_PRICING_MODE_VALUES.includes(mode as ModelPricingMode)
    ? (mode as ModelPricingMode)
    : 'inherit'
}

function buildRatioPricingFields(
  ratioState: RatioFormState
): Partial<ModelPricingConfig> {
  const hasRatioPricing =
    ratioState.usePrice || ratioState.baseRatio.trim().length > 0
  if (!hasRatioPricing) return {}
  return {
    use_price: ratioState.usePrice,
    use_ratio: !ratioState.usePrice,
    model_price: parseNonNegativeNumber(ratioState.modelPrice),
    base_ratio: parseNonNegativeNumber(ratioState.baseRatio),
    completion_ratio: parseNonNegativeNumber(ratioState.completionRatio),
    cache_ratio: parseNonNegativeNumber(ratioState.cacheRatio),
    create_cache_ratio: parseNonNegativeNumber(ratioState.createCacheRatio),
    image_ratio: parseNonNegativeNumber(ratioState.imageRatio),
    audio_ratio: parseNonNegativeNumber(ratioState.audioRatio),
    audio_completion_ratio: parseNonNegativeNumber(
      ratioState.audioCompletionRatio
    ),
  }
}

function buildPricingConfig(
  pricingMode: ModelPricingMode,
  ratioState: RatioFormState,
  imageRows: ImageSpecRow[],
  imageDefault: string,
  videoRows: VideoMatrixRow[],
  videoDefault: string,
  videoMin: string,
  videoMax: string
): ModelPricingConfig {
  if (pricingMode === 'ratio') {
    return {
      mode: 'ratio',
      use_price: ratioState.usePrice,
      model_price: parseNonNegativeNumber(ratioState.modelPrice),
      base_ratio: parseNonNegativeNumber(ratioState.baseRatio),
      completion_ratio: parseNonNegativeNumber(ratioState.completionRatio),
      cache_ratio: parseNonNegativeNumber(ratioState.cacheRatio),
      create_cache_ratio: parseNonNegativeNumber(ratioState.createCacheRatio),
      image_ratio: parseNonNegativeNumber(ratioState.imageRatio),
      audio_ratio: parseNonNegativeNumber(ratioState.audioRatio),
      audio_completion_ratio: parseNonNegativeNumber(
        ratioState.audioCompletionRatio
      ),
    }
  }
  if (pricingMode === 'image_spec') {
    return {
      mode: 'image_spec',
      ...buildRatioPricingFields(ratioState),
      unit: 'per_image',
      resolutions: imageRowsToResolutions(imageRows),
      default_cny_per_image: parseNonNegativeNumber(imageDefault),
    }
  }
  if (pricingMode === 'video_matrix') {
    return {
      mode: 'video_matrix',
      ...buildRatioPricingFields(ratioState),
      unit: 'per_second',
      prices: videoRowsToPrices(videoRows),
      default_cny_per_second: parseNonNegativeNumber(videoDefault),
      min_cny: parseNonNegativeNumber(videoMin),
      max_cny: parseNonNegativeNumber(videoMax),
    }
  }
  return { mode: pricingMode }
}

function readQuotaPerCNY(options?: Array<{ key: string; value: string }>) {
  return getOptionValue(options, { QuotaPerCNY: 0 }).QuotaPerCNY
}

function parsePastedVideoRows(text: string, startId: number): VideoMatrixRow[] {
  const rows: VideoMatrixRow[] = []
  let id = startId
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const [resolution, ratio, mode, priceOrStatus] = trimmed.split(/\t|,/)
    if (!resolution || !ratio || !mode) continue
    const normalizedValue = (priceOrStatus || '').trim().toLowerCase()
    rows.push({
      id,
      resolution: resolution.trim(),
      ratio: ratio.trim(),
      mode: mode.trim(),
      supported: normalizedValue !== 'unsupported',
      cnyPerSecond: normalizedValue === 'unsupported' ? '' : normalizedValue,
    })
    id += 1
  }
  return rows
}

function parseChannelGroups(groupValue?: string): string[] {
  const seen = new Set<string>()
  const groups: string[] = []
  for (const group of (groupValue || '').split(',')) {
    const normalized = group.trim()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    groups.push(normalized)
  }
  return groups
}

export function ModelMutateDrawer(props: ModelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const isEditing = Boolean(props.currentRow?.id)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [pricingMode, setPricingMode] = useState<ModelPricingMode>('inherit')
  const [ratioState, setRatioState] =
    useState<RatioFormState>(DEFAULT_RATIO_STATE)
  const [imageRows, setImageRows] = useState<ImageSpecRow[]>([
    { id: 1, resolution: '1k', cnyPerImage: '' },
  ])
  const [imageDefault, setImageDefault] = useState('')
  const [videoRows, setVideoRows] = useState<VideoMatrixRow[]>([
    {
      id: 1,
      resolution: '720p',
      ratio: '16:9',
      mode: 'no_video_input',
      supported: true,
      cnyPerSecond: '',
    },
  ])
  const [videoDefault, setVideoDefault] = useState('')
  const [videoMin, setVideoMin] = useState('')
  const [videoMax, setVideoMax] = useState('')
  const [pasteText, setPasteText] = useState('')
  const [selectedChannelIds, setSelectedChannelIds] = useState<number[]>([])

  const form = useForm<ModelFormValues>({
    resolver: zodResolver(modelFormSchema),
    defaultValues: {
      model_name: '',
      alias: '',
      description: '',
      icon: '',
      tags: [],
      vendor_id: undefined,
      endpoints: '',
      name_rule: 0,
      status: true,
      sync_official: true,
      modal: 'text',
    },
  })

  const { data: vendorsData } = useQuery({
    queryKey: vendorsQueryKeys.list(),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: props.open,
  })

  const { data: channelsData } = useQuery({
    queryKey: ['model-access-channels'],
    queryFn: () => getChannels({ page_size: 1000 }),
    enabled: props.open,
  })

  const { data: modelData } = useQuery({
    queryKey: modelsQueryKeys.detail(props.currentRow?.id || 0),
    queryFn: () => getModel(props.currentRow!.id),
    enabled: props.open && isEditing,
  })

  const { data: systemOptionsData } = useSystemOptions()
  const quotaPerCNY = readQuotaPerCNY(systemOptionsData?.data)
  const vendors = useMemo(
    () => vendorsData?.data?.items || [],
    [vendorsData?.data?.items]
  )
  const channels = useMemo(
    () => channelsData?.data?.items || [],
    [channelsData?.data?.items]
  )
  const model = modelData?.data || props.currentRow || null
  const selectedChannelIdSet = useMemo(
    () => new Set(selectedChannelIds),
    [selectedChannelIds]
  )
  const selectedGroups = useMemo(() => {
    const groups = new Set<string>()
    for (const channel of channels) {
      if (!selectedChannelIdSet.has(channel.id)) continue
      for (const group of parseChannelGroups(channel.group)) {
        groups.add(group)
      }
    }
    return Array.from(groups)
  }, [channels, selectedChannelIdSet])

  const currentPricingConfig = useMemo(
    () =>
      buildPricingConfig(
        pricingMode,
        ratioState,
        imageRows,
        imageDefault,
        videoRows,
        videoDefault,
        videoMin,
        videoMax
      ),
    [
      imageDefault,
      imageRows,
      pricingMode,
      ratioState,
      videoDefault,
      videoMax,
      videoMin,
      videoRows,
    ]
  )

  const currentPricingJson = useMemo(
    () => stringifyModelPricingConfig(currentPricingConfig),
    [currentPricingConfig]
  )

  useEffect(() => {
    return deferEffect(() => {
      if (!props.open) return
      const source = model
      const config = parseModelPricingConfig(source?.pricing_config)
      const mode = normalizePricingMode(source?.pricing_mode || config.mode)
      setPricingMode(mode)
      setRatioState(buildRatioState(config))
      setImageRows(imageRowsFromConfig(config))
      setImageDefault(toInput(config.default_cny_per_image))
      setVideoRows(videoRowsFromConfig(config))
      setVideoDefault(toInput(config.default_cny_per_second))
      setVideoMin(toInput(config.min_cny))
      setVideoMax(toInput(config.max_cny))
      setPasteText('')
      setSelectedChannelIds(
        (source?.bound_channels || [])
          .map((channel) => channel.id)
          .filter((id) => id > 0)
      )
      form.reset({
        id: source?.id,
        model_name: source?.model_name || props.currentRow?.model_name || '',
        alias: source?.alias || '',
        description: source?.description || '',
        icon: source?.icon || '',
        tags: parseModelTags(source?.tags),
        vendor_id: source?.vendor_id,
        endpoints: source?.endpoints || '',
        name_rule: source?.name_rule || 0,
        status: source?.status !== 0,
        sync_official: source?.sync_official !== 0,
        modal:
          MODEL_MODAL_VALUES.find((value) => value === source?.modal) || 'text',
      })
    })
  }, [form, model, props.currentRow, props.open])

  const updateRatioField = useCallback(
    (field: keyof RatioFormState, value: string | boolean) => {
      setRatioState((current) => ({ ...current, [field]: value }))
    },
    []
  )

  const addImageRow = useCallback(() => {
    setImageRows((current) => [
      ...current,
      { id: nextRowId(current), resolution: '1k', cnyPerImage: '' },
    ])
  }, [])

  const addVideoRow = useCallback(() => {
    setVideoRows((current) => [
      ...current,
      {
        id: nextRowId(current),
        resolution: '720p',
        ratio: '16:9',
        mode: 'no_video_input',
        supported: true,
        cnyPerSecond: '',
      },
    ])
  }, [])

  const applyVideoPaste = useCallback(() => {
    const rows = parsePastedVideoRows(pasteText, nextRowId(videoRows))
    if (rows.length === 0) {
      toast.error(t('No valid rows found'))
      return
    }
    setVideoRows((current) => [...current, ...rows])
    setPasteText('')
  }, [pasteText, t, videoRows])

  const toggleChannelAccess = useCallback(
    (channelId: number, enabled: boolean) => {
      setSelectedChannelIds((current) => {
        if (enabled) {
          return current.includes(channelId) ? current : [...current, channelId]
        }
        return current.filter((id) => id !== channelId)
      })
    },
    []
  )

  const handleFillEndpointTemplate = useCallback(
    (templateKey: string) => {
      const template = ENDPOINT_TEMPLATES[templateKey]
      if (!template) return
      form.setValue(
        'endpoints',
        JSON.stringify({ [templateKey]: template }, null, 2)
      )
    },
    [form]
  )

  const onSubmit = useCallback(
    async (values: ModelFormValues): Promise<void> => {
      setIsSubmitting(true)
      try {
        const payload = {
          id: isEditing ? props.currentRow!.id : undefined,
          model_name: values.model_name,
          alias: values.alias.trim(),
          description: values.description || '',
          icon: values.icon || '',
          tags: values.tags.filter(Boolean).join(','),
          vendor_id: values.vendor_id,
          endpoints: values.endpoints || '',
          name_rule: values.name_rule,
          status: values.status ? 1 : 0,
          sync_official: values.sync_official ? 1 : 0,
          modal: values.modal,
          pricing_mode: pricingMode,
          pricing_config: currentPricingJson,
          pricing_updated_time: Math.floor(Date.now() / 1000),
        }
        const response = isEditing
          ? await updateModel({ ...payload, id: props.currentRow!.id })
          : await createModel(payload)
        if (!response.success) {
          toast.error(response.message || t('Operation failed'))
          return
        }
        const savedModelId = isEditing
          ? props.currentRow!.id
          : response.data?.id
        if (savedModelId) {
          const accessResponse = await updateModelAccess(savedModelId, {
            channel_ids: selectedChannelIds,
          })
          if (!accessResponse.success) {
            toast.error(accessResponse.message || t('Failed to update access'))
            return
          }
        }
        toast.success(
          isEditing
            ? t('Model updated successfully')
            : t('Model created successfully')
        )
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
        if (savedModelId) {
          queryClient.invalidateQueries({
            queryKey: modelsQueryKeys.detail(savedModelId),
          })
        }
        props.onOpenChange(false)
      } catch (error: unknown) {
        toast.error((error as Error)?.message || t('Operation failed'))
      } finally {
        setIsSubmitting(false)
      }
    },
    [
      currentPricingJson,
      isEditing,
      pricingMode,
      props,
      queryClient,
      selectedChannelIds,
      t,
    ]
  )

  const goToChannels = useCallback(() => {
    void navigate({ to: '/channels' })
  }, [navigate])

  const goToGroupSettings = useCallback(() => {
    void navigate({
      to: '/system-settings/billing/$section',
      params: { section: 'group-pricing' },
    })
  }, [navigate])

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-4xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing ? t('Edit Model') : t('Create Model')}
          </SheetTitle>
          <SheetDescription>
            {isEditing
              ? t("Update model configuration and click save when you're done.")
              : t(
                  'Add a new model to the system by providing the necessary information.'
                )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='model-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('Basic Information')}
              </h3>
              <div className='grid gap-4 md:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='model_name'
                  render={({ field }) => (
                    <FormItem className='md:col-span-2'>
                      <FormLabel>{t('Model Name *')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('gpt-4, claude-3-opus, etc.')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='alias'
                  render={({ field }) => (
                    <FormItem className='md:col-span-2'>
                      <FormLabel>{t('Display alias')}</FormLabel>
                      <FormControl>
                        <Input placeholder='Nano Banana' {...field} />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Only used for display. Requests still use the real model ID.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='modal'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Modality')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          value={field.value}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          className='w-full'
                        >
                          <NativeSelectOption value='text'>
                            {t('Text')}
                          </NativeSelectOption>
                          <NativeSelectOption value='image'>
                            {t('Image')}
                          </NativeSelectOption>
                          <NativeSelectOption value='video'>
                            {t('Video')}
                          </NativeSelectOption>
                          <NativeSelectOption value='audio'>
                            {t('Audio')}
                          </NativeSelectOption>
                        </NativeSelect>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='vendor_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Vendor')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          value={field.value ? String(field.value) : ''}
                          onChange={(event) =>
                            field.onChange(
                              event.target.value
                                ? Number(event.target.value)
                                : undefined
                            )
                          }
                          className='w-full'
                        >
                          <NativeSelectOption value=''>
                            {t('Unassigned')}
                          </NativeSelectOption>
                          {vendors.map((vendor) => (
                            <NativeSelectOption
                              key={vendor.id}
                              value={String(vendor.id)}
                            >
                              {vendor.name}
                            </NativeSelectOption>
                          ))}
                        </NativeSelect>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='description'
                  render={({ field }) => (
                    <FormItem className='md:col-span-2'>
                      <FormLabel>{t('Description')}</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t('Describe this model...')}
                          rows={3}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='icon'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Icon')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('OpenAI, Anthropic, etc.')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='tags'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Tags')}</FormLabel>
                      <FormControl>
                        <TagInput
                          value={field.value || []}
                          onChange={field.onChange}
                          placeholder={t('Add tags...')}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>{t('Matching Rules')}</h3>
              <div className='grid gap-4 md:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='name_rule'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Name Rule')}</FormLabel>
                      <FormControl>
                        <NativeSelect
                          value={String(field.value)}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value))
                          }
                          className='w-full'
                        >
                          {getNameRuleOptions(t).map((option) => (
                            <NativeSelectOption
                              key={option.value}
                              value={String(option.value)}
                            >
                              {option.label}
                            </NativeSelectOption>
                          ))}
                        </NativeSelect>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className='grid gap-3'>
                  <FormField
                    control={form.control}
                    name='status'
                    render={({ field }) => (
                      <FormItem className={sideDrawerSwitchItemClassName()}>
                        <div>
                          <FormLabel>{t('Enabled')}</FormLabel>
                          <FormDescription>
                            {t('Allow this model to appear in routing.')}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='sync_official'
                    render={({ field }) => (
                      <FormItem className={sideDrawerSwitchItemClassName()}>
                        <div>
                          <FormLabel>{t('Official Sync')}</FormLabel>
                          <FormDescription>
                            {t('Allow upstream metadata sync to update it.')}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </div>
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <h3 className='text-sm font-semibold'>{t('Endpoints')}</h3>
                <NativeSelect
                  defaultValue=''
                  onChange={(event) => {
                    if (event.target.value) {
                      handleFillEndpointTemplate(event.target.value)
                      event.target.value = ''
                    }
                  }}
                >
                  <NativeSelectOption value=''>
                    {t('Load template...')}
                  </NativeSelectOption>
                  {Object.keys(ENDPOINT_TEMPLATES).map((key) => (
                    <NativeSelectOption key={key} value={key}>
                      {key}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </div>
              <FormField
                control={form.control}
                name='endpoints'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Endpoint Configuration')}</FormLabel>
                    <FormControl>
                      <JsonEditor
                        value={field.value || ''}
                        onChange={field.onChange}
                        keyPlaceholder={t('endpoint_type')}
                        valuePlaceholder='{"path": "/v1/...", "method": "POST"}'
                        keyLabel={t('Endpoint Type')}
                        valueLabel={t('Configuration')}
                        valueType='any'
                        emptyMessage={t(
                          'No endpoints configured. Switch to JSON mode or add rows to define endpoints.'
                        )}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <h3 className='text-sm font-semibold'>
                  {t('Pricing Configuration')}
                </h3>
                <StatusBadge
                  label={t('Quota/CNY {{quota}}', {
                    quota: formatNumber(quotaPerCNY),
                  })}
                  variant='info'
                  size='sm'
                  copyable={false}
                />
              </div>
              <Tabs
                value={pricingMode}
                onValueChange={(value) =>
                  setPricingMode(normalizePricingMode(value))
                }
              >
                <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                  <TabsTrigger value='ratio'>{t('Ratio')}</TabsTrigger>
                  <TabsTrigger value='image_spec'>
                    {t('Image spec')}
                  </TabsTrigger>
                  <TabsTrigger value='video_matrix'>
                    {t('Video matrix')}
                  </TabsTrigger>
                  <TabsTrigger value='free'>{t('Free')}</TabsTrigger>
                  <TabsTrigger value='inherit'>
                    {t('Legacy fallback')}
                  </TabsTrigger>
                </TabsList>
              </Tabs>

              {pricingMode === 'ratio' && (
                <RatioPricingEditor
                  ratioState={ratioState}
                  onChange={updateRatioField}
                  quotaPerCNY={quotaPerCNY}
                />
              )}
              {pricingMode === 'image_spec' && (
                <ImageSpecEditor
                  rows={imageRows}
                  onRowsChange={setImageRows}
                  defaultCNY={imageDefault}
                  onDefaultCNYChange={setImageDefault}
                  onAddRow={addImageRow}
                  quotaPerCNY={quotaPerCNY}
                />
              )}
              {pricingMode === 'video_matrix' && (
                <VideoMatrixEditor
                  rows={videoRows}
                  onRowsChange={setVideoRows}
                  defaultCNY={videoDefault}
                  onDefaultCNYChange={setVideoDefault}
                  minCNY={videoMin}
                  onMinCNYChange={setVideoMin}
                  maxCNY={videoMax}
                  onMaxCNYChange={setVideoMax}
                  pasteText={pasteText}
                  onPasteTextChange={setPasteText}
                  onAddRow={addVideoRow}
                  onApplyPaste={applyVideoPaste}
                  quotaPerCNY={quotaPerCNY}
                />
              )}
              {(pricingMode === 'free' || pricingMode === 'inherit') && (
                <div className='border-border bg-muted/30 rounded-lg border p-3 text-sm'>
                  {pricingMode === 'free'
                    ? t('This model will be billed as free.')
                    : t('This model will fall back to legacy pricing options.')}
                </div>
              )}

              <JsonEditor
                value={currentPricingJson}
                onChange={() => undefined}
                keyPlaceholder={t('key')}
                valuePlaceholder={t('value')}
                keyLabel={t('Pricing key')}
                valueLabel={t('Pricing value')}
                valueType='any'
                emptyMessage={t('No pricing config generated.')}
                disabled
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <h3 className='text-sm font-semibold'>{t('Access')}</h3>
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={goToChannels}
                  >
                    <ExternalLink className='size-3.5' />
                    {t('Channels')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={goToGroupSettings}
                  >
                    <ExternalLink className='size-3.5' />
                    {t('Groups')}
                  </Button>
                </div>
              </div>
              <div className='grid gap-3 md:grid-cols-2'>
                <ChannelAccessEditor
                  title={t('Channel mapping')}
                  emptyText={t('No channels available')}
                  channels={channels}
                  selectedChannelIds={selectedChannelIds}
                  onToggle={toggleChannelAccess}
                />
                <div className='space-y-2'>
                  <Label>{t('Group visibility')}</Label>
                  <div className='border-border bg-background flex min-h-10 flex-wrap gap-1 rounded-lg border p-2'>
                    {(selectedGroups.length > 0
                      ? selectedGroups
                      : model?.enable_groups || []
                    ).length > 0 ? (
                      (selectedGroups.length > 0
                        ? selectedGroups
                        : model?.enable_groups || []
                      ).map((group) => (
                        <GroupBadge key={group} group={group} size='sm' />
                      ))
                    ) : (
                      <span className='text-muted-foreground text-sm'>
                        {t('No enabled groups')}
                      </span>
                    )}
                  </div>
                </div>
              </div>
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form='model-form' disabled={isSubmitting}>
            {isSubmitting && <Loader2 className='size-4 animate-spin' />}
            {isEditing ? t('Save changes') : t('Create Model')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function RatioPricingEditor(props: {
  ratioState: RatioFormState
  onChange: (field: keyof RatioFormState, value: string | boolean) => void
  quotaPerCNY: number
}) {
  const { t } = useTranslation()
  const previewCNY = props.ratioState.usePrice
    ? parseNonNegativeNumber(props.ratioState.modelPrice)
    : 0
  return (
    <div className='grid gap-4 md:grid-cols-2'>
      <label className={sideDrawerSwitchItemClassName()}>
        <span>
          <span className='block text-sm font-medium'>
            {t('Fixed per request')}
          </span>
          <span className='text-muted-foreground text-xs'>
            {t('Use model_price as an absolute per-call price.')}
          </span>
        </span>
        <Switch
          checked={props.ratioState.usePrice}
          onCheckedChange={(checked) => props.onChange('usePrice', checked)}
        />
      </label>
      <NumberField
        label={t('Model price')}
        value={props.ratioState.modelPrice}
        onChange={(value) => props.onChange('modelPrice', value)}
        disabled={!props.ratioState.usePrice}
        suffix={t('{{quota}} quota', {
          quota: quotaPreview(previewCNY, props.quotaPerCNY),
        })}
      />
      <NumberField
        label={t('Base ratio')}
        value={props.ratioState.baseRatio}
        onChange={(value) => props.onChange('baseRatio', value)}
      />
      <NumberField
        label={t('Completion ratio')}
        value={props.ratioState.completionRatio}
        onChange={(value) => props.onChange('completionRatio', value)}
      />
      <NumberField
        label={t('Cache ratio')}
        value={props.ratioState.cacheRatio}
        onChange={(value) => props.onChange('cacheRatio', value)}
      />
      <NumberField
        label={t('Create cache ratio')}
        value={props.ratioState.createCacheRatio}
        onChange={(value) => props.onChange('createCacheRatio', value)}
      />
      <NumberField
        label={t('Image token ratio')}
        value={props.ratioState.imageRatio}
        onChange={(value) => props.onChange('imageRatio', value)}
      />
      <NumberField
        label={t('Audio ratio')}
        value={props.ratioState.audioRatio}
        onChange={(value) => props.onChange('audioRatio', value)}
      />
      <NumberField
        label={t('Audio completion ratio')}
        value={props.ratioState.audioCompletionRatio}
        onChange={(value) => props.onChange('audioCompletionRatio', value)}
      />
    </div>
  )
}

function ImageSpecEditor(props: {
  rows: ImageSpecRow[]
  onRowsChange: (rows: ImageSpecRow[]) => void
  defaultCNY: string
  onDefaultCNYChange: (value: string) => void
  onAddRow: () => void
  quotaPerCNY: number
}) {
  const { t } = useTranslation()
  const updateRow = (id: number, patch: Partial<ImageSpecRow>) => {
    props.onRowsChange(
      props.rows.map((row) => (row.id === id ? { ...row, ...patch } : row))
    )
  }
  return (
    <div className='space-y-3'>
      <div className='grid gap-3 md:grid-cols-[1fr_auto]'>
        <NumberField
          label={t('Default CNY per image')}
          value={props.defaultCNY}
          onChange={props.onDefaultCNYChange}
          suffix={t('{{quota}} quota', {
            quota: quotaPreview(
              parseNonNegativeNumber(props.defaultCNY),
              props.quotaPerCNY
            ),
          })}
        />
        <div className='flex items-end'>
          <Button type='button' variant='outline' onClick={props.onAddRow}>
            <Plus className='size-4' />
            {t('Add tier')}
          </Button>
        </div>
      </div>
      <div className='overflow-x-auto'>
        <div className='min-w-[560px] space-y-2'>
          {props.rows.map((row) => (
            <div
              key={row.id}
              className='grid grid-cols-[140px_1fr_120px_36px] items-end gap-2'
            >
              <SelectField
                label={t('Resolution')}
                value={row.resolution}
                options={IMAGE_RESOLUTION_OPTIONS}
                onChange={(value) => updateRow(row.id, { resolution: value })}
              />
              <NumberField
                label={t('CNY per image')}
                value={row.cnyPerImage}
                onChange={(value) => updateRow(row.id, { cnyPerImage: value })}
                suffix={t('{{quota}} quota', {
                  quota: quotaPreview(
                    parseNonNegativeNumber(row.cnyPerImage),
                    props.quotaPerCNY
                  ),
                })}
              />
              <div className='text-muted-foreground pb-2 text-xs'>
                {t('n=1 preview')}
              </div>
              <IconButton
                label={t('Remove')}
                onClick={() =>
                  props.onRowsChange(
                    props.rows.filter((current) => current.id !== row.id)
                  )
                }
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function VideoMatrixEditor(props: {
  rows: VideoMatrixRow[]
  onRowsChange: (rows: VideoMatrixRow[]) => void
  defaultCNY: string
  onDefaultCNYChange: (value: string) => void
  minCNY: string
  onMinCNYChange: (value: string) => void
  maxCNY: string
  onMaxCNYChange: (value: string) => void
  pasteText: string
  onPasteTextChange: (value: string) => void
  onAddRow: () => void
  onApplyPaste: () => void
  quotaPerCNY: number
}) {
  const { t } = useTranslation()
  const updateRow = (id: number, patch: Partial<VideoMatrixRow>) => {
    props.onRowsChange(
      props.rows.map((row) => (row.id === id ? { ...row, ...patch } : row))
    )
  }
  return (
    <div className='space-y-4'>
      <div className='grid gap-3 md:grid-cols-3'>
        <NumberField
          label={t('Default CNY per second')}
          value={props.defaultCNY}
          onChange={props.onDefaultCNYChange}
        />
        <NumberField
          label={t('Minimum CNY')}
          value={props.minCNY}
          onChange={props.onMinCNYChange}
        />
        <NumberField
          label={t('Maximum CNY')}
          value={props.maxCNY}
          onChange={props.onMaxCNYChange}
        />
      </div>
      <div className='grid gap-2 md:grid-cols-[1fr_auto]'>
        <Textarea
          value={props.pasteText}
          onChange={(event) => props.onPasteTextChange(event.target.value)}
          rows={3}
          placeholder={t('resolution,ratio,mode,cny_per_second')}
        />
        <div className='flex items-start gap-2 md:flex-col'>
          <Button type='button' variant='outline' onClick={props.onApplyPaste}>
            <ClipboardPaste className='size-4' />
            {t('Paste rows')}
          </Button>
          <Button type='button' variant='outline' onClick={props.onAddRow}>
            <Plus className='size-4' />
            {t('Add cell')}
          </Button>
        </div>
      </div>
      <div className='overflow-x-auto'>
        <div className='min-w-[860px] space-y-2'>
          {props.rows.map((row) => (
            <div
              key={row.id}
              className='grid grid-cols-[120px_110px_170px_92px_1fr_120px_36px] items-end gap-2'
            >
              <SelectField
                label={t('Resolution')}
                value={row.resolution}
                options={VIDEO_RESOLUTION_OPTIONS}
                onChange={(value) => updateRow(row.id, { resolution: value })}
              />
              <SelectField
                label={t('Ratio')}
                value={row.ratio}
                options={VIDEO_RATIO_OPTIONS}
                onChange={(value) => updateRow(row.id, { ratio: value })}
              />
              <SelectField
                label={t('Mode')}
                value={row.mode}
                options={VIDEO_MODE_OPTIONS}
                onChange={(value) => updateRow(row.id, { mode: value })}
              />
              <label className='flex h-8 items-center gap-2 pb-0.5 text-sm'>
                <Switch
                  checked={row.supported}
                  onCheckedChange={(checked) =>
                    updateRow(row.id, { supported: checked })
                  }
                />
                {t('On')}
              </label>
              <NumberField
                label={t('CNY per second')}
                value={row.cnyPerSecond}
                onChange={(value) => updateRow(row.id, { cnyPerSecond: value })}
                disabled={!row.supported}
                suffix={t('{{quota}} quota', {
                  quota: quotaPreview(
                    parseNonNegativeNumber(row.cnyPerSecond) * 5,
                    props.quotaPerCNY
                  ),
                })}
              />
              <div className='text-muted-foreground pb-2 text-xs'>
                {t('5s preview')}
              </div>
              <IconButton
                label={t('Remove')}
                onClick={() =>
                  props.onRowsChange(
                    props.rows.filter((current) => current.id !== row.id)
                  )
                }
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function NumberField(props: {
  label: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  suffix?: string
}) {
  return (
    <div className='space-y-1.5'>
      <Label>{props.label}</Label>
      <Input
        value={props.value}
        disabled={props.disabled}
        inputMode='decimal'
        onChange={(event) => {
          const value = event.target.value
          if (/^\d*\.?\d*$/.test(value)) {
            props.onChange(value)
          }
        }}
      />
      {props.suffix && (
        <div className='text-muted-foreground text-xs'>{props.suffix}</div>
      )}
    </div>
  )
}

function SelectField(props: {
  label: string
  value: string
  options: readonly string[]
  onChange: (value: string) => void
}) {
  return (
    <div className='space-y-1.5'>
      <Label>{props.label}</Label>
      <NativeSelect
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        className='w-full'
      >
        {props.options.map((option) => (
          <NativeSelectOption key={option} value={option}>
            {option}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  )
}

function IconButton(props: { label: string; onClick: () => void }) {
  return (
    <Button
      type='button'
      variant='ghost'
      size='icon-sm'
      aria-label={props.label}
      title={props.label}
      onClick={props.onClick}
    >
      <Trash2 className='size-4' />
    </Button>
  )
}

function ChannelAccessEditor(props: {
  title: string
  emptyText: string
  channels: Channel[]
  selectedChannelIds: number[]
  onToggle: (channelId: number, enabled: boolean) => void
}) {
  const selected = new Set(props.selectedChannelIds)
  return (
    <div className='space-y-2'>
      <Label>{props.title}</Label>
      <div
        className={cn(
          'border-border bg-background flex max-h-56 min-h-10 flex-col gap-1 overflow-y-auto rounded-lg border p-2',
          props.channels.length === 0 && 'items-center justify-center'
        )}
      >
        {props.channels.length > 0 ? (
          props.channels.map((channel) => (
            <label
              key={channel.id}
              className='hover:bg-muted/50 flex min-h-9 cursor-pointer items-center gap-2 rounded-md px-2 py-1.5'
            >
              <Checkbox
                checked={selected.has(channel.id)}
                onCheckedChange={(checked) =>
                  props.onToggle(channel.id, checked === true)
                }
                aria-label={channel.name}
              />
              <span className='min-w-0 flex-1 truncate text-sm'>
                {channel.name}
              </span>
              <StatusBadge
                label={String(channel.type)}
                autoColor={String(channel.type)}
                size='sm'
              />
            </label>
          ))
        ) : (
          <span className='text-muted-foreground text-sm'>
            {props.emptyText}
          </span>
        )}
      </div>
    </div>
  )
}
