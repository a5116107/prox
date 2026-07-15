/*
Copyright (C) 2023-2026 QuantumNous
*/

export type RiskAudit = {
  id: number
  site_id: string
  risk_type: string
  severity: string
  subject: string
  user_ids: string
  token_ids: string
  ip: string
  evidence: string
  status: string
  created_at: number
  updated_at: number
}

type RiskSummary = {
  total_open: number
  high_risk_open: number
  key_risk_open: number
  admin_candidates: number
  by_type?: Record<string, number>
  by_severity?: Record<string, number>
}

export type RiskListResult = {
  items: RiskAudit[]
  total: number
  page: number
  page_size: number
  summary: RiskSummary
}

export type RiskFilters = {
  riskType: string
  severity: string
  status: string
  keyword: string
}

type RiskTokenDetail = {
  id: number
  user_id: number
  name: string
  status: number
  group: string
}

export type RiskActionResult = {
  audit_id: number
  user_ids: number[]
  token_ids: number[]
  dry_run: boolean
  tokens: RiskTokenDetail[]
  matched_tokens: number
  matched_user_controls?: number
  disabled_tokens?: number
  restored_tokens?: number
  released_user_controls?: number
}

export type RiskActionKind = 'disable' | 'restore'

export type RiskActionPreview = {
  kind: RiskActionKind
  result: RiskActionResult
}

export const riskTypeOptions = [
  'all',
  'same_first_ip_recent_multi_account',
  'token_many_ips_7d',
  'user_many_ips_7d',
  'active_many_tokens',
  'invite_abuse',
  'no_group_membership_active_key',
  'admin_candidate',
]

export const severityOptions = ['all', 'critical', 'high', 'medium', 'low']
export const statusOptions = [
  'active',
  'open',
  'reviewing',
  'ignored',
  'closed',
  'all',
]
export const pageSizeOptions = [20, 50, 100]
