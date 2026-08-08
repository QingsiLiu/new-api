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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  createGroupRegistry,
  deleteGroupRegistry,
  getGroupRegistry,
  updateGroupRegistry,
} from '../api'
import type { GroupRegistryItem } from '../types'
import { normalizeGroupRegistryItems } from '../utils'

const GROUP_FORM_ID = 'group-registry-form'

const createGroupSchema = (t: (key: string) => string) =>
  z.object({
    displayName: z.string().trim().min(1, t('Value is required')),
    description: z.string().trim().max(255),
    ratio: z.number().positive(t('Amount must be greater than 0')),
    userUsable: z.boolean(),
    sort: z.number().int(),
  })

type GroupFormValues = z.infer<ReturnType<typeof createGroupSchema>>

type GroupManagementDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function formatDeleteReferences(references?: Record<string, number>) {
  if (!references) return ''
  return Object.entries(references)
    .map(([key, value]) => `${key}: ${value}`)
    .join(', ')
}

export function GroupManagementDialog(props: GroupManagementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingGroup, setEditingGroup] = useState<GroupRegistryItem | null>(
    null
  )
  const [deleteTarget, setDeleteTarget] = useState<GroupRegistryItem | null>(
    null
  )
  const groupSchema = useMemo(() => createGroupSchema(t), [t])
  const form = useForm<GroupFormValues>({
    resolver: zodResolver(groupSchema),
    defaultValues: {
      displayName: '',
      description: '',
      ratio: 1,
      userUsable: false,
      sort: 0,
    },
  })

  const groupsQuery = useQuery({
    queryKey: ['group-registry', 'admin'],
    queryFn: getGroupRegistry,
    enabled: props.open,
  })
  const groups = useMemo(
    () => normalizeGroupRegistryItems(groupsQuery.data),
    [groupsQuery.data]
  )

  const saveMutation = useMutation({
    mutationFn: async (values: GroupFormValues) => {
      const data = {
        display_name: values.displayName,
        description: values.description,
        ratio: values.ratio,
        user_usable: values.userUsable,
        sort: values.sort,
      }
      return editingGroup
        ? updateGroupRegistry(editingGroup.code, data)
        : createGroupRegistry(data)
    },
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save'))
        return
      }
      toast.success(t(editingGroup ? 'Updated successfully' : 'Group created'))
      await queryClient.invalidateQueries({ queryKey: ['group-registry'] })
      setEditorOpen(false)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (group: GroupRegistryItem) => {
      const response = await deleteGroupRegistry(group.code)
      return { group, response }
    },
    onSuccess: async ({ response }) => {
      if (!response.success) {
        const references = formatDeleteReferences(response.data)
        const message = references
          ? `${response.message || t('Failed to delete group')}: ${references}`
          : response.message || t('Failed to delete group')
        toast.error(message)
        return
      }
      toast.success(t('Deleted successfully'))
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['group-registry'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete group'))
    },
  })

  useEffect(() => {
    if (!editorOpen) return
    form.reset({
      displayName: editingGroup?.display_name ?? '',
      description: editingGroup?.description ?? '',
      ratio: editingGroup?.ratio ?? 1,
      userUsable: editingGroup?.user_usable ?? false,
      sort: editingGroup?.sort ?? 0,
    })
  }, [editingGroup, editorOpen, form])

  const openCreate = () => {
    setEditingGroup(null)
    setEditorOpen(true)
  }

  const openEdit = (group: GroupRegistryItem) => {
    setEditingGroup(group)
    setEditorOpen(true)
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t('Groups')}
        description={t(
          'Edit billing ratios and user-selectable groups in one table.'
        )}
        contentClassName='sm:max-w-5xl'
        contentHeight='min(68vh, 680px)'
      >
        <div className='space-y-4'>
          <div className='flex justify-end'>
            <Button size='sm' onClick={openCreate}>
              <Plus className='h-4 w-4' />
              {t('Add group')}
            </Button>
          </div>

          {groupsQuery.isLoading ? (
            <div className='text-muted-foreground flex min-h-40 items-center justify-center gap-2 text-sm'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('Loading...')}
            </div>
          ) : (
            <StaticDataTable
              data={groups}
              getRowKey={(group) => group.code}
              emptyContent={t('No data')}
              columns={[
                {
                  id: 'name',
                  header: t('Group name'),
                  cell: (group) => (
                    <div className='min-w-0'>
                      <div className='flex items-center gap-2'>
                        <span className='truncate font-medium'>
                          {group.display_name}
                        </span>
                        {group.is_reserved && (
                          <Badge variant='outline'>{t('System')}</Badge>
                        )}
                      </div>
                      <div className='text-muted-foreground truncate font-mono text-xs'>
                        {group.code}
                      </div>
                    </div>
                  ),
                },
                {
                  id: 'description',
                  header: t('Description'),
                  cell: (group) => group.description || '-',
                },
                {
                  id: 'ratio',
                  header: t('Multiplier'),
                  className: 'w-24',
                  cell: (group) => group.ratio.toFixed(4).replace(/\.?0+$/, ''),
                },
                {
                  id: 'availability',
                  header: t('Selectable groups'),
                  className: 'w-36',
                  cell: (group) =>
                    group.user_usable ? t('Enabled') : t('Disabled'),
                },
                {
                  id: 'actions',
                  header: t('Actions'),
                  className: 'w-24 text-right',
                  cellClassName: 'text-right',
                  cell: (group) => (
                    <div className='flex justify-end gap-1'>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => openEdit(group)}
                        aria-label={t('Edit group')}
                      >
                        <Pencil className='h-4 w-4' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => setDeleteTarget(group)}
                        disabled={group.is_reserved}
                        aria-label={t('Delete group')}
                      >
                        <Trash2 className='text-destructive h-4 w-4' />
                      </Button>
                    </div>
                  ),
                },
              ]}
            />
          )}
        </div>
      </Dialog>

      <Dialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        title={editingGroup ? t('Edit group') : t('Add group')}
        contentClassName='sm:max-w-xl'
        contentHeight='auto'
        footer={
          <>
            <Button variant='outline' onClick={() => setEditorOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              form={GROUP_FORM_ID}
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending && (
                <Loader2 className='h-4 w-4 animate-spin' />
              )}
              {t('Save changes')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={GROUP_FORM_ID}
            onSubmit={form.handleSubmit((values) =>
              saveMutation.mutate(values)
            )}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='displayName'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Group name')}</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='off' />
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
                    <Textarea {...field} rows={3} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='ratio'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Multiplier')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0.0001'
                        step='0.0001'
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'A billing multiplier. Lower ratios mean lower API call costs.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='sort'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Sort')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='1'
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='userUsable'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4 rounded-md border p-3'>
                  <div className='space-y-1'>
                    <FormLabel>{t('Selectable groups')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Users only see groups marked as user selectable. Non-selectable groups can still be assigned by administrators.'
                      )}
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
          </form>
        </Form>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Delete group')}
        desc={t(
          'Are you sure you want to delete group "{{name}}"? This action cannot be undone.',
          {
            name: deleteTarget?.display_name || '',
          }
        )}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget)
        }}
        confirmText={t('Delete')}
      />
    </>
  )
}
