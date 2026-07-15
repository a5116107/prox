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
import {
  type OpsAccessExplainResult,
  type OpsAuditOverview,
  type OpsCommunityGateOverview,
  type OpsControlPlaneField,
  type OpsControlPlaneSnapshot,
  type OpsControlPlaneSource,
  type OpsGroupBulkSaveResult,
  type OpsGroupCapabilityMatrixItem,
  type OpsGroupCapabilityMatrixOverview,
  type OpsInviteJourneyOverview,
  type OpsRegistryGroup,
  type OpsReleaseImpactDiffBucket,
  type OpsReleaseImpactPreview,
  type OpsReleaseImpactState,
  type OpsReleaseOverview,
  type OpsReleaseRecord,
  type OpsRewardFundOverview,
  type OpsUnifiedAuditEvent,
  type RawRecord,
} from './ops-registry-contract'

export function pickSiteId(
  defaultValues: Record<string, string | number | boolean>
) {
  const explicitSiteId = [
    defaultValues['agent_setting.site_id'],
    defaultValues['site_id'],
  ]
    .map((value) => String(value || '').trim())
    .find((value) => value && value.toLowerCase() !== 'default')

  if (explicitSiteId) return explicitSiteId

  if (typeof window === 'undefined') return ''

  const host = String(window.location.host || '')
    .trim()
    .toLowerCase()
  const hostname = String(window.location.hostname || '')
    .trim()
    .toLowerCase()
  const port = String(window.location.port || '').trim()

  const directHostMap: Record<string, string> = {
    'ai.prox.us.ci': 'prox',
    '127.0.0.1:33001': 'prox',
    'localhost:33001': 'prox',
  }

  const inferredDirect =
    directHostMap[host] ||
    directHostMap[hostname] ||
    directHostMap[`${hostname}:${port}`]

  if (inferredDirect) return inferredDirect
  if (host.includes('prox') || hostname.includes('prox')) return 'prox'

  return ''
}

export function asRecord(value: unknown): RawRecord {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as RawRecord)
    : {}
}

function asArray(value: unknown): RawRecord[] {
  return Array.isArray(value)
    ? (value.filter((item) => item && typeof item === 'object') as RawRecord[])
    : []
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.map((item) => String(item || '').trim()).filter(Boolean)
}

function optionalNumber(value: unknown): number | undefined {
  return value === undefined ? undefined : Number(value || 0)
}

export function normalizeGroupsPayload(payload: unknown): OpsRegistryGroup[] {
  const data = asRecord(payload)
  const items = asArray(data.items)
  return items.map((item) => ({
    id: Number(item.id || 0),
    site_id: String(item.site_id || ''),
    platform: String(item.platform || ''),
    platform_family: String(item.platform_family || ''),
    group_id: String(item.group_id || ''),
    group_name: String(item.group_name || item.group_id || ''),
    invite_target_group_id: String(item.invite_target_group_id || ''),
    role: String(item.role || ''),
    status: String(item.status || ''),
    enabled: Boolean(item.enabled),
    config: asRecord(item.config),
    capabilities: asRecord(item.capabilities),
    game_configs: asArray(item.game_configs),
    latest_metrics: asRecord(item.latest_metrics),
    access_qualifiers: asRecord(item.access_qualifiers),
    runtime_connectors: asRecord(item.runtime_connectors),
    source_tables: asRecord(item.source_tables) as Record<string, string>,
  }))
}

export function normalizeGroupPayload(
  payload: unknown
): OpsRegistryGroup | null {
  const data = asRecord(payload)
  const candidate = 'id' in data ? data : asRecord(data.group)
  if (!('id' in candidate)) return null
  return normalizeGroupsPayload({ items: [candidate] })[0] ?? null
}

export function normalizeGroupBulkPayload(
  payload: unknown
): OpsGroupBulkSaveResult | null {
  const data = asRecord(payload)
  const failed = asArray(data.failed).map((item) => ({
    index: Number(item.index || 0),
    group_id: String(item.group_id || ''),
    group_name: String(item.group_name || ''),
    reason: String(item.reason || ''),
  }))

  return {
    site_id: String(data.site_id || ''),
    total: Number(data.total || 0),
    created_count: Number(data.created_count || 0),
    failed_count: Number(data.failed_count || 0),
    created: normalizeGroupsPayload({ items: data.created }),
    failed,
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
  }
}

export function normalizeReleaseRecord(
  payload: unknown
): OpsReleaseRecord | null {
  const data = asRecord(payload)
  if (!('id' in data)) return null
  return {
    id: Number(data.id || 0),
    site_id: String(data.site_id || ''),
    action: String(data.action || ''),
    release_label: String(data.release_label || ''),
    actor_user_id: Number(data.actor_user_id || 0),
    actor_username: String(data.actor_username || ''),
    note: String(data.note || ''),
    snapshot_hash: String(data.snapshot_hash || ''),
    option_count: Number(data.option_count || 0),
    missing_option_count: Number(data.missing_option_count || 0),
    group_count: Number(data.group_count || 0),
    group_chatops_count: Number(data.group_chatops_count || 0),
    group_game_count: Number(data.group_game_count || 0),
    source_release_id: Number(data.source_release_id || 0),
    created_at: optionalNumber(data.created_at),
    applied_at: optionalNumber(data.applied_at),
  }
}

function normalizeReleaseRecords(payload: unknown): OpsReleaseRecord[] {
  return asArray(payload)
    .map((item) => normalizeReleaseRecord(item))
    .filter(Boolean) as OpsReleaseRecord[]
}

export function normalizeReleasePayload(payload: unknown): OpsReleaseOverview {
  const data = asRecord(payload)
  const scopeCounts = asRecord(data.audit_scope_counts)
  const normalizedScopeCounts: Record<string, number> = {}
  Object.entries(scopeCounts).forEach(([key, value]) => {
    normalizedScopeCounts[key] = Number(value || 0)
  })
  return {
    site_id: String(data.site_id || ''),
    release_mode: String(data.release_mode || ''),
    publish_supported: Boolean(data.publish_supported),
    rollback_supported: Boolean(data.rollback_supported),
    latest_change_at:
      data.latest_change_at === undefined
        ? undefined
        : Number(data.latest_change_at || 0),
    audit_scope_counts: normalizedScopeCounts,
    recent_audits: asArray(data.recent_audits),
    recent_releases: normalizeReleaseRecords(data.recent_releases),
    current_release: normalizeReleaseRecord(data.current_release),
    control_plane: asRecord(data.control_plane),
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
  }
}

function normalizeReleaseImpactState(
  payload: unknown
): OpsReleaseImpactState | null {
  const data = asRecord(payload)
  if (!Object.keys(data).length) return null
  return {
    site_id: String(data.site_id || ''),
    hash: String(data.hash || ''),
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
    option_count: Number(data.option_count || 0),
    missing_option_count: Number(data.missing_option_count || 0),
    group_count: Number(data.group_count || 0),
    group_chatops_count: Number(data.group_chatops_count || 0),
    group_game_count: Number(data.group_game_count || 0),
  }
}

function normalizeReleaseImpactDiffBucket(
  payload: unknown
): OpsReleaseImpactDiffBucket {
  const data = asRecord(payload)
  return {
    added_count: Number(data.added_count || 0),
    updated_count: Number(data.updated_count || 0),
    removed_count: Number(data.removed_count || 0),
    added_keys: asArray(data.added_keys).map((item) => String(item || '')),
    updated_keys: asArray(data.updated_keys).map((item) => String(item || '')),
    removed_keys: asArray(data.removed_keys).map((item) => String(item || '')),
    added: asArray(data.added).map((item) => asRecord(item)),
    updated: asArray(data.updated).map((item) => asRecord(item)),
    removed: asArray(data.removed).map((item) => asRecord(item)),
  }
}

export function normalizeReleaseImpactPayload(
  payload: unknown
): OpsReleaseImpactPreview {
  const data = asRecord(payload)
  const diffSummary = asRecord(data.diff_summary)
  const changes = asRecord(data.changes)
  return {
    site_id: String(data.site_id || ''),
    current_hash: String(data.current_hash || ''),
    previous_hash: String(data.previous_hash || ''),
    current_state: normalizeReleaseImpactState(data.current_state),
    previous_release: normalizeReleaseRecord(data.previous_release),
    has_changes: Boolean(data.has_changes),
    diff_summary: {
      option_changes: Number(diffSummary.option_changes || 0),
      group_changes: Number(diffSummary.group_changes || 0),
      group_chatops_changes: Number(diffSummary.group_chatops_changes || 0),
      group_game_changes: Number(diffSummary.group_game_changes || 0),
    },
    changes: {
      options: normalizeReleaseImpactDiffBucket(changes.options),
      groups: normalizeReleaseImpactDiffBucket(changes.groups),
      group_chatops: normalizeReleaseImpactDiffBucket(changes.group_chatops),
      group_game_configs: normalizeReleaseImpactDiffBucket(
        changes.group_game_configs
      ),
    },
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
  }
}

export function normalizeControlPlanePayload(
  payload: unknown
): OpsControlPlaneSnapshot {
  const data = asRecord(payload)
  const configured = asRecord(data.configured)
  const normalizedConfigured: Record<
    string,
    Record<string, OpsControlPlaneField>
  > = {}
  Object.entries(configured).forEach(([domain, fields]) => {
    const fieldRecord = asRecord(fields)
    normalizedConfigured[domain] = {}
    Object.entries(fieldRecord).forEach(([field, value]) => {
      normalizedConfigured[domain][field] = asRecord(
        value
      ) as OpsControlPlaneField
    })
  })

  const effective = asRecord(data.effective)
  const runtime = asRecord(data.runtime)
  const sourceMap = asRecord(data.source_map)
  const normalizedSourceMap: Record<string, OpsControlPlaneSource> = {}
  Object.entries(sourceMap).forEach(([key, value]) => {
    normalizedSourceMap[key] = asRecord(value) as OpsControlPlaneSource
  })

  const normalizedEffective: Record<string, RawRecord> = {}
  Object.entries(effective).forEach(([domain, value]) => {
    normalizedEffective[domain] = asRecord(value)
  })

  const normalizedRuntime: Record<string, RawRecord> = {}
  Object.entries(runtime).forEach(([domain, value]) => {
    normalizedRuntime[domain] = asRecord(value)
  })

  return {
    site_id: String(data.site_id || ''),
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
    configured: normalizedConfigured,
    effective: normalizedEffective,
    runtime: normalizedRuntime,
    source_map: normalizedSourceMap,
    drift: asArray(data.drift),
  }
}

export function normalizeCommunityGatePayload(
  payload: unknown
): OpsCommunityGateOverview {
  const data = asRecord(payload)
  return {
    enabled: data.enabled === undefined ? undefined : Boolean(data.enabled),
    provider_slug: String(data.provider_slug || ''),
    community_host: String(data.community_host || ''),
    room_id: String(data.room_id || ''),
    room_ids: asArray(data.room_ids).map((item) => String(item || '')),
    room_match_mode: String(data.room_match_mode || ''),
    matched_room_ids: asArray(data.matched_room_ids).map((item) =>
      String(item || '')
    ),
    missing_room_ids: asArray(data.missing_room_ids).map((item) =>
      String(item || '')
    ),
    join_targets: asArray(data.join_targets),
    bot_token_configured:
      data.bot_token_configured === undefined
        ? undefined
        : Boolean(data.bot_token_configured),
    runtime_cache: asRecord(data.runtime_cache),
    recent_audits: asArray(data.recent_audits),
    denied_message: String(data.denied_message || ''),
    audit_enabled:
      data.audit_enabled === undefined
        ? undefined
        : Boolean(data.audit_enabled),
  }
}

export function normalizeGroupCapabilitiesPayload(
  payload: unknown
): OpsGroupCapabilityMatrixOverview {
  const data = asRecord(payload)
  const sourceItems = asArray(data.items)
  const items: OpsGroupCapabilityMatrixItem[] = sourceItems.map((item) => ({
    id: Number(item.id || 0),
    site_id: String(item.site_id || ''),
    platform: String(item.platform || ''),
    platform_family: String(item.platform_family || ''),
    group_id: String(item.group_id || ''),
    group_name: String(item.group_name || item.group_id || ''),
    invite_target_group_id: String(item.invite_target_group_id || ''),
    role: String(item.role || ''),
    status: String(item.status || ''),
    enabled: Boolean(item.enabled),
    capability_policy: asRecord(item.capability_policy),
    reward_policy: asRecord(item.reward_policy),
    game_policy: asRecord(item.game_policy),
    game_configs: asArray(item.game_configs),
    access_qualifiers: asRecord(item.access_qualifiers),
    runtime_connectors: asRecord(item.runtime_connectors),
    latest_metrics: asRecord(item.latest_metrics),
    source_tables: asRecord(item.source_tables) as Record<string, string>,
  }))

  return {
    site_id: String(data.site_id || ''),
    filters: asRecord(data.filters) as Record<string, string>,
    summary: asRecord(data.summary),
    items,
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
  }
}

export function normalizeAccessExplainPayload(
  payload: unknown
): OpsAccessExplainResult {
  const data = asRecord(payload)
  return {
    site_id: String(data.site_id || ''),
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
    user: asRecord(data.user),
    requested_group: String(data.requested_group || ''),
    decision: String(data.decision || ''),
    allowed:
      data.allowed === undefined || data.allowed === null
        ? undefined
        : Boolean(data.allowed),
    reason_code: String(data.reason_code || ''),
    reason_message: String(data.reason_message || ''),
    human_message: String(data.human_message || ''),
    next_steps: asStringArray(data.next_steps),
    base_groups: asStringArray(data.base_groups),
    effective_groups: asStringArray(data.effective_groups),
    requested_group_source: asRecord(data.requested_group_source),
    status: asRecord(data.status),
    policy: data.policy === null ? null : asRecord(data.policy),
  }
}

function normalizeAuditEvent(record: RawRecord): OpsUnifiedAuditEvent {
  return {
    id: String(record.id || ''),
    domain: String(record.domain || ''),
    event_type: String(record.event_type || ''),
    title: String(record.title || ''),
    subject: String(record.subject || ''),
    status: String(record.status || ''),
    severity: String(record.severity || ''),
    reason_code: String(record.reason_code || ''),
    reason_message: String(record.reason_message || ''),
    actor: String(record.actor || ''),
    actor_user_id: optionalNumber(record.actor_user_id),
    user_id: optionalNumber(record.user_id),
    room_id: String(record.room_id || ''),
    provider_slug: String(record.provider_slug || ''),
    access_level: String(record.access_level || ''),
    at: optionalNumber(record.at),
    raw: asRecord(record.raw),
  }
}

export function normalizeAuditPayload(payload: unknown): OpsAuditOverview {
  const data = asRecord(payload)
  return {
    site_id: String(data.site_id || ''),
    summary: asRecord(data.summary),
    events: asArray(data.events).map(normalizeAuditEvent),
  }
}

export function normalizeRewardFundPayload(
  payload: unknown
): OpsRewardFundOverview {
  const rewardFundPayload = asRecord(payload)
  return {
    site_id: String(rewardFundPayload.site_id || ''),
    fund: asRecord(rewardFundPayload.fund),
    budget_pools_today: asArray(rewardFundPayload.budget_pools_today),
    budget_settings: asRecord(rewardFundPayload.budget_settings),
    effective_available_quota: Number(
      rewardFundPayload.effective_available_quota || 0
    ),
    reward_policy: asRecord(rewardFundPayload.reward_policy),
    source_breakdown: asArray(rewardFundPayload.source_breakdown),
    source_breakdown_today: asArray(rewardFundPayload.source_breakdown_today),
    commission_audit: asRecord(rewardFundPayload.commission_audit),
    invite_reward_audit: asRecord(rewardFundPayload.invite_reward_audit),
    degradation: asRecord(rewardFundPayload.degradation),
    recent_ledgers: asArray(rewardFundPayload.recent_ledgers),
    source_tables: asRecord(rewardFundPayload.source_tables) as Record<
      string,
      string
    >,
    generated_at:
      rewardFundPayload.generated_at === undefined
        ? undefined
        : Number(rewardFundPayload.generated_at || 0),
  }
}

export function normalizeInviteJourneyPayload(
  payload: unknown
): OpsInviteJourneyOverview {
  const data = asRecord(payload)
  return {
    site_id: String(data.site_id || ''),
    funnel: asRecord(data.funnel),
    state_machine: asArray(data.state_machine),
    campaigns: asArray(data.campaigns),
    claim_statuses: asArray(data.claim_statuses),
    edge_statuses: asArray(data.edge_statuses),
    event_types: asArray(data.event_types),
    risk_flags: asArray(data.risk_flags),
    reward_source: asRecord(data.reward_source),
    problems: asArray(data.problems),
    recent_events: asArray(data.recent_events),
    recent_claims: asArray(data.recent_claims),
    source_tables: asRecord(data.source_tables) as Record<string, string>,
    generated_at:
      data.generated_at === undefined
        ? undefined
        : Number(data.generated_at || 0),
  }
}
