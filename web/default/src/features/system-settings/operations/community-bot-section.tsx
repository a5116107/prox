/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useCallback, useEffect, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Form } from '@/components/ui/form'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { CommunityBotFieldGroups } from './community-bot-fields'
import {
  buildCommunityBotDefaults,
  communityBotOptionKeyMap,
  communityBotSchema,
  type CommunityBotDefaultValues,
  type CommunityBotStats,
  type CommunityBotStatus,
  type CommunityBotValues,
} from './community-bot-model'
import { CommunityBotOverview } from './community-bot-overview'

type CommunityBotSectionProps = {
  defaultValues: CommunityBotDefaultValues
}

export function CommunityBotSection(props: CommunityBotSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [status, setStatus] = useState<CommunityBotStatus | null>(null)
  const [stats, setStats] = useState<CommunityBotStats | null>(null)
  const [actionLoading, setActionLoading] = useState(false)
  const [botToken, setBotToken] = useState('')
  const defaults = buildCommunityBotDefaults(props.defaultValues)
  const form = useForm<CommunityBotValues>({
    resolver: zodResolver(communityBotSchema) as Resolver<CommunityBotValues>,
    defaultValues: defaults,
  })

  const refreshStatus = useCallback(async () => {
    try {
      const response = await api.get('/api/community-bot/status')
      if (response.data?.success) setStatus(response.data.data)
    } catch {
      toast.error(t('Failed to load community bot status'))
    }
  }, [t])

  const refreshStats = useCallback(async () => {
    try {
      const response = await api.get('/api/community-bot/stats')
      if (response.data?.success) setStats(response.data.data)
    } catch {
      toast.error(t('Failed to load community bot stats'))
    }
  }, [t])

  useEffect(() => {
    void refreshStatus()
    void refreshStats()
  }, [refreshStats, refreshStatus])

  async function onSubmit(values: CommunityBotValues) {
    const updates = (
      Object.keys(communityBotOptionKeyMap) as Array<keyof CommunityBotValues>
    ).filter((key) => values[key] !== defaults[key])
    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key: communityBotOptionKeyMap[key],
        value: String(values[key] ?? ''),
      })
    }
    form.reset(values)
    await refreshStatus()
    await refreshStats()
  }

  async function startMiAuth() {
    setActionLoading(true)
    try {
      const response = await api.post('/api/community-bot/oauth/start')
      if (response.data?.success && response.data?.data?.authorize_url) {
        window.open(
          response.data.data.authorize_url,
          '_blank',
          'noopener,noreferrer'
        )
        toast.success(t('Authorization page opened'))
      } else {
        toast.error(
          response.data?.message || t('Failed to start authorization')
        )
      }
    } finally {
      setActionLoading(false)
    }
  }

  async function saveBotToken() {
    if (!botToken.trim()) {
      toast.error(t('Community bot token is empty'))
      return
    }

    setActionLoading(true)
    try {
      const response = await api.put('/api/community-bot/token', {
        token: botToken.trim(),
      })
      if (response.data?.success) {
        toast.success(t('Bot token verified and saved'))
        setBotToken('')
        await refreshStatus()
        await refreshStats()
      } else {
        toast.error(response.data?.message || t('Failed to save bot token'))
      }
    } finally {
      setActionLoading(false)
    }
  }

  async function testMessage() {
    setActionLoading(true)
    try {
      const response = await api.post('/api/community-bot/test-message', {
        text: t(
          'Community bot test message: New API is connected to the community room.'
        ),
      })
      if (response.data?.success) toast.success(t('Test message sent'))
      else {
        toast.error(response.data?.message || t('Failed to send test message'))
      }
    } finally {
      setActionLoading(false)
    }
  }

  async function scanNow() {
    setActionLoading(true)
    try {
      const response = await api.post('/api/community-bot/scan')
      if (response.data?.success) {
        toast.success(
          t('Scan completed: {{messages}} messages, {{users}} rewarded', {
            messages: response.data.data?.scanned_messages ?? 0,
            users: response.data.data?.rewarded_users ?? 0,
          })
        )
        await refreshStatus()
        await refreshStats()
      } else {
        toast.error(response.data?.message || t('Scan failed'))
      }
    } finally {
      setActionLoading(false)
    }
  }

  return (
    <SettingsSection title={t('Community Bot')}>
      <CommunityBotOverview
        status={status}
        stats={stats}
        actionLoading={actionLoading}
        onRefreshStatus={refreshStatus}
        onRefreshStats={refreshStats}
        onAuthorize={startMiAuth}
        onTestMessage={testMessage}
        onScan={scanNow}
      />

      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            isSaveDisabled={!form.formState.isDirty}
            saveLabel='Save community bot settings'
          />
          <CommunityBotFieldGroups
            control={form.control}
            botToken={botToken}
            actionLoading={actionLoading}
            onBotTokenChange={setBotToken}
            onSaveBotToken={saveBotToken}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
