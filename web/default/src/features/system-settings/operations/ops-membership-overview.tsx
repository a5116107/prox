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
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { type OpsTranslate } from './ops-i18n'
import {
  type OpsTruthRowDefinition,
  type SavedOpsContext,
  UnifiedAuditTimeline,
  accessLevelLabel,
  accessStateExplanation,
  auditEventsByDomain,
  auditRawRows,
  bindingLabel,
  boolValue,
  displayText,
  firstDistinctText,
  formatGroupList,
  formatTime,
  normalizeText,
  overrideModeLabel,
  reasonCodeLabel,
  reasonMessageLabel,
  recordValue,
  statusBadge,
  stringList,
} from './ops-live-foundation'
import {
  ControlPlaneTruthTable,
  accessIdentityLabel,
  releaseModeBadge,
} from './ops-live-truth'
import { RewardFundInviteOverview } from './ops-reward-overview'
import {
  OpsDataTable,
  OpsInsightCard,
  OpsPanel,
  OpsSurfaceGrid,
} from './ops-shared'
import {
  type OpsAccessExplainResult,
  type OpsAuditOverview,
  type OpsCommunityGateOverview,
  type OpsControlPlaneSnapshot,
  type OpsInviteJourneyOverview,
  type OpsReleaseOverview,
  type OpsRewardFundOverview,
} from './use-ops-registry'

type AccessExplainController = {
  data: OpsAccessExplainResult | null
  loading: boolean
  error: Error | null
  run: (payload: {
    userId: number
    requestedGroup?: string
    refresh?: boolean
  }) => Promise<OpsAccessExplainResult | null>
  reset: () => void
}

type MembershipOverviewProps = {
  communityGate: OpsCommunityGateOverview | null
  audits: OpsAuditOverview | null
  releases: OpsReleaseOverview | null
  controlPlane: OpsControlPlaneSnapshot | null
  saved: SavedOpsContext
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  accessExplain: AccessExplainController
  t: OpsTranslate
}

type MembershipAuditRow = ReturnType<typeof auditRawRows>[number]
type ExplainCandidate = {
  userId: string
  accessLevel: string
  label: string
}

function buildMembershipMetrics(
  accessRows: MembershipAuditRow[],
  gateRows: MembershipAuditRow[],
  liveGateAuditCount: number
) {
  const countRows = (predicate: (row: MembershipAuditRow) => boolean) =>
    accessRows.filter(predicate).length
  const topAccessLevels = Array.from(
    accessRows.reduce((counts, row) => {
      const key = String(row.access_level || 'unknown')
      counts.set(key, (counts.get(key) || 0) + 1)
      return counts
    }, new Map<string, number>())
  )
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4)

  return {
    recentAccessCount: accessRows.length,
    fullAccessCount: countRows(
      (row) => String(row.access_level || '') === 'full_access'
    ),
    communityOnlyCount: countRows(
      (row) => String(row.access_level || '') === 'community_only'
    ),
    manualOverrideCount: countRows(
      (row) => String(row.manual_override_mode || '') !== ''
    ),
    primaryBoundCount: countRows((row) => boolValue(row.primary_bound)),
    gateAuditCount: liveGateAuditCount || gateRows.length,
    topAccessLevels,
  }
}

function buildExplainCandidates(
  accessRows: MembershipAuditRow[]
): ExplainCandidate[] {
  const seen = new Set<string>()
  return accessRows
    .map((row) => {
      const userId = normalizeText(row.user_id)
      if (!userId || seen.has(userId)) return null
      seen.add(userId)
      return {
        userId,
        accessLevel: String(row.access_level || ''),
        label:
          firstDistinctText([
            accessIdentityLabel(row as Record<string, unknown>),
            `UID ${userId}`,
          ]) || `UID ${userId}`,
      }
    })
    .filter(Boolean)
    .slice(0, 6) as ExplainCandidate[]
}

function buildMembershipTruthRows(t: OpsTranslate): OpsTruthRowDefinition[] {
  return [
    {
      key: 'membership-enabled',
      label: t('Saved risk switch'),
      help: t(
        'These are the configured values currently loaded from system settings. Edit them in the advanced configuration below.'
      ),
      domain: 'membership_risk',
      fields: ['enabled'],
    },
    {
      key: 'membership-dry-run',
      label: t('Saved execution mode'),
      help: t(
        'The current live mode is dry-run, so runtime rows should explain what would happen before hard enforcement.'
      ),
      domain: 'membership_risk',
      fields: ['dry_run'],
    },
    {
      key: 'membership-grace-hours',
      label: t('Saved grace window'),
      help: t(
        'How many hours a user can stay outside the required groups before enforcement takes effect.'
      ),
      domain: 'membership_risk',
      fields: ['grace_hours'],
    },
    {
      key: 'membership-paid-bypass',
      label: t('Saved paid bypass'),
      help: t('Paid users keep the highest-priority bypass lane.'),
      domain: 'membership_risk',
      fields: ['paid_bypass_enabled'],
    },
    {
      key: 'membership-main-groups',
      label: t('Saved main groups'),
      help: t(
        'Loaded from access-control settings that decide which primary QQ or TG groups unlock broader access.'
      ),
      domain: 'access_control',
      fields: ['primary_group_ids'],
    },
    {
      key: 'membership-community-only',
      label: t('Saved community-only groups'),
      help: t(
        'Saved fallback groups that a user can use after community binding but before joining the main field.'
      ),
      domain: 'access_control',
      fields: ['community_only_groups'],
    },
  ]
}

function membershipSwitchSummary(saved: SavedOpsContext, t: OpsTranslate) {
  if (saved.membershipEnabled === true) {
    return t('Membership risk is enabled for this site.')
  }
  if (saved.membershipEnabled === false) {
    return t('Membership risk is currently disabled for this site.')
  }
  return t(
    'Membership risk switch is not visible in the current live policy yet.'
  )
}

function membershipModeSummary(saved: SavedOpsContext, t: OpsTranslate) {
  if (saved.membershipDryRun === true) {
    return t(
      'The current live mode is dry-run, so runtime rows should explain what would happen before hard enforcement.'
    )
  }
  if (saved.membershipDryRun === false) {
    return t(
      'The current live mode is hard enforcement, so denied states should really block free or community-only paths.'
    )
  }
  return t(
    'Membership-risk mode is not visible in the current live policy yet.'
  )
}

function membershipPaidBypassSummary(saved: SavedOpsContext, t: OpsTranslate) {
  if (saved.membershipPaidBypass === true) {
    return t('Paid users keep the highest-priority bypass lane.')
  }
  if (saved.membershipPaidBypass === false) {
    return t('Paid-user bypass is turned off in the current live policy.')
  }
  return t(
    'Paid-user bypass rule is not visible in the current live policy yet.'
  )
}

function buildMembershipRuleLines(saved: SavedOpsContext, t: OpsTranslate) {
  return [
    membershipSwitchSummary(saved, t),
    membershipModeSummary(saved, t),
    saved.membershipGraceHours !== null
      ? `${t('Grace window')}: ${saved.membershipGraceHours}h`
      : t('Grace window is not visible in the current live policy yet.'),
    membershipPaidBypassSummary(saved, t),
  ]
}

function MembershipInsightGrid({
  metrics,
  releases,
  t,
}: {
  metrics: ReturnType<typeof buildMembershipMetrics>
  releases: OpsReleaseOverview | null
  t: OpsTranslate
}) {
  return (
    <OpsSurfaceGrid className='xl:grid-cols-4'>
      <OpsInsightCard
        title={t('Recent access rows')}
        value={metrics.recentAccessCount || '0'}
        description={t(
          'Latest real access-control evaluation rows for this site.'
        )}
        badge={releaseModeBadge(releases?.release_mode, t)}
      />
      <OpsInsightCard
        title={t('Full-access users')}
        value={metrics.fullAccessCount || '0'}
        description={t('Users currently evaluated into the full-access lane.')}
      />
      <OpsInsightCard
        title={t('Community-only users')}
        value={metrics.communityOnlyCount || '0'}
        description={t('Users currently limited to community-only API groups.')}
      />
      <OpsInsightCard
        title={t('Manual overrides')}
        value={metrics.manualOverrideCount || '0'}
        description={t('Rows that still carry a manual override lane.')}
      />
      <OpsInsightCard
        title={t('Live gate audits')}
        value={metrics.gateAuditCount || '0'}
        description={t(
          'Recent live community-gate audits available to membership operators.'
        )}
      />
    </OpsSurfaceGrid>
  )
}

function MembershipPolicyPanels({
  saved,
  ruleLines,
  t,
}: {
  saved: SavedOpsContext
  ruleLines: string[]
  t: OpsTranslate
}) {
  return (
    <OpsSurfaceGrid className='xl:grid-cols-[minmax(0,1.02fr)_minmax(320px,.98fr)] xl:items-start'>
      <OpsPanel
        title={t('Current live policy summary')}
        description={t(
          'This converts the current live membership-risk policy into plain-language rules so admins can explain the system without reading raw boolean fields.'
        )}
      >
        <div className='space-y-3'>
          {ruleLines.map((line, index) => (
            <div
              key={index}
              className='border-border/70 bg-background/70 text-muted-foreground rounded-[18px] border p-4 text-sm leading-6'
            >
              {line}
            </div>
          ))}
        </div>
      </OpsPanel>
      <OpsPanel
        title={t('What users will be told')}
        description={t(
          'The current live prompts determine what blocked users see when they are missing community binding, main-field binding, or access eligibility.'
        )}
      >
        <div className='grid gap-3'>
          <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
            <div className='text-sm font-semibold'>{t('Deny prompt')}</div>
            <div className='text-muted-foreground mt-2 text-sm leading-6'>
              {saved.denyMessage || t('Not saved')}
            </div>
          </div>
          <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
            <div className='text-sm font-semibold'>{t('Upgrade prompt')}</div>
            <div className='text-muted-foreground mt-2 text-sm leading-6'>
              {saved.upgradeMessage || t('Not saved')}
            </div>
          </div>
          <div className='border-border/70 bg-background/70 text-muted-foreground rounded-[18px] border p-4 text-sm leading-6'>
            {saved.primaryJoinUrl
              ? `${t('Primary-field join link')}: ${saved.primaryJoinUrl}`
              : t(
                  'Primary-field join link is not visible in the current live policy yet.'
                )}
          </div>
        </div>
      </OpsPanel>
    </OpsSurfaceGrid>
  )
}

function useMembershipAccessExplainer({
  accessRows,
  accessExplain,
  t,
}: {
  accessRows: MembershipAuditRow[]
  accessExplain: AccessExplainController
  t: OpsTranslate
}) {
  const explainCandidates = useMemo(
    () => buildExplainCandidates(accessRows),
    [accessRows]
  )
  const [selectedUserId, setSelectedUserId] = useState('')
  const effectiveSelectedUserId =
    selectedUserId || explainCandidates[0]?.userId || ''
  const [requestedGroup, setRequestedGroup] = useState('')

  const runExplain = async (
    userIdInput: string = effectiveSelectedUserId,
    requestedGroupInput: string = requestedGroup
  ) => {
    const numericUserId = Number(userIdInput)
    if (!Number.isFinite(numericUserId) || numericUserId <= 0) {
      toast.error(t('Enter a valid user ID first.'))
      return
    }
    try {
      await accessExplain.run({
        userId: numericUserId,
        requestedGroup: normalizeText(requestedGroupInput),
        refresh: true,
      })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to explain this user right now.')
      )
    }
  }

  return {
    explainCandidates,
    selectedUserId: effectiveSelectedUserId,
    setSelectedUserId,
    requestedGroup,
    setRequestedGroup,
    explainResult: accessExplain.data,
    explainStatus: recordValue(accessExplain.data?.status),
    explainSource: recordValue(accessExplain.data?.requested_group_source),
    runExplain,
  }
}

type MembershipExplainerState = ReturnType<typeof useMembershipAccessExplainer>

function AccessExplainControls({
  state,
  accessExplain,
  t,
}: {
  state: MembershipExplainerState
  accessExplain: AccessExplainController
  t: OpsTranslate
}) {
  return (
    <>
      <div className='grid gap-3 md:grid-cols-[180px_minmax(0,1fr)_auto]'>
        <Input
          value={state.selectedUserId}
          onChange={(event) => state.setSelectedUserId(event.target.value)}
          placeholder={t('User ID')}
        />
        <Input
          value={state.requestedGroup}
          onChange={(event) => state.setRequestedGroup(event.target.value)}
          placeholder={t('Requested group (optional)')}
        />
        <Button
          type='button'
          onClick={() => void state.runExplain()}
          disabled={accessExplain.loading}
        >
          {accessExplain.loading ? t('Explaining') : t('Explain this user')}
        </Button>
      </div>
      {state.explainCandidates.length > 0 ? (
        <div className='space-y-2'>
          <div className='text-muted-foreground text-xs font-semibold tracking-[0.18em] uppercase'>
            {t('Recent user shortcuts')}
          </div>
          <div className='flex flex-wrap gap-2'>
            {state.explainCandidates.map((candidate) => (
              <Button
                key={candidate.userId}
                type='button'
                variant={
                  state.selectedUserId === candidate.userId
                    ? 'default'
                    : 'outline'
                }
                size='sm'
                onClick={() => {
                  state.setSelectedUserId(candidate.userId)
                  void state.runExplain(candidate.userId, state.requestedGroup)
                }}
              >
                {candidate.label} · {accessLevelLabel(candidate.accessLevel, t)}
              </Button>
            ))}
          </div>
        </div>
      ) : null}
      {accessExplain.error ? (
        <div className='border-destructive/30 bg-destructive/10 text-destructive rounded-[18px] border p-4 text-sm leading-6'>
          {accessExplain.error.message}
        </div>
      ) : null}
    </>
  )
}

function AccessExplainResultSummary({
  result,
  t,
}: {
  result: OpsAccessExplainResult | null
  t: OpsTranslate
}) {
  if (!result) {
    return (
      <div className='border-border/70 bg-background/50 text-muted-foreground rounded-[18px] border border-dashed p-4 text-sm leading-6'>
        {t(
          'Pick a recent user or type a user ID to load a live access explanation.'
        )}
      </div>
    )
  }

  const nextSteps = stringList(result.next_steps)
  return (
    <div className='space-y-4'>
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>{t('User')}</div>
          <div className='mt-2 text-sm leading-6'>
            {displayText(result.user?.username)}
          </div>
          <div className='text-muted-foreground text-xs'>
            UID {displayText(result.user?.id)}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>{t('Decision')}</div>
          <div className='mt-2'>
            {statusBadge(
              boolValue(result.allowed),
              t('Allowed'),
              t('Denied'),
              t('Unknown')
            )}
          </div>
          <div className='text-muted-foreground mt-2 text-xs'>
            {reasonCodeLabel(result.reason_code, t)}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>{t('Requested group')}</div>
          <div className='mt-2 text-sm leading-6'>
            {displayText(result.requested_group)}
          </div>
          <div className='text-muted-foreground text-xs'>
            {result.requested_group
              ? t('This group was checked against the current live policy.')
              : t(
                  'Requested group is not provided. The system explains the full current lane instead.'
                )}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>{t('Effective groups')}</div>
          <div className='mt-2 text-sm leading-6'>
            {formatGroupList(result.effective_groups)}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Base groups')}: {formatGroupList(result.base_groups)}
          </div>
        </div>
      </div>
      <div className='border-border/70 bg-background/70 rounded-[20px] border p-5'>
        <div className='text-sm font-semibold'>{t('Human message')}</div>
        <div className='text-muted-foreground mt-2 text-sm leading-6'>
          {displayText(result.human_message)}
        </div>
        <div className='text-muted-foreground mt-3 text-xs'>
          {reasonMessageLabel(result.reason_message, t)}
        </div>
      </div>
      <div className='border-border/70 bg-background/70 rounded-[20px] border p-5'>
        <div className='text-sm font-semibold'>{t('Next steps')}</div>
        <div className='text-muted-foreground mt-3 space-y-2 text-sm leading-6'>
          {nextSteps.length > 0 ? (
            nextSteps.map((step, index) => (
              <div key={`${step}-${index}`}>
                0{index + 1} {step}
              </div>
            ))
          ) : (
            <div>{t('No next steps are required right now.')}</div>
          )}
        </div>
      </div>
    </div>
  )
}

function AccessExplainPanel({
  state,
  accessExplain,
  t,
}: {
  state: MembershipExplainerState
  accessExplain: AccessExplainController
  t: OpsTranslate
}) {
  return (
    <OpsPanel
      title={t('Access explainer')}
      description={t(
        'Explain one live user and copy the exact reason, current lane, and next steps without leaving this page.'
      )}
    >
      <div className='space-y-4'>
        <AccessExplainControls
          state={state}
          accessExplain={accessExplain}
          t={t}
        />
        <AccessExplainResultSummary result={state.explainResult} t={t} />
      </div>
    </OpsPanel>
  )
}

function AccessExplainDecisionPanel({
  state,
  t,
}: {
  state: MembershipExplainerState
  t: OpsTranslate
}) {
  const { explainResult, explainStatus, explainSource } = state
  return (
    <OpsPanel
      title={t('How the last explanation was decided')}
      description={t(
        'This panel shows which live bindings matched, whether the requested group belongs to the base package, community lane, full-access lane, or paid bypass lane, and which reason code actually fired.'
      )}
    >
      {explainResult ? (
        <div className='grid gap-3'>
          <div className='border-border/70 bg-background/70 rounded-[18px] border p-4 text-sm'>
            <div className='font-semibold'>{t('Live binding snapshot')}</div>
            <div className='text-muted-foreground mt-3 space-y-2'>
              <div>
                {t('Community')}:{' '}
                {bindingLabel(explainStatus.community_bound, t)}
              </div>
              <div>
                {t('Primary')}: {bindingLabel(explainStatus.primary_bound, t)}
              </div>
              <div>
                {t('OAuth')}: {bindingLabel(explainStatus.has_oauth_binding, t)}
              </div>
              <div>
                {t('Access')}: {accessLevelLabel(explainStatus.access_level, t)}
              </div>
              <div>
                {t('Main-field group')}:{' '}
                {displayText(explainStatus.matched_primary_group_id)}
              </div>
              <div>
                {t('Manual overrides')}:{' '}
                {overrideModeLabel(explainStatus.manual_override_mode, t)}
              </div>
            </div>
          </div>
          <div className='border-border/70 bg-background/70 rounded-[18px] border p-4 text-sm'>
            <div className='font-semibold'>{t('Requested group source')}</div>
            <div className='text-muted-foreground mt-3 space-y-2'>
              <div>
                {t('In base package')}:{' '}
                {boolValue(explainSource.in_user_base_group)
                  ? t('Included')
                  : t('Not included')}
              </div>
              <div>
                {t('Community lane')}:{' '}
                {boolValue(explainSource.in_community_only_groups)
                  ? t('Included')
                  : t('Not included')}
              </div>
              <div>
                {t('Full-access lane')}:{' '}
                {boolValue(explainSource.in_full_access_groups)
                  ? t('Included')
                  : t('Not included')}
              </div>
              <div>
                {t('Paid bypass lane')}:{' '}
                {boolValue(explainSource.in_paid_bypass_groups)
                  ? t('Included')
                  : t('Not included')}
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className='border-border/70 bg-background/50 text-muted-foreground rounded-[18px] border border-dashed p-4 text-sm leading-6'>
          {t(
            'Run the access explainer once and this panel will show the live decision path.'
          )}
        </div>
      )}
    </OpsPanel>
  )
}

function MembershipRuntimeShape({
  metrics,
  t,
}: {
  metrics: ReturnType<typeof buildMembershipMetrics>
  t: OpsTranslate
}) {
  return (
    <OpsPanel
      title={t('Current runtime access shape')}
      description={t(
        'These counts come from the latest access-state rows and are useful for spotting whether the current live policy is actually producing the intended lanes.'
      )}
    >
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>
            {t('Primary-bound users')}
          </div>
          <div className='mt-2 text-2xl font-semibold tracking-tight'>
            {metrics.primaryBoundCount}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>{t('Recent gate audits')}</div>
          <div className='mt-2 text-2xl font-semibold tracking-tight'>
            {metrics.gateAuditCount}
          </div>
        </div>
        {metrics.topAccessLevels.map(([label, count]) => (
          <div
            key={label}
            className='border-border/70 bg-background/70 rounded-[18px] border p-4'
          >
            <div className='text-sm font-semibold'>
              {accessLevelLabel(label, t)}
            </div>
            <div className='mt-2 text-2xl font-semibold tracking-tight'>
              {count}
            </div>
          </div>
        ))}
      </div>
    </OpsPanel>
  )
}

function MembershipEvidenceTable({
  accessRows,
  state,
  t,
}: {
  accessRows: MembershipAuditRow[]
  state: MembershipExplainerState
  t: OpsTranslate
}) {
  return (
    <OpsDataTable
      title={t('Recent access-state evidence')}
      description={t(
        'These rows show the latest evaluated access state, which bindings matched, and whether manual overrides or premium lanes are still active.'
      )}
      columns={[
        { key: 'user', label: t('User') },
        { key: 'access', label: t('Access') },
        { key: 'bindings', label: t('Bindings') },
        { key: 'groups', label: t('Effective groups') },
        { key: 'action', label: t('Explain') },
        { key: 'time', label: t('Time') },
      ]}
      rows={accessRows.slice(0, 8).map((row, index) => ({
        id: `${row.id || index}`,
        cells: [
          <div className='space-y-1'>
            <div className='font-medium'>UID {String(row.user_id || '—')}</div>
            {accessIdentityLabel(row as Record<string, unknown>) ? (
              <div className='text-muted-foreground text-xs'>
                {accessIdentityLabel(row as Record<string, unknown>)}
              </div>
            ) : null}
          </div>,
          <div className='space-y-1'>
            <div>{accessLevelLabel(row.access_level, t)}</div>
            {accessStateExplanation(row as Record<string, unknown>, t) ? (
              <div className='text-muted-foreground text-xs'>
                {accessStateExplanation(row as Record<string, unknown>, t)}
              </div>
            ) : null}
          </div>,
          <div className='space-y-1 text-sm'>
            <div>
              {t('Community')}: {bindingLabel(row.community_bound, t)}
            </div>
            <div>
              {t('Primary')}: {bindingLabel(row.primary_bound, t)}
            </div>
            <div>
              {t('OAuth')}: {bindingLabel(row.has_oauth_binding, t)}
            </div>
          </div>,
          <div className='space-y-1 text-sm'>
            <div>{overrideModeLabel(row.manual_override_mode, t)}</div>
            <div className='text-muted-foreground text-xs'>
              {formatGroupList(row.effective_groups)}
            </div>
          </div>,
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => {
              const userId = normalizeText(row.user_id)
              if (!userId) return
              state.setSelectedUserId(userId)
              void state.runExplain(userId, state.requestedGroup)
            }}
          >
            {t('Explain')}
          </Button>,
          formatTime(row.updated_at || row.last_evaluated_at),
        ],
      }))}
      emptyMessage={t('No recent access-state evidence yet.')}
    />
  )
}

function MembershipAccessWorkspace({
  accessRows,
  accessExplain,
  metrics,
  t,
}: {
  accessRows: MembershipAuditRow[]
  accessExplain: AccessExplainController
  metrics: ReturnType<typeof buildMembershipMetrics>
  t: OpsTranslate
}) {
  const state = useMembershipAccessExplainer({ accessRows, accessExplain, t })
  return (
    <>
      <OpsSurfaceGrid className='xl:grid-cols-[minmax(0,1.04fr)_minmax(320px,.96fr)] xl:items-start'>
        <AccessExplainPanel state={state} accessExplain={accessExplain} t={t} />
        <AccessExplainDecisionPanel state={state} t={t} />
      </OpsSurfaceGrid>
      <MembershipRuntimeShape metrics={metrics} t={t} />
      <MembershipEvidenceTable accessRows={accessRows} state={state} t={t} />
    </>
  )
}

export function MembershipOverview({
  communityGate,
  audits,
  releases,
  controlPlane,
  saved,
  rewardFund,
  inviteJourney,
  accessExplain,
  t,
}: MembershipOverviewProps) {
  const accessRows = useMemo(
    () => auditRawRows(audits, 'access_control'),
    [audits]
  )
  const gateRows = auditRawRows(audits, 'community_gate')
  const metrics = buildMembershipMetrics(
    accessRows,
    gateRows,
    communityGate?.recent_audits?.length || 0
  )
  const membershipAuditEvents = auditEventsByDomain(audits, [
    'access_control',
    'community_gate',
    'risk_control',
  ])

  return (
    <div className='space-y-6'>
      <MembershipInsightGrid metrics={metrics} releases={releases} t={t} />
      <RewardFundInviteOverview
        rewardFund={rewardFund}
        inviteJourney={inviteJourney}
        siteId={normalizeText(controlPlane?.site_id) || saved.siteId}
        t={t}
      />
      <ControlPlaneTruthTable
        title={t('Saved policy vs live runtime')}
        description={t(
          'Configured values explain what the site should do; runtime counters explain what is happening now. They are shown separately here so the page stops looking contradictory.'
        )}
        rows={buildMembershipTruthRows(t)}
        controlPlane={controlPlane}
        t={t}
      />
      <MembershipPolicyPanels
        saved={saved}
        ruleLines={buildMembershipRuleLines(saved, t)}
        t={t}
      />
      <MembershipAccessWorkspace
        accessRows={accessRows}
        accessExplain={accessExplain}
        metrics={metrics}
        t={t}
      />
      <UnifiedAuditTimeline
        title={t('Unified audit timeline')}
        description={t(
          'Membership decisions, community gate checks, and risk findings now share one ordered stream, so freeze, deny, restore, and manual-override stories can be explained from one place.'
        )}
        events={membershipAuditEvents}
        t={t}
        emptyMessage={t('No recent membership audit events yet.')}
      />
    </div>
  )
}
