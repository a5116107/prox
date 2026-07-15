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
import { Fragment, useMemo, useState, type ReactNode } from 'react'
import { Badge } from '@/components/ui/badge'
import {
  AgentLiveEditingPanels,
  AgentOperationsHeader,
} from './ops-agent-controls'
import { AgentOverview } from './ops-agent-overview'
import { CommunityOverview } from './ops-community-overview'
import { useOpsT, type OpsTranslate } from './ops-i18n'
import {
  type Props,
  type QuickAction,
  type SavedOpsContext,
  type ViewMode,
  platformLabel,
} from './ops-live-foundation'
import {
  EditorContainer,
  buildSavedContext,
  buildSectionActions,
  releaseModeBadge,
  scrollToOpsAnchor,
  sectionHeroDescription,
  sectionLabel,
} from './ops-live-truth'
import { MembershipOverview } from './ops-membership-overview'
import { ReleaseControlPanel } from './ops-release-control'
import { OpsHeroCard, OpsPanel, OpsStatusBadge } from './ops-shared'
import type { OperationsSectionId } from './section-registry'
import {
  useOpsRegistry,
  type OpsAccessExplainResult,
  type OpsAuditOverview,
  type OpsCommunityGateOverview,
  type OpsControlPlaneSnapshot,
  type OpsGroupCapabilityMatrixOverview,
  type OpsGroupActions,
  type OpsReleaseActions,
  type OpsInviteJourneyOverview,
  type OpsReleaseImpactPreview,
  type OpsRegistryScope,
  type OpsRegistryGroup,
  type OpsRewardFundOverview,
  type OpsReleaseOverview,
} from './use-ops-registry'

function RealDataUnavailable({
  error,
  t,
}: {
  error: Error | null
  t: OpsTranslate
}) {
  if (!error) return null
  return (
    <OpsPanel
      title={t('Real endpoints need attention')}
      description={t(
        'The page is still showing the latest successful live truth and fallback configuration data. The endpoint error below is explicit so admins do not mistake it for “no configuration”.'
      )}
    >
      <div className='text-muted-foreground text-sm leading-6'>
        {error.message || t('Real endpoints are temporarily unavailable.')}
      </div>
    </OpsPanel>
  )
}

function SectionOverview({
  sectionId,
  groups,
  communityGate,
  groupCapabilities,
  controlPlane,
  rewardFund,
  inviteJourney,
  releases,
  releaseImpact,
  audits,
  saved,
  groupActions,
  releaseActions,
  accessExplain,
  siteId,
  onEditGroups,
  onEditCapabilities,
  t,
}: {
  sectionId: OperationsSectionId
  groups: OpsRegistryGroup[]
  communityGate: OpsCommunityGateOverview | null
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null
  controlPlane: OpsControlPlaneSnapshot | null
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  releases: OpsReleaseOverview | null
  releaseImpact: OpsReleaseImpactPreview | null
  audits: OpsAuditOverview | null
  saved: SavedOpsContext
  groupActions: OpsGroupActions
  releaseActions: OpsReleaseActions
  accessExplain: {
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
  siteId: string
  onEditGroups: () => void
  onEditCapabilities: () => void
  t: OpsTranslate
}) {
  if (sectionId === 'community-ops') {
    return (
      <div className='space-y-6'>
        <CommunityOverview
          groups={groups}
          communityGate={communityGate}
          groupCapabilities={groupCapabilities}
          controlPlane={controlPlane}
          rewardFund={rewardFund}
          inviteJourney={inviteJourney}
          audits={audits}
          releases={releases}
          saved={saved}
          groupActions={groupActions}
          siteId={siteId}
          t={t}
        />
        <ReleaseControlPanel
          siteId={siteId}
          releases={releases}
          releaseImpact={releaseImpact}
          releaseActions={releaseActions}
          t={t}
        />
      </div>
    )
  }
  if (sectionId === 'agent-ops') {
    return (
      <div className='space-y-6'>
        <AgentOverview
          groups={groups}
          groupCapabilities={groupCapabilities}
          controlPlane={controlPlane}
          rewardFund={rewardFund}
          inviteJourney={inviteJourney}
          audits={audits}
          releases={releases}
          saved={saved}
          siteId={siteId}
          onEditGroups={onEditGroups}
          onEditCapabilities={onEditCapabilities}
          t={t}
        />
        <ReleaseControlPanel
          siteId={siteId}
          releases={releases}
          releaseImpact={releaseImpact}
          releaseActions={releaseActions}
          t={t}
        />
      </div>
    )
  }
  if (sectionId === 'membership-risk') {
    return (
      <div className='space-y-6'>
        <MembershipOverview
          communityGate={communityGate}
          audits={audits}
          releases={releases}
          controlPlane={controlPlane}
          saved={saved}
          rewardFund={rewardFund}
          inviteJourney={inviteJourney}
          accessExplain={accessExplain}
          t={t}
        />
        <ReleaseControlPanel
          siteId={siteId}
          releases={releases}
          releaseImpact={releaseImpact}
          releaseActions={releaseActions}
          t={t}
        />
      </div>
    )
  }
  return null
}

function buildRegistryScope(
  sectionId: OperationsSectionId,
  mode: ViewMode
): OpsRegistryScope {
  if (mode === 'editor') {
    if (sectionId === 'agent-ops') {
      return {
        groups: true,
        releases: true,
        releaseImpact: true,
        controlPlane: true,
        groupCapabilities: true,
        rewardFund: true,
        inviteJourney: true,
        audits: true,
      }
    }
    return {
      controlPlane: true,
      releases: true,
    }
  }
  if (sectionId === 'membership-risk') {
    return {
      releases: true,
      releaseImpact: true,
      controlPlane: true,
      communityGate: true,
      rewardFund: true,
      inviteJourney: true,
      audits: true,
    }
  }
  if (sectionId === 'community-ops' || sectionId === 'agent-ops') {
    return {
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
  }
  return {
    controlPlane: true,
  }
}

type OpsRegistryState = ReturnType<typeof useOpsRegistry>

function useOperationsViewNavigation(
  sectionId: OperationsSectionId,
  t: OpsTranslate
) {
  const [mode, setMode] = useState<ViewMode>('overview')
  const setViewMode = (nextMode: ViewMode) => {
    setMode(nextMode)
    if (
      nextMode === 'overview' &&
      typeof window !== 'undefined' &&
      window.location.hash.startsWith('#ops-')
    ) {
      window.history.replaceState(
        null,
        '',
        `${window.location.pathname}${window.location.search}`
      )
    }
  }
  const openEditorAt = (anchor: string) => {
    setViewMode('editor')
    window.setTimeout(() => scrollToOpsAnchor(anchor), 80)
  }
  const sectionActions = buildSectionActions(sectionId, t).map((action) => {
    const marker = action.href?.indexOf('#') ?? -1
    if (marker < 0) return action
    const path = action.href?.slice(0, marker) || ''
    const anchor = action.href?.slice(marker + 1) || ''
    const samePage = !path || path.endsWith(`/operations/${sectionId}`)
    if (!samePage || !anchor) return action
    return {
      ...action,
      href: undefined,
      onClick: () => openEditorAt(anchor),
    }
  })
  return { mode, setViewMode, openEditorAt, sectionActions }
}

function buildOperationsHeroActions({
  sectionId,
  mode,
  loading,
  onSetMode,
  onRefresh,
  t,
}: {
  sectionId: OperationsSectionId
  mode: ViewMode
  loading: boolean
  onSetMode: (mode: ViewMode) => void
  onRefresh: () => void
  t: OpsTranslate
}): QuickAction[] {
  return [
    ...(sectionId === 'community-ops'
      ? [
          {
            id: 'hero-edit-community',
            label: t('Edit community settings'),
            onClick: () => onSetMode('editor'),
            tone: 'success' as const,
          },
        ]
      : []),
    ...(sectionId === 'membership-risk'
      ? [
          {
            id: 'hero-edit-membership',
            label: t('Edit membership rules'),
            onClick: () => onSetMode('editor'),
            tone: 'success' as const,
          },
        ]
      : []),
    {
      id: 'overview',
      label: t('Overview'),
      onClick: () => onSetMode('overview'),
      tone: mode === 'overview' ? 'success' : 'neutral',
    },
    {
      id: 'editor',
      label: t('Edit settings'),
      onClick: () => onSetMode('editor'),
      tone: mode === 'editor' ? 'info' : 'neutral',
    },
    {
      id: 'refresh-truth',
      label: loading ? t('Loading') : t('Refresh data'),
      onClick: onRefresh,
      tone: 'info',
    },
  ]
}

function OperationsSectionHeader({
  sectionId,
  saved,
  effectiveSiteId,
  registry,
  recentAuditCount,
  mode,
  onSetMode,
  onOpenEditor,
  sectionActions,
  t,
}: {
  sectionId: OperationsSectionId
  saved: SavedOpsContext
  effectiveSiteId: string
  registry: OpsRegistryState
  recentAuditCount: number
  mode: ViewMode
  onSetMode: (mode: ViewMode) => void
  onOpenEditor: (anchor: string) => void
  sectionActions: QuickAction[]
  t: OpsTranslate
}) {
  const onRefresh = () => void registry.refetchAll()
  if (sectionId === 'agent-ops') {
    return (
      <AgentOperationsHeader
        saved={saved}
        effectiveSiteId={effectiveSiteId}
        releases={registry.releases}
        recentAuditCount={recentAuditCount}
        mode={mode}
        loading={registry.loading}
        onSetMode={onSetMode}
        onRefresh={onRefresh}
        onEditGroups={() => onOpenEditor('ops-group-registry-editor')}
        onEditCapabilities={() => onOpenEditor('ops-group-capability-editor')}
        t={t}
      />
    )
  }
  return (
    <div className='border-border/70 from-background via-background to-muted/25 dark:from-background dark:via-background dark:to-muted/10 rounded-[28px] border bg-gradient-to-br p-1 shadow-[0_18px_48px_rgba(15,23,42,0.08)] dark:shadow-[0_24px_56px_rgba(0,0,0,0.38)]'>
      <OpsHeroCard
        eyebrow={t('Site operations')}
        title={sectionLabel(sectionId, t)}
        description={sectionHeroDescription(sectionId, saved, t)}
        badges={[
          <Badge key='site' variant='outline'>
            {t('Site ID')}: {effectiveSiteId || t('Unknown')}
          </Badge>,
          <Badge key='site-name' variant='outline'>
            {saved.siteName
              ? `${t('Site name')}: ${saved.siteName}`
              : t('Site name not visible in live policy')}
          </Badge>,
          <Badge key='platform' variant='outline'>
            {saved.primaryPlatform
              ? `${t('Primary platform')}: ${platformLabel(saved.primaryPlatform, t)}`
              : t('Primary platform not visible in live policy')}
          </Badge>,
          <div key='release-mode'>
            {releaseModeBadge(registry.releases?.release_mode, t)}
          </div>,
          <OpsStatusBadge key='audit-count' tone='info'>
            {t('Recent changes')}: {recentAuditCount}
          </OpsStatusBadge>,
        ]}
        actions={buildOperationsHeroActions({
          sectionId,
          mode,
          loading: registry.loading,
          onSetMode,
          onRefresh,
          t,
        })}
        quickLinks={sectionActions}
      />
    </div>
  )
}

function OperationsSectionContent({
  sectionId,
  mode,
  saved,
  effectiveSiteId,
  registry,
  onSetMode,
  onOpenEditor,
  children,
  t,
}: {
  sectionId: OperationsSectionId
  mode: ViewMode
  saved: SavedOpsContext
  effectiveSiteId: string
  registry: OpsRegistryState
  onSetMode: (mode: ViewMode) => void
  onOpenEditor: (anchor: string) => void
  children: ReactNode
  t: OpsTranslate
}) {
  return (
    <>
      <RealDataUnavailable error={registry.error} t={t} />
      {mode === 'overview' ? (
        <SectionOverview
          sectionId={sectionId}
          groups={registry.groups}
          communityGate={registry.communityGate}
          groupCapabilities={registry.groupCapabilities}
          controlPlane={registry.controlPlane}
          rewardFund={registry.rewardFund}
          inviteJourney={registry.inviteJourney}
          releases={registry.releases}
          releaseImpact={registry.releaseImpact}
          audits={registry.audits}
          saved={saved}
          groupActions={registry.groupActions}
          releaseActions={registry.releaseActions}
          accessExplain={registry.accessExplain}
          siteId={effectiveSiteId || registry.siteId}
          onEditGroups={() => onOpenEditor('ops-group-registry-editor')}
          onEditCapabilities={() => onOpenEditor('ops-group-capability-editor')}
          t={t}
        />
      ) : null}
      {mode === 'editor' ? (
        <div id='ops-editor-root' className='space-y-6'>
          <EditorContainer mode={mode} onSwitch={onSetMode} t={t}>
            {children}
          </EditorContainer>
          {sectionId === 'agent-ops' ? (
            <AgentLiveEditingPanels
              siteId={effectiveSiteId || registry.siteId}
              groups={registry.groups}
              groupCapabilities={registry.groupCapabilities}
              saved={saved}
              groupActions={registry.groupActions}
              t={t}
            />
          ) : null}
        </div>
      ) : null}
    </>
  )
}

export function OperationsLiveShell({
  sectionId,
  defaultValues,
  children,
}: Props) {
  const t = useOpsT()
  const shouldShow =
    sectionId === 'community-ops' ||
    sectionId === 'agent-ops' ||
    sectionId === 'membership-risk'
  const navigation = useOperationsViewNavigation(sectionId, t)
  const registryScope = useMemo(
    () => buildRegistryScope(sectionId, navigation.mode),
    [navigation.mode, sectionId]
  )
  const registry = useOpsRegistry(defaultValues, shouldShow, registryScope)
  const saved = useMemo(
    () => buildSavedContext(defaultValues, registry.controlPlane, t),
    [defaultValues, registry.controlPlane, t]
  )

  if (!shouldShow) return <Fragment>{children}</Fragment>

  const effectiveSiteId = saved.siteId || registry.siteId
  const recentAuditCount = registryScope.audits
    ? registry.audits?.events?.length || 0
    : 0
  return (
    <div className='space-y-6'>
      <OperationsSectionHeader
        sectionId={sectionId}
        saved={saved}
        effectiveSiteId={effectiveSiteId}
        registry={registry}
        recentAuditCount={recentAuditCount}
        mode={navigation.mode}
        onSetMode={navigation.setViewMode}
        onOpenEditor={navigation.openEditorAt}
        sectionActions={navigation.sectionActions}
        t={t}
      />
      <OperationsSectionContent
        sectionId={sectionId}
        mode={navigation.mode}
        saved={saved}
        effectiveSiteId={effectiveSiteId}
        registry={registry}
        onSetMode={navigation.setViewMode}
        onOpenEditor={navigation.openEditorAt}
        t={t}
      >
        {children}
      </OperationsSectionContent>
    </div>
  )
}
