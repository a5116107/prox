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
import { useState } from 'react'
import { type Table } from '@tanstack/react-table'
import { KeyRound, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { recomputeAdminUserMembership, restoreAdminUserKeys } from '../api'
import { type AdminUserOpsGridItem } from '../types'
import { useUsers } from './users-provider'

interface DataTableBulkActionsProps {
  table: Table<AdminUserOpsGridItem>
}

export function DataTableBulkActions({ table }: DataTableBulkActionsProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [recomputing, setRecomputing] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedUsers = selectedRows
    .map((row) => row.original)
    .filter(Boolean) as AdminUserOpsGridItem[]
  const selectedUserIds = selectedUsers
    .map((user) => Number(user.user_id || user.id || 0))
    .filter((userId) => Number.isFinite(userId) && userId > 0)
  const restorableUsers = selectedUsers.filter((user) =>
    Boolean(
      user.can_restore ||
      user.has_active_risk_control ||
      user.has_active_frozen_keys ||
      (user.active_frozen_key_count || 0) > 0
    )
  )

  const resetSelection = () => {
    table.resetRowSelection()
  }

  const handleRecomputeAll = async () => {
    if (selectedUserIds.length === 0) return
    setRecomputing(true)
    try {
      const results = await Promise.allSettled(
        selectedUserIds.map((userId) => recomputeAdminUserMembership(userId))
      )
      const successCount = results.filter(
        (item) => item.status === 'fulfilled' && item.value?.success
      ).length
      const failedCount = results.length - successCount
      if (failedCount === 0) {
        toast.success(
          t('Membership recomputed for {{count}} selected users', {
            count: successCount,
          })
        )
      } else {
        toast.warning(
          t(
            'Membership recompute finished: {{success}} succeeded, {{failed}} failed',
            {
              success: successCount,
              failed: failedCount,
            }
          )
        )
      }
      triggerRefresh()
      resetSelection()
    } finally {
      setRecomputing(false)
    }
  }

  const handleRestoreAll = async () => {
    if (restorableUsers.length === 0) {
      toast.info(t('No selected users currently have frozen keys'))
      return
    }
    setRestoring(true)
    try {
      const results = await Promise.allSettled(
        restorableUsers.map((user) => restoreAdminUserKeys(user.user_id))
      )
      const successCount = results.filter(
        (item) => item.status === 'fulfilled' && item.value?.success
      ).length
      const failedCount = results.length - successCount
      if (failedCount === 0) {
        toast.success(
          t('Frozen keys restored for {{count}} selected users', {
            count: successCount,
          })
        )
      } else {
        toast.warning(
          t('Key restore finished: {{success}} succeeded, {{failed}} failed', {
            success: successCount,
            failed: failedCount,
          })
        )
      }
      triggerRefresh()
      resetSelection()
    } finally {
      setRestoring(false)
    }
  }

  return (
    <BulkActionsToolbar table={table} entityName='user'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='outline'
              size='icon'
              onClick={handleRecomputeAll}
              className='size-8'
              aria-label={t('Recompute selected memberships')}
              title={t('Recompute selected memberships')}
              disabled={recomputing}
            />
          }
        >
          <RefreshCw className={recomputing ? 'animate-spin' : ''} />
          <span className='sr-only'>{t('Recompute selected memberships')}</span>
        </TooltipTrigger>
        <TooltipContent>
          <p>{t('Recompute selected memberships')}</p>
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='outline'
              size='icon'
              onClick={handleRestoreAll}
              className='size-8'
              aria-label={t('Restore frozen keys for selected users')}
              title={t('Restore frozen keys for selected users')}
              disabled={restoring || restorableUsers.length === 0}
            />
          }
        >
          <KeyRound className={restoring ? 'animate-pulse' : ''} />
          <span className='sr-only'>
            {t('Restore frozen keys for selected users')}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <p>{t('Restore frozen keys for selected users')}</p>
        </TooltipContent>
      </Tooltip>
    </BulkActionsToolbar>
  )
}
