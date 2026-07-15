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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, getUserGroups } from '@/lib/api'
import { getPrimaryPlatformDisplayName } from '@/hooks/use-access-control'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'
import {
  AccessControlEditorView,
  type AccessControlValues,
} from './access-control-editor-view'
import {
  type AccessControlRuntimeStatus,
  type AccessRuntimeView,
  type GroupCatalogEntry,
  type SavedAccessView,
} from './access-control-overview'
import { type OpsTranslate, useOpsT } from './ops-i18n'
import {
  getOpsSavedState,
  readOpsSavedBool,
  readOpsSavedList,
  readOpsSavedNumber,
  readOpsSavedText,
} from './ops-snapshots'

type Props = {
  defaultValues: Record<string, string | number | boolean>
  embedded?: boolean
}

const optionKeyMap: Record<keyof AccessControlValues, string> = {
  enabled: 'access_control_setting.enabled',
  primaryPlatform: 'access_control_setting.primary_platform',
  primaryGroupIds: 'access_control_setting.primary_group_ids',
  communityGroupIds: 'access_control_setting.community_group_ids',
  communityOnlyGroups: 'access_control_setting.community_only_groups',
  fullAccessGroups: 'access_control_setting.full_access_groups',
  paidBypassGroups: 'access_control_setting.paid_bypass_groups',
  paidUserGroups: 'access_control_setting.paid_user_groups',
  allowPaidBypass: 'access_control_setting.allow_paid_bypass',
  allowAdminBypass: 'access_control_setting.allow_admin_bypass',
  checkOnLogin: 'access_control_setting.check_on_login',
  blockTokenCreate: 'access_control_setting.block_token_create',
  blockTokenEnable: 'access_control_setting.block_token_enable',
  enforceRequestTime: 'access_control_setting.enforce_request_time',
  freezeLegacyTokens: 'access_control_setting.freeze_legacy_tokens',
  autoRestoreCompliantTokens:
    'access_control_setting.auto_restore_compliant_tokens',
  stateCacheTTLSeconds: 'access_control_setting.state_cache_ttl_seconds',
  communityJoinURL: 'access_control_setting.community_join_url',
  primaryJoinURL: 'access_control_setting.primary_join_url',
  denyMessage: 'access_control_setting.deny_message',
  upgradeMessage: 'access_control_setting.upgrade_message',
  rewardSoftFloorQuota: 'access_control_setting.reward_soft_floor_quota',
  rewardHardFloorQuota: 'access_control_setting.reward_hard_floor_quota',
  dailySiteRewardCap: 'access_control_setting.daily_site_reward_cap',
  dailyUserRewardCap: 'access_control_setting.daily_user_reward_cap',
}

const listKeys: Array<keyof AccessControlValues> = [
  'primaryGroupIds',
  'communityGroupIds',
  'communityOnlyGroups',
  'fullAccessGroups',
  'paidBypassGroups',
  'paidUserGroups',
]

type AccessControlStatusResponse = {
  data?: AccessControlRuntimeStatus
}

function buildSavedAccessView(
  defaultValues: Props['defaultValues']
): SavedAccessView {
  return {
    enabled: readOpsSavedBool(defaultValues, 'access_control_setting.enabled'),
    primaryPlatform: readOpsSavedText(
      defaultValues,
      'access_control_setting.primary_platform'
    ),
    blockTokenCreate: readOpsSavedBool(
      defaultValues,
      'access_control_setting.block_token_create'
    ),
    blockTokenEnable: readOpsSavedBool(
      defaultValues,
      'access_control_setting.block_token_enable'
    ),
    allowPaidBypass: readOpsSavedBool(
      defaultValues,
      'access_control_setting.allow_paid_bypass'
    ),
    allowAdminBypass: readOpsSavedBool(
      defaultValues,
      'access_control_setting.allow_admin_bypass'
    ),
    communityGroupIds: readOpsSavedList(
      defaultValues,
      'access_control_setting.community_group_ids'
    ),
    primaryGroupIds: readOpsSavedList(
      defaultValues,
      'access_control_setting.primary_group_ids'
    ),
    communityOnlyGroups: readOpsSavedList(
      defaultValues,
      'access_control_setting.community_only_groups'
    ),
    fullAccessGroups: readOpsSavedList(
      defaultValues,
      'access_control_setting.full_access_groups'
    ),
    communityJoinURL: readOpsSavedText(
      defaultValues,
      'access_control_setting.community_join_url'
    ),
    primaryJoinURL: readOpsSavedText(
      defaultValues,
      'access_control_setting.primary_join_url'
    ),
    denyMessage: readOpsSavedText(
      defaultValues,
      'access_control_setting.deny_message'
    ),
    upgradeMessage: readOpsSavedText(
      defaultValues,
      'access_control_setting.upgrade_message'
    ),
    rewardSoftFloorQuota: readOpsSavedNumber(
      defaultValues,
      'access_control_setting.reward_soft_floor_quota'
    ),
    rewardHardFloorQuota: readOpsSavedNumber(
      defaultValues,
      'access_control_setting.reward_hard_floor_quota'
    ),
    dailySiteRewardCap: readOpsSavedNumber(
      defaultValues,
      'access_control_setting.daily_site_reward_cap'
    ),
    dailyUserRewardCap: readOpsSavedNumber(
      defaultValues,
      'access_control_setting.daily_user_reward_cap'
    ),
  }
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

function stringValue(value: unknown, fallback = '') {
  if (value === null || value === undefined) return fallback
  const text = String(value).trim()
  return text || fallback
}

function runtimeBoolValue(value: unknown, fallback: boolean) {
  return typeof value === 'boolean' ? value : fallback
}

function decodeList(value: unknown) {
  if (Array.isArray(value)) return value.join('\n')
  const text = String(value || '').trim()
  if (!text) return ''
  try {
    const parsed = JSON.parse(text)
    if (Array.isArray(parsed)) return parsed.join('\n')
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error
  }
  return text
}

function encodeList(value: string) {
  return JSON.stringify(
    value
      .split(/\r?\n|,/)
      .map((item) => item.trim())
      .filter(Boolean)
  )
}

function splitEditorList(value: string) {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function toggleEditorList(value: string, item: string) {
  const current = splitEditorList(value)
  const next = current.includes(item)
    ? current.filter((entry) => entry !== item)
    : [...current, item]
  return next.join('\n')
}

function buildDefaults(
  defaultValues: Props['defaultValues']
): AccessControlValues {
  return {
    enabled: boolValue(defaultValues['access_control_setting.enabled']),
    primaryPlatform: String(
      defaultValues['access_control_setting.primary_platform'] || ''
    ),
    primaryGroupIds: decodeList(
      defaultValues['access_control_setting.primary_group_ids']
    ),
    communityGroupIds: decodeList(
      defaultValues['access_control_setting.community_group_ids']
    ),
    communityOnlyGroups: decodeList(
      defaultValues['access_control_setting.community_only_groups']
    ),
    fullAccessGroups: decodeList(
      defaultValues['access_control_setting.full_access_groups']
    ),
    paidBypassGroups: decodeList(
      defaultValues['access_control_setting.paid_bypass_groups']
    ),
    paidUserGroups: decodeList(
      defaultValues['access_control_setting.paid_user_groups']
    ),
    allowPaidBypass: boolValue(
      defaultValues['access_control_setting.allow_paid_bypass']
    ),
    allowAdminBypass: boolValue(
      defaultValues['access_control_setting.allow_admin_bypass']
    ),
    checkOnLogin: boolValue(
      defaultValues['access_control_setting.check_on_login']
    ),
    blockTokenCreate: boolValue(
      defaultValues['access_control_setting.block_token_create']
    ),
    blockTokenEnable: boolValue(
      defaultValues['access_control_setting.block_token_enable']
    ),
    enforceRequestTime: boolValue(
      defaultValues['access_control_setting.enforce_request_time']
    ),
    freezeLegacyTokens: boolValue(
      defaultValues['access_control_setting.freeze_legacy_tokens']
    ),
    autoRestoreCompliantTokens: boolValue(
      defaultValues['access_control_setting.auto_restore_compliant_tokens']
    ),
    stateCacheTTLSeconds: numberValue(
      defaultValues['access_control_setting.state_cache_ttl_seconds'],
      0
    ),
    communityJoinURL: String(
      defaultValues['access_control_setting.community_join_url'] || ''
    ),
    primaryJoinURL: String(
      defaultValues['access_control_setting.primary_join_url'] || ''
    ),
    denyMessage: String(
      defaultValues['access_control_setting.deny_message'] || ''
    ),
    upgradeMessage: String(
      defaultValues['access_control_setting.upgrade_message'] || ''
    ),
    rewardSoftFloorQuota: numberValue(
      defaultValues['access_control_setting.reward_soft_floor_quota'],
      0
    ),
    rewardHardFloorQuota: numberValue(
      defaultValues['access_control_setting.reward_hard_floor_quota'],
      0
    ),
    dailySiteRewardCap: numberValue(
      defaultValues['access_control_setting.daily_site_reward_cap'],
      0
    ),
    dailyUserRewardCap: numberValue(
      defaultValues['access_control_setting.daily_user_reward_cap'],
      0
    ),
  }
}

function buildAccessRuntimeView(
  saved: AccessControlValues,
  status: AccessControlRuntimeStatus | null
): AccessRuntimeView {
  return {
    enabled: runtimeBoolValue(status?.enabled, saved.enabled),
    primaryPlatform: stringValue(
      status?.primary_platform,
      saved.primaryPlatform
    ),
    blockTokenCreate: runtimeBoolValue(
      status?.block_token_create,
      saved.blockTokenCreate
    ),
    blockTokenEnable: runtimeBoolValue(
      status?.block_token_enable,
      saved.blockTokenEnable
    ),
    allowPaidBypass: runtimeBoolValue(
      status?.allow_paid_bypass,
      saved.allowPaidBypass
    ),
    allowAdminBypass: runtimeBoolValue(
      status?.allow_admin_bypass,
      saved.allowAdminBypass
    ),
    communityJoinURL: stringValue(
      status?.community_join_url,
      saved.communityJoinURL
    ),
    primaryJoinURL: stringValue(status?.primary_join_url, saved.primaryJoinURL),
    denyMessage: stringValue(status?.deny_message, saved.denyMessage),
    upgradeMessage: stringValue(status?.upgrade_message, saved.upgradeMessage),
    rewardSoftFloorQuota: numberValue(
      status?.reward_soft_floor_quota,
      saved.rewardSoftFloorQuota
    ),
    rewardHardFloorQuota: numberValue(
      status?.reward_hard_floor_quota,
      saved.rewardHardFloorQuota
    ),
    dailySiteRewardCap: numberValue(
      status?.daily_site_reward_cap,
      saved.dailySiteRewardCap
    ),
    dailyUserRewardCap: numberValue(
      status?.daily_user_reward_cap,
      saved.dailyUserRewardCap
    ),
    activeFrozenUsers: numberValue(status?.active_frozen_users, 0),
    activeFrozenTokens: numberValue(status?.active_frozen_tokens, 0),
  }
}

type AccessControlSectionEditorProps = Props & {
  defaults: AccessControlValues
}

export function AccessControlSection(props: Props) {
  const defaults = useMemo(
    () => buildDefaults(props.defaultValues),
    [props.defaultValues]
  )
  return (
    <AccessControlSectionEditor
      key={JSON.stringify(defaults)}
      {...props}
      defaults={defaults}
    />
  )
}

type RefreshAccessStatus = () => Promise<unknown>

function useAccessControlSources(t: OpsTranslate) {
  const { data: groupCatalogData } = useQuery({
    queryKey: ['access-control-group-catalog'],
    queryFn: getUserGroups,
    staleTime: 0,
  })
  const { data: allGroupsData } = useQuery({
    queryKey: ['access-control-all-groups'],
    queryFn: async () => {
      const res = await api.get('/api/group/')
      return Array.isArray(res.data?.data) ? res.data.data : []
    },
    staleTime: 0,
  })
  const {
    data: status = null,
    error: statusError,
    refetch: refreshStatus,
  } = useQuery({
    queryKey: ['access-control-status'],
    queryFn: async () => {
      const res = await api.get<AccessControlStatusResponse>(
        '/api/access-control/status'
      )
      return res.data?.data ?? null
    },
    retry: false,
    staleTime: 0,
  })

  useEffect(() => {
    if (!statusError) return
    toast.error(
      statusError instanceof Error
        ? statusError.message
        : t('Failed to load access control status')
    )
  }, [statusError, t])

  const groupCatalog = useMemo<GroupCatalogEntry[]>(() => {
    const detailMap = groupCatalogData?.data || {}
    const names = new Set<string>([
      ...Object.keys(detailMap),
      ...((allGroupsData as string[] | undefined) || []),
    ])
    return Array.from(names)
      .map((value) => ({
        value,
        desc: detailMap[value]?.desc || value,
        ratio: detailMap[value]?.ratio,
      }))
      .sort((a, b) => a.value.localeCompare(b.value))
  }, [allGroupsData, groupCatalogData])

  return { status, refreshStatus, groupCatalog }
}

type AccessControlDerivedStateInput = {
  defaultValues: Props['defaultValues']
  defaults: AccessControlValues
  values: AccessControlValues
  status: AccessControlRuntimeStatus | null
  t: OpsTranslate
}

function useAccessControlDerivedState({
  defaultValues,
  defaults,
  values,
  status,
  t,
}: AccessControlDerivedStateInput) {
  const counts = useMemo(() => status?.counts || {}, [status])
  const savedView = useMemo(
    () => buildSavedAccessView(defaultValues),
    [defaultValues]
  )
  const savedStates = {
    communityGroupIds: getOpsSavedState(
      defaultValues,
      'access_control_setting.community_group_ids'
    ),
    primaryGroupIds: getOpsSavedState(
      defaultValues,
      'access_control_setting.primary_group_ids'
    ),
    communityOnlyGroups: getOpsSavedState(
      defaultValues,
      'access_control_setting.community_only_groups'
    ),
    fullAccessGroups: getOpsSavedState(
      defaultValues,
      'access_control_setting.full_access_groups'
    ),
    communityJoinURL: getOpsSavedState(
      defaultValues,
      'access_control_setting.community_join_url'
    ),
    primaryJoinURL: getOpsSavedState(
      defaultValues,
      'access_control_setting.primary_join_url'
    ),
    denyMessage: getOpsSavedState(
      defaultValues,
      'access_control_setting.deny_message'
    ),
    upgradeMessage: getOpsSavedState(
      defaultValues,
      'access_control_setting.upgrade_message'
    ),
  }
  const runtimeView = useMemo(
    () => buildAccessRuntimeView(defaults, status),
    [defaults, status]
  )
  const runtimePrimaryPlatformLabel = runtimeView.primaryPlatform
    ? getPrimaryPlatformDisplayName(runtimeView.primaryPlatform, t)
    : savedView.primaryPlatform
      ? getPrimaryPlatformDisplayName(savedView.primaryPlatform, t)
      : t('Not saved')
  const editorPrimaryPlatformLabel = values.primaryPlatform
    ? getPrimaryPlatformDisplayName(values.primaryPlatform, t)
    : t('Not selected yet')
  const accessWarnings = useMemo(() => {
    const warnings: string[] = []
    if (
      (counts.community_only || 0) > 0 &&
      savedView.communityOnlyGroups.length === 0
    ) {
      warnings.push(
        t(
          'Runtime already has community-only users, but no saved community-only group mapping was read from the current configuration.'
        )
      )
    }
    if (runtimeView.enabled && savedView.communityGroupIds.length === 0) {
      warnings.push(
        t(
          'Access control is enabled, but no saved community room IDs were read. Community binding may have no explicit fallback scope.'
        )
      )
    }
    if (runtimeView.enabled && savedView.primaryGroupIds.length === 0) {
      warnings.push(
        t(
          'Access control is enabled, but no saved primary group IDs were read. Full unlock may be relying only on paid/admin/manual overrides.'
        )
      )
    }
    return warnings
  }, [counts.community_only, runtimeView.enabled, savedView, t])
  const communityOnlyGroupSet = useMemo(
    () => new Set(splitEditorList(values.communityOnlyGroups)),
    [values.communityOnlyGroups]
  )
  const fullAccessGroupSet = useMemo(
    () => new Set(splitEditorList(values.fullAccessGroups)),
    [values.fullAccessGroups]
  )

  return {
    counts,
    savedView,
    savedStates,
    runtimeView,
    runtimePrimaryPlatformLabel,
    editorPrimaryPlatformLabel,
    accessWarnings,
    communityOnlyGroupSet,
    fullAccessGroupSet,
  }
}

type AccessControlSaveInput = {
  values: AccessControlValues
  defaults: AccessControlValues
  refreshStatus: RefreshAccessStatus
  t: OpsTranslate
}

function useAccessControlSave({
  values,
  defaults,
  refreshStatus,
  t,
}: AccessControlSaveInput) {
  const updateOptionsBulk = useUpdateOptionsBulk()
  const [saving, setSaving] = useState(false)

  async function saveAll() {
    const updates = (
      Object.entries(optionKeyMap) as Array<[keyof AccessControlValues, string]>
    )
      .map(([field, optionKey]) => {
        const currentValue = listKeys.includes(field)
          ? encodeList(String(values[field]))
          : values[field]
        const defaultValue = listKeys.includes(field)
          ? encodeList(String(defaults[field]))
          : defaults[field]
        return { optionKey, currentValue, defaultValue }
      })
      .filter((item) => String(item.currentValue) !== String(item.defaultValue))
      .map((item) => ({ key: item.optionKey, value: item.currentValue }))

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    setSaving(true)
    try {
      await updateOptionsBulk.mutateAsync({ updates })
      toast.success(t('Setting updated successfully'))
      await refreshStatus()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update setting')
      )
    } finally {
      setSaving(false)
    }
  }

  return {
    saveAll,
    saving,
    updateOptionsPending: updateOptionsBulk.isPending,
  }
}

function useAccessControlTools(
  refreshStatus: RefreshAccessStatus,
  t: OpsTranslate
) {
  const [scanLoading, setScanLoading] = useState(false)
  const [checkUserId, setCheckUserId] = useState('')
  const [checkResult, setCheckResult] = useState<unknown>(null)
  const [overrideUserId, setOverrideUserId] = useState('')
  const [overrideMode, setOverrideMode] = useState('clear')
  const [overrideGroups, setOverrideGroups] = useState('')
  const [overrideReason, setOverrideReason] = useState('')
  const accessOverrideModeOptions = useMemo(
    () => [
      { value: 'clear', label: t('Clear') },
      { value: 'full_access', label: t('Full access') },
      { value: 'community_only', label: t('Community only') },
      { value: 'custom_groups', label: t('Custom groups') },
      { value: 'none', label: t('Deny all') },
    ],
    [t]
  )

  async function runScan(dryRun: boolean) {
    setScanLoading(true)
    try {
      await api.post('/api/access-control/scan', {
        dry_run: dryRun,
        limit: 1000,
      })
      toast.success(
        dryRun ? t('Dry-run scan completed') : t('Access scan completed')
      )
      await refreshStatus()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to run access scan')
      )
    } finally {
      setScanLoading(false)
    }
  }

  async function checkUser() {
    const targetUserId = checkUserId.trim()
    if (!targetUserId) return
    try {
      const res = await api.post(`/api/access-control/check/${targetUserId}`)
      setCheckResult(res.data?.data || null)
      toast.success(t('User access status loaded'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to check user')
      )
    }
  }

  async function submitOverride() {
    if (!overrideUserId.trim()) return
    try {
      const res = await api.post('/api/access-control/override', {
        user_id: Number(overrideUserId),
        mode: overrideMode,
        groups:
          overrideMode === 'custom_groups'
            ? splitEditorList(overrideGroups)
            : [],
        reason: overrideReason,
      })
      setCheckResult(res.data?.data || null)
      toast.success(t('Override updated'))
      await refreshStatus()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update override')
      )
    }
  }

  return {
    scanLoading,
    runScan,
    checkUserId,
    setCheckUserId,
    checkUser,
    checkResult,
    overrideUserId,
    setOverrideUserId,
    overrideMode,
    setOverrideMode,
    accessOverrideModeOptions,
    overrideGroups,
    setOverrideGroups,
    overrideReason,
    setOverrideReason,
    submitOverride,
  }
}

function AccessControlSectionEditor({
  defaultValues,
  embedded = false,
  defaults,
}: AccessControlSectionEditorProps) {
  const t = useOpsT()
  const [values, setValues] = useState<AccessControlValues>(() => defaults)
  const { status, refreshStatus, groupCatalog } = useAccessControlSources(t)
  const derivedState = useAccessControlDerivedState({
    defaultValues,
    defaults,
    values,
    status,
    t,
  })
  const saveState = useAccessControlSave({
    values,
    defaults,
    refreshStatus,
    t,
  })
  const toolState = useAccessControlTools(refreshStatus, t)

  function setValue<K extends keyof AccessControlValues>(
    key: K,
    value: AccessControlValues[K]
  ) {
    setValues((previous) => ({ ...previous, [key]: value }))
  }

  function toggleBucketGroup(
    key: 'communityOnlyGroups' | 'fullAccessGroups',
    groupName: string
  ) {
    setValue(key, toggleEditorList(values[key], groupName))
  }

  return (
    <AccessControlEditorView
      t={t}
      embedded={embedded}
      status={status}
      groupCatalog={groupCatalog}
      values={values}
      setValue={setValue}
      onToggleBucketGroup={toggleBucketGroup}
      {...derivedState}
      tools={toolState}
      {...saveState}
    />
  )
}
