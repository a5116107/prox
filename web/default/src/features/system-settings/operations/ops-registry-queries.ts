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
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { type OpsRegistryQueryKey } from './ops-registry-contract'
import {
  normalizeAuditPayload,
  normalizeCommunityGatePayload,
  normalizeControlPlanePayload,
  normalizeGroupCapabilitiesPayload,
  normalizeGroupsPayload,
  normalizeInviteJourneyPayload,
  normalizeReleaseImpactPayload,
  normalizeReleasePayload,
  normalizeRewardFundPayload,
} from './ops-registry-normalizers'

export function useOpsRegistryCoreQueries(
  siteId: string,
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>
) {
  const groupsQuery = useQuery({
    queryKey: ['ops-registry-groups', siteId],
    enabled: queryEnabled && scope.groups,
    staleTime: 60_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get('/api/ops/groups', {
        params: { site_id: siteId },
      })
      return normalizeGroupsPayload(res.data?.data ?? res.data)
    },
  })

  const releasesQuery = useQuery({
    queryKey: ['ops-registry-releases', siteId],
    enabled: queryEnabled && scope.releases,
    staleTime: 60_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/releases/${encodeURIComponent(siteId)}`
      )
      return normalizeReleasePayload(res.data?.data ?? res.data)
    },
  })

  const releaseImpactQuery = useQuery({
    queryKey: ['ops-registry-release-impact', siteId],
    enabled: queryEnabled && scope.releaseImpact,
    staleTime: 45_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/releases/${encodeURIComponent(siteId)}/impact-preview`
      )
      return normalizeReleaseImpactPayload(res.data?.data ?? res.data)
    },
  })

  const controlPlaneQuery = useQuery({
    queryKey: ['ops-registry-control-plane', siteId],
    enabled: queryEnabled && scope.controlPlane,
    staleTime: 60_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/control-plane/site/${encodeURIComponent(siteId)}`
      )
      return normalizeControlPlanePayload(res.data?.data ?? res.data)
    },
  })
  return { groupsQuery, releasesQuery, releaseImpactQuery, controlPlaneQuery }
}

function useOpsRegistryCommunityQueries(
  siteId: string,
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>
) {
  const communityGateQuery = useQuery({
    queryKey: ['ops-registry-community-gate', siteId],
    enabled: queryEnabled && scope.communityGate,
    staleTime: 60_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/community-gate/${encodeURIComponent(siteId)}`
      )
      return normalizeCommunityGatePayload(res.data?.data ?? res.data)
    },
  })

  const auditsQuery = useQuery({
    queryKey: ['ops-registry-audits', siteId],
    enabled: queryEnabled && scope.audits,
    staleTime: 45_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/audits/${encodeURIComponent(siteId)}`,
        {
          params: { limit: 20 },
        }
      )
      return normalizeAuditPayload(res.data?.data ?? res.data)
    },
  })

  const groupCapabilitiesQuery = useQuery({
    queryKey: ['ops-registry-group-capabilities', siteId],
    enabled: queryEnabled && scope.groupCapabilities,
    staleTime: 120_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/group-capabilities/${encodeURIComponent(siteId)}`
      )
      return normalizeGroupCapabilitiesPayload(res.data?.data ?? res.data)
    },
  })

  return { communityGateQuery, auditsQuery, groupCapabilitiesQuery }
}

function useOpsRegistryRewardQueries(
  siteId: string,
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>
) {
  const rewardFundQuery = useQuery({
    queryKey: ['ops-registry-fund', siteId],
    enabled: queryEnabled && scope.rewardFund,
    staleTime: 120_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(`/api/ops/fund/${encodeURIComponent(siteId)}`)
      return normalizeRewardFundPayload(res.data?.data ?? res.data)
    },
  })

  const inviteJourneyQuery = useQuery({
    queryKey: ['ops-registry-invite-journey', siteId],
    enabled: queryEnabled && scope.inviteJourney,
    staleTime: 120_000,
    placeholderData: (previousData) => previousData,
    refetchOnWindowFocus: false,
    retry: 1,
    queryFn: async () => {
      const res = await api.get(
        `/api/ops/invite-journey/${encodeURIComponent(siteId)}`
      )
      return normalizeInviteJourneyPayload(res.data?.data ?? res.data)
    },
  })

  return { rewardFundQuery, inviteJourneyQuery }
}

export function useOpsRegistryOperationalQueries(
  siteId: string,
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>
) {
  const community = useOpsRegistryCommunityQueries(siteId, queryEnabled, scope)
  const rewards = useOpsRegistryRewardQueries(siteId, queryEnabled, scope)
  return {
    ...community,
    ...rewards,
  }
}

export type OpsRegistryCoreQueries = ReturnType<
  typeof useOpsRegistryCoreQueries
>
export type OpsRegistryOperationalQueries = ReturnType<
  typeof useOpsRegistryOperationalQueries
>
type OpsRegistryQuery =
  | OpsRegistryCoreQueries[keyof OpsRegistryCoreQueries]
  | OpsRegistryOperationalQueries[keyof OpsRegistryOperationalQueries]

export type OpsRegistryActiveQuery = {
  enabled: boolean
  query: OpsRegistryQuery
}

function buildCoreActiveQueries(
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>,
  core: OpsRegistryCoreQueries
): OpsRegistryActiveQuery[] {
  return [
    { enabled: queryEnabled && scope.groups, query: core.groupsQuery },
    { enabled: queryEnabled && scope.releases, query: core.releasesQuery },
    {
      enabled: queryEnabled && scope.releaseImpact,
      query: core.releaseImpactQuery,
    },
    {
      enabled: queryEnabled && scope.controlPlane,
      query: core.controlPlaneQuery,
    },
  ]
}

function buildOperationalActiveQueries(
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>,
  operational: OpsRegistryOperationalQueries
): OpsRegistryActiveQuery[] {
  return [
    {
      enabled: queryEnabled && scope.communityGate,
      query: operational.communityGateQuery,
    },
    { enabled: queryEnabled && scope.audits, query: operational.auditsQuery },
    {
      enabled: queryEnabled && scope.groupCapabilities,
      query: operational.groupCapabilitiesQuery,
    },
    {
      enabled: queryEnabled && scope.rewardFund,
      query: operational.rewardFundQuery,
    },
    {
      enabled: queryEnabled && scope.inviteJourney,
      query: operational.inviteJourneyQuery,
    },
  ]
}

export function buildOpsRegistryActiveQueries(
  queryEnabled: boolean,
  scope: Record<OpsRegistryQueryKey, boolean>,
  core: OpsRegistryCoreQueries,
  operational: OpsRegistryOperationalQueries
): OpsRegistryActiveQuery[] {
  return [
    ...buildCoreActiveQueries(queryEnabled, scope, core),
    ...buildOperationalActiveQueries(queryEnabled, scope, operational),
  ]
}

export function isOpsRegistryLoading(activeQueries: OpsRegistryActiveQuery[]) {
  return activeQueries.some(
    ({ enabled, query }) => enabled && (query.isLoading || query.isFetching)
  )
}

export function getOpsRegistryError(activeQueries: OpsRegistryActiveQuery[]) {
  return (
    activeQueries.find(({ enabled, query }) => enabled && Boolean(query.error))
      ?.query.error ?? null
  )
}

export async function refetchActiveOpsRegistryQueries(
  activeQueries: OpsRegistryActiveQuery[]
) {
  await Promise.all(
    activeQueries
      .filter(({ enabled }) => enabled)
      .map(({ query }) => query.refetch())
  )
}
