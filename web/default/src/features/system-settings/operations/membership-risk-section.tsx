/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useForm, type UseFormReturn } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Form } from '@/components/ui/form'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  buildMembershipBenefitMatrix,
  buildMembershipRiskDefaults,
  membershipRiskOptionKeyMap,
  membershipRiskSchema,
  type MembershipDryRun,
  type MembershipOverview,
  type MembershipRiskValues,
  type MembershipState,
  type MembershipUnresolvedOverview,
} from './membership-risk-model'
import {
  MembershipRiskRuntimeView,
  type MembershipBatchAction,
  type MembershipStateAction,
} from './membership-risk-runtime-view'
import { MembershipRiskSettingsForm } from './membership-risk-settings-form'
import { type OpsTranslate, useOpsT } from './ops-i18n'

type Props = {
  defaultValues: Record<string, string | number | boolean>
}

type MembershipRiskSource = {
  overview: MembershipOverview | null
  states: MembershipState[]
  unresolved: MembershipUnresolvedOverview | null
  statusFilter: string
  setStatusFilter: (status: string) => void
  loading: boolean
  refresh: (status: string) => Promise<void>
  refreshWithFeedback: (status: string) => Promise<void>
}

function useMembershipRiskSource(t: OpsTranslate): MembershipRiskSource {
  const [overview, setOverview] = useState<MembershipOverview | null>(null)
  const [, setDryRun] = useState<MembershipDryRun | null>(null)
  const [states, setStates] = useState<MembershipState[]>([])
  const [unresolved, setUnresolved] =
    useState<MembershipUnresolvedOverview | null>(null)
  const [statusFilter, setStatusFilter] = useState('')
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async (nextStatus: string) => {
    setLoading(true)
    try {
      const [overviewRes, unresolvedRes, dryRunRes, statesRes] =
        await Promise.all([
          api.get('/api/chat-membership/admin/overview', {
            disableDuplicate: true,
          }),
          api.get('/api/chat-membership/admin/unresolved', {
            params: { limit: 20 },
            disableDuplicate: true,
          }),
          api.post('/api/chat-membership/admin/dry-run'),
          api.get('/api/chat-membership/admin/states', {
            params: {
              status: nextStatus || undefined,
              limit: 50,
            },
            disableDuplicate: true,
          }),
        ])
      if (overviewRes.data?.success) setOverview(overviewRes.data.data)
      if (unresolvedRes.data?.success) setUnresolved(unresolvedRes.data.data)
      if (dryRunRes.data?.success) setDryRun(dryRunRes.data.data)
      if (statesRes.data?.success) setStates(statesRes.data.data || [])
    } finally {
      setLoading(false)
    }
  }, [])

  const refreshWithFeedback = useCallback(
    async (nextStatus: string) => {
      try {
        await refresh(nextStatus)
      } catch (error) {
        toast.error(t('Failed to load'), {
          description: error instanceof Error ? error.message : undefined,
        })
      }
    },
    [refresh, t]
  )

  useEffect(() => {
    const refreshTimer = window.setTimeout(() => {
      void refreshWithFeedback('')
    }, 0)
    return () => window.clearTimeout(refreshTimer)
  }, [refreshWithFeedback])

  return {
    overview,
    states,
    unresolved,
    statusFilter,
    setStatusFilter,
    loading,
    refresh,
    refreshWithFeedback,
  }
}

type MembershipRiskSaveInput = {
  form: UseFormReturn<MembershipRiskValues>
  defaults: MembershipRiskValues
  source: Pick<MembershipRiskSource, 'refresh' | 'statusFilter'>
  t: OpsTranslate
}

function useMembershipRiskSave({
  form,
  defaults,
  source,
  t,
}: MembershipRiskSaveInput) {
  const updateOption = useUpdateOption()

  async function save(values: MembershipRiskValues) {
    const changedKeys = (
      Object.keys(membershipRiskOptionKeyMap) as Array<
        keyof MembershipRiskValues
      >
    ).filter((key) => values[key] !== defaults[key])
    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key: membershipRiskOptionKeyMap[key],
        value: String(values[key] ?? ''),
      })
    }
    form.reset(values)
    await source.refresh(source.statusFilter)
  }

  return { save, saving: updateOption.isPending }
}

type MembershipRiskActionsInput = {
  source: Pick<MembershipRiskSource, 'refresh' | 'statusFilter'>
  t: OpsTranslate
}

function useMembershipRiskActions({ source, t }: MembershipRiskActionsInput) {
  const [actionLoading, setActionLoading] = useState<number | null>(null)
  const [batchLoading, setBatchLoading] =
    useState<MembershipBatchAction | null>(null)
  const [manualActionReason, setManualActionReason] = useState(
    '管理员人工复核：用户已重新满足对应群资格或需要临时放行'
  )
  const [manualBypassHours, setManualBypassHours] = useState(72)

  async function runStateAction(
    state: MembershipState,
    action: MembershipStateAction
  ) {
    setActionLoading(state.id)
    try {
      const payload =
        action === 'bypass'
          ? {
              reason: manualActionReason.trim(),
              until_hours: Math.max(
                1,
                Math.min(720, Number(manualBypassHours || 72))
              ),
            }
          : { reason: manualActionReason.trim() }
      const response = await api.post(
        `/api/chat-membership/admin/states/${state.id}/${action}`,
        payload
      )
      if (response.data?.success) {
        toast.success(t('Membership state updated'))
        await source.refresh(source.statusFilter)
      }
    } finally {
      setActionLoading(null)
    }
  }

  async function runBatchAction(action: MembershipBatchAction) {
    setBatchLoading(action)
    try {
      const endpoint =
        action === 'backfill'
          ? '/api/chat-membership/admin/backfill'
          : '/api/chat-membership/admin/demote-unresolved'
      const response = await api.post(endpoint, { limit: 200 })
      if (response.data?.success) {
        const batchResult = response.data.data || {}
        const message =
          action === 'backfill'
            ? `${t('Backfill completed')} · ${batchResult.state_rows ?? 0}/${batchResult.event_rows ?? 0}/${batchResult.activated_states ?? 0}`
            : `${t('Demoted unresolved active states')} · ${batchResult.demoted_states ?? 0}`
        toast.success(message)
        await source.refresh(source.statusFilter)
      }
    } finally {
      setBatchLoading(null)
    }
  }

  return {
    actionLoading,
    batchLoading,
    manualActionReason,
    setManualActionReason,
    manualBypassHours,
    setManualBypassHours,
    runStateAction,
    runBatchAction,
  }
}

export function MembershipRiskSection({ defaultValues }: Props) {
  const t = useOpsT()
  const defaults = useMemo(
    () => buildMembershipRiskDefaults(defaultValues),
    [defaultValues]
  )
  const form = useForm<MembershipRiskValues>({
    resolver: zodResolver(membershipRiskSchema),
    defaultValues: defaults,
  })
  const source = useMembershipRiskSource(t)
  const saveState = useMembershipRiskSave({ form, defaults, source, t })
  const actionState = useMembershipRiskActions({ source, t })

  useEffect(() => {
    form.reset(defaults)
  }, [defaults, form])

  const totalTracked = useMemo(
    () =>
      Object.values(source.overview?.counts || {}).reduce(
        (sum, value) => sum + Number(value || 0),
        0
      ),
    [source.overview]
  )
  const watchedValues = form.watch()
  const benefitRows = useMemo(
    () => buildMembershipBenefitMatrix(watchedValues, t),
    [watchedValues, t]
  )
  const isDryRun = source.overview ? source.overview.dry_run : defaults.dryRun

  return (
    <Form {...form}>
      <SettingsSection title={t('Membership Risk')}>
        <div className='space-y-6'>
          <MembershipRiskRuntimeView
            t={t}
            snapshot={{
              overview: source.overview,
              unresolved: source.unresolved,
              states: source.states,
              loading: source.loading,
              isDryRun,
              totalTracked,
              benefitRows,
            }}
            controls={{
              statusFilter: source.statusFilter,
              onStatusFilterChange: source.setStatusFilter,
              manualActionReason: actionState.manualActionReason,
              onManualActionReasonChange: actionState.setManualActionReason,
              manualBypassHours: actionState.manualBypassHours,
              onManualBypassHoursChange: actionState.setManualBypassHours,
              actionLoading: actionState.actionLoading,
              batchLoading: actionState.batchLoading,
            }}
            actions={{
              refresh: source.refresh,
              refreshWithFeedback: source.refreshWithFeedback,
              runStateAction: actionState.runStateAction,
              runBatchAction: actionState.runBatchAction,
            }}
          />
          <MembershipRiskSettingsForm
            t={t}
            form={form}
            onSubmit={saveState.save}
            saving={saveState.saving}
          />
        </div>
      </SettingsSection>
    </Form>
  )
}
