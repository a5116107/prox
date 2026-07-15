/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import type {
  CommunityBotStats,
  CommunityBotStatus,
} from './community-bot-model'

type CommunityBotOverviewProps = {
  status: CommunityBotStatus | null
  stats: CommunityBotStats | null
  actionLoading: boolean
  onRefreshStatus: () => void | Promise<void>
  onRefreshStats: () => void | Promise<void>
  onAuthorize: () => void | Promise<void>
  onTestMessage: () => void | Promise<void>
  onScan: () => void | Promise<void>
}

function formatCommunityUnix(value?: number) {
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString()
}

function formatCommunityNumber(value?: number) {
  return Number(value ?? 0).toLocaleString()
}

function formatCommunityQuota(value?: number) {
  const quota = Number(value ?? 0)
  return quota
    ? `${quota.toLocaleString()} (${(quota / 500000).toFixed(2)} USD)`
    : '0'
}

function StatusPill(props: { ok: boolean; text: string }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${props.ok ? 'bg-emerald-500/10 text-emerald-600' : 'bg-amber-500/10 text-amber-600'}`}
    >
      {props.text}
    </span>
  )
}

function CommunityBotStatusToolbar(props: CommunityBotOverviewProps) {
  const { t } = useTranslation()

  return (
    <div className='bg-muted/20 flex flex-wrap items-center gap-2 rounded-xl border p-3 text-sm'>
      <StatusPill
        ok={Boolean(props.status?.configured)}
        text={props.status?.configured ? t('Configured') : t('Not configured')}
      />
      <StatusPill
        ok={Boolean(props.status?.authorized)}
        text={
          props.status?.authorized
            ? t('Bot authorized')
            : t('Bot not authorized')
        }
      />
      {props.status?.bot_username ? (
        <span className='text-muted-foreground'>
          {t('Bot')}: @{props.status.bot_username}
        </span>
      ) : null}
      {props.status?.last_scanned_at ? (
        <span className='text-muted-foreground'>
          {t('Last scan')}:{' '}
          {new Date(props.status.last_scanned_at * 1000).toLocaleString()}
        </span>
      ) : null}
      <div className='ml-auto flex flex-wrap gap-2'>
        <Button type='button' variant='outline' onClick={props.onRefreshStatus}>
          {t('Refresh')}
        </Button>
        <Button
          type='button'
          variant='outline'
          disabled={props.actionLoading}
          onClick={props.onAuthorize}
        >
          {t('Authorize bot')}
        </Button>
        <Button
          type='button'
          variant='outline'
          disabled={props.actionLoading || !props.status?.authorized}
          onClick={props.onTestMessage}
        >
          {t('Send test message')}
        </Button>
        <Button
          type='button'
          disabled={props.actionLoading || !props.status?.authorized}
          onClick={props.onScan}
        >
          {t('Scan now')}
        </Button>
      </div>
    </div>
  )
}

function CommunityBotSummary(props: { stats: CommunityBotStats | null }) {
  const { t } = useTranslation()
  const rows = [
    [t('Daily users'), formatCommunityNumber(props.stats?.totals?.users)],
    [
      t('Counted messages'),
      formatCommunityNumber(props.stats?.totals?.message_count),
    ],
    [
      t('Rewarded count'),
      formatCommunityNumber(props.stats?.totals?.rewarded_count),
    ],
    [
      t('Rewarded quota'),
      formatCommunityQuota(props.stats?.totals?.rewarded_quota),
    ],
  ]

  return (
    <div className='grid gap-3 md:grid-cols-4'>
      {rows.map(([label, value]) => (
        <div key={label} className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>{label}</div>
          <div className='mt-1 font-medium'>{value}</div>
        </div>
      ))}
    </div>
  )
}

function CommunityBotDailyStats(props: { stats: CommunityBotStats | null }) {
  const { t } = useTranslation()
  const rows = props.stats?.stats ?? []

  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>{t('Daily message stats')}</div>
      <div className='overflow-hidden rounded-lg border'>
        {rows.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No community stats yet')}
          </div>
        ) : (
          <div className='divide-y'>
            {rows.slice(0, 8).map((row) => (
              <div
                key={`${row.user_id}-${row.provider_user_id}-${row.stat_date}`}
                className='grid gap-2 p-3 text-sm md:grid-cols-[1.2fr_.8fr_.8fr_.8fr]'
              >
                <div className='truncate'>
                  @{row.provider_user_id || row.user_id || '-'}
                </div>
                <div>
                  {t('Messages')}: {formatCommunityNumber(row.message_count)}
                </div>
                <div>
                  {t('Distinct')}: {formatCommunityNumber(row.distinct_texts)}
                </div>
                <div className='text-muted-foreground truncate'>
                  {row.last_message_id || '-'}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function CommunityBotRecentRewards(props: { stats: CommunityBotStats | null }) {
  const { t } = useTranslation()
  const rows = props.stats?.rewards ?? []

  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>{t('Recent reward records')}</div>
      <div className='overflow-hidden rounded-lg border'>
        {rows.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No reward records yet')}
          </div>
        ) : (
          <div className='divide-y'>
            {rows.slice(0, 8).map((row) => (
              <div
                key={`${row.id}-${row.user_id}-${row.created_at}`}
                className='grid gap-2 p-3 text-sm md:grid-cols-[1fr_.8fr_.8fr_1fr]'
              >
                <div>
                  {t('User ID')}: {row.user_id || '-'}
                </div>
                <div>
                  {t('Messages')}: {formatCommunityNumber(row.message_count)}
                </div>
                <div>{formatCommunityQuota(row.quota)}</div>
                <div className='text-muted-foreground'>
                  {formatCommunityUnix(row.created_at)}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export function CommunityBotOverview(props: CommunityBotOverviewProps) {
  const { t } = useTranslation()

  return (
    <>
      <CommunityBotStatusToolbar {...props} />
      <Card>
        <CardHeader>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div>
              <CardTitle>{t('Community bot stats')}</CardTitle>
              <CardDescription>
                {t(
                  'Review daily message counting and reward records from the active community bot engine.'
                )}
              </CardDescription>
            </div>
            <Button
              type='button'
              variant='secondary'
              onClick={props.onRefreshStats}
            >
              {t('Refresh stats')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          <CommunityBotSummary stats={props.stats} />
          <div className='grid gap-4 lg:grid-cols-2'>
            <CommunityBotDailyStats stats={props.stats} />
            <CommunityBotRecentRewards stats={props.stats} />
          </div>
        </CardContent>
      </Card>
    </>
  )
}
