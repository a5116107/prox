/*
Copyright (C) 2023-2026 QuantumNous
*/
import { z } from 'zod'
import { type OpsMatrixRow, type OpsTone } from './ops-shared'

type MembershipRiskStoredValues = Record<string, string | number | boolean>
type Translate = (key: string) => string

export type MembershipOverview = {
  enabled: boolean
  dry_run: boolean
  counts: Record<string, number>
}

type MembershipUnresolvedCandidate = {
  user_id: number
  username: string
  community_external_user_id: string
  matched_by: string
  match_type: string
}

type MembershipUnresolvedBindingHint = {
  source: string
  room_id: string
  new_api_user_id: number
  username: string
  enabled: boolean
  remark: string
}

export type MembershipUnresolvedRecord = {
  state_id: number
  source: string
  room_id: string
  external_user_id: string
  status: string
  updated_at: number
  latest_event_type: string
  latest_event_at: number
  identity_hints: string[]
  match_candidates: MembershipUnresolvedCandidate[]
  existing_bindings: MembershipUnresolvedBindingHint[]
  suggested_action: string
  reason: string
}

export type MembershipUnresolvedOverview = {
  limit: number
  unresolved_state_count: number
  unresolved_event_count: number
  records: MembershipUnresolvedRecord[]
}

export type MembershipDryRun = {
  grace_to_expire: number
  dry_run: boolean
}

export type MembershipState = {
  id: number
  site_id: string
  source: string
  room_id: string
  external_user_id: string
  new_api_user_id: number
  status: string
  joined_at: number
  last_seen_at: number
  left_at: number
  grace_until: number
  restored_at: number
  leave_count_30d: number
  risk_score: number
  bypass_reason: string
  bypass_until: number
  updated_at: number
}

export const membershipRiskSchema = z.object({
  enabled: z.boolean(),
  dryRun: z.boolean(),
  graceHours: z.number().int().min(1).max(720),
  autoRestoreOnRejoin: z.boolean(),
  paidBypassEnabled: z.boolean(),
  eventSecret: z.string().optional(),
  freezeCommunityTokensAfterGrace: z.boolean(),
  revokeCommunityAccessAfterGrace: z.boolean(),
  blockCheckinOnLeft: z.boolean(),
  blockGameRewardOnLeft: z.boolean(),
  blockInviteRewardOnLeft: z.boolean(),
  blockCampaignBonusOnLeft: z.boolean(),
  notifyUserOnLeft: z.boolean(),
  notifyAdminOnBulkLeft: z.boolean(),
  qqEventsEnabled: z.boolean(),
  tgEventsEnabled: z.boolean(),
  scheduledRecheckEnabled: z.boolean(),
  scheduledRecheckIntervalHours: z.number().int().min(1).max(720),
})

export type MembershipRiskValues = z.infer<typeof membershipRiskSchema>

export const membershipRiskOptionKeyMap: Record<
  keyof MembershipRiskValues,
  string
> = {
  enabled: 'membership_risk.enabled',
  dryRun: 'membership_risk.dry_run',
  graceHours: 'membership_risk.grace_hours',
  autoRestoreOnRejoin: 'membership_risk.auto_restore_on_rejoin',
  paidBypassEnabled: 'membership_risk.paid_bypass_enabled',
  eventSecret: 'membership_risk.event_secret',
  freezeCommunityTokensAfterGrace:
    'membership_risk.freeze_community_tokens_after_grace',
  revokeCommunityAccessAfterGrace:
    'membership_risk.revoke_community_access_after_grace',
  blockCheckinOnLeft: 'membership_risk.block_checkin_on_left',
  blockGameRewardOnLeft: 'membership_risk.block_game_reward_on_left',
  blockInviteRewardOnLeft: 'membership_risk.block_invite_reward_on_left',
  blockCampaignBonusOnLeft: 'membership_risk.block_campaign_bonus_on_left',
  notifyUserOnLeft: 'membership_risk.notify_user_on_left',
  notifyAdminOnBulkLeft: 'membership_risk.notify_admin_on_bulk_left',
  qqEventsEnabled: 'membership_risk.qq_events_enabled',
  tgEventsEnabled: 'membership_risk.tg_events_enabled',
  scheduledRecheckEnabled: 'membership_risk.scheduled_recheck_enabled',
  scheduledRecheckIntervalHours:
    'membership_risk.scheduled_recheck_interval_hours',
}

export const membershipStatusOptions = [
  '',
  'active',
  'unbound_observed',
  'grace',
  'left_expired',
  'restored',
  'manual_bypass',
  'rejoin_pending',
  'suspected_left',
]

function booleanOption(
  storedValues: MembershipRiskStoredValues,
  key: string,
  fallback: boolean
) {
  const storedValue = storedValues[key]
  if (storedValue === undefined || storedValue === null) return fallback
  return Boolean(storedValue)
}

function numberOption(
  storedValues: MembershipRiskStoredValues,
  key: string,
  fallback: number
) {
  const storedValue = storedValues[key]
  if (!storedValue) return fallback
  return Number(storedValue)
}

function stringOption(storedValues: MembershipRiskStoredValues, key: string) {
  const storedValue = storedValues[key]
  if (!storedValue) return ''
  return String(storedValue)
}

export function buildMembershipRiskDefaults(
  defaultValues: MembershipRiskStoredValues
): MembershipRiskValues {
  return {
    enabled: booleanOption(defaultValues, 'membership_risk.enabled', false),
    dryRun: booleanOption(defaultValues, 'membership_risk.dry_run', true),
    graceHours: numberOption(defaultValues, 'membership_risk.grace_hours', 24),
    autoRestoreOnRejoin: booleanOption(
      defaultValues,
      'membership_risk.auto_restore_on_rejoin',
      true
    ),
    paidBypassEnabled: booleanOption(
      defaultValues,
      'membership_risk.paid_bypass_enabled',
      true
    ),
    eventSecret: stringOption(defaultValues, 'membership_risk.event_secret'),
    freezeCommunityTokensAfterGrace: booleanOption(
      defaultValues,
      'membership_risk.freeze_community_tokens_after_grace',
      true
    ),
    revokeCommunityAccessAfterGrace: booleanOption(
      defaultValues,
      'membership_risk.revoke_community_access_after_grace',
      true
    ),
    blockCheckinOnLeft: booleanOption(
      defaultValues,
      'membership_risk.block_checkin_on_left',
      true
    ),
    blockGameRewardOnLeft: booleanOption(
      defaultValues,
      'membership_risk.block_game_reward_on_left',
      true
    ),
    blockInviteRewardOnLeft: booleanOption(
      defaultValues,
      'membership_risk.block_invite_reward_on_left',
      true
    ),
    blockCampaignBonusOnLeft: booleanOption(
      defaultValues,
      'membership_risk.block_campaign_bonus_on_left',
      true
    ),
    notifyUserOnLeft: booleanOption(
      defaultValues,
      'membership_risk.notify_user_on_left',
      true
    ),
    notifyAdminOnBulkLeft: booleanOption(
      defaultValues,
      'membership_risk.notify_admin_on_bulk_left',
      true
    ),
    qqEventsEnabled: booleanOption(
      defaultValues,
      'membership_risk.qq_events_enabled',
      true
    ),
    tgEventsEnabled: booleanOption(
      defaultValues,
      'membership_risk.tg_events_enabled',
      true
    ),
    scheduledRecheckEnabled: booleanOption(
      defaultValues,
      'membership_risk.scheduled_recheck_enabled',
      true
    ),
    scheduledRecheckIntervalHours: numberOption(
      defaultValues,
      'membership_risk.scheduled_recheck_interval_hours',
      12
    ),
  }
}

export function formatMembershipTime(ts?: number) {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString()
}

export function membershipStatusTone(status: string) {
  if (['active', 'restored', 'rejoin_pending'].includes(status)) {
    return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
  }
  if (status === 'manual_bypass') {
    return 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300'
  }
  if (status === 'unbound_observed') {
    return 'border-orange-500/20 bg-orange-500/10 text-orange-700 dark:text-orange-300'
  }
  if (status === 'grace') {
    return 'border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300'
  }
  return 'border-rose-500/20 bg-rose-500/10 text-rose-700 dark:text-rose-300'
}

export function translateMembershipStatus(status: string, t: Translate) {
  const labels: Record<string, string> = {
    enabled: 'Enabled',
    disabled: 'Disabled',
    dry_run: 'Dry run',
    enforced: 'Live enforcement',
    active: 'Active',
    unbound_observed: 'Observed but not linked',
    grace: 'Grace',
    left_expired: 'Expired',
    restored: 'Restored',
    manual_bypass: 'Manual bypass',
    rejoin_pending: 'Pending rejoin',
    suspected_left: 'Suspected leave',
  }
  return t(labels[status] || status)
}

export function translateUnresolvedAction(action: string, t: Translate) {
  const labels: Record<string, string> = {
    rerun_binding_backfill: 'Recheck binding backfill',
    review_then_backfill: 'Review and backfill',
    manual_pick_user: 'Manually pick the user',
    ask_user_to_finish_binding: 'Guide the user to finish binding',
    collect_identity_evidence: 'Collect more identity evidence',
  }
  return t(labels[action] || action || 'Unknown')
}

export function unresolvedActionTone(action: string): OpsTone {
  if (
    action === 'review_then_backfill' ||
    action === 'rerun_binding_backfill'
  ) {
    return 'info'
  }
  if (
    action === 'manual_pick_user' ||
    action === 'ask_user_to_finish_binding'
  ) {
    return 'warning'
  }
  return 'danger'
}

export function membershipStatusOpsTone(status: string): OpsTone {
  if (['active', 'restored', 'rejoin_pending'].includes(status)) {
    return 'success'
  }
  if (status === 'manual_bypass') return 'info'
  if (status === 'unbound_observed') return 'warning'
  if (status === 'grace') return 'warning'
  return 'danger'
}

export function buildMembershipBenefitMatrix(
  values: MembershipRiskValues,
  t: Translate
): OpsMatrixRow[] {
  const allow = { label: t('Allowed'), tone: 'success' as const }
  const block = { label: t('Blocked'), tone: 'danger' as const }
  const byRule = { label: t('By rule'), tone: 'warning' as const }
  const paidCell = values.paidBypassEnabled
    ? { label: t('Allowed'), tone: 'info' as const }
    : { label: t('Blocked'), tone: 'danger' as const }

  const gateRow = (benefit: string, blockFlag: boolean): OpsMatrixRow => ({
    benefit: t(benefit),
    active: allow,
    grace: blockFlag ? byRule : allow,
    expired: blockFlag ? block : allow,
    paid: paidCell,
  })

  const notifyOnLeft = values.notifyUserOnLeft || values.notifyAdminOnBulkLeft
  const adminRow: OpsMatrixRow = {
    benefit: t('Admin notification'),
    active: { label: t('Record'), tone: 'success' },
    grace: notifyOnLeft
      ? { label: t('Remind'), tone: 'warning' }
      : { label: t('Record'), tone: 'success' },
    expired: values.notifyAdminOnBulkLeft
      ? { label: t('Alert'), tone: 'danger' }
      : notifyOnLeft
        ? { label: t('Remind'), tone: 'warning' }
        : { label: t('Record'), tone: 'success' },
    paid: { label: t('Record'), tone: 'info' },
  }

  return [
    gateRow(
      'Community key',
      values.freezeCommunityTokensAfterGrace ||
        values.revokeCommunityAccessAfterGrace
    ),
    gateRow('Check-in reward', values.blockCheckinOnLeft),
    gateRow('Game reward', values.blockGameRewardOnLeft),
    gateRow('Invite reward', values.blockInviteRewardOnLeft),
    gateRow('Campaign bonus', values.blockCampaignBonusOnLeft),
    adminRow,
  ]
}
