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
import { useStatus } from '@/hooks/use-status'
import { SettingsPage } from '../components/settings-page'
import type { OperationsSettings, SystemOption } from '../types'
import { OperationsLiveShell } from './ops-live-shell'
import {
  OPERATIONS_DEFAULT_SECTION,
  getOperationsSectionContent,
  getOperationsSectionMeta,
  type OperationsSectionId,
} from './section-registry.tsx'

function parseLooseOptionValue(value: string): string | number | boolean {
  const trimmed = value.trim()
  if (!trimmed) return ''

  const normalized = trimmed.toLowerCase()
  if (['true', '1', 'yes', 'on'].includes(normalized)) return true
  if (['false', '0', 'no', 'off'].includes(normalized)) return false

  return value
}

function mergeOperationsSettings(
  settings: OperationsSettings,
  raw: SystemOption[] | undefined
): OperationsSettings {
  if (!raw?.length) return settings

  const merged: OperationsSettings = { ...settings }
  raw.forEach((option) => {
    merged[option.key] = parseLooseOptionValue(option.value)
    merged[`__raw.${option.key}`] = option.value
  })

  return merged
}

const defaultOperationsSettings: OperationsSettings = {
  RetryTimes: 0,
  DefaultCollapseSidebar: false,
  DemoSiteEnabled: false,
  SelfUseModeEnabled: false,
  ChannelDisableThreshold: '',
  QuotaRemindThreshold: '',
  AutomaticDisableChannelEnabled: false,
  AutomaticEnableChannelEnabled: false,
  AutomaticDisableKeywords: '',
  AutomaticDisableStatusCodes: '401',
  AutomaticRetryStatusCodes:
    '100-199,300-399,401-407,409-499,500-503,505-523,525-599',
  'monitor_setting.auto_test_channel_enabled': false,
  'monitor_setting.auto_test_channel_minutes': 10,
  SMTPServer: '',
  SMTPPort: '',
  SMTPAccount: '',
  SMTPFrom: '',
  SMTPToken: '',
  SMTPSSLEnabled: false,
  SMTPForceAuthLogin: false,
  WorkerUrl: '',
  WorkerValidKey: '',
  WorkerAllowHttpImageRequestEnabled: false,
  LogConsumeEnabled: false,
  'performance_setting.disk_cache_enabled': false,
  'performance_setting.disk_cache_threshold_mb': 10,
  'performance_setting.disk_cache_max_size_mb': 1024,
  'performance_setting.disk_cache_path': '',
  'performance_setting.monitor_enabled': false,
  'performance_setting.monitor_cpu_threshold': 90,
  'performance_setting.monitor_memory_threshold': 90,
  'performance_setting.monitor_disk_threshold': 95,
  'perf_metrics_setting.enabled': true,
  'perf_metrics_setting.flush_interval': 5,
  'perf_metrics_setting.bucket_time': 'hour',
  'perf_metrics_setting.retention_days': 0,
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
}

export function OperationsSettings() {
  const { status } = useStatus()

  function getWrappedOperationsSectionContent(
    sectionId: OperationsSectionId,
    settings: OperationsSettings,
    ...extraArgs: [string | null | undefined, number | null | undefined]
  ) {
    return (
      <OperationsLiveShell sectionId={sectionId} defaultValues={settings}>
        {getOperationsSectionContent(sectionId, settings, ...extraArgs)}
      </OperationsLiveShell>
    )
  }

  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/operations/$section'
      defaultSettings={defaultOperationsSettings}
      resolveSettings={mergeOperationsSettings}
      defaultSection={OPERATIONS_DEFAULT_SECTION}
      getSectionContent={getWrappedOperationsSectionContent}
      getSectionMeta={getOperationsSectionMeta}
      extraArgs={[
        status?.version as string | undefined,
        status?.start_time as number | null | undefined,
      ]}
      loadingMessage='Loading maintenance settings...'
    />
  )
}
