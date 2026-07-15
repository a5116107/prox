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

type AccessControlT = (key: string, options?: Record<string, unknown>) => string

function normalizePrimaryPlatform(platform?: string) {
  const value = String(platform || '')
    .trim()
    .toLowerCase()
  if (value === 'telegram') return 'tg'
  if (value === 'community') return 'community'
  if (value === 'tg') return 'tg'
  return 'qq'
}

export function getPrimaryPlatformDisplayName(
  platform: string | undefined,
  t: AccessControlT
) {
  switch (normalizePrimaryPlatform(platform)) {
    case 'tg':
      return t('Telegram')
    case 'community':
      return t('Community')
    default:
      return t('QQ')
  }
}

export function getAccessControlActionLabels(
  status: AccessControlStatus | null | undefined,
  t: AccessControlT
) {
  const platform = normalizePrimaryPlatform(status?.primary_platform)
  const primary =
    platform === 'tg'
      ? t('Open Telegram main group')
      : platform === 'community'
        ? t('Open community group')
        : t('Open QQ main group')

  return {
    bind: t('Open binding page / generate bind code'),
    community: t('Open community group'),
    primary,
    close: t('Close'),
    refresh: t('Refresh status'),
    done: t('I have finished, recheck now'),
  }
}

export function getAccessControlDisplayMessage(
  status: AccessControlStatus | null | undefined,
  t: AccessControlT
) {
  const primaryPlatform = getPrimaryPlatformDisplayName(
    status?.primary_platform,
    t
  )

  switch (status?.access_level) {
    case 'admin_bypass':
      return t('Administrator bypass is active')
    case 'paid_bypass':
      return t('Paid-user bypass is active')
    case 'full_access':
    case 'manual_override':
      return t('Primary-group binding completed')
    case 'community_only':
      return (
        status.upgrade_message ||
        t(
          'Community authorization is complete. Bind {{platform}} to unlock more groups.',
          { platform: primaryPlatform }
        )
      )
    default:
      return (
        status?.reason_message ||
        status?.denied_message ||
        t('Please complete account binding and group join requirements first.')
      )
  }
}

export function getAccessControlNextSteps(
  status: AccessControlStatus | null | undefined,
  t: AccessControlT
) {
  if (Array.isArray(status?.next_steps) && status.next_steps.length > 0) {
    return Array.from(
      new Set(
        status.next_steps
          .map((step) => String(step || '').trim())
          .filter(Boolean)
      )
    )
  }
  const steps: string[] = []
  const primaryPlatform = getPrimaryPlatformDisplayName(
    status?.primary_platform,
    t
  )

  if (
    status?.access_level === 'admin_bypass' ||
    status?.access_level === 'paid_bypass'
  ) {
    return steps
  }

  if (!status?.has_oauth_binding) {
    steps.push(t('Use HHHL community OAuth login first'))
  }
  if (status?.has_oauth_binding && !status?.has_room_membership) {
    steps.push(t('Join the community room configured by this site'))
  }
  if (!status?.primary_bound) {
    steps.push(
      t(
        'Bind your {{platform}} account to the primary group to unlock more groups.',
        { platform: primaryPlatform }
      )
    )
  }

  return Array.from(new Set(steps))
}

export function getAccessControlGroupPickerHint(
  status: AccessControlStatus | null | undefined,
  t: AccessControlT
) {
  const primaryPlatform = getPrimaryPlatformDisplayName(
    status?.primary_platform,
    t
  )

  switch (status?.access_level) {
    case 'admin_bypass':
      return t(
        'As an administrator, you can use all API groups allowed by the current site policy.'
      )
    case 'paid_bypass':
      return t(
        'You currently bypass binding checks via a paid-user package. The visible groups still follow this site’s policy.'
      )
    case 'community_only':
      return t(
        'Only unlocked groups are shown here. Complete {{platform}} primary-group binding to reveal more groups.',
        { platform: primaryPlatform }
      )
    case 'none':
      return t(
        'Finish community authorization and room join first. Unlocked groups will appear here afterwards.'
      )
    default:
      return null
  }
}
