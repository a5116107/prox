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
import { useMutation, type QueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import {
  type OpsAccessExplainRequest,
  type OpsGroupActions,
  type OpsGroupBulkSavePayload,
  type OpsGroupChatOpsSavePayload,
  type OpsGroupGamesSavePayload,
  type OpsGroupSavePayload,
  type OpsReleaseActions,
  type OpsReleasePublishPayload,
  type OpsReleaseRollbackPayload,
} from './ops-registry-contract'
import {
  asRecord,
  normalizeAccessExplainPayload,
  normalizeGroupBulkPayload,
  normalizeGroupPayload,
  normalizeReleaseRecord,
} from './ops-registry-normalizers'

export async function refreshOpsRegistryQueries(
  queryClient: QueryClient,
  siteId: string
) {
  const queryNames = [
    'groups',
    'releases',
    'release-impact',
    'control-plane',
    'community-gate',
    'group-capabilities',
    'fund',
    'invite-journey',
    'audits',
  ] as const
  await Promise.all(
    queryNames.map((name) =>
      queryClient.invalidateQueries({
        queryKey: [`ops-registry-${name}`, siteId],
      })
    )
  )
}

function useGroupCreateMutations(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  const createGroupMutation = useMutation({
    mutationFn: async (payload: OpsGroupSavePayload) => {
      const res = await api.post(
        '/api/ops/groups',
        { ...payload, site_id: payload.site_id || siteId },
        { params: { site_id: siteId } }
      )
      return normalizeGroupPayload(res.data?.data ?? res.data)
    },
    onSuccess: refreshRegistryQueries,
  })

  const createGroupBulkMutation = useMutation({
    mutationFn: async (payload: OpsGroupBulkSavePayload) => {
      const res = await api.post(
        '/api/ops/groups/bulk',
        {
          ...payload,
          site_id: payload.site_id || siteId,
        },
        { params: { site_id: siteId } }
      )
      return normalizeGroupBulkPayload(res.data?.data ?? res.data)
    },
    onSuccess: refreshRegistryQueries,
  })

  return {
    create: (payload: OpsGroupSavePayload) =>
      createGroupMutation.mutateAsync(payload),
    createBulk: (payload: OpsGroupBulkSavePayload) =>
      createGroupBulkMutation.mutateAsync(payload),
    saving: createGroupMutation.isPending || createGroupBulkMutation.isPending,
  }
}

function useGroupEditMutations(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  const updateGroupMutation = useMutation({
    mutationFn: async ({
      id,
      payload,
    }: {
      id: number
      payload: OpsGroupSavePayload
    }) => {
      const res = await api.put(
        `/api/ops/groups/${encodeURIComponent(String(id))}`,
        { ...payload, site_id: payload.site_id || siteId },
        { params: { site_id: siteId } }
      )
      return normalizeGroupPayload(res.data?.data ?? res.data)
    },
    onSuccess: refreshRegistryQueries,
  })

  const cloneGroupMutation = useMutation({
    mutationFn: async ({
      id,
      payload,
    }: {
      id: number
      payload: OpsGroupSavePayload
    }) => {
      const res = await api.post(
        `/api/ops/groups/${encodeURIComponent(String(id))}/clone`,
        { ...payload, site_id: payload.site_id || siteId },
        { params: { site_id: siteId } }
      )
      return normalizeGroupPayload(res.data?.data ?? res.data)
    },
    onSuccess: refreshRegistryQueries,
  })

  return {
    update: (id: number, payload: OpsGroupSavePayload) =>
      updateGroupMutation.mutateAsync({ id, payload }),
    clone: (id: number, payload: OpsGroupSavePayload) =>
      cloneGroupMutation.mutateAsync({ id, payload }),
    saving: updateGroupMutation.isPending || cloneGroupMutation.isPending,
  }
}

function useGroupRegistryRecordMutations(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  const create = useGroupCreateMutations(siteId, refreshRegistryQueries)
  const edit = useGroupEditMutations(siteId, refreshRegistryQueries)
  return {
    create: create.create,
    createBulk: create.createBulk,
    update: edit.update,
    clone: edit.clone,
    saving: create.saving || edit.saving,
  }
}

function useGroupCapabilityMutations(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  const saveGroupChatOpsMutation = useMutation({
    mutationFn: async ({
      id,
      payload,
    }: {
      id: number
      payload: OpsGroupChatOpsSavePayload
    }) => {
      const res = await api.put(
        `/api/ops/groups/${encodeURIComponent(String(id))}/chatops`,
        payload,
        { params: { site_id: siteId } }
      )
      return normalizeGroupPayload(res.data?.data ?? res.data)
    },
    onSuccess: refreshRegistryQueries,
  })

  const saveGroupGamesMutation = useMutation({
    mutationFn: async ({
      id,
      payload,
    }: {
      id: number
      payload: OpsGroupGamesSavePayload
    }) => {
      const res = await api.put(
        `/api/ops/groups/${encodeURIComponent(String(id))}/games`,
        payload,
        { params: { site_id: siteId } }
      )
      return normalizeGroupPayload(res.data?.data ?? res.data)
    },
    onSuccess: refreshRegistryQueries,
  })

  return {
    saveChatOps: (id: number, payload: OpsGroupChatOpsSavePayload) =>
      saveGroupChatOpsMutation.mutateAsync({ id, payload }),
    saveGames: (id: number, payload: OpsGroupGamesSavePayload) =>
      saveGroupGamesMutation.mutateAsync({ id, payload }),
    saving:
      saveGroupChatOpsMutation.isPending || saveGroupGamesMutation.isPending,
  }
}

export function useOpsRegistryGroupMutations(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  const registry = useGroupRegistryRecordMutations(
    siteId,
    refreshRegistryQueries
  )
  const capabilities = useGroupCapabilityMutations(
    siteId,
    refreshRegistryQueries
  )
  return {
    create: registry.create,
    createBulk: registry.createBulk,
    update: registry.update,
    clone: registry.clone,
    saveChatOps: capabilities.saveChatOps,
    saveGames: capabilities.saveGames,
    saving: registry.saving || capabilities.saving,
  } satisfies OpsGroupActions
}

function useReleaseActions(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  const publishReleaseMutation = useMutation({
    mutationFn: async (payload: OpsReleasePublishPayload) => {
      if (!siteId) return null
      const res = await api.post(
        `/api/ops/releases/${encodeURIComponent(siteId)}/publish`,
        payload
      )
      const data = asRecord(res.data?.data ?? res.data)
      return normalizeReleaseRecord(data.release)
    },
    onSuccess: refreshRegistryQueries,
  })

  const rollbackReleaseMutation = useMutation({
    mutationFn: async (payload: OpsReleaseRollbackPayload) => {
      if (!siteId) return null
      const res = await api.post(
        `/api/ops/releases/${encodeURIComponent(siteId)}/rollback`,
        payload
      )
      const data = asRecord(res.data?.data ?? res.data)
      return normalizeReleaseRecord(data.release)
    },
    onSuccess: refreshRegistryQueries,
  })

  return {
    publish: (payload: OpsReleasePublishPayload) =>
      publishReleaseMutation.mutateAsync(payload),
    rollback: (payload: OpsReleaseRollbackPayload) =>
      rollbackReleaseMutation.mutateAsync(payload),
    publishing: publishReleaseMutation.isPending,
    rollingBack: rollbackReleaseMutation.isPending,
    busy: publishReleaseMutation.isPending || rollbackReleaseMutation.isPending,
  } satisfies OpsReleaseActions
}

function useAccessExplainMutation(siteId: string) {
  const explainAccessMutation = useMutation({
    mutationFn: async ({
      userId,
      requestedGroup,
      refresh = true,
    }: OpsAccessExplainRequest) => {
      if (!siteId) return null
      const res = await api.get(
        `/api/ops/access-policy/${encodeURIComponent(siteId)}/explain-user/${encodeURIComponent(String(userId))}`,
        {
          params: {
            group: requestedGroup || undefined,
            refresh: refresh ? 1 : 0,
          },
        }
      )
      return normalizeAccessExplainPayload(res.data?.data ?? res.data)
    },
  })
  return {
    data: explainAccessMutation.data ?? null,
    loading: explainAccessMutation.isPending,
    error: (explainAccessMutation.error as Error | null) ?? null,
    run: (payload: OpsAccessExplainRequest) =>
      explainAccessMutation.mutateAsync(payload),
    reset: () => explainAccessMutation.reset(),
  }
}

export function useOpsRegistryReleaseMutations(
  siteId: string,
  refreshRegistryQueries: () => Promise<void>
) {
  return {
    releaseActions: useReleaseActions(siteId, refreshRegistryQueries),
    accessExplain: useAccessExplainMutation(siteId),
  }
}
