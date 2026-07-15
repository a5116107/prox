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
import { useMemo, useState } from 'react'
import type { AccessControlStatus } from '@/types/access-control'
import { ShieldAlert, ShieldCheck, ShieldQuestion } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { refreshMyAccessControlStatus } from '@/lib/api'
import {
  getAccessControlActionLabels,
  getAccessControlDisplayMessage,
  getAccessControlNextSteps,
} from '@/hooks/use-access-control'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

function getAccessLevelTone(accessLevel?: string) {
  switch (accessLevel) {
    case 'full_access':
    case 'paid_bypass':
    case 'admin_bypass':
    case 'manual_override':
      return {
        icon: ShieldCheck,
        title: 'Access binding status is healthy',
        className: 'text-emerald-500',
      }
    case 'community_only':
      return {
        icon: ShieldQuestion,
        title: 'Community-only access is active',
        className: 'text-amber-500',
      }
    default:
      return {
        icon: ShieldAlert,
        title: 'Binding required before using full features',
        className: 'text-red-500',
      }
  }
}

type AccessControlDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  status?: AccessControlStatus | null
  force?: boolean
  onStatusChanged?: (status: AccessControlStatus | null) => void
}

export function AccessControlDialog({
  open,
  onOpenChange,
  status,
  force = false,
  onStatusChanged,
}: AccessControlDialogProps) {
  const { t } = useTranslation()
  const [refreshing, setRefreshing] = useState(false)

  const tone = useMemo(
    () => getAccessLevelTone(status?.access_level),
    [status?.access_level]
  )

  const forceOpen = force && open
  const message = getAccessControlDisplayMessage(status, t)
  const nextSteps = getAccessControlNextSteps(status, t)
  const effectiveGroups = status?.effective_groups || []
  const actionLabels = getAccessControlActionLabels(status, t)

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      const res = await refreshMyAccessControlStatus()
      const next = res.data?.status || null
      if (next) {
        onStatusChanged?.(next)
      }
      if (res.success) {
        toast.success(t('Access status refreshed successfully'))
      } else {
        toast.error(res.message || t('Access status refresh failed'))
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Access status refresh failed')
      toast.error(message)
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={forceOpen ? undefined : onOpenChange}
    >
      <AlertDialogContent>
        <AlertDialogHeader className='text-left'>
          <AlertDialogTitle className='flex items-center gap-2'>
            <tone.icon className={`h-5 w-5 ${tone.className}`} />
            {t(tone.title)}
          </AlertDialogTitle>
          <AlertDialogDescription render={<div className='space-y-2' />}>
            <div>{message}</div>
            {nextSteps.length > 0 ? (
              <ul className='text-muted-foreground list-disc space-y-1 pl-5 text-sm'>
                {nextSteps.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ul>
            ) : null}
            {status?.has_active_frozen_keys ? (
              <div className='text-muted-foreground text-sm'>
                {t('Detected frozen API keys')}:{' '}
                {status.active_frozen_keys || 0}
              </div>
            ) : null}
            {status?.requested_group ? (
              <div className='text-muted-foreground text-sm'>
                {t('Requested group')}: {status.requested_group}
              </div>
            ) : null}
            {effectiveGroups.length > 0 ? (
              <div className='text-muted-foreground text-sm'>
                {t('Currently allowed groups')}: {effectiveGroups.join(', ')}
              </div>
            ) : null}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className='flex flex-wrap gap-2'>
          {status?.bind_url ? (
            <Button
              type='button'
              variant='outline'
              onClick={() => window.open(status.bind_url, '_blank')}
            >
              {actionLabels.bind}
            </Button>
          ) : null}
          {status?.join_url ? (
            <Button
              type='button'
              variant='outline'
              onClick={() => window.open(status.join_url, '_blank')}
            >
              {actionLabels.community}
            </Button>
          ) : null}
          {status?.primary_join_url ? (
            <Button
              type='button'
              variant='outline'
              onClick={() => window.open(status.primary_join_url, '_blank')}
            >
              {actionLabels.primary}
            </Button>
          ) : null}
        </div>
        <AlertDialogFooter>
          <Button
            type='button'
            variant='ghost'
            onClick={() => onOpenChange(false)}
          >
            {actionLabels.close}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={refreshing}
            onClick={handleRefresh}
          >
            {refreshing ? t('Refreshing...') : actionLabels.refresh}
          </Button>
          {forceOpen ? (
            <AlertDialogAction
              disabled={refreshing}
              onClick={() => {
                void handleRefresh()
              }}
            >
              {actionLabels.done}
            </AlertDialogAction>
          ) : null}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
