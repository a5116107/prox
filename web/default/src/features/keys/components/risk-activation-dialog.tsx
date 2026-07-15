/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useMemo } from 'react'
import {
  AlertTriangle,
  Copy,
  ExternalLink,
  Loader2,
  MessageCircleMore,
  RefreshCw,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { ApiKey } from '../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKey: ApiKey | null
  bindCode: string
  expiresAt: number
  onRegenerate?: () => Promise<void>
  regenerating?: boolean
}

export function RiskActivationDialog({
  open,
  onOpenChange,
  apiKey,
  bindCode,
  expiresAt,
  onRegenerate,
  regenerating = false,
}: Props) {
  const { t } = useTranslation()

  const commandText = useMemo(() => {
    if (!bindCode) return ''
    return `${t('Bind command prefix')} ${bindCode}`
  }, [bindCode, t])

  const expireText = useMemo(() => {
    if (!expiresAt) return '-'
    return new Date(expiresAt * 1000).toLocaleString()
  }, [expiresAt])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <button
          type='button'
          aria-label={t('Close')}
          onClick={() => onOpenChange(false)}
          className='ring-offset-background focus:ring-ring absolute top-4 right-4 rounded-sm opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-none'
        >
          <X className='size-4' />
        </button>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <AlertTriangle className='size-5 text-amber-500' />
            {t('QQ activation required')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'This API key belongs to a high-risk account. It stays disabled until you complete QQ binding activation.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <Alert>
            <AlertDescription className='space-y-2 text-sm'>
              <div>
                {t('Key name')}:{' '}
                <span className='font-medium'>{apiKey?.name || '-'}</span>
              </div>
              <div>
                {t('Activation source')}:{' '}
                <span className='font-medium'>{t('QQ group')}</span>
              </div>
              <div>
                {t('Expires at')}:{' '}
                <span className='font-medium'>{expireText}</span>
              </div>
            </AlertDescription>
          </Alert>

          <div className='bg-muted/30 rounded-xl border p-4'>
            <div className='text-muted-foreground mb-2 text-xs'>
              {t('Send this command in the QQ group to activate the key')}
            </div>
            <div className='bg-background flex items-center gap-2 rounded-lg border px-3 py-3 font-mono text-sm'>
              <span className='flex-1 break-all'>{commandText || '-'}</span>
              <Button
                type='button'
                size='icon-sm'
                variant='ghost'
                disabled={!commandText}
                onClick={async () => {
                  if (!commandText) return
                  const ok = await copyToClipboard(commandText)
                  if (ok) toast.success(t('Copied'))
                }}
              >
                <Copy className='size-4' />
              </Button>
            </div>
          </div>

          <div className='rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-100'>
            {t(
              'After QQ binding succeeds, the platform will automatically enable this API key. You do not need to edit the key again.'
            )}
          </div>
        </div>

        <DialogFooter className='flex-col gap-2 sm:flex-col sm:justify-start'>
          <div className='flex w-full flex-col gap-2 sm:flex-row sm:justify-end'>
            <Button
              type='button'
              variant='secondary'
              onClick={() =>
                window.open(
                  '/system-settings/operations/community-ops',
                  '_blank',
                  'noopener,noreferrer'
                )
              }
            >
              <ExternalLink className='mr-2 size-4' />
              {t('Open community binding guide')}
            </Button>
            {onRegenerate ? (
              <Button
                type='button'
                variant='outline'
                disabled={regenerating}
                onClick={() => void onRegenerate()}
              >
                {regenerating ? (
                  <Loader2 className='mr-2 size-4 animate-spin' />
                ) : (
                  <RefreshCw className='mr-2 size-4' />
                )}
                {t('Regenerate bind code')}
              </Button>
            ) : null}
            <Button
              type='button'
              disabled={!commandText}
              onClick={async () => {
                if (!commandText) return
                const ok = await copyToClipboard(commandText)
                if (ok) toast.success(t('Copied'))
              }}
            >
              <MessageCircleMore className='mr-2 size-4' />
              {t('Copy binding command')}
            </Button>
          </div>
          <Button
            type='button'
            variant='ghost'
            onClick={() => onOpenChange(false)}
            className='w-full sm:w-auto sm:self-end'
          >
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
