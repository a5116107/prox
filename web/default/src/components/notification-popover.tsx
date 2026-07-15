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
import type { TFunction } from 'i18next'
import { Bell, Megaphone, Send, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getAnnouncementColorClass } from '@/lib/colors'
import { formatDateTimeObject } from '@/lib/time'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Markdown } from '@/components/ui/markdown'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const NOTICE_SKIN = 'prox' as const

interface AnnouncementItem {
  type?: string
  content?: string
  extra?: string
  publishDate?: string | Date
  forcePopup?: boolean | string | number
  force_popup?: boolean | string | number
  forced?: boolean | string | number
  qqGroup?: string
  qq_group?: string
  qqGroupLink?: string
  qq_group_link?: string
  communityType?: string
  community_type?: string
  communityValue?: string
  community_value?: string
  communityLink?: string
  community_link?: string
  tgGroup?: string
  tg_group?: string
  tgGroupLink?: string
  tg_group_link?: string
  telegram?: string
  telegramLink?: string
  telegram_link?: string
}

function normalizeBoolean(value: unknown): boolean {
  return value === true || value === 'true' || value === 1 || value === '1'
}

const QQ_GROUP_PATTERN =
  /(^|[\s，。；;、（(【[])(Q群|QQ群|qq群|qq\s*群|QQ\s*群|qq|QQ)\s*[：: ]\s*(\d{5,12})/g
const QQ_GROUP_EXTRACT_PATTERN =
  /(?:Q群|QQ群|qq群|qq\s*群|QQ\s*群|qq|QQ)\s*[：: ]\s*(\d{5,12})/i

function buildQqGroupLink(group: string): string {
  return `mqqapi://card/show_pslcard?src_type=internal&version=1&uin=${encodeURIComponent(group)}&card_type=group&source=qrcode`
}

function buildTelegramLink(value: string): string {
  const raw = String(value || '').trim()
  if (!raw) return ''
  if (/^https?:\/\//i.test(raw) || /^tg:\/\//i.test(raw)) return raw
  return `https://t.me/${raw.replace(/^@+/, '')}`
}

function extractQqGroup(text = ''): string {
  return String(text || '').match(QQ_GROUP_EXTRACT_PATTERN)?.[1] || ''
}

function escapeAttr(value = ''): string {
  return String(value).replace(/&/g, '&amp;').replace(/"/g, '&quot;')
}

function normalizeCommunityType(value: unknown): 'qq' | 'tg' {
  const normalized = String(value || '').toLowerCase()
  return normalized === 'tg' || normalized === 'telegram' ? 'tg' : 'qq'
}

const SITE_PRIMARY_COMMUNITY_INFO = {
  type: 'qq' as const,
  value: '925249987',
  link: buildQqGroupLink('925249987'),
  label: 'QQ',
  joinText: '立即加入 QQ 群',
}

function getCommunityInfo(item: AnnouncementItem = {}) {
  const inferredType =
    item.tgGroup ||
    item.tg_group ||
    item.telegram ||
    item.telegramLink ||
    item.telegram_link
      ? 'tg'
      : 'qq'
  const type = normalizeCommunityType(
    item.communityType || item.community_type || inferredType
  )
  const value =
    item.communityValue ||
    item.community_value ||
    (type === 'tg'
      ? item.tgGroup || item.tg_group || item.telegram || ''
      : item.qqGroup ||
        item.qq_group ||
        extractQqGroup(item.content) ||
        extractQqGroup(item.extra) ||
        '')
  const link =
    item.communityLink ||
    item.community_link ||
    (type === 'tg'
      ? item.tgGroupLink ||
        item.tg_group_link ||
        item.telegramLink ||
        item.telegram_link ||
        buildTelegramLink(value)
      : item.qqGroupLink ||
        item.qq_group_link ||
        (value ? buildQqGroupLink(value) : ''))
  const label = type === 'tg' ? 'TG' : 'QQ'
  const joinText = type === 'tg' ? '立即加入 TG 群' : '立即加入 QQ 群'
  return { type, value, link, label, joinText }
}

function getPrimaryCommunityInfo(announcements: AnnouncementItem[]) {
  for (const item of announcements) {
    const info = getCommunityInfo(item)
    if (info.link) return info
  }
  return { ...SITE_PRIMARY_COMMUNITY_INFO }
}

function linkifyQqGroups(text = '', item: AnnouncementItem = {}): string {
  const source = String(text || '')
  if (!source) return source

  return source.replace(QQ_GROUP_PATTERN, (_match, prefix, label, group) => {
    const configured = getCommunityInfo({ ...item, communityType: 'qq' })
    const link =
      configured.link && (!configured.value || configured.value === group)
        ? configured.link
        : buildQqGroupLink(group)
    return `${prefix}<a href="${escapeAttr(link)}" target="_blank" rel="noopener noreferrer">${label}：${group}</a>`
  })
}

function openCommunityLink(link: string) {
  if (!link) return
  if (/^mqqapi:/i.test(link) || /^tg:\/\//i.test(link)) {
    window.location.href = link
    return
  }
  window.open(link, '_blank', 'noopener,noreferrer')
}

interface NotificationPopoverProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  unreadCount: number
  activeTab: 'notice' | 'announcements'
  onTabChange: (tab: 'notice' | 'announcements') => void
  notice: string
  announcements: AnnouncementItem[]
  loading: boolean
  className?: string
}

/**
 * Get relative time string from a date
 */
function getRelativeTime(publishDate: string | Date, t: TFunction): string {
  if (!publishDate) return ''

  const now = new Date()
  const pubDate = new Date(publishDate)

  // If invalid date, return original string
  if (isNaN(pubDate.getTime()))
    return typeof publishDate === 'string' ? publishDate : ''

  const diffMs = now.getTime() - pubDate.getTime()
  const diffSeconds = Math.floor(diffMs / 1000)
  const diffMinutes = Math.floor(diffSeconds / 60)
  const diffHours = Math.floor(diffMinutes / 60)
  const diffDays = Math.floor(diffHours / 24)
  const diffWeeks = Math.floor(diffDays / 7)
  const diffMonths = Math.floor(diffDays / 30)
  const diffYears = Math.floor(diffDays / 365)

  // If future time, show specific date
  if (diffMs < 0) return formatDateTimeObject(pubDate)

  // Return relative time based on difference
  if (diffSeconds < 60) return t('Just now')
  if (diffMinutes < 60)
    return diffMinutes === 1
      ? t('1 minute ago')
      : t('{{count}} minutes ago', { count: diffMinutes })
  if (diffHours < 24)
    return diffHours === 1
      ? t('1 hour ago')
      : t('{{count}} hours ago', { count: diffHours })
  if (diffDays < 7)
    return diffDays === 1
      ? t('1 day ago')
      : t('{{count}} days ago', { count: diffDays })
  if (diffWeeks < 4)
    return diffWeeks === 1
      ? t('1 week ago')
      : t('{{count}} weeks ago', { count: diffWeeks })
  if (diffMonths < 12)
    return diffMonths === 1
      ? t('1 month ago')
      : t('{{count}} months ago', { count: diffMonths })
  if (diffYears < 2) return t('1 year ago')

  // Over 2 years, show specific date
  return formatDateTimeObject(pubDate)
}

/**
 * Announcement status dot indicator
 */
function AnnouncementDot({ type }: { type?: string }) {
  return (
    <span
      className={cn(
        'mt-1.5 inline-block size-2 shrink-0 rounded-full',
        getAnnouncementColorClass(type)
      )}
    />
  )
}

/**
 * Empty state component
 */
function EmptyState({
  icon,
  title,
  description,
}: {
  icon: React.ReactNode
  title: string
  description?: string
}) {
  return (
    <Empty className='min-h-48 border-0 p-4'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>{icon}</EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description ? (
          <EmptyDescription>{description}</EmptyDescription>
        ) : null}
      </EmptyHeader>
    </Empty>
  )
}

/**
 * Notice tab content
 */
function NoticeContent({
  notice,
  loading,
  t,
}: {
  notice: string
  loading: boolean
  t: TFunction
}) {
  if (loading) {
    return (
      <EmptyState
        icon={<Bell />}
        title={t('Loading...')}
        description={t('Latest platform updates and notices')}
      />
    )
  }

  if (!notice) {
    return (
      <EmptyState icon={<Bell />} title={t('No announcements at this time')} />
    )
  }

  return (
    <ScrollArea className='h-[min(52vh,28rem)] pr-3'>
      <Markdown>{linkifyQqGroups(notice)}</Markdown>
    </ScrollArea>
  )
}

/**
 * Announcements tab content
 */
function AnnouncementsContent({
  announcements,
  loading,
  t,
  strong = false,
}: {
  announcements: AnnouncementItem[]
  loading: boolean
  t: TFunction
  strong?: boolean
}) {
  if (loading) {
    return (
      <EmptyState
        icon={<Megaphone />}
        title={t('Loading...')}
        description={t('Latest platform updates and notices')}
      />
    )
  }

  if (announcements.length === 0) {
    return (
      <EmptyState icon={<Megaphone />} title={t('No system announcements')} />
    )
  }

  return (
    <ScrollArea
      className={cn(
        strong ? 'h-[min(62vh,34rem)] pr-4' : 'h-[min(52vh,28rem)] pr-3'
      )}
    >
      <div className={cn('flex flex-col', strong && 'gap-3')}>
        {announcements.map((item, idx) => {
          const publishDate = item.publishDate
            ? new Date(item.publishDate)
            : null
          const relativeTime = publishDate
            ? getRelativeTime(publishDate, t)
            : ''
          const absoluteTime = publishDate
            ? formatDateTimeObject(publishDate)
            : ''
          return (
            <div key={idx}>
              <div
                className={cn(
                  strong
                    ? 'rounded-2xl border border-white/10 bg-white/[0.045] px-4 py-4 shadow-[0_16px_40px_-28px_rgba(0,0,0,0.85)] ring-1 ring-white/[0.04]'
                    : 'py-3'
                )}
              >
                <div className='flex items-start gap-3'>
                  <span
                    className={cn(
                      strong &&
                        'bg-background ring-primary/35 before:bg-primary relative mt-1 flex size-4 items-center justify-center rounded-full ring-2 before:absolute before:size-2 before:rounded-full before:shadow-[0_0_18px_hsl(var(--primary))]'
                    )}
                  >
                    <AnnouncementDot type={item.type} />
                  </span>
                  <div className='flex min-w-0 flex-1 flex-col gap-2.5'>
                    <div
                      className={cn(
                        strong
                          ? 'text-foreground [&_a]:text-primary text-[15px] leading-7 font-semibold [&_a]:underline [&_a]:underline-offset-4'
                          : 'text-sm'
                      )}
                    >
                      <Markdown>
                        {linkifyQqGroups(item.content || '', item)}
                      </Markdown>
                    </div>

                    {item.extra ? (
                      <div
                        className={cn(
                          strong
                            ? 'text-muted-foreground rounded-xl bg-black/10 p-3 text-xs leading-6 dark:bg-white/[0.035]'
                            : 'text-muted-foreground text-xs'
                        )}
                      >
                        <Markdown>{linkifyQqGroups(item.extra, item)}</Markdown>
                      </div>
                    ) : null}

                    {absoluteTime ? (
                      <div
                        className={cn(
                          strong
                            ? 'text-muted-foreground/80 text-xs font-medium'
                            : 'text-muted-foreground text-xs'
                        )}
                      >
                        {relativeTime ? `${relativeTime} • ` : null}
                        {absoluteTime}
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
              {!strong && idx < announcements.length - 1 ? <Separator /> : null}
            </div>
          )
        })}
      </div>
    </ScrollArea>
  )
}

function hasForcePopupAnnouncements(
  announcements: AnnouncementItem[]
): boolean {
  return announcements.some(
    (item) =>
      normalizeBoolean(item.forcePopup) ||
      normalizeBoolean(item.force_popup) ||
      normalizeBoolean(item.forced)
  )
}

function StrongAnnouncementDialog({
  open,
  onOpenChange,
  announcements,
  loading,
  t,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  announcements: AnnouncementItem[]
  loading: boolean
  t: TFunction
}) {
  const primaryCommunityInfo = getPrimaryCommunityInfo(announcements)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        data-notice-skin={NOTICE_SKIN}
        className='notice-skin-prox-terminal-v2 max-h-[min(92vh,860px)] overflow-hidden border-cyan-300/25 bg-slate-950/94 p-0 text-cyan-50 shadow-[0_34px_130px_-42px_rgba(0,0,0,0.98),0_0_0_1px_rgba(34,211,238,0.12)] ring-cyan-200/10 backdrop-blur-2xl sm:max-w-3xl'
      >
        <div className='pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(125,211,252,0.055)_1px,transparent_1px),linear-gradient(90deg,rgba(125,211,252,0.05)_1px,transparent_1px),radial-gradient(circle_at_18%_0%,rgba(56,189,248,0.24),transparent_32%),radial-gradient(circle_at_88%_12%,rgba(16,185,129,0.16),transparent_34%)] bg-[length:28px_28px,28px_28px,auto,auto]' />
        <div className='relative p-6 sm:p-8'>
          <DialogHeader className='gap-3 border-b border-white/10 pb-5'>
            <div className='flex items-center gap-3'>
              <span className='flex size-11 items-center justify-center rounded-2xl bg-cyan-400/15 text-cyan-200 ring-1 ring-cyan-300/30'>
                <Megaphone className='size-5' />
              </span>
              <div>
                <div className='text-[11px] font-black tracking-[0.18em] text-cyan-200 uppercase'>
                  AI PROX CONTROL
                </div>
                <DialogTitle className='text-2xl font-black tracking-tight text-white sm:text-3xl'>
                  {t('System Announcements')}
                </DialogTitle>
                <DialogDescription className='mt-1 text-sm font-medium text-zinc-300'>
                  {t('Latest platform updates and notices')}
                </DialogDescription>
              </div>
            </div>
          </DialogHeader>

          <div className='relative mt-5'>
            <AnnouncementsContent
              announcements={announcements}
              loading={loading}
              t={t}
              strong
            />
          </div>

          <div className='mt-5 flex flex-col gap-3 border-t border-white/10 pt-5 sm:flex-row sm:items-center sm:justify-between'>
            {primaryCommunityInfo.link ? (
              <div className='flex flex-1 flex-col gap-2 rounded-2xl border border-cyan-300/25 bg-cyan-300/10 p-3 ring-1 ring-white/10 sm:flex-row sm:items-center sm:justify-between'>
                <div className='min-w-0'>
                  <div className='text-sm font-black text-cyan-100'>
                    {t('第一时间获取线路和模型状态')}
                  </div>
                  <div className='text-xs font-medium text-zinc-300'>
                    {primaryCommunityInfo.type === 'tg'
                      ? t('点击右侧按钮加入 Telegram，公告更新会同步通知')
                      : t('点击右侧按钮加入官方群，公告更新会同步通知')}
                  </div>
                </div>
                <Button
                  className='h-12 shrink-0 rounded-full bg-gradient-to-r from-sky-400 via-cyan-300 to-emerald-300 px-6 text-base font-black text-slate-950 shadow-[0_18px_54px_-14px_rgba(34,211,238,0.95)] hover:from-sky-300 hover:to-emerald-200'
                  onClick={() => openCommunityLink(primaryCommunityInfo.link)}
                >
                  {primaryCommunityInfo.type === 'tg' ? (
                    <Send className='mr-2 size-5' />
                  ) : (
                    <Users className='mr-2 size-5' />
                  )}
                  {primaryCommunityInfo.value
                    ? `${t(primaryCommunityInfo.type === 'tg' ? '立即加 TG' : '立即加群')} ${primaryCommunityInfo.value}`
                    : t(
                        primaryCommunityInfo.type === 'tg'
                          ? '立即加 TG'
                          : '立即加群'
                      )}
                </Button>
              </div>
            ) : null}
            <Button
              className='h-11 rounded-full px-6 font-bold shadow-[0_14px_36px_-18px_rgba(255,255,255,0.55)]'
              onClick={() => onOpenChange(false)}
            >
              {t('Close')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

/**
 * Notification popover with Notice and Announcements tabs
 */
export function NotificationPopover({
  open,
  onOpenChange,
  unreadCount,
  activeTab,
  onTabChange,
  notice,
  announcements,
  loading,
  className,
}: NotificationPopoverProps) {
  const { t } = useTranslation()
  const strongDialog =
    activeTab === 'announcements' && hasForcePopupAnnouncements(announcements)

  return (
    <>
      <StrongAnnouncementDialog
        open={open && strongDialog}
        onOpenChange={onOpenChange}
        announcements={announcements}
        loading={loading}
        t={t}
      />

      <Popover open={open && !strongDialog} onOpenChange={onOpenChange}>
        <PopoverTrigger
          render={
            <Button
              variant='ghost'
              size='icon'
              className={cn('relative size-9', className)}
              aria-label={t('Notifications')}
            />
          }
        >
          <Bell className='size-[1.2rem]' />
          {unreadCount > 0 ? (
            <Badge
              variant='destructive'
              className='absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center px-1 text-[10px] font-semibold tabular-nums'
            >
              {unreadCount > 99 ? '99+' : unreadCount}
            </Badge>
          ) : null}
        </PopoverTrigger>

        <PopoverContent
          align='end'
          sideOffset={8}
          className='w-[min(26rem,calc(100vw-1rem))] gap-3 p-3'
        >
          <PopoverHeader className='gap-1 px-1'>
            <PopoverTitle>{t('System Announcements')}</PopoverTitle>
            <p className='text-muted-foreground text-xs'>
              {t('Latest platform updates and notices')}
            </p>
          </PopoverHeader>

          <Tabs
            value={activeTab}
            onValueChange={onTabChange as (value: string) => void}
          >
            <TabsList className='grid w-full grid-cols-2'>
              <TabsTrigger value='notice' className='gap-1.5'>
                <Bell className='size-3.5' />
                {t('Notice')}
              </TabsTrigger>
              <TabsTrigger value='announcements' className='gap-1.5'>
                <Megaphone className='size-3.5' />
                {t('Timeline')}
              </TabsTrigger>
            </TabsList>

            <TabsContent value='notice' className='mt-2'>
              <NoticeContent notice={notice} loading={loading} t={t} />
            </TabsContent>

            <TabsContent value='announcements' className='mt-2'>
              <AnnouncementsContent
                announcements={announcements}
                loading={loading}
                t={t}
              />
            </TabsContent>
          </Tabs>

          <div className='flex justify-end'>
            <Button size='sm' onClick={() => onOpenChange(false)}>
              {t('Close')}
            </Button>
          </div>
        </PopoverContent>
      </Popover>
    </>
  )
}
