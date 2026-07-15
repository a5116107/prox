/*
Copyright (C) 2023-2026 QuantumNous
*/
type SettingRecord = Record<string, string | number | boolean>

const OPS_RAW_PREFIX = '__raw.'

function getOpsRawValue(settings: SettingRecord, key: string) {
  const rawKey = `${OPS_RAW_PREFIX}${key}`
  return Object.prototype.hasOwnProperty.call(settings, rawKey)
    ? settings[rawKey]
    : undefined
}

export type OpsSavedState = 'missing' | 'empty' | 'value'

export function getOpsSavedState(
  settings: SettingRecord,
  key: string
): OpsSavedState {
  const raw = getOpsRawValue(settings, key)
  if (raw === undefined || raw === null) return 'missing'
  if (Array.isArray(raw)) return raw.length > 0 ? 'value' : 'empty'
  if (typeof raw === 'string') {
    const trimmed = raw.trim()
    if (!trimmed) return 'empty'
    try {
      const parsed = JSON.parse(trimmed)
      if (Array.isArray(parsed)) return parsed.length > 0 ? 'value' : 'empty'
    } catch (error) {
      if (!(error instanceof SyntaxError)) throw error
    }
    return 'value'
  }
  return 'value'
}

export function readOpsSavedText(settings: SettingRecord, key: string) {
  const raw = getOpsRawValue(settings, key)
  if (raw === undefined || raw === null) return ''
  return String(raw).trim()
}

export function readOpsSavedBool(settings: SettingRecord, key: string) {
  const raw = getOpsRawValue(settings, key)
  if (raw === undefined || raw === null || raw === '') return null
  return readOpsBool(raw, false)
}

export function readOpsSavedNumber(settings: SettingRecord, key: string) {
  const raw = getOpsRawValue(settings, key)
  if (raw === undefined || raw === null || raw === '') return null
  return readOpsNumber(raw, 0)
}

export function readOpsSavedList(settings: SettingRecord, key: string) {
  const raw = getOpsRawValue(settings, key)
  if (raw === undefined || raw === null || raw === '') return []
  return parseOpsList(raw)
}

export type OpsAccessSnapshot = {
  enabled: boolean
  primaryPlatform: string
  primaryGroupIds: string[]
  communityGroupIds: string[]
  communityOnlyGroups: string[]
  fullAccessGroups: string[]
  paidBypassGroups: string[]
  paidUserGroups: string[]
  allowPaidBypass: boolean
  allowAdminBypass: boolean
  checkOnLogin: boolean
  blockTokenCreate: boolean
  blockTokenEnable: boolean
  enforceRequestTime: boolean
  freezeLegacyTokens: boolean
  autoRestoreCompliantTokens: boolean
  stateCacheTTLSeconds: number
  communityJoinURL: string
  primaryJoinURL: string
  denyMessage: string
  upgradeMessage: string
  rewardSoftFloorQuota: number
  rewardHardFloorQuota: number
  dailySiteRewardCap: number
  dailyUserRewardCap: number
}

export type OpsGateSnapshot = {
  enabled: boolean
  providerSlug: string
  communityHost: string
  roomId: string
  roomIds: string[]
  roomMatchMode: string
  requireOAuthBinding: boolean
  requireRoomMembership: boolean
  autoInviteOnLogin: boolean
  blockTokenWhenNotCompliant: boolean
  allowAdminBypass: boolean
  memberCacheTTLSeconds: number
  memberScanLimit: number
  tokenDisableMode: string
  deniedMessage: string
  auditEnabled: boolean
}

function readOpsBool(value: unknown, fallback = false) {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') return value === 'true'
  return fallback
}

function readOpsNumber(value: unknown, fallback = 0) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function parseOpsList(value: unknown) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean)
  }

  const text = String(value ?? '').trim()
  if (!text) return []

  try {
    const parsed = JSON.parse(text)
    if (Array.isArray(parsed)) {
      return parsed.map((item) => String(item).trim()).filter(Boolean)
    }
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error
  }

  return text
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export function buildOpsAccessSnapshot(
  defaultValues: SettingRecord
): OpsAccessSnapshot {
  return {
    enabled: readOpsBool(defaultValues['access_control_setting.enabled']),
    primaryPlatform: String(
      defaultValues['access_control_setting.primary_platform'] || ''
    ),
    primaryGroupIds: parseOpsList(
      defaultValues['access_control_setting.primary_group_ids']
    ),
    communityGroupIds: parseOpsList(
      defaultValues['access_control_setting.community_group_ids']
    ),
    communityOnlyGroups: parseOpsList(
      defaultValues['access_control_setting.community_only_groups']
    ),
    fullAccessGroups: parseOpsList(
      defaultValues['access_control_setting.full_access_groups']
    ),
    paidBypassGroups: parseOpsList(
      defaultValues['access_control_setting.paid_bypass_groups']
    ),
    paidUserGroups: parseOpsList(
      defaultValues['access_control_setting.paid_user_groups']
    ),
    allowPaidBypass: readOpsBool(
      defaultValues['access_control_setting.allow_paid_bypass']
    ),
    allowAdminBypass: readOpsBool(
      defaultValues['access_control_setting.allow_admin_bypass']
    ),
    checkOnLogin: readOpsBool(
      defaultValues['access_control_setting.check_on_login']
    ),
    blockTokenCreate: readOpsBool(
      defaultValues['access_control_setting.block_token_create']
    ),
    blockTokenEnable: readOpsBool(
      defaultValues['access_control_setting.block_token_enable']
    ),
    enforceRequestTime: readOpsBool(
      defaultValues['access_control_setting.enforce_request_time']
    ),
    freezeLegacyTokens: readOpsBool(
      defaultValues['access_control_setting.freeze_legacy_tokens']
    ),
    autoRestoreCompliantTokens: readOpsBool(
      defaultValues['access_control_setting.auto_restore_compliant_tokens']
    ),
    stateCacheTTLSeconds: readOpsNumber(
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
    rewardSoftFloorQuota: readOpsNumber(
      defaultValues['access_control_setting.reward_soft_floor_quota']
    ),
    rewardHardFloorQuota: readOpsNumber(
      defaultValues['access_control_setting.reward_hard_floor_quota']
    ),
    dailySiteRewardCap: readOpsNumber(
      defaultValues['access_control_setting.daily_site_reward_cap']
    ),
    dailyUserRewardCap: readOpsNumber(
      defaultValues['access_control_setting.daily_user_reward_cap']
    ),
  }
}

export function buildOpsGateSnapshot(
  defaultValues: SettingRecord
): OpsGateSnapshot {
  const roomId = String(
    defaultValues['community_gate_setting.room_id'] ||
      defaultValues['community_bot_setting.room_id'] ||
      ''
  )
  const roomIds = parseOpsList(defaultValues['community_gate_setting.room_ids'])
  const resolvedRoomIds = roomIds.length > 0 ? roomIds : roomId ? [roomId] : []

  return {
    enabled: readOpsBool(defaultValues['community_gate_setting.enabled']),
    providerSlug: String(
      defaultValues['community_gate_setting.provider_slug'] ||
        defaultValues['community_bot_setting.provider_slug'] ||
        ''
    ),
    communityHost: String(
      defaultValues['community_gate_setting.community_host'] ||
        defaultValues['community_bot_setting.community_host'] ||
        ''
    ),
    roomId,
    roomIds: resolvedRoomIds,
    roomMatchMode: String(
      defaultValues['community_gate_setting.room_match_mode'] || ''
    ),
    requireOAuthBinding: readOpsBool(
      defaultValues['community_gate_setting.require_oauth_binding']
    ),
    requireRoomMembership: readOpsBool(
      defaultValues['community_gate_setting.require_room_membership']
    ),
    autoInviteOnLogin: readOpsBool(
      defaultValues['community_gate_setting.auto_invite_on_login']
    ),
    blockTokenWhenNotCompliant: readOpsBool(
      defaultValues['community_gate_setting.block_token_when_not_compliant']
    ),
    allowAdminBypass: readOpsBool(
      defaultValues['community_gate_setting.allow_admin_bypass']
    ),
    memberCacheTTLSeconds: readOpsNumber(
      defaultValues['community_gate_setting.member_cache_ttl_seconds'],
      0
    ),
    memberScanLimit: readOpsNumber(
      defaultValues['community_gate_setting.member_scan_limit'],
      0
    ),
    tokenDisableMode: String(
      defaultValues['community_gate_setting.token_disable_mode'] || ''
    ),
    deniedMessage: String(
      defaultValues['community_gate_setting.denied_message'] || ''
    ),
    auditEnabled: readOpsBool(
      defaultValues['community_gate_setting.audit_enabled']
    ),
  }
}
