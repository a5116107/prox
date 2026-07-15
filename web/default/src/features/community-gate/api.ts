/*
Copyright (C) 2023-2026 QuantumNous
*/
import { api } from '@/lib/api'
import type {
  ApiResponse,
  CommunityGateMeStatus,
  CommunityGateRestoreResponse,
} from './types'

export async function getCommunityGateMe(
  refresh = false
): Promise<ApiResponse<CommunityGateMeStatus>> {
  const res = await api.get('/api/community-gate/me', {
    params: refresh ? { refresh: 'true' } : undefined,
    disableDuplicate: true,
    skipBusinessError: true,
  })
  return res.data
}

export async function restoreCommunityGateSelf(): Promise<
  ApiResponse<CommunityGateRestoreResponse>
> {
  const res = await api.post('/api/community-gate/restore-self', undefined, {
    skipBusinessError: true,
  })
  return res.data
}
