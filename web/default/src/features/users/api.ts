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
import { api } from '@/lib/api'
import type {
  User,
  AdminUserOpsGridItem,
  AdminUserOpsGridParams,
  AdminUserOpsGridResponse,
  AdminUserOpsProfile,
  AdminUserBindingsResponse,
  AdminUserBindingsData,
  AdminUserAccessOverridePayload,
  UserFormData,
  ManageUserAction,
  ManageUserQuotaPayload,
  ApiResponse,
} from './types'

// ============================================================================
// User Management APIs
// ============================================================================

/**
 * Get single user by ID
 */
export async function getUser(id: number): Promise<ApiResponse<User>> {
  const res = await api.get(`/api/user/${id}`)
  return res.data
}

/**
 * Create a new user
 */
export async function createUser(
  data: UserFormData
): Promise<ApiResponse<User>> {
  const res = await api.post('/api/user/', data)
  return res.data
}

/**
 * Update an existing user
 */
export async function updateUser(
  data: UserFormData & { id: number }
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.put('/api/user/', data)
  return res.data
}

/**
 * Delete a single user (hard delete)
 */
export async function deleteUser(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/`)
  return res.data
}

/**
 * Manage user (promote, demote, enable, disable, delete)
 */
export async function manageUser(
  id: number,
  action: ManageUserAction
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', { id, action })
  return res.data
}

/**
 * Adjust user quota atomically (add/subtract/override)
 */
export async function adjustUserQuota(
  payload: ManageUserQuotaPayload
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Reset user's Passkey registration
 */
export async function resetUserPasskey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/reset_passkey`)
  return res.data
}

/**
 * Reset user's Two-Factor Authentication setup
 */
export async function resetUserTwoFA(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/2fa`)
  return res.data
}

/**
 * Get all available groups
 */
export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/')
  return res.data
}

function normalizeStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.map((item) => String(item || '').trim()).filter(Boolean)
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)))
}

export async function getOpsAssignableGroups(
  siteId: string
): Promise<ApiResponse<string[]>> {
  const resolvedSiteId = String(siteId || '').trim()
  if (!resolvedSiteId) {
    return { success: false, message: 'Site ID is required', data: [] }
  }
  const res = await api.get(
    `/api/ops/access-policy/${encodeURIComponent(resolvedSiteId)}`
  )
  const raw = (res.data?.data || {}) as Record<string, unknown>
  const effective = (raw.effective || {}) as Record<string, unknown>
  const runtime = (raw.runtime || {}) as Record<string, unknown>
  const groups = uniqueStrings([
    ...normalizeStringList(effective.community_only_groups),
    ...normalizeStringList(effective.full_access_groups),
    ...normalizeStringList(effective.paid_bypass_groups),
    ...normalizeStringList(effective.paid_user_groups),
    ...normalizeStringList(runtime.community_only_groups),
    ...normalizeStringList(runtime.full_access_groups),
    ...normalizeStringList(runtime.paid_bypass_groups),
    ...normalizeStringList(runtime.paid_user_groups),
  ])
  return {
    success: Boolean(res.data?.success),
    message: res.data?.message,
    data: groups,
  }
}

// ============================================================================
// Admin Binding Management APIs
// ============================================================================

export interface OAuthBinding {
  provider_id: string
  provider_name: string
  user_id?: number
  external_id?: string
}

function toStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.map((item) => String(item || '').trim()).filter(Boolean)
}

function toOptionalNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : undefined
}

function normalizeAdminUserOpsGridItem(value: unknown): AdminUserOpsGridItem {
  const raw = (value || {}) as Record<string, unknown>
  const userId = Number(raw.user_id ?? raw.id ?? 0)
  return {
    ...(raw as Partial<User>),
    id: userId,
    user_id: userId,
    site_id: raw.site_id ? String(raw.site_id) : undefined,
    community_site_id: raw.community_site_id
      ? String(raw.community_site_id)
      : undefined,
    username: String(raw.username || ''),
    display_name: String(raw.display_name || raw.username || ''),
    email: raw.email ? String(raw.email) : undefined,
    quota: Number(raw.quota || 0),
    used_quota: Number(raw.used_quota || 0),
    request_count: Number(raw.request_count || 0),
    group: String(raw.group || raw.base_group || ''),
    base_group: String(raw.base_group || raw.group || ''),
    role: Number(raw.role || 0),
    status: Number(raw.status || 0),
    access_level: String(raw.access_level || 'none'),
    effective_groups: toStringArray(raw.effective_groups),
    community_bound: Boolean(raw.community_bound),
    has_community_room_membership: Boolean(raw.has_community_room_membership),
    community_external_user_id: raw.community_external_user_id
      ? String(raw.community_external_user_id)
      : undefined,
    community_username: raw.community_username
      ? String(raw.community_username)
      : undefined,
    qq_bound: Boolean(raw.qq_bound),
    qq_bound_group_ids: toStringArray(raw.qq_bound_group_ids),
    qq_external_user_id: raw.qq_external_user_id
      ? String(raw.qq_external_user_id)
      : undefined,
    qq_username: raw.qq_username ? String(raw.qq_username) : undefined,
    tg_bound: Boolean(raw.tg_bound),
    tg_bound_group_ids: toStringArray(raw.tg_bound_group_ids),
    tg_external_user_id: raw.tg_external_user_id
      ? String(raw.tg_external_user_id)
      : undefined,
    tg_username: raw.tg_username ? String(raw.tg_username) : undefined,
    primary_bound: Boolean(raw.primary_bound),
    primary_platform: raw.primary_platform
      ? String(raw.primary_platform)
      : undefined,
    matched_primary_group_id: raw.matched_primary_group_id
      ? String(raw.matched_primary_group_id)
      : undefined,
    manual_override_mode: String(raw.manual_override_mode || ''),
    manual_override_groups: toStringArray(raw.manual_override_groups),
    manual_override_reason: raw.manual_override_reason
      ? String(raw.manual_override_reason)
      : undefined,
    active_frozen_key_count: Number(raw.active_frozen_key_count || 0),
    access_control_frozen_keys: toOptionalNumber(
      raw.access_control_frozen_keys
    ),
    community_gate_frozen_keys: toOptionalNumber(
      raw.community_gate_frozen_keys
    ),
    risk_controlled_keys: toOptionalNumber(raw.risk_controlled_keys),
    has_active_risk_control: Boolean(raw.has_active_risk_control),
    has_active_frozen_keys: Boolean(raw.has_active_frozen_keys),
    can_restore: Boolean(raw.can_restore),
    inviter_id: toOptionalNumber(raw.inviter_id),
    inviter_username: raw.inviter_username
      ? String(raw.inviter_username)
      : undefined,
    inviter_display_name: raw.inviter_display_name
      ? String(raw.inviter_display_name)
      : undefined,
    invitee_count: toOptionalNumber(raw.invitee_count),
    invitee_preview: Array.isArray(raw.invitee_preview)
      ? raw.invitee_preview.map((item) => {
          const preview = (item || {}) as Record<string, unknown>
          return {
            user_id: Number(preview.user_id || preview.id || 0),
            username: String(preview.username || ''),
            display_name: preview.display_name
              ? String(preview.display_name)
              : undefined,
            email: preview.email ? String(preview.email) : undefined,
            status: toOptionalNumber(preview.status),
            last_login_at: toOptionalNumber(preview.last_login_at),
            created_at: toOptionalNumber(preview.created_at),
          }
        })
      : [],
    aff_count: toOptionalNumber(raw.aff_count),
    aff_quota: toOptionalNumber(raw.aff_quota),
    aff_history_quota: toOptionalNumber(raw.aff_history_quota),
    last_login_at: toOptionalNumber(raw.last_login_at),
    last_login_ip: raw.last_login_ip ? String(raw.last_login_ip) : undefined,
    last_login_source: raw.last_login_source
      ? String(raw.last_login_source)
      : undefined,
    last_login_ip_at: toOptionalNumber(raw.last_login_ip_at),
    reason_code: raw.reason_code ? String(raw.reason_code) : undefined,
    reason_message: raw.reason_message ? String(raw.reason_message) : undefined,
  }
}

function normalizeAdminUserOpsProfile(value: unknown): AdminUserOpsProfile {
  const raw = (value || {}) as Record<string, unknown>
  return {
    ...normalizeAdminUserOpsGridItem(raw),
    site_id: String(raw.site_id || ''),
    community_site_id: raw.community_site_id
      ? String(raw.community_site_id)
      : undefined,
    created_at: toOptionalNumber(raw.created_at),
    reason_message: raw.reason_message ? String(raw.reason_message) : undefined,
    manual_override_reason: raw.manual_override_reason
      ? String(raw.manual_override_reason)
      : undefined,
    has_community_oauth_binding: Boolean(raw.has_community_oauth_binding),
    community_room_ids: toStringArray(raw.community_room_ids),
    community_external_user_id: raw.community_external_user_id
      ? String(raw.community_external_user_id)
      : undefined,
    community_username: raw.community_username
      ? String(raw.community_username)
      : undefined,
    qq_external_user_id: raw.qq_external_user_id
      ? String(raw.qq_external_user_id)
      : undefined,
    qq_username: raw.qq_username ? String(raw.qq_username) : undefined,
    tg_external_user_id: raw.tg_external_user_id
      ? String(raw.tg_external_user_id)
      : undefined,
    tg_username: raw.tg_username ? String(raw.tg_username) : undefined,
    access_control_frozen_keys: Number(raw.access_control_frozen_keys || 0),
    community_gate_frozen_keys: Number(raw.community_gate_frozen_keys || 0),
    risk_controlled_keys: Number(raw.risk_controlled_keys || 0),
    has_active_risk_control: Boolean(raw.has_active_risk_control),
    has_active_frozen_keys: Boolean(raw.has_active_frozen_keys),
    access_control_status:
      (raw.access_control_status as Record<string, unknown>) || undefined,
    community_gate_status:
      (raw.community_gate_status as Record<string, unknown>) || undefined,
    identity_bindings: Array.isArray(raw.identity_bindings)
      ? (raw.identity_bindings as AdminUserOpsProfile['identity_bindings'])
      : [],
    agent_chat_bindings: Array.isArray(raw.agent_chat_bindings)
      ? (raw.agent_chat_bindings as AdminUserOpsProfile['agent_chat_bindings'])
      : [],
    chat_membership_states: Array.isArray(raw.chat_membership_states)
      ? (raw.chat_membership_states as AdminUserOpsProfile['chat_membership_states'])
      : [],
  }
}

function normalizeAdminUserBindings(value: unknown): AdminUserBindingsData {
  const raw = (value || {}) as Record<string, unknown>
  return {
    site_id: String(raw.site_id || ''),
    user_id: Number(raw.user_id || 0),
    identity_bindings: Array.isArray(raw.identity_bindings)
      ? (raw.identity_bindings as AdminUserBindingsData['identity_bindings'])
      : [],
    agent_chat_bindings: Array.isArray(raw.agent_chat_bindings)
      ? (raw.agent_chat_bindings as AdminUserBindingsData['agent_chat_bindings'])
      : [],
    chat_membership_states: Array.isArray(raw.chat_membership_states)
      ? (raw.chat_membership_states as AdminUserBindingsData['chat_membership_states'])
      : [],
  }
}

/**
 * Get user's custom OAuth bindings (admin)
 */
export async function getUserOAuthBindings(
  userId: number
): Promise<ApiResponse<OAuthBinding[]>> {
  const res = await api.get(`/api/user/${userId}/oauth/bindings`)
  return res.data
}

export async function getAdminUserOpsGrid(
  params: AdminUserOpsGridParams = {}
): Promise<AdminUserOpsGridResponse> {
  const {
    keyword = '',
    group = '',
    role = '',
    status = '',
    access_level = '',
    community_bound = '',
    has_community_room = '',
    qq_bound = '',
    tg_bound = '',
    primary_bound = '',
    has_frozen_keys = '',
    override_mode = '',
    p = 1,
    page_size = 20,
  } = params
  const queryParams = new URLSearchParams()
  if (keyword.trim()) queryParams.set('keyword', keyword.trim())
  if (group.trim()) queryParams.set('group', group.trim())
  if (role.trim()) queryParams.set('role', role.trim())
  if (status.trim()) queryParams.set('status', status.trim())
  if (access_level.trim()) queryParams.set('access_level', access_level.trim())
  if (community_bound) queryParams.set('community_bound', community_bound)
  if (has_community_room) {
    queryParams.set('has_community_room', has_community_room)
  }
  if (qq_bound) queryParams.set('qq_bound', qq_bound)
  if (tg_bound) queryParams.set('tg_bound', tg_bound)
  if (primary_bound) queryParams.set('primary_bound', primary_bound)
  if (has_frozen_keys) queryParams.set('has_frozen_keys', has_frozen_keys)
  if (override_mode.trim())
    queryParams.set('override_mode', override_mode.trim())
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))

  const res = await api.get(
    `/api/admin/users/ops-grid?${queryParams.toString()}`
  )
  const payload = res.data as AdminUserOpsGridResponse
  if (!payload?.success || !payload.data) {
    return payload
  }
  return {
    ...payload,
    data: {
      ...payload.data,
      items: Array.isArray(payload.data.items)
        ? payload.data.items.map(normalizeAdminUserOpsGridItem)
        : [],
    },
  }
}

export async function getAdminUserOpsProfile(
  userId: number,
  options: { refresh?: boolean } = {}
): Promise<ApiResponse<AdminUserOpsProfile>> {
  const res = await api.get(`/api/admin/users/${userId}/ops-profile`, {
    params: options.refresh ? { refresh: 1 } : undefined,
  })
  const payload = res.data as ApiResponse<AdminUserOpsProfile>
  if (!payload?.success || !payload.data) {
    return payload
  }
  return {
    ...payload,
    data: normalizeAdminUserOpsProfile(payload.data),
  }
}

export async function getAdminUserBindings(
  userId: number
): Promise<AdminUserBindingsResponse> {
  const res = await api.get(`/api/admin/users/${userId}/bindings`)
  const payload = res.data as AdminUserBindingsResponse
  if (!payload?.success || !payload.data) {
    return payload
  }
  return {
    ...payload,
    data: normalizeAdminUserBindings(payload.data),
  }
}

export async function recomputeAdminUserMembership(
  userId: number
): Promise<ApiResponse<Record<string, unknown>>> {
  const res = await api.post(`/api/admin/users/${userId}/recompute-membership`)
  return res.data
}

export async function updateAdminUserAccessOverride(
  userId: number,
  payload: AdminUserAccessOverridePayload
): Promise<ApiResponse<Record<string, unknown>>> {
  const res = await api.put(
    `/api/admin/users/${userId}/access-override`,
    payload
  )
  return res.data
}

export async function restoreAdminUserKeys(
  userId: number
): Promise<ApiResponse<Record<string, unknown>>> {
  const res = await api.post(`/api/admin/users/${userId}/restore-keys`)
  return res.data
}

/**
 * Clear a user's built-in binding (admin)
 */
export async function adminClearUserBinding(
  userId: number,
  bindingType: string
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${userId}/bindings/${bindingType}`)
  return res.data
}

/**
 * Unbind custom OAuth for a user (admin)
 */
export async function adminUnbindCustomOAuth(
  userId: number,
  providerId: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/oauth/bindings/${providerId}`
  )
  return res.data
}
