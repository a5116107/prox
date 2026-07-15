/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useState, type ReactNode } from 'react'
import {
  CheckCircle2,
  ExternalLink,
  Link2,
  Loader2,
  RefreshCw,
  ShieldAlert,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { CommunityGateMeStatus } from '../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  status: CommunityGateMeStatus | null
  loading?: boolean
  onRefresh: (refresh?: boolean) => Promise<CommunityGateMeStatus | null>
  onRestore?: () => Promise<void>
  title?: string
  description?: string
}

function StepRow({
  done,
  icon,
  title,
  desc,
}: {
  done: boolean
  icon: ReactNode
  title: ReactNode
  desc: ReactNode
}) {
  return (
    <div
      className={cn(
        'rounded-lg border p-3',
        done
          ? 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900/40 dark:bg-emerald-950/20 dark:text-emerald-100'
          : 'border-amber-200 bg-amber-50 text-amber-950 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-100'
      )}
    >
      <div className='flex items-start gap-3'>
        <div className='mt-0.5 shrink-0'>
          {done ? <CheckCircle2 className='size-4' /> : icon}
        </div>
        <div className='min-w-0 space-y-1'>
          <div className='text-sm font-medium'>{title}</div>
          <div className='text-xs opacity-80'>{desc}</div>
        </div>
      </div>
    </div>
  )
}

export function CommunityGateDialog({
  open,
  onOpenChange,
  status,
  loading,
  onRefresh,
  onRestore,
  title,
  description,
}: Props) {
  const { t } = useTranslation()
  const [refreshing, setRefreshing] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const gate = status?.gate
  const hasBinding = gate?.has_oauth_binding ?? false
  const hasRoom = gate?.has_room_membership ?? false
  const frozenCount = status?.active_frozen_keys ?? 0
  const joinUrl = status?.join_url

  const handleBind = () => {
    window.location.href = status?.bind_url || '/profile?tab=bindings'
  }

  const handleJoin = () => {
    if (!joinUrl) {
      toast.error(t('Community room link is not configured'))
      return
    }
    window.open(joinUrl, '_blank', 'noopener')
  }

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      const next = await onRefresh(true)
      if (next?.compliant) {
        toast.success(t('Community verification passed'))
        if ((next.active_frozen_keys ?? 0) > 0 && onRestore) {
          setRestoring(true)
          await onRestore()
          setRestoring(false)
        }
        onOpenChange(false)
      } else {
        toast.info(t('Community verification is still incomplete'))
      }
    } finally {
      setRefreshing(false)
      setRestoring(false)
    }
  }

  const handleRestore = async () => {
    if (!onRestore) return
    setRestoring(true)
    try {
      await onRestore()
      onOpenChange(false)
    } finally {
      setRestoring(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <ShieldAlert className='size-5 text-amber-500' />
            {title || t('Community verification required')}
          </DialogTitle>
          <DialogDescription>
            {description ||
              status?.denied_message ||
              t(
                'Please complete community OAuth binding and join the required room before creating or enabling API keys.'
              )}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-3'>
          {loading ? (
            <div className='text-muted-foreground flex items-center gap-2 rounded-lg border p-3 text-sm'>
              <Loader2 className='size-4 animate-spin' />
              {t('Checking community verification status...')}
            </div>
          ) : (
            <>
              <StepRow
                done={hasBinding}
                icon={<Link2 className='size-4' />}
                title={t('Bind dc.hhhl.cc community account')}
                desc={
                  hasBinding
                    ? t('Community OAuth binding has been detected.')
                    : t('Bind the same community account used for site login.')
                }
              />
              <StepRow
                done={hasRoom}
                icon={<Users className='size-4' />}
                title={t('Join the site community room')}
                desc={
                  hasRoom
                    ? t('Required community room membership has been detected.')
                    : t(
                        'Open the community room link and join the room, then re-check.'
                      )
                }
              />
              {frozenCount > 0 && (
                <div className='rounded-lg border border-sky-200 bg-sky-50 p-3 text-sm text-sky-900 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-100'>
                  {t(
                    '{{count}} API key(s) were temporarily disabled by community verification.',
                    { count: frozenCount }
                  )}
                </div>
              )}
            </>
          )}
        </div>

        <DialogFooter className='flex-col gap-2 sm:flex-col sm:justify-start'>
          <div className='flex w-full flex-col gap-2 sm:flex-row sm:justify-end'>
            {!hasBinding && (
              <Button onClick={handleBind} className='gap-2'>
                <Link2 className='size-4' />
                {t('Go to OAuth binding')}
              </Button>
            )}
            {hasBinding && !hasRoom && (
              <Button onClick={handleJoin} className='gap-2'>
                <ExternalLink className='size-4' />
                {t('Join community room')}
              </Button>
            )}
            {status?.can_restore && (
              <Button
                onClick={handleRestore}
                disabled={restoring || refreshing}
                className='gap-2'
              >
                {restoring && <Loader2 className='size-4 animate-spin' />}
                {t('Restore disabled keys')}
              </Button>
            )}
            <Button
              variant='outline'
              onClick={handleRefresh}
              disabled={refreshing || restoring}
              className='gap-2'
            >
              {refreshing ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <RefreshCw className='size-4' />
              )}
              {t('Re-check')}
            </Button>
          </div>
          <Button
            variant='ghost'
            onClick={() => onOpenChange(false)}
            className='w-full sm:w-auto sm:self-end'
          >
            {t('Handle later')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
