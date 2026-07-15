/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useMemo } from 'react'
import { Copy, MessageCircleMore } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { ApiKey } from '../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKeys: ApiKey[]
}

export function RiskActivationBatchDialog({
  open,
  onOpenChange,
  apiKeys,
}: Props) {
  const { t } = useTranslation()
  const commandPrefix = t('Bind command prefix')
  const commands = useMemo(
    () =>
      apiKeys
        .filter((apiKey) => apiKey.risk_activation_bind_code)
        .map(
          (apiKey) =>
            `${apiKey.name}: ${commandPrefix} ${apiKey.risk_activation_bind_code}`
        ),
    [apiKeys, commandPrefix]
  )

  const copyText = async (text: string) => {
    const copied = await copyToClipboard(text)
    if (copied) toast.success(t('Copied'))
    else toast.error(t('Copy failed'))
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <MessageCircleMore className='size-5 text-amber-500' />
            {t('QQ activation required')}
          </DialogTitle>
          <DialogDescription>
            {t('Some API keys require action')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[min(58vh,560px)] pr-3'>
          <div className='divide-y overflow-hidden rounded-lg border'>
            {apiKeys.map((apiKey) => {
              const command = `${commandPrefix} ${apiKey.risk_activation_bind_code}`
              const expiresAt = apiKey.risk_activation_expires_at
                ? new Date(
                    apiKey.risk_activation_expires_at * 1000
                  ).toLocaleString()
                : '-'

              return (
                <div key={apiKey.id} className='space-y-3 p-4'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <div className='truncate text-sm font-medium'>
                        {apiKey.name}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {t('Expires at')}: {expiresAt}
                      </div>
                    </div>
                    <Button
                      type='button'
                      size='icon-sm'
                      variant='ghost'
                      disabled={!apiKey.risk_activation_bind_code}
                      aria-label={t('Copy binding command')}
                      onClick={() => void copyText(command)}
                    >
                      <Copy className='size-4' />
                    </Button>
                  </div>
                  <div className='bg-muted/40 rounded-md px-3 py-2 font-mono text-xs break-all'>
                    {apiKey.risk_activation_bind_code ? command : '-'}
                  </div>
                </div>
              )
            })}
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Close')}
          </Button>
          <Button
            type='button'
            disabled={commands.length === 0}
            onClick={() => void copyText(commands.join('\n'))}
          >
            <Copy className='mr-2 size-4' />
            {t('Copy All')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
