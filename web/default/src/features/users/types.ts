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
import type { AccessControlStatus } from '@/types/access-control'

// ============================================================================
// User Types
// ============================================================================

export interface User {
  [key: string]: unknown
  id: number
  username: string
  display_name: string
  password?: string
  github_id?: string
  oidc_id?: string
  wechat_id?: string
  telegram_id?: string
  email?: string
  quota: number
  used_quota: number
  request_count: number
  group: string
  aff_code?: string
  aff_count?: number
  aff_quota?: number
  aff_history_quota?: number
  inviter_id?: number
  linux_do_id?: string
  status: number // 1 = enabled, 2 = disabled, 3+ = other states
  role: number // 1 = common user, 10 = admin, 100 = root
  access_control?: AccessControlStatus
  created_at?: number
  updated_at?: number
  last_login_at?: number
  DeletedAt?: unknown | null
  remark?: string
}

// ============================================================================
// API Request/Response Types
// ============================================================================

/** Generic API response */
export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

type BooleanFilterValue = 'true' | 'false'

export interface AdminUserOpsGridParams {
  keyword?: string
  group?: string
  role?: string
  status?: string
  access_level?: string
  community_bound?: BooleanFilterValue | ''
  has_community_room?: BooleanFilterValue | ''
  qq_bound?: BooleanFilterValue | ''
  tg_bound?: BooleanFilterValue | ''
  primary_bound?: BooleanFilterValue | ''
  has_frozen_keys?: BooleanFilterValue | ''
  override_mode?: string
  p?: number
  page_size?: number
}

interface AdminIdentityBinding {
  id?: number
  site_id?: string
  provider?: string
  external_user_id?: string
  username?: string
  status?: string
  created_at?: number
  updated_at?: number
}

interface AdminAgentChatBinding {
  id?: number
  site_id?: string
  source?: string
  room_id?: string
  external_user_id?: string
  username?: string
  enabled?: boolean
  created_at?: number
  updated_at?: number
}

interface AdminChatMembershipState {
  id?: number
  source?: string
  room_id?: string
  external_user_id?: string
  username?: string
  status?: string
  reason_code?: string
  reason?: string
  checked_at?: number
  updated_at?: number
}

interface AdminUserInviteePreview {
  user_id: number
  username: string
  display_name?: string
  email?: string
  status?: number
  last_login_at?: number
  created_at?: number
}

export interface AdminUserOpsGridItem extends User {
  id: number
  user_id: number
  site_id?: string
  community_site_id?: string
  username: string
  display_name: string
  email?: string
  quota: number
  used_quota: number
  request_count: number
  group: string
  base_group: string
  role: number
  status: number
  access_level: string
  effective_groups: string[]
  community_bound: boolean
  has_community_room_membership: boolean
  community_external_user_id?: string
  community_username?: string
  qq_bound: boolean
  qq_bound_group_ids: string[]
  qq_external_user_id?: string
  qq_username?: string
  tg_bound: boolean
  tg_bound_group_ids: string[]
  tg_external_user_id?: string
  tg_username?: string
  primary_bound: boolean
  primary_platform?: string
  matched_primary_group_id?: string
  manual_override_mode: string
  manual_override_groups: string[]
  manual_override_reason?: string
  active_frozen_key_count: number
  access_control_frozen_keys?: number
  community_gate_frozen_keys?: number
  risk_controlled_keys?: number
  has_active_risk_control?: boolean
  has_active_frozen_keys?: boolean
  can_restore?: boolean
  inviter_id?: number
  inviter_username?: string
  inviter_display_name?: string
  invitee_count?: number
  invitee_preview?: AdminUserInviteePreview[]
  aff_count?: number
  aff_quota?: number
  aff_history_quota?: number
  last_login_at?: number
  last_login_ip?: string
  last_login_source?: string
  last_login_ip_at?: number
  reason_code?: string
  reason_message?: string
}

export interface AdminUserOpsGridResponse {
  success: boolean
  message?: string
  data?: {
    items: AdminUserOpsGridItem[]
    total: number
    page: number
    page_size: number
  }
}

export interface AdminUserOpsProfile extends AdminUserOpsGridItem {
  site_id: string
  community_site_id?: string
  created_at?: number
  reason_message?: string
  manual_override_reason?: string
  has_community_oauth_binding: boolean
  community_room_ids: string[]
  community_external_user_id?: string
  community_username?: string
  qq_external_user_id?: string
  qq_username?: string
  tg_external_user_id?: string
  tg_username?: string
  access_control_frozen_keys: number
  community_gate_frozen_keys: number
  risk_controlled_keys: number
  has_active_risk_control: boolean
  has_active_frozen_keys: boolean
  access_control_status?: AccessControlStatus | Record<string, unknown>
  community_gate_status?: Record<string, unknown>
  identity_bindings: AdminIdentityBinding[]
  agent_chat_bindings: AdminAgentChatBinding[]
  chat_membership_states: AdminChatMembershipState[]
}

export interface AdminUserBindingsData {
  site_id: string
  user_id: number
  identity_bindings: AdminIdentityBinding[]
  agent_chat_bindings: AdminAgentChatBinding[]
  chat_membership_states: AdminChatMembershipState[]
}

export type AdminUserBindingsResponse = ApiResponse<AdminUserBindingsData>

export type AdminUserAccessOverrideMode =
  | 'clear'
  | 'full_access'
  | 'community_only'
  | 'custom_groups'
  | 'none'

export interface AdminUserAccessOverridePayload {
  mode: AdminUserAccessOverrideMode
  groups?: string[]
  reason?: string
}

export interface UserFormData {
  username: string
  display_name: string
  password?: string
  role?: number // Only used when creating user
  quota?: number // Only used when updating user
  group?: string // Only used when updating user
  remark?: string // Only used when updating user
}

export type ManageUserAction =
  | 'promote'
  | 'demote'
  | 'enable'
  | 'disable'
  | 'delete'
  | 'add_quota'

export type QuotaAdjustMode = 'add' | 'subtract' | 'override'

export interface ManageUserQuotaPayload {
  id: number
  action: 'add_quota'
  mode: QuotaAdjustMode
  value: number
}

// ============================================================================
// Dialog Types
// ============================================================================

export type UsersDialogType = 'create' | 'update' | 'delete'

export type UserListRow = User | AdminUserOpsGridItem
