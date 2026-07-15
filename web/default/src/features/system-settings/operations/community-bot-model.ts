/*
Copyright (C) 2023-2026 QuantumNous
*/
import { z } from 'zod'

export type CommunityBotStatus = {
  enabled: boolean
  configured: boolean
  authorized: boolean
  community_host: string
  room_id: string
  bot_user_id: string
  bot_username: string
  last_message_id: string
  last_scanned_at: number
}

type CommunityBotStatRow = {
  id?: number
  user_id?: number
  provider_user_id?: string
  room_id?: string
  stat_date?: string
  message_count?: number
  distinct_texts?: number
  last_message_id?: string
  updated_at?: number
}

type CommunityBotRewardRow = {
  id?: number
  user_id?: number
  provider_user_id?: string
  room_id?: string
  reward_type?: string
  reward_key?: string
  quota?: number
  message_count?: number
  created_at?: number
}

export type CommunityBotStats = {
  date?: string
  totals?: {
    users?: number
    message_count?: number
    rewarded_count?: number
    rewarded_quota?: number
  }
  stats?: CommunityBotStatRow[]
  rewards?: CommunityBotRewardRow[]
}

export type CommunityBotDefaultValues = Record<
  string,
  string | number | boolean
>

export const communityBotSchema = z.object({
  enabled: z.boolean(),
  communityHost: z.string().url(),
  providerSlug: z.string().min(1),
  roomId: z.string().min(1),
  oauthCallbackUrl: z.string().url().optional().or(z.literal('')),
  oauthClientId: z.string().min(1),
  oauthClientSecret: z.string().optional(),
  autoInviteEnabled: z.boolean(),
  inviteOnMiAuthLogin: z.boolean(),
  joinRewardEnabled: z.boolean(),
  joinRewardMinQuota: z.coerce.number().int().min(0),
  joinRewardMaxQuota: z.coerce.number().int().min(0),
  dailyMessageRewardEnabled: z.boolean(),
  dailyMessageThreshold: z.coerce.number().int().min(1),
  dailyRewardMinQuota: z.coerce.number().int().min(0),
  dailyRewardMaxQuota: z.coerce.number().int().min(0),
  dailyMaxRewardsPerUser: z.coerce.number().int().min(0),
  messageScanIntervalMinutes: z.coerce.number().int().min(1),
  messageLookbackMinutes: z.coerce.number().int().min(1),
  messageScanLimit: z.coerce.number().int().min(1).max(100),
  antiSpamMinChars: z.coerce.number().int().min(0),
  antiSpamMinDistinctTexts: z.coerce.number().int().min(1),
  antiSpamIgnoreBot: z.boolean(),
  notificationEnabled: z.boolean(),
  notifyOnInvite: z.boolean(),
  notifyOnJoinReward: z.boolean(),
  notifyOnDailyReward: z.boolean(),
  notifyOnOpsAlert: z.boolean(),
  inviteNotificationTemplate: z.string(),
  joinRewardTemplate: z.string(),
  dailyRewardTemplate: z.string(),
  opsAlertTemplate: z.string(),
  messageScanIntervalSeconds: z.coerce.number().int().min(0),
  streamingEnabled: z.boolean(),
  commandBurnAfterSeconds: z.coerce.number().int().min(0),
  checkinSuccessTemplate: z.string(),
  checkinFailedTemplate: z.string(),
  verifyPassTemplate: z.string(),
  verifyFailedTemplate: z.string(),
  bindGuideTemplate: z.string(),
})

export type CommunityBotValues = z.infer<typeof communityBotSchema>

export const communityBotOptionKeyMap: Record<
  keyof CommunityBotValues,
  string
> = {
  enabled: 'community_bot_setting.enabled',
  communityHost: 'community_bot_setting.community_host',
  providerSlug: 'community_bot_setting.provider_slug',
  roomId: 'community_bot_setting.room_id',
  oauthCallbackUrl: 'community_bot_setting.oauth_callback_url',
  oauthClientId: 'community_bot_setting.oauth_client_id',
  oauthClientSecret: 'community_bot_setting.oauth_client_secret',
  autoInviteEnabled: 'community_bot_setting.auto_invite_enabled',
  inviteOnMiAuthLogin: 'community_bot_setting.invite_on_oauth_login',
  joinRewardEnabled: 'community_bot_setting.join_reward_enabled',
  joinRewardMinQuota: 'community_bot_setting.join_reward_min_quota',
  joinRewardMaxQuota: 'community_bot_setting.join_reward_max_quota',
  dailyMessageRewardEnabled:
    'community_bot_setting.daily_message_reward_enabled',
  dailyMessageThreshold: 'community_bot_setting.daily_message_threshold',
  dailyRewardMinQuota: 'community_bot_setting.daily_reward_min_quota',
  dailyRewardMaxQuota: 'community_bot_setting.daily_reward_max_quota',
  dailyMaxRewardsPerUser: 'community_bot_setting.daily_max_rewards_per_user',
  messageScanIntervalMinutes:
    'community_bot_setting.message_scan_interval_minutes',
  messageLookbackMinutes: 'community_bot_setting.message_lookback_minutes',
  messageScanLimit: 'community_bot_setting.message_scan_limit',
  antiSpamMinChars: 'community_bot_setting.anti_spam_min_chars',
  antiSpamMinDistinctTexts:
    'community_bot_setting.anti_spam_min_distinct_texts',
  antiSpamIgnoreBot: 'community_bot_setting.anti_spam_ignore_bot',
  notificationEnabled: 'community_bot_setting.notification_enabled',
  notifyOnInvite: 'community_bot_setting.notify_on_invite',
  notifyOnJoinReward: 'community_bot_setting.notify_on_join_reward',
  notifyOnDailyReward: 'community_bot_setting.notify_on_daily_reward',
  notifyOnOpsAlert: 'community_bot_setting.notify_on_ops_alert',
  inviteNotificationTemplate:
    'community_bot_setting.invite_notification_template',
  joinRewardTemplate: 'community_bot_setting.join_reward_template',
  dailyRewardTemplate: 'community_bot_setting.daily_reward_template',
  opsAlertTemplate: 'community_bot_setting.ops_alert_template',
  messageScanIntervalSeconds:
    'community_bot_setting.message_scan_interval_seconds',
  streamingEnabled: 'community_bot_setting.streaming_enabled',
  commandBurnAfterSeconds: 'community_bot_setting.command_burn_after_seconds',
  checkinSuccessTemplate: 'community_bot_setting.checkin_success_template',
  checkinFailedTemplate: 'community_bot_setting.checkin_failed_template',
  verifyPassTemplate: 'community_bot_setting.verify_pass_template',
  verifyFailedTemplate: 'community_bot_setting.verify_failed_template',
  bindGuideTemplate: 'community_bot_setting.bind_guide_template',
}

export function buildCommunityBotDefaults(
  defaultValues: CommunityBotDefaultValues
): CommunityBotValues {
  return {
    enabled: Boolean(defaultValues['community_bot_setting.enabled']),
    communityHost: String(
      defaultValues['community_bot_setting.community_host'] ||
        'https://dc.hhhl.cc'
    ),
    providerSlug: String(
      defaultValues['community_bot_setting.provider_slug'] || 'dc.hhhl.cc'
    ),
    roomId: String(defaultValues['community_bot_setting.room_id'] || ''),
    oauthCallbackUrl: String(
      defaultValues['community_bot_setting.oauth_callback_url'] || ''
    ),
    oauthClientId: String(
      defaultValues['community_bot_setting.oauth_client_id'] || ''
    ),
    oauthClientSecret: String(
      defaultValues['community_bot_setting.oauth_client_secret'] || ''
    ),
    autoInviteEnabled: Boolean(
      defaultValues['community_bot_setting.auto_invite_enabled']
    ),
    inviteOnMiAuthLogin: Boolean(
      defaultValues['community_bot_setting.invite_on_oauth_login']
    ),
    joinRewardEnabled: Boolean(
      defaultValues['community_bot_setting.join_reward_enabled']
    ),
    joinRewardMinQuota: Number(
      defaultValues['community_bot_setting.join_reward_min_quota'] || 2500000
    ),
    joinRewardMaxQuota: Number(
      defaultValues['community_bot_setting.join_reward_max_quota'] || 2500000
    ),
    dailyMessageRewardEnabled: Boolean(
      defaultValues['community_bot_setting.daily_message_reward_enabled']
    ),
    dailyMessageThreshold: Number(
      defaultValues['community_bot_setting.daily_message_threshold'] || 5
    ),
    dailyRewardMinQuota: Number(
      defaultValues['community_bot_setting.daily_reward_min_quota'] || 500000
    ),
    dailyRewardMaxQuota: Number(
      defaultValues['community_bot_setting.daily_reward_max_quota'] || 500000
    ),
    dailyMaxRewardsPerUser: Number(
      defaultValues['community_bot_setting.daily_max_rewards_per_user'] || 1
    ),
    messageScanIntervalMinutes: Number(
      defaultValues['community_bot_setting.message_scan_interval_minutes'] || 5
    ),
    messageLookbackMinutes: Number(
      defaultValues['community_bot_setting.message_lookback_minutes'] || 1440
    ),
    messageScanLimit: Number(
      defaultValues['community_bot_setting.message_scan_limit'] || 100
    ),
    antiSpamMinChars: Number(
      defaultValues['community_bot_setting.anti_spam_min_chars'] || 2
    ),
    antiSpamMinDistinctTexts: Number(
      defaultValues['community_bot_setting.anti_spam_min_distinct_texts'] || 3
    ),
    antiSpamIgnoreBot: Boolean(
      defaultValues['community_bot_setting.anti_spam_ignore_bot'] ?? true
    ),
    notificationEnabled: Boolean(
      defaultValues['community_bot_setting.notification_enabled']
    ),
    notifyOnInvite: Boolean(
      defaultValues['community_bot_setting.notify_on_invite']
    ),
    notifyOnJoinReward: Boolean(
      defaultValues['community_bot_setting.notify_on_join_reward']
    ),
    notifyOnDailyReward: Boolean(
      defaultValues['community_bot_setting.notify_on_daily_reward']
    ),
    notifyOnOpsAlert: Boolean(
      defaultValues['community_bot_setting.notify_on_ops_alert']
    ),
    inviteNotificationTemplate: String(
      defaultValues['community_bot_setting.invite_notification_template'] ||
        '已邀请 @{username} 加入社区群聊，欢迎来群里交流。'
    ),
    joinRewardTemplate: String(
      defaultValues['community_bot_setting.join_reward_template'] ||
        '欢迎 @{username} 加入社区群聊，已发放社区奖励 {amount}。'
    ),
    dailyRewardTemplate: String(
      defaultValues['community_bot_setting.daily_reward_template'] ||
        '@{username} 今日有效发言已达 {count} 条，已自动发放 {amount}。'
    ),
    opsAlertTemplate: String(
      defaultValues['community_bot_setting.ops_alert_template'] ||
        '社区管家提醒：{message}'
    ),
    messageScanIntervalSeconds: Number(
      defaultValues['community_bot_setting.message_scan_interval_seconds'] || 30
    ),
    streamingEnabled: Boolean(
      defaultValues['community_bot_setting.streaming_enabled'] ?? true
    ),
    commandBurnAfterSeconds: Number(
      defaultValues['community_bot_setting.command_burn_after_seconds'] ?? 15
    ),
    checkinSuccessTemplate: String(
      defaultValues['community_bot_setting.checkin_success_template'] || ''
    ),
    checkinFailedTemplate: String(
      defaultValues['community_bot_setting.checkin_failed_template'] || ''
    ),
    verifyPassTemplate: String(
      defaultValues['community_bot_setting.verify_pass_template'] || ''
    ),
    verifyFailedTemplate: String(
      defaultValues['community_bot_setting.verify_failed_template'] || ''
    ),
    bindGuideTemplate: String(
      defaultValues['community_bot_setting.bind_guide_template'] || ''
    ),
  }
}
