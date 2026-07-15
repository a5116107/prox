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
import { useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  OPS_REGISTRY_DEFAULT_SCOPE,
  type OpsRegistryScope,
} from './ops-registry-contract'
import {
  refreshOpsRegistryQueries,
  useOpsRegistryGroupMutations,
  useOpsRegistryReleaseMutations,
} from './ops-registry-mutations'
import { pickSiteId } from './ops-registry-normalizers'
import {
  buildOpsRegistryActiveQueries,
  getOpsRegistryError,
  isOpsRegistryLoading,
  type OpsRegistryCoreQueries,
  type OpsRegistryOperationalQueries,
  refetchActiveOpsRegistryQueries,
  useOpsRegistryCoreQueries,
  useOpsRegistryOperationalQueries,
} from './ops-registry-queries'

export type {
  OpsAccessExplainRequest,
  OpsAccessExplainResult,
  OpsAuditOverview,
  OpsCommunityGateOverview,
  OpsControlPlaneField,
  OpsControlPlaneSnapshot,
  OpsControlPlaneSource,
  OpsGroupActions,
  OpsGroupBulkSaveFailure,
  OpsGroupBulkSavePayload,
  OpsGroupBulkSaveResult,
  OpsGroupCapabilityMatrixItem,
  OpsGroupCapabilityMatrixOverview,
  OpsGroupChatOpsSavePayload,
  OpsGroupGamesSavePayload,
  OpsGroupGameSaveItem,
  OpsGroupSavePayload,
  OpsInviteJourneyOverview,
  OpsRegistryGroup,
  OpsRegistryScope,
  OpsReleaseActions,
  OpsReleaseImpactDiffBucket,
  OpsReleaseImpactPreview,
  OpsReleaseImpactState,
  OpsReleaseOverview,
  OpsReleasePublishPayload,
  OpsReleaseRecord,
  OpsReleaseRollbackPayload,
  OpsRewardFundOverview,
  OpsUnifiedAuditEvent,
} from './ops-registry-contract'

function buildOpsRegistrySnapshot(
  core: OpsRegistryCoreQueries,
  operational: OpsRegistryOperationalQueries
) {
  return {
    groups: core.groupsQuery.data ?? [],
    releases: core.releasesQuery.data ?? null,
    releaseImpact: core.releaseImpactQuery.data ?? null,
    controlPlane: core.controlPlaneQuery.data ?? null,
    communityGate: operational.communityGateQuery.data ?? null,
    groupCapabilities: operational.groupCapabilitiesQuery.data ?? null,
    rewardFund: operational.rewardFundQuery.data ?? null,
    inviteJourney: operational.inviteJourneyQuery.data ?? null,
    audits: operational.auditsQuery.data ?? null,
  }
}

export function useOpsRegistry(
  defaultValues: Record<string, string | number | boolean>,
  enabled = true,
  scope: OpsRegistryScope = OPS_REGISTRY_DEFAULT_SCOPE
) {
  const siteId = useMemo(() => pickSiteId(defaultValues), [defaultValues])
  const queryEnabled = enabled && Boolean(siteId)
  const queryClient = useQueryClient()
  const resolvedScope = useMemo(
    () => ({ ...OPS_REGISTRY_DEFAULT_SCOPE, ...(scope || {}) }),
    [scope]
  )
  const refreshRegistryQueries = () =>
    refreshOpsRegistryQueries(queryClient, siteId)
  const core = useOpsRegistryCoreQueries(siteId, queryEnabled, resolvedScope)
  const operational = useOpsRegistryOperationalQueries(
    siteId,
    queryEnabled,
    resolvedScope
  )
  const groupActions = useOpsRegistryGroupMutations(
    siteId,
    refreshRegistryQueries
  )
  const mutations = useOpsRegistryReleaseMutations(
    siteId,
    refreshRegistryQueries
  )
  const activeQueries = buildOpsRegistryActiveQueries(
    queryEnabled,
    resolvedScope,
    core,
    operational
  )
  const snapshot = buildOpsRegistrySnapshot(core, operational)

  return {
    siteId,
    ...snapshot,
    loading: isOpsRegistryLoading(activeQueries),
    error: getOpsRegistryError(activeQueries),
    refetchAll: () => refetchActiveOpsRegistryQueries(activeQueries),
    groupActions,
    releaseActions: mutations.releaseActions,
    accessExplain: mutations.accessExplain,
  }
}
