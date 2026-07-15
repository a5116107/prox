/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type Props = {
  defaultValues: Record<string, string | number | boolean>
}

type GateValues = {
  enabled: boolean
  providerSlug: string
  communityHost: string
  roomId: string
  requireOAuthBinding: boolean
  requireRoomMembership: boolean
  onlyAllowProviderRegister: boolean
  disablePasswordRegister: boolean
  disableBuiltinOAuthRegister: boolean
  autoInviteOnLogin: boolean
  blockTokenWhenNotCompliant: boolean
  allowAdminBypass: boolean
  memberCacheTTLSeconds: number
  memberScanLimit: number
  tokenDisableMode: string
  deniedMessage: string
  auditEnabled: boolean
}

type GateScanResult = {
  dry_run?: boolean
  scanned_users?: number
  compliant_users?: number
  blocked_users?: number
  error_users?: number
  tokens_eligible?: number
  tokens_disabled?: number
  users?: Array<{
    user_id: number
    username: string
    provider_user_id?: string
    reason_code?: string
    reason?: string
  }>
}

type GateStatus = GateValues & {
  room_id?: string
  bot_token_configured?: boolean
  runtime_cache?: { rooms?: number; users?: number }
  recent_audits?: Array<{
    id: number
    user_id: number
    username: string
    provider_user_id?: string
    compliant: boolean
    reason_code?: string
    checked_at?: number
  }>
}

const optionKeyMap: Record<keyof GateValues, string> = {
  enabled: 'community_gate_setting.enabled',
  providerSlug: 'community_gate_setting.provider_slug',
  communityHost: 'community_gate_setting.community_host',
  roomId: 'community_gate_setting.room_id',
  requireOAuthBinding: 'community_gate_setting.require_oauth_binding',
  requireRoomMembership: 'community_gate_setting.require_room_membership',
  onlyAllowProviderRegister:
    'community_gate_setting.only_allow_provider_register',
  disablePasswordRegister: 'community_gate_setting.disable_password_register',
  disableBuiltinOAuthRegister:
    'community_gate_setting.disable_builtin_oauth_register',
  autoInviteOnLogin: 'community_gate_setting.auto_invite_on_login',
  blockTokenWhenNotCompliant:
    'community_gate_setting.block_token_when_not_compliant',
  allowAdminBypass: 'community_gate_setting.allow_admin_bypass',
  memberCacheTTLSeconds: 'community_gate_setting.member_cache_ttl_seconds',
  memberScanLimit: 'community_gate_setting.member_scan_limit',
  tokenDisableMode: 'community_gate_setting.token_disable_mode',
  deniedMessage: 'community_gate_setting.denied_message',
  auditEnabled: 'community_gate_setting.audit_enabled',
}

function boolValue(value: unknown, fallback = false) {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') return value === 'true'
  return fallback
}

function numberValue(value: unknown, fallback: number) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function buildDefaults(defaultValues: Props['defaultValues']): GateValues {
  return {
    enabled: boolValue(defaultValues['community_gate_setting.enabled'], true),
    providerSlug: String(
      defaultValues['community_gate_setting.provider_slug'] || 'dc.hhhl.cc'
    ),
    communityHost: String(
      defaultValues['community_gate_setting.community_host'] ||
        'https://dc.hhhl.cc'
    ),
    roomId: String(
      defaultValues['community_gate_setting.room_id'] ||
        defaultValues['community_bot_setting.room_id'] ||
        ''
    ),
    requireOAuthBinding: boolValue(
      defaultValues['community_gate_setting.require_oauth_binding'],
      true
    ),
    requireRoomMembership: boolValue(
      defaultValues['community_gate_setting.require_room_membership'],
      true
    ),
    onlyAllowProviderRegister: boolValue(
      defaultValues['community_gate_setting.only_allow_provider_register'],
      true
    ),
    disablePasswordRegister: boolValue(
      defaultValues['community_gate_setting.disable_password_register'],
      true
    ),
    disableBuiltinOAuthRegister: boolValue(
      defaultValues['community_gate_setting.disable_builtin_oauth_register'],
      true
    ),
    autoInviteOnLogin: boolValue(
      defaultValues['community_gate_setting.auto_invite_on_login'],
      true
    ),
    blockTokenWhenNotCompliant: boolValue(
      defaultValues['community_gate_setting.block_token_when_not_compliant'],
      true
    ),
    allowAdminBypass: boolValue(
      defaultValues['community_gate_setting.allow_admin_bypass'],
      true
    ),
    memberCacheTTLSeconds: numberValue(
      defaultValues['community_gate_setting.member_cache_ttl_seconds'],
      600
    ),
    memberScanLimit: numberValue(
      defaultValues['community_gate_setting.member_scan_limit'],
      3000
    ),
    tokenDisableMode: String(
      defaultValues['community_gate_setting.token_disable_mode'] ||
        'temporary_disable'
    ),
    deniedMessage: String(
      defaultValues['community_gate_setting.denied_message'] ||
        '请先使用 dc.hhhl.cc 社区授权登录，并加入本站社区群聊后再使用 API Key。'
    ),
    auditEnabled: boolValue(
      defaultValues['community_gate_setting.audit_enabled'],
      true
    ),
  }
}

function formatTime(value?: number) {
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString()
}

export function CommunityGateSection({ defaultValues }: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [values, setValues] = useState<GateValues>(() =>
    buildDefaults(defaultValues)
  )
  const [status, setStatus] = useState<GateStatus | null>(null)
  const [scan, setScan] = useState<GateScanResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [checkUserId, setCheckUserId] = useState('')

  async function refreshStatus() {
    const res = await api.get('/api/community-gate/status')
    if (res.data?.success) setStatus(res.data.data)
  }

  useEffect(() => {
    refreshStatus().catch(() => undefined)
  }, [])

  function setField<K extends keyof GateValues>(key: K, value: GateValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }))
  }

  async function save() {
    setLoading(true)
    try {
      for (const [field, optionKey] of Object.entries(optionKeyMap) as Array<
        [keyof GateValues, string]
      >) {
        await updateOption.mutateAsync({
          key: optionKey,
          value: String(values[field] ?? ''),
        })
      }
      toast.success(t('Community gate settings saved'))
      await refreshStatus()
    } finally {
      setLoading(false)
    }
  }

  async function runScan(dryRun: boolean) {
    setLoading(true)
    try {
      const res = await api.post('/api/community-gate/scan', {
        dry_run: dryRun,
      })
      if (res.data?.success) {
        setScan(res.data.data)
        toast.success(
          dryRun
            ? t('Community gate dry-run completed')
            : t('Community gate enforcement completed')
        )
        await refreshStatus()
      } else {
        toast.error(res.data?.message || t('Community gate scan failed'))
      }
    } finally {
      setLoading(false)
    }
  }

  async function checkUser() {
    if (!checkUserId.trim()) return
    setLoading(true)
    try {
      const res = await api.post(
        `/api/community-gate/check/${checkUserId.trim()}`
      )
      if (res.data?.success) {
        const user = res.data.data as NonNullable<
          GateScanResult['users']
        >[number]
        setScan({ users: [user] })
        toast.success(t('Community gate user check completed'))
      } else {
        toast.error(res.data?.message || t('Community gate user check failed'))
      }
    } finally {
      setLoading(false)
    }
  }

  async function restoreUser() {
    if (!checkUserId.trim()) return
    setLoading(true)
    try {
      const res = await api.post(
        `/api/community-gate/restore/${checkUserId.trim()}`
      )
      if (res.data?.success) {
        toast.success(t('Community gate temporary token freeze restored'))
        await refreshStatus()
      } else {
        toast.error(res.data?.message || t('Community gate restore failed'))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <SettingsSection title={t('Community Access Gate')}>
      <Card className='border-emerald-500/20 bg-emerald-500/5'>
        <CardHeader>
          <CardTitle>{t('Community Access Gate')}</CardTitle>
          <CardDescription>
            {t(
              'Require dc.hhhl.cc OAuth binding and room membership before API keys can be used.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-3 text-sm md:grid-cols-4'>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground'>{t('Gate status')}</div>
            <div className='font-medium'>
              {status?.enabled ? t('Enabled') : t('Disabled')}
            </div>
          </div>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground'>{t('Room ID')}</div>
            <div className='font-medium'>
              {status?.room_id || values.roomId || '-'}
            </div>
          </div>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground'>{t('Bot token')}</div>
            <div className='font-medium'>
              {status?.bot_token_configured
                ? t('Configured')
                : t('Not configured')}
            </div>
          </div>
          <div className='bg-background/70 rounded-lg border p-3'>
            <div className='text-muted-foreground'>{t('Runtime cache')}</div>
            <div className='font-medium'>
              {status?.runtime_cache?.rooms ?? 0} /{' '}
              {status?.runtime_cache?.users ?? 0}
            </div>
          </div>
        </CardContent>
      </Card>

      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          void save()
        }}
      >
        <SettingsPageFormActions
          onSave={save}
          isSaving={loading || updateOption.isPending}
          isSaveDisabled={loading}
          saveLabel='Save community gate settings'
        />

        {(
          [
            [
              'enabled',
              'Enable community access gate',
              'Master switch for OAuth binding, room membership, and API key enforcement.',
            ],
            [
              'requireOAuthBinding',
              'Require dc.hhhl.cc OAuth binding',
              'Users must be registered or bound through the configured community provider.',
            ],
            [
              'requireRoomMembership',
              'Require room membership',
              'Users must be present in the configured community room.',
            ],
            [
              'onlyAllowProviderRegister',
              'Only allow community OAuth registration',
              'New users can only be created by the configured dc.hhhl.cc custom OAuth provider.',
            ],
            [
              'disablePasswordRegister',
              'Disable password registration by gate',
              'Password registration is rejected even if the legacy switch is accidentally enabled.',
            ],
            [
              'disableBuiltinOAuthRegister',
              'Disable built-in OAuth registration',
              'GitHub, Telegram, OIDC and other built-in providers cannot create new users.',
            ],
            [
              'autoInviteOnLogin',
              'Auto invite on community login',
              'Keep using the existing community bot invitation flow after OAuth login.',
            ],
            [
              'blockTokenWhenNotCompliant',
              'Block non-compliant API keys',
              'TokenAuth rejects users that do not pass this gate.',
            ],
            [
              'allowAdminBypass',
              'Allow administrator bypass',
              'Root and admin accounts are excluded from community gate enforcement.',
            ],
            [
              'auditEnabled',
              'Enable gate audit log',
              'Persist the latest gate decision for each user.',
            ],
          ] as Array<[keyof GateValues, string, string]>
        ).map(([key, label, description]) => (
          <SettingsSwitchItem key={key}>
            <SettingsSwitchContent>
              <label className='font-medium'>{t(label)}</label>
              <p className='text-muted-foreground text-sm'>{t(description)}</p>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(values[key])}
              onCheckedChange={(checked) => setField(key, checked)}
            />
          </SettingsSwitchItem>
        ))}

        <div className='grid gap-4 md:grid-cols-3'>
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Provider slug')}</span>
            <Input
              value={values.providerSlug}
              onChange={(event) => setField('providerSlug', event.target.value)}
            />
          </label>
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Community host')}</span>
            <Input
              value={values.communityHost}
              onChange={(event) =>
                setField('communityHost', event.target.value)
              }
            />
          </label>
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Room ID')}</span>
            <Input
              value={values.roomId}
              onChange={(event) => setField('roomId', event.target.value)}
            />
          </label>
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Member cache TTL seconds')}</span>
            <Input
              type='number'
              min={60}
              value={values.memberCacheTTLSeconds}
              onChange={(event) =>
                setField('memberCacheTTLSeconds', Number(event.target.value))
              }
            />
          </label>
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Member scan limit')}</span>
            <Input
              type='number'
              min={100}
              value={values.memberScanLimit}
              onChange={(event) =>
                setField('memberScanLimit', Number(event.target.value))
              }
            />
          </label>
          <label className='space-y-2 text-sm'>
            <span className='font-medium'>{t('Token disable mode')}</span>
            <Input
              value={values.tokenDisableMode}
              onChange={(event) =>
                setField('tokenDisableMode', event.target.value)
              }
            />
          </label>
        </div>

        <label className='space-y-2 text-sm'>
          <span className='font-medium'>{t('Denied message')}</span>
          <Textarea
            rows={2}
            value={values.deniedMessage}
            onChange={(event) => setField('deniedMessage', event.target.value)}
          />
        </label>

        <Card>
          <CardHeader>
            <CardTitle>{t('Gate audit and enforcement')}</CardTitle>
            <CardDescription>
              {t(
                'Dry-run first, then temporarily disable tokens for non-compliant users. Restores only tokens frozen by this gate.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='flex flex-wrap gap-2'>
              <Button
                type='button'
                variant='outline'
                disabled={loading}
                onClick={() => runScan(true)}
              >
                {t('Dry-run scan')}
              </Button>
              <Button
                type='button'
                disabled={loading}
                onClick={() => runScan(false)}
              >
                {t('Apply temporary token freeze')}
              </Button>
              <Button
                type='button'
                variant='outline'
                disabled={loading}
                onClick={refreshStatus}
              >
                {t('Refresh')}
              </Button>
            </div>
            <div className='grid gap-2 md:grid-cols-[1fr_auto_auto]'>
              <Input
                value={checkUserId}
                onChange={(event) => setCheckUserId(event.target.value)}
                placeholder={t('User ID for check or restore')}
              />
              <Button
                type='button'
                variant='outline'
                disabled={loading || !checkUserId.trim()}
                onClick={checkUser}
              >
                {t('Check user')}
              </Button>
              <Button
                type='button'
                variant='secondary'
                disabled={loading || !checkUserId.trim()}
                onClick={restoreUser}
              >
                {t('Restore gate-frozen tokens')}
              </Button>
            </div>

            {scan ? (
              <div className='rounded-lg border p-3 text-sm'>
                <div className='grid gap-2 md:grid-cols-6'>
                  <div>
                    {t('Scanned')}: {scan.scanned_users ?? '-'}
                  </div>
                  <div>
                    {t('Compliant')}: {scan.compliant_users ?? '-'}
                  </div>
                  <div>
                    {t('Blocked')}: {scan.blocked_users ?? '-'}
                  </div>
                  <div>
                    {t('Errors')}: {scan.error_users ?? '-'}
                  </div>
                  <div>
                    {t('Eligible tokens')}: {scan.tokens_eligible ?? '-'}
                  </div>
                  <div>
                    {t('Disabled tokens')}: {scan.tokens_disabled ?? '-'}
                  </div>
                </div>
                <div className='mt-3 divide-y rounded-md border'>
                  {(scan.users ?? []).slice(0, 20).map((item) => (
                    <div
                      key={`${item.user_id}-${item.reason_code}`}
                      className='grid gap-2 p-2 md:grid-cols-[80px_1fr_1fr_1fr]'
                    >
                      <div>#{item.user_id}</div>
                      <div className='truncate'>{item.username}</div>
                      <div className='truncate'>
                        {item.provider_user_id || '-'}
                      </div>
                      <div className='text-muted-foreground truncate'>
                        {item.reason_code || item.reason || '-'}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            <div className='space-y-2'>
              <div className='text-sm font-medium'>
                {t('Recent gate audits')}
              </div>
              <div className='divide-y rounded-lg border text-sm'>
                {(status?.recent_audits ?? []).slice(0, 10).map((audit) => (
                  <div
                    key={audit.id}
                    className='grid gap-2 p-2 md:grid-cols-[80px_1fr_1fr_120px_160px]'
                  >
                    <div>#{audit.user_id}</div>
                    <div className='truncate'>{audit.username}</div>
                    <div className='truncate'>
                      {audit.provider_user_id || '-'}
                    </div>
                    <div>{audit.compliant ? t('Compliant') : t('Blocked')}</div>
                    <div className='text-muted-foreground'>
                      {formatTime(audit.checked_at)}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>
      </SettingsForm>
    </SettingsSection>
  )
}
