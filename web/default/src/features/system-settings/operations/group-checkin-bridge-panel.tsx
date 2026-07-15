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
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useOpsT } from './ops-i18n'
import { OpsModeBanner, OpsPanel, OpsStatusBadge } from './ops-shared'

type GameAdminGroup = {
  site?: string
  platform?: string
  group_id?: string
  label?: string
  enabled?: boolean
  config?: {
    traffic_role?: string
    invite_target?: {
      platform?: string
      group_id?: string
    }
    games?: {
      checkin?: {
        enabled?: boolean
        reward_min?: number
        reward_max?: number
        bonus_days?: number
        bonus_extra?: number
        daily_group_reward_limit?: number
        require_verify?: boolean
      }
    }
  }
}

type BudgetGroupMetric = {
  platform?: string
  group_id?: string
  game_rounds?: number
  game_players?: number
  reward_cost_quota?: number
  commission_quota?: number
  checkins?: number
  checkin_count?: number
}

type Props = {
  siteId?: string
  title: string
  description: string
  platformKinds?: Array<'community' | 'qq' | 'tg'>
}

type MetricSummary = {
  platforms: string[]
  rewardCostQuota: number
  commissionQuota: number
  rounds: number
  players: number
  checkins: number
}

type GroupCheckinConfig = NonNullable<
  NonNullable<NonNullable<GameAdminGroup['config']>['games']>['checkin']
>

type BridgeRow = {
  id: string
  group: GameAdminGroup
  groupId: string
  checkin: GroupCheckinConfig
  metrics?: MetricSummary
  trafficRole?: string
  inviteTarget?: NonNullable<GameAdminGroup['config']>['invite_target']
}

const EMPTY_GROUPS: GameAdminGroup[] = []
const EMPTY_METRICS_BY_GROUP: Record<string, MetricSummary> = {}

function quotaToUsd(quota?: number) {
  return `$${((Number(quota) || 0) / 500000).toFixed(2)}`
}

function roleTone(role?: string) {
  if (role === 'mainfield') return 'success' as const
  if (role === 'inbound_source') return 'info' as const
  return 'neutral' as const
}

function metricKeysOf(input: {
  site?: string
  platform?: string
  group_id?: string
  groupId?: string
}) {
  const groupId = String(input.group_id || input.groupId || '')
  const platform = String(input.platform || '')
  const site = String(input.site || '')
  const baseKey = `${platform}::${groupId}`
  return site ? [`${site}::${baseKey}`, baseKey] : [baseKey]
}

function buildGameAdminUrl(params: {
  siteId?: string
  platform?: string
  groupId?: string
  openCheckin?: boolean
}) {
  const query = new URLSearchParams()
  query.set('view', 'budget')
  if (params.siteId) query.set('site', params.siteId)
  if (params.platform) query.set('platform', params.platform)
  if (params.groupId) query.set('group_id', params.groupId)
  if (params.openCheckin) query.set('open', 'checkin')
  return `/game-admin/?${query.toString()}`
}

function formatTrafficRole(
  role: string | undefined,
  t: ReturnType<typeof useOpsT>
) {
  if (role === 'mainfield') return t('Main unlock')
  if (role === 'inbound_source') return t('Community intake')
  return role || t('Not reported')
}

function normalizeGroupPlatformKind(raw: string | undefined) {
  const value = String(raw || '')
    .trim()
    .toLowerCase()
  if (!value) return ''
  if (value === 'community' || value === 'dc' || value === 'hhhl')
    return 'community'
  if (value.includes('qq')) return 'qq'
  if (value.includes('tg') || value.includes('telegram')) return 'tg'
  return ''
}

function matchesPlatformKinds(
  rawPlatform: string | undefined,
  platformKinds: Props['platformKinds']
) {
  if (!platformKinds?.length) return true
  const kind = normalizeGroupPlatformKind(rawPlatform)
  return kind
    ? platformKinds.includes(kind as 'community' | 'qq' | 'tg')
    : false
}

function summarizeGroupMetrics(metricRows: BudgetGroupMetric[]) {
  return metricRows.reduce<Record<string, MetricSummary>>((acc, row) => {
    const groupId = String(row.group_id || '')
    if (!groupId) return acc
    const [primaryKey, ...aliasKeys] = metricKeysOf(row)
    if (!acc[primaryKey]) {
      acc[primaryKey] = {
        platforms: [],
        rewardCostQuota: 0,
        commissionQuota: 0,
        rounds: 0,
        players: 0,
        checkins: 0,
      }
    }
    const current = acc[primaryKey]
    for (const aliasKey of aliasKeys) acc[aliasKey] = current
    if (row.platform && !current.platforms.includes(row.platform)) {
      current.platforms.push(row.platform)
    }
    current.rewardCostQuota += Number(row.reward_cost_quota || 0)
    current.commissionQuota += Number(row.commission_quota || 0)
    current.rounds += Number(row.game_rounds || 0)
    current.players += Number(row.game_players || 0)
    current.checkins += Number(row.checkins || row.checkin_count || 0)
    return acc
  }, {})
}

function buildBridgeRows(
  groups: GameAdminGroup[],
  metricsByGroup: Record<string, MetricSummary>
): BridgeRow[] {
  return [...groups]
    .sort((a, b) => {
      const order = (value?: string) =>
        value === 'mainfield' ? 0 : value === 'inbound_source' ? 1 : 2
      return (
        order(a.config?.traffic_role) - order(b.config?.traffic_role) ||
        String(a.group_id || '').localeCompare(String(b.group_id || ''))
      )
    })
    .map((group) => {
      const groupId = String(group.group_id || '')
      const [siteKey, fallbackKey] = metricKeysOf(group)
      return {
        id: `${group.platform || 'unknown'}:${groupId}`,
        group,
        groupId,
        checkin: group.config?.games?.checkin || {},
        metrics: metricsByGroup[siteKey] || metricsByGroup[fallbackKey],
        trafficRole: group.config?.traffic_role,
        inviteTarget: group.config?.invite_target,
      }
    })
}

function useGroupQuotaBridge(
  siteId: string | undefined,
  platformKinds: Props['platformKinds'],
  t: ReturnType<typeof useOpsT>
) {
  const query = useQuery({
    queryKey: ['ops-group-quota-bridge', siteId ?? '', platformKinds ?? []],
    queryFn: async () => {
      const [groupsRes, budgetRes] = await Promise.all([
        api.get('/api/game-admin/groups', { skipBusinessError: true }),
        api.get('/api/game-admin/budget', { skipBusinessError: true }),
      ])
      const rawGroups = Array.isArray(groupsRes.data?.groups)
        ? groupsRes.data.groups
        : []
      const groups = rawGroups.filter(
        (row: GameAdminGroup) =>
          (!siteId || row.site === siteId) &&
          matchesPlatformKinds(row.platform, platformKinds)
      )
      const metricRows: BudgetGroupMetric[] =
        budgetRes.data?.fund?.observability?.group_metrics_today ||
        budgetRes.data?.fund?.group_metrics_today ||
        []
      return { groups, metricsByGroup: summarizeGroupMetrics(metricRows) }
    },
    retry: false,
    staleTime: 0,
  })
  const groups = query.data?.groups ?? EMPTY_GROUPS
  const metricsByGroup = query.data?.metricsByGroup ?? EMPTY_METRICS_BY_GROUP
  return {
    rows: useMemo(
      () => buildBridgeRows(groups, metricsByGroup),
      [groups, metricsByGroup]
    ),
    error: query.error
      ? query.error instanceof Error
        ? query.error.message
        : t('Failed to load group quota bridge')
      : '',
    loading: query.isFetching,
    refresh: query.refetch,
  }
}

function GroupIdentityCell({
  row,
  t,
}: {
  row: BridgeRow
  t: ReturnType<typeof useOpsT>
}) {
  return (
    <TableCell className='min-w-[220px] align-top'>
      <div className='space-y-1'>
        <div className='font-medium'>
          {row.group.label || row.groupId || t('Not reported')}
        </div>
        <div className='text-muted-foreground text-xs'>
          {row.group.platform || '-'} / {row.groupId || '-'}
        </div>
        {row.inviteTarget?.group_id ? (
          <div className='text-muted-foreground text-xs'>
            {t('Invite target')}: {row.inviteTarget.platform || '-'} /{' '}
            {row.inviteTarget.group_id}
          </div>
        ) : null}
      </div>
    </TableCell>
  )
}

function GroupStatusCell({
  row,
  t,
}: {
  row: BridgeRow
  t: ReturnType<typeof useOpsT>
}) {
  return (
    <TableCell className='min-w-[150px] align-top'>
      <div className='flex flex-wrap gap-2'>
        <OpsStatusBadge tone={roleTone(row.trafficRole)}>
          {formatTrafficRole(row.trafficRole, t)}
        </OpsStatusBadge>
        <OpsStatusBadge
          tone={row.group.enabled === false ? 'danger' : 'success'}
        >
          {row.group.enabled === false ? t('Disabled') : t('Enabled')}
        </OpsStatusBadge>
      </div>
    </TableCell>
  )
}

function GroupCheckinRuleCell({
  row,
  t,
}: {
  row: BridgeRow
  t: ReturnType<typeof useOpsT>
}) {
  const checkinEnabled = row.checkin.enabled !== false
  const dailyCap = Number(row.checkin.daily_group_reward_limit || 0)
  return (
    <TableCell className='min-w-[290px] align-top'>
      <div className='space-y-1 text-sm'>
        <div className='font-medium'>
          {checkinEnabled
            ? `${quotaToUsd(row.checkin.reward_min)} ~ ${quotaToUsd(row.checkin.reward_max)}`
            : t('Group override off')}
        </div>
        <div className='text-muted-foreground text-xs leading-5'>
          {t('Consecutive bonus')} {row.checkin.bonus_days || 0} {t('days')} /{' '}
          {quotaToUsd(row.checkin.bonus_extra)}
        </div>
        <div className='text-muted-foreground text-xs leading-5'>
          {row.checkin.require_verify === false
            ? t('No verify required')
            : t('Verify first')}
          {' · '}
          {dailyCap > 0
            ? `${t('Daily cap')} ${quotaToUsd(dailyCap)}`
            : t('No daily cap')}
        </div>
      </div>
    </TableCell>
  )
}

function GroupRuntimeCell({
  row,
  t,
}: {
  row: BridgeRow
  t: ReturnType<typeof useOpsT>
}) {
  return (
    <TableCell className='min-w-[260px] align-top'>
      {row.metrics ? (
        <div className='space-y-1 text-sm'>
          <div className='font-medium'>
            {t('Today reward cost')} {quotaToUsd(row.metrics.rewardCostQuota)}
          </div>
          <div className='text-muted-foreground text-xs leading-5'>
            {t('Commission')} {quotaToUsd(row.metrics.commissionQuota)} ·{' '}
            {t('Rounds')} {row.metrics.rounds} · {t('Players')}{' '}
            {row.metrics.players}
          </div>
          <div className='text-muted-foreground text-xs leading-5'>
            {t('Runtime channels')}: {row.metrics.platforms.join(', ') || '-'}
            {row.metrics.checkins > 0
              ? ` · ${t('Check-ins')} ${row.metrics.checkins}`
              : ''}
          </div>
        </div>
      ) : (
        <div className='text-muted-foreground text-sm'>
          {t('Budget data unavailable')}
        </div>
      )}
    </TableCell>
  )
}

function GroupActionsCell({
  row,
  t,
}: {
  row: BridgeRow
  t: ReturnType<typeof useOpsT>
}) {
  const editUrl = buildGameAdminUrl({
    siteId: row.group.site,
    platform: row.group.platform,
    groupId: row.groupId,
    openCheckin: true,
  })
  return (
    <TableCell className='min-w-[210px] text-right align-top'>
      <Button
        type='button'
        size='sm'
        variant='outline'
        onClick={() => window.open(editUrl, '_blank', 'noopener,noreferrer')}
      >
        {t('Edit in Game Admin')}
      </Button>
    </TableCell>
  )
}

function GroupCheckinBridgeRow({
  row,
  t,
}: {
  row: BridgeRow
  t: ReturnType<typeof useOpsT>
}) {
  return (
    <TableRow>
      <GroupIdentityCell row={row} t={t} />
      <GroupStatusCell row={row} t={t} />
      <GroupCheckinRuleCell row={row} t={t} />
      <GroupRuntimeCell row={row} t={t} />
      <GroupActionsCell row={row} t={t} />
    </TableRow>
  )
}

function GroupCheckinBridgeTable({
  rows,
  loading,
  t,
}: {
  rows: BridgeRow[]
  loading: boolean
  t: ReturnType<typeof useOpsT>
}) {
  return (
    <div className='overflow-x-auto'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Group')}</TableHead>
            <TableHead>{t('Role')}</TableHead>
            <TableHead>{t('Check-in rule')}</TableHead>
            <TableHead>{t('Runtime pressure')}</TableHead>
            <TableHead className='text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length > 0 ? (
            rows.map((row) => (
              <GroupCheckinBridgeRow key={row.id} row={row} t={t} />
            ))
          ) : (
            <TableRow>
              <TableCell
                colSpan={5}
                className='text-muted-foreground py-8 text-center'
              >
                {loading
                  ? t('Loading group quota bridge')
                  : t('No group quota data yet')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

export function GroupCheckinBridgePanel({
  siteId,
  title,
  description,
  platformKinds,
}: Props) {
  const t = useOpsT()
  const { rows, error, loading, refresh } = useGroupQuotaBridge(
    siteId,
    platformKinds,
    t
  )

  return (
    <>
      <OpsModeBanner
        tone='info'
        title={t(
          'One write path for defaults, one write path for group overrides'
        )}
        description={t(
          'This page stays read-only. Global check-in min/max, entry links, and success or failure copy stay in Billing > Check-in Rewards; each concrete community room / QQ / TG group override stays in Game Admin.'
        )}
      >
        <Button
          type='button'
          size='sm'
          onClick={() =>
            window.open(
              buildGameAdminUrl({ siteId, openCheckin: false }),
              '_blank',
              'noopener,noreferrer'
            )
          }
        >
          {t('Open group quota console')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={() => {
            window.open(
              '/system-settings/billing/checkin',
              '_blank',
              'noopener,noreferrer'
            )
          }}
        >
          {t('Open check-in defaults')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={loading}
          onClick={() => void refresh()}
        >
          {loading ? t('Refreshing...') : t('Refresh')}
        </Button>
      </OpsModeBanner>

      <OpsPanel className='mt-4' title={title} description={description}>
        {error ? (
          <div className='mb-4 rounded-2xl border border-rose-500/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-700 dark:text-rose-300'>
            {error}
          </div>
        ) : null}

        <GroupCheckinBridgeTable rows={rows} loading={loading} t={t} />
      </OpsPanel>
    </>
  )
}
