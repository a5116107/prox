/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useEffect, useState } from 'react'
import {
  ExternalLink,
  Radio,
  MessageSquare,
  Users,
  Activity,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { SettingsSection } from '../components/settings-section'
import { AccessControlSection } from './access-control-section'
import { CommunityBotSection } from './community-bot-section'
import { CommunityGateSection } from './community-gate-section'

type Props = {
  defaultValues: Record<string, string | number | boolean>
}

type CommunityBotStatus = {
  enabled?: boolean
  configured?: boolean
  authorized?: boolean
  community_host?: string
  room_id?: string
  bot_username?: string
  last_scanned_at?: number
}

const zhCommunityOpsText: Record<string, string> = {
  'Community Operations Manager': '社区运营管家',
  'Manage community automation, room invitations, daily activity rewards, anti-spam rules, and room notifications in one place.':
    '集中管理社区自动化、房间邀请、签到验牌、每日活跃奖励、阅后即焚与房间通知。',
  'Independent per site': '按站点独立',
  'Protocol based': '协议直连',
  'Realtime streaming': '实时 streaming',
  'Bot account': '机器人账号',
  'Community room': '社区房间',
  'Last scan': '最近扫描',
  Enabled: '已启用',
  Disabled: '未启用',
  Authorized: '已授权',
  'Not authorized': '未授权',
  Never: '从未',
  'Community room (Misskey)': '社区房间 (Misskey)',
  'Sign-in / verify + burn-after-read, realtime via streaming.':
    '签到 / 验牌 + 阅后即焚，通过 streaming 实时响应。',
  'QQ / TG groups': 'QQ / TG 群',
  'Same sign-in & verify via bot bridge, channel-isolated quota.':
    '通过 bot 桥接同样支持签到 / 验牌，按渠道独立额度。',
  'Quick links': '快捷入口',
  'Game admin panel': '游戏管理面板',
  'Bind code page': '绑定码页面',
}

function formatScanTime(ts?: number, neverText = 'Never') {
  if (!ts || ts <= 0) return neverText
  try {
    return new Date(ts * 1000).toLocaleString()
  } catch {
    return neverText
  }
}

export function CommunityOpsSection({ defaultValues }: Props) {
  const { t, i18n } = useTranslation()
  const tr = (key: string) =>
    i18n.language?.toLowerCase().startsWith('zh')
      ? zhCommunityOpsText[key] || t(key)
      : t(key)

  const [status, setStatus] = useState<CommunityBotStatus | null>(null)

  useEffect(() => {
    let alive = true
    api
      .get('/api/community-bot/status')
      .then((res) => {
        if (alive && res?.data?.data) setStatus(res.data.data)
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])

  const enabled = Boolean(status?.enabled)
  const authorized = Boolean(status?.authorized)

  return (
    <SettingsSection title={tr('Community Operations Manager')}>
      {/* 顶部：项目语境状态条 */}
      <Card className='border-primary/20 overflow-hidden'>
        <CardHeader className='bg-primary/5'>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div className='space-y-1'>
              <CardTitle className='flex items-center gap-2'>
                <Activity className='text-primary h-5 w-5' />
                {tr('Community Operations Manager')}
              </CardTitle>
              <CardDescription>
                {tr(
                  'Manage community automation, room invitations, daily activity rewards, anti-spam rules, and room notifications in one place.'
                )}
              </CardDescription>
            </div>
            <div className='flex flex-wrap gap-2'>
              <Badge variant={enabled ? 'default' : 'secondary'}>
                {enabled ? tr('Enabled') : tr('Disabled')}
              </Badge>
              <Badge variant={authorized ? 'default' : 'outline'}>
                {authorized ? tr('Authorized') : tr('Not authorized')}
              </Badge>
              <Badge variant='outline'>{tr('Realtime streaming')}</Badge>
              <Badge variant='outline'>{tr('Independent per site')}</Badge>
            </div>
          </div>
        </CardHeader>

        {/* 运行指标条 */}
        <CardContent className='bg-border grid gap-px p-0 sm:grid-cols-3'>
          <div className='bg-card p-4'>
            <div className='text-muted-foreground flex items-center gap-2 text-xs'>
              <Radio className='h-3.5 w-3.5' />
              {tr('Bot account')}
            </div>
            <div className='mt-1 truncate font-medium'>
              {status?.bot_username ? `@${status.bot_username}` : '-'}
            </div>
          </div>
          <div className='bg-card p-4'>
            <div className='text-muted-foreground flex items-center gap-2 text-xs'>
              <Users className='h-3.5 w-3.5' />
              {tr('Community room')}
            </div>
            <div className='mt-1 truncate font-mono text-sm'>
              {status?.room_id || '-'}
            </div>
          </div>
          <div className='bg-card p-4'>
            <div className='text-muted-foreground flex items-center gap-2 text-xs'>
              <Activity className='h-3.5 w-3.5' />
              {tr('Last scan')}
            </div>
            <div className='mt-1 truncate text-sm'>
              {formatScanTime(status?.last_scanned_at, tr('Never'))}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 三端语境：社区房间 + QQ/TG 群（2 列非对称，替代三等分反模式） */}
      <div className='grid gap-3 md:grid-cols-2'>
        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='flex items-center gap-2 text-base'>
              <Users className='text-primary h-4 w-4' />
              {tr('Community room (Misskey)')}
            </CardTitle>
            <CardDescription>
              {tr(
                'Sign-in / verify + burn-after-read, realtime via streaming.'
              )}
            </CardDescription>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className='pb-3'>
            <CardTitle className='flex items-center gap-2 text-base'>
              <MessageSquare className='text-primary h-4 w-4' />
              {tr('QQ / TG groups')}
            </CardTitle>
            <CardDescription>
              {tr(
                'Same sign-in & verify via bot bridge, channel-isolated quota.'
              )}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>

      {/* 快捷入口：把分散的 game-admin / bind-page 收拢到主后台 */}
      <Card>
        <CardHeader className='pb-3'>
          <CardTitle className='text-muted-foreground text-sm'>
            {tr('Quick links')}
          </CardTitle>
        </CardHeader>
        <CardContent className='flex flex-wrap gap-2 pt-0'>
          <a
            className={buttonVariants({ variant: 'outline', size: 'sm' })}
            href='/game-admin'
            target='_blank'
            rel='noreferrer'
          >
            <ExternalLink className='mr-1.5 h-3.5 w-3.5' />
            {tr('Game admin panel')}
          </a>
          <a
            className={buttonVariants({ variant: 'outline', size: 'sm' })}
            href='/api/agent/chatops/bind-page'
            target='_blank'
            rel='noreferrer'
          >
            <ExternalLink className='mr-1.5 h-3.5 w-3.5' />
            {tr('Bind code page')}
          </a>
        </CardContent>
      </Card>

      <CommunityGateSection defaultValues={defaultValues} />

      <div id='community-access-control' className='scroll-mt-20'>
        <AccessControlSection defaultValues={defaultValues} embedded />
      </div>

      <CommunityBotSection defaultValues={defaultValues} />
    </SettingsSection>
  )
}
