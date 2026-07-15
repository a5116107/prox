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
import { Button } from '@/components/ui/button'
import { type OpsTranslate } from './ops-i18n'
import {
  type OpsRenderableGroup,
  type OpsTruthRowDefinition,
  type QuickAction,
  type SavedOpsContext,
  type ViewMode,
  controlPlaneSourceLabel,
  controlPlaneValue,
  firstDistinctText,
  normalizeText,
  numberValue,
  pickConfiguredField,
  pickEffectiveField,
  pickRuntimeField,
  pickSourceField,
  platformLabel,
  prettifyIdentifier,
  releaseModeLabel,
  renderTruthValueCell,
  sourceOriginLabel,
  sourceStateLabelDetailed,
  stringList,
  truthAlignmentState,
} from './ops-live-foundation'
import {
  OpsDataTable,
  OpsPanel,
  OpsStatusBadge,
  type OpsTone,
} from './ops-shared'
import {
  readOpsSavedBool,
  readOpsSavedList,
  readOpsSavedNumber,
  readOpsSavedText,
} from './ops-snapshots'
import type { OperationsSectionId } from './section-registry'
import {
  type OpsControlPlaneSnapshot,
  type OpsRegistryGroup,
} from './use-ops-registry'

export function accessIdentityLabel(row: Record<string, unknown>) {
  return firstDistinctText([
    row.username,
    row.display_name,
    row.nickname,
    row.community_username,
    row.primary_username,
    row.email,
  ])
}

export function sectionLabel(sectionId: OperationsSectionId, t: OpsTranslate) {
  if (sectionId === 'agent-ops') return t('Group bot management')
  if (sectionId === 'community-ops') return t('Community Operations Manager')
  if (sectionId === 'membership-risk') return t('Membership Risk')
  return t('Operations')
}

export function sectionHeroDescription(
  sectionId: OperationsSectionId,
  saved: SavedOpsContext,
  t: OpsTranslate
) {
  const platform = saved.primaryPlatform
    ? platformLabel(saved.primaryPlatform, t)
    : t('QQ / TG')

  if (sectionId === 'agent-ops') {
    return t(
      'See which groups are connected, what each group can do, and whether rewards have enough available balance.'
    )
  }

  if (sectionId === 'community-ops') {
    return t(
      'Start here by checking community rooms, community-only groups, the {{platform}} main-group unlock path, and the exact prompts users will receive.',
      { platform }
    )
  }

  if (sectionId === 'membership-risk') {
    return t(
      'See why a user can or cannot use the site, including community binding, {{platform}} main-group membership, and temporary exceptions.',
      { platform }
    )
  }

  return t(
    'Start here by checking the current live rules first, then open the editor only when you really need to change the saved configuration.'
  )
}

function buildSavedSiteIdentity(
  defaultValues: Record<string, string | number | boolean>,
  controlPlane: OpsControlPlaneSnapshot | null
): Pick<SavedOpsContext, 'siteId' | 'siteName'> {
  return {
    siteId:
      normalizeText(controlPlaneValue(controlPlane, 'agent', 'site_id')) ||
      readOpsSavedText(defaultValues, 'agent_setting.site_id') ||
      readOpsSavedText(defaultValues, 'site_id'),
    siteName:
      normalizeText(controlPlaneValue(controlPlane, 'agent', 'site_name')) ||
      readOpsSavedText(defaultValues, 'agent_setting.site_name'),
  }
}

function buildSavedAccessContext(
  defaultValues: Record<string, string | number | boolean>,
  controlPlane: OpsControlPlaneSnapshot | null
): Pick<
  SavedOpsContext,
  | 'primaryPlatform'
  | 'primaryGroupIds'
  | 'communityGroupIds'
  | 'communityOnlyGroups'
  | 'fullAccessGroups'
  | 'primaryJoinUrl'
  | 'communityJoinUrl'
  | 'denyMessage'
  | 'upgradeMessage'
  | 'blockTokenCreate'
  | 'blockTokenEnable'
> {
  return {
    primaryPlatform:
      normalizeText(
        controlPlaneValue(controlPlane, 'access_control', 'primary_platform')
      ) ||
      readOpsSavedText(
        defaultValues,
        'access_control_setting.primary_platform'
      ),
    primaryGroupIds: stringList(
      controlPlaneValue(
        controlPlane,
        'access_control',
        'primary_group_ids',
        readOpsSavedList(
          defaultValues,
          'access_control_setting.primary_group_ids'
        )
      )
    ),
    communityGroupIds: stringList(
      controlPlaneValue(
        controlPlane,
        'access_control',
        'community_group_ids',
        readOpsSavedList(
          defaultValues,
          'access_control_setting.community_group_ids'
        )
      )
    ),
    communityOnlyGroups: stringList(
      controlPlaneValue(
        controlPlane,
        'access_control',
        'community_only_groups',
        readOpsSavedList(
          defaultValues,
          'access_control_setting.community_only_groups'
        )
      )
    ),
    fullAccessGroups: stringList(
      controlPlaneValue(
        controlPlane,
        'access_control',
        'full_access_groups',
        readOpsSavedList(
          defaultValues,
          'access_control_setting.full_access_groups'
        )
      )
    ),
    primaryJoinUrl:
      normalizeText(
        controlPlaneValue(controlPlane, 'access_control', 'primary_join_url')
      ) ||
      readOpsSavedText(
        defaultValues,
        'access_control_setting.primary_join_url'
      ),
    communityJoinUrl:
      normalizeText(
        controlPlaneValue(controlPlane, 'access_control', 'community_join_url')
      ) ||
      readOpsSavedText(
        defaultValues,
        'access_control_setting.community_join_url'
      ),
    denyMessage:
      normalizeText(
        controlPlaneValue(controlPlane, 'access_control', 'deny_message')
      ) ||
      readOpsSavedText(defaultValues, 'access_control_setting.deny_message'),
    upgradeMessage:
      normalizeText(
        controlPlaneValue(controlPlane, 'access_control', 'upgrade_message')
      ) ||
      readOpsSavedText(defaultValues, 'access_control_setting.upgrade_message'),
    blockTokenCreate:
      (controlPlaneValue(
        controlPlane,
        'access_control',
        'block_token_create'
      ) as boolean | null) ??
      readOpsSavedBool(
        defaultValues,
        'access_control_setting.block_token_create'
      ),
    blockTokenEnable:
      (controlPlaneValue(
        controlPlane,
        'access_control',
        'block_token_enable'
      ) as boolean | null) ??
      readOpsSavedBool(
        defaultValues,
        'access_control_setting.block_token_enable'
      ),
  }
}

function buildSavedCommunityContext(
  defaultValues: Record<string, string | number | boolean>,
  controlPlane: OpsControlPlaneSnapshot | null
): Pick<
  SavedOpsContext,
  'gateEnabled' | 'gateRoomIds' | 'gateRoomMatchMode' | 'botEnabled'
> {
  const singleRoom = readOpsSavedText(
    defaultValues,
    'community_gate_setting.room_id'
  )
  const roomIds = readOpsSavedList(
    defaultValues,
    'community_gate_setting.room_ids'
  )
  const effectiveRoomIds = stringList(
    controlPlaneValue(controlPlane, 'community_gate', 'room_ids', roomIds)
  )
  return {
    gateEnabled:
      (controlPlaneValue(controlPlane, 'community_gate', 'enabled') as
        | boolean
        | null) ??
      readOpsSavedBool(defaultValues, 'community_gate_setting.enabled'),
    gateRoomIds:
      effectiveRoomIds.length > 0
        ? effectiveRoomIds
        : singleRoom
          ? [singleRoom]
          : [],
    gateRoomMatchMode:
      normalizeText(
        controlPlaneValue(controlPlane, 'community_gate', 'room_match_mode')
      ) ||
      readOpsSavedText(defaultValues, 'community_gate_setting.room_match_mode'),
    botEnabled:
      (controlPlaneValue(controlPlane, 'community_bot', 'enabled') as
        | boolean
        | null) ??
      readOpsSavedBool(defaultValues, 'community_bot_setting.enabled'),
  }
}

function buildSavedMembershipContext(
  defaultValues: Record<string, string | number | boolean>,
  controlPlane: OpsControlPlaneSnapshot | null
): Pick<
  SavedOpsContext,
  | 'membershipEnabled'
  | 'membershipDryRun'
  | 'membershipGraceHours'
  | 'membershipPaidBypass'
> {
  return {
    membershipEnabled:
      (controlPlaneValue(controlPlane, 'membership_risk', 'enabled') as
        | boolean
        | null) ?? readOpsSavedBool(defaultValues, 'membership_risk.enabled'),
    membershipDryRun:
      (controlPlaneValue(controlPlane, 'membership_risk', 'dry_run') as
        | boolean
        | null) ?? readOpsSavedBool(defaultValues, 'membership_risk.dry_run'),
    membershipGraceHours:
      (controlPlaneValue(controlPlane, 'membership_risk', 'grace_hours') as
        | number
        | null) ??
      readOpsSavedNumber(defaultValues, 'membership_risk.grace_hours'),
    membershipPaidBypass:
      (controlPlaneValue(
        controlPlane,
        'membership_risk',
        'paid_bypass_enabled'
      ) as boolean | null) ??
      readOpsSavedBool(defaultValues, 'membership_risk.paid_bypass_enabled'),
  }
}

function buildSavedSourceSummary(
  controlPlane: OpsControlPlaneSnapshot | null,
  t: OpsTranslate
): SavedOpsContext['sourceSummary'] {
  return {
    accessControl: controlPlaneSourceLabel(
      controlPlane,
      'access_control',
      ['primary_group_ids', 'community_only_groups', 'full_access_groups'],
      t
    ),
    communityGate: controlPlaneSourceLabel(
      controlPlane,
      'community_gate',
      ['room_ids', 'room_id', 'room_match_mode'],
      t
    ),
    communityBot: controlPlaneSourceLabel(
      controlPlane,
      'community_bot',
      ['enabled', 'room_id'],
      t
    ),
    membershipRisk: controlPlaneSourceLabel(
      controlPlane,
      'membership_risk',
      ['enabled', 'dry_run', 'grace_hours'],
      t
    ),
  }
}

export function buildSavedContext(
  defaultValues: Record<string, string | number | boolean>,
  controlPlane: OpsControlPlaneSnapshot | null,
  t: OpsTranslate
): SavedOpsContext {
  return {
    ...buildSavedSiteIdentity(defaultValues, controlPlane),
    ...buildSavedAccessContext(defaultValues, controlPlane),
    ...buildSavedCommunityContext(defaultValues, controlPlane),
    ...buildSavedMembershipContext(defaultValues, controlPlane),
    sourceSummary: buildSavedSourceSummary(controlPlane, t),
  }
}

export function releaseModeBadge(
  releaseMode: string | undefined,
  t: OpsTranslate
) {
  const normalized = String(releaseMode || '').toLowerCase()
  const label = releaseModeLabel(releaseMode, t)
  if (normalized === 'published' || normalized === 'live') {
    return <OpsStatusBadge tone='success'>{label}</OpsStatusBadge>
  }
  if (normalized === 'audit_only' || normalized === 'draft') {
    return (
      <OpsStatusBadge tone='warning'>{label || t('Unknown')}</OpsStatusBadge>
    )
  }
  return <OpsStatusBadge tone='neutral'>{label || t('Unknown')}</OpsStatusBadge>
}

export function quotaDisplay(quota: unknown, usd?: unknown) {
  const numeric = numberValue(quota)
  const usdText = normalizeText(usd)
  if (usdText) return `${numeric.toLocaleString()} / ${usdText}`
  return numeric ? numeric.toLocaleString() : '0'
}

export function degradationTone(raw: unknown): OpsTone {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'ok' || value === 'active') return 'success'
  if (value === 'degraded' || value === 'low' || value === 'budget_exhausted') {
    return 'warning'
  }
  if (value === 'hard_stop' || value === 'exhausted' || value === 'failed') {
    return 'danger'
  }
  if (value === 'paused') return 'info'
  return 'neutral'
}

export function degradationLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'ok') return t('Rewards are funded')
  if (value === 'active') return t('Active')
  if (value === 'degraded') return t('Reward downgrade recommended')
  if (value === 'budget_exhausted') return t('Today budget is exhausted')
  if (value === 'hard_stop') return t('Stop fund-backed rewards')
  if (value === 'exhausted') return t('Budget exhausted')
  if (value === 'low') return t('Budget is running low')
  if (value === 'paused') return t('Paused')
  return value ? prettifyIdentifier(value) : t('Unknown')
}

export function claimStatusTone(raw: unknown): OpsTone {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'paid') return 'success'
  if (
    value === 'blocked_membership' ||
    value === 'degraded_fund' ||
    value === 'degraded_budget'
  ) {
    return 'warning'
  }
  if (value === 'failed') return 'danger'
  if (value === 'pending' || value === 'created') return 'info'
  return 'neutral'
}

export function claimStatusLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'paid') return t('Paid')
  if (value === 'blocked_membership') return t('Blocked: membership missing')
  if (value === 'degraded_fund') return t('Waiting: fund is low')
  if (value === 'degraded_budget') return t('Waiting: today budget is low')
  if (value === 'failed') return t('Failed')
  if (value === 'pending') return t('Pending')
  if (value === 'created') return t('Created')
  return value ? prettifyIdentifier(value) : t('Unknown')
}

export function inviteEventLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'verify_claim') return t('Invitee verified')
  if (value === 'link_created') return t('Invite link created')
  if (value === 'invitee_joined') return t('Invitee joined')
  if (value === 'membership_verified') return t('Membership verified')
  if (value === 'reward_claimed') return t('Reward claim created')
  return value ? prettifyIdentifier(value) : '—'
}

export function sourceTypeLabel(raw: unknown, t: OpsTranslate) {
  const value = normalizeText(raw).toLowerCase()
  if (value === 'admin_budget_restore')
    return t('Administrator restored available rewards')
  if (value === 'daily_fund_reset')
    return t('Automatic daily operating fund replenishment')
  if (value === 'fund_topup') return t('Manual fund top-up')
  if (value === 'game_stake') return t('Game stake income')
  if (value === 'game_payout') return t('Game payout')
  if (value === 'game_commission') return t('Game commission')
  if (value === 'invite_reward' || value === 'invite_inviter_reward') {
    return t('Invite reward to inviter')
  }
  if (value === 'invite_invitee_reward') return t('Invite reward to invitee')
  return value ? prettifyIdentifier(value) : '—'
}

export function ControlPlaneTruthTable({
  title,
  description,
  rows,
  controlPlane,
  t,
}: {
  title: string
  description: string
  rows: OpsTruthRowDefinition[]
  controlPlane: OpsControlPlaneSnapshot | null
  t: OpsTranslate
}) {
  return (
    <OpsDataTable
      title={title}
      description={description}
      columns={[
        { key: 'field', label: t('Field') },
        { key: 'configured', label: t('Saved configuration') },
        { key: 'effective', label: t('Actually effective') },
        { key: 'runtime', label: t('Runtime observation') },
        { key: 'source', label: t('Why this value is used') },
      ]}
      rows={rows.map((row) => {
        const configured = pickConfiguredField(
          controlPlane,
          row.domain,
          row.fields
        )
        const effective = pickEffectiveField(
          controlPlane,
          row.domain,
          row.fields
        )
        const runtime = pickRuntimeField(controlPlane, row.domain, row.fields)
        const source = pickSourceField(controlPlane, row.domain, row.fields)
        const alignment = truthAlignmentState(configured, effective, runtime, t)

        return {
          id: row.key,
          cells: [
            <div className='space-y-2'>
              <div className='font-medium'>{row.label}</div>
              <div className='text-muted-foreground text-xs leading-5'>
                {row.help}
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <OpsStatusBadge tone={alignment.tone}>
                  {alignment.label}
                </OpsStatusBadge>
              </div>
            </div>,
            <div className='space-y-1 text-sm'>
              <div>
                {renderTruthValueCell(
                  'configured',
                  configured.found,
                  configured.entry?.value,
                  t
                )}
              </div>
            </div>,
            <div className='space-y-1 text-sm'>
              <div>
                {renderTruthValueCell(
                  'effective',
                  effective.found,
                  effective.value,
                  t
                )}
              </div>
              {effective.found ? (
                <div className='text-muted-foreground text-xs'>
                  {t('Saved + registry merge')}
                </div>
              ) : null}
            </div>,
            <div className='space-y-1 text-sm'>
              <div>
                {renderTruthValueCell(
                  'runtime',
                  runtime.found,
                  runtime.value,
                  t
                )}
              </div>
              {runtime.found ? (
                <div className='text-muted-foreground text-xs'>
                  {t('Live runtime summary')}
                </div>
              ) : null}
            </div>,
            <div className='space-y-1 text-sm'>
              <div>{sourceStateLabelDetailed(source.entry, t)}</div>
              {sourceOriginLabel(source.entry) ? (
                <div className='text-muted-foreground text-xs'>
                  {sourceOriginLabel(source.entry)}
                </div>
              ) : null}
            </div>,
          ],
        }
      })}
      emptyMessage={t(
        'The page is loading the configured, effective, and runtime values together.'
      )}
    />
  )
}

export function inviteTargetSummary(
  group: OpsRenderableGroup,
  groups: OpsRegistryGroup[],
  t: OpsTranslate
) {
  const targetGroupId = normalizeText(group.invite_target_group_id)
  if (!targetGroupId) {
    return {
      primary: t('No invite target'),
      secondary: t(
        'This group currently ends in-place and does not push the user into another room or main group.'
      ),
    }
  }

  const matchedTarget =
    groups.find(
      (candidate) => normalizeText(candidate.group_id) === targetGroupId
    ) || null

  return {
    primary: matchedTarget?.group_name || targetGroupId,
    secondary: matchedTarget
      ? `${platformLabel(matchedTarget.platform_family || matchedTarget.platform, t)} · ${matchedTarget.group_id}`
      : t('Target group is not visible in the current live registry yet.'),
  }
}

export function scrollToOpsAnchor(id: string) {
  if (typeof window === 'undefined') return
  const element = window.document.getElementById(id)
  if (!element) return
  element.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.history.replaceState(
    null,
    '',
    `${window.location.pathname}${window.location.search}#${id}`
  )
}

export function buildSectionActions(
  sectionId: OperationsSectionId,
  t: OpsTranslate
): QuickAction[] {
  if (sectionId === 'agent-ops') {
    return [
      {
        id: 'quick-edit-groups',
        label: t('Edit groups'),
        href: '#ops-group-registry-editor',
        tone: 'success',
      },
      {
        id: 'quick-edit-game-capabilities',
        label: t('Edit game capabilities'),
        href: '#ops-group-capability-editor',
        tone: 'info',
      },
      {
        id: 'quick-open-game-admin',
        label: t('Open game admin'),
        href: '/game-admin',
        tone: 'warning',
      },
      {
        id: 'open-community-ops',
        label: t('Go to community funnel'),
        href: '/system-settings/operations/community-ops',
        tone: 'neutral',
      },
      {
        id: 'open-membership-risk',
        label: t('Go to permission decisions'),
        href: '/system-settings/operations/membership-risk',
        tone: 'neutral',
      },
    ]
  }
  if (sectionId === 'community-ops') {
    return [
      {
        id: 'open-access-control',
        label: t('Go to unlock rules'),
        href: '/system-settings/operations/community-ops#community-access-control',
        tone: 'neutral',
      },
      {
        id: 'open-community-bot',
        label: t('Go to community bot'),
        href: '/system-settings/operations/community-ops#community-bot',
        tone: 'neutral',
      },
    ]
  }
  if (sectionId === 'membership-risk') {
    return [
      {
        id: 'open-community-ops',
        label: t('Go to community funnel'),
        href: '/system-settings/operations/community-ops',
        tone: 'neutral',
      },
      {
        id: 'open-agent-ops',
        label: t('Go to group capabilities'),
        href: '/system-settings/operations/agent-ops',
        tone: 'neutral',
      },
    ]
  }
  return []
}

export function EditorContainer({
  mode,
  onSwitch,
  children,
  t,
}: {
  mode: ViewMode
  onSwitch: (mode: ViewMode) => void
  children: ReactNode
  t: OpsTranslate
}) {
  return (
    <OpsPanel
      title={t('Edit settings')}
      description={t(
        'Change the settings below, then save. The status page will refresh after the change.'
      )}
    >
      <div className='flex flex-col gap-4'>
        <div className='border-border/70 bg-muted/20 flex flex-wrap items-center justify-between gap-3 rounded-2xl border p-3'>
          <div className='text-muted-foreground text-sm leading-6'>
            {t('Changes here affect the current prox site after saving.')}
          </div>
          <Button
            type='button'
            size='sm'
            variant='outline'
            onClick={() => onSwitch('overview')}
          >
            {t('Back to status')}
          </Button>
        </div>
        {mode === 'editor' ? (
          <div className='border-border/70 bg-background/70 rounded-[18px] border p-1'>
            {children}
          </div>
        ) : (
          <div className='border-border/70 bg-muted/20 text-muted-foreground rounded-[18px] border border-dashed p-5 text-sm leading-6'>
            {t(
              'Open Edit settings when you need to change a field or run a manual action.'
            )}
          </div>
        )}
      </div>
    </OpsPanel>
  )
}
