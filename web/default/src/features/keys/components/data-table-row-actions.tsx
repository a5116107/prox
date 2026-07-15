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
import { useCallback, useEffect, useState } from 'react'
import { type Row } from '@tanstack/react-table'
import type { AccessControlStatus } from '@/types/access-control'
import {
  Trash2,
  Edit,
  Power,
  PowerOff,
  ExternalLink,
  ArrowRightLeft,
  Copy,
  Link,
  Loader2,
  ShieldAlert,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getMyAccessControlStatus } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { AccessControlDialog } from '@/components/access-control-dialog'
import { useChatPresets } from '@/features/chat/hooks/use-chat-presets'
import { resolveChatUrl, type ChatPreset } from '@/features/chat/lib/chat-links'
import { sendToFluent } from '@/features/chat/lib/send-to-fluent'
import { regenerateRiskActivationCode, updateApiKeyStatus } from '../api'
import { API_KEY_STATUS, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { apiKeySchema } from '../types'
import { useApiKeys } from './api-keys-provider'
import { RiskActivationDialog } from './risk-activation-dialog'

function getServerAddress(): string {
  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw)
      if (status.server_address) return status.server_address as string
    }
  } catch {
    /* empty */
  }
  return window.location.origin
}

function encodeConnectionString(key: string, url: string): string {
  return JSON.stringify({
    _type: 'newapi_channel_conn',
    key,
    url,
  })
}

type DataTableRowActionsProps<TData> = {
  row: Row<TData>
}

export function DataTableRowActions<TData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const apiKey = apiKeySchema.parse(row.original)
  const {
    setOpen,
    setCurrentRow,
    triggerRefresh,
    setResolvedKey,
    resolveRealKey,
    resolvedKeys,
    loadingKeys,
  } = useApiKeys()
  const isEnabled = apiKey.status === API_KEY_STATUS.ENABLED
  const { chatPresets, serverAddress } = useChatPresets()
  const [isTogglingStatus, setIsTogglingStatus] = useState(false)
  const resolvedRealKey = resolvedKeys[apiKey.id]
  const isRealKeyLoading = Boolean(loadingKeys[apiKey.id])

  const hasChatPresets = chatPresets.length > 0
  const [accessDialogOpen, setAccessDialogOpen] = useState(false)
  const [riskActivationOpen, setRiskActivationOpen] = useState(false)
  const [activationCode, setActivationCode] = useState(
    apiKey.risk_activation_bind_code
  )
  const [activationExpiresAt, setActivationExpiresAt] = useState(
    apiKey.risk_activation_expires_at
  )
  const [isRegenerating, setIsRegenerating] = useState(false)
  const [accessControlStatus, setAccessControlStatus] =
    useState<AccessControlStatus | null>(null)

  useEffect(() => {
    setActivationCode(apiKey.risk_activation_bind_code)
    setActivationExpiresAt(apiKey.risk_activation_expires_at)
  }, [
    apiKey.id,
    apiKey.risk_activation_bind_code,
    apiKey.risk_activation_expires_at,
  ])

  const handleMenuOpenChange = useCallback(
    (open: boolean) => {
      if (open && !resolvedRealKey && !isRealKeyLoading) {
        void resolveRealKey(apiKey.id)
      }
    },
    [apiKey.id, isRealKeyLoading, resolvedRealKey, resolveRealKey]
  )

  const getCachedRealKey = useCallback(() => {
    if (resolvedRealKey) return resolvedRealKey
    void resolveRealKey(apiKey.id)
    toast.info(t('API key is loading, please try again in a moment'))
    return null
  }, [apiKey.id, resolvedRealKey, resolveRealKey, t])

  const handleOpenChatPreset = useCallback(
    async (preset: ChatPreset) => {
      const realKey = await resolveRealKey(apiKey.id)
      if (!realKey) return

      if (preset.type === 'fluent') {
        const success = sendToFluent(realKey, serverAddress)
        if (success) {
          toast.success(t('Sent the API key to FluentRead.'))
        } else {
          toast.info(
            t(
              'FluentRead extension not detected. Please ensure it is installed and active.'
            )
          )
        }
        return
      }

      const resolvedUrl = resolveChatUrl({
        template: preset.url,
        apiKey: realKey,
        serverAddress,
      })

      if (!resolvedUrl) {
        toast.error(t('Invalid chat link. Please contact your administrator.'))
        return
      }

      if (typeof window === 'undefined') return

      try {
        window.open(resolvedUrl, '_blank', 'noopener')
      } catch {
        window.location.href = resolvedUrl
      }
    },
    [resolveRealKey, apiKey.id, serverAddress, t]
  )

  const handleToggleStatus = async () => {
    const newStatus = isEnabled
      ? API_KEY_STATUS.DISABLED
      : API_KEY_STATUS.ENABLED

    setIsTogglingStatus(true)
    try {
      if (!isEnabled) {
        const access = await getMyAccessControlStatus()
        const next = access.data || null
        setAccessControlStatus(next)
        if (
          next?.block_token_enable &&
          (next?.access_level === 'none' ||
            (Array.isArray(next?.effective_groups) &&
              next.effective_groups.length === 0))
        ) {
          setAccessDialogOpen(true)
          return
        }
      }
      const result = await updateApiKeyStatus(apiKey, newStatus)
      if (result.success) {
        const message = isEnabled
          ? t(SUCCESS_MESSAGES.API_KEY_DISABLED)
          : t(SUCCESS_MESSAGES.API_KEY_ENABLED)
        toast.success(message)
        if (!isEnabled) {
          setAccessControlStatus(null)
          setAccessDialogOpen(false)
        }
        triggerRefresh()
      } else {
        const resultData = result.data as
          | (typeof apiKey & { access_control?: AccessControlStatus })
          | undefined
        const maybeAccess = resultData?.access_control
        if (maybeAccess) {
          setAccessControlStatus(maybeAccess)
          setAccessDialogOpen(true)
        }
        if (resultData?.risk_activation_required) {
          setActivationCode(resultData.risk_activation_bind_code || '')
          setActivationExpiresAt(resultData.risk_activation_expires_at || 0)
          setRiskActivationOpen(true)
        }
        toast.error(result.message || t(ERROR_MESSAGES.STATUS_UPDATE_FAILED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsTogglingStatus(false)
    }
  }

  const handleRegenerateRiskActivationCode = async () => {
    setIsRegenerating(true)
    try {
      const result = await regenerateRiskActivationCode(apiKey.id)
      if (!result.success || !result.data?.code) {
        toast.error(result.message || t('Operation failed'))
        return
      }
      setActivationCode(result.data.code)
      setActivationExpiresAt(result.data.expires_at)
      toast.success(t('Operation successful'))
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsRegenerating(false)
    }
  }

  return (
    <>
      <div className='-ml-1.5 flex items-center gap-1'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='ghost'
                size='icon-sm'
                onClick={handleToggleStatus}
                disabled={isTogglingStatus}
                aria-label={isEnabled ? t('Disable') : t('Enable')}
                className={
                  isEnabled
                    ? 'text-destructive hover:text-destructive'
                    : 'text-emerald-600 hover:text-emerald-600 dark:text-emerald-400 dark:hover:text-emerald-400'
                }
              />
            }
          >
            {isTogglingStatus ? (
              <Loader2 className='size-4 animate-spin' />
            ) : isEnabled ? (
              <PowerOff className='size-4' />
            ) : (
              <Power className='size-4' />
            )}
          </TooltipTrigger>
          <TooltipContent>
            {isEnabled ? t('Disable') : t('Enable')}
          </TooltipContent>
        </Tooltip>

        <DropdownMenu modal={false} onOpenChange={handleMenuOpenChange}>
          <DropdownMenuTrigger
            render={
              <Button
                variant='ghost'
                className='data-popup-open:bg-muted flex h-8 w-8 p-0'
              />
            }
          >
            <DotsHorizontalIcon className='h-4 w-4' />
            <span className='sr-only'>{t('Open menu')}</span>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end' className='w-[200px]'>
            <DropdownMenuItem
              onClick={async () => {
                const realKey = getCachedRealKey()
                if (!realKey) return
                const ok = await copyToClipboard(realKey)
                if (ok) toast.success(t('Copied'))
              }}
            >
              {t('Copy Key')}
              <DropdownMenuShortcut>
                <Copy size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={async () => {
                const realKey = getCachedRealKey()
                if (!realKey) return
                const connStr = encodeConnectionString(
                  realKey,
                  getServerAddress()
                )
                const ok = await copyToClipboard(connStr)
                if (ok) toast.success(t('Copied'))
              }}
            >
              {t('Copy Connection Info')}
              <DropdownMenuShortcut>
                <Link size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {apiKey.risk_activation_required ? (
              <DropdownMenuItem onClick={() => setRiskActivationOpen(true)}>
                {t('View unlock steps')}
                <DropdownMenuShortcut>
                  <ShieldAlert size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(apiKey)
                setOpen('update')
              }}
            >
              {t('Edit')}
              <DropdownMenuShortcut>
                <Edit size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={async () => {
                const realKey = await resolveRealKey(apiKey.id)
                if (!realKey) return
                setResolvedKey(realKey)
                setCurrentRow(apiKey)
                setOpen('cc-switch')
              }}
            >
              {t('CC Switch')}
              <DropdownMenuShortcut>
                <ArrowRightLeft size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            {hasChatPresets && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>{t('Chat')}</DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  {chatPresets.map((preset) => (
                    <DropdownMenuItem
                      key={preset.id}
                      onClick={() => handleOpenChatPreset(preset)}
                    >
                      {preset.name}
                      {preset.type !== 'web' && (
                        <DropdownMenuShortcut>
                          <ExternalLink size={16} />
                        </DropdownMenuShortcut>
                      )}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(apiKey)
                setOpen('delete')
              }}
              className='text-destructive focus:text-destructive'
            >
              {t('Delete')}
              <DropdownMenuShortcut>
                <Trash2 size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <AccessControlDialog
        open={accessDialogOpen}
        onOpenChange={setAccessDialogOpen}
        status={accessControlStatus}
        onStatusChanged={setAccessControlStatus}
      />
      <RiskActivationDialog
        open={riskActivationOpen}
        onOpenChange={setRiskActivationOpen}
        apiKey={apiKey}
        bindCode={activationCode}
        expiresAt={activationExpiresAt}
        onRegenerate={handleRegenerateRiskActivationCode}
        regenerating={isRegenerating}
      />
    </>
  )
}
