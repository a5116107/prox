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
import { SettingsPage } from '../components/settings-page'
import type { SecuritySettings } from '../types'
import {
  SECURITY_DEFAULT_SECTION,
  getSecuritySectionContent,
  getSecuritySectionMeta,
} from './section-registry.tsx'

const defaultSecuritySettings: SecuritySettings = {
  ModelRequestRateLimitEnabled: false,
  ModelRequestRateLimitCount: 0,
  ModelRequestRateLimitSuccessCount: 1000,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitGroup: '',
  CheckSensitiveEnabled: false,
  CheckSensitiveOnPromptEnabled: false,
  SensitiveWords: '',
  'risk_control_setting.enabled': true,
  'risk_control_setting.high_risk_key_recreate_required': true,
  'risk_control_setting.high_risk_activation_required': true,
  'risk_control_setting.high_risk_activation_source': 'qq',
  'risk_control_setting.activation_code_ttl_minutes': 10,
  'risk_control_setting.same_ip_same_day_oauth_register_enabled': true,
  'risk_control_setting.same_ip_same_day_oauth_register_limit': 1,
  'risk_control_setting.same_ip_register_block_message':
    '检测到同一 IP 当天已注册过账号，请更换 IP 后再试。',
  'risk_control_setting.request_ip_tracking_enabled': true,
  'risk_control_setting.same_ip_multi_account_usage_enabled': false,
  'risk_control_setting.same_ip_multi_account_usage_window_minutes': 60,
  'risk_control_setting.same_ip_multi_account_usage_user_limit': 1,
  'risk_control_setting.same_ip_multi_account_usage_block_message':
    '检测到同一 IP 在短时间内切换多个账号访问，当前 Key 已触发风控，请更换独立网络后重试。',
  'risk_control_setting.dynamic_ip_churn_enabled': false,
  'risk_control_setting.dynamic_ip_churn_window_minutes': 30,
  'risk_control_setting.dynamic_ip_churn_distinct_ip_limit': 6,
  'risk_control_setting.dynamic_ip_churn_block_message':
    '检测到当前 Key 在短时间内频繁切换 IP，已触发风控，请完成账号校验后再试。',
  'risk_control_setting.burst_register_enabled': false,
  'risk_control_setting.burst_register_window_minutes': 10,
  'risk_control_setting.burst_register_limit': 3,
  'risk_control_setting.burst_register_block_message':
    '检测到该 IP 在短时间内注册过多账号，请稍后更换 IP 后再试。',
  'risk_control_setting.inactive_token_disable_enabled': false,
  'risk_control_setting.inactive_token_disable_days': 7,
  'risk_control_setting.inactive_token_disable_reason':
    '长时间未活跃的账号需重新创建 Key 并完成校验后再使用。',
  'membership_risk.enabled': false,
  'membership_risk.dry_run': true,
  'membership_risk.grace_hours': 24,
  'membership_risk.auto_restore_on_rejoin': true,
  'membership_risk.paid_bypass_enabled': true,
  'membership_risk.event_secret': '',
  'membership_risk.freeze_community_tokens_after_grace': true,
  'membership_risk.revoke_community_access_after_grace': true,
  'membership_risk.block_checkin_on_left': true,
  'membership_risk.block_game_reward_on_left': true,
  'membership_risk.block_invite_reward_on_left': true,
  'membership_risk.block_campaign_bonus_on_left': true,
  'membership_risk.notify_user_on_left': true,
  'membership_risk.notify_admin_on_bulk_left': true,
  'membership_risk.qq_events_enabled': true,
  'membership_risk.tg_events_enabled': true,
  'membership_risk.scheduled_recheck_enabled': true,
  'membership_risk.scheduled_recheck_interval_hours': 12,
  'fetch_setting.enable_ssrf_protection': true,
  'fetch_setting.allow_private_ip': false,
  'fetch_setting.domain_filter_mode': false,
  'fetch_setting.ip_filter_mode': false,
  'fetch_setting.domain_list': [],
  'fetch_setting.ip_list': [],
  'fetch_setting.allowed_ports': [],
  'fetch_setting.apply_ip_filter_for_domain': false,
}

export function SecuritySettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/security/$section'
      defaultSettings={defaultSecuritySettings}
      defaultSection={SECURITY_DEFAULT_SECTION}
      getSectionContent={getSecuritySectionContent}
      getSectionMeta={getSecuritySectionMeta}
    />
  )
}
