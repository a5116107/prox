/*
Copyright (C) 2023-2026 QuantumNous
*/
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { z } from 'zod'
import { useForm, type Resolver, type UseFormReturn } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  Bot,
  ChevronDown,
  Coins,
  RefreshCw,
  RotateCcw,
  Save,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { getPrimaryPlatformDisplayName } from '@/hooks/use-access-control'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { GroupCheckinBridgePanel } from './group-checkin-bridge-panel'
import { useOpsT } from './ops-i18n'
import {
  OpsDataTable,
  OpsInsightCard,
  OpsPanel,
  OpsStatusBadge,
  OpsSurfaceGrid,
  OpsTimeline,
  type OpsTimelineItem,
} from './ops-shared'
import {
  buildOpsAccessSnapshot,
  buildOpsGateSnapshot,
  readOpsSavedBool,
  readOpsSavedList,
  readOpsSavedText,
} from './ops-snapshots'

type Props = {
  defaultValues: Record<string, string | number | boolean>
}

type AgentBudgetPool = {
  id?: number
  pool_type?: string
  budget_date?: string
  total_quota?: number
  used_quota?: number
  frozen_quota?: number
  remaining_quota?: number
  status?: string
  degrade_state?: string
}

type OpsRewardFundOverview = {
  site_id?: string
  fund?: {
    balance_quota?: number
    status?: string
  }
  budget_pools_today?: AgentBudgetPool[]
  generated_at?: number
}

type AgentAction = {
  id?: number
  action_type?: string
  agent_name?: string
  target_type?: string
  target_id?: string
  risk_level?: string
  risk_score?: number
  quota_amount?: number
  budget_pool?: string
  approval_required?: boolean
  status?: string
  reason?: string
  created_at?: number
}

type AgentTask = {
  id?: number
  task_type?: string
  agent_name?: string
  source?: string
  room_id?: string
  message_id?: string
  issuer_external_id?: string
  issuer_username?: string
  issuer_role?: string
  text?: string
  command?: string
  status?: string
  risk_level?: string
  risk_score?: number
  action_id?: number
  approval_id?: number
  result_json?: string
  error?: string
  created_at?: number
  updated_at?: number
}

type AgentApproval = {
  id?: number
  action_id?: number
  status?: string
  requested_by?: string
  reviewer_id?: number
  decision?: string
  comment?: string
  created_at?: number
}

type AgentRisk = {
  id?: number
  user_id?: number
  community_user_id?: string
  risk_score?: number
  risk_level?: string
  reason?: string
  updated_at?: number
}

type AgentGroupState = {
  name: string
  ratio: number
  available: boolean
  models: string[]
  healthy_channels: number
  disabled_channels: number
  recent_error_count: number
}

type AgentSiteState = {
  site_id: string
  site_name: string
  system_name: string
  community?: Record<string, unknown>
  bots?: Record<string, unknown>
  channels?: {
    total?: number
    enabled?: number
    manually_disabled?: number
    auto_disabled?: number
    avg_response_ms?: number
  }
  groups?: AgentGroupState[]
  recent_errors?: Array<{
    id?: number
    created_at?: number
    content?: string
    model_name?: string
    group?: string
    channel_id?: number
    request_id?: string
  }>
}

type AgentTool = {
  name: string
  title: string
  category: string
  description: string
  approval_required: boolean
  risk_level: string
}

type AgentDashboard = {
  setting?: Record<string, unknown>
  site_state?: AgentSiteState
  budgets?: AgentBudgetPool[]
  actions?: AgentAction[]
  tasks?: AgentTask[]
  approvals?: AgentApproval[]
  risks?: AgentRisk[]
  tools?: AgentTool[]
}

const imageApiBaseUrlSchema = z.string().refine((value) => {
  if (!value) return true
  try {
    const parsed = new URL(value)
    return (
      ['http:', 'https:'].includes(parsed.protocol) &&
      !parsed.username &&
      !parsed.password
    )
  } catch {
    return false
  }
}, 'Enter an HTTP(S) URL without embedded credentials')

const imageSizeSchema = z.string().refine((value) => {
  if (value === 'auto') return true
  const match = /^(\d+)x(\d+)$/.exec(value)
  if (!match) return false
  const width = Number(match[1])
  const height = Number(match[2])
  return width >= 256 && width <= 4096 && height >= 256 && height <= 4096
}, 'Use auto or WIDTHxHEIGHT with dimensions from 256 to 4096')

const agentSettingsSchema = z.object({
  enabled: z.boolean(),
  siteId: z.string().min(1),
  siteName: z.string().min(1),
  publicBaseUrl: z.string().url().optional().or(z.literal('')),
  apiBaseUrl: z.string().url().optional().or(z.literal('')),
  llmProvider: z.string().min(1),
  llmModel: z.string().min(1),
  llmBaseUrl: z.string().url().optional().or(z.literal('')),
  llmApiKey: z.string().optional(),
  plannerProvider: z.string().min(1),
  hermesBaseUrl: z.string().url().optional().or(z.literal('')),
  hermesApiKey: z.string().optional(),
  imageGenerationEnabled: z.boolean(),
  imageApiBaseUrl: imageApiBaseUrlSchema,
  imageApiKey: z.string().optional(),
  imageModel: z.string().min(1).max(128),
  imageSize: imageSizeSchema,
  imageTimeoutSeconds: z.coerce.number().min(1).max(600),
  imageRetryLimit: z.coerce.number().int().min(1).max(5),
  imageRetryBaseDelaySeconds: z.coerce.number().min(0).max(300),
  imageRetryMaxDelaySeconds: z.coerce.number().min(0).max(300),
  imageCooldownSeconds: z.coerce.number().int().min(0).max(86400),
  imageRequireBind: z.boolean(),
  directorEnabled: z.boolean(),
  communityEnabled: z.boolean(),
  growthEnabled: z.boolean(),
  activityEnabled: z.boolean(),
  gameEnabled: z.boolean(),
  riskEnabled: z.boolean(),
  opsEnabled: z.boolean(),
  budgetEnabled: z.boolean(),
  autoExecuteLowRisk: z.boolean(),
  humanApprovalEnabled: z.boolean(),
  dailyBudgetQuota: z.coerce.number().int().min(0),
  growthBudgetQuota: z.coerce.number().int().min(0),
  activityBudgetQuota: z.coerce.number().int().min(0),
  gameBudgetQuota: z.coerce.number().int().min(0),
  opsCompBudgetQuota: z.coerce.number().int().min(0),
  communityBudgetQuota: z.coerce.number().int().min(0),
  singleActionLimitQuota: z.coerce.number().int().min(0),
  userDailyLimitQuota: z.coerce.number().int().min(0),
  approvalThresholdQuota: z.coerce.number().int().min(0),
  riskDenyThreshold: z.coerce.number().int().min(0).max(100),
  riskReviewThreshold: z.coerce.number().int().min(0).max(100),
  minMessageChars: z.coerce.number().int().min(0),
  minDistinctMessages: z.coerce.number().int().min(1),
  qqBotEnabled: z.boolean(),
  qqOneBotUrl: z.string().url().optional().or(z.literal('')),
  qqGroupId: z.string().optional(),
  qqAccessToken: z.string().optional(),
  tgBotEnabled: z.boolean(),
  tgBotToken: z.string().optional(),
  tgChatId: z.string().optional(),
  chatOpsEnabled: z.boolean(),
  chatOpsWebhookSecret: z.string().optional(),
  chatOpsAdminExternalIds: z.string().optional(),
  chatOpsCommandPrefixes: z.string().optional(),
  chatOpsAutoReply: z.boolean(),
  chatOpsAllowNaturalLanguage: z.boolean(),
  chatOpsRequireAdminForOps: z.boolean(),
  chatOpsTrustGroupAdmin: z.boolean(),
  legacyConfigImportEnabled: z.boolean(),
  legacyConfigImportReasons: z.string().optional(),
  communityRoomId: z.string().optional(),
  communityHost: z.string().url().optional().or(z.literal('')),
  systemPrompt: z.string(),
  siteKnowledge: z.string(),
  welcomeTemplate: z.string(),
  activityPolicy: z.string(),
  riskPolicy: z.string(),
})

const schema = agentSettingsSchema.superRefine((values, context) => {
  if (values.imageRetryMaxDelaySeconds < values.imageRetryBaseDelaySeconds) {
    context.addIssue({
      code: 'custom',
      path: ['imageRetryMaxDelaySeconds'],
      message: 'Maximum retry delay must be at least the initial delay',
    })
  }
})

type Values = z.infer<typeof schema>

type AgentNumberField =
  | 'dailyBudgetQuota'
  | 'growthBudgetQuota'
  | 'activityBudgetQuota'
  | 'gameBudgetQuota'
  | 'opsCompBudgetQuota'
  | 'communityBudgetQuota'
  | 'singleActionLimitQuota'
  | 'userDailyLimitQuota'
  | 'approvalThresholdQuota'
  | 'riskDenyThreshold'
  | 'riskReviewThreshold'
  | 'minMessageChars'
  | 'minDistinctMessages'

type DailyBudgetField = Extract<
  AgentNumberField,
  | 'dailyBudgetQuota'
  | 'growthBudgetQuota'
  | 'activityBudgetQuota'
  | 'gameBudgetQuota'
  | 'opsCompBudgetQuota'
  | 'communityBudgetQuota'
>

const dailyBudgetFields: Array<{
  name: DailyBudgetField
  poolType: string
  label: string
  description: string
}> = [
  {
    name: 'dailyBudgetQuota',
    poolType: 'daily',
    label: 'General daily rewards',
    description: 'Shared daily allowance for rewards without a dedicated pool.',
  },
  {
    name: 'growthBudgetQuota',
    poolType: 'growth',
    label: 'Registration and invitation rewards',
    description: 'Daily allowance for registration and invitation rewards.',
  },
  {
    name: 'activityBudgetQuota',
    poolType: 'activity',
    label: 'Check-in and activity rewards',
    description:
      'Daily allowance used by QQ group check-in and site activities.',
  },
  {
    name: 'gameBudgetQuota',
    poolType: 'game',
    label: 'Game rewards',
    description: 'Daily allowance for game payouts.',
  },
  {
    name: 'opsCompBudgetQuota',
    poolType: 'ops_comp',
    label: 'Service compensation',
    description: 'Daily allowance for administrator-approved compensation.',
  },
  {
    name: 'communityBudgetQuota',
    poolType: 'community',
    label: 'Community rewards',
    description: 'Daily allowance for community participation rewards.',
  },
]

function isDailyBudgetField(name: keyof Values): name is DailyBudgetField {
  return dailyBudgetFields.some((field) => field.name === name)
}

type AgentTextAreaField =
  | 'systemPrompt'
  | 'siteKnowledge'
  | 'welcomeTemplate'
  | 'activityPolicy'
  | 'riskPolicy'

const optionKeyMap: Record<keyof Values, string> = {
  enabled: 'agent_setting.enabled',
  siteId: 'agent_setting.site_id',
  siteName: 'agent_setting.site_name',
  publicBaseUrl: 'agent_setting.public_base_url',
  apiBaseUrl: 'agent_setting.api_base_url',
  llmProvider: 'agent_setting.llm_provider',
  llmModel: 'agent_setting.llm_model',
  llmBaseUrl: 'agent_setting.llm_base_url',
  llmApiKey: 'agent_setting.llm_api_key',
  plannerProvider: 'agent_setting.planner_provider',
  hermesBaseUrl: 'agent_setting.hermes_base_url',
  hermesApiKey: 'agent_setting.hermes_api_key',
  imageGenerationEnabled: 'agent_setting.image_generation_enabled',
  imageApiBaseUrl: 'agent_setting.image_api_base_url',
  imageApiKey: 'agent_setting.image_api_key',
  imageModel: 'agent_setting.image_model',
  imageSize: 'agent_setting.image_size',
  imageTimeoutSeconds: 'agent_setting.image_timeout_seconds',
  imageRetryLimit: 'agent_setting.image_retry_limit',
  imageRetryBaseDelaySeconds: 'agent_setting.image_retry_base_delay_seconds',
  imageRetryMaxDelaySeconds: 'agent_setting.image_retry_max_delay_seconds',
  imageCooldownSeconds: 'agent_setting.image_cooldown_seconds',
  imageRequireBind: 'agent_setting.image_require_bind',
  directorEnabled: 'agent_setting.director_enabled',
  communityEnabled: 'agent_setting.community_enabled',
  growthEnabled: 'agent_setting.growth_enabled',
  activityEnabled: 'agent_setting.activity_enabled',
  gameEnabled: 'agent_setting.game_enabled',
  riskEnabled: 'agent_setting.risk_enabled',
  opsEnabled: 'agent_setting.ops_enabled',
  budgetEnabled: 'agent_setting.budget_enabled',
  autoExecuteLowRisk: 'agent_setting.auto_execute_low_risk',
  humanApprovalEnabled: 'agent_setting.human_approval_enabled',
  dailyBudgetQuota: 'agent_setting.daily_budget_quota',
  growthBudgetQuota: 'agent_setting.growth_budget_quota',
  activityBudgetQuota: 'agent_setting.activity_budget_quota',
  gameBudgetQuota: 'agent_setting.game_budget_quota',
  opsCompBudgetQuota: 'agent_setting.ops_comp_budget_quota',
  communityBudgetQuota: 'agent_setting.community_budget_quota',
  singleActionLimitQuota: 'agent_setting.single_action_limit_quota',
  userDailyLimitQuota: 'agent_setting.user_daily_limit_quota',
  approvalThresholdQuota: 'agent_setting.approval_threshold_quota',
  riskDenyThreshold: 'agent_setting.risk_deny_threshold',
  riskReviewThreshold: 'agent_setting.risk_review_threshold',
  minMessageChars: 'agent_setting.min_message_chars',
  minDistinctMessages: 'agent_setting.min_distinct_messages',
  qqBotEnabled: 'agent_setting.qq_bot_enabled',
  qqOneBotUrl: 'agent_setting.qq_onebot_url',
  qqGroupId: 'agent_setting.qq_group_id',
  qqAccessToken: 'agent_setting.qq_access_token',
  tgBotEnabled: 'agent_setting.tg_bot_enabled',
  tgBotToken: 'agent_setting.tg_bot_token',
  tgChatId: 'agent_setting.tg_chat_id',
  chatOpsEnabled: 'agent_setting.chatops_enabled',
  chatOpsWebhookSecret: 'agent_setting.chatops_webhook_secret',
  chatOpsAdminExternalIds: 'agent_setting.chatops_admin_external_ids',
  chatOpsCommandPrefixes: 'agent_setting.chatops_command_prefixes',
  chatOpsAutoReply: 'agent_setting.chatops_auto_reply',
  chatOpsAllowNaturalLanguage: 'agent_setting.chatops_allow_natural_language',
  chatOpsRequireAdminForOps: 'agent_setting.chatops_require_admin_for_ops',
  chatOpsTrustGroupAdmin: 'agent_setting.chatops_trust_group_admin',
  legacyConfigImportEnabled: 'agent_setting.legacy_config_import_enabled',
  legacyConfigImportReasons: 'agent_setting.legacy_config_import_reasons',
  communityRoomId: 'agent_setting.community_room_id',
  communityHost: 'agent_setting.community_host',
  systemPrompt: 'agent_setting.system_prompt',
  siteKnowledge: 'agent_setting.site_knowledge',
  welcomeTemplate: 'agent_setting.welcome_template',
  activityPolicy: 'agent_setting.activity_policy',
  riskPolicy: 'agent_setting.risk_policy',
}

const sensitiveFields: Array<keyof Values> = [
  'llmApiKey',
  'hermesApiKey',
  'imageApiKey',
  'qqAccessToken',
  'tgBotToken',
  'chatOpsWebhookSecret',
]

function buildDefaults(defaultValues: Props['defaultValues']): Values {
  return {
    enabled:
      readConfiguredRobotBool(defaultValues['agent_setting.enabled']) ?? false,
    siteId: readConfiguredRobotText(defaultValues['agent_setting.site_id']),
    siteName: readConfiguredRobotText(defaultValues['agent_setting.site_name']),
    publicBaseUrl: String(defaultValues['agent_setting.public_base_url'] || ''),
    apiBaseUrl: String(defaultValues['agent_setting.api_base_url'] || ''),
    llmProvider: readConfiguredRobotText(
      defaultValues['agent_setting.llm_provider']
    ),
    llmModel: readConfiguredRobotText(defaultValues['agent_setting.llm_model']),
    llmBaseUrl: String(defaultValues['agent_setting.llm_base_url'] || ''),
    llmApiKey: '',
    plannerProvider: readConfiguredRobotText(
      defaultValues['agent_setting.planner_provider']
    ),
    hermesBaseUrl: String(defaultValues['agent_setting.hermes_base_url'] || ''),
    hermesApiKey: '',
    imageGenerationEnabled:
      readConfiguredRobotBool(
        defaultValues['agent_setting.image_generation_enabled']
      ) ?? true,
    imageApiBaseUrl: String(
      defaultValues['agent_setting.image_api_base_url'] ||
        'https://api.acica.top/v1'
    ),
    imageApiKey: '',
    imageModel: String(
      defaultValues['agent_setting.image_model'] || 'gpt-image-2'
    ),
    imageSize: String(defaultValues['agent_setting.image_size'] || '1024x1024'),
    imageTimeoutSeconds: readConfiguredRobotNumber(
      defaultValues['agent_setting.image_timeout_seconds'],
      180
    ),
    imageRetryLimit: readConfiguredRobotNumber(
      defaultValues['agent_setting.image_retry_limit'],
      2
    ),
    imageRetryBaseDelaySeconds: readConfiguredRobotNumber(
      defaultValues['agent_setting.image_retry_base_delay_seconds'],
      3
    ),
    imageRetryMaxDelaySeconds: readConfiguredRobotNumber(
      defaultValues['agent_setting.image_retry_max_delay_seconds'],
      15
    ),
    imageCooldownSeconds: readConfiguredRobotNumber(
      defaultValues['agent_setting.image_cooldown_seconds'],
      45
    ),
    imageRequireBind:
      readConfiguredRobotBool(
        defaultValues['agent_setting.image_require_bind']
      ) ?? false,
    directorEnabled:
      readConfiguredRobotBool(
        defaultValues['agent_setting.director_enabled']
      ) ?? false,
    communityEnabled:
      readConfiguredRobotBool(
        defaultValues['agent_setting.community_enabled']
      ) ?? false,
    growthEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.growth_enabled']) ??
      false,
    activityEnabled:
      readConfiguredRobotBool(
        defaultValues['agent_setting.activity_enabled']
      ) ?? false,
    gameEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.game_enabled']) ??
      false,
    riskEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.risk_enabled']) ??
      false,
    opsEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.ops_enabled']) ??
      false,
    budgetEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.budget_enabled']) ??
      false,
    autoExecuteLowRisk:
      readConfiguredRobotBool(
        defaultValues['agent_setting.auto_execute_low_risk']
      ) ?? false,
    humanApprovalEnabled:
      readConfiguredRobotBool(
        defaultValues['agent_setting.human_approval_enabled']
      ) ?? false,
    dailyBudgetQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.daily_budget_quota']
    ),
    growthBudgetQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.growth_budget_quota']
    ),
    activityBudgetQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.activity_budget_quota']
    ),
    gameBudgetQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.game_budget_quota']
    ),
    opsCompBudgetQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.ops_comp_budget_quota']
    ),
    communityBudgetQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.community_budget_quota']
    ),
    singleActionLimitQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.single_action_limit_quota']
    ),
    userDailyLimitQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.user_daily_limit_quota']
    ),
    approvalThresholdQuota: readConfiguredRobotNumber(
      defaultValues['agent_setting.approval_threshold_quota']
    ),
    riskDenyThreshold: readConfiguredRobotNumber(
      defaultValues['agent_setting.risk_deny_threshold']
    ),
    riskReviewThreshold: readConfiguredRobotNumber(
      defaultValues['agent_setting.risk_review_threshold']
    ),
    minMessageChars: readConfiguredRobotNumber(
      defaultValues['agent_setting.min_message_chars']
    ),
    minDistinctMessages: readConfiguredRobotNumber(
      defaultValues['agent_setting.min_distinct_messages']
    ),
    qqBotEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.qq_bot_enabled']) ??
      false,
    qqOneBotUrl: String(defaultValues['agent_setting.qq_onebot_url'] || ''),
    qqGroupId: String(defaultValues['agent_setting.qq_group_id'] || ''),
    qqAccessToken: '',
    tgBotEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.tg_bot_enabled']) ??
      false,
    tgBotToken: '',
    tgChatId: String(defaultValues['agent_setting.tg_chat_id'] || ''),
    chatOpsEnabled:
      readConfiguredRobotBool(defaultValues['agent_setting.chatops_enabled']) ??
      false,
    chatOpsWebhookSecret: '',
    chatOpsAdminExternalIds: String(
      defaultValues['agent_setting.chatops_admin_external_ids'] || ''
    ),
    chatOpsCommandPrefixes: String(
      defaultValues['agent_setting.chatops_command_prefixes'] || ''
    ),
    chatOpsAutoReply:
      readConfiguredRobotBool(
        defaultValues['agent_setting.chatops_auto_reply']
      ) ?? false,
    chatOpsAllowNaturalLanguage:
      readConfiguredRobotBool(
        defaultValues['agent_setting.chatops_allow_natural_language']
      ) ?? false,
    chatOpsRequireAdminForOps:
      readConfiguredRobotBool(
        defaultValues['agent_setting.chatops_require_admin_for_ops']
      ) ?? false,
    chatOpsTrustGroupAdmin:
      readConfiguredRobotBool(
        defaultValues['agent_setting.chatops_trust_group_admin']
      ) ?? false,
    legacyConfigImportEnabled:
      readConfiguredRobotBool(
        defaultValues['agent_setting.legacy_config_import_enabled']
      ) ?? false,
    legacyConfigImportReasons: String(
      defaultValues['agent_setting.legacy_config_import_reasons'] || ''
    ),
    communityRoomId: String(
      defaultValues['agent_setting.community_room_id'] || ''
    ),
    communityHost: String(defaultValues['agent_setting.community_host'] || ''),
    systemPrompt: String(defaultValues['agent_setting.system_prompt'] || ''),
    siteKnowledge: String(defaultValues['agent_setting.site_knowledge'] || ''),
    welcomeTemplate: String(
      defaultValues['agent_setting.welcome_template'] || ''
    ),
    activityPolicy: String(
      defaultValues['agent_setting.activity_policy'] || ''
    ),
    riskPolicy: String(defaultValues['agent_setting.risk_policy'] || ''),
  }
}

function formatQuota(value?: number) {
  const quota = Number(value ?? 0)
  return `${quota.toLocaleString()} (${(quota / 500000).toFixed(2)} USD)`
}

function readDailyBudgetValues(values: Values) {
  return Object.fromEntries(
    dailyBudgetFields.map(({ name }) => [name, Number(values[name] ?? 0)])
  ) as Record<DailyBudgetField, number>
}

function findBudgetPool(
  overview: OpsRewardFundOverview | null,
  poolType: string
) {
  return overview?.budget_pools_today?.find(
    (pool) => pool.pool_type === poolType
  )
}

function formatTime(value?: number) {
  return value ? new Date(value * 1000).toLocaleString() : '-'
}

type Translate = (key: string, options?: Record<string, unknown>) => string

function translateRobotStatus(status: string | undefined, t: Translate) {
  if (!status) return '-'
  const normalized = status.toLowerCase()
  const map: Record<string, string> = {
    completed: t('Completed'),
    failed: t('Failed'),
    pending: t('Pending'),
    denied: t('Denied'),
    ignored: t('Ignored'),
    approved: t('Approved'),
    rejected: t('Rejected'),
    running: t('Running'),
    queued: t('Queued'),
  }
  return map[normalized] || status
}

function translateRiskLevel(level: string | undefined, t: Translate) {
  if (!level) return '-'
  const normalized = level.toLowerCase()
  const map: Record<string, string> = {
    low: t('Low'),
    medium: t('Medium'),
    high: t('High'),
    critical: t('Critical'),
  }
  return map[normalized] || level
}

function translateRobotSource(source: string | undefined, t: Translate) {
  if (!source) return '-'
  const normalized = source.toLowerCase()
  const map: Record<string, string> = {
    qq: t('QQ'),
    tg: t('TG'),
    community: t('Community'),
    system: t('System'),
    web: t('Web'),
  }
  return map[normalized] || source
}

function translateRobotActionType(
  actionType: string | undefined,
  t: Translate
) {
  if (!actionType) return '-'
  const normalized = actionType.toLowerCase()
  const map: Record<string, string> = {
    'reward.settlement.batch': t('Reward settlement batch'),
    'reward.grant.small': t('Small reward grant'),
    'fund.report.read': t('Read fund report'),
    'budget.check': t('Budget check'),
    'fund.topup': t('Fund top-up'),
    'message.tg.send': t('TG message send'),
    game_admin_budget_view: t('Game admin budget view'),
    admin_ui: t('Admin UI'),
  }
  return map[normalized] || actionType
}

function translateRobotActionReason(reason: string | undefined, t: Translate) {
  if (!reason) return '-'
  const normalized = reason.toLowerCase()
  if (normalized === 'game action reward.settlement.batch') {
    return t('Game action: reward settlement batch')
  }
  if (normalized === 'system smoke') return t('System smoke')
  return reason
}

function translateBudgetPoolName(poolType: string | undefined, t: Translate) {
  if (!poolType) return '-'
  const normalized = poolType.toLowerCase()
  const map: Record<string, string> = {
    activity: t('Activity pool'),
    community: t('Community pool'),
    daily: t('Daily pool'),
    game: t('Game pool'),
    growth: t('Growth pool'),
    ops_comp: t('Operations compensation pool'),
  }
  return map[normalized] || poolType
}

type AgentFormProps = {
  form: UseFormReturn<Values>
  t: Translate
}

function formatErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message
  if (typeof error === 'string' && error.trim()) return error
  return fallback
}

function readConfiguredRobotText(value: unknown) {
  if (value === null || value === undefined) return ''
  return String(value).trim()
}

function readConfiguredRobotBool(value: unknown): boolean | null {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  if (typeof value !== 'string') return null
  const normalized = value.trim().toLowerCase()
  if (!normalized) return null
  if (['true', '1', 'yes', 'on'].includes(normalized)) return true
  if (['false', '0', 'no', 'off'].includes(normalized)) return false
  return null
}

function readConfiguredRobotNumber(value: unknown, fallback = 0) {
  if (value === null || value === undefined || value === '') return fallback
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function buildRobotConfiguredView(defaultValues: Props['defaultValues']) {
  return {
    enabled: readOpsSavedBool(defaultValues, 'agent_setting.enabled'),
    siteId: readOpsSavedText(defaultValues, 'agent_setting.site_id'),
    siteName: readOpsSavedText(defaultValues, 'agent_setting.site_name'),
    publicBaseUrl: readOpsSavedText(
      defaultValues,
      'agent_setting.public_base_url'
    ),
    apiBaseUrl: readOpsSavedText(defaultValues, 'agent_setting.api_base_url'),
    llmProvider: readOpsSavedText(defaultValues, 'agent_setting.llm_provider'),
    llmModel: readOpsSavedText(defaultValues, 'agent_setting.llm_model'),
    plannerProvider: readOpsSavedText(
      defaultValues,
      'agent_setting.planner_provider'
    ),
    imageGenerationEnabled: readOpsSavedBool(
      defaultValues,
      'agent_setting.image_generation_enabled'
    ),
    imageApiBaseUrl: readOpsSavedText(
      defaultValues,
      'agent_setting.image_api_base_url'
    ),
    imageModel: readOpsSavedText(defaultValues, 'agent_setting.image_model'),
    imageSize: readOpsSavedText(defaultValues, 'agent_setting.image_size'),
    qqBotEnabled: readOpsSavedBool(
      defaultValues,
      'agent_setting.qq_bot_enabled'
    ),
    qqOneBotUrl: readOpsSavedText(defaultValues, 'agent_setting.qq_onebot_url'),
    qqGroupIds: readOpsSavedList(defaultValues, 'agent_setting.qq_group_id'),
    tgBotEnabled: readOpsSavedBool(
      defaultValues,
      'agent_setting.tg_bot_enabled'
    ),
    tgChatIds: readOpsSavedList(defaultValues, 'agent_setting.tg_chat_id'),
    chatOpsEnabled: readOpsSavedBool(
      defaultValues,
      'agent_setting.chatops_enabled'
    ),
    chatOpsAdminExternalIds: readOpsSavedList(
      defaultValues,
      'agent_setting.chatops_admin_external_ids'
    ),
    chatOpsCommandPrefixes: readOpsSavedList(
      defaultValues,
      'agent_setting.chatops_command_prefixes'
    ),
    legacyConfigImportEnabled: readOpsSavedBool(
      defaultValues,
      'agent_setting.legacy_config_import_enabled'
    ),
    legacyConfigImportReasons: readOpsSavedList(
      defaultValues,
      'agent_setting.legacy_config_import_reasons'
    ),
  }
}

function FieldGroup({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <Card className='overflow-hidden'>
      <CardHeader className='bg-muted/30 border-b py-4'>
        <CardTitle className='text-base'>{title}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </CardHeader>
      <CardContent className='grid gap-4 p-4 lg:grid-cols-2'>
        {children}
      </CardContent>
    </Card>
  )
}

function AgentPolicyMatrix({
  defaultValues,
  t,
}: {
  defaultValues: Props['defaultValues']
  t: Translate
}) {
  const access = buildOpsAccessSnapshot(defaultValues)
  const gate = buildOpsGateSnapshot(defaultValues)
  const primaryPlatform = access.primaryPlatform
    ? getPrimaryPlatformDisplayName(access.primaryPlatform, t)
    : t('Not assigned yet')
  const primaryGroups =
    access.primaryGroupIds.length > 0
      ? access.primaryGroupIds
      : [t('Not assigned yet')]
  const communityGroups =
    access.communityGroupIds.length > 0
      ? access.communityGroupIds
      : [t('Not assigned yet')]
  const communityOnly =
    access.communityOnlyGroups.length > 0
      ? access.communityOnlyGroups
      : [t('Not assigned yet')]
  const fullAccess =
    access.fullAccessGroups.length > 0
      ? access.fullAccessGroups
      : [t('Not assigned yet')]

  const rows = [
    {
      id: 'primary',
      cells: [
        <div className='space-y-1' key='group'>
          <div className='font-medium'>{t('Primary field groups')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('{{platform}} groups that unlock the full site after binding.', {
              platform: primaryPlatform,
            })}
          </div>
        </div>,
        <div key='platform' className='space-y-1 text-sm'>
          <div>{primaryPlatform}</div>
          <div className='text-muted-foreground text-xs'>
            {primaryGroups.join(', ')}
          </div>
        </div>,
        <div key='access' className='space-y-1 text-sm'>
          <div>{fullAccess.join(', ')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Unlocked after primary-group binding')}
          </div>
        </div>,
        <div key='reward' className='space-y-1 text-sm'>
          <div>
            {access.blockTokenCreate
              ? t('Token create blocked')
              : t('Token create allowed')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Reward soft floor')}: {formatQuota(access.rewardSoftFloorQuota)}
          </div>
        </div>,
        <div key='status'>
          <OpsStatusBadge tone='success'>{t('Live')}</OpsStatusBadge>
        </div>,
      ],
    },
    {
      id: 'community',
      cells: [
        <div className='space-y-1' key='group'>
          <div className='font-medium'>{t('Community intake groups')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Community-only groups and room-gate fallback policy.')}
          </div>
        </div>,
        <div key='platform' className='space-y-1 text-sm'>
          <div>{t('Community')}</div>
          <div className='text-muted-foreground text-xs'>
            {communityGroups.join(', ')}
          </div>
        </div>,
        <div key='access' className='space-y-1 text-sm'>
          <div>{communityOnly.join(', ')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Room match mode')}: {gate.roomMatchMode || t('Not configured')}
          </div>
        </div>,
        <div key='reward' className='space-y-1 text-sm'>
          <div>
            {access.blockTokenCreate
              ? t('Community key only')
              : t('Community key creation enabled')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Required rooms')}:{' '}
            {gate.roomIds.join(', ') || t('Not assigned yet')}
          </div>
        </div>,
        <div key='status'>
          <OpsStatusBadge tone={gate.enabled ? 'info' : 'warning'}>
            {gate.enabled ? t('Strict gate') : t('Disabled')}
          </OpsStatusBadge>
        </div>,
      ],
    },
    {
      id: 'paid',
      cells: [
        <div className='space-y-1' key='group'>
          <div className='font-medium'>{t('Paid / manual override')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Packages and bypass groups that skip binding checks.')}
          </div>
        </div>,
        <div key='platform' className='space-y-1 text-sm'>
          <div>{t('User package')}</div>
          <div className='text-muted-foreground text-xs'>
            {access.paidUserGroups.join(', ') || t('Not assigned yet')}
          </div>
        </div>,
        <div key='access' className='space-y-1 text-sm'>
          <div>
            {access.paidBypassGroups.join(', ') || t('Not assigned yet')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Administrator bypass')}:{' '}
            {access.allowAdminBypass ? t('Enabled') : t('Disabled')}
          </div>
        </div>,
        <div key='reward' className='space-y-1 text-sm'>
          <div>
            {access.allowPaidBypass
              ? t('Paid bypass enabled')
              : t('Paid bypass disabled')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Daily site reward cap')}:{' '}
            {formatQuota(access.dailySiteRewardCap)}
          </div>
        </div>,
        <div key='status'>
          <OpsStatusBadge tone={access.allowPaidBypass ? 'warning' : 'neutral'}>
            {access.allowPaidBypass ? t('Requires audit') : t('Off')}
          </OpsStatusBadge>
        </div>,
      ],
    },
  ]

  return (
    <OpsDataTable
      title={t('Group policy matrix')}
      description={t(
        'Use the current access-control and community-gate settings as the source of truth for who can create keys, which groups unlock after each binding step, and when rewards must pause.'
      )}
      columns={[
        { key: 'group', label: t('Group') },
        { key: 'platform', label: t('Platform / IDs') },
        { key: 'access', label: t('Unlocked groups') },
        { key: 'reward', label: t('Token / reward policy') },
        { key: 'status', label: t('Status') },
      ]}
      rows={rows}
      emptyMessage={t('No group policy yet')}
      className='xl:col-span-3'
    />
  )
}

function AgentQuotaGuardPanel({
  dashboard,
  defaultValues,
  t,
}: {
  dashboard: AgentDashboard | null
  defaultValues: Props['defaultValues']
  t: Translate
}) {
  const access = buildOpsAccessSnapshot(defaultValues)
  const budgets = dashboard?.budgets ?? []
  const actions = dashboard?.actions ?? []
  const totalQuota = budgets.reduce(
    (sum, pool) => sum + Number(pool.total_quota ?? 0),
    0
  )
  const usedQuota = budgets.reduce(
    (sum, pool) => sum + Number(pool.used_quota ?? 0),
    0
  )
  const frozenQuota = budgets.reduce(
    (sum, pool) => sum + Number(pool.frozen_quota ?? 0),
    0
  )
  const approvalRequiredCount = actions.filter((item) =>
    Boolean(item.approval_required)
  ).length
  const highRiskCount = actions.filter(
    (item) => String(item.risk_level || '').toLowerCase() === 'high'
  ).length

  return (
    <OpsPanel
      title={t('Quota, reward, and approval guardrails')}
      description={t(
        'This view translates current runtime quota pools and approval rules into operator-facing fund health, reward floors, and manual-review pressure.'
      )}
      className='xl:col-span-1'
    >
      <div className='space-y-3'>
        <OpsInsightCard
          title={t('Fund availability')}
          value={formatQuota(Math.max(totalQuota - usedQuota - frozenQuota, 0))}
          description={t(
            'Available quota after subtracting used and frozen pools from the current dashboard snapshot.'
          )}
          badge={
            <OpsStatusBadge
              tone={
                totalQuota - usedQuota - frozenQuota <=
                access.rewardHardFloorQuota
                  ? 'danger'
                  : totalQuota - usedQuota - frozenQuota <=
                      access.rewardSoftFloorQuota
                    ? 'warning'
                    : 'success'
              }
            >
              {totalQuota - usedQuota - frozenQuota <=
              access.rewardHardFloorQuota
                ? t('Hard floor')
                : totalQuota - usedQuota - frozenQuota <=
                    access.rewardSoftFloorQuota
                  ? t('Soft floor')
                  : t('Healthy')}
            </OpsStatusBadge>
          }
        />
        <OpsInsightCard
          title={t('Manual review load')}
          value={`${approvalRequiredCount}/${actions.length || 0}`}
          description={t(
            'Actions currently marked as approval-required in the latest runtime payload.'
          )}
          badge={
            <OpsStatusBadge
              tone={approvalRequiredCount > 0 ? 'warning' : 'neutral'}
            >
              {t('Approvals')}
            </OpsStatusBadge>
          }
        />
        <OpsInsightCard
          title={t('High-risk actions')}
          value={highRiskCount}
          description={t(
            'Use this with the risk panel below to decide whether operator intervention is needed before restoring user access or enabling automation.'
          )}
          badge={
            <OpsStatusBadge tone={highRiskCount > 0 ? 'danger' : 'success'}>
              {highRiskCount > 0 ? t('Needs review') : t('Clear status')}
            </OpsStatusBadge>
          }
        />
      </div>
    </OpsPanel>
  )
}

function AgentRuntimeState({
  dashboard,
  defaultValues,
  t,
}: {
  dashboard: AgentDashboard | null
  defaultValues: Props['defaultValues']
  t: Translate
}) {
  const state = dashboard?.site_state
  return (
    <div className='space-y-4'>
      <OpsSurfaceGrid className='xl:grid-cols-[1.45fr_.95fr_1fr_1fr]'>
        <AgentPolicyMatrix defaultValues={defaultValues} t={t} />
        <AgentQuotaGuardPanel
          dashboard={dashboard}
          defaultValues={defaultValues}
          t={t}
        />
      </OpsSurfaceGrid>
      <Card className='border-primary/10 shadow-sm'>
        <CardHeader>
          <CardTitle>{t('Current bot status')}</CardTitle>
          <CardDescription>
            {t(
              'Read-only view of current connections, groups, reward balances, risks, pending reviews, and available actions.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-4 xl:grid-cols-2'>
          <AgentGroupsPanel groups={state?.groups ?? []} t={t} />
          <AgentBudgetPanel budgets={dashboard?.budgets ?? []} t={t} />
          <AgentActionsPanel actions={dashboard?.actions ?? []} t={t} />
          <AgentTasksPanel tasks={dashboard?.tasks ?? []} t={t} />
          <AgentRisksPanel risks={dashboard?.risks ?? []} t={t} />
        </CardContent>
      </Card>
    </div>
  )
}

function AgentGroupsPanel({
  groups,
  t,
}: {
  groups: AgentGroupState[]
  t: Translate
}) {
  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>
        {t('Groups and channel health')}
      </div>
      <div className='max-h-96 overflow-auto rounded-lg border'>
        {groups.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No group state yet')}
          </div>
        ) : (
          groups.map((group) => (
            <div
              key={group.name}
              className='grid gap-2 border-b p-3 text-sm last:border-b-0 md:grid-cols-[1fr_.6fr_.8fr_.8fr]'
            >
              <div className='min-w-0'>
                <div className='font-medium'>{group.name}</div>
                <div className='text-muted-foreground truncate'>
                  {group.models.slice(0, 5).join(', ') || '-'}
                </div>
              </div>
              <div>
                {t('Ratio')}: {group.ratio}
              </div>
              <div>
                {t('Healthy channels')}: {group.healthy_channels}
              </div>
              <div>
                {t('Errors')}: {group.recent_error_count}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function AgentBudgetPanel({
  budgets,
  t,
}: {
  budgets: AgentBudgetPool[]
  t: Translate
}) {
  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>{t('Reward balances')}</div>
      <div className='rounded-lg border'>
        {budgets.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No reward balance record yet')}
          </div>
        ) : (
          budgets.map((pool) => (
            <div
              key={`${pool.pool_type}-${pool.budget_date}`}
              className='grid gap-2 border-b p-3 text-sm last:border-b-0 md:grid-cols-[.8fr_1fr_1fr]'
            >
              <div className='font-medium'>
                {translateBudgetPoolName(pool.pool_type, t)}
              </div>
              <div>
                {t('Used')}: {formatQuota(pool.used_quota)}
              </div>
              <div>
                {t('Total')}: {formatQuota(pool.total_quota)}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function AgentActionsPanel({
  actions,
  t,
}: {
  actions: AgentAction[]
  t: Translate
}) {
  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>
        {t('Pending and recent actions')}
      </div>
      <div className='max-h-96 overflow-auto rounded-lg border'>
        {actions.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No helper action yet')}
          </div>
        ) : (
          actions.map((action) => (
            <div
              key={action.id}
              className='grid gap-2 border-b p-3 text-sm last:border-b-0 md:grid-cols-[1fr_.7fr_.7fr_.8fr]'
            >
              <div className='min-w-0'>
                <div className='truncate font-medium'>
                  {translateRobotActionType(action.action_type, t)}
                </div>
                <div className='text-muted-foreground truncate'>
                  {translateRobotActionReason(
                    action.reason || action.target_id || '-',
                    t
                  )}
                </div>
              </div>
              <div>
                {t('Risk')}: {translateRiskLevel(action.risk_level, t)}{' '}
                {action.risk_score ?? 0}
              </div>
              <div>
                {t('Status')}: {translateRobotStatus(action.status, t)}
              </div>
              <div>{formatTime(action.created_at)}</div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function AgentTasksPanel({ tasks, t }: { tasks: AgentTask[]; t: Translate }) {
  return (
    <div className='space-y-2 xl:col-span-2'>
      <div className='text-sm font-medium'>
        {t('Chat command handling tasks')}
      </div>
      <div className='max-h-96 overflow-auto rounded-lg border'>
        {tasks.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No Chat command handling task yet')}
          </div>
        ) : (
          tasks.map((task) => (
            <div
              key={task.id}
              className='grid gap-2 border-b p-3 text-sm last:border-b-0 md:grid-cols-[.6fr_.7fr_1fr_.7fr_.8fr]'
            >
              <div>
                {t('Source')}: {translateRobotSource(task.source, t)}
              </div>
              <div>
                {t('Issuer')}:{' '}
                {task.issuer_username || task.issuer_external_id || '-'}
              </div>
              <div className='truncate'>
                {t('Command')}: {task.command || task.text || '-'}
              </div>
              <div>
                {t('Task status')}: {translateRobotStatus(task.status, t)}
              </div>
              <div>{formatTime(task.created_at)}</div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function AgentRisksPanel({ risks, t }: { risks: AgentRisk[]; t: Translate }) {
  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>{t('Risk users')}</div>
      <div className='max-h-96 overflow-auto rounded-lg border'>
        {risks.length === 0 ? (
          <div className='text-muted-foreground p-3 text-sm'>
            {t('No risk profile yet')}
          </div>
        ) : (
          risks.map((risk) => (
            <div
              key={risk.id || risk.user_id}
              className='grid gap-2 border-b p-3 text-sm last:border-b-0 md:grid-cols-[.7fr_.7fr_1fr_.8fr]'
            >
              <div>
                {t('User ID')}: {risk.user_id}
              </div>
              <div>
                {translateRiskLevel(risk.risk_level, t)} {risk.risk_score}
              </div>
              <div className='truncate'>{risk.reason || '-'}</div>
              <div>{formatTime(risk.updated_at)}</div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function DailyRewardBudgetPanel({
  form,
  t,
  overview,
  onSave,
  onSaveAndReset,
  isSaving,
  isResetting,
}: AgentFormProps & {
  overview: OpsRewardFundOverview | null
  onSave: () => Promise<void>
  onSaveAndReset: () => Promise<void>
  isSaving: boolean
  isResetting: boolean
}) {
  const budgetDate = overview?.budget_pools_today?.[0]?.budget_date
  const isBusy = isSaving || isResetting

  return (
    <Form {...form}>
      <section
        data-ui-revision='agentops-daily-budget-r11-20260713'
        className='bg-card mt-4 overflow-hidden rounded-lg border'
      >
        <div className='flex flex-col gap-4 border-b px-5 py-4 xl:flex-row xl:items-start xl:justify-between'>
          <div className='flex min-w-0 items-start gap-3'>
            <div className='flex size-9 shrink-0 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'>
              <Coins className='size-4' aria-hidden='true' />
            </div>
            <div className='space-y-1'>
              <h3 className='text-base font-semibold'>
                {t('Daily reward budgets')}
              </h3>
              <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
                {t(
                  'Set the amount available to each reward type every day. The system opens the new daily budget automatically and keeps a separate audit record.'
                )}
              </p>
            </div>
          </div>
          <div className='grid min-w-0 grid-cols-2 gap-x-6 gap-y-2 text-sm xl:min-w-96'>
            <CardLine
              label={t('Reward account balance')}
              value={formatQuota(overview?.fund?.balance_quota)}
            />
            <CardLine
              label={t('Budget date')}
              value={budgetDate || t('Not loaded')}
            />
          </div>
        </div>

        <div className='grid gap-3 p-4 md:grid-cols-2 xl:grid-cols-3'>
          {dailyBudgetFields.map(({ name, poolType, label, description }) => {
            const pool = findBudgetPool(overview, poolType)
            const state = pool?.degrade_state
            const tone =
              state === 'exhausted'
                ? 'danger'
                : state === 'low'
                  ? 'warning'
                  : pool
                    ? 'success'
                    : 'neutral'
            return (
              <div
                key={name}
                className='bg-background flex min-h-64 flex-col rounded-lg border p-4'
              >
                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <div className='font-medium'>{t(label)}</div>
                    <div className='text-muted-foreground mt-1 text-sm leading-5'>
                      {t(description)}
                    </div>
                  </div>
                  <OpsStatusBadge tone={tone}>
                    {pool
                      ? t(
                          state === 'low'
                            ? 'Low'
                            : state === 'exhausted'
                              ? 'Exhausted'
                              : 'Ready'
                        )
                      : t('Not opened')}
                  </OpsStatusBadge>
                </div>

                <FormField
                  control={form.control}
                  name={name}
                  render={({ field }) => (
                    <FormItem className='mt-4'>
                      <FormLabel>{t('Configured daily amount')}</FormLabel>
                      <FormControl>
                        <Input type='number' min={0} {...field} />
                      </FormControl>
                      <FormDescription>
                        {formatQuota(Number(field.value))}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <div className='mt-auto grid grid-cols-3 gap-2 border-t pt-4 text-sm'>
                  <CardLine
                    label={t('Used')}
                    value={Number(pool?.used_quota ?? 0).toLocaleString()}
                  />
                  <CardLine
                    label={t('Frozen')}
                    value={Number(pool?.frozen_quota ?? 0).toLocaleString()}
                  />
                  <CardLine
                    label={t('Remaining')}
                    value={Number(pool?.remaining_quota ?? 0).toLocaleString()}
                  />
                </div>
              </div>
            )
          })}
        </div>

        <div className='bg-muted/20 flex flex-col gap-3 border-t px-5 py-4 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-muted-foreground max-w-2xl text-sm leading-6'>
            {t(
              'Saving changes affects the next automatic reset. Use Save and reset today when the new amounts must take effect immediately.'
            )}
          </p>
          <div className='flex flex-wrap gap-2 sm:justify-end'>
            <Button
              type='button'
              variant='outline'
              onClick={() => void onSave()}
              disabled={isBusy}
            >
              <Save className='size-4' aria-hidden='true' />
              {isSaving ? t('Saving') : t('Save budgets')}
            </Button>
            <Button
              type='button'
              onClick={() => void onSaveAndReset()}
              disabled={isBusy}
            >
              <RotateCcw
                className={`size-4 ${isResetting ? 'animate-spin' : ''}`}
                aria-hidden='true'
              />
              {isResetting ? t('Resetting') : t('Save and reset today')}
            </Button>
          </div>
        </div>
      </section>
    </Form>
  )
}

function AgentSettingsForm({
  form,
  t,
  onSubmit,
  isSaving,
}: AgentFormProps & {
  onSubmit: (values: Values) => Promise<void>
  isSaving: boolean
}) {
  return (
    <Form {...form}>
      <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
        <SettingsPageFormActions
          onSave={form.handleSubmit(onSubmit)}
          onReset={() => form.reset()}
          isSaving={isSaving}
          isSaveDisabled={!form.formState.isDirty}
          isResetDisabled={!form.formState.isDirty}
          saveLabel={t('Save robot settings')}
          resetLabel={t('Discard unsaved changes')}
          inline
          className='bg-background/95 sticky top-16 z-20 rounded-lg border p-3 shadow-sm supports-[backdrop-filter]:backdrop-blur'
        />
        <AgentBaseConfigurationFields form={form} t={t} />
        <AgentCapabilityFields form={form} t={t} />
        <AgentBudgetRiskFields form={form} t={t} />
        <AgentConnectorFields form={form} t={t} />
        <AgentImageGenerationFields form={form} t={t} />
        <AgentChatOpsFields form={form} t={t} />
        <AgentLegacySyncFields form={form} t={t} />
        <AgentPromptFields form={form} t={t} />
      </SettingsForm>
    </Form>
  )
}

type AgentStringFieldName =
  | 'siteId'
  | 'siteName'
  | 'publicBaseUrl'
  | 'apiBaseUrl'
  | 'llmProvider'
  | 'llmModel'
  | 'llmBaseUrl'
  | 'llmApiKey'
  | 'plannerProvider'
  | 'hermesBaseUrl'
  | 'hermesApiKey'
  | 'imageApiBaseUrl'
  | 'imageApiKey'
  | 'imageModel'
  | 'imageSize'
  | 'communityHost'
  | 'communityRoomId'
  | 'qqOneBotUrl'
  | 'qqGroupId'
  | 'qqAccessToken'
  | 'tgChatId'
  | 'tgBotToken'
  | 'chatOpsWebhookSecret'
  | 'chatOpsAdminExternalIds'
  | 'chatOpsCommandPrefixes'
  | 'legacyConfigImportReasons'

type AgentBooleanFieldName =
  | 'enabled'
  | 'qqBotEnabled'
  | 'tgBotEnabled'
  | 'chatOpsEnabled'
  | 'chatOpsAutoReply'
  | 'chatOpsAllowNaturalLanguage'
  | 'chatOpsRequireAdminForOps'
  | 'chatOpsTrustGroupAdmin'
  | 'legacyConfigImportEnabled'
  | 'directorEnabled'
  | 'communityEnabled'
  | 'growthEnabled'
  | 'activityEnabled'
  | 'gameEnabled'
  | 'riskEnabled'
  | 'opsEnabled'
  | 'budgetEnabled'
  | 'autoExecuteLowRisk'
  | 'humanApprovalEnabled'
  | 'imageGenerationEnabled'
  | 'imageRequireBind'

function AgentSwitchField({
  form,
  t,
  name,
  label,
  description,
  className,
}: AgentFormProps & {
  name: AgentBooleanFieldName
  label: string
  description: string
  className?: string
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <SettingsSwitchItem className={className}>
          <SettingsSwitchContent>
            <FormLabel>{t(label)}</FormLabel>
            <FormDescription>{t(description)}</FormDescription>
          </SettingsSwitchContent>
          <FormControl>
            <Switch
              checked={Boolean(field.value)}
              onCheckedChange={field.onChange}
            />
          </FormControl>
        </SettingsSwitchItem>
      )}
    />
  )
}

function AgentInputField({
  form,
  t,
  name,
  label,
  description,
  placeholder,
  secret = false,
}: AgentFormProps & {
  name: AgentStringFieldName
  label: string
  description?: string
  placeholder?: string
  secret?: boolean
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{t(label)}</FormLabel>
          <FormControl>
            <Input
              type={secret ? 'password' : 'text'}
              autoComplete={secret ? 'off' : undefined}
              placeholder={placeholder}
              {...field}
            />
          </FormControl>
          {description ? (
            <FormDescription>{t(description)}</FormDescription>
          ) : null}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function AgentBaseConfigurationFields({ form, t }: AgentFormProps) {
  return (
    <FieldGroup
      title={t('Robot base configuration')}
      description={t(
        'Site identity, API endpoints, and LLM runtime used by this isolated agent.'
      )}
    >
      <AgentSwitchField
        form={form}
        t={t}
        name='enabled'
        label='Enable helper platform'
        description='Turns on the site-local helper control plane; individual capabilities remain separately configurable.'
        className='lg:col-span-2'
      />
      <AgentInputField
        form={form}
        t={t}
        name='siteId'
        label='Site ID'
        description='Isolation key for budgets, memory, events, actions, and risk profiles'
      />
      <AgentInputField form={form} t={t} name='siteName' label='Site name' />
      <AgentInputField
        form={form}
        t={t}
        name='publicBaseUrl'
        label='Public site URL'
      />
      <AgentInputField
        form={form}
        t={t}
        name='apiBaseUrl'
        label='API base URL'
      />
      <AgentInputField
        form={form}
        t={t}
        name='llmProvider'
        label='AI provider'
      />
      <AgentInputField form={form} t={t} name='llmModel' label='AI model' />
      <AgentInputField
        form={form}
        t={t}
        name='llmBaseUrl'
        label='AI service address'
      />
      <AgentInputField
        form={form}
        t={t}
        name='llmApiKey'
        label='AI service key'
        description='Leave blank to keep the current secret'
        secret
      />
      <AgentInputField
        form={form}
        t={t}
        name='plannerProvider'
        label='Task planner provider'
        placeholder='builtin'
        description='Optional external planner provider such as builtin, hermes, or openclaw. Business configuration stays in admin system.'
      />
      <AgentInputField
        form={form}
        t={t}
        name='hermesBaseUrl'
        label='Advanced service address'
      />
      <AgentInputField
        form={form}
        t={t}
        name='hermesApiKey'
        label='Advanced service key'
        description='Leave blank to keep the current secret'
        secret
      />
    </FieldGroup>
  )
}

function AgentCapabilityFields({ form, t }: AgentFormProps) {
  const names: Array<keyof Values> = [
    'directorEnabled',
    'communityEnabled',
    'growthEnabled',
    'activityEnabled',
    'gameEnabled',
    'riskEnabled',
    'opsEnabled',
    'budgetEnabled',
    'autoExecuteLowRisk',
    'humanApprovalEnabled',
  ]
  return (
    <FieldGroup
      title={t('Robot capabilities')}
      description={t(
        'Turn each helper on or off separately so community, activity, game, risk, and budget work do not affect each other.'
      )}
    >
      {names.map((name) => (
        <FormField
          key={name}
          control={form.control}
          name={name}
          render={({ field }) => (
            <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormLabel>{t(String(name))}</FormLabel>
                <FormDescription>{t(`${name} description`)}</FormDescription>
              </SettingsSwitchContent>
              <FormControl>
                <Switch
                  checked={Boolean(field.value)}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </SettingsSwitchItem>
          )}
        />
      ))}
    </FieldGroup>
  )
}

function AgentBudgetRiskFields({ form, t }: AgentFormProps) {
  const names: AgentNumberField[] = [
    'singleActionLimitQuota',
    'userDailyLimitQuota',
    'approvalThresholdQuota',
    'riskDenyThreshold',
    'riskReviewThreshold',
    'minMessageChars',
    'minDistinctMessages',
  ]
  return (
    <FieldGroup
      title={t('Reward safety limits')}
      description={t(
        'Set per-action limits, approval thresholds, and anti-spam checks. Daily reward amounts are managed in the budget section above.'
      )}
    >
      {names.map((name) => (
        <FormField
          key={name}
          control={form.control}
          name={name}
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t(name)}</FormLabel>
              <FormControl>
                <Input type='number' min={0} {...field} />
              </FormControl>
              <FormDescription>
                {formatQuota(Number(field.value))} · {t(`${name} description`)}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      ))}
    </FieldGroup>
  )
}

function AgentConnectorFields({ form, t }: AgentFormProps) {
  return (
    <FieldGroup
      title={t('QQ / TG / community connections')}
      description={t(
        'Configure the QQ, Telegram, and community connections for this site.'
      )}
    >
      <AgentSwitchField
        form={form}
        t={t}
        name='qqBotEnabled'
        label='Enable QQ bot'
        description='Use OneBot or LLoneBot for QQ group operations on this site'
      />
      <AgentSwitchField
        form={form}
        t={t}
        name='tgBotEnabled'
        label='Enable Telegram bot'
        description='Use Telegram bot for TG group operations on this site'
      />
      <AgentInputField
        form={form}
        t={t}
        name='qqOneBotUrl'
        label='QQ bot service address'
      />
      <AgentInputField form={form} t={t} name='qqGroupId' label='QQ group ID' />
      <AgentInputField
        form={form}
        t={t}
        name='qqAccessToken'
        label='QQ bot access token'
        description='Leave blank to keep the current secret'
        secret
      />
      <AgentInputField
        form={form}
        t={t}
        name='tgChatId'
        label='Telegram chat ID'
      />
      <AgentInputField
        form={form}
        t={t}
        name='tgBotToken'
        label='Telegram bot token'
        description='Leave blank to keep the current secret'
        secret
      />
      <AgentInputField
        form={form}
        t={t}
        name='communityHost'
        label='Community host'
      />
      <AgentInputField
        form={form}
        t={t}
        name='communityRoomId'
        label='Community room ID'
      />
    </FieldGroup>
  )
}

function AgentImageGenerationFields({ form, t }: AgentFormProps) {
  const numericFields = [
    {
      name: 'imageTimeoutSeconds',
      label: 'Image request timeout',
      description: 'Maximum time to wait for one image generation request.',
      min: 1,
      max: 600,
      step: 1,
    },
    {
      name: 'imageRetryLimit',
      label: 'Image retry attempts',
      description: 'Total attempts for temporary upstream failures.',
      min: 1,
      max: 5,
      step: 1,
    },
    {
      name: 'imageRetryBaseDelaySeconds',
      label: 'Initial retry delay',
      description: 'Delay before the first retry; later retries use backoff.',
      min: 0,
      max: 300,
      step: 0.5,
    },
    {
      name: 'imageRetryMaxDelaySeconds',
      label: 'Maximum retry delay',
      description: 'Upper limit for retry backoff and Retry-After handling.',
      min: 0,
      max: 300,
      step: 0.5,
    },
    {
      name: 'imageCooldownSeconds',
      label: 'Per-user image cooldown',
      description: 'Minimum wait between image requests from the same user.',
      min: 0,
      max: 86400,
      step: 1,
    },
  ] as const

  return (
    <FieldGroup
      title={t('QQ image generation')}
      description={t(
        'Manage the image service, model, delivery timing, retries, and per-user limits without restarting the bot.'
      )}
    >
      <AgentSwitchField
        form={form}
        t={t}
        name='imageGenerationEnabled'
        label='Enable QQ image generation'
        description='Allow image commands in QQ groups and private chats.'
      />
      <AgentSwitchField
        form={form}
        t={t}
        name='imageRequireBind'
        label='Require account binding for images'
        description='Only users linked to a site account can start image generation.'
      />
      <AgentInputField
        form={form}
        t={t}
        name='imageApiBaseUrl'
        label='Image service address'
        description='OpenAI-compatible base URL ending at /v1.'
      />
      <AgentInputField
        form={form}
        t={t}
        name='imageApiKey'
        label='Image service key'
        description='Leave blank to keep the current secret or environment fallback.'
        secret
      />
      <AgentInputField
        form={form}
        t={t}
        name='imageModel'
        label='Image model'
        placeholder='gpt-image-2'
      />
      <AgentInputField
        form={form}
        t={t}
        name='imageSize'
        label='Image size'
        placeholder='1024x1024'
        description='Use auto or WIDTHxHEIGHT supported by the selected service.'
      />
      {numericFields.map((item) => (
        <FormField
          key={item.name}
          control={form.control}
          name={item.name}
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t(item.label)}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={item.min}
                  max={item.max}
                  step={item.step}
                  {...field}
                />
              </FormControl>
              <FormDescription>{t(item.description)}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      ))}
    </FieldGroup>
  )
}

function AgentChatOpsFields({ form, t }: AgentFormProps) {
  return (
    <FieldGroup
      title={t('Group chat command security')}
      description={t(
        'Let QQ, Telegram, and community messages create tasks, while approvals, budgets, and logs remain inside this admin system.'
      )}
    >
      <AgentSwitchField
        form={form}
        t={t}
        name='chatOpsEnabled'
        label='Enable group chat commands'
        description='Allow QQ/TG/community webhook messages to create Robot tasks.'
      />
      <AgentSwitchField
        form={form}
        t={t}
        name='chatOpsAutoReply'
        label='Auto reply in group chat'
        description='Send the result back to the same QQ, Telegram, or community room when the connection is configured.'
      />
      <AgentSwitchField
        form={form}
        t={t}
        name='chatOpsAllowNaturalLanguage'
        label='Allow natural language mention'
        description='Treat @helper mentions as tasks even without a strict command prefix.'
      />
      <AgentSwitchField
        form={form}
        t={t}
        name='chatOpsRequireAdminForOps'
        label='Only admins can run operations'
        description='Messaging, rewards, and approval commands require configured Chat command handling admins.'
      />
      <AgentSwitchField
        form={form}
        t={t}
        name='chatOpsTrustGroupAdmin'
        label='Trust group admin identity role'
        description='Also trust owner/admin role reported by QQ/TG adapters. Keep off unless the adapter is trusted.'
      />
      <AgentInputField
        form={form}
        t={t}
        name='chatOpsWebhookSecret'
        label='Chat command secret'
        description='Shared secret for webhook calls; leave blank to keep current secret.'
        secret
      />
      <AgentInputField
        form={form}
        t={t}
        name='chatOpsAdminExternalIds'
        label='Allowed external admin IDs'
        placeholder='123456,calun'
        description='Comma-separated QQ/TG/community user IDs or usernames allowed to execute admin commands.'
      />
      <AgentInputField
        form={form}
        t={t}
        name='chatOpsCommandPrefixes'
        label='Command prefixes'
        description='Comma-separated prefixes such as /agent,@agent.'
      />
    </FieldGroup>
  )
}

function AgentLegacySyncFields({ form, t }: AgentFormProps) {
  const enabled = Boolean(form.watch('legacyConfigImportEnabled'))
  const [open, setOpen] = useState(false)
  const visibleOpen = enabled || open

  return (
    <Collapsible open={visibleOpen} onOpenChange={setOpen}>
      <Card
        className={
          enabled
            ? 'border-destructive/50 bg-destructive/5 overflow-hidden'
            : 'bg-muted/10 overflow-hidden border-dashed'
        }
      >
        <CardHeader className='bg-muted/30 border-b py-4'>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div className='space-y-2'>
              <div className='flex flex-wrap items-center gap-2'>
                <CardTitle className='text-base'>
                  {t('Emergency migration / old snapshot import')}
                </CardTitle>
                <Badge variant={enabled ? 'destructive' : 'secondary'}>
                  {enabled
                    ? t('Emergency import is ON')
                    : t('Daily mode: database is the source of truth')}
                </Badge>
              </div>
              <CardDescription>
                {t(
                  'Normal group and game operations should be changed from the live group registry and capability matrix. This old adapter import is only for one-time migration or emergency recovery, and every attempt is audited.'
                )}
              </CardDescription>
            </div>
            <CollapsibleTrigger className='border-input bg-background hover:bg-accent hover:text-accent-foreground focus-visible:ring-ring inline-flex h-9 items-center justify-center gap-2 rounded-md border px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none'>
              {visibleOpen
                ? t('Hide emergency fields')
                : t('Open emergency fields')}
              <ChevronDown
                className={`h-4 w-4 transition-transform ${visibleOpen ? 'rotate-180' : ''}`}
              />
            </CollapsibleTrigger>
          </div>
        </CardHeader>
        <CollapsibleContent>
          <CardContent className='space-y-4 p-4'>
            <div className='rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm leading-6 text-amber-900 dark:text-amber-100'>
              {t(
                'Keep this closed during daily operations. If it is enabled, old adapter-local snapshots can overwrite live database-backed group/game configuration when the request reason is allowlisted.'
              )}
            </div>
            <div className='grid gap-4 lg:grid-cols-2'>
              <AgentSwitchField
                form={form}
                t={t}
                name='legacyConfigImportEnabled'
                label='Allow emergency old snapshot import'
                description='When disabled, /api/agent/chatops/config/import is blocked and the database-backed group/game config remains the only source of truth.'
              />
              <AgentInputField
                form={form}
                t={t}
                name='legacyConfigImportReasons'
                label='Emergency import reason allowlist'
                placeholder='manual_legacy_import,manual_legacy_recovery'
                description='Comma-separated allowlist. Example: manual_legacy_import, manual_legacy_recovery. Requests with other reasons are rejected and logged.'
              />
            </div>
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}

function AgentPromptFields({ form, t }: AgentFormProps) {
  const names: AgentTextAreaField[] = [
    'systemPrompt',
    'siteKnowledge',
    'welcomeTemplate',
    'activityPolicy',
    'riskPolicy',
  ]
  return (
    <FieldGroup
      title={t('Robot replies and handling rules')}
      description={t(
        'Set site knowledge, welcome messages, activity rules, and risk-handling instructions used in robot replies.'
      )}
    >
      {names.map((name) => (
        <FormField
          key={name}
          control={form.control}
          name={name}
          render={({ field }) => (
            <FormItem className='lg:col-span-2'>
              <FormLabel>{t(name)}</FormLabel>
              <FormControl>
                <Textarea rows={4} {...field} />
              </FormControl>
              <FormDescription>{t(`${name} description`)}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      ))}
    </FieldGroup>
  )
}

function AgentToolCatalog({ tools, t }: { tools: AgentTool[]; t: Translate }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Available bot actions')}</CardTitle>
        <CardDescription>
          {t(
            'High-risk actions require approval instead of running immediately.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
        {tools.map((tool) => (
          <div key={tool.name} className='rounded-lg border p-3 text-sm'>
            <div className='flex items-start justify-between gap-2'>
              <div className='font-medium'>{tool.title || tool.name}</div>
              <Badge
                variant={tool.approval_required ? 'destructive' : 'secondary'}
              >
                {tool.approval_required ? t('Approval') : t('Auto')}
              </Badge>
            </div>
            <div className='text-muted-foreground mt-1'>{tool.description}</div>
            <div className='mt-2 flex gap-2'>
              <Badge variant='outline'>{tool.category}</Badge>
              <Badge variant='outline'>{tool.risk_level}</Badge>
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function CardLine({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 leading-6 break-words'>{value}</div>
    </div>
  )
}

function AgentAdvancedCard({
  title,
  description,
  children,
  defaultOpen = true,
}: {
  title: string
  description: string
  children: ReactNode
  defaultOpen?: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className='border-primary/10 bg-card overflow-hidden rounded-xl border'
    >
      <div className='border-border/70 border-b p-4'>
        <CollapsibleTrigger
          render={
            <button
              type='button'
              aria-expanded={open}
              className='flex w-full items-start justify-between gap-3 text-left'
            />
          }
        >
          <div className='space-y-1'>
            <div className='font-medium'>{title}</div>
            <div className='text-muted-foreground text-sm leading-6'>
              {description}
            </div>
          </div>
          <ChevronDown
            className={`text-muted-foreground mt-1 size-4 shrink-0 transition-transform ${open ? 'rotate-180' : ''}`}
          />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent className='px-4 pt-4 pb-4'>
        {children}
      </CollapsibleContent>
    </Collapsible>
  )
}

export function AgentOpsSection({ defaultValues }: Props) {
  const t = useOpsT()
  const updateOption = useUpdateOption()
  const [dashboard, setDashboard] = useState<AgentDashboard | null>(null)
  const [fundOverview, setFundOverview] =
    useState<OpsRewardFundOverview | null>(null)
  const [budgetAction, setBudgetAction] = useState<'save' | 'reset' | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const defaults = useMemo(() => buildDefaults(defaultValues), [defaultValues])
  const savedBudgetValuesRef = useRef(readDailyBudgetValues(defaults))
  const savedView = useMemo(
    () => buildRobotConfiguredView(defaultValues),
    [defaultValues]
  )
  const form = useForm<Values>({
    resolver: zodResolver(schema) as Resolver<Values>,
    defaultValues: defaults,
  })
  const siteState = dashboard?.site_state
  const tools = dashboard?.tools ?? []
  const groups = useMemo(() => siteState?.groups ?? [], [siteState?.groups])
  const avgResponse = siteState?.channels?.avg_response_ms
  const channels = siteState?.channels
  const channelsSummary =
    channels && typeof channels.enabled === 'number'
      ? `${channels.enabled} / ${channels.total ?? '?'}`
      : t('Not reported')
  const recentErrorCount = siteState?.recent_errors?.length ?? 0
  const rt = dashboard?.setting || {}
  const rtBool = (key: string) => rt[key] === true
  const actionTimeline = useMemo<OpsTimelineItem[]>(() => {
    const tone = (level?: string) => {
      const lower = (level || '').toLowerCase()
      if (lower === 'low') return 'success' as const
      if (lower === 'medium') return 'info' as const
      if (lower === 'high' || lower === 'critical') return 'danger' as const
      return 'neutral' as const
    }
    return [...(dashboard?.actions ?? [])]
      .sort((a, b) => (b.created_at || 0) - (a.created_at || 0))
      .slice(0, 8)
      .map((a) => ({
        time: a.created_at ? formatTime(a.created_at) : '—',
        type: translateRobotActionType(a.action_type, t),
        detail: `${a.agent_name || '—'} · ${a.status || '—'}${
          a.reason ? ' · ' + a.reason : ''
        }`,
        tone: tone(a.risk_level),
      }))
  }, [dashboard, t])
  const groupRows = useMemo(
    () =>
      groups.map((g) => ({
        id: g.name,
        cells: [
          g.name,
          `${g.healthy_channels ?? 0} / ${g.disabled_channels ?? 0}`,
          String(g.recent_error_count ?? 0),
          <div key='state' className='flex justify-center'>
            <OpsStatusBadge tone={g.available ? 'success' : 'danger'}>
              {g.available ? t('Available') : t('Disabled')}
            </OpsStatusBadge>
          </div>,
        ],
      })),
    [groups, t]
  )

  const loadDashboardData = useCallback(async () => {
    const res = await api.get('/api/agent/dashboard')
    if (!res.data?.success) {
      throw new Error(res.data?.message || t('Failed to load helper dashboard'))
    }
    const nextDashboard = res.data.data as AgentDashboard
    const siteId = nextDashboard.site_state?.site_id || savedView.siteId || ''
    let nextFundOverview: OpsRewardFundOverview | null = null
    if (siteId) {
      const fundRes = await api.get(
        `/api/ops/fund/${encodeURIComponent(siteId)}`
      )
      if (!fundRes.data?.success) {
        throw new Error(
          fundRes.data?.message || t('Failed to load daily reward budgets')
        )
      }
      nextFundOverview = fundRes.data.data as OpsRewardFundOverview
    }
    return { dashboard: nextDashboard, fundOverview: nextFundOverview }
  }, [savedView.siteId, t])

  const refreshDashboard = useCallback(async () => {
    setLoading(true)
    try {
      const next = await loadDashboardData()
      setDashboard(next.dashboard)
      setFundOverview(next.fundOverview)
    } catch (error) {
      toast.error(
        formatErrorMessage(error, t('Failed to load current robot status'))
      )
    } finally {
      setLoading(false)
    }
  }, [loadDashboardData, t])

  async function recordHealthCheck() {
    setLoading(true)
    try {
      await api.post('/api/agent/events', {
        event_type: 'admin.health_check',
        source: 'admin',
        severity: 'info',
        status: 'closed',
        title: 'Admin health check',
      })
      toast.success(t('Health-check event recorded'))
      await refreshDashboard()
    } catch (error) {
      toast.error(
        formatErrorMessage(error, t('Failed to record health-check event'))
      )
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    form.reset(defaults)
    savedBudgetValuesRef.current = readDailyBudgetValues(defaults)
  }, [defaults, form])

  useEffect(() => {
    let cancelled = false
    void loadDashboardData()
      .then((next) => {
        if (cancelled) return
        setDashboard(next.dashboard)
        setFundOverview(next.fundOverview)
      })
      .catch((error) => {
        if (!cancelled) {
          toast.error(
            formatErrorMessage(error, t('Failed to load current robot status'))
          )
        }
      })
    return () => {
      cancelled = true
    }
  }, [loadDashboardData, t])

  async function onSubmit(values: Values) {
    const updates = (Object.keys(optionKeyMap) as Array<keyof Values>).filter(
      (key) => {
        if (sensitiveFields.includes(key) && !String(values[key] ?? '').trim())
          return false
        const baseline = isDailyBudgetField(key)
          ? savedBudgetValuesRef.current[key]
          : defaults[key]
        return values[key] !== baseline
      }
    )
    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const key of updates) {
      await updateOption.mutateAsync({
        key: optionKeyMap[key],
        value: String(values[key] ?? ''),
      })
    }
    form.reset({
      ...values,
      llmApiKey: '',
      hermesApiKey: '',
      imageApiKey: '',
      qqAccessToken: '',
      tgBotToken: '',
      chatOpsWebhookSecret: '',
    })
    savedBudgetValuesRef.current = readDailyBudgetValues(values)
    await refreshDashboard()
  }

  async function saveDailyBudgets(resetToday: boolean) {
    const budgetNames = dailyBudgetFields.map(({ name }) => name)
    if (!(await form.trigger(budgetNames))) return

    setBudgetAction(resetToday ? 'reset' : 'save')
    try {
      const values = form.getValues()
      const nextBudgets = readDailyBudgetValues(values)
      const changed = dailyBudgetFields.filter(
        ({ name }) => nextBudgets[name] !== savedBudgetValuesRef.current[name]
      )

      for (const { name } of changed) {
        await updateOption.mutateAsync({
          key: optionKeyMap[name],
          value: String(nextBudgets[name]),
        })
      }
      savedBudgetValuesRef.current = nextBudgets
      for (const { name } of dailyBudgetFields) {
        form.resetField(name, { defaultValue: nextBudgets[name] })
      }

      if (resetToday) {
        const siteId = dashboard?.site_state?.site_id || savedView.siteId
        if (!siteId) throw new Error(t('Site ID is required'))
        const poolTypes = dailyBudgetFields
          .filter(({ name }) => nextBudgets[name] > 0)
          .map(({ poolType }) => poolType)
        if (poolTypes.length === 0) {
          throw new Error(
            t('At least one daily budget must be greater than zero')
          )
        }
        const response = await api.post(
          `/api/ops/fund/${encodeURIComponent(siteId)}/restore-capacity`,
          {
            pool_types: poolTypes,
            request_id: `admin-budget-reset-${Date.now()}`,
            reason: 'Apply configured daily reward budgets from admin settings',
          }
        )
        if (!response.data?.success) {
          throw new Error(
            response.data?.message || t('Failed to reset daily reward budgets')
          )
        }
        toast.success(t('Daily reward budgets saved and reset for today'))
      } else if (changed.length > 0) {
        toast.success(t('Daily reward budgets saved'))
      } else {
        toast.info(t('No budget changes to save'))
      }
      await refreshDashboard()
    } catch (error) {
      toast.error(
        formatErrorMessage(error, t('Failed to save daily reward budgets'))
      )
    } finally {
      setBudgetAction(null)
    }
  }

  return (
    <SettingsSection title={t('Robot settings')}>
      <section
        data-ui-revision='agentops-editor-first-r11-20260713'
        className='bg-card rounded-lg border px-5 py-4 shadow-sm'
      >
        <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start'>
          <div className='flex min-w-0 items-start gap-3'>
            <div className='bg-muted text-foreground flex h-9 w-9 shrink-0 items-center justify-center rounded-lg'>
              <Bot className='h-4 w-4' aria-hidden='true' />
            </div>
            <div className='space-y-1'>
              <h3 className='text-base font-semibold'>
                {t('Connection and reply settings')}
              </h3>
              <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
                {t(
                  'Change bot connections, reward limits, command permissions, and reply rules for this site.'
                )}
              </p>
            </div>
          </div>
          <div className='flex flex-wrap items-center gap-2 lg:justify-end'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => void refreshDashboard()}
              disabled={loading}
            >
              <RefreshCw
                className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`}
                aria-hidden='true'
              />
              {loading ? t('Refreshing') : t('Refresh current status')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant='secondary'
              onClick={() => void recordHealthCheck()}
            >
              {t('Record a connection check')}
            </Button>
          </div>
        </div>
        <div className='mt-3 flex flex-wrap gap-2 border-t pt-3'>
          <Badge
            variant={
              savedView.enabled === null
                ? 'outline'
                : savedView.enabled
                  ? 'default'
                  : 'secondary'
            }
          >
            {savedView.enabled === null
              ? t('Not configured')
              : savedView.enabled
                ? t('Bot enabled')
                : t('Bot disabled')}
          </Badge>
          <Badge variant='outline'>
            {t('Site')}: {savedView.siteId || t('Not configured')}
          </Badge>
          <Badge variant='outline'>
            {savedView.qqBotEnabled === true || savedView.tgBotEnabled === true
              ? t('QQ / TG connection configured')
              : t('QQ / TG connection not configured')}
          </Badge>
          <Badge variant={dashboard ? 'default' : 'outline'}>
            {dashboard
              ? t('Current status loaded')
              : t('Current status unavailable')}
          </Badge>
        </div>
      </section>

      <DailyRewardBudgetPanel
        form={form}
        t={t}
        overview={fundOverview}
        onSave={() => saveDailyBudgets(false)}
        onSaveAndReset={() => saveDailyBudgets(true)}
        isSaving={budgetAction === 'save'}
        isResetting={budgetAction === 'reset'}
      />

      <div id='helper-config' className='mt-4 scroll-mt-24'>
        <AgentAdvancedCard
          title={t('Edit robot settings')}
          description={t(
            'Edit QQ and TG connections, reward limits, safety rules, command permissions, and reply settings here.'
          )}
        >
          <AgentSettingsForm
            form={form}
            t={t}
            onSubmit={onSubmit}
            isSaving={updateOption.isPending}
          />
        </AgentAdvancedCard>
      </div>

      <OpsSurfaceGrid className='mt-4 xl:grid-cols-2'>
        <OpsPanel
          title={t('Saved settings')}
          description={t(
            'Saved values shown here are read-only. Use Robot settings above to change them.'
          )}
        >
          <div className='space-y-3 text-sm'>
            <CardLine
              label={t('Site identity')}
              value={
                savedView.siteId || savedView.siteName
                  ? `${savedView.siteId || t('Not configured')}${
                      savedView.siteName ? ` / ${savedView.siteName}` : ''
                    }`
                  : t('Not configured')
              }
            />
            <CardLine
              label={t('Configured public site URL')}
              value={savedView.publicBaseUrl || t('Not configured')}
            />
            <CardLine
              label={t('AI service and model')}
              value={
                savedView.llmProvider || savedView.llmModel
                  ? `${savedView.llmProvider || '-'} / ${
                      savedView.llmModel || '-'
                    }`
                  : t('Not configured')
              }
            />
            <CardLine
              label={t('QQ image generation')}
              value={`${
                savedView.imageGenerationEnabled === false
                  ? t('Disabled')
                  : t('Enabled')
              } · ${savedView.imageModel || t('Not configured')} / ${
                savedView.imageSize || t('Not configured')
              }`}
            />
            <CardLine
              label={t('Image service configuration')}
              value={`${savedView.imageApiBaseUrl || t('Not configured')} · ${
                rtBool('image_api_key_configured')
                  ? t('Key saved in admin')
                  : t('Using environment key or not configured')
              }`}
            />
            <CardLine
              label={t('Configured QQ group IDs')}
              value={
                savedView.qqGroupIds.length > 0
                  ? savedView.qqGroupIds.join(', ')
                  : t('Not configured')
              }
            />
            <CardLine
              label={t('Configured Telegram chat IDs')}
              value={
                savedView.tgChatIds.length > 0
                  ? savedView.tgChatIds.join(', ')
                  : t('Not configured')
              }
            />
            <CardLine
              label={t('Legacy adapter snapshot import')}
              value={
                savedView.legacyConfigImportEnabled === null
                  ? t('Not configured')
                  : savedView.legacyConfigImportEnabled
                    ? t('Enabled')
                    : t('Disabled')
              }
            />
            <CardLine
              label={t('Allowed legacy import reasons')}
              value={
                savedView.legacyConfigImportReasons.length > 0
                  ? savedView.legacyConfigImportReasons.join(', ')
                  : t('Not configured')
              }
            />
          </div>
        </OpsPanel>

        <OpsPanel
          title={t('Current online status')}
          description={t(
            'Not enabled means the service is switched off; it does not mean the connection failed.'
          )}
        >
          <div className='space-y-3 text-sm'>
            <div className='flex flex-wrap gap-2'>
              <OpsStatusBadge
                tone={rtBool('qq_bot_enabled') ? 'success' : 'neutral'}
              >
                {t('QQ')}{' '}
                {rtBool('qq_bot_enabled') ? t('Online') : t('Not enabled')}
              </OpsStatusBadge>
              <OpsStatusBadge
                tone={rtBool('tg_bot_enabled') ? 'success' : 'neutral'}
              >
                {t('TG')}{' '}
                {rtBool('tg_bot_enabled') ? t('Online') : t('Not enabled')}
              </OpsStatusBadge>
              <OpsStatusBadge
                tone={rtBool('community_enabled') ? 'success' : 'neutral'}
              >
                {t('Community')}{' '}
                {rtBool('community_enabled') ? t('Online') : t('Not enabled')}
              </OpsStatusBadge>
              <OpsStatusBadge
                tone={
                  typeof avgResponse === 'number' && avgResponse <= 2000
                    ? 'success'
                    : typeof avgResponse === 'number'
                      ? 'warning'
                      : 'neutral'
                }
              >
                {typeof avgResponse === 'number'
                  ? `${avgResponse} ms`
                  : t('Not reported')}
              </OpsStatusBadge>
            </div>
            <CardLine
              label={t('Channels enabled / total')}
              value={channelsSummary}
            />
            <CardLine
              label={t('Recent errors')}
              value={String(recentErrorCount)}
            />
          </div>
        </OpsPanel>
      </OpsSurfaceGrid>

      <div className='mt-4 grid gap-4 xl:grid-cols-2'>
        <OpsTimeline
          title={t('Recent handling records')}
          description={t('Latest handling records, newest first.')}
          items={actionTimeline}
          emptyMessage={loading ? t('Loading...') : t('No recent actions')}
        />
        <OpsDataTable
          title={t('Group connection status')}
          description={t(
            'Current availability and recent errors for each group.'
          )}
          columns={[
            { key: 'name', label: t('Group') },
            { key: 'health', label: t('Healthy / disabled') },
            { key: 'errors', label: t('Recent errors') },
            { key: 'state', label: t('State'), className: 'text-center' },
          ]}
          rows={groupRows}
          emptyMessage={loading ? t('Loading...') : t('No groups')}
        />
      </div>

      <div className='mt-4'>
        <GroupCheckinBridgePanel
          siteId={savedView.siteId}
          title={t('Group check-in and reward limits')}
          description={t(
            'Review each QQ or TG group check-in amount, verification requirement, and daily reward limit before making changes.'
          )}
          platformKinds={['qq', 'tg']}
        />
      </div>

      <div className='mt-4'>
        <AgentAdvancedCard
          title={t('Current bot status')}
          description={t(
            'Read-only view of current connections, groups, reward balances, risks, pending reviews, and available actions.'
          )}
          defaultOpen={false}
        >
          <div className='space-y-4'>
            <AgentRuntimeState
              dashboard={dashboard}
              defaultValues={defaultValues}
              t={t}
            />
            <AgentToolCatalog tools={tools} t={t} />
          </div>
        </AgentAdvancedCard>
      </div>
    </SettingsSection>
  )
}
