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
import { type ColumnDef } from '@tanstack/react-table'
import { Shield, User, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { Checkbox } from '@/components/ui/checkbox'
import { BadgeCell, BadgeListCell } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { USER_ROLE, USER_ROLES, USER_STATUS, isUserDeleted } from '../constants'
import type { AdminUserOpsGridItem } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

function boolBadge(value: boolean, labels: { on: string; off: string }) {
  return (
    <StatusBadge
      label={value ? labels.on : labels.off}
      variant={value ? 'success' : 'neutral'}
      copyable={false}
    />
  )
}

function accessVariant(value: string) {
  const normalized = String(value || 'none')
  if (normalized === 'full_access') {
    return 'success' as const
  }
  if (normalized === 'community_only') {
    return 'warning' as const
  }
  if (normalized === 'paid_bypass' || normalized === 'admin_bypass') {
    return 'info' as const
  }
  if (normalized === 'manual_override') {
    return 'warning' as const
  }
  return 'neutral' as const
}

function accessLabel(value: string, t: (key: string) => string) {
  const normalized = String(value || 'none')
  if (normalized === 'full_access') return t('Full access')
  if (normalized === 'community_only') return t('Community only')
  if (normalized === 'paid_bypass') return t('Paid-user bypass')
  if (normalized === 'admin_bypass') return t('Administrator bypass')
  if (normalized === 'manual_override') return t('Manual override')
  if (normalized === 'none') return t('No access')
  return normalized
}

function accessBadge(
  value: string,
  t: (key: string) => string
): React.ReactNode {
  return (
    <StatusBadge
      label={accessLabel(value, t)}
      variant={accessVariant(value)}
      copyable={false}
    />
  )
}

function reasonLabel(value: string | undefined, t: (key: string) => string) {
  const normalized = String(value || '').trim()
  if (!normalized) return t('No reason')
  if (normalized === 'not_bound') return t('Binding incomplete')
  if (normalized === 'community_bound') return t('Community binding matched')
  if (normalized === 'primary_bound') return t('Primary binding matched')
  if (normalized === 'admin_bypass') return t('Administrator bypass')
  if (normalized === 'paid_bypass') return t('Paid-user bypass')
  if (
    normalized === 'missing_community_binding' ||
    normalized === 'missing_oauth_binding'
  ) {
    return t('Missing community binding')
  }
  if (
    normalized === 'missing_primary_binding' ||
    normalized === 'missing_mainfield_binding'
  ) {
    return t('Missing main-field binding')
  }
  if (normalized === 'manual_allow') return t('Manual allow')
  if (normalized === 'manual_deny') return t('Manual deny')
  if (normalized === 'none') return t('No access')
  return normalized
}

function platformLabel(value: string | undefined, t: (key: string) => string) {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  if (normalized === 'qq') return 'QQ'
  if (normalized === 'tg') return 'TG'
  if (normalized === 'community') return t('Community')
  return value || '—'
}

function reasonBadge(
  value: string | undefined,
  t: (key: string) => string
): React.ReactNode {
  if (!value) {
    return (
      <StatusBadge label={t('No reason')} variant='neutral' copyable={false} />
    )
  }
  return (
    <StatusBadge
      label={reasonLabel(value, t)}
      variant='neutral'
      copyable={false}
    />
  )
}

function overrideLabel(
  mode: string,
  groups: string[],
  t: (key: string) => string
) {
  if (!mode) return t('Policy default')
  if (mode === 'custom_groups')
    return `${t('Custom groups')} · ${groups.length}`
  if (mode === 'allow' || mode === 'manual_allow') return t('Manual allow')
  if (mode === 'deny' || mode === 'manual_deny') return t('Manual deny')
  return mode
}

function roleIcon(role: number) {
  if (role === USER_ROLE.ROOT) return Shield
  if (role === USER_ROLE.ADMIN) return Users
  return User
}

function renderBindingPair(primary?: string, secondary?: string) {
  const first = String(primary || '').trim()
  const second = String(secondary || '').trim()
  if (!first && !second) return '—'
  if (!first) return second
  if (!second || second === first) return first
  return `${first} · ${second}`
}

export function useUsersColumns(): ColumnDef<AdminUserOpsGridItem>[] {
  const { t } = useTranslation()

  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label='Select all'
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label='Select row'
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'id',
      header: t('ID'),
      cell: ({ row }) => (
        <TableId value={row.original.user_id} className='w-[72px]' />
      ),
      size: 88,
      meta: { mobileHidden: true },
    },
    {
      accessorKey: 'username',
      header: t('User'),
      cell: ({ row }) => {
        const user = row.original
        return (
          <div className='flex min-w-[180px] flex-col gap-1'>
            <div className='flex items-center gap-2'>
              <LongText className='max-w-[150px] font-medium'>
                {user.username}
              </LongText>
              {isUserDeleted(user) ? (
                <StatusBadge
                  label={t('Deleted')}
                  variant='danger'
                  copyable={false}
                />
              ) : null}
            </div>
            <LongText className='text-muted-foreground max-w-[220px] text-xs'>
              {user.display_name || user.username}
              {user.email ? ` · ${user.email}` : ''}
            </LongText>
            <LongText className='text-muted-foreground max-w-[220px] text-xs'>
              {`${t('Site')}: ${user.site_id || '—'}`}
              {user.last_login_ip ? ` · ${t('IP')}: ${user.last_login_ip}` : ''}
            </LongText>
          </div>
        )
      },
      enableHiding: false,
      size: 230,
      meta: { mobileTitle: true },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = Number(row.original.status)
        if (isUserDeleted(row.original) || status === USER_STATUS.DELETED) {
          return (
            <StatusBadge
              label={t('Deleted')}
              variant='danger'
              copyable={false}
            />
          )
        }
        if (status === USER_STATUS.DISABLED) {
          return (
            <StatusBadge
              label={t('Disabled')}
              variant='neutral'
              copyable={false}
            />
          )
        }
        return (
          <StatusBadge
            label={t('Enabled')}
            variant='success'
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      enableSorting: false,
      size: 112,
      meta: { mobileBadge: true },
    },
    {
      accessorKey: 'base_group',
      header: t('Base group'),
      cell: ({ row }) => (
        <BadgeCell>
          <GroupBadge group={row.original.base_group} />
        </BadgeCell>
      ),
      filterFn: (row, id, value) => {
        const group = String(row.getValue(id) || '').toLowerCase()
        return group.includes(String(value || '').toLowerCase())
      },
      size: 132,
    },
    {
      accessorKey: 'role',
      header: t('Role'),
      cell: ({ row }) => {
        const roleValue = row.original.role
        const RoleIcon = roleIcon(roleValue)
        const labelKey =
          USER_ROLES[roleValue as keyof typeof USER_ROLES]?.labelKey || 'User'
        return (
          <div className='flex items-center gap-2'>
            <RoleIcon className='text-muted-foreground h-4 w-4' />
            <span className='text-sm'>{t(labelKey)}</span>
          </div>
        )
      },
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      enableSorting: false,
      size: 116,
      meta: { mobileHidden: true },
    },
    {
      accessorKey: 'access_level',
      header: t('Access'),
      cell: ({ row }) => {
        const user = row.original
        return (
          <div className='min-w-[210px] space-y-1.5'>
            <div className='flex flex-wrap gap-1.5'>
              {accessBadge(user.access_level, t)}
              {user.active_frozen_key_count > 0 ? (
                <StatusBadge
                  label={`${t('Frozen keys')}: ${user.active_frozen_key_count}`}
                  variant='warning'
                  copyable={false}
                />
              ) : null}
            </div>
            <LongText className='text-muted-foreground max-w-[200px] text-xs'>
              {reasonLabel(user.reason_code, t)}
            </LongText>
            <LongText className='text-muted-foreground max-w-[200px] text-xs'>
              {`${t('Manual lane')}: ${
                user.manual_override_mode
                  ? overrideLabel(
                      user.manual_override_mode,
                      user.manual_override_groups || [],
                      t
                    )
                  : t('Policy default')
              }`}
            </LongText>
          </div>
        )
      },
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      enableSorting: false,
      size: 224,
    },
    {
      accessorKey: 'effective_groups',
      header: t('Available groups'),
      cell: ({ row }) => (
        <BadgeListCell
          max={3}
          items={(row.original.effective_groups || []).map((group) => (
            <StatusBadge
              key={group}
              label={group}
              variant='neutral'
              copyable={false}
            />
          ))}
        />
      ),
      enableSorting: false,
      size: 240,
    },
    {
      accessorKey: 'community_bound',
      header: t('Bindings'),
      cell: ({ row }) => (
        <div className='min-w-[280px] space-y-1.5'>
          <div className='flex flex-wrap gap-1.5'>
            {boolBadge(row.original.community_bound, {
              on: t('Bound'),
              off: t('Unbound'),
            })}
            {boolBadge(row.original.has_community_room_membership, {
              on: t('Room joined'),
              off: t('Room missing'),
            })}
            {boolBadge(row.original.primary_bound, {
              on: t('Unlocked'),
              off: t('Locked'),
            })}
          </div>
          <LongText className='text-muted-foreground max-w-[260px] text-xs'>
            {`${t('Community')}: ${renderBindingPair(
              row.original.community_external_user_id,
              row.original.community_username
            )} · ${t('Community site ID')}: ${
              row.original.community_site_id || row.original.site_id || '—'
            }`}
          </LongText>
          <LongText className='text-muted-foreground max-w-[260px] text-xs'>
            {`QQ: ${renderBindingPair(
              row.original.qq_external_user_id,
              row.original.qq_username
            )}${
              row.original.qq_bound_group_ids?.length
                ? ` · ${row.original.qq_bound_group_ids.join(', ')}`
                : ''
            }`}
          </LongText>
          <LongText className='text-muted-foreground max-w-[260px] text-xs'>
            {`TG: ${renderBindingPair(
              row.original.tg_external_user_id,
              row.original.tg_username
            )}${
              row.original.tg_bound_group_ids?.length
                ? ` · ${row.original.tg_bound_group_ids.join(', ')}`
                : ''
            }`}
          </LongText>
          <LongText className='text-muted-foreground max-w-[260px] text-xs'>
            {`${t('Primary field')}: ${platformLabel(
              row.original.primary_platform,
              t
            )}${
              row.original.matched_primary_group_id
                ? ` · ${row.original.matched_primary_group_id}`
                : ''
            }`}
          </LongText>
        </div>
      ),
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      enableSorting: false,
      size: 300,
    },
    {
      accessorKey: 'reason_code',
      header: t('Reason'),
      cell: ({ row }) => (
        <BadgeCell>{reasonBadge(row.original.reason_code, t)}</BadgeCell>
      ),
      enableSorting: false,
      size: 164,
      meta: { mobileHidden: true },
    },
    {
      id: 'invite_info',
      header: t('Invite relation / Login'),
      cell: ({ row }) => (
        <div className='min-w-[220px] space-y-1 text-sm'>
          <div>
            {`${t('Inviter')}: ${
              row.original.inviter_username ||
              row.original.inviter_display_name ||
              (row.original.inviter_id
                ? `UID ${row.original.inviter_id}`
                : t('None'))
            }`}
          </div>
          <div className='text-muted-foreground text-xs'>
            {`${t('Invitee count')}: ${
              row.original.invitee_count ?? row.original.aff_count ?? 0
            }`}
          </div>
          <div className='text-muted-foreground text-xs'>
            {(row.original.invitee_preview || [])
              .slice(0, 3)
              .map(
                (item) =>
                  item.username || item.display_name || `UID ${item.user_id}`
              )
              .join(', ') || '—'}
          </div>
          <div className='text-muted-foreground text-xs'>
            {`${t('Revenue')}: ${formatQuota(row.original.aff_history_quota || 0)}`}
          </div>
        </div>
      ),
      size: 240,
      enableSorting: false,
      meta: { mobileHidden: true },
    },
    {
      accessorKey: 'last_login_at',
      header: t('Last login & IP'),
      cell: ({ row }) => (
        <div className='min-w-[180px] space-y-1 text-sm'>
          <span className='text-muted-foreground'>
            {row.original.last_login_at
              ? formatTimestamp(row.original.last_login_at)
              : '—'}
          </span>
          <LongText className='text-muted-foreground max-w-[160px] text-xs'>
            {row.original.last_login_ip || '—'}
            {row.original.last_login_source
              ? ` · ${row.original.last_login_source}`
              : ''}
          </LongText>
        </div>
      ),
      size: 210,
      meta: { mobileHidden: true },
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => <DataTableRowActions row={row} />,
      meta: { pinned: 'right' as const },
    },
  ]
}
