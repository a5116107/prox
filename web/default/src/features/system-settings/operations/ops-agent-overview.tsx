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
import { AgentConfigurationEntryPoints } from './ops-agent-controls'
import { type OpsTranslate } from './ops-i18n'
import {
  type OpsRenderableGroup,
  type OpsTruthRowDefinition,
  type SavedOpsContext,
  UnifiedAuditTimeline,
  accessQualifiersOf,
  auditEventsByDomain,
  boolValue,
  capabilityLabel,
  capabilityPolicyOf,
  countEnabledGameRules,
  gamePolicyOf,
  groupRoleLabel,
  groupStatusLabel,
  latestMetricsOf,
  numberValue,
  platformLabel,
  recordValue,
  resolveCapabilityRows,
  rewardPolicyOf,
  runtimeConnectorsOf,
  statusBadge,
  stringList,
} from './ops-live-foundation'
import {
  ControlPlaneTruthTable,
  inviteTargetSummary,
  releaseModeBadge,
} from './ops-live-truth'
import { RewardFundInviteOverview } from './ops-reward-overview'
import { OpsDataTable, OpsPanel, OpsSurfaceGrid } from './ops-shared'
import {
  type OpsAuditOverview,
  type OpsControlPlaneSnapshot,
  type OpsGroupCapabilityMatrixOverview,
  type OpsInviteJourneyOverview,
  type OpsRegistryGroup,
  type OpsRewardFundOverview,
  type OpsReleaseOverview,
} from './use-ops-registry'

function buildAgentOverviewMetrics(
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null,
  groups: OpsRegistryGroup[]
) {
  const capabilityRows = resolveCapabilityRows(groupCapabilities, groups)
  const summary = recordValue(groupCapabilities?.summary)
  return {
    capabilityRows,
    summary,
    connectorHits: capabilityRows.filter((group) =>
      boolValue(runtimeConnectorsOf(group).matches_runtime_connector)
    ).length,
    checkinEnabled:
      numberValue(summary.checkin_enabled_groups) ||
      capabilityRows.filter((group) =>
        boolValue(capabilityPolicyOf(group).checkin_enabled)
      ).length,
    verifyEnabled:
      numberValue(summary.verify_enabled_groups) ||
      capabilityRows.filter((group) =>
        boolValue(capabilityPolicyOf(group).verify_enabled)
      ).length,
    inviteEnabled:
      numberValue(summary.invite_enabled_groups) ||
      capabilityRows.filter((group) =>
        boolValue(capabilityPolicyOf(group).invite_enabled)
      ).length,
    enabledGameRules: countEnabledGameRules(groupCapabilities, groups),
    budgetPools: stringList(summary.budget_pools),
    commissionTotal: capabilityRows.reduce(
      (sum, group) =>
        sum + numberValue(latestMetricsOf(group).commission_quota),
      0
    ),
  }
}

type AgentOverviewMetrics = ReturnType<typeof buildAgentOverviewMetrics>

function buildAgentTruthRows(
  primaryPlatformText: string,
  t: OpsTranslate
): OpsTruthRowDefinition[] {
  return [
    {
      key: 'site-id',
      label: t('Saved site ID'),
      help: t('Saved site identity'),
      domain: 'agent',
      fields: ['site_id'],
    },
    {
      key: 'primary-platform',
      label: t('Which platform is the main unlock path'),
      help: t(
        'Check whether the current site routes users through QQ or TG first, then verify the linked group IDs and runtime channel health.'
      ),
      domain: 'access_control',
      fields: ['primary_platform'],
    },
    {
      key: 'main-groups',
      label: t('Saved main groups'),
      help: t(
        'Loaded from access-control settings that decide which primary QQ or TG groups unlock broader access.'
      ),
      domain: 'access_control',
      fields: ['primary_group_ids'],
    },
    {
      key: 'community-only-groups',
      label: t('Saved community-only groups'),
      help: t(
        'Saved fallback groups that a user can use after community binding but before joining the main field.'
      ),
      domain: 'access_control',
      fields: ['community_only_groups'],
    },
    {
      key: 'full-access-groups',
      label: t('Saved full-access groups'),
      help: t('{{platform}} groups that unlock the full site after binding.', {
        platform: primaryPlatformText,
      }),
      domain: 'access_control',
      fields: ['full_access_groups'],
    },
    {
      key: 'block-token-create',
      label: t('Saved create-key gate'),
      help: t(
        'Use the current access-control and community-gate settings as the source of truth for who can create keys, which groups unlock after each binding step, and when rewards must pause.'
      ),
      domain: 'access_control',
      fields: ['block_token_create'],
    },
  ]
}

function AgentSummaryGrid({
  metrics,
  groupCount,
  releases,
  t,
}: {
  metrics: AgentOverviewMetrics
  groupCount: number
  releases: OpsReleaseOverview | null
  t: OpsTranslate
}) {
  const summaryItems = [
    {
      key: 'groups',
      label: t('Groups and rooms'),
      value: numberValue(metrics.summary.total_groups) || groupCount || '0',
      help: t('Groups and rooms currently registered for this site.'),
      badge: releaseModeBadge(releases?.release_mode, t),
    },
    {
      key: 'connected',
      label: t('Groups currently connected'),
      value: metrics.connectorHits || '0',
      help: t('QQ, TG, or community groups the bot can currently reach.'),
    },
    {
      key: 'features',
      label: t('Enabled group features'),
      value: `${metrics.checkinEnabled}/${metrics.verifyEnabled}/${metrics.inviteEnabled}`,
      help:
        metrics.enabledGameRules > 0
          ? `${t('Check-in / verification / invitation enabled groups.')} · ${t('Enabled games')}: ${metrics.enabledGameRules}`
          : t('Check-in / verification / invitation enabled groups.'),
    },
    {
      key: 'rewards',
      label: t('Recently recorded reward amount'),
      value: metrics.commissionTotal.toLocaleString(),
      help:
        metrics.budgetPools.length > 0
          ? `${t('Total from the latest group records.')} · ${t('Reward accounts')}: ${metrics.budgetPools.join(', ')}`
          : t('Total from the latest group records.'),
    },
  ]
  return (
    <div className='bg-card grid overflow-hidden rounded-lg border sm:grid-cols-2 xl:grid-cols-4'>
      {summaryItems.map((item) => (
        <div
          key={item.key}
          className='border-b p-4 sm:border-r xl:border-b-0 xl:last:border-r-0 sm:[&:nth-child(2n)]:border-r-0 xl:[&:nth-child(2n)]:border-r'
        >
          <div className='flex items-start justify-between gap-3'>
            <div className='text-muted-foreground text-sm font-medium'>
              {item.label}
            </div>
            {'badge' in item && item.badge ? <div>{item.badge}</div> : null}
          </div>
          <div className='mt-2 text-2xl font-semibold tabular-nums'>
            {item.value}
          </div>
          <p className='text-muted-foreground mt-1 text-xs leading-5'>
            {item.help}
          </p>
        </div>
      ))}
    </div>
  )
}

function AgentAccessRulesPanel({
  saved,
  t,
}: {
  saved: SavedOpsContext
  t: OpsTranslate
}) {
  return (
    <OpsPanel
      title={t('Current group and access rules')}
      description={t(
        'Review the main channel, community-only groups, full-access groups, and key restrictions in one place.'
      )}
    >
      <div className='grid gap-3 md:grid-cols-2'>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>
            {t('Main channel and groups')}
          </div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.primaryPlatform
              ? `${platformLabel(saved.primaryPlatform, t)} · ${saved.primaryGroupIds.join(', ') || t('No main group IDs configured yet.')}`
              : t('The main channel is not configured yet.')}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>
            {t('Community-only groups')}
          </div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.communityOnlyGroups.length > 0
              ? saved.communityOnlyGroups.join(', ')
              : t('No community-only groups are active yet.')}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>{t('Full-access groups')}</div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.fullAccessGroups.length > 0
              ? saved.fullAccessGroups.join(', ')
              : t('No full-access groups are active yet.')}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>{t('Key restrictions')}</div>
          <div className='mt-2 flex flex-wrap gap-2'>
            {statusBadge(
              saved.blockTokenCreate,
              t('Create key gated'),
              t('Create key open'),
              t('Create-key rule is not visible in the live policy')
            )}
            {statusBadge(
              saved.blockTokenEnable,
              t('Enable key gated'),
              t('Enable key open'),
              t('Enable-key rule is not visible in the live policy')
            )}
          </div>
        </div>
      </div>
    </OpsPanel>
  )
}

function AgentMainGroupSetupPanel({
  capabilityRows,
  saved,
  primaryPlatformText,
  t,
}: {
  capabilityRows: OpsRenderableGroup[]
  saved: SavedOpsContext
  primaryPlatformText: string
  t: OpsTranslate
}) {
  const connectorMatchedGroups = capabilityRows.filter((group) => {
    const family = String(
      group.platform_family || group.platform || ''
    ).toLowerCase()
    return (
      family !== 'community' &&
      boolValue(runtimeConnectorsOf(group).matches_runtime_connector)
    )
  })
  return (
    <OpsPanel
      title={t('Add a main group without missing a step')}
      description={t(
        'When adding a new {{platform}} main group, update these three settings together so the bot connection, access rule, and group features agree.',
        { platform: primaryPlatformText }
      )}
    >
      <div className='grid gap-3'>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>
            {t('Groups currently visible to the bot')}
          </div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {connectorMatchedGroups.length > 0
              ? connectorMatchedGroups
                  .map((group) => group.group_name || group.group_id)
                  .join(', ')
              : t('The bot cannot see any main groups yet.')}
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {t(
              'Only groups the {{platform}} bot can currently reach appear here. After a new group appears, its connection status below should also be online.',
              { platform: primaryPlatformText }
            )}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>{t('Main unlock groups')}</div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.primaryGroupIds.length > 0
              ? saved.primaryGroupIds.join(', ')
              : t('No saved main unlock groups yet.')}
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {t(
              'These are the {{platform}} groups that unlock broader access after the user finishes binding.',
              { platform: primaryPlatformText }
            )}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
          <div className='text-sm font-semibold'>
            {t('Community-only and full-access account groups')}
          </div>
          <div className='text-muted-foreground mt-2 space-y-2 text-sm leading-6'>
            <div>
              {saved.communityOnlyGroups.length > 0
                ? `${t('Community-only groups')}: ${saved.communityOnlyGroups.join(', ')}`
                : t('No community-only groups saved yet.')}
            </div>
            <div>
              {saved.fullAccessGroups.length > 0
                ? `${t('Full-access groups')}: ${saved.fullAccessGroups.join(', ')}`
                : t('No full-access groups saved yet.')}
            </div>
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {t(
              'Keep community-only and full-access groups separate so users do not receive conflicting access rules.'
            )}
          </div>
        </div>
      </div>
    </OpsPanel>
  )
}

function buildAgentCapabilityRow(
  group: OpsRenderableGroup,
  groups: OpsRegistryGroup[],
  t: OpsTranslate
) {
  const capabilityPolicy = capabilityPolicyOf(group)
  const rewardPolicy = rewardPolicyOf(group)
  const gamePolicy = gamePolicyOf(group)
  const latestMetrics = latestMetricsOf(group)
  const accessQualifiers = accessQualifiersOf(group)
  const groupBudgetPools = stringList(
    rewardPolicy.budget_pools || gamePolicy.budget_pools
  )
  const enabledGameCodes = stringList(gamePolicy.enabled_game_codes)
  const inviteTarget = inviteTargetSummary(group, groups, t)
  return {
    id: String(group.id),
    cells: [
      <div className='space-y-1'>
        <div className='font-medium'>{group.group_name || group.group_id}</div>
        <div className='text-muted-foreground text-xs'>
          {platformLabel(group.platform_family || group.platform, t)} ·{' '}
          {group.group_id}
        </div>
      </div>,
      <div className='space-y-2 text-sm'>
        <div>{groupRoleLabel(group.role, t)}</div>
        <div className='text-muted-foreground text-xs'>
          {groupStatusLabel(group.status, group.enabled, t)}
        </div>
        <div className='flex flex-wrap gap-2'>
          {statusBadge(
            boolValue(accessQualifiers.qualifies_community_binding),
            t('Community unlock'),
            t('No community unlock'),
            t('Unknown')
          )}
          {statusBadge(
            boolValue(accessQualifiers.qualifies_primary_binding),
            t('Main-field unlock'),
            t('No main-field unlock'),
            t('Unknown')
          )}
        </div>
      </div>,
      <div className='space-y-1 text-sm'>
        <div>
          {t('Check-in')}:{' '}
          {capabilityLabel(capabilityPolicy.checkin_enabled, t)} ·{' '}
          {t('Check-in quota')}: {numberValue(rewardPolicy.checkin_quota)}
        </div>
        <div>
          {t('Verify')}: {capabilityLabel(capabilityPolicy.verify_enabled, t)} ·{' '}
          {t('Verify min quota')}: {numberValue(rewardPolicy.verify_min_quota)}
        </div>
        <div>
          {t('Invite')}: {capabilityLabel(capabilityPolicy.invite_enabled, t)} ·{' '}
          {t('Invite reward')}: {numberValue(rewardPolicy.invite_reward_quota)}{' '}
          / {numberValue(rewardPolicy.invitee_reward_quota)}
        </div>
      </div>,
      <div className='space-y-1 text-sm'>
        <div>{inviteTarget.primary}</div>
        <div className='text-muted-foreground text-xs'>
          {inviteTarget.secondary}
        </div>
      </div>,
      <div className='space-y-1 text-sm'>
        <div>
          {t('Reward account')}: {groupBudgetPools.join(', ') || '—'}
        </div>
        <div>
          {t('Game rules')}:{' '}
          {enabledGameCodes.join(', ') ||
            t('No enabled game rules visible yet.')}
        </div>
        <div>
          {t('Check-ins')}: {numberValue(latestMetrics.checkins)}
        </div>
        <div>
          {t('Commission')}: {numberValue(latestMetrics.commission_quota)}
        </div>
      </div>,
      <div className='space-y-2'>
        {statusBadge(
          boolValue(runtimeConnectorsOf(group).matches_runtime_connector),
          t('Matched'),
          t('Not matched'),
          t('Unknown')
        )}
        <div className='text-muted-foreground text-xs'>
          {t('Verifies')}: {numberValue(latestMetrics.verifies)}
        </div>
      </div>,
    ],
  }
}

function AgentCapabilityTable({
  capabilityRows,
  groups,
  t,
}: {
  capabilityRows: OpsRenderableGroup[]
  groups: OpsRegistryGroup[]
  t: OpsTranslate
}) {
  return (
    <OpsDataTable
      title={t('Group feature table')}
      description={t(
        'See each group, its available features, rewards, games, and current connection status before editing it.'
      )}
      columns={[
        { key: 'group', label: t('Group') },
        { key: 'role', label: t('Role') },
        { key: 'capabilities', label: t('Available features') },
        { key: 'invite_target', label: t('Invite target') },
        { key: 'metrics', label: t('Rewards and games') },
        { key: 'runtime', label: t('Connection status') },
      ]}
      rows={capabilityRows.map((group) =>
        buildAgentCapabilityRow(group, groups, t)
      )}
      emptyMessage={t('No real group capability rows yet.')}
    />
  )
}

type AgentOverviewProps = {
  groups: OpsRegistryGroup[]
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null
  controlPlane: OpsControlPlaneSnapshot | null
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  audits: OpsAuditOverview | null
  releases: OpsReleaseOverview | null
  saved: SavedOpsContext
  siteId: string
  onEditGroups: () => void
  onEditCapabilities: () => void
  t: OpsTranslate
}

export function AgentOverview({
  groups,
  groupCapabilities,
  controlPlane,
  rewardFund,
  inviteJourney,
  audits,
  releases,
  saved,
  siteId,
  onEditGroups,
  onEditCapabilities,
  t,
}: AgentOverviewProps) {
  const metrics = buildAgentOverviewMetrics(groupCapabilities, groups)
  const auditTimelineEvents = auditEventsByDomain(audits, [
    'admin_config',
    'risk_control',
  ])
  const primaryPlatformText = saved.primaryPlatform
    ? platformLabel(saved.primaryPlatform, t)
    : t('QQ / TG')
  const truthRows = buildAgentTruthRows(primaryPlatformText, t)

  return (
    <div className='space-y-6'>
      <AgentSummaryGrid
        metrics={metrics}
        groupCount={groups.length}
        releases={releases}
        t={t}
      />
      <AgentConfigurationEntryPoints
        groupCount={groups.length}
        capabilityRowCount={metrics.capabilityRows.length}
        enabledGameRules={metrics.enabledGameRules}
        onEditGroups={onEditGroups}
        onEditCapabilities={onEditCapabilities}
        t={t}
      />
      <RewardFundInviteOverview
        rewardFund={rewardFund}
        inviteJourney={inviteJourney}
        siteId={siteId}
        t={t}
      />
      <ControlPlaneTruthTable
        title={t('Configuration value / effective value / runtime value')}
        description={t(
          'Each row shows the saved value, the applied value, and the live runtime signal side by side.'
        )}
        rows={truthRows}
        controlPlane={controlPlane}
        t={t}
      />
      <OpsSurfaceGrid className='xl:grid-cols-[minmax(0,1.08fr)_minmax(340px,.92fr)] xl:items-start'>
        <AgentAccessRulesPanel saved={saved} t={t} />
        <AgentMainGroupSetupPanel
          capabilityRows={metrics.capabilityRows}
          saved={saved}
          primaryPlatformText={primaryPlatformText}
          t={t}
        />
      </OpsSurfaceGrid>
      <AgentCapabilityTable
        capabilityRows={metrics.capabilityRows}
        groups={groups}
        t={t}
      />
      <UnifiedAuditTimeline
        title={t('Changes and risk history')}
        description={t(
          'Configuration changes and risk findings are shown together in time order.'
        )}
        events={auditTimelineEvents}
        t={t}
        emptyMessage={t('No recent config or risk-control audit events yet.')}
      />
    </div>
  )
}
