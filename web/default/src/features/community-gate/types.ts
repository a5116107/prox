/*
Copyright (C) 2023-2026 QuantumNous
*/

interface CommunityGateResult {
  enabled: boolean
  user_id: number
  username?: string
  provider_slug?: string
  provider_user_id?: string
  room_id?: string
  compliant: boolean
  has_oauth_binding: boolean
  has_room_membership: boolean
  reason_code?: string
  reason?: string
  denied_message?: string
  checked_at?: number
}

export interface CommunityGateMeStatus {
  gate?: CommunityGateResult | null
  compliant: boolean
  has_active_frozen_keys: boolean
  active_frozen_keys: number
  can_restore: boolean
  join_url?: string
  bind_url?: string
  provider_slug?: string
  community_host?: string
  room_id?: string
  denied_message?: string
}

interface CommunityGateRestoreStats {
  eligible?: number
  disabled?: number
  already_frozen?: number
  active_frozen?: number
  restored?: number
}

export interface CommunityGateRestoreResponse {
  stats?: CommunityGateRestoreStats
  gate?: CommunityGateResult | null
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}
