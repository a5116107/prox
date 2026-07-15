/*
Copyright (C) 2023-2026 QuantumNous
*/
import type { OpsTranslate } from './ops-i18n'
import type { OpsTone } from './ops-shared'
import type { RiskAudit } from './risk-control-types'

type ErrorResponse = {
  response?: {
    data?: {
      message?: unknown
    }
  }
  message?: unknown
}

export function parseRiskIdList(value: string) {
  const raw = String(value || '').trim()
  if (!raw || raw === '[]') return []
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error
  }
  if (Array.isArray(parsed)) {
    return parsed
      .map((item) => Number(item))
      .filter((item) => Number.isInteger(item) && item > 0)
  }
  return []
}

export function parseRiskEvidence(value: string) {
  const raw = String(value || '').trim()
  if (!raw || raw === '{}') return null
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error
  }
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    return parsed as Record<string, unknown>
  }
  return null
}

export function compactRiskEvidence(value: string, t: OpsTranslate) {
  const evidence = parseRiskEvidence(value)
  if (!evidence) return String(value || '').trim() || '-'
  const parts: string[] = []
  if (evidence.reason) parts.push(`${t('Reason')}: ${String(evidence.reason)}`)
  if (evidence.action) parts.push(`${t('Action')}: ${String(evidence.action)}`)
  if (evidence.users)
    parts.push(`${t('User count')}: ${String(evidence.users)}`)
  if (evidence.ips) parts.push(`${t('IP count')}: ${String(evidence.ips)}`)
  if (evidence.reqs) parts.push(`${t('Requests')}: ${String(evidence.reqs)}`)
  if (evidence.quota) parts.push(`${t('Quota')}: ${String(evidence.quota)}`)
  if (evidence.token_id)
    parts.push(`${t('Token')}: ${String(evidence.token_id)}`)
  if (evidence.token_name) {
    parts.push(`${t('Key name')}: ${String(evidence.token_name)}`)
  }
  if (Array.isArray(evidence.ip_list)) {
    const ips = evidence.ip_list.map(String)
    parts.push(
      `${t('IPs')}: ${ips.slice(0, 3).join(' / ')}${ips.length > 3 ? '...' : ''}`
    )
  }
  if (Array.isArray(evidence.details)) {
    parts.push(`${t('Details')}: ${evidence.details.length}`)
  }
  if (Array.isArray(evidence.tokens)) {
    const tokens = evidence.tokens.map(String)
    parts.push(
      `${t('Keys')}: ${tokens.slice(0, 3).join(' / ')}${tokens.length > 3 ? '...' : ''}`
    )
  }
  return (
    parts.length > 0 ? parts.join(' | ') : JSON.stringify(evidence)
  ).slice(0, 260)
}

export function defaultRiskActionReason(
  audit: RiskAudit | null,
  t: OpsTranslate
) {
  if (!audit) return ''
  return t('Risk action default reason', {
    risk: riskTypeLabel(audit.risk_type, t),
    subject: audit.subject || String(audit.id),
    defaultValue:
      'Administrator review: {{risk}}, subject {{subject}}. Operate only the API keys explicitly attached to this audit.',
  })
}

export function severityTone(severity: string): OpsTone {
  if (severity === 'critical' || severity === 'high') return 'danger'
  if (severity === 'medium') return 'warning'
  if (severity === 'low') return 'info'
  return 'neutral'
}

export function statusTone(status: string): OpsTone {
  if (status === 'open') return 'warning'
  if (status === 'reviewing') return 'info'
  if (status === 'closed') return 'success'
  return 'neutral'
}

export function riskTypeLabel(type: string, t: OpsTranslate) {
  const labels: Record<string, string> = {
    same_first_ip_recent_multi_account: 'Same first-login IP multiple accounts',
    token_many_ips_7d: 'Key touched many IPs in 7 days',
    user_many_ips_7d: 'User touched many IPs in 7 days',
    active_many_tokens: 'Multiple active keys',
    invite_abuse: 'Invite abuse',
    no_group_membership_active_key: 'Active key without valid group membership',
    no_group_active_key: 'Active key without valid group membership',
    admin_candidate: 'Admin candidate / false positive',
  }
  return t(labels[type] || type)
}

export function tokenStatusLabel(status: number, t: OpsTranslate) {
  const labels: Record<number, string> = {
    1: 'Enabled',
    2: 'Disabled',
    3: 'Expired',
    4: 'Exhausted',
  }
  return t(labels[status] || 'Unknown')
}

export function tokenStatusTone(status: number): OpsTone {
  if (status === 1) return 'success'
  if (status === 2) return 'warning'
  return status > 2 ? 'danger' : 'neutral'
}

export function formatRiskTime(value: number) {
  if (!value) return '-'
  const millis = value > 10_000_000_000 ? value : value * 1000
  return new Date(millis).toLocaleString()
}

export function riskErrorMessage(error: unknown, fallback: string) {
  if (!error || typeof error !== 'object') return fallback
  const candidate = error as ErrorResponse
  const responseMessage = candidate.response?.data?.message
  if (typeof responseMessage === 'string' && responseMessage.trim()) {
    return responseMessage
  }
  return typeof candidate.message === 'string' && candidate.message.trim()
    ? candidate.message
    : fallback
}

export function allowedRiskStatuses(current: string) {
  const transitions: Record<string, string[]> = {
    open: ['reviewing', 'ignored', 'closed'],
    reviewing: ['open', 'ignored', 'closed'],
    ignored: ['open', 'closed'],
    closed: [],
  }
  return transitions[current] ?? []
}
