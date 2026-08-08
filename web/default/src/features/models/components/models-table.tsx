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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { getModels, searchModels, getVendors } from '../api'
import {
  DEFAULT_PAGE_SIZE,
  getModelStatusOptions,
  getSyncStatusOptions,
} from '../constants'
import { modelsQueryKeys, type ModelModal, vendorsQueryKeys } from '../lib'
import type { TextModelCategory } from '../types'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { ModelCategoryTabs } from './model-category-tabs'
import { useModelsColumns } from './models-columns'
import { useModels } from './models-provider'
import { TextPricingCategoryPanel } from './text-pricing-category-panel'

const route = getRouteApi('/_authenticated/models/$section')

export function ModelsTable() {
  const { t } = useTranslation()
  const { selectedVendor } = useModels()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [modal, setModal] = useState<ModelModal>('text')
  const [textCategory, setTextCategory] = useState<TextModelCategory>('gpt')

  // URL state management
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: {
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : DEFAULT_PAGE_SIZE,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'vendor_id', searchKey: 'vendor', type: 'array' },
      { columnId: 'sync_official', searchKey: 'sync', type: 'array' },
    ],
  })

  // Extract filters from column filters
  const statusFilter =
    (columnFilters.find((f) => f.id === 'status')?.value as string[]) || []
  const vendorFilter =
    (columnFilters.find((f) => f.id === 'vendor_id')?.value as string[]) || []
  const syncFilter =
    (columnFilters.find((f) => f.id === 'sync_official')?.value as string[]) ||
    []

  // Fetch vendors for filter
  const { data: vendorsData } = useQuery({
    queryKey: vendorsQueryKeys.list(),
    queryFn: () => getVendors({ page_size: 1000 }),
  })

  const vendors = useMemo(
    () => vendorsData?.data?.items || [],
    [vendorsData?.data?.items]
  )

  const vendorOptions = useMemo(() => {
    return vendors.map((v) => ({
      label: v.name,
      value: String(v.id),
    }))
  }, [vendors])

  // Apply selected vendor from context or filter
  const activeVendorFilter =
    selectedVendor ||
    (vendorFilter.length > 0 && !vendorFilter.includes('all')
      ? vendorFilter[0]
      : undefined)

  const statusFilterValue =
    statusFilter.length > 0 && !statusFilter.includes('all')
      ? statusFilter[0]
      : undefined
  const syncFilterValue =
    syncFilter.length > 0 && !syncFilter.includes('all')
      ? syncFilter[0]
      : undefined

  // Use search API whenever any filter is active so status/sync are applied server-side
  const shouldSearch = Boolean(globalFilter?.trim())
  const activeTextCategory = modal === 'text' ? textCategory : undefined

  // Fetch models data
  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const { data, isLoading, isFetching } = useQuery({
    queryKey: modelsQueryKeys.list({
      keyword: globalFilter,
      vendor: activeVendorFilter,
      status: statusFilterValue,
      sync_official: syncFilterValue,
      modal,
      text_category: activeTextCategory,
      p: pagination.pageIndex + 1,
      page_size: pagination.pageSize,
    }),
    queryFn: async () => {
      if (shouldSearch) {
        return searchModels({
          keyword: globalFilter,
          vendor: activeVendorFilter,
          status: statusFilterValue,
          sync_official: syncFilterValue,
          modal,
          text_category: activeTextCategory,
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
        })
      }
      return getModels({
        vendor: activeVendorFilter,
        status: statusFilterValue,
        sync_official: syncFilterValue,
        modal,
        text_category: activeTextCategory,
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
    },
  })

  const models = data?.data?.items || []
  const totalCount = data?.data?.total || 0
  const vendorCounts = data?.data?.vendor_counts

  // Columns configuration
  const columns = useModelsColumns(vendors)

  // React Table instance
  const { table } = useDataTable({
    data: models,
    columns,
    totalCount,
    initialColumnVisibility: {
      name_rule: false,
      description: false,
      tags: false,
      endpoints: false,
      bound_channels: false,
      enable_groups: false,
      quota_types: false,
      sync_official: false,
      created_time: false,
      updated_time: false,
    },
    columnFilters,
    pagination,
    globalFilter,
    enableRowSelection: true,
    onColumnFiltersChange,
    onPaginationChange,
    onGlobalFilterChange,
    manualPagination: true,
    manualSorting: true,
    manualFiltering: true,
    ensurePageInRange,
  })

  // Prepare filter options
  const vendorFilterOptions = [
    {
      label: `${t('All Vendors')}${vendorCounts?.all ? ` (${vendorCounts.all})` : ''}`,
      value: 'all',
    },
    ...vendorOptions.map((option) => ({
      label: `${option.label}${vendorCounts?.[option.value] ? ` (${vendorCounts[option.value]})` : ''}`,
      value: option.value,
    })),
  ]

  const resetPagination = () => {
    if (pagination.pageIndex === 0) return
    onPaginationChange({ ...pagination, pageIndex: 0 })
  }

  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <ModelCategoryTabs
        modal={modal}
        textCategory={textCategory}
        onModalChange={(value) => {
          setModal(value)
          resetPagination()
        }}
        onTextCategoryChange={(value) => {
          setTextCategory(value)
          resetPagination()
        }}
      />

      {modal === 'text' ? (
        <TextPricingCategoryPanel category={textCategory} />
      ) : null}

      <div className='min-h-0 flex-1'>
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          isFetching={isFetching}
          emptyTitle={t('No Models Found')}
          emptyDescription={t(
            'No models available in this category. Add or reclassify a model to get started.'
          )}
          skeletonKeyPrefix='model-skeleton'
          applyHeaderSize
          toolbarProps={{
            searchPlaceholder: t('Filter by model name...'),
            filters: [
              {
                columnId: 'status',
                title: t('Status'),
                options: [...getModelStatusOptions(t)],
                singleSelect: true,
              },
              {
                columnId: 'vendor_id',
                title: t('Vendor'),
                options: vendorFilterOptions,
                singleSelect: true,
              },
              {
                columnId: 'sync_official',
                title: t('Official Sync'),
                options: [...getSyncStatusOptions(t)],
                singleSelect: true,
              },
            ],
          }}
          bulkActions={<DataTableBulkActions table={table} />}
        />
      </div>
    </div>
  )
}
