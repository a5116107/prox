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
export interface AccessControlStatus {
  state?: Record<string, unknown>
  compliant?: boolean
  access_level?: string
  reason_code?: string
  reason_message?: string
  next_steps?: string[]
  action?: string
  join_url?: string
  primary_join_url?: string
  bind_url?: string
  effective_groups?: string[]
  requested_group?: string
  has_active_frozen_keys?: boolean
  active_frozen_keys?: number
  can_restore?: boolean
  community_bound?: boolean
  has_oauth_binding?: boolean
  has_room_membership?: boolean
  primary_bound?: boolean
  primary_platform?: string
  matched_primary_group_id?: string
  denied_message?: string
  upgrade_message?: string
  check_on_login?: boolean
  block_token_create?: boolean
  block_token_enable?: boolean
}
