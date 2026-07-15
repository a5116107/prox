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
import { GroupRegistryActionsPanel } from './ops-group-registry'
import { type OpsTranslate } from './ops-i18n'
import {
  type OpsRenderableGroup,
  type OpsTruthRowDefinition,
  type SavedOpsContext,
  UnifiedAuditTimeline,
  accessQualifiersOf,
  auditEventsByDomain,
  auditRawRows,
  boolValue,
  groupRoleLabel,
  groupStatusLabel,
  platformLabel,
  resolveCapabilityRows,
  runtimeConnectorsOf,
  statusBadge,
} from './ops-live-foundation'
import {
  ControlPlaneTruthTable,
  inviteTargetSummary,
  releaseModeBadge,
} from './ops-live-truth'
import { RewardFundInviteOverview } from './ops-reward-overview'
import {
  OpsDataTable,
  OpsInsightCard,
  OpsPanel,
  OpsStageRail,
  OpsStatusBadge,
  OpsSurfaceGrid,
} from './ops-shared'
import {
  type OpsAuditOverview,
  type OpsCommunityGateOverview,
  type OpsControlPlaneSnapshot,
  type OpsGroupCapabilityMatrixOverview,
  type OpsGroupActions,
  type OpsInviteJourneyOverview,
  type OpsRegistryGroup,
  type OpsRewardFundOverview,
  type OpsReleaseOverview,
} from './use-ops-registry'

function buildCommunityTruthRows(
  primaryPlatformText: string,
  t: OpsTranslate
): OpsTruthRowDefinition[] {
  return [
    {
      key: 'community-room-ids',
      label: t('Saved community rooms'),
      help: t('Saved gate room IDs for this site.'),
      domain: 'community_gate',
      fields: ['room_ids', 'room_id'],
    },
    {
      key: 'community-room-match-mode',
      label: t('Saved room match mode'),
      help: t(
        'How the saved community rooms are evaluated: any matched room or every required room.'
      ),
      domain: 'community_gate',
      fields: ['room_match_mode'],
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
      key: 'primary-group-ids',
      label: t('Saved main groups'),
      help: t(
        'Loaded from access-control settings that decide which primary QQ or TG groups unlock broader access.'
      ),
      domain: 'access_control',
      fields: ['primary_group_ids'],
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
  ]
}

function CommunitySummaryGrid({
  communityGate,
  saved,
  mainUnlockGroupCount,
  deniedAuditCount,
  releases,
  t,
}: {
  communityGate: OpsCommunityGateOverview | null
  saved: SavedOpsContext
  mainUnlockGroupCount: number
  deniedAuditCount: number
  releases: OpsReleaseOverview | null
  t: OpsTranslate
}) {
  return (
    <OpsSurfaceGrid className='xl:grid-cols-4'>
      <OpsInsightCard
        title={t('Effective community rooms')}
        value={
          communityGate?.room_ids?.length || saved.gateRoomIds.length || '0'
        }
        description={
          <div className='space-y-1'>
            <div>
              {t('Community rooms currently visible in the live gate policy.')}
            </div>
            <div className='text-muted-foreground/80 text-xs'>
              {saved.sourceSummary.communityGate}
            </div>
          </div>
        }
        badge={statusBadge(
          communityGate?.enabled ?? saved.gateEnabled,
          t('Enabled'),
          t('Disabled'),
          t('Not saved')
        )}
      />
      <OpsInsightCard
        title={t('Community-only API groups')}
        value={saved.communityOnlyGroups.length || '0'}
        description={
          <div className='space-y-1'>
            <div>
              {t(
                'API groups that community-bound users can use before joining the main field.'
              )}
            </div>
            <div className='text-muted-foreground/80 text-xs'>
              {saved.sourceSummary.accessControl}
            </div>
          </div>
        }
      />
      <OpsInsightCard
        title={t('Main-field unlock groups')}
        value={mainUnlockGroupCount || '0'}
        description={t(
          'Real QQ / TG groups that currently unlock full access.'
        )}
      />
      <OpsInsightCard
        title={t('Recent gate audits')}
        value={communityGate?.recent_audits?.length || deniedAuditCount || '0'}
        description={t(
          'Latest live gate audits currently returned by the backend.'
        )}
        badge={releaseModeBadge(releases?.release_mode, t)}
      />
    </OpsSurfaceGrid>
  )
}

function CommunityAccessPolicyPanel({
  saved,
  mainUnlockGroups,
  t,
}: {
  saved: SavedOpsContext
  mainUnlockGroups: OpsRenderableGroup[]
  t: OpsTranslate
}) {
  return (
    <OpsPanel
      title={t('Community-to-main-field policy')}
      description={t(
        'This is the current live funnel: community first, then QQ / TG main groups, with explicit restrictions for key creation and group usage.'
      )}
    >
      <div className='grid gap-3 md:grid-cols-2'>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>
            {t('Community-bound users')}
          </div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.communityOnlyGroups.length > 0
              ? `${t('Only these API groups are available')}: ${saved.communityOnlyGroups.join(', ')}`
              : t('No effective community-only API groups are visible yet.')}
          </div>
          <div className='mt-3 flex flex-wrap gap-2'>
            {statusBadge(
              saved.blockTokenCreate,
              t('Key creation is gated'),
              t('Key creation not gated'),
              t('Key creation rule is not visible in the live policy')
            )}
            {statusBadge(
              saved.blockTokenEnable,
              t('Key enable is gated'),
              t('Key enable not gated'),
              t('Key enable rule is not visible in the live policy')
            )}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>{t('Main-field unlock')}</div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {mainUnlockGroups.length > 0
              ? `${t('Current unlock groups')}: ${mainUnlockGroups
                  .map((group) => group.group_name || group.group_id)
                  .join(', ')}`
              : t('No real main-field unlock groups detected yet.')}
          </div>
          <div className='mt-3 flex flex-wrap gap-2'>
            <OpsStatusBadge tone='info'>
              {saved.primaryPlatform
                ? `${t('Primary platform')}: ${platformLabel(saved.primaryPlatform, t)}`
                : t('Primary platform not visible in live policy')}
            </OpsStatusBadge>
            <OpsStatusBadge tone='neutral'>
              {saved.fullAccessGroups.length > 0
                ? `${t('Full-access groups')}: ${saved.fullAccessGroups.join(', ')}`
                : t('Full-access groups are not visible in the live policy')}
            </OpsStatusBadge>
          </div>
        </div>
      </div>
    </OpsPanel>
  )
}

function CommunityUserPromptsPanel({
  saved,
  t,
}: {
  saved: SavedOpsContext
  t: OpsTranslate
}) {
  const prompts = [
    [t('Community join link'), saved.communityJoinUrl, 'font-mono text-xs'],
    [t('Primary-field join link'), saved.primaryJoinUrl, 'font-mono text-xs'],
    [t('Current deny prompt'), saved.denyMessage, 'text-sm leading-6'],
    [t('Current upgrade prompt'), saved.upgradeMessage, 'text-sm leading-6'],
  ] as const
  return (
    <OpsPanel
      title={t('User-facing prompts and links')}
      description={t(
        'Admins need to see the exact live prompts and jump links users will receive when they are missing community binding or main-field binding.'
      )}
    >
      <div className='grid gap-3'>
        {prompts.map(([label, value, valueClass]) => (
          <div
            key={label}
            className='border-border/70 bg-background/70 rounded-[18px] border p-4'
          >
            <div className='text-sm font-semibold'>{label}</div>
            <div
              className={`text-muted-foreground mt-2 break-all ${valueClass}`}
            >
              {value || t('Not saved')}
            </div>
          </div>
        ))}
      </div>
    </OpsPanel>
  )
}

function CommunitySetupGuide({
  saved,
  primaryPlatformText,
  t,
}: {
  saved: SavedOpsContext
  primaryPlatformText: string
  t: OpsTranslate
}) {
  return (
    <OpsPanel
      title={t('How to add rooms and unlock groups')}
      description={t(
        'When a new community room or fallback group is added, update these three real settings together so the room gate, usable groups, and user prompts stay consistent.'
      )}
    >
      <div className='grid gap-3 md:grid-cols-3'>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>
            {t('Community valid room list')}
          </div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.gateRoomIds.length > 0
              ? saved.gateRoomIds.join(', ')
              : t('No saved room IDs yet.')}
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {t(
              'Put every community room ID that counts as valid membership here. Save first, then confirm the room appears in the live room table below.'
            )}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>
            {t('Community fallback groups')}
          </div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {saved.communityOnlyGroups.length > 0
              ? saved.communityOnlyGroups.join(', ')
              : t('No community fallback groups saved yet.')}
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {t(
              'Put the API groups that community-bound users can use before joining the {{platform}} main group here.',
              { platform: primaryPlatformText }
            )}
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>
            {t('Join links and user prompts')}
          </div>
          <div className='text-muted-foreground mt-2 space-y-2 text-sm leading-6'>
            <div>
              {saved.communityJoinUrl
                ? `${t('Community join link')}: ${saved.communityJoinUrl}`
                : t('Community join link is not saved yet.')}
            </div>
            <div>
              {saved.denyMessage
                ? `${t('Blocked message')}: ${saved.denyMessage}`
                : t('Blocked message is not saved yet.')}
            </div>
            <div>
              {saved.upgradeMessage
                ? `${t('Upgrade message')}: ${saved.upgradeMessage}`
                : t('Upgrade message is not saved yet.')}
            </div>
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {t(
              'Keep the join link and prompt copy aligned, so blocked users know whether they still need community binding or the {{platform}} main-group bind.',
              { platform: primaryPlatformText }
            )}
          </div>
        </div>
      </div>
    </OpsPanel>
  )
}

function buildCommunityRegistryRow(
  group: OpsRenderableGroup,
  groups: OpsRegistryGroup[],
  t: OpsTranslate
) {
  const qualifiers = accessQualifiersOf(group)
  const communityUnlock = boolValue(qualifiers.qualifies_community_binding)
  const primaryUnlock = boolValue(qualifiers.qualifies_primary_binding)
  const runtimeMatched = boolValue(
    runtimeConnectorsOf(group).matches_runtime_connector
  )
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
      <div className='space-y-1 text-sm'>
        <div>{groupRoleLabel(group.role, t)}</div>
        <div className='text-muted-foreground text-xs'>
          {groupStatusLabel(group.status, group.enabled, t)}
        </div>
      </div>,
      <div className='flex flex-wrap gap-2'>
        {statusBadge(
          communityUnlock,
          t('Community unlock'),
          t('No community unlock'),
          t('Unknown')
        )}
        {statusBadge(
          primaryUnlock,
          t('Main-field unlock'),
          t('No main-field unlock'),
          t('Unknown')
        )}
      </div>,
      <div className='space-y-1 text-sm'>
        <div>{inviteTarget.primary}</div>
        <div className='text-muted-foreground text-xs'>
          {inviteTarget.secondary}
        </div>
      </div>,
      <div className='text-muted-foreground text-sm leading-6'>
        {primaryUnlock
          ? t('Unlocks full-access API groups after main-field binding.')
          : communityUnlock
            ? t(
                'Keeps users in the community-only key lane until they bind the main field.'
              )
            : t('No key unlock rule is visible for this group yet.')}
      </div>,
      statusBadge(runtimeMatched, t('Matched'), t('Not matched'), t('Unknown')),
    ],
  }
}

function CommunityRegistryTable({
  communityGroups,
  mainUnlockGroups,
  groups,
  t,
}: {
  communityGroups: OpsRenderableGroup[]
  mainUnlockGroups: OpsRenderableGroup[]
  groups: OpsRegistryGroup[]
  t: OpsTranslate
}) {
  const rows = [...communityGroups, ...mainUnlockGroups]
    .filter(
      (group, index, all) =>
        all.findIndex((item) => item.id === group.id) === index
    )
    .map((group) => buildCommunityRegistryRow(group, groups, t))
  return (
    <OpsDataTable
      title={t('Real room and unlock registry')}
      description={t(
        'Every community room, QQ group, and TG group should be visible here before admins trust any copy or runtime result.'
      )}
      columns={[
        { key: 'group', label: t('Group / room') },
        { key: 'role', label: t('Role') },
        { key: 'unlock', label: t('Unlock policy') },
        { key: 'invite_target', label: t('Invite target') },
        { key: 'key', label: t('Key scope') },
        { key: 'runtime', label: t('Runtime match') },
      ]}
      rows={rows}
      emptyMessage={t('No real room or unlock registry rows yet.')}
    />
  )
}

function CommunityStageRail({ t }: { t: OpsTranslate }) {
  return (
    <OpsStageRail
      steps={[
        {
          id: '01',
          title: t('Community first'),
          description: t(
            'Users who have only finished community binding should stay on community-only keys and receive a clear prompt toward the main field.'
          ),
        },
        {
          id: '02',
          title: t('Main-field unlock'),
          description: t(
            'After QQ or TG binding succeeds, the user can move from community-only groups into the site full-access groups.'
          ),
        },
        {
          id: '03',
          title: t('Paid-user override'),
          description: t(
            'Paid users keep the highest priority access lane and should always have an explicit audit trail.'
          ),
        },
      ]}
      columnsClassName='xl:grid-cols-3'
    />
  )
}

type CommunityOverviewProps = {
  groups: OpsRegistryGroup[]
  communityGate: OpsCommunityGateOverview | null
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null
  controlPlane: OpsControlPlaneSnapshot | null
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  audits: OpsAuditOverview | null
  releases: OpsReleaseOverview | null
  saved: SavedOpsContext
  groupActions: OpsGroupActions
  siteId: string
  t: OpsTranslate
}

export function CommunityOverview({
  groups,
  communityGate,
  groupCapabilities,
  controlPlane,
  rewardFund,
  inviteJourney,
  audits,
  releases,
  saved,
  groupActions,
  siteId,
  t,
}: CommunityOverviewProps) {
  const capabilityRows = resolveCapabilityRows(groupCapabilities, groups)
  const communityGroups = capabilityRows.filter((group) => {
    const family = String(
      group.platform_family || group.platform || ''
    ).toLowerCase()
    return family === 'community'
  })
  const mainUnlockGroups = capabilityRows.filter((group) =>
    boolValue(accessQualifiersOf(group).qualifies_primary_binding)
  )
  const deniedAuditCount = auditRawRows(audits, 'community_gate').filter(
    (row) => !boolValue(row.compliant)
  ).length
  const auditTimelineEvents = auditEventsByDomain(audits, [
    'community_gate',
    'admin_config',
  ])
  const primaryPlatformText = saved.primaryPlatform
    ? platformLabel(saved.primaryPlatform, t)
    : t('QQ / TG')
  const truthRows = buildCommunityTruthRows(primaryPlatformText, t)

  return (
    <div className='space-y-6'>
      <RewardFundInviteOverview
        rewardFund={rewardFund}
        inviteJourney={inviteJourney}
        siteId={siteId}
        t={t}
      />
      <CommunitySummaryGrid
        communityGate={communityGate}
        saved={saved}
        mainUnlockGroupCount={mainUnlockGroups.length}
        deniedAuditCount={deniedAuditCount}
        releases={releases}
        t={t}
      />
      <GroupRegistryActionsPanel
        siteId={siteId}
        groups={groups}
        saved={saved}
        groupActions={groupActions}
        defaultPlatform='community'
        title={t('Add or clone a community room')}
        description={t(
          'Register a new community intake room, or clone an existing room template before wiring the room gate and fallback API groups.'
        )}
        t={t}
      />
      <ControlPlaneTruthTable
        title={t('Saved, active, and observed values')}
        description={t(
          'Compare what was saved, what is active now, and what the service most recently reported.'
        )}
        rows={truthRows}
        controlPlane={controlPlane}
        t={t}
      />
      <OpsSurfaceGrid className='xl:grid-cols-[minmax(0,1.12fr)_minmax(340px,.88fr)] xl:items-start'>
        <CommunityAccessPolicyPanel
          saved={saved}
          mainUnlockGroups={mainUnlockGroups}
          t={t}
        />
        <CommunityUserPromptsPanel saved={saved} t={t} />
      </OpsSurfaceGrid>
      <CommunitySetupGuide
        saved={saved}
        primaryPlatformText={primaryPlatformText}
        t={t}
      />
      <CommunityRegistryTable
        communityGroups={communityGroups}
        mainUnlockGroups={mainUnlockGroups}
        groups={groups}
        t={t}
      />
      <UnifiedAuditTimeline
        title={t('Unified audit timeline')}
        description={t(
          'Community gate checks and related config changes now share one ordered event stream, so operators can explain the latest deny, room update, and release-facing config move without jumping across multiple tables.'
        )}
        events={auditTimelineEvents}
        t={t}
        emptyMessage={t('No recent community gate or config audit events yet.')}
      />
      <CommunityStageRail t={t} />
    </div>
  )
}
