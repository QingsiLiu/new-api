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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  Loader2,
  Pencil,
  Search,
  Trash2,
} from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'

import {
  getModels,
  getTextPricingConfig,
  searchModels,
  deleteModel,
} from '../api'
import { modelsQueryKeys, normalizeTextPricingGroups } from '../lib'
import { TEXT_PRICING_GROUPS, type Model } from '../types'
import { useModels } from './models-provider'
import { TextPricingGroupRow } from './text-pricing-group-row'

const PENDING_PAGE_SIZE = 10

export function TextPricingCenter() {
  const { t } = useTranslation()
  const configQuery = useQuery({
    queryKey: modelsQueryKeys.textPricing(),
    queryFn: getTextPricingConfig,
  })

  if (configQuery.isLoading) {
    return (
      <div className='text-muted-foreground flex h-full min-h-40 items-center justify-center gap-2 text-sm'>
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading text pricing configuration')}
      </div>
    )
  }

  if (!configQuery.data?.success || !configQuery.data.data) {
    return (
      <Alert variant='destructive'>
        <CircleAlert aria-hidden='true' />
        <AlertTitle>{t('Text pricing configuration unavailable')}</AlertTitle>
        <AlertDescription>
          {configQuery.data?.message ||
            configQuery.error?.message ||
            t('Check the pricing service and try again.')}
        </AlertDescription>
      </Alert>
    )
  }

  const config = configQuery.data.data
  const groups = normalizeTextPricingGroups(config.categories)
  const pendingCount = config.pending_count || 0

  return (
    <div
      className='flex h-full min-h-0 flex-col gap-3 overflow-y-auto pr-1'
      data-testid='text-pricing-center'
    >
      <div className='flex min-w-0 flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <h2 className='text-base font-semibold'>
            {t('Text pricing center')}
          </h2>
          <p className='text-muted-foreground text-xs'>
            {t('Official catalog prices are read-only.')}
          </p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant='outline'>
            {t('Catalog')} {config.catalog_version}
          </Badge>
          <Badge variant={config.activation_ready ? 'outline' : 'destructive'}>
            {config.activation_ready
              ? t('Activation ready')
              : t('Action needed')}
          </Badge>
        </div>
      </div>

      {pendingCount > 0 ? <PendingPricingNotice count={pendingCount} /> : null}

      <div className='border-border overflow-hidden rounded-lg border'>
        <div className='bg-muted/40 hidden grid-cols-[auto_minmax(120px,1.1fr)_minmax(110px,0.8fr)_minmax(150px,0.9fr)_minmax(230px,1fr)] gap-3 border-b px-3 py-2 text-xs font-medium lg:grid'>
          <span aria-hidden='true' />
          <span>{t('Vendor')}</span>
          <span>{t('Billable')}</span>
          <span>{t('Overrides / catalog')}</span>
          <span>{t('Group multiplier')}</span>
        </div>
        {TEXT_PRICING_GROUPS.map((category) => {
          const group = groups.find((entry) => entry.category === category) || {
            category,
          }
          return (
            <TextPricingGroupRow
              key={category}
              group={group as typeof group & { category: typeof category }}
              catalogVersion={config.catalog_version}
            />
          )
        })}
      </div>
    </div>
  )
}

function PendingPricingNotice(props: { count: number }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div className='rounded-lg border border-amber-300 bg-amber-50 p-3 text-amber-950 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-100'>
        <div className='flex min-w-0 items-start gap-3'>
          <CircleAlert className='mt-0.5 size-4 shrink-0' aria-hidden='true' />
          <div className='min-w-0 flex-1'>
            <div className='font-medium'>
              {t('{{count}} text models need pricing metadata', {
                count: props.count,
              })}
            </div>
            <p className='mt-1 text-xs opacity-80'>
              {t(
                'Unclassified or unmatched models stay out of the four pricing groups.'
              )}
            </p>
          </div>
          <CollapsibleTrigger
            render={<Button variant='outline' size='sm' />}
            aria-label={t('View pending pricing models')}
          >
            <Search className='size-4' aria-hidden='true' />
            {t('View details')}
          </CollapsibleTrigger>
        </div>
        <CollapsibleContent className='mt-3 border-t border-current/15 pt-3'>
          <PendingPricingModels open={open} />
        </CollapsibleContent>
      </div>
    </Collapsible>
  )
}

function PendingPricingModels(props: { open: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { setCurrentRow, setOpen } = useModels()
  const [search, setSearch] = useState('')
  const deferredSearch = useDeferredValue(search)
  const [page, setPage] = useState(1)
  const keyword = deferredSearch.trim()
  const params = {
    modal: 'text' as const,
    text_pricing_status: 'pending' as const,
    ...(keyword ? { keyword } : {}),
    p: page,
    page_size: PENDING_PAGE_SIZE,
  }
  const query = useQuery({
    queryKey: modelsQueryKeys.pendingTextPricingModels(params),
    queryFn: () => (keyword ? searchModels(params) : getModels(params)),
    enabled: props.open,
  })
  const models = query.data?.data?.items || []
  const total = query.data?.data?.total || 0
  const totalPages = Math.max(1, Math.ceil(total / PENDING_PAGE_SIZE))
  const deleteMutation = useMutation({
    mutationFn: deleteModel,
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to delete model'))
        return
      }
      toast.success(t('Model deleted successfully'))
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: modelsQueryKeys.textPricing(),
        }),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
      ])
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete model'))
    },
  })

  const renderPendingContent = () => {
    if (query.isLoading) {
      return (
        <div className='flex min-h-20 items-center justify-center gap-2 text-sm'>
          <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          {t('Loading models')}
        </div>
      )
    }
    if (query.isError) {
      return (
        <div className='text-destructive text-sm'>
          {query.error.message || t('Unable to load models')}
        </div>
      )
    }
    if (models.length === 0) {
      return <div className='py-4 text-sm'>{t('No pending models found')}</div>
    }
    return (
      <div className='divide-y divide-current/15'>
        {models.map((model) => (
          <PendingPricingModelRow
            key={model.id}
            model={model}
            deleting={deleteMutation.isPending}
            onDelete={() => deleteMutation.mutate(model.id)}
            onEdit={() => {
              setCurrentRow(model)
              setOpen('update-model')
            }}
          />
        ))}
      </div>
    )
  }

  return (
    <div className='min-w-0'>
      <div className='mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <span className='text-xs font-medium'>{t('Pending metadata')}</span>
        <div className='relative w-full sm:w-64'>
          <Search className='pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 opacity-70' />
          <Input
            value={search}
            onChange={(event) => {
              setSearch(event.target.value)
              setPage(1)
            }}
            placeholder={t('Search models...')}
            aria-label={t('Search pending pricing models')}
            className='border-current/25 pl-9'
          />
        </div>
      </div>
      {renderPendingContent()}
      {total > PENDING_PAGE_SIZE ? (
        <div className='mt-3 flex items-center justify-between gap-2 text-xs'>
          <span>
            {t('Page {{current}} of {{total}}', {
              current: page,
              total: totalPages,
            })}
          </span>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page <= 1 || query.isFetching}
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
              disabled={page >= totalPages || query.isFetching}
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

function PendingPricingModelRow(props: {
  model: Model
  deleting: boolean
  onDelete: () => void
  onEdit: () => void
}) {
  const { t } = useTranslation()
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

  return (
    <div className='flex min-w-0 flex-col gap-2 py-2 sm:flex-row sm:items-center sm:justify-between'>
      <div className='min-w-0'>
        <div className='font-mono text-xs font-medium break-all'>
          {props.model.model_name}
        </div>
        <div className='mt-1 text-xs break-words opacity-75'>
          {props.model.pricing_error ||
            t('Official profile or category is missing.')}
        </div>
      </div>
      <div className='flex shrink-0 gap-2 self-start sm:self-auto'>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={props.onEdit}
        >
          <Pencil className='size-4' aria-hidden='true' />
          {t('Fix metadata')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='destructive'
          onClick={() => setDeleteConfirmOpen(true)}
        >
          <Trash2 className='size-4' aria-hidden='true' />
          {t('Delete')}
        </Button>
      </div>
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={t('Delete Model')}
        desc={t(
          'Are you sure you want to delete model "{{name}}"? This action cannot be undone.',
          {
            name: props.model.model_name,
          }
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={props.deleting}
        handleConfirm={() => {
          props.onDelete()
          setDeleteConfirmOpen(false)
        }}
      />
    </div>
  )
}
