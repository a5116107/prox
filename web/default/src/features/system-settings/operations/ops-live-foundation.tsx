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
import { type ReactNode } from 'react'
import { type OpsTranslate } from './ops-i18n'
import { OpsDataTable, OpsStatusBadge, type OpsTone } from './ops-shared'
import type { OperationsSectionId } from './section-registry'
import {
  type OpsAuditOverview,
  type OpsControlPlaneField,
  type OpsControlPlaneSource,
  type OpsControlPlaneSnapshot,
  type OpsUnifiedAuditEvent,
  type OpsGroupCapabilityMatrixItem,
  type OpsGroupCapabilityMatrixOverview,
  type OpsRegistryGroup,
} from './use-ops-registry'

export type Props = {
  sectionId: OperationsSectionId
  defaultValues: Record<string, string | number | boolean>
  children: ReactNode
}

export type ViewMode = 'overview' | 'editor'
export type QuickAction = {
  id: string
  label: string
  href?: string
  onClick?: () => void
  tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
}

export type SavedOpsContext = {
  siteId: string
  siteName: string
  primaryPlatform: string
  primaryGroupIds: string[]
  communityGroupIds: string[]
  communityOnlyGroups: string[]
  fullAccessGroups: string[]
  primaryJoinUrl: string
  communityJoinUrl: string
  denyMessage: string
  upgradeMessage: string
  blockTokenCreate: boolean | null
  blockTokenEnable: boolean | null
  gateEnabled: boolean | null
  gateRoomIds: string[]
  gateRoomMatchMode: string
  botEnabled: boolean | null
  membershipEnabled: boolean | null
  membershipDryRun: boolean | null
  membershipGraceHours: number | null
  membershipPaidBypass: boolean | null
  sourceSummary: {
    accessControl: string
    communityGate: string
    communityBot: string
    membershipRisk: string
  }
}

export type OpsRenderableGroup = OpsRegistryGroup | OpsGroupCapabilityMatrixItem
export type OpsTruthRowDefinition = {
  key: string
  label: string
  help: string
  domain: string
  fields: string[]
}
type OpsPickedConfiguredField = {
  found: boolean
  field: string
  entry: OpsControlPlaneField | null
}
type OpsPickedSourceField = {
  found: boolean
  field: string
  entry: OpsControlPlaneSource | null
}
type OpsPickedValueField = {
  found: boolean
  field: string
  value: unknown
}

export const AGENT_OPS_UI_REVISION = 'agentops-workbench-r4-20260711'

const EMPTY_TEXT_VALUES = new Set([
  '',
  '-',
  '—',
  '<nil>',
  'nil',
  'null',
  'undefined',
])

export function formatTime(value: unknown) {
  const numeric = Number(value || 0)
  if (!numeric) return '—'
  const millis = numeric > 10_000_000_000 ? numeric : numeric * 1000
  return new Date(millis).toLocaleString()
}

export function normalizeText(value: unknown) {
  const text = String(value ?? '').trim()
  if (!text) return ''
  return EMPTY_TEXT_VALUES.has(text.toLowerCase()) ? '' : text
}

export function prettifyIdentifier(raw: string) {
  return raw
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function comparableText(value: unknown) {
  return normalizeText(value).toLowerCase()
}

export function firstDistinctText(values: unknown[], blocked: unknown[] = []) {
  const seen = new Set(
    blocked.map((value) => comparableText(value)).filter(Boolean)
  )

  for (const value of values) {
    const text = normalizeText(value)
    const comparable = comparableText(text)
    if (!comparable || seen.has(comparable)) continue
    seen.add(comparable)
    return text
  }

  return ''
}

export function displayText(value: unknown) {
  return normalizeText(value) || '—'
}

export function recordValue(value: unknown) {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

export function boolValue(value: unknown) {
  if (typeof value === 'boolean') return value
  const normalized = normalizeText(value).toLowerCase()
  if (
    [
      'true',
      '1',
      'yes',
      'on',
      'enabled',
      'active',
      'ok',
      'matched',
      'compliant',
    ].includes(normalized)
  ) {
    return true
  }
  if (
    [
      'false',
      '0',
      'no',
      'off',
      'disabled',
      'inactive',
      'blocked',
      'denied',
    ].includes(normalized)
  ) {
    return false
  }
  return Boolean(value)
}

export function numberValue(value: unknown) {
  const numeric = Number(value || 0)
  return Number.isFinite(numeric) ? numeric : 0
}

export function stringList(value: unknown) {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeText(item)).filter(Boolean)
  }

  const normalized = normalizeText(value)
  if (!normalized) return []

  if (normalized.startsWith('[') && normalized.endsWith(']')) {
    try {
      const parsed = JSON.parse(normalized)
      if (Array.isArray(parsed)) {
        return parsed.map((item) => normalizeText(item)).filter(Boolean)
      }
    } catch (error) {
      if (!(error instanceof SyntaxError)) throw error
    }
  }

  if (normalized.includes(',')) {
    return normalized
      .split(',')
      .map((item) => normalizeText(item))
      .filter(Boolean)
  }

  return [normalized]
}

export function resolveCapabilityRows(
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null,
  groups: OpsRegistryGroup[]
) {
  return (
    groupCapabilities?.items?.length ? groupCapabilities.items : groups
  ) as OpsRenderableGroup[]
}

export function capabilityPolicyOf(group: OpsRenderableGroup) {
  return 'capability_policy' in group
    ? recordValue(group.capability_policy)
    : recordValue('capabilities' in group ? group.capabilities : {})
}

export function rewardPolicyOf(group: OpsRenderableGroup) {
  if ('reward_policy' in group) {
    return recordValue(group.reward_policy)
  }
  const capabilityPolicy = capabilityPolicyOf(group)
  return {
    checkin_quota: capabilityPolicy.checkin_quota,
    verify_min_quota: capabilityPolicy.verify_min_quota,
    invite_reward_quota: capabilityPolicy.invite_reward_quota,
    invitee_reward_quota: capabilityPolicy.invitee_reward_quota,
    daily_group_reward_limit: capabilityPolicy.daily_group_reward_limit,
  }
}

export function gamePolicyOf(group: OpsRenderableGroup) {
  return 'game_policy' in group ? recordValue(group.game_policy) : {}
}

export function gameConfigsOf(group: OpsRenderableGroup) {
  const raw = 'game_configs' in group ? group.game_configs : []
  return Array.isArray(raw) ? raw.map((item) => recordValue(item)) : []
}

export function countEnabledGameRules(
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null,
  groups: OpsRegistryGroup[]
) {
  const summaryCount = numberValue(
    recordValue(groupCapabilities?.summary).enabled_game_rules
  )
  if (summaryCount) return summaryCount

  return resolveCapabilityRows(groupCapabilities, groups).reduce(
    (sum, group) => {
      const gamePolicy = gamePolicyOf(group)
      const enabledCodes = stringList(gamePolicy.enabled_game_codes).length
      const enabledConfigs = gameConfigsOf(group).filter((config) =>
        boolValue(config.enabled)
      ).length
      return sum + Math.max(enabledCodes, enabledConfigs)
    },
    0
  )
}

export function accessQualifiersOf(group: OpsRenderableGroup) {
  return recordValue(group.access_qualifiers)
}

export function runtimeConnectorsOf(group: OpsRenderableGroup) {
  return recordValue(group.runtime_connectors)
}

export function latestMetricsOf(group: OpsRenderableGroup) {
  return recordValue(group.latest_metrics)
}

export function platformLabel(raw: string | undefined, t?: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'qq') return t ? t('QQ') : 'QQ'
  if (value === 'tg') return t ? t('TG') : 'TG'
  if (value === 'community') return t ? t('Community') : 'Community'
  return normalizeText(raw) || '—'
}

export function releaseModeLabel(raw: string | undefined, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'audit_only') return t('Audit only')
  if (value === 'draft') return t('Draft')
  if (value === 'published') return t('Published')
  if (value === 'live') return t('Live')
  return value ? prettifyIdentifier(value) : t('Unknown')
}

export function releaseActionLabel(raw: string | undefined, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'publish') return t('Published snapshot')
  if (value === 'rollback') return t('Rollback snapshot')
  return value ? prettifyIdentifier(value) : t('Snapshot')
}

export function groupRoleLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'primary_mainfield') return t('Primary main field')
  if (value === 'community_intake') return t('Community intake')
  if (value === 'ops_secondary') return t('Ops secondary')
  if (value === 'campaign') return t('Campaign')
  if (value === 'backup') return t('Backup')
  if (value === 'manual_whitelist') return t('Manual whitelist')
  return value ? prettifyIdentifier(value) : t('Role not set')
}

export function groupStatusLabel(
  raw: unknown,
  enabled: boolean,
  t: OpsTranslate
) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'active' || value === 'enabled') return t('Active')
  if (value === 'inactive' || value === 'disabled') return t('Disabled')
  if (value === 'audit_only') return t('Audit only')
  return value
    ? prettifyIdentifier(value)
    : enabled
      ? t('Active')
      : t('Disabled')
}

export function accessLevelLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'full_access') return t('Full access')
  if (value === 'community_only') return t('Community-only access')
  if (value === 'admin_bypass') return t('Administrator bypass')
  if (value === 'paid_bypass') return t('Paid-user bypass')
  if (value === 'none') return t('No access')
  return value ? prettifyIdentifier(value) : '—'
}

export function overrideModeLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (!value || value === 'none') return t('No override')
  if (value === 'admin_bypass') return t('Administrator bypass')
  if (value === 'paid_bypass') return t('Paid-user bypass')
  if (value === 'manual_allow') return t('Manual allow')
  if (value === 'manual_deny') return t('Manual deny')
  return prettifyIdentifier(value)
}

export function reasonCodeLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'ok') return t('Passed')
  if (value === 'disabled') return t('Disabled')
  if (value === 'invalid_user') return t('Invalid user')
  if (value === 'community_bound') return t('Community binding matched')
  if (value === 'community_bound_user') return t('Community binding matched')
  if (value === 'primary_bound') return t('Primary binding matched')
  if (value === 'admin_bypass') return t('Administrator bypass')
  if (value === 'paid_bypass') return t('Paid-user bypass')
  if (value === 'not_bound') return t('Not bound')
  if (value === 'missing_community_binding')
    return t('Missing community binding')
  if (value === 'missing_oauth_binding')
    return t('Missing community OAuth binding')
  if (value === 'missing_primary_binding')
    return t('Missing main-field binding')
  if (value === 'room_not_configured') return t('Community room not configured')
  if (value === 'missing_required_rooms')
    return t('Missing required community rooms')
  if (value === 'not_in_any_required_room') return t('Not in any required room')
  if (value === 'check_error') return t('Check failed')
  if (value === 'freeze_error') return t('Freeze failed')
  return value ? prettifyIdentifier(value) : '—'
}

const REASON_MESSAGE_KEYS: Record<string, string> = {
  game_admin_update_group: 'Game admin group update',
  'community only': 'Community-only access',
  'community bound': 'Community binding matched',
  'community group bound': 'Community binding matched',
  'administrator bypass': 'Administrator bypass',
  'admin bypass': 'Administrator bypass',
  'paid bypass': 'Paid-user bypass',
  'primary group bound': 'Primary binding matched',
  'community gate disabled': 'Community gate disabled',
  'community gate passed': 'Community gate passed',
  'community gate room ids are not configured':
    'Community gate rooms are not configured yet',
  'missing required dc.hhhl.cc oauth binding':
    'Missing required community OAuth binding',
}

const ROOM_REASON_PATTERNS = [
  {
    pattern: /^matched required community rooms:\s*(.+)$/i,
    label: 'Matched required community rooms',
  },
  {
    pattern: /^missing required community rooms:\s*(.+)$/i,
    label: 'Missing required community rooms',
  },
  {
    pattern: /^not in any required community room \(required:\s*(.+)\)$/i,
    label: 'Not in any required room',
  },
] as const

function matchedRoomReasonMessage(value: string, t: OpsTranslate) {
  for (const { pattern, label } of ROOM_REASON_PATTERNS) {
    const match = value.match(pattern)
    if (match) return `${t(label)}: ${match[1]}`
  }
  return null
}

export function reasonMessageLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw)
  if (!value) return '—'
  const staticLabel = REASON_MESSAGE_KEYS[value.toLowerCase()]
  if (staticLabel) return t(staticLabel)
  return matchedRoomReasonMessage(value, t) ?? value
}

export function bindingLabel(value: unknown, t: OpsTranslate) {
  const normalized = normalizeText(value)
  if (!normalized) return '—'
  return boolValue(value) ? t('Bound') : t('Not bound')
}

export function capabilityLabel(value: unknown, t: OpsTranslate) {
  const normalized = normalizeText(value)
  if (!normalized) return '—'
  return boolValue(value) ? t('Enabled') : t('Disabled')
}

export function formatGroupList(value: unknown) {
  const groups = stringList(value)
  return groups.length > 0 ? groups.join(', ') : '—'
}

function controlPlaneConfiguredValue(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  field: string
) {
  return controlPlane?.configured?.[domain]?.[field]?.value
}

function controlPlaneEffectiveValue(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  field: string
) {
  return controlPlane?.effective?.[domain]?.[field]
}

function hasOwnField(record: Record<string, unknown>, field: string) {
  return Object.prototype.hasOwnProperty.call(record, field)
}

export function pickConfiguredField(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  fields: string[]
): OpsPickedConfiguredField {
  const domainFields = controlPlane?.configured?.[domain] || {}
  for (const field of fields) {
    if (!hasOwnField(domainFields, field)) continue
    const entry = domainFields[field]
    return {
      found: true,
      field,
      entry:
        entry && typeof entry === 'object'
          ? (entry as OpsControlPlaneField)
          : null,
    }
  }
  return {
    found: false,
    field: fields[0] || '',
    entry: null,
  }
}

function pickValueField(
  source: Record<string, unknown> | undefined,
  fields: string[]
): OpsPickedValueField {
  const sourceRecord = source || {}
  for (const field of fields) {
    if (!hasOwnField(sourceRecord, field)) continue
    return {
      found: true,
      field,
      value: sourceRecord[field],
    }
  }
  return {
    found: false,
    field: fields[0] || '',
    value: undefined,
  }
}

export function pickEffectiveField(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  fields: string[]
) {
  return pickValueField(controlPlane?.effective?.[domain], fields)
}

export function pickRuntimeField(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  fields: string[]
) {
  return pickValueField(controlPlane?.runtime?.[domain], fields)
}

export function pickSourceField(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  fields: string[]
): OpsPickedSourceField {
  const sourceMap = controlPlane?.source_map || {}
  for (const field of fields) {
    const key = `${domain}.${field}`
    if (!hasOwnField(sourceMap, key)) continue
    const entry = sourceMap[key]
    return {
      found: true,
      field,
      entry:
        entry && typeof entry === 'object'
          ? (entry as OpsControlPlaneSource)
          : null,
    }
  }
  return {
    found: false,
    field: fields[0] || '',
    entry: null,
  }
}

function formatOpsFieldValue(value: unknown, t: OpsTranslate): string {
  if (typeof value === 'boolean') return value ? t('Enabled') : t('Disabled')
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeText(item))
      .filter(Boolean)
      .join(', ')
  }
  if (value && typeof value === 'object') {
    const objectValue = recordValue(value)
    return Object.keys(objectValue).length > 0
      ? JSON.stringify(objectValue)
      : ''
  }

  const text = normalizeText(value)
  if (!text) return ''

  if (
    (text.startsWith('[') && text.endsWith(']')) ||
    (text.startsWith('{') && text.endsWith('}'))
  ) {
    try {
      const parsed = JSON.parse(text)
      if (Array.isArray(parsed)) {
        return parsed
          .map((item) => normalizeText(item))
          .filter(Boolean)
          .join(', ')
      }
      if (parsed && typeof parsed === 'object') {
        return JSON.stringify(parsed)
      }
    } catch (error) {
      if (!(error instanceof SyntaxError)) throw error
    }
  }

  return text
}

function comparableOpsFieldValue(value: unknown) {
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  if (Array.isArray(value)) {
    return JSON.stringify(
      value.map((item) => normalizeText(item)).filter(Boolean)
    )
  }
  if (value && typeof value === 'object') {
    return JSON.stringify(recordValue(value))
  }
  return normalizeText(value)
}

export function truthAlignmentState(
  configured: OpsPickedConfiguredField,
  effective: OpsPickedValueField,
  runtime: OpsPickedValueField,
  t: OpsTranslate
): {
  label: string
  tone: OpsTone
} {
  if (!configured.found && !effective.found && !runtime.found) {
    return {
      label: t('No configured value yet'),
      tone: 'neutral',
    }
  }
  if (!configured.found && !effective.found && runtime.found) {
    return {
      label: t('Only runtime has a value'),
      tone: 'warning',
    }
  }
  if (!configured.found && (effective.found || runtime.found)) {
    return {
      label: t('Only effective/runtime has a value'),
      tone: 'info',
    }
  }
  if (configured.found && !effective.found) {
    return {
      label: t('Saved but not currently applied'),
      tone: 'warning',
    }
  }

  const configuredValue = comparableOpsFieldValue(configured.entry?.value)
  const effectiveValue = comparableOpsFieldValue(effective.value)
  const runtimeValue = comparableOpsFieldValue(runtime.value)

  if (
    configuredValue === effectiveValue &&
    (!runtime.found || effectiveValue === runtimeValue)
  ) {
    return {
      label: t('Configured, applied, and runtime values are aligned'),
      tone: 'success',
    }
  }

  return {
    label: t('Saved and live still differ'),
    tone: 'warning',
  }
}

export function sourceStateLabelDetailed(
  source: OpsControlPlaneSource | null | undefined,
  t: OpsTranslate
) {
  const state = normalizeText(source?.state)
  if (state === 'in_effect') return t('Saved and active')
  if (state === 'using_compatible_value') return t('Using compatible fallback')
  if (state === 'using_default') return t('Using runtime default')
  return (
    normalizeText(source?.message) || t('Runtime truth from live control plane')
  )
}

export function sourceOriginLabel(
  _source: OpsControlPlaneSource | null | undefined
) {
  return ''
}

export function renderTruthValueCell(
  kind: 'configured' | 'effective' | 'runtime',
  found: boolean,
  value: unknown,
  t: OpsTranslate
) {
  const formatted = formatOpsFieldValue(value, t)
  if (formatted) return formatted
  if (kind === 'configured') {
    return found ? t('Saved empty') : t('No configured value yet')
  }
  if (kind === 'effective') {
    return found ? t('Saved empty') : t('No effective value')
  }
  return found ? '—' : t('Awaiting runtime')
}

export function controlPlaneValue(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  field: string,
  fallback?: unknown
) {
  const effective = controlPlaneEffectiveValue(controlPlane, domain, field)
  if (
    effective !== undefined &&
    effective !== null &&
    normalizeText(effective) !== ''
  ) {
    return effective
  }
  const configured = controlPlaneConfiguredValue(controlPlane, domain, field)
  if (
    configured !== undefined &&
    configured !== null &&
    normalizeText(configured) !== ''
  ) {
    return configured
  }
  return fallback
}

export function controlPlaneSourceLabel(
  controlPlane: OpsControlPlaneSnapshot | null,
  domain: string,
  fields: string[],
  t: OpsTranslate
) {
  const labels = fields
    .map((field) => controlPlane?.source_map?.[`${domain}.${field}`])
    .filter(Boolean)
    .map((source) => {
      const state = normalizeText(source?.state)
      if (state === 'in_effect') return t('Saved and active')
      if (state === 'using_compatible_value')
        return t('Using compatible fallback')
      if (state === 'using_default') return t('Using runtime default')
      return normalizeText(source?.message)
    })
    .filter(Boolean)
  return labels[0] || t('Runtime truth from live control plane')
}

function scopeLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'game_config_sync') return t('Game config sync')
  if (value === 'legacy_game_config_import')
    return t('Legacy adapter import attempt')
  if (value === 'access_control') return t('Access control')
  if (value === 'community_gate') return t('Community gate')
  if (value === 'membership_risk') return t('Membership risk')
  return value ? prettifyIdentifier(value) : '—'
}

export function accessStateExplanation(
  row: Record<string, unknown>,
  t: OpsTranslate
) {
  const access = accessLevelLabel(row.access_level, t)
  return firstDistinctText(
    [
      reasonMessageLabel(row.reason_message || row.reason, t),
      reasonCodeLabel(row.reason_code, t),
    ],
    [access]
  )
}

export function auditEventsByDomain(
  audits: OpsAuditOverview | null,
  domains: string[]
) {
  const wanted = new Set(
    domains.map((domain) => normalizeText(domain).toLowerCase())
  )
  return (audits?.events ?? []).filter((event) =>
    wanted.has(normalizeText(event.domain).toLowerCase())
  )
}

export function auditRawRows(audits: OpsAuditOverview | null, domain: string) {
  return auditEventsByDomain(audits, [domain]).map((event) =>
    recordValue(event.raw)
  )
}

function auditDomainLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'admin_config') return t('Config audit')
  if (value === 'community_gate') return t('Community gate')
  if (value === 'access_control') return t('Access control')
  if (value === 'risk_control') return t('Risk control')
  return value ? prettifyIdentifier(value) : t('Audit')
}

function riskSeverityTone(raw: unknown): OpsTone {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'critical' || value === 'high') return 'danger'
  if (value === 'medium') return 'warning'
  if (value === 'low') return 'info'
  return 'neutral'
}

function auditEventStatusBadge(event: OpsUnifiedAuditEvent, t: OpsTranslate) {
  const domain = normalizeText(event.domain).toLowerCase()
  if (domain === 'community_gate') {
    return statusBadge(
      normalizeText(event.status).toLowerCase() === 'compliant',
      t('Compliant'),
      t('Denied'),
      t('Unknown')
    )
  }
  if (domain === 'access_control') {
    return (
      <OpsStatusBadge tone='info'>
        {accessLevelLabel(event.access_level || event.status, t)}
      </OpsStatusBadge>
    )
  }
  if (domain === 'risk_control') {
    const label = firstDistinctText(
      [prettifyIdentifier(event.severity), prettifyIdentifier(event.status)],
      []
    )
    return (
      <OpsStatusBadge tone={riskSeverityTone(event.severity)}>
        {label || t('Risk')}
      </OpsStatusBadge>
    )
  }
  return (
    <OpsStatusBadge tone='neutral'>
      {scopeLabel(event.title || event.event_type, t)}
    </OpsStatusBadge>
  )
}

function auditEventSubjectLines(event: OpsUnifiedAuditEvent) {
  return [
    firstDistinctText([event.subject, recordValue(event.raw).target_key]),
    firstDistinctText([
      recordValue(event.raw).target_value,
      event.room_id
        ? `${event.provider_slug || 'room'} · ${event.room_id}`
        : '',
      event.actor,
      event.access_level,
    ]),
  ].filter(Boolean)
}

export function UnifiedAuditTimeline({
  title,
  description,
  events,
  t,
  emptyMessage,
}: {
  title: string
  description: string
  events: OpsUnifiedAuditEvent[]
  t: OpsTranslate
  emptyMessage: string
}) {
  return (
    <OpsDataTable
      title={title}
      description={description}
      columns={[
        { key: 'source', label: t('Source') },
        { key: 'subject', label: t('Subject') },
        { key: 'status', label: t('Status') },
        { key: 'reason', label: t('Reason') },
        { key: 'time', label: t('Time') },
      ]}
      rows={events.slice(0, 10).map((event, index) => {
        const raw = recordValue(event.raw)
        const subjectLines = auditEventSubjectLines(event)
        const reasonPrimary = firstDistinctText(
          [
            reasonCodeLabel(event.reason_code, t),
            reasonCodeLabel(raw.reason_code, t),
            event.reason_message,
          ],
          []
        )
        const reasonSecondary = firstDistinctText(
          [
            reasonMessageLabel(event.reason_message || raw.reason || raw.ip, t),
            event.actor,
          ],
          [reasonPrimary]
        )
        return {
          id: event.id || `${event.domain}-${event.at || index}`,
          cells: [
            <div className='space-y-1 text-sm'>
              <div className='font-medium'>
                {auditDomainLabel(event.domain, t)}
              </div>
              <div className='text-muted-foreground text-xs'>
                {scopeLabel(event.title || event.event_type, t)}
              </div>
            </div>,
            <div className='space-y-1 text-sm'>
              <div>{displayText(subjectLines[0])}</div>
              {subjectLines[1] ? (
                <div className='text-muted-foreground text-xs'>
                  {subjectLines[1]}
                </div>
              ) : null}
            </div>,
            auditEventStatusBadge(event, t),
            <div className='space-y-1 text-sm'>
              <div>{displayText(reasonPrimary)}</div>
              {reasonSecondary ? (
                <div className='text-muted-foreground text-xs'>
                  {reasonSecondary}
                </div>
              ) : null}
            </div>,
            formatTime(event.at),
          ],
        }
      })}
      emptyMessage={emptyMessage}
    />
  )
}

export function statusBadge(
  value: boolean | null,
  trueLabel: string,
  falseLabel: string,
  unknownLabel: string
) {
  if (value === null)
    return <OpsStatusBadge tone='neutral'>{unknownLabel}</OpsStatusBadge>
  return (
    <OpsStatusBadge tone={value ? 'success' : 'warning'}>
      {value ? trueLabel : falseLabel}
    </OpsStatusBadge>
  )
}
