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
export type RawRecord = Record<string, unknown>

export type OpsRegistryQueryKey =
  | 'groups'
  | 'releases'
  | 'releaseImpact'
  | 'controlPlane'
  | 'communityGate'
  | 'groupCapabilities'
  | 'rewardFund'
  | 'inviteJourney'
  | 'audits'

export type OpsRegistryScope = Partial<Record<OpsRegistryQueryKey, boolean>>

export const OPS_REGISTRY_DEFAULT_SCOPE: Record<OpsRegistryQueryKey, boolean> =
  {
    groups: true,
    releases: true,
    releaseImpact: true,
    controlPlane: true,
    communityGate: true,
    groupCapabilities: true,
    rewardFund: true,
    inviteJourney: true,
    audits: true,
  }

export type OpsRegistryGroup = {
  id: number
  site_id: string
  platform: string
  platform_family?: string
  group_id: string
  group_name: string
  invite_target_group_id?: string
  role: string
  status: string
  enabled: boolean
  config?: RawRecord
  capabilities?: RawRecord
  game_configs?: RawRecord[]
  latest_metrics?: RawRecord
  access_qualifiers?: RawRecord
  runtime_connectors?: RawRecord
  source_tables?: Record<string, string>
}

export type OpsGroupSavePayload = {
  site_id?: string
  platform?: string
  group_id?: string
  group_name?: string
  invite_target_group_id?: string
  role?: string
  status?: string
  language?: string
  timezone?: string
  config?: RawRecord
  copy_chatops?: boolean
  copy_game_rule?: boolean
}

export type OpsGroupBulkSavePayload = {
  site_id?: string
  items: OpsGroupSavePayload[]
  continue_on_error?: boolean
}

export type OpsGroupBulkSaveFailure = {
  index: number
  group_id: string
  group_name?: string
  reason: string
}

export type OpsGroupBulkSaveResult = {
  site_id: string
  total: number
  created_count: number
  failed_count: number
  created: OpsRegistryGroup[]
  failed: OpsGroupBulkSaveFailure[]
  generated_at?: number
}

export type OpsGroupChatOpsSavePayload = {
  checkin_enabled?: boolean
  verify_enabled?: boolean
  invite_enabled?: boolean
  checkin_quota?: number
  verify_min_quota?: number
  invite_reward_quota?: number
  invitee_reward_quota?: number
  daily_group_reward_limit?: number
  rule?: RawRecord
}

export type OpsGroupGameSaveItem = {
  game_code: string
  enabled?: boolean
  budget_pool?: string
  rule?: RawRecord
}

export type OpsGroupGamesSavePayload = {
  games: OpsGroupGameSaveItem[]
}

export type OpsGroupActions = {
  create: (payload: OpsGroupSavePayload) => Promise<OpsRegistryGroup | null>
  createBulk: (
    payload: OpsGroupBulkSavePayload
  ) => Promise<OpsGroupBulkSaveResult | null>
  update: (
    id: number,
    payload: OpsGroupSavePayload
  ) => Promise<OpsRegistryGroup | null>
  clone: (
    id: number,
    payload: OpsGroupSavePayload
  ) => Promise<OpsRegistryGroup | null>
  saveChatOps: (
    id: number,
    payload: OpsGroupChatOpsSavePayload
  ) => Promise<OpsRegistryGroup | null>
  saveGames: (
    id: number,
    payload: OpsGroupGamesSavePayload
  ) => Promise<OpsRegistryGroup | null>
  saving: boolean
}

export type OpsReleaseRecord = {
  id: number
  site_id: string
  action: string
  release_label: string
  actor_user_id: number
  actor_username: string
  note: string
  snapshot_hash: string
  option_count: number
  missing_option_count: number
  group_count: number
  group_chatops_count: number
  group_game_count: number
  source_release_id: number
  created_at?: number
  applied_at?: number
}

export type OpsReleasePublishPayload = {
  release_label?: string
  note?: string
}

export type OpsReleaseRollbackPayload = {
  release_id: number
  note?: string
}

export type OpsReleaseActions = {
  publish: (
    payload: OpsReleasePublishPayload
  ) => Promise<OpsReleaseRecord | null>
  rollback: (
    payload: OpsReleaseRollbackPayload
  ) => Promise<OpsReleaseRecord | null>
  publishing: boolean
  rollingBack: boolean
  busy: boolean
}

export type OpsReleaseOverview = {
  site_id: string
  release_mode?: string
  publish_supported?: boolean
  rollback_supported?: boolean
  latest_change_at?: number
  audit_scope_counts?: Record<string, number>
  recent_audits?: Array<RawRecord>
  recent_releases?: OpsReleaseRecord[]
  current_release?: OpsReleaseRecord | null
  control_plane?: RawRecord | null
  generated_at?: number
}

export type OpsReleaseImpactState = {
  site_id: string
  hash: string
  generated_at?: number
  option_count: number
  missing_option_count: number
  group_count: number
  group_chatops_count: number
  group_game_count: number
}

export type OpsReleaseImpactDiffBucket = {
  added_count: number
  updated_count: number
  removed_count: number
  added_keys?: string[]
  updated_keys?: string[]
  removed_keys?: string[]
  added?: RawRecord[]
  updated?: RawRecord[]
  removed?: RawRecord[]
}

export type OpsReleaseImpactPreview = {
  site_id: string
  current_hash: string
  previous_hash: string
  current_state: OpsReleaseImpactState | null
  previous_release: OpsReleaseRecord | null
  has_changes: boolean
  diff_summary: {
    option_changes: number
    group_changes: number
    group_chatops_changes: number
    group_game_changes: number
  }
  changes: {
    options: OpsReleaseImpactDiffBucket
    groups: OpsReleaseImpactDiffBucket
    group_chatops: OpsReleaseImpactDiffBucket
    group_game_configs: OpsReleaseImpactDiffBucket
  }
  generated_at?: number
}

export type OpsControlPlaneField = {
  configured?: boolean
  option_key?: string
  value?: unknown
}

export type OpsControlPlaneSource = {
  field?: string
  domain?: string
  option_key?: string
  source?: string
  state?: string
  message?: string
  display_name?: string
}

export type OpsControlPlaneSnapshot = {
  site_id: string
  generated_at?: number
  configured?: Record<string, Record<string, OpsControlPlaneField>>
  effective?: Record<string, RawRecord>
  runtime?: Record<string, RawRecord>
  source_map?: Record<string, OpsControlPlaneSource>
  drift?: RawRecord[]
}

export type OpsCommunityGateOverview = {
  enabled?: boolean
  provider_slug?: string
  community_host?: string
  room_id?: string
  room_ids?: string[]
  room_match_mode?: string
  matched_room_ids?: string[]
  missing_room_ids?: string[]
  join_targets?: RawRecord[]
  bot_token_configured?: boolean
  runtime_cache?: RawRecord
  recent_audits?: RawRecord[]
  denied_message?: string
  audit_enabled?: boolean
}

export type OpsGroupCapabilityMatrixItem = {
  id: number
  site_id: string
  platform: string
  platform_family?: string
  group_id: string
  group_name: string
  invite_target_group_id?: string
  role: string
  status: string
  enabled: boolean
  capability_policy?: RawRecord
  reward_policy?: RawRecord
  game_policy?: RawRecord
  game_configs?: RawRecord[]
  access_qualifiers?: RawRecord
  runtime_connectors?: RawRecord
  latest_metrics?: RawRecord
  source_tables?: Record<string, string>
}

export type OpsGroupCapabilityMatrixOverview = {
  site_id: string
  filters?: Record<string, string>
  summary?: RawRecord
  items: OpsGroupCapabilityMatrixItem[]
  generated_at?: number
}

export type OpsRewardFundOverview = {
  site_id: string
  fund: RawRecord
  budget_pools_today: RawRecord[]
  budget_settings: RawRecord
  effective_available_quota: number
  reward_policy: RawRecord
  source_breakdown: RawRecord[]
  source_breakdown_today: RawRecord[]
  commission_audit: RawRecord
  invite_reward_audit: RawRecord
  degradation: RawRecord
  recent_ledgers: RawRecord[]
  source_tables?: Record<string, string>
  generated_at?: number
}

export type OpsInviteJourneyOverview = {
  site_id: string
  funnel: RawRecord
  state_machine: RawRecord[]
  campaigns: RawRecord[]
  claim_statuses: RawRecord[]
  edge_statuses: RawRecord[]
  event_types: RawRecord[]
  risk_flags: RawRecord[]
  reward_source: RawRecord
  problems: RawRecord[]
  recent_events: RawRecord[]
  recent_claims: RawRecord[]
  source_tables?: Record<string, string>
  generated_at?: number
}

export type OpsAccessExplainRequest = {
  userId: number
  requestedGroup?: string
  refresh?: boolean
}

export type OpsAccessExplainResult = {
  site_id: string
  generated_at?: number
  user?: RawRecord
  requested_group?: string
  decision?: string
  allowed?: boolean
  reason_code?: string
  reason_message?: string
  human_message?: string
  next_steps?: string[]
  base_groups?: string[]
  effective_groups?: string[]
  requested_group_source?: RawRecord
  status?: RawRecord
  policy?: RawRecord | null
}

export type OpsAuditOverview = {
  site_id: string
  summary: RawRecord
  events: Array<OpsUnifiedAuditEvent>
}

export type OpsUnifiedAuditEvent = {
  id: string
  domain: string
  event_type: string
  title: string
  subject: string
  status: string
  severity: string
  reason_code: string
  reason_message: string
  actor: string
  actor_user_id?: number
  user_id?: number
  room_id?: string
  provider_slug?: string
  access_level?: string
  at?: number
  raw: RawRecord
}
