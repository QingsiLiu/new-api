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
import { memo, useCallback, useMemo, useState } from 'react'
import { Code2, Eye, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import { StaticDataTable } from '@/components/data-table'
import { useUpdateOption } from '../hooks/use-update-option'

const PRICING_OPTION_KEY = 'AsyncSpecPricing'

type AsyncSpecPricingSettingsProps = {
  pricingDefault: string
  readOnly?: boolean
}

type AsyncSpecPricingConfig = {
  currency?: string
  video?: Record<string, VideoModelSpec>
  image?: Record<string, ImageModelSpec>
}

type VideoModelSpec = {
  unit?: string
  resolutions?: Record<string, { cny_per_second?: number }>
  prices?: Record<
    string,
    Record<
      string,
      Record<string, { cny_per_second?: number; unsupported?: boolean }>
    >
  >
  default_cny_per_second?: number
  min_cny?: number
  max_cny?: number
}

type ImageModelSpec = {
  unit?: string
  resolutions?: Record<string, { cny_per_image?: number }>
  qualities?: Record<string, { cny_per_image?: number }>
  default_cny_per_image?: number
}

type VideoRow = {
  id: number
  model: string
  resolution: string
  ratio: string
  mode: string
  supported: boolean
  cnyPerSecond: number
  defaultCNYPerSecond: number
  minCNY: number
  maxCNY: number
}

type ImageRow = {
  id: number
  model: string
  resolution: string
  cnyPerImage: number
  defaultCNYPerImage: number
}

type ParsedSpec = {
  videoRows: VideoRow[]
  imageRows: ImageRow[]
}

type InitialEditorState = ParsedSpec & {
  jsonText: string
  jsonError: string
  nextRowId: number
}

const DEFAULT_SPEC: AsyncSpecPricingConfig = {
  currency: 'CNY',
  video: {},
  image: {},
}

const DEFAULT_VIDEO_RESOLUTION = '720p'
const DEFAULT_VIDEO_RATIO = '16:9'
const DEFAULT_VIDEO_MODE = 'no_video_input'
const DEFAULT_IMAGE_RESOLUTION = '1k'

const VIDEO_RESOLUTION_OPTIONS = ['480p', '720p', '1080p', '2k', '4k']
const VIDEO_RATIO_OPTIONS = ['16:9', '9:16', '4:3', '3:4', '1:1', '21:9']
const VIDEO_MODE_OPTIONS = [
  { value: 'no_video_input', label: 'No video input' },
  { value: 'with_video_input', label: 'With video input' },
  { value: 'text_audio', label: 'Text with audio' },
  { value: 'text_no_audio', label: 'Text without audio' },
  { value: 'image_audio', label: 'Image with audio' },
  { value: 'image_no_audio', label: 'Image without audio' },
]
const VIDEO_STATUS_OPTIONS = [
  { value: 'supported', label: 'Supported' },
  { value: 'unsupported', label: 'Unsupported' },
]
const IMAGE_RESOLUTION_OPTIONS = ['1k', '2k', '4k']

function parseSpecPricing(
  rawValue: string | undefined
): AsyncSpecPricingConfig {
  if (!rawValue) return DEFAULT_SPEC
  try {
    const parsed = JSON.parse(rawValue) as AsyncSpecPricingConfig
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return {
        currency: parsed.currency || 'CNY',
        video: parsed.video || {},
        image: parsed.image || {},
      }
    }
  } catch {
    // fall through to defaults
  }
  return DEFAULT_SPEC
}

function rowsFromSpec(spec: AsyncSpecPricingConfig): ParsedSpec {
  let nextId = 1
  const videoRows: VideoRow[] = []
  const imageRows: ImageRow[] = []

  for (const [model, modelSpec] of Object.entries(spec.video || {})) {
    const prices = modelSpec.prices || {}
    for (const [resolution, ratioPrices] of Object.entries(prices)) {
      for (const [ratio, modePrices] of Object.entries(ratioPrices || {})) {
        for (const [mode, price] of Object.entries(modePrices || {})) {
          videoRows.push({
            id: nextId++,
            model,
            resolution,
            ratio,
            mode,
            supported: !price.unsupported,
            cnyPerSecond: Number(price.cny_per_second) || 0,
            defaultCNYPerSecond: Number(modelSpec.default_cny_per_second) || 0,
            minCNY: Number(modelSpec.min_cny) || 0,
            maxCNY: Number(modelSpec.max_cny) || 0,
          })
        }
      }
    }

    const resolutions = modelSpec.resolutions || {}
    if (Object.keys(prices).length === 0) {
      for (const [resolution, price] of Object.entries(resolutions)) {
        videoRows.push({
          id: nextId++,
          model,
          resolution,
          ratio: DEFAULT_VIDEO_RATIO,
          mode: DEFAULT_VIDEO_MODE,
          supported: true,
          cnyPerSecond: Number(price.cny_per_second) || 0,
          defaultCNYPerSecond: Number(modelSpec.default_cny_per_second) || 0,
          minCNY: Number(modelSpec.min_cny) || 0,
          maxCNY: Number(modelSpec.max_cny) || 0,
        })
      }
    }
    if (
      Object.keys(prices).length === 0 &&
      Object.keys(resolutions).length === 0
    ) {
      videoRows.push({
        id: nextId++,
        model,
        resolution: DEFAULT_VIDEO_RESOLUTION,
        ratio: DEFAULT_VIDEO_RATIO,
        mode: DEFAULT_VIDEO_MODE,
        supported: true,
        cnyPerSecond: 0,
        defaultCNYPerSecond: Number(modelSpec.default_cny_per_second) || 0,
        minCNY: Number(modelSpec.min_cny) || 0,
        maxCNY: Number(modelSpec.max_cny) || 0,
      })
    }
  }

  for (const [model, modelSpec] of Object.entries(spec.image || {})) {
    const resolutions = modelSpec.resolutions || {}
    for (const [resolution, price] of Object.entries(resolutions)) {
      imageRows.push({
        id: nextId++,
        model,
        resolution,
        cnyPerImage: Number(price.cny_per_image) || 0,
        defaultCNYPerImage: Number(modelSpec.default_cny_per_image) || 0,
      })
    }
    if (Object.keys(resolutions).length === 0) {
      imageRows.push({
        id: nextId++,
        model,
        resolution: DEFAULT_IMAGE_RESOLUTION,
        cnyPerImage: 0,
        defaultCNYPerImage: Number(modelSpec.default_cny_per_image) || 0,
      })
    }
  }

  return { videoRows, imageRows }
}

function rowsToSpec(
  videoRows: VideoRow[],
  imageRows: ImageRow[]
): AsyncSpecPricingConfig {
  const video: NonNullable<AsyncSpecPricingConfig['video']> = {}
  const image: NonNullable<AsyncSpecPricingConfig['image']> = {}

  for (const row of videoRows) {
    const model = row.model.trim()
    const resolution = row.resolution.trim()
    const ratio = row.ratio.trim()
    const mode = row.mode.trim()
    if (!model || !resolution || !ratio || !mode) continue
    const spec = video[model] || {
      unit: 'per_second',
      prices: {},
    }
    spec.unit = 'per_second'
    spec.default_cny_per_second = Number(row.defaultCNYPerSecond) || 0
    spec.min_cny = Number(row.minCNY) || 0
    spec.max_cny = Number(row.maxCNY) || 0
    spec.prices = spec.prices || {}
    spec.prices[resolution] = spec.prices[resolution] || {}
    spec.prices[resolution][ratio] = spec.prices[resolution][ratio] || {}
    spec.prices[resolution][ratio][mode] = row.supported
      ? {
          cny_per_second: Number(row.cnyPerSecond) || 0,
        }
      : {
          unsupported: !row.supported,
        }
    video[model] = spec
  }

  for (const row of imageRows) {
    const model = row.model.trim()
    const resolution = row.resolution.trim()
    if (!model || !resolution) continue
    const spec = image[model] || {
      unit: 'per_image',
      resolutions: {},
    }
    spec.unit = 'per_image'
    spec.default_cny_per_image = Number(row.defaultCNYPerImage) || 0
    spec.resolutions = spec.resolutions || {}
    spec.resolutions[resolution] = {
      cny_per_image: Number(row.cnyPerImage) || 0,
    }
    image[model] = spec
  }

  return {
    currency: 'CNY',
    video,
    image,
  }
}

function normalizeNumber(value: string) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0
}

function buildInitialEditorState(pricingDefault: string): InitialEditorState {
  const spec = parseSpecPricing(pricingDefault)
  const rows = rowsFromSpec(spec)
  const rowCount = rows.videoRows.length + rows.imageRows.length
  return {
    ...rows,
    jsonText: JSON.stringify(spec, null, 2),
    jsonError: '',
    nextRowId: rowCount + 1,
  }
}

export const AsyncSpecPricingSettings = memo(function AsyncSpecPricingSettings(
  props: AsyncSpecPricingSettingsProps
) {
  const resetKey = props.pricingDefault
  return <AsyncSpecPricingSettingsInner key={resetKey} {...props} />
})

const AsyncSpecPricingSettingsInner = memo(
  function AsyncSpecPricingSettingsInner({
    pricingDefault,
    readOnly = false,
  }: AsyncSpecPricingSettingsProps) {
    const { t } = useTranslation()
    const updateOption = useUpdateOption()
    const initialState = buildInitialEditorState(pricingDefault)
    const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
    const [videoRows, setVideoRows] = useState<VideoRow[]>(
      () => initialState.videoRows
    )
    const [imageRows, setImageRows] = useState<ImageRow[]>(
      () => initialState.imageRows
    )
    const [jsonText, setJsonText] = useState(() => initialState.jsonText)
    const [jsonError, setJsonError] = useState(() => initialState.jsonError)
    const [nextRowId, setNextRowId] = useState(() => initialState.nextRowId)

    const currentSpec = useMemo(
      () => rowsToSpec(videoRows, imageRows),
      [imageRows, videoRows]
    )

    const currentJson = useMemo(
      () => JSON.stringify(currentSpec, null, 2),
      [currentSpec]
    )

    const syncFromRows = useCallback(
      (nextVideoRows: VideoRow[], nextImageRows: ImageRow[]) => {
        setVideoRows(nextVideoRows)
        setImageRows(nextImageRows)
        setJsonText(
          JSON.stringify(rowsToSpec(nextVideoRows, nextImageRows), null, 2)
        )
        setJsonError('')
      },
      []
    )

    const handleJsonChange = useCallback(
      (text: string) => {
        setJsonText(text)
        try {
          const parsed = JSON.parse(text) as AsyncSpecPricingConfig
          const rows = rowsFromSpec(parsed)
          setVideoRows(rows.videoRows)
          setImageRows(rows.imageRows)
          setNextRowId(rows.videoRows.length + rows.imageRows.length + 1)
          setJsonError('')
        } catch (error) {
          setJsonError(
            error instanceof Error ? error.message : t('Invalid JSON')
          )
        }
      },
      [t]
    )

    const addVideoRow = useCallback(() => {
      const next: VideoRow = {
        id: nextRowId,
        model: '',
        resolution: DEFAULT_VIDEO_RESOLUTION,
        ratio: DEFAULT_VIDEO_RATIO,
        mode: DEFAULT_VIDEO_MODE,
        supported: true,
        cnyPerSecond: 0,
        defaultCNYPerSecond: 0,
        minCNY: 0,
        maxCNY: 0,
      }
      setNextRowId((prev) => prev + 1)
      syncFromRows([...videoRows, next], imageRows)
    }, [imageRows, nextRowId, syncFromRows, videoRows])

    const addImageRow = useCallback(() => {
      const next: ImageRow = {
        id: nextRowId,
        model: '',
        resolution: DEFAULT_IMAGE_RESOLUTION,
        cnyPerImage: 0,
        defaultCNYPerImage: 0,
      }
      setNextRowId((prev) => prev + 1)
      syncFromRows(videoRows, [...imageRows, next])
    }, [imageRows, nextRowId, syncFromRows, videoRows])

    const updateVideoRow = useCallback(
      (id: number, patch: Partial<VideoRow>) => {
        syncFromRows(
          videoRows.map((row) => (row.id === id ? { ...row, ...patch } : row)),
          imageRows
        )
      },
      [imageRows, syncFromRows, videoRows]
    )

    const updateImageRow = useCallback(
      (id: number, patch: Partial<ImageRow>) => {
        syncFromRows(
          videoRows,
          imageRows.map((row) => (row.id === id ? { ...row, ...patch } : row))
        )
      },
      [imageRows, syncFromRows, videoRows]
    )

    const removeVideoRow = useCallback(
      (id: number) => {
        syncFromRows(
          videoRows.filter((row) => row.id !== id),
          imageRows
        )
      },
      [imageRows, syncFromRows, videoRows]
    )

    const removeImageRow = useCallback(
      (id: number) => {
        syncFromRows(
          videoRows,
          imageRows.filter((row) => row.id !== id)
        )
      },
      [imageRows, syncFromRows, videoRows]
    )

    const toggleEditMode = useCallback(() => {
      setEditMode((prev) => {
        const next = prev === 'visual' ? 'json' : 'visual'
        if (next === 'json') {
          setJsonText(currentJson)
          setJsonError('')
        }
        return next
      })
    }, [currentJson])

    const handleSave = useCallback(async () => {
      if (readOnly) return
      if (editMode === 'json' && jsonError) {
        toast.error(t('Please fix JSON errors before saving'))
        return
      }
      const value = editMode === 'json' ? jsonText : currentJson
      await updateOption.mutateAsync({
        key: PRICING_OPTION_KEY,
        value,
      })
    }, [currentJson, editMode, jsonError, jsonText, readOnly, t, updateOption])

    return (
      <div className='space-y-5'>
        <div className='flex flex-wrap justify-end gap-2'>
          <Button variant='outline' size='sm' onClick={toggleEditMode}>
            {editMode === 'visual' ? (
              <>
                <Code2 className='mr-2 h-4 w-4' />
                {t('Switch to JSON')}
              </>
            ) : (
              <>
                <Eye className='mr-2 h-4 w-4' />
                {t('Switch to Visual')}
              </>
            )}
          </Button>
          {!readOnly && (
            <Button
              size='sm'
              onClick={handleSave}
              disabled={
                updateOption.isPending || (editMode === 'json' && !!jsonError)
              }
            >
              {updateOption.isPending ? t('Saving...') : t('Save spec pricing')}
            </Button>
          )}
        </div>

        {editMode === 'json' ? (
          <div className='space-y-2'>
            <Textarea
              value={jsonText}
              onChange={(event) => handleJsonChange(event.target.value)}
              className='min-h-80 font-mono text-sm'
              spellCheck={false}
              disabled={readOnly}
            />
            {jsonError ? (
              <p className='text-destructive text-sm'>{jsonError}</p>
            ) : null}
          </div>
        ) : (
          <div className='space-y-6'>
            <SpecTableHeader
              title={t('Video matrix prices')}
              actionLabel={t('Add video price')}
              onAdd={addVideoRow}
              disabled={readOnly}
            />
            <StaticDataTable
              data={videoRows}
              getRowKey={(row) => row.id}
              emptyClassName='text-muted-foreground py-8'
              emptyContent={t('No video prices configured')}
              columns={[
                {
                  id: 'model',
                  header: t('Model'),
                  cell: (row) => (
                    <Input
                      value={row.model}
                      placeholder='seedance-2.0'
                      disabled={readOnly}
                      onChange={(event) =>
                        updateVideoRow(row.id, { model: event.target.value })
                      }
                    />
                  ),
                },
                {
                  id: 'resolution',
                  header: t('Resolution'),
                  className: 'w-32',
                  cell: (row) => (
                    <NativeSelect
                      className='w-full'
                      value={row.resolution}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateVideoRow(row.id, {
                          resolution: event.target.value,
                        })
                      }
                    >
                      {VIDEO_RESOLUTION_OPTIONS.map((option) => (
                        <NativeSelectOption key={option} value={option}>
                          {option}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  ),
                },
                {
                  id: 'ratio',
                  header: t('Ratio'),
                  className: 'w-28',
                  cell: (row) => (
                    <NativeSelect
                      className='w-full'
                      value={row.ratio}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateVideoRow(row.id, {
                          ratio: event.target.value,
                        })
                      }
                    >
                      {VIDEO_RATIO_OPTIONS.map((option) => (
                        <NativeSelectOption key={option} value={option}>
                          {option}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  ),
                },
                {
                  id: 'mode',
                  header: t('Mode'),
                  className: 'w-52',
                  cell: (row) => (
                    <NativeSelect
                      className='w-full'
                      value={row.mode}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateVideoRow(row.id, {
                          mode: event.target.value,
                        })
                      }
                    >
                      {VIDEO_MODE_OPTIONS.map((option) => (
                        <NativeSelectOption
                          key={option.value}
                          value={option.value}
                        >
                          {t(option.label)}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  ),
                },
                {
                  id: 'status',
                  header: t('Status'),
                  className: 'w-36',
                  cell: (row) => (
                    <NativeSelect
                      className='w-full'
                      value={row.supported ? 'supported' : 'unsupported'}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateVideoRow(row.id, {
                          supported: event.target.value === 'supported',
                        })
                      }
                    >
                      {VIDEO_STATUS_OPTIONS.map((option) => (
                        <NativeSelectOption
                          key={option.value}
                          value={option.value}
                        >
                          {t(option.label)}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  ),
                },
                {
                  id: 'rate',
                  header: t('CNY / second'),
                  className: 'w-36',
                  cell: (row) => (
                    <Input
                      type='number'
                      min={0}
                      step={0.01}
                      value={row.cnyPerSecond}
                      disabled={readOnly || !row.supported}
                      onChange={(event) =>
                        updateVideoRow(row.id, {
                          cnyPerSecond: normalizeNumber(event.target.value),
                        })
                      }
                    />
                  ),
                },
                {
                  id: 'default',
                  header: t('Default'),
                  className: 'w-32',
                  cell: (row) => (
                    <Input
                      type='number'
                      min={0}
                      step={0.01}
                      value={row.defaultCNYPerSecond}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateVideoRow(row.id, {
                          defaultCNYPerSecond: normalizeNumber(
                            event.target.value
                          ),
                        })
                      }
                    />
                  ),
                },
                {
                  id: 'bounds',
                  header: t('Min / max'),
                  className: 'w-44',
                  cell: (row) => (
                    <div className='grid grid-cols-2 gap-2'>
                      <Input
                        type='number'
                        min={0}
                        step={0.01}
                        value={row.minCNY}
                        disabled={readOnly}
                        onChange={(event) =>
                          updateVideoRow(row.id, {
                            minCNY: normalizeNumber(event.target.value),
                          })
                        }
                      />
                      <Input
                        type='number'
                        min={0}
                        step={0.01}
                        value={row.maxCNY}
                        disabled={readOnly}
                        onChange={(event) =>
                          updateVideoRow(row.id, {
                            maxCNY: normalizeNumber(event.target.value),
                          })
                        }
                      />
                    </div>
                  ),
                },
                {
                  id: 'actions',
                  header: t('Actions'),
                  className: 'w-20 text-right',
                  cellClassName: 'text-right',
                  cell: (row) => (
                    <DeleteRowButton
                      onClick={() => removeVideoRow(row.id)}
                      disabled={readOnly}
                    />
                  ),
                },
              ]}
            />

            <SpecTableHeader
              title={t('Image prices')}
              actionLabel={t('Add image price')}
              onAdd={addImageRow}
              disabled={readOnly}
            />
            <StaticDataTable
              data={imageRows}
              getRowKey={(row) => row.id}
              emptyClassName='text-muted-foreground py-8'
              emptyContent={t('No image prices configured')}
              columns={[
                {
                  id: 'model',
                  header: t('Model'),
                  cell: (row) => (
                    <Input
                      value={row.model}
                      placeholder='gpt-image-2'
                      disabled={readOnly}
                      onChange={(event) =>
                        updateImageRow(row.id, { model: event.target.value })
                      }
                    />
                  ),
                },
                {
                  id: 'resolution',
                  header: t('Resolution'),
                  className: 'w-36',
                  cell: (row) => (
                    <NativeSelect
                      className='w-full'
                      value={row.resolution}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateImageRow(row.id, {
                          resolution: event.target.value,
                        })
                      }
                    >
                      {IMAGE_RESOLUTION_OPTIONS.map((option) => (
                        <NativeSelectOption key={option} value={option}>
                          {option}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  ),
                },
                {
                  id: 'rate',
                  header: t('CNY / image'),
                  className: 'w-36',
                  cell: (row) => (
                    <Input
                      type='number'
                      min={0}
                      step={0.01}
                      value={row.cnyPerImage}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateImageRow(row.id, {
                          cnyPerImage: normalizeNumber(event.target.value),
                        })
                      }
                    />
                  ),
                },
                {
                  id: 'default',
                  header: t('Default'),
                  className: 'w-36',
                  cell: (row) => (
                    <Input
                      type='number'
                      min={0}
                      step={0.01}
                      value={row.defaultCNYPerImage}
                      disabled={readOnly}
                      onChange={(event) =>
                        updateImageRow(row.id, {
                          defaultCNYPerImage: normalizeNumber(
                            event.target.value
                          ),
                        })
                      }
                    />
                  ),
                },
                {
                  id: 'actions',
                  header: t('Actions'),
                  className: 'w-20 text-right',
                  cellClassName: 'text-right',
                  cell: (row) => (
                    <DeleteRowButton
                      onClick={() => removeImageRow(row.id)}
                      disabled={readOnly}
                    />
                  ),
                },
              ]}
            />
          </div>
        )}
      </div>
    )
  }
)

function SpecTableHeader({
  title,
  actionLabel,
  onAdd,
  disabled = false,
}: {
  title: string
  actionLabel: string
  onAdd: () => void
  disabled?: boolean
}) {
  return (
    <div className='flex flex-wrap items-center justify-between gap-2'>
      <h4 className='text-sm font-semibold'>{title}</h4>
      <Button variant='outline' size='sm' onClick={onAdd} disabled={disabled}>
        <Plus className='mr-2 h-4 w-4' />
        {actionLabel}
      </Button>
    </div>
  )
}

function DeleteRowButton({
  onClick,
  disabled = false,
}: {
  onClick: () => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  return (
    <Button
      variant='ghost'
      size='icon'
      onClick={onClick}
      disabled={disabled}
      aria-label={t('Delete')}
    >
      <Trash2 className='text-destructive h-4 w-4' />
    </Button>
  )
}
