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
import { useEffect, useMemo, useState } from 'react'
import { Loader2, ShieldPlus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Dialog } from '@/components/dialog'
import { MultiSelect } from '@/components/multi-select'
import {
  getOpsAssignableGroups,
  updateAdminUserAccessOverride,
} from '../../api'
import type {
  AdminUserAccessOverrideMode,
  AdminUserAccessOverridePayload,
  AdminUserOpsGridItem,
} from '../../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: AdminUserOpsGridItem | null
  onSuccess?: () => void
}

const OVERRIDE_MODES: AdminUserAccessOverrideMode[] = [
  'clear',
  'full_access',
  'community_only',
  'custom_groups',
  'none',
]

function overrideModeLabel(
  mode: AdminUserAccessOverrideMode,
  t: (key: string) => string
) {
  switch (mode) {
    case 'clear':
      return t('Follow live policy')
    case 'full_access':
      return t('Grant full-access groups')
    case 'community_only':
      return t('Restrict to community groups')
    case 'custom_groups':
      return t('Grant custom API groups')
    case 'none':
      return t('Block all API groups')
    default:
      return mode
  }
}

export function UserAccessOverrideDialog({
  open,
  onOpenChange,
  user,
  onSuccess,
}: Props) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<AdminUserAccessOverrideMode>('clear')
  const [groups, setGroups] = useState<string[]>([])
  const [reason, setReason] = useState('')
  const [loadingGroups, setLoadingGroups] = useState(false)
  const [availableGroups, setAvailableGroups] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (!open || !user) return
    setMode(
      user.manual_override_mode
        ? (user.manual_override_mode as AdminUserAccessOverrideMode)
        : 'clear'
    )
    setGroups(user.manual_override_groups || [])
    setReason('')
  }, [open, user])

  useEffect(() => {
    if (!open) return
    setLoadingGroups(true)
    const siteId = String(user?.site_id || user?.community_site_id || '').trim()
    getOpsAssignableGroups(siteId)
      .then((res) => {
        if (res.success && Array.isArray(res.data)) {
          setAvailableGroups(res.data)
        } else {
          setAvailableGroups([])
        }
      })
      .finally(() => setLoadingGroups(false))
  }, [open, user?.site_id, user?.community_site_id])

  const groupOptions = useMemo(() => {
    const all = new Set([...(availableGroups || []), ...(groups || [])])
    return Array.from(all).map((item) => ({
      value: item,
      label: item,
    }))
  }, [availableGroups, groups])

  const submit = async () => {
    if (!user) return
    if (mode === 'custom_groups' && groups.length === 0) {
      toast.error(t('Please select at least one API group'))
      return
    }
    setSubmitting(true)
    try {
      const payload: AdminUserAccessOverridePayload = {
        mode,
        groups: mode === 'custom_groups' ? groups : [],
        reason,
      }
      const res = await updateAdminUserAccessOverride(user.user_id, payload)
      if (!res.success) {
        toast.error(res.message || t('Save failed'))
        return
      }
      toast.success(t('Access override updated'))
      onSuccess?.()
      onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('An unexpected error occurred')
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <div className='flex items-center gap-2'>
          <ShieldPlus className='h-5 w-5' />
          {t('Adjust access override')}
        </div>
      }
      description={t(
        'Use this only when package level, community binding, or QQ/TG main-field state is not enough to explain the required access.'
      )}
      contentClassName='sm:max-w-xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      showCloseButton
      footer={
        <div className='flex w-full flex-col gap-2 sm:flex-row sm:justify-end'>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={submit} disabled={submitting}>
            {submitting ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : null}
            {submitting ? t('Saving...') : t('Save changes')}
          </Button>
        </div>
      }
    >
      <div className='border-border/70 bg-muted/30 text-muted-foreground rounded-2xl border p-4 text-sm leading-6'>
        <div className='text-foreground font-medium'>{user?.username}</div>
        <div>
          {t('Current access')}: {user?.access_level || t('None')}
        </div>
        <div>
          {t('Current override')}:{' '}
          {user?.manual_override_mode
            ? overrideModeLabel(
                user.manual_override_mode as AdminUserAccessOverrideMode,
                t
              )
            : t('None')}
        </div>
        <div>
          {t('Policy source')}:{' '}
          {user?.site_id || user?.community_site_id || t('Unknown')}
        </div>
      </div>

      <div className='space-y-2'>
        <Label>{t('Override mode')}</Label>
        <Select
          value={mode}
          onValueChange={(value) =>
            setMode(value as AdminUserAccessOverrideMode)
          }
        >
          <SelectTrigger>
            <SelectValue placeholder={t('Select a mode')} />
          </SelectTrigger>
          <SelectContent>
            {OVERRIDE_MODES.map((item) => (
              <SelectItem key={item} value={item}>
                {overrideModeLabel(item, t)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className='text-muted-foreground text-xs leading-6'>
          {mode === 'clear'
            ? t(
                'Clear the manual override and go back to normal policy evaluation.'
              )
            : mode === 'full_access'
              ? t('Give this user the full-access API groups immediately.')
              : mode === 'community_only'
                ? t(
                    'Force the user into the community-only lane even if the main field is not fully satisfied.'
                  )
                : mode === 'custom_groups'
                  ? t('Choose the exact API groups that should be granted.')
                  : t(
                      'Block this user from access even if community or main-field conditions are currently met.'
                    )}
        </div>
      </div>

      {mode === 'custom_groups' ? (
        <div className='space-y-2'>
          <Label>{t('Custom API groups')}</Label>
          {loadingGroups ? (
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('Loading...')}
            </div>
          ) : (
            <div className='space-y-2'>
              <MultiSelect
                options={groupOptions}
                selected={groups}
                onChange={setGroups}
                placeholder={t('Select API groups')}
              />
              <div className='text-muted-foreground text-xs leading-6'>
                {groupOptions.length > 0
                  ? t(
                      'The selectable groups come from the live ops access policy for this site, not the legacy /api/group list.'
                    )
                  : t(
                      'No assignable groups are visible in the live ops access policy for this site yet.'
                    )}
              </div>
            </div>
          )}
        </div>
      ) : null}

      <div className='space-y-2'>
        <Label>{t('Reason')}</Label>
        <Textarea
          rows={4}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder={t(
            'Explain why this override exists so support and future operators can understand it.'
          )}
        />
      </div>
    </Dialog>
  )
}
