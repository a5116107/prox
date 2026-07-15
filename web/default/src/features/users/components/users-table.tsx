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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Card, CardContent } from '@/components/ui/card'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDataTable,
} from '@/components/data-table'
import { getAdminUserOpsGrid } from '../api'
import {
  USER_STATUS,
  getUserRoleOptions,
  getUserStatusOptions,
  isUserDeleted,
} from '../constants'
import type { AdminUserOpsGridItem } from '../types'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { useUsersColumns } from './users-columns'
import { useUsers } from './users-provider'

const route = getRouteApi('/_authenticated/users/')

function isDisabledUserRow(user: AdminUserOpsGridItem) {
  return isUserDeleted(user) || user.status === USER_STATUS.DISABLED
}

function summarizeRows(items: AdminUserOpsGridItem[]) {
  return {
    communityBound: items.filter((item) => item.community_bound).length,
    primaryBound: items.filter((item) => item.primary_bound).length,
    fullAccess: items.filter((item) => item.access_level === 'full_access')
      .length,
    communityOnly: items.filter(
      (item) => item.access_level === 'community_only'
    ).length,
    override: items.filter((item) => item.manual_override_mode).length,
    frozen: items.filter((item) => item.active_frozen_key_count > 0).length,
  }
}

export function UsersTable() {
  const { t } = useTranslation()
  const columns = useUsersColumns()
  const { refreshTrigger } = useUsers()

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
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'role', searchKey: 'role', type: 'array' },
      { columnId: 'base_group', searchKey: 'group', type: 'string' },
      { columnId: 'access_level', searchKey: 'access_level', type: 'array' },
      {
        columnId: 'community_bound',
        searchKey: 'community_bound',
        type: 'array',
      },
      {
        columnId: 'has_community_room_membership',
        searchKey: 'has_community_room',
        type: 'array',
      },
      { columnId: 'qq_bound', searchKey: 'qq_bound', type: 'array' },
      { columnId: 'tg_bound', searchKey: 'tg_bound', type: 'array' },
      {
        columnId: 'primary_bound',
        searchKey: 'primary_bound',
        type: 'array',
      },
      {
        columnId: 'active_frozen_key_count',
        searchKey: 'has_frozen_keys',
        type: 'array',
      },
      {
        columnId: 'manual_override_mode',
        searchKey: 'override_mode',
        type: 'array',
      },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const roleFilter =
    (columnFilters.find((filter) => filter.id === 'role')?.value as
      | string[]
      | undefined) ?? []
  const groupFilter =
    (columnFilters.find((filter) => filter.id === 'base_group')
      ?.value as string) ?? ''
  const statusValue = statusFilter[0] ?? ''
  const roleValue = roleFilter[0] ?? ''
  const accessLevelFilter =
    (columnFilters.find((filter) => filter.id === 'access_level')?.value as
      | string[]
      | undefined) ?? []
  const communityBoundFilter =
    (columnFilters.find((filter) => filter.id === 'community_bound')?.value as
      | string[]
      | undefined) ?? []
  const communityRoomFilter =
    (columnFilters.find(
      (filter) => filter.id === 'has_community_room_membership'
    )?.value as string[] | undefined) ?? []
  const qqBoundFilter =
    (columnFilters.find((filter) => filter.id === 'qq_bound')?.value as
      | string[]
      | undefined) ?? []
  const tgBoundFilter =
    (columnFilters.find((filter) => filter.id === 'tg_bound')?.value as
      | string[]
      | undefined) ?? []
  const primaryBoundFilter =
    (columnFilters.find((filter) => filter.id === 'primary_bound')?.value as
      | string[]
      | undefined) ?? []
  const frozenFilter =
    (columnFilters.find((filter) => filter.id === 'active_frozen_key_count')
      ?.value as string[] | undefined) ?? []
  const overrideFilter =
    (columnFilters.find((filter) => filter.id === 'manual_override_mode')
      ?.value as string[] | undefined) ?? []
  const accessLevelValue = accessLevelFilter[0] ?? ''
  const communityBoundValue =
    (communityBoundFilter[0] as 'true' | 'false') ?? ''
  const communityRoomValue = (communityRoomFilter[0] as 'true' | 'false') ?? ''
  const qqBoundValue = (qqBoundFilter[0] as 'true' | 'false') ?? ''
  const tgBoundValue = (tgBoundFilter[0] as 'true' | 'false') ?? ''
  const primaryBoundValue = (primaryBoundFilter[0] as 'true' | 'false') ?? ''
  const frozenValue = (frozenFilter[0] as 'true' | 'false') ?? ''
  const overrideValue = overrideFilter[0] ?? ''
  const failedToLoadUsersMessage = t('Failed to load users')

  const query = useQuery({
    queryKey: [
      'admin-users-ops-grid',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      statusValue,
      roleValue,
      groupFilter,
      accessLevelValue,
      communityBoundValue,
      communityRoomValue,
      qqBoundValue,
      tgBoundValue,
      primaryBoundValue,
      frozenValue,
      overrideValue,
      failedToLoadUsersMessage,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getAdminUserOpsGrid({
        keyword: globalFilter,
        status: statusValue,
        role: roleValue,
        group: groupFilter,
        access_level: accessLevelValue,
        community_bound: communityBoundValue,
        has_community_room: communityRoomValue,
        qq_bound: qqBoundValue,
        tg_bound: tgBoundValue,
        primary_bound: primaryBoundValue,
        has_frozen_keys: frozenValue,
        override_mode: overrideValue,
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })

      if (!result.success) {
        toast.error(result.message || failedToLoadUsersMessage)
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    staleTime: 30_000,
    gcTime: 5 * 60_000,
    refetchOnWindowFocus: false,
    retry: 1,
    placeholderData: (previousData) => previousData,
  })

  const users = useMemo(() => query.data?.items ?? [], [query.data?.items])
  const summary = useMemo(() => summarizeRows(users), [users])

  const { table } = useDataTable({
    data: users,
    columns,
    enableRowSelection: true,
    columnFilters,
    globalFilter,
    pagination,
    globalFilterFn: (row, _columnId, filterValue) => {
      const searchValue = String(filterValue || '').toLowerCase()
      const fields = [
        row.original.username,
        row.original.display_name,
        row.original.email,
        row.original.reason_code,
        row.original.primary_platform,
      ]
      return fields.some((field) =>
        String(field || '')
          .toLowerCase()
          .includes(searchValue)
      )
    },
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: query.data?.total || 0,
    ensurePageInRange,
  })

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-6'>
        {[
          {
            label: t('Community bound'),
            value: summary.communityBound,
            helper: t('Users who have completed the community binding step.'),
          },
          {
            label: t('Primary unlocked'),
            value: summary.primaryBound,
            helper: t('Users already unlocked by QQ or TG main groups.'),
          },
          {
            label: t('Full access'),
            value: summary.fullAccess,
            helper: t('Current rows already sitting in the full-access lane.'),
          },
          {
            label: t('Community only'),
            value: summary.communityOnly,
            helper: t('Rows still limited to the community-only key scope.'),
          },
          {
            label: t('Manual overrides'),
            value: summary.override,
            helper: t('Rows that are currently being forced by operators.'),
          },
          {
            label: t('Frozen keys'),
            value: summary.frozen,
            helper: t('Users who still have frozen keys waiting for restore.'),
          },
        ].map((item) => (
          <Card key={item.label} size='sm' className='shadow-none'>
            <CardContent className='space-y-2 pt-4'>
              <div className='text-muted-foreground text-xs tracking-[0.18em] uppercase'>
                {item.label}
              </div>
              <div className='text-2xl font-semibold tracking-tight'>
                {item.value}
              </div>
              <div className='text-muted-foreground text-xs leading-5'>
                {item.helper}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className='min-h-0 flex-1'>
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={query.isLoading}
          isFetching={query.isFetching}
          emptyTitle={t('No users found')}
          emptyDescription={t(
            'No user rows matched the current binding, access, or override filters.'
          )}
          skeletonKeyPrefix='users-ops-grid'
          applyHeaderSize
          fixedHeight
          toolbarProps={{
            searchPlaceholder: t(
              'Search by username, display name, email, reason code...'
            ),
            filters: [
              {
                columnId: 'status',
                title: t('Status'),
                options: getUserStatusOptions(t),
                singleSelect: true,
              },
              {
                columnId: 'role',
                title: t('Role'),
                options: getUserRoleOptions(t),
                singleSelect: true,
              },
              {
                columnId: 'access_level',
                title: t('Access'),
                options: [
                  { label: t('None'), value: 'none' },
                  { label: t('Community only'), value: 'community_only' },
                  { label: t('Full access'), value: 'full_access' },
                  { label: t('Paid bypass'), value: 'paid_bypass' },
                  { label: t('Admin bypass'), value: 'admin_bypass' },
                  { label: t('Manual override'), value: 'manual_override' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'community_bound',
                title: t('Community'),
                options: [
                  { label: t('Bound'), value: 'true' },
                  { label: t('Unbound'), value: 'false' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'has_community_room_membership',
                title: t('Room'),
                options: [
                  { label: t('Joined'), value: 'true' },
                  { label: t('Missing'), value: 'false' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'qq_bound',
                title: t('QQ'),
                options: [
                  { label: t('Bound'), value: 'true' },
                  { label: t('Unbound'), value: 'false' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'tg_bound',
                title: t('TG'),
                options: [
                  { label: t('Bound'), value: 'true' },
                  { label: t('Unbound'), value: 'false' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'primary_bound',
                title: t('Primary'),
                options: [
                  { label: t('Unlocked'), value: 'true' },
                  { label: t('Locked'), value: 'false' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'active_frozen_key_count',
                title: t('Frozen'),
                options: [
                  { label: t('Has frozen keys'), value: 'true' },
                  { label: t('No frozen keys'), value: 'false' },
                ],
                singleSelect: true,
              },
              {
                columnId: 'manual_override_mode',
                title: t('Override'),
                options: [
                  { label: t('Policy default'), value: 'none' },
                  { label: 'full_access', value: 'full_access' },
                  { label: 'community_only', value: 'community_only' },
                  { label: 'custom_groups', value: 'custom_groups' },
                  { label: 'none', value: 'none' },
                ],
                singleSelect: true,
              },
            ],
          }}
          getRowClassName={(row, { isMobile }) =>
            isDisabledUserRow(row.original)
              ? isMobile
                ? DISABLED_ROW_MOBILE
                : DISABLED_ROW_DESKTOP
              : undefined
          }
          bulkActions={<DataTableBulkActions table={table} />}
        />
      </div>
    </div>
  )
}
