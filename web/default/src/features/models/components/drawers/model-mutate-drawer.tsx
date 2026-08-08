import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { JsonEditor } from '@/components/json-editor'
import { TagInput } from '@/components/tag-input'
import { Button } from '@/components/ui/button'
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
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  createModel,
  getModel,
  getTextPricingConfig,
  getVendors,
  updateModel,
} from '../../api'
import { ENDPOINT_TEMPLATES, getNameRuleOptions } from '../../constants'
import {
  formatCNYInput,
  imageRowsFromConfig,
  imageRowsToResolutions,
  modelsQueryKeys,
  normalizeOfficialPriceProfiles,
  normalizeTextPricingCategories,
  parseModelPricingConfig,
  parseModelTags,
  stringifyModelPricingConfig,
  vendorsQueryKeys,
  videoRowsFromConfig,
  videoRowsToPrices,
} from '../../lib'
import { modelFormSchema, type Model, type ModelFormValues } from '../../types'
import {
  type MediaPricingDraft,
  ModelPricingEditor,
} from '../model-pricing-editor'

const editorSchema = modelFormSchema.superRefine((values, context) => {
  if (values.endpoints.trim()) {
    try {
      JSON.parse(values.endpoints)
    } catch {
      context.addIssue({
        code: 'custom',
        path: ['endpoints'],
        message: 'Endpoint configuration must be valid JSON',
      })
    }
  }

  if (
    values.modal === 'text' &&
    values.status &&
    values.text_category === 'unclassified'
  ) {
    context.addIssue({
      code: 'custom',
      path: ['text_category'],
      message: 'Enabled text models require a supported category',
    })
  }

  if (
    values.modal === 'text' &&
    values.status &&
    !values.official_price_key.trim()
  ) {
    context.addIssue({
      code: 'custom',
      path: ['official_price_key'],
      message: 'Enabled text models require an official price profile',
    })
  }
})

type ModelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Model | null
}

const EMPTY_MEDIA_DRAFT: MediaPricingDraft = {
  imageRows: [{ id: 1, resolution: '1k', cnyPerImage: '' }],
  videoRows: [
    {
      id: 1,
      resolution: '720p',
      ratio: '16:9',
      mode: 'no_video_input',
      supported: true,
      cnyPerSecond: '',
    },
  ],
  imageDefaultCNY: '',
  videoDefaultCNY: '',
}

export function ModelMutateDrawer(props: ModelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentModelId = props.currentRow?.id
  const isEditing = Boolean(currentModelId)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [mediaDraft, setMediaDraft] =
    useState<MediaPricingDraft>(EMPTY_MEDIA_DRAFT)

  const vendorsQuery = useQuery({
    queryKey: vendorsQueryKeys.list(),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: props.open,
  })
  const modelQuery = useQuery({
    queryKey: modelsQueryKeys.detail(currentModelId || 0),
    queryFn: () => {
      if (!currentModelId) throw new Error('Model ID is required')
      return getModel(currentModelId)
    },
    enabled: props.open && isEditing,
  })
  const textPricingQuery = useQuery({
    queryKey: modelsQueryKeys.textPricing(),
    queryFn: getTextPricingConfig,
    enabled: props.open,
  })

  const vendors = vendorsQuery.data?.data?.items || []
  const categories = useMemo(
    () =>
      normalizeTextPricingCategories(textPricingQuery.data?.data?.categories),
    [textPricingQuery.data?.data?.categories]
  )
  const profiles = useMemo(
    () => normalizeOfficialPriceProfiles(textPricingQuery.data?.data?.profiles),
    [textPricingQuery.data?.data?.profiles]
  )

  const form = useForm<ModelFormValues>({
    resolver: zodResolver(editorSchema) as unknown as Resolver<ModelFormValues>,
    defaultValues: createEmptyFormValues(props.currentRow?.model_name),
  })

  const modal = form.watch('modal')
  const textCategory = form.watch('text_category')
  const officialPriceKey = form.watch('official_price_key')
  const pricingMode = form.watch('pricing_mode')
  const pricingConfig = form.watch('pricing_config')

  useEffect(() => {
    if (!props.open) return
    if (isEditing && !modelQuery.data?.data) return

    const model = isEditing ? modelQuery.data?.data : props.currentRow
    const values = model
      ? modelToFormValues(model, !isEditing)
      : createEmptyFormValues()
    form.reset(values)

    const parsed = parseModelPricingConfig(model?.pricing_config)
    setMediaDraft({
      imageRows: imageRowsFromConfig(parsed),
      videoRows: videoRowsFromConfig(parsed),
      imageDefaultCNY: formatCNYInput(parsed.default_cny_per_image),
      videoDefaultCNY: formatCNYInput(parsed.default_cny_per_second),
    })
  }, [form, isEditing, modelQuery.data?.data, props.currentRow, props.open])

  const handleModalChange = useCallback(
    (value: string) => {
      let nextPricingMode = 'inherit'
      if (value === 'image') {
        nextPricingMode = 'image_spec'
      } else if (value === 'video') {
        nextPricingMode = 'video_matrix'
      }

      form.setValue('modal', value, { shouldDirty: true })
      form.setValue('text_category', value === 'text' ? 'unclassified' : '', {
        shouldDirty: true,
      })
      form.setValue('official_price_key', '', { shouldDirty: true })
      form.setValue('pricing_mode', nextPricingMode, { shouldDirty: true })
      form.setValue('pricing_config', '', { shouldDirty: true })
      setMediaDraft(EMPTY_MEDIA_DRAFT)
    },
    [form]
  )

  const onSubmit = useCallback(
    async (values: ModelFormValues): Promise<void> => {
      const pricing = buildPricingPayload(values, mediaDraft)
      if (!pricing.ok) {
        toast.error(t(pricing.message))
        return
      }

      if (values.modal === 'text' && values.status) {
        const category = categories.find(
          (entry) => entry.category === values.text_category
        )
        if (!category?.multiplier) {
          toast.error(
            t('The selected text category has no multiplier configured')
          )
          return
        }
      }

      const existing = modelQuery.data?.data
      const pricingChanged =
        !existing ||
        existing.modal !== values.modal ||
        existing.text_category !== values.text_category ||
        existing.official_price_key !== values.official_price_key ||
        existing.pricing_mode !== pricing.mode ||
        existing.pricing_config !== pricing.config

      const payload: Partial<Model> & { id?: number } = {
        id: isEditing ? currentModelId : undefined,
        model_name: values.model_name,
        alias: values.alias,
        description: values.description,
        icon: values.icon,
        tags: values.tags.filter(Boolean).join(','),
        vendor_id: values.vendor_id,
        endpoints: values.endpoints,
        name_rule: values.name_rule,
        status: values.status ? 1 : 0,
        sync_official: values.sync_official ? 1 : 0,
        modal: values.modal,
        text_category: values.modal === 'text' ? values.text_category : '',
        official_price_key:
          values.modal === 'text' ? values.official_price_key : '',
        pricing_mode: pricing.mode,
        pricing_config: pricing.config,
        pricing_updated_time: pricingChanged
          ? Math.floor(Date.now() / 1000)
          : values.pricing_updated_time,
      }

      setIsSubmitting(true)
      try {
        const response =
          isEditing && currentModelId
            ? await updateModel({ ...payload, id: currentModelId })
            : await createModel(payload)

        if (!response.success) {
          toast.error(response.message || t('Operation failed'))
          return
        }

        toast.success(
          isEditing
            ? t('Model updated successfully')
            : t('Model created successfully')
        )
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
          queryClient.invalidateQueries({
            queryKey: modelsQueryKeys.detail(currentModelId || 0),
          }),
          queryClient.invalidateQueries({
            queryKey: modelsQueryKeys.textPricing(),
          }),
        ])
        props.onOpenChange(false)
      } catch (error: unknown) {
        toast.error((error as Error)?.message || t('Operation failed'))
      } finally {
        setIsSubmitting(false)
      }
    },
    [
      categories,
      currentModelId,
      isEditing,
      mediaDraft,
      modelQuery.data?.data,
      props,
      queryClient,
      t,
    ]
  )

  const handleFillEndpointTemplate = (templateKey: string) => {
    const template = ENDPOINT_TEMPLATES[templateKey]
    if (!template) return
    form.setValue(
      'endpoints',
      JSON.stringify({ [templateKey]: template }, null, 2),
      { shouldDirty: true }
    )
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing ? t('Edit Model') : t('Create Model')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Manage model metadata, classification, and pricing in one place.'
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

              <FormField
                control={form.control}
                name='model_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Model Name *')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('gpt-4, claude-3-opus, etc.')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('The unique identifier for this model')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='alias'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Alias')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Optional display alias')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
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

              <div className='grid gap-4 sm:grid-cols-2'>
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
                  name='vendor_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Vendor')}</FormLabel>
                      <Select
                        value={field.value ? String(field.value) : undefined}
                        onValueChange={(value) =>
                          field.onChange(
                            value ? Number.parseInt(value, 10) : undefined
                          )
                        }
                      >
                        <FormControl>
                          <SelectTrigger className='w-full'>
                            <SelectValue placeholder={t('Select vendor')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {vendors.map((vendor) => (
                              <SelectItem
                                key={vendor.id}
                                value={String(vendor.id)}
                              >
                                {vendor.name}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

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
                    <FormDescription>
                      {t('Press Enter or comma to add tags')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('Classification & Pricing')}
              </h3>

              <FormField
                control={form.control}
                name='modal'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Model modality')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={(value) => {
                        if (value) handleModalChange(value)
                      }}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='text'>{t('Text')}</SelectItem>
                          <SelectItem value='image'>{t('Image')}</SelectItem>
                          <SelectItem value='video'>{t('Video')}</SelectItem>
                          <SelectItem value='audio'>{t('Audio')}</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <ModelPricingEditor
                modal={modal}
                textCategory={textCategory}
                officialPriceKey={officialPriceKey}
                pricingMode={pricingMode}
                pricingConfig={pricingConfig}
                categories={categories}
                profiles={profiles}
                currentModel={modelQuery.data?.data}
                mediaDraft={mediaDraft}
                onTextCategoryChange={(value) =>
                  form.setValue('text_category', value, {
                    shouldDirty: true,
                    shouldValidate: true,
                  })
                }
                onOfficialPriceKeyChange={(value) =>
                  form.setValue('official_price_key', value, {
                    shouldDirty: true,
                    shouldValidate: true,
                  })
                }
                onPricingModeChange={(value) =>
                  form.setValue('pricing_mode', value, { shouldDirty: true })
                }
                onPricingConfigChange={(value) =>
                  form.setValue('pricing_config', value, { shouldDirty: true })
                }
                onMediaDraftChange={setMediaDraft}
              />

              <FormField
                control={form.control}
                name='text_category'
                render={() => <FormMessage />}
              />
              <FormField
                control={form.control}
                name='official_price_key'
                render={() => <FormMessage />}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>{t('Matching Rules')}</h3>
              <FormField
                control={form.control}
                name='name_rule'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name Rule')}</FormLabel>
                    <FormControl>
                      <RadioGroup
                        value={String(field.value)}
                        onValueChange={(value) =>
                          field.onChange(Number.parseInt(value, 10))
                        }
                        className='grid grid-cols-2 gap-4'
                      >
                        {getNameRuleOptions(t).map((option) => (
                          <div
                            key={option.value}
                            className='flex items-center space-x-2'
                          >
                            <RadioGroupItem
                              value={String(option.value)}
                              id={`rule-${option.value}`}
                            />
                            <Label
                              htmlFor={`rule-${option.value}`}
                              className='cursor-pointer font-normal'
                            >
                              {option.label}
                            </Label>
                          </div>
                        ))}
                      </RadioGroup>
                    </FormControl>
                    <FormDescription>
                      {t('How this model name should match requests')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <div className='flex items-center justify-between gap-3'>
                <h3 className='text-sm font-semibold'>{t('Endpoints')}</h3>
                <Select
                  onValueChange={(value) => {
                    if (typeof value === 'string') {
                      handleFillEndpointTemplate(value)
                    }
                  }}
                >
                  <SelectTrigger size='sm' className='w-[200px]'>
                    <SelectValue placeholder={t('Load template...')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {Object.keys(ENDPOINT_TEMPLATES).map((key) => (
                        <SelectItem key={key} value={key}>
                          {key}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
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
                        keyPlaceholder='endpoint_type'
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
              <h3 className='text-sm font-semibold'>{t('Status & Sync')}</h3>

              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enabled')}
                      </FormLabel>
                      <FormDescription>
                        {modal === 'text'
                          ? t(
                              'Enabled text models must have a valid category and official price profile.'
                            )
                          : t('Enable or disable this model')}
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
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-base'>
                        {t('Official Sync')}
                      </FormLabel>
                      <FormDescription>
                        {t('Sync this model with official upstream')}
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
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' disabled={isSubmitting} />}
          >
            {t('Cancel')}
          </SheetClose>
          <Button form='model-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? (
              <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            ) : null}
            {isEditing ? t('Update Model') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function createEmptyFormValues(modelName = ''): ModelFormValues {
  return {
    model_name: modelName,
    alias: '',
    description: '',
    icon: '',
    tags: [],
    vendor_id: undefined,
    endpoints: '',
    name_rule: 0,
    status: false,
    sync_official: true,
    modal: 'text',
    text_category: 'unclassified',
    official_price_key: '',
    pricing_mode: 'inherit',
    pricing_config: '',
    pricing_updated_time: 0,
  }
}

function modelToFormValues(
  model: Model,
  forceDisabled: boolean
): ModelFormValues {
  return {
    id: model.id,
    model_name: model.model_name,
    alias: model.alias || '',
    description: model.description || '',
    icon: model.icon || '',
    tags: parseModelTags(model.tags),
    vendor_id: model.vendor_id,
    endpoints: model.endpoints || '',
    name_rule: model.name_rule || 0,
    status: forceDisabled ? false : model.status === 1,
    sync_official: model.sync_official === 1,
    modal: model.modal || 'text',
    text_category: model.text_category || 'unclassified',
    official_price_key: model.official_price_key || '',
    pricing_mode: model.pricing_mode || 'inherit',
    pricing_config: model.pricing_config || '',
    pricing_updated_time: model.pricing_updated_time || 0,
  }
}

type PricingPayloadResult =
  | { ok: true; mode: string; config: string }
  | { ok: false; message: string }

function buildPricingPayload(
  values: ModelFormValues,
  draft: MediaPricingDraft
): PricingPayloadResult {
  if (values.modal === 'text') {
    return { ok: true, mode: 'inherit', config: '' }
  }

  if (values.pricing_mode === 'free' || values.pricing_mode === 'inherit') {
    return {
      ok: true,
      mode: values.pricing_mode,
      config: stringifyModelPricingConfig({ mode: values.pricing_mode }),
    }
  }

  if (values.modal === 'image') {
    const validRows = draft.imageRows.filter(
      (row) => row.resolution.trim() && isValidPrice(row.cnyPerImage)
    )
    const duplicateResolution = hasDuplicates(
      validRows.map((row) => row.resolution.trim())
    )
    if (duplicateResolution) {
      return { ok: false, message: 'Image resolutions must be unique' }
    }
    if (validRows.length === 0 && !isValidPrice(draft.imageDefaultCNY)) {
      return {
        ok: false,
        message: 'Configure at least one image price or a default price',
      }
    }
    const defaultPrice = parseOptionalPrice(draft.imageDefaultCNY)
    return {
      ok: true,
      mode: 'image_spec',
      config: stringifyModelPricingConfig({
        mode: 'image_spec',
        unit: 'per_image',
        resolutions: imageRowsToResolutions(validRows),
        ...(defaultPrice === undefined
          ? {}
          : { default_cny_per_image: defaultPrice }),
      }),
    }
  }

  if (values.modal === 'video') {
    const validRows = draft.videoRows.filter(
      (row) =>
        row.resolution.trim() &&
        row.ratio.trim() &&
        row.mode.trim() &&
        (!row.supported || isValidPrice(row.cnyPerSecond))
    )
    const duplicateCell = hasDuplicates(
      validRows.map((row) =>
        [row.resolution.trim(), row.ratio.trim(), row.mode.trim()].join('|')
      )
    )
    if (duplicateCell) {
      return { ok: false, message: 'Video matrix cells must be unique' }
    }
    if (
      !validRows.some((row) => row.supported) &&
      !isValidPrice(draft.videoDefaultCNY)
    ) {
      return {
        ok: false,
        message:
          'Configure at least one supported video price or a default price',
      }
    }
    const defaultPrice = parseOptionalPrice(draft.videoDefaultCNY)
    return {
      ok: true,
      mode: 'video_matrix',
      config: stringifyModelPricingConfig({
        mode: 'video_matrix',
        unit: 'per_second',
        prices: videoRowsToPrices(validRows),
        ...(defaultPrice === undefined
          ? {}
          : { default_cny_per_second: defaultPrice }),
      }),
    }
  }

  if (values.pricing_config.trim()) {
    try {
      JSON.parse(values.pricing_config)
    } catch {
      return {
        ok: false,
        message: 'Compatibility pricing configuration must be valid JSON',
      }
    }
  }
  return {
    ok: true,
    mode: values.pricing_mode || 'inherit',
    config: values.pricing_config,
  }
}

function isValidPrice(value: string): boolean {
  if (!value.trim()) return false
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0
}

function parseOptionalPrice(value: string): number | undefined {
  return isValidPrice(value) ? Number(value) : undefined
}

function hasDuplicates(values: string[]): boolean {
  return new Set(values).size !== values.length
}
