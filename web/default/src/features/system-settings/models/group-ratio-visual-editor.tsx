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
import { useState, useMemo, useEffect, useCallback, memo } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2, GripVertical, ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { deferEffect } from '@/lib/defer-effect'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { StaticDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { createGroupRegistry, updateGroupRegistry } from '@/features/groups/api'
import { GroupSelect } from '@/features/groups/components/group-select'
import { useGroupRegistry } from '@/features/groups/hooks/use-group-registry'
import { safeJsonParse } from '../utils/json-parser'

type GroupRatioVisualEditorProps = {
  groupRatio: string
  topupGroupRatio: string
  userUsableGroups: string
  groupGroupRatio: string
  autoGroups: string
  onChange: (field: string, value: string) => void
}

type SimpleGroup = {
  name: string
  displayName?: string
  value: string
}

type GroupPricingRow = {
  _id: string
  name: string
  ratio: number
  selectable: boolean
  description: string
  displayName?: string
}

type GroupOverride = {
  targetGroup: string
  ratio: number
}

const sectionCardClassName =
  'relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border before:border-border/90'
const sectionHeaderClassName = 'border-b bg-muted/20'

let groupPricingIdCounter = 0
function createGroupPricingId() {
  groupPricingIdCounter += 1
  return `gpr_${groupPricingIdCounter}`
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function buildGroupPricingRows(
  groupRatio: string,
  userUsableGroups: string,
  getDisplayName?: (code?: string | null) => string
): GroupPricingRow[] {
  const ratioMap = safeJsonParse<Record<string, number>>(groupRatio, {
    fallback: {},
    context: 'group ratios',
  })
  const usableMap = safeJsonParse<Record<string, string>>(userUsableGroups, {
    fallback: {},
    context: 'user usable groups',
  })
  const names = new Set([...Object.keys(ratioMap), ...Object.keys(usableMap)])

  return Array.from(names).map((name) => ({
    _id: createGroupPricingId(),
    name,
    ratio: normalizeRatio(ratioMap[name]),
    selectable: Object.prototype.hasOwnProperty.call(usableMap, name),
    description: String(usableMap[name] ?? ''),
    displayName: getDisplayName?.(name) ?? name,
  }))
}

function serializeGroupPricingRows(rows: GroupPricingRow[]) {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeRatio(row.ratio)
    if (row.selectable) {
      userUsableGroups[name] = row.description
    }
  }

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
  }
}

function groupPricingSignature(rows: GroupPricingRow[]): string {
  const serialized = serializeGroupPricingRows(rows)
  return JSON.stringify({
    groupRatio: safeJsonParse(serialized.GroupRatio, {
      fallback: {},
      silent: true,
    }),
    userUsableGroups: safeJsonParse(serialized.UserUsableGroups, {
      fallback: {},
      silent: true,
    }),
  })
}

function sourceGroupPricingSignature(
  groupRatio: string,
  userUsableGroups: string
): string {
  return JSON.stringify({
    groupRatio: safeJsonParse(groupRatio, { fallback: {}, silent: true }),
    userUsableGroups: safeJsonParse(userUsableGroups, {
      fallback: {},
      silent: true,
    }),
  })
}

export const GroupRatioVisualEditor = memo(function GroupRatioVisualEditor({
  groupRatio,
  topupGroupRatio,
  userUsableGroups,
  groupGroupRatio,
  autoGroups,
  onChange,
}: GroupRatioVisualEditorProps) {
  const { t } = useTranslation()
  const [simpleDialogOpen, setSimpleDialogOpen] = useState(false)
  const [simpleDialogType, setSimpleDialogType] = useState<
    'groupRatio' | 'topupGroupRatio' | null
  >(null)
  const [simpleEditData, setSimpleEditData] = useState<SimpleGroup | null>(null)

  const [autoGroupDialogOpen, setAutoGroupDialogOpen] = useState(false)
  const [autoGroupInput, setAutoGroupInput] = useState('')

  const [groupOverrideDialogOpen, setGroupOverrideDialogOpen] = useState(false)
  const [groupOverrideUserGroup, setGroupOverrideUserGroup] = useState<
    string | null
  >(null)
  const [groupOverrideEditData, setGroupOverrideEditData] =
    useState<GroupOverride | null>(null)

  const [userGroupDialogOpen, setUserGroupDialogOpen] = useState(false)
  const [userGroupInput, setUserGroupInput] = useState('')
  const groupRegistry = useGroupRegistry()
  const getGroupDisplayName = groupRegistry.getDisplayName

  // Parse topup group ratios
  const topupRatioList = useMemo(() => {
    const map = safeJsonParse<Record<string, number>>(topupGroupRatio, {
      fallback: {},
      context: 'topup group ratios',
    })
    return Object.entries(map).map(([name, value]) => ({
      name,
      displayName: getGroupDisplayName(name),
      value: String(value),
    }))
  }, [getGroupDisplayName, topupGroupRatio])

  // Parse auto groups
  const autoGroupsList = useMemo(() => {
    return safeJsonParse<string[]>(autoGroups, {
      fallback: [],
      context: 'auto groups',
    })
  }, [autoGroups])

  // Parse group-group ratios
  const groupGroupRatioList = useMemo(() => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        context: 'group-group ratios',
      }
    )
    return Object.entries(map).map(([userGroup, overrides]) => ({
      userGroup,
      userGroupDisplayName: getGroupDisplayName(userGroup),
      overrides: Object.entries(overrides).map(([targetGroup, ratio]) => ({
        targetGroup,
        targetGroupDisplayName: getGroupDisplayName(targetGroup),
        ratio,
      })),
    }))
  }, [getGroupDisplayName, groupGroupRatio])

  // Simple group handlers (for groupRatio and topupGroupRatio)
  const handleSimpleAdd = (type: 'groupRatio' | 'topupGroupRatio') => {
    setSimpleDialogType(type)
    setSimpleEditData(null)
    setSimpleDialogOpen(true)
  }

  const handleSimpleEdit = (
    type: 'groupRatio' | 'topupGroupRatio',
    group: SimpleGroup
  ) => {
    setSimpleDialogType(type)
    setSimpleEditData(group)
    setSimpleDialogOpen(true)
  }

  const handleSimpleSave = (name: string, value: string) => {
    if (!simpleDialogType) return

    const fieldName =
      simpleDialogType === 'groupRatio' ? groupRatio : topupGroupRatio
    const map = safeJsonParse<Record<string, number>>(fieldName, {
      fallback: {},
      silent: true,
    })

    if (simpleEditData && simpleEditData.name !== name) {
      delete map[simpleEditData.name]
    }

    map[name] = parseFloat(value)

    const field =
      simpleDialogType === 'groupRatio' ? 'GroupRatio' : 'TopupGroupRatio'
    onChange(field, JSON.stringify(map, null, 2))
    setSimpleDialogOpen(false)
  }

  const handleSimpleDelete = (
    type: 'groupRatio' | 'topupGroupRatio',
    name: string
  ) => {
    const fieldName = type === 'groupRatio' ? groupRatio : topupGroupRatio
    const map = safeJsonParse<Record<string, number>>(fieldName, {
      fallback: {},
      silent: true,
    })
    delete map[name]

    const field = type === 'groupRatio' ? 'GroupRatio' : 'TopupGroupRatio'
    onChange(field, JSON.stringify(map, null, 2))
  }

  // Auto groups handlers
  const handleAutoGroupAdd = () => {
    setAutoGroupInput('')
    setAutoGroupDialogOpen(true)
  }

  const handleAutoGroupSave = () => {
    if (!autoGroupInput.trim()) return

    const list = [...autoGroupsList, autoGroupInput.trim()]
    onChange('AutoGroups', JSON.stringify(list, null, 2))
    setAutoGroupDialogOpen(false)
  }

  const handleAutoGroupDelete = (index: number) => {
    const list = autoGroupsList.filter((_, i) => i !== index)
    onChange('AutoGroups', JSON.stringify(list, null, 2))
  }

  const handleAutoGroupMove = (index: number, direction: 'up' | 'down') => {
    const list = [...autoGroupsList]
    const newIndex = direction === 'up' ? index - 1 : index + 1

    if (newIndex < 0 || newIndex >= list.length) return
    ;[list[index], list[newIndex]] = [list[newIndex], list[index]]
    onChange('AutoGroups', JSON.stringify(list, null, 2))
  }

  // Group-group ratio handlers
  const handleUserGroupAdd = () => {
    setUserGroupInput('')
    setUserGroupDialogOpen(true)
  }

  const handleUserGroupSave = () => {
    if (!userGroupInput.trim()) return

    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (!map[userGroupInput.trim()]) {
      map[userGroupInput.trim()] = {}
    }

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    setUserGroupDialogOpen(false)
  }

  const handleUserGroupDelete = (userGroup: string) => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )
    delete map[userGroup]
    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
  }

  const handleOverrideAdd = (userGroup: string) => {
    setGroupOverrideUserGroup(userGroup)
    setGroupOverrideEditData(null)
    setGroupOverrideDialogOpen(true)
  }

  const handleOverrideEdit = (userGroup: string, override: GroupOverride) => {
    setGroupOverrideUserGroup(userGroup)
    setGroupOverrideEditData(override)
    setGroupOverrideDialogOpen(true)
  }

  const handleOverrideSave = (
    targetGroup: string,
    ratio: number,
    oldTargetGroup?: string
  ) => {
    if (!groupOverrideUserGroup) return

    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (!map[groupOverrideUserGroup]) {
      map[groupOverrideUserGroup] = {}
    }

    if (oldTargetGroup && oldTargetGroup !== targetGroup) {
      delete map[groupOverrideUserGroup][oldTargetGroup]
    }

    map[groupOverrideUserGroup][targetGroup] = ratio

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    setGroupOverrideDialogOpen(false)
  }

  const handleOverrideDelete = (userGroup: string, targetGroup: string) => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (map[userGroup]) {
      delete map[userGroup][targetGroup]
      if (Object.keys(map[userGroup]).length === 0) {
        delete map[userGroup]
      }
    }

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
  }

  return (
    <div className='space-y-4'>
      <GroupPricingTable
        groupRatio={groupRatio}
        userUsableGroups={userUsableGroups}
        onChange={onChange}
        getDisplayName={getGroupDisplayName}
      />

      {/* Topup Group Ratios */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Top-up group ratios')}</CardTitle>
          <CardDescription>
            {t('Multipliers for recharge pricing based on user groups.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Button
              onClick={() => handleSimpleAdd('topupGroupRatio')}
              size='sm'
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add group')}
            </Button>
            {topupRatioList.length > 0 && (
              <StaticDataTable
                data={topupRatioList}
                getRowKey={(group) => group.name}
                columns={[
                  {
                    id: 'group',
                    header: t('Group name'),
                    cellClassName: 'font-medium',
                    cell: (group) => group.displayName,
                  },
                  {
                    id: 'multiplier',
                    header: t('Multiplier'),
                    cell: (group) => group.value,
                  },
                  {
                    id: 'actions',
                    header: t('Actions'),
                    className: 'text-right',
                    cellClassName: 'text-right',
                    cell: (group) => (
                      <div className='flex justify-end gap-2'>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() =>
                            handleSimpleEdit('topupGroupRatio', group)
                          }
                        >
                          <Pencil className='h-4 w-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() =>
                            handleSimpleDelete('topupGroupRatio', group.name)
                          }
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    ),
                  },
                ]}
              />
            )}
          </div>
        </CardContent>
      </Card>

      {/* Inter-group ratio overrides */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Inter-group ratio overrides')}</CardTitle>
          <CardDescription>
            {t(
              'Custom multipliers when specific user groups use specific token groups. Example: VIP users get 0.9x rate when using "edit_this" group tokens.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Button onClick={handleUserGroupAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add user group')}
            </Button>
            {groupGroupRatioList.length > 0 && (
              <div className='space-y-3'>
                {groupGroupRatioList.map((userGroupData) => (
                  <Collapsible key={userGroupData.userGroup}>
                    <div className='rounded-lg border'>
                      <div className='flex items-center justify-between p-4'>
                        <div className='flex items-center gap-2'>
                          <CollapsibleTrigger
                            render={<Button variant='ghost' size='sm' />}
                          >
                            <ChevronDown className='h-4 w-4' />
                          </CollapsibleTrigger>
                          <span className='font-semibold'>
                            {userGroupData.userGroupDisplayName}
                          </span>
                          <span className='text-muted-foreground text-sm'>
                            {t('{{count}} override', {
                              count: userGroupData.overrides.length,
                            })}
                          </span>
                        </div>
                        <div className='flex gap-2'>
                          <Button
                            variant='ghost'
                            size='sm'
                            onClick={() =>
                              handleOverrideAdd(userGroupData.userGroup)
                            }
                          >
                            <Plus className='h-4 w-4' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='sm'
                            onClick={() =>
                              handleUserGroupDelete(userGroupData.userGroup)
                            }
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      </div>
                      <CollapsibleContent>
                        {userGroupData.overrides.length > 0 && (
                          <div className='border-t'>
                            <StaticDataTable
                              className='rounded-none border-0'
                              data={userGroupData.overrides}
                              getRowKey={(override) => override.targetGroup}
                              columns={[
                                {
                                  id: 'target-group',
                                  header: t('Target group'),
                                  cellClassName: 'font-medium',
                                  cell: (override) =>
                                    override.targetGroupDisplayName,
                                },
                                {
                                  id: 'ratio',
                                  header: t('Ratio'),
                                  cell: (override) => override.ratio,
                                },
                                {
                                  id: 'actions',
                                  header: t('Actions'),
                                  className: 'text-right',
                                  cellClassName: 'text-right',
                                  cell: (override) => (
                                    <div className='flex justify-end gap-2'>
                                      <Button
                                        variant='ghost'
                                        size='sm'
                                        onClick={() =>
                                          handleOverrideEdit(
                                            userGroupData.userGroup,
                                            override
                                          )
                                        }
                                      >
                                        <Pencil className='h-4 w-4' />
                                      </Button>
                                      <Button
                                        variant='ghost'
                                        size='sm'
                                        onClick={() =>
                                          handleOverrideDelete(
                                            userGroupData.userGroup,
                                            override.targetGroup
                                          )
                                        }
                                      >
                                        <Trash2 className='h-4 w-4' />
                                      </Button>
                                    </div>
                                  ),
                                },
                              ]}
                            />
                          </div>
                        )}
                      </CollapsibleContent>
                    </div>
                  </Collapsible>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Auto Groups */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Auto assignment order')}</CardTitle>
          <CardDescription>
            {t(
              'Priority order for automatic group assignment. New tokens rotate through this list.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Button onClick={handleAutoGroupAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add group')}
            </Button>
            {autoGroupsList.length > 0 && (
              <div className='space-y-2'>
                {autoGroupsList.map((group, index) => (
                  <div
                    key={index}
                    className='flex items-center gap-2 rounded-md border p-3'
                  >
                    <GripVertical className='text-muted-foreground h-4 w-4' />
                    <span className='flex-1 font-medium'>
                      {getGroupDisplayName(group)}
                    </span>
                    <div className='flex gap-1'>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={index === 0}
                        onClick={() => handleAutoGroupMove(index, 'up')}
                      >
                        ↑
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={index === autoGroupsList.length - 1}
                        onClick={() => handleAutoGroupMove(index, 'down')}
                      >
                        ↓
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => handleAutoGroupDelete(index)}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Simple Group Dialog */}
      <SimpleGroupDialog
        open={simpleDialogOpen}
        onOpenChange={setSimpleDialogOpen}
        onSave={handleSimpleSave}
        editData={simpleEditData}
        type={simpleDialogType}
        getDisplayName={getGroupDisplayName}
      />

      {/* Auto Group Dialog */}
      <Dialog
        open={autoGroupDialogOpen}
        onOpenChange={setAutoGroupDialogOpen}
        title={t('Add auto group')}
        description={t('Choose a group for the auto assignment list.')}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setAutoGroupDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleAutoGroupSave}>{t('Add')}</Button>
          </>
        }
      >
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('Group')}</Label>
            <GroupSelect
              value={autoGroupInput}
              onValueChange={setAutoGroupInput}
              placeholder={t('Select a group')}
            />
          </div>
        </div>
      </Dialog>

      {/* User Group Dialog */}
      <Dialog
        open={userGroupDialogOpen}
        onOpenChange={setUserGroupDialogOpen}
        title={t('Add user group')}
        description={t(
          'Create a new user group to configure ratio overrides for.'
        )}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setUserGroupDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleUserGroupSave}>{t('Add')}</Button>
          </>
        }
      >
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('User group')}</Label>
            <GroupSelect
              value={userGroupInput}
              onValueChange={setUserGroupInput}
              placeholder={t('Select a group')}
            />
          </div>
        </div>
      </Dialog>

      {/* Group Override Dialog */}
      <GroupOverrideDialog
        open={groupOverrideDialogOpen}
        onOpenChange={setGroupOverrideDialogOpen}
        onSave={handleOverrideSave}
        editData={groupOverrideEditData}
        userGroup={groupOverrideUserGroup}
        getDisplayName={getGroupDisplayName}
      />
    </div>
  )
})

type GroupPricingTableProps = {
  groupRatio: string
  userUsableGroups: string
  onChange: (field: string, value: string) => void
  getDisplayName: (code?: string | null) => string
}

function GroupPricingTable({
  groupRatio,
  userUsableGroups,
  onChange,
  getDisplayName,
}: GroupPricingTableProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [rows, setRows] = useState<GroupPricingRow[]>(() =>
    buildGroupPricingRows(groupRatio, userUsableGroups, getDisplayName)
  )
  const createMutation = useMutation({
    mutationFn: createGroupRegistry,
    onSuccess: async (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to create group'))
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['group-registry'] })
      const created = response.data
      emitRows([
        ...rows,
        {
          _id: createGroupPricingId(),
          name: created.code,
          ratio: normalizeRatio(created.ratio),
          selectable: created.user_usable,
          description: created.description,
          displayName: created.display_name || created.code,
        },
      ])
      setCreateDialogOpen(false)
      toast.success(t('Group created'))
    },
    onError: () => {
      toast.error(t('Failed to create group'))
    },
  })
  const { mutate: renameGroup } = useMutation({
    mutationFn: (data: { row: GroupPricingRow; displayName: string }) =>
      updateGroupRegistry(data.row.name, {
        display_name: data.displayName,
        description: data.row.description,
        ratio: data.row.ratio,
        user_usable: data.row.selectable,
      }),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to update group'))
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['group-registry'] })
    },
    onError: () => {
      toast.error(t('Failed to update group'))
    },
  })

  useEffect(() => {
    return deferEffect(() => {
      const incomingSignature = sourceGroupPricingSignature(
        groupRatio,
        userUsableGroups
      )
      setRows((currentRows) => {
        if (groupPricingSignature(currentRows) === incomingSignature) {
          return currentRows.map((row) => ({
            ...row,
            displayName: getDisplayName(row.name),
          }))
        }
        return buildGroupPricingRows(groupRatio, userUsableGroups, getDisplayName)
      })
    })
  }, [getDisplayName, groupRatio, userUsableGroups])

  const emitRows = useCallback(
    (nextRows: GroupPricingRow[]) => {
      setRows(nextRows)
      const serialized = serializeGroupPricingRows(nextRows)
      onChange('GroupRatio', serialized.GroupRatio)
      onChange('UserUsableGroups', serialized.UserUsableGroups)
    },
    [onChange]
  )

  const updateRow = useCallback(
    (
      id: string,
      field: Exclude<keyof GroupPricingRow, '_id'>,
      value: string | number | boolean
    ) => {
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    setCreateDialogOpen(true)
  }, [])

  const removeRow = useCallback(
    (id: string) => {
      emitRows(rows.filter((row) => row._id !== id))
    },
    [emitRows, rows]
  )

  const handleDisplayNameBlur = useCallback(
    (row: GroupPricingRow) => {
      const displayName = row.displayName?.trim()
      if (!displayName || displayName === getDisplayName(row.name)) return
      renameGroup({ row, displayName })
    },
    [getDisplayName, renameGroup]
  )

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('Pricing groups')}</CardTitle>
            <CardDescription>
              {t(
                'Edit billing ratios and user-selectable groups in one table.'
              )}
            </CardDescription>
          </div>
          <Button onClick={addRow} size='sm' className='sm:self-start'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add group')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <StaticDataTable
            data={rows}
            getRowKey={(row) => row._id}
            emptyClassName='text-muted-foreground h-20 text-sm'
            emptyContent={t('No groups yet. Add a group to get started.')}
            columns={[
              {
                id: 'group',
                header: t('Group name'),
                className: 'min-w-40',
                cell: (row) => (
                  <Input
                    value={row.displayName ?? getDisplayName(row.name)}
                    onChange={(event) =>
                      updateRow(row._id, 'displayName', event.target.value)
                    }
                    onBlur={() => handleDisplayNameBlur(row)}
                  />
                ),
              },
              {
                id: 'ratio',
                header: t('Ratio'),
                className: 'w-28',
                cell: (row) => (
                  <Input
                    type='number'
                    min={0}
                    step={0.1}
                    value={String(row.ratio)}
                    onChange={(event) =>
                      updateRow(
                        row._id,
                        'ratio',
                        normalizeRatio(event.target.value)
                      )
                    }
                  />
                ),
              },
              {
                id: 'selectable',
                header: t('User selectable'),
                className: 'w-28 text-center',
                cell: (row) => (
                  <div className='flex justify-center'>
                    <Checkbox
                      checked={row.selectable}
                      onCheckedChange={(checked) =>
                        updateRow(row._id, 'selectable', checked === true)
                      }
                      aria-label={t('User selectable')}
                    />
                  </div>
                ),
              },
              {
                id: 'description',
                header: t('Description'),
                className: 'min-w-56',
                cell: (row) =>
                  row.selectable ? (
                    <Input
                      value={row.description}
                      placeholder={t('Group description')}
                      onChange={(event) =>
                        updateRow(row._id, 'description', event.target.value)
                      }
                    />
                  ) : (
                    <span className='text-muted-foreground px-3 text-sm'>
                      -
                    </span>
                  ),
              },
              {
                id: 'actions',
                header: t('Actions'),
                className: 'w-16 text-right',
                cellClassName: 'text-right',
                cell: (row) => (
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => removeRow(row._id)}
                    aria-label={t('Delete')}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                ),
              },
            ]}
          />
        </div>
      </CardContent>
      <CreatePricingGroupDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        onSave={(values) => createMutation.mutate(values)}
        pending={createMutation.isPending}
      />
    </Card>
  )
}

type CreatePricingGroupDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (values: {
    display_name: string
    description: string
    ratio: number
    user_usable: boolean
  }) => void
  pending: boolean
}

function CreatePricingGroupDialog(props: CreatePricingGroupDialogProps) {
  const { t } = useTranslation()
  const [displayName, setDisplayName] = useState('')
  const [description, setDescription] = useState('')
  const [ratio, setRatio] = useState('1')
  const [userUsable, setUserUsable] = useState(true)

  useEffect(() => {
    return deferEffect(() => {
      if (props.open) return
      setDisplayName('')
      setDescription('')
      setRatio('1')
      setUserUsable(true)
    })
  }, [props.open])

  const handleSave = () => {
    const name = displayName.trim()
    if (!name) return
    props.onSave({
      display_name: name,
      description: description.trim(),
      ratio: normalizeRatio(ratio),
      user_usable: userUsable,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Add group')}
      description={t('Create a group with a visible name.')}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave} disabled={props.pending}>
            {props.pending ? t('Saving...') : t('Add')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='space-y-2'>
          <Label>{t('Group name')}</Label>
          <Input
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder={t('Premium')}
          />
        </div>
        <div className='space-y-2'>
          <Label>{t('Ratio')}</Label>
          <Input
            type='number'
            min={0}
            step={0.1}
            value={ratio}
            onChange={(event) => setRatio(event.target.value)}
            placeholder='1.0'
          />
        </div>
        <div className='flex items-center gap-2'>
          <Checkbox
            checked={userUsable}
            onCheckedChange={(checked) => setUserUsable(checked === true)}
            aria-label={t('User selectable')}
          />
          <Label>{t('User selectable')}</Label>
        </div>
        <div className='space-y-2'>
          <Label>{t('Description')}</Label>
          <Input
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder={t('Group description')}
          />
        </div>
      </div>
    </Dialog>
  )
}

// Simple Group Dialog Component
type SimpleGroupDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (name: string, value: string) => void
  editData: SimpleGroup | null
  type: 'groupRatio' | 'topupGroupRatio' | null
  getDisplayName: (code?: string | null) => string
}

function SimpleGroupDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  type,
  getDisplayName,
}: SimpleGroupDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [value, setValue] = useState('')

  const title = type === 'groupRatio' ? t('group ratio') : t('top-up ratio')

  useEffect(() => {
    return deferEffect(() => {
      if (!open) {
        setName('')
        setValue('')
        return
      }

      setName(editData?.name ?? '')
      setValue(editData?.value ?? '')
    })
  }, [editData, open])

  const handleSave = () => {
    if (!name.trim() || !value.trim()) return
    onSave(name.trim(), value.trim())
    setName('')
    setValue('')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        editData
          ? t('Edit {{title}}', { title })
          : t('Add {{title}}', { title })
      }
      description={t('Configure the ratio for this group.')}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='space-y-2'>
          <Label>{t('Group name')}</Label>
          <GroupSelect
            value={name}
            onValueChange={setName}
            placeholder={t('Select a group')}
            disabled={!!editData}
          />
          {editData && (
            <p className='text-muted-foreground text-xs'>
              {getDisplayName(editData.name)}
            </p>
          )}
        </div>
        <div className='space-y-2'>
          <Label>{t('Ratio')}</Label>
          <Input
            value={value}
            onChange={(e) => {
              const val = e.target.value
              if (val === '' || !isNaN(parseFloat(val))) {
                setValue(val)
              }
            }}
            placeholder='1.0'
          />
        </div>
      </div>
    </Dialog>
  )
}

// Group Override Dialog Component
type GroupOverrideDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (targetGroup: string, ratio: number, oldTargetGroup?: string) => void
  editData: GroupOverride | null
  userGroup: string | null
  getDisplayName: (code?: string | null) => string
}

function GroupOverrideDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  userGroup,
  getDisplayName,
}: GroupOverrideDialogProps) {
  const { t } = useTranslation()
  const [targetGroup, setTargetGroup] = useState('')
  const [ratio, setRatio] = useState('')

  useEffect(() => {
    return deferEffect(() => {
      if (!open) {
        setTargetGroup('')
        setRatio('')
        return
      }

      setTargetGroup(editData?.targetGroup ?? '')
      setRatio(editData ? String(editData.ratio) : '')
    })
  }, [editData, open])

  const handleSave = () => {
    if (!targetGroup.trim() || !ratio.trim()) return
    const parsedRatio = parseFloat(ratio)
    if (isNaN(parsedRatio)) return

    onSave(targetGroup.trim(), parsedRatio, editData?.targetGroup)
    setTargetGroup('')
    setRatio('')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={editData ? t('Edit ratio override') : t('Add ratio override')}
      description={
        userGroup
          ? t(
              'Configure a custom ratio for "{{userGroup}}" users when using a specific token group.',
              { userGroup: getDisplayName(userGroup) }
            )
          : t(
              'Configure a custom ratio for when users use a specific token group.'
            )
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='space-y-2'>
          <Label>{t('Target group')}</Label>
          <GroupSelect
            value={targetGroup}
            onValueChange={setTargetGroup}
            placeholder={t('Select a group')}
            disabled={!!editData}
          />
          <p className='text-muted-foreground text-xs'>
            {t('The token group that will have a custom ratio')}
          </p>
        </div>
        <div className='space-y-2'>
          <Label>{t('Ratio')}</Label>
          <Input
            value={ratio}
            onChange={(e) => {
              const val = e.target.value
              if (val === '' || !isNaN(parseFloat(val))) {
                setRatio(val)
              }
            }}
            placeholder='0.9'
          />
          <p className='text-muted-foreground text-xs'>
            {t('Multiplier applied when {{userGroup}} uses {{targetGroup}}', {
              userGroup: userGroup
                ? getDisplayName(userGroup)
                : t('this user group'),
              targetGroup: targetGroup
                ? getDisplayName(targetGroup)
                : t('this token group'),
            })}
          </p>
        </div>
      </div>
    </Dialog>
  )
}
