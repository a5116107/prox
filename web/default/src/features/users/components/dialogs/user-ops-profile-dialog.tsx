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
import {
  AlertTriangle,
  Bot,
  Link2,
  Loader2,
  MessageSquareText,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  UserRoundCog,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { BadgeListCell } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { getAdminUserBindings, getAdminUserOpsProfile } from '../../api'
import type { AdminUserOpsProfile } from '../../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number | null
}

function boolBadge(
  enabled: boolean | undefined,
  labels: { on: string; off: string }
) {
  return (
    <StatusBadge
      label={enabled ? labels.on : labels.off}
      variant={enabled ? 'success' : 'neutral'}
      copyable={false}
    />
  )
}

function infoCard(props: {
  title: string
  description: string
  value: string
  badge?: React.ReactNode
}) {
  return (
    <Card size='sm'>
      <CardHeader className='pb-2'>
        <div className='flex items-start justify-between gap-3'>
          <div>
            <CardTitle className='text-sm'>{props.title}</CardTitle>
            <CardDescription className='text-xs'>
              {props.description}
            </CardDescription>
          </div>
          {props.badge}
        </div>
      </CardHeader>
      <CardContent>
        <div className='text-lg font-semibold tracking-tight'>
          {props.value}
        </div>
      </CardContent>
    </Card>
  )
}

function profileEffectiveGroups(profile: AdminUserOpsProfile | undefined) {
  if (!profile) return []
  return Array.isArray(profile.effective_groups) ? profile.effective_groups : []
}

function renderBindingPair(primary?: string, secondary?: string) {
  const first = String(primary || '').trim()
  const second = String(secondary || '').trim()
  if (!first && !second) return '—'
  if (!first) return second
  if (!second || second === first) return first
  return `${first} · ${second}`
}

function accessLabel(value: string | undefined, t: (key: string) => string) {
  const normalized = String(value || 'none')
  if (normalized === 'full_access') return t('Full access')
  if (normalized === 'community_only') return t('Community only')
  if (normalized === 'paid_bypass') return t('Paid-user bypass')
  if (normalized === 'admin_bypass') return t('Administrator bypass')
  if (normalized === 'manual_override') return t('Manual override')
  if (normalized === 'none') return t('No access')
  return normalized
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

export function UserOpsProfileDialog({ open, onOpenChange, userId }: Props) {
  const { t } = useTranslation()
  const failedToLoadMessage = t('Failed to load')

  const profileQuery = useQuery({
    queryKey: ['admin-user-ops-profile', userId, failedToLoadMessage],
    enabled: open && Boolean(userId),
    queryFn: async () => {
      const res = await getAdminUserOpsProfile(userId as number)
      if (!res.success || !res.data) {
        throw new Error(res.message || failedToLoadMessage)
      }
      return res.data
    },
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    retry: 1,
    placeholderData: (previousData) => previousData,
  })

  const bindingsQuery = useQuery({
    queryKey: ['admin-user-ops-bindings', userId, failedToLoadMessage],
    enabled: open && Boolean(userId),
    queryFn: async () => {
      const res = await getAdminUserBindings(userId as number)
      if (!res.success || !res.data) {
        throw new Error(res.message || failedToLoadMessage)
      }
      return res.data
    },
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    retry: 1,
    placeholderData: (previousData) => previousData,
  })

  const profile = profileQuery.data
  const effectiveGroups = profileEffectiveGroups(profile)

  const bindingSummary = useMemo(() => {
    const rows = bindingsQuery.data
    if (!rows) return []
    return [
      {
        title: t('Community identity'),
        value: rows.identity_bindings.length
          ? rows.identity_bindings
              .map((item) => item.username || item.external_user_id || '—')
              .join(', ')
          : t('Not bound'),
      },
      {
        title: t('Chat bindings'),
        value: rows.agent_chat_bindings.length
          ? rows.agent_chat_bindings
              .map(
                (item) =>
                  `${String(item.source || '—').toUpperCase()} · ${
                    item.room_id || item.external_user_id || '—'
                  }`
              )
              .join(' / ')
          : t('No active binding'),
      },
      {
        title: t('Membership states'),
        value: rows.chat_membership_states.length
          ? rows.chat_membership_states
              .map(
                (item) =>
                  `${String(item.source || '—').toUpperCase()} · ${
                    item.room_id || '—'
                  } · ${item.status || '—'}`
              )
              .join(' / ')
          : t('No recent membership evidence'),
      },
    ]
  }, [bindingsQuery.data, t])

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <div className='flex items-center gap-2'>
          <UserRoundCog className='h-5 w-5' />
          {t('User operations profile')}
        </div>
      }
      description={t(
        'Explain why this user can or cannot use certain API groups, create keys, or recover frozen keys.'
      )}
      contentClassName='sm:max-w-5xl'
      contentHeight='auto'
      bodyClassName='space-y-5'
      showCloseButton
      footer={
        <div className='flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-xs'>
            {profile ? `${t('Site')}: ${profile.site_id}` : t('Loading...')}
          </div>
          <Button
            type='button'
            variant='outline'
            onClick={() => {
              void profileQuery.refetch()
              void bindingsQuery.refetch()
            }}
            disabled={profileQuery.isFetching || bindingsQuery.isFetching}
          >
            <RefreshCw
              className={`mr-2 h-4 w-4 ${
                profileQuery.isFetching || bindingsQuery.isFetching
                  ? 'animate-spin'
                  : ''
              }`}
            />
            {t('Refresh truth')}
          </Button>
        </div>
      }
    >
      {profileQuery.isLoading ? (
        <div className='flex items-center justify-center py-16'>
          <Loader2 className='text-muted-foreground h-6 w-6 animate-spin' />
        </div>
      ) : profileQuery.error ? (
        <div className='rounded-xl border border-rose-500/20 bg-rose-500/8 p-5 text-sm text-rose-600 dark:text-rose-300'>
          {(profileQuery.error as Error).message}
        </div>
      ) : profile ? (
        <div className='space-y-5'>
          <div className='border-border/70 from-background via-background to-muted/20 rounded-[28px] border bg-gradient-to-br p-5 shadow-sm'>
            <div className='flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between'>
              <div className='space-y-3'>
                <div className='text-muted-foreground text-xs tracking-[0.22em] uppercase'>
                  {t('User access truth')}
                </div>
                <div>
                  <div className='text-2xl font-semibold tracking-tight'>
                    {profile.username}
                  </div>
                  <div className='text-muted-foreground mt-1 text-sm leading-6'>
                    {profile.display_name || profile.username}
                    {profile.email ? ` · ${profile.email}` : ''}
                  </div>
                </div>
                <div className='flex flex-wrap gap-2'>
                  <StatusBadge
                    label={`${t('Site')}: ${profile.site_id || '—'}`}
                    variant='info'
                    copyable={false}
                  />
                  <StatusBadge
                    label={`${t('Access')}: ${accessLabel(
                      profile.access_level,
                      t
                    )}`}
                    variant={
                      profile.access_level === 'full_access'
                        ? 'success'
                        : profile.access_level === 'community_only'
                          ? 'warning'
                          : profile.access_level === 'paid_bypass' ||
                              profile.access_level === 'admin_bypass'
                            ? 'info'
                            : 'neutral'
                    }
                    copyable={false}
                  />
                  <StatusBadge
                    label={`${t('Base group')}: ${profile.base_group || '—'}`}
                    variant='info'
                    copyable={false}
                  />
                  <StatusBadge
                    label={`${t('Reason')}: ${reasonLabel(
                      profile.reason_code,
                      t
                    )}`}
                    variant='neutral'
                    copyable={false}
                  />
                  {profile.manual_override_mode ? (
                    <StatusBadge
                      label={`${t('Manual override')}: ${
                        profile.manual_override_mode
                      }`}
                      variant='warning'
                      copyable={false}
                    />
                  ) : null}
                </div>
              </div>
              <div className='grid w-full gap-3 sm:grid-cols-2 xl:w-[520px]'>
                {infoCard({
                  title: t('Current effective groups'),
                  description: t('The API groups this user can use right now.'),
                  value: String(effectiveGroups.length || 0),
                  badge: (
                    <StatusBadge
                      label={
                        effectiveGroups.length
                          ? effectiveGroups.join(', ')
                          : t('None')
                      }
                      variant='neutral'
                      copyable={false}
                      className='max-w-[180px] truncate'
                    />
                  ),
                })}
                {infoCard({
                  title: t('Frozen keys'),
                  description: t(
                    'Access-control and community-gate frozen keys combined.'
                  ),
                  value: String(profile.active_frozen_key_count || 0),
                  badge: boolBadge(profile.can_restore, {
                    on: t('Needs restore'),
                    off: t('Clean'),
                  }),
                })}
                {infoCard({
                  title: t('Invite contribution'),
                  description: t('Historical invite count and quota.'),
                  value: `${profile.aff_count || 0} / ${formatQuota(
                    profile.aff_history_quota || 0
                  )}`,
                })}
                {infoCard({
                  title: t('Last login'),
                  description: t(
                    'Show the latest login time, IP, and login source.'
                  ),
                  value: profile.last_login_at
                    ? formatTimestamp(profile.last_login_at)
                    : '—',
                  badge: (
                    <StatusBadge
                      label={profile.last_login_ip || t('No login IP')}
                      variant='neutral'
                      copyable={false}
                      className='max-w-[180px] truncate'
                    />
                  ),
                })}
              </div>
            </div>
          </div>

          <div className='grid gap-4 xl:grid-cols-[1.4fr_1fr]'>
            <Card>
              <CardHeader>
                <CardTitle className='flex items-center gap-2 text-base'>
                  <Sparkles className='h-4 w-4' />
                  {t('What the system decided')}
                </CardTitle>
                <CardDescription>
                  {t(
                    'Support and operators should be able to copy these exact reasons when explaining access results to a user.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='grid gap-4 md:grid-cols-2'>
                  <div className='border-border/70 bg-muted/30 space-y-2 rounded-2xl border p-4'>
                    <div className='text-sm font-semibold'>{t('Reason')}</div>
                    <div className='text-muted-foreground text-sm leading-6'>
                      {reasonLabel(profile.reason_code, t)}
                      {profile.reason_message ? ' · ' : ''}
                      {profile.reason_message || '—'}
                    </div>
                  </div>
                  <div className='border-border/70 bg-muted/30 space-y-2 rounded-2xl border p-4'>
                    <div className='text-sm font-semibold'>
                      {t('Manual override note')}
                    </div>
                    <div className='text-muted-foreground text-sm leading-6'>
                      {profile.manual_override_reason || t('No override note')}
                    </div>
                  </div>
                </div>

                <Separator />

                <div className='grid gap-4 md:grid-cols-3'>
                  <div className='border-border/70 bg-background/60 space-y-3 rounded-2xl border p-4'>
                    <div className='flex items-center gap-2 text-sm font-semibold'>
                      <Link2 className='h-4 w-4' />
                      {t('Community')}
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {boolBadge(profile.community_bound, {
                        on: t('Bound'),
                        off: t('Unbound'),
                      })}
                      {boolBadge(profile.has_community_oauth_binding, {
                        on: t('OAuth ready'),
                        off: t('OAuth missing'),
                      })}
                      {boolBadge(profile.has_community_room_membership, {
                        on: t('Room joined'),
                        off: t('Room missing'),
                      })}
                    </div>
                    <div className='text-muted-foreground text-xs leading-6'>
                      {renderBindingPair(
                        profile.community_external_user_id,
                        profile.community_username
                      )}
                      {profile.community_site_id
                        ? ` · ${t('Community site ID')}: ${profile.community_site_id}`
                        : ''}
                      {profile.community_room_ids.length
                        ? ` · ${profile.community_room_ids.join(', ')}`
                        : ''}
                    </div>
                  </div>
                  <div className='border-border/70 bg-background/60 space-y-3 rounded-2xl border p-4'>
                    <div className='flex items-center gap-2 text-sm font-semibold'>
                      <Bot className='h-4 w-4' />
                      {t('QQ')}
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {boolBadge(profile.qq_bound, {
                        on: t('Bound'),
                        off: t('Unbound'),
                      })}
                      {boolBadge(
                        profile.primary_platform === 'qq' &&
                          profile.primary_bound,
                        {
                          on: t('Primary unlock'),
                          off: t('Not primary unlock'),
                        }
                      )}
                    </div>
                    <div className='text-muted-foreground text-xs leading-6'>
                      {renderBindingPair(
                        profile.qq_external_user_id,
                        profile.qq_username
                      )}
                      {profile.qq_bound_group_ids.length
                        ? ` · ${profile.qq_bound_group_ids.join(', ')}`
                        : ''}
                    </div>
                  </div>
                  <div className='border-border/70 bg-background/60 space-y-3 rounded-2xl border p-4'>
                    <div className='flex items-center gap-2 text-sm font-semibold'>
                      <MessageSquareText className='h-4 w-4' />
                      {t('TG')}
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {boolBadge(profile.tg_bound, {
                        on: t('Bound'),
                        off: t('Unbound'),
                      })}
                      {boolBadge(
                        profile.primary_platform === 'tg' &&
                          profile.primary_bound,
                        {
                          on: t('Primary unlock'),
                          off: t('Not primary unlock'),
                        }
                      )}
                    </div>
                    <div className='text-muted-foreground text-xs leading-6'>
                      {renderBindingPair(
                        profile.tg_external_user_id,
                        profile.tg_username
                      )}
                      {profile.tg_bound_group_ids.length
                        ? ` · ${profile.tg_bound_group_ids.join(', ')}`
                        : ''}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className='flex items-center gap-2 text-base'>
                  <ShieldCheck className='h-4 w-4' />
                  {t('Realtime binding evidence')}
                </CardTitle>
                <CardDescription>
                  {t(
                    'These rows come from the binding and membership tables, not from assumptions in the UI.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                {bindingsQuery.isLoading ? (
                  <div className='flex items-center justify-center py-10'>
                    <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />
                  </div>
                ) : bindingsQuery.error ? (
                  <div className='rounded-xl border border-amber-500/20 bg-amber-500/10 p-4 text-sm text-amber-700 dark:text-amber-300'>
                    {(bindingsQuery.error as Error).message}
                  </div>
                ) : (
                  bindingSummary.map((item) => (
                    <div
                      key={item.title}
                      className='border-border/70 bg-muted/30 rounded-2xl border p-4'
                    >
                      <div className='text-sm font-semibold'>{item.title}</div>
                      <div className='text-muted-foreground mt-2 text-sm leading-6'>
                        {item.value}
                      </div>
                    </div>
                  ))
                )}

                <div className='border-border/70 bg-background/60 rounded-2xl border p-4'>
                  <div className='flex items-center gap-2 text-sm font-semibold'>
                    <AlertTriangle className='h-4 w-4' />
                    {t('Frozen key split')}
                  </div>
                  <div className='mt-3 flex flex-wrap gap-2'>
                    <StatusBadge
                      label={`${t('Access control')}: ${
                        profile.access_control_frozen_keys || 0
                      }`}
                      variant='neutral'
                      copyable={false}
                    />
                    <StatusBadge
                      label={`${t('Community gate')}: ${
                        profile.community_gate_frozen_keys || 0
                      }`}
                      variant='neutral'
                      copyable={false}
                    />
                  </div>
                </div>

                <div className='border-border/70 bg-background/60 rounded-2xl border p-4'>
                  <div className='flex items-center gap-2 text-sm font-semibold'>
                    <Users className='h-4 w-4' />
                    {t('Invite relation / Login')}
                  </div>
                  <div className='mt-3 space-y-2 text-sm'>
                    <div>
                      {t('Inviter')}:&nbsp;
                      <span className='text-muted-foreground'>
                        {profile.inviter_username ||
                          profile.inviter_display_name ||
                          (profile.inviter_id
                            ? `UID ${profile.inviter_id}`
                            : t('None'))}
                      </span>
                    </div>
                    <div>
                      {t('Invitee count')}:&nbsp;
                      <span className='text-muted-foreground'>
                        {profile.invitee_count ?? profile.aff_count ?? 0}
                      </span>
                    </div>
                    <div className='text-muted-foreground text-xs leading-6'>
                      {(profile.invitee_preview || [])
                        .slice(0, 5)
                        .map(
                          (item) =>
                            item.username ||
                            item.display_name ||
                            `UID ${item.user_id}`
                        )
                        .join(', ') || t('No invited users yet')}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className='text-base'>
                {t('Effective groups and manual lanes')}
              </CardTitle>
              <CardDescription>
                {t(
                  'This section helps admins explain which API groups are currently open because of community binding, primary binding, paid package, or manual override.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='grid gap-4 lg:grid-cols-3'>
                <div className='border-border/70 bg-muted/30 space-y-2 rounded-2xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('Effective groups')}
                  </div>
                  <BadgeListCell
                    max={3}
                    items={effectiveGroups.map((group) => (
                      <StatusBadge
                        key={group}
                        label={group}
                        variant='neutral'
                        copyable={false}
                      />
                    ))}
                  />
                </div>
                <div className='border-border/70 bg-muted/30 space-y-2 rounded-2xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('Manual override groups')}
                  </div>
                  <BadgeListCell
                    max={3}
                    items={(profile.manual_override_groups || []).map(
                      (group) => (
                        <StatusBadge
                          key={group}
                          label={group}
                          variant='warning'
                          copyable={false}
                        />
                      )
                    )}
                  />
                </div>
                <div className='border-border/70 bg-muted/30 space-y-2 rounded-2xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('Primary binding result')}
                  </div>
                  <div className='flex flex-wrap gap-2'>
                    {boolBadge(profile.primary_bound, {
                      on: t('Primary bound'),
                      off: t('Primary not bound'),
                    })}
                    <StatusBadge
                      label={`${t('Platform')}: ${
                        profile.primary_platform === 'qq'
                          ? 'QQ'
                          : profile.primary_platform === 'tg'
                            ? 'TG'
                            : profile.primary_platform || '—'
                      }`}
                      variant='info'
                      copyable={false}
                    />
                    <StatusBadge
                      label={`${t('Matched group')}: ${
                        profile.matched_primary_group_id || '—'
                      }`}
                      variant='neutral'
                      copyable={false}
                    />
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </Dialog>
  )
}
