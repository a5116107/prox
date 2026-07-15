/*
Copyright (C) 2023-2026 QuantumNous
*/
import type { ReactNode } from 'react'
import type { Control } from 'react-hook-form'
import { ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
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
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import type { CommunityBotValues } from './community-bot-model'

type KeysMatching<Value> = {
  [Key in keyof CommunityBotValues]-?: NonNullable<
    CommunityBotValues[Key]
  > extends Value
    ? Key
    : never
}[keyof CommunityBotValues]

type StringFieldName = KeysMatching<string>
type NumberFieldName = KeysMatching<number>
type BooleanFieldName = KeysMatching<boolean>

type CommunityBotFieldGroupsProps = {
  control: Control<CommunityBotValues>
  botToken: string
  actionLoading: boolean
  onBotTokenChange: (value: string) => void
  onSaveBotToken: () => void | Promise<void>
}

type FieldProps<Name> = {
  control: Control<CommunityBotValues>
  name: Name
  label: ReactNode
  description?: ReactNode
}

function FieldGroup(props: {
  title: string
  description?: string
  defaultOpen?: boolean
  children: ReactNode
}) {
  return (
    <Collapsible
      defaultOpen={props.defaultOpen}
      className='bg-card rounded-xl border'
    >
      <CollapsibleTrigger className='group flex w-full items-center justify-between gap-2 px-4 py-3 text-left'>
        <div>
          <div className='font-medium'>{props.title}</div>
          {props.description ? (
            <div className='text-muted-foreground text-xs'>
              {props.description}
            </div>
          ) : null}
        </div>
        <ChevronDown className='text-muted-foreground h-4 w-4 shrink-0 transition-transform group-data-[panel-open]:rotate-180' />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className='space-y-4 border-t p-4'>{props.children}</div>
      </CollapsibleContent>
    </Collapsible>
  )
}

function TextField(
  props: FieldProps<StringFieldName> & {
    type?: 'text' | 'password' | 'url'
    placeholder?: string
  }
) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Input
              {...field}
              type={props.type}
              placeholder={props.placeholder}
              value={String(field.value ?? '')}
            />
          </FormControl>
          {props.description ? (
            <FormDescription>{props.description}</FormDescription>
          ) : null}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function NumberField(
  props: FieldProps<NumberFieldName> & { min?: number; max?: number }
) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Input
              {...field}
              type='number'
              min={props.min}
              max={props.max}
              value={Number(field.value ?? 0)}
            />
          </FormControl>
          {props.description ? (
            <FormDescription>{props.description}</FormDescription>
          ) : null}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function SwitchField(props: FieldProps<BooleanFieldName>) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <SettingsSwitchItem>
          <SettingsSwitchContent>
            <FormLabel>{props.label}</FormLabel>
            {props.description ? (
              <FormDescription>{props.description}</FormDescription>
            ) : null}
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

function renderTemplateSample(
  template: string,
  replacements: Record<string, string>
) {
  let result = template
  for (const [key, value] of Object.entries(replacements)) {
    result = result.split(`{${key}}`).join(value)
  }
  return result
}

function TemplatePreview(props: { value: string }) {
  const { t } = useTranslation()
  const sample = renderTemplateSample(props.value, {
    username: 'alice',
    amount: '2.48 USD',
    balance: '33.06 USD',
    reason: t('Do not repeat check-in; only once per day'),
    key: 'API Key: OK',
    quota: 'Quota: OK',
    tip: t('Guide the user to finish binding'),
    host: 'https://dc.hhhl.cc',
  })

  if (!props.value.trim()) {
    return (
      <div className='bg-muted/30 text-muted-foreground mt-1 rounded-md border border-dashed px-3 py-2 text-xs'>
        {t('Leave blank to use the system default copy')}
      </div>
    )
  }

  return (
    <div className='bg-muted/40 mt-1 rounded-md border px-3 py-2'>
      <div className='text-muted-foreground mb-1 text-[10px] tracking-wide uppercase'>
        {t('Preview (sample data)')}
      </div>
      <div className='text-sm whitespace-pre-wrap'>{sample}</div>
    </div>
  )
}

function TemplateField(
  props: FieldProps<StringFieldName> & {
    rows?: number
    placeholder?: string
    preview?: boolean
  }
) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Textarea
              {...field}
              rows={props.rows}
              placeholder={props.placeholder}
              value={String(field.value ?? '')}
            />
          </FormControl>
          {props.description ? (
            <FormDescription>{props.description}</FormDescription>
          ) : null}
          {props.preview ? (
            <TemplatePreview value={String(field.value ?? '')} />
          ) : null}
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function ConnectionFields(props: CommunityBotFieldGroupsProps) {
  const { t } = useTranslation()

  return (
    <FieldGroup
      title={t('Connection and binding')}
      description={t(
        'Bot enablement, community host, room, OAuth credentials, token, and auto invite settings.'
      )}
      defaultOpen
    >
      <SwitchField
        control={props.control}
        name='enabled'
        label={t('Enable community bot')}
        description={t(
          'Enable automation for community invitations, rewards, and notifications'
        )}
      />
      <TextField
        control={props.control}
        name='communityHost'
        label={t('Community host')}
        description={t('Base URL of the independent community site')}
      />
      <TextField
        control={props.control}
        name='providerSlug'
        label={t('MiAuth provider slug')}
        description={t('Must match the New API custom MiAuth provider slug')}
      />
      <TextField
        control={props.control}
        name='roomId'
        label={t('Community room ID')}
        description={t(
          'Room that receives auto invitations and bot notifications'
        )}
      />
      <TextField
        control={props.control}
        name='oauthCallbackUrl'
        label={t('Bot MiAuth callback URL')}
        description={t(
          'MiAuth callback URL; client_id is not used by the current community bot authorization flow'
        )}
        placeholder='https://ai.prox.us.ci/oauth/callback'
      />
      <TextField
        control={props.control}
        name='oauthClientId'
        label={t('Bot MiAuth Client ID')}
        description={t('Client ID from the community app settings')}
      />
      <TextField
        control={props.control}
        name='oauthClientSecret'
        label={t('Bot MiAuth Client Secret')}
        description={t('Optional; stored server-side only')}
        type='password'
      />

      <div className='grid gap-3 rounded-xl border p-4 md:grid-cols-[1fr_auto]'>
        <div>
          <FormLabel>{t('Community bot token')}</FormLabel>
          <FormDescription>
            {t(
              'Paste a community API token with read:account, read:chat, and write:chat permissions, then verify and save it.'
            )}
          </FormDescription>
          <Input
            type='password'
            value={props.botToken}
            onChange={(event) => props.onBotTokenChange(event.target.value)}
            placeholder={t('Leave empty after saving; stored server-side only')}
            autoComplete='off'
          />
        </div>
        <div className='flex items-end'>
          <Button
            type='button'
            variant='outline'
            disabled={props.actionLoading || !props.botToken.trim()}
            onClick={props.onSaveBotToken}
          >
            {t('Verify and save token')}
          </Button>
        </div>
      </div>

      <SwitchField
        control={props.control}
        name='autoInviteEnabled'
        label={t('Auto invite users')}
        description={t(
          'Invite users to the configured room after community MiAuth login'
        )}
      />
      <SwitchField
        control={props.control}
        name='inviteOnMiAuthLogin'
        label={t('Invite during MiAuth login')}
        description={t(
          'Runs after successful custom MiAuth login; login is not blocked by invite failures'
        )}
      />
    </FieldGroup>
  )
}

function RewardFields(props: { control: Control<CommunityBotValues> }) {
  const { t } = useTranslation()

  return (
    <FieldGroup
      title={t('Reward rules')}
      description={t(
        'Join rewards, daily rewards, and invite messages should only run when community status, room gate, and notification settings still agree.'
      )}
    >
      <SwitchField
        control={props.control}
        name='joinRewardEnabled'
        label={t('Enable one-time join reward')}
        description={t(
          'Optional; disabled by default for the cost-saving plan'
        )}
      />
      <NumberField
        control={props.control}
        name='joinRewardMinQuota'
        label={t('Join reward minimum quota')}
        min={0}
      />
      <NumberField
        control={props.control}
        name='joinRewardMaxQuota'
        label={t('Join reward maximum quota')}
        min={0}
      />
      <SwitchField
        control={props.control}
        name='dailyMessageRewardEnabled'
        label={t('Enable daily message reward')}
        description={t(
          'Reward bound New API users after enough valid room messages'
        )}
      />
      <NumberField
        control={props.control}
        name='dailyMessageThreshold'
        label={t('Daily valid message threshold')}
        min={1}
      />
      <NumberField
        control={props.control}
        name='dailyRewardMinQuota'
        label={t('Daily reward minimum quota')}
        min={0}
      />
      <NumberField
        control={props.control}
        name='dailyRewardMaxQuota'
        label={t('Daily reward maximum quota')}
        min={0}
      />
      <NumberField
        control={props.control}
        name='dailyMaxRewardsPerUser'
        label={t('Daily reward times per user')}
        description={t('0 means unlimited; recommended value is 1')}
        min={0}
      />
    </FieldGroup>
  )
}

function RealtimeFields(props: { control: Control<CommunityBotValues> }) {
  const { t } = useTranslation()

  return (
    <FieldGroup
      title={t('Realtime and burn after read')}
      description={t(
        'Realtime streaming, scan intervals, and automatic recall for command messages.'
      )}
    >
      <NumberField
        control={props.control}
        name='messageScanIntervalMinutes'
        label={t('Scan interval minutes')}
        min={1}
      />
      <NumberField
        control={props.control}
        name='messageLookbackMinutes'
        label={t('Message lookback minutes')}
        min={1}
      />
      <NumberField
        control={props.control}
        name='messageScanLimit'
        label={t('Messages per scan')}
        min={1}
        max={100}
      />
      <SwitchField
        control={props.control}
        name='streamingEnabled'
        label={t('Enable realtime streaming (sub-second response)')}
        description={t(
          'When enabled, commands are received through Misskey streaming in realtime, while polling remains as fallback.'
        )}
      />
      <NumberField
        control={props.control}
        name='messageScanIntervalSeconds'
        label={t('Scan interval seconds (preferred over minutes)')}
        description={t(
          'Current effective scan interval used by the community bot reward worker.'
        )}
        min={0}
      />
      <NumberField
        control={props.control}
        name='commandBurnAfterSeconds'
        label={t('Burn-after-read seconds')}
        description={t(
          'Seconds before check-in, verify, and command messages are automatically recalled. Use 0 to disable.'
        )}
        min={0}
      />
    </FieldGroup>
  )
}

function AntiSpamFields(props: { control: Control<CommunityBotValues> }) {
  const { t } = useTranslation()

  return (
    <FieldGroup
      title={t('Anti-spam rules')}
      description={t('Anti-spam and scan settings')}
    >
      <NumberField
        control={props.control}
        name='antiSpamMinChars'
        label={t('Minimum message characters')}
        min={0}
      />
      <NumberField
        control={props.control}
        name='antiSpamMinDistinctTexts'
        label={t('Minimum distinct messages')}
        description={t('Filters repeated water messages')}
        min={1}
      />
      <SwitchField
        control={props.control}
        name='antiSpamIgnoreBot'
        label={t('Ignore bot messages')}
        description={t('Bot messages are excluded from reward statistics')}
      />
    </FieldGroup>
  )
}

function NotificationSwitches(props: { control: Control<CommunityBotValues> }) {
  const { t } = useTranslation()

  return (
    <>
      <SwitchField
        control={props.control}
        name='notificationEnabled'
        label={t('Enable room notifications')}
        description={t('Bot sends invite, reward, and ops notices to the room')}
      />
      <SwitchField
        control={props.control}
        name='notifyOnInvite'
        label={t('Notify on invite')}
      />
      <SwitchField
        control={props.control}
        name='notifyOnJoinReward'
        label={t('Notify on join reward')}
      />
      <SwitchField
        control={props.control}
        name='notifyOnDailyReward'
        label={t('Notify on daily reward')}
      />
      <SwitchField
        control={props.control}
        name='notifyOnOpsAlert'
        label={t('Notify on ops alert')}
      />
    </>
  )
}

function RewardTemplateFields(props: { control: Control<CommunityBotValues> }) {
  const { t } = useTranslation()

  return (
    <>
      <TemplateField
        control={props.control}
        name='inviteNotificationTemplate'
        label={t('Invite notification template')}
        description={t('Available variables: {username}, {user_id}, {room_id}')}
        rows={2}
      />
      <TemplateField
        control={props.control}
        name='joinRewardTemplate'
        label={t('Join reward notification template')}
        description={t(
          'Available variables: {username}, {user_id}, {amount}, {room_id}'
        )}
        rows={2}
      />
      <TemplateField
        control={props.control}
        name='dailyRewardTemplate'
        label={t('Daily reward notification template')}
        description={t(
          'Available variables: {username}, {user_id}, {amount}, {count}, {room_id}'
        )}
        rows={2}
      />
      <TemplateField
        control={props.control}
        name='opsAlertTemplate'
        label={t('Ops alert template')}
        description={t('Available variables: {message}')}
        rows={2}
      />
    </>
  )
}

function CommandTemplateFields(props: {
  control: Control<CommunityBotValues>
}) {
  const { t } = useTranslation()
  const placeholder = t('Leave blank to use the system default copy')

  return (
    <>
      <TemplateField
        control={props.control}
        name='checkinSuccessTemplate'
        label={t('Check-in success template')}
        description={t(
          'Available variables: {username}, {amount} for this reward, and {balance} for current quota. Leave blank to use the default copy.'
        )}
        placeholder={placeholder}
        rows={3}
        preview
      />
      <TemplateField
        control={props.control}
        name='checkinFailedTemplate'
        label={t('Check-in failure template')}
        description={t(
          'Available variables: {username} and {reason}, such as already checked in today.'
        )}
        placeholder={placeholder}
        rows={2}
        preview
      />
      <TemplateField
        control={props.control}
        name='verifyPassTemplate'
        label={t('Verify success template')}
        description={t(
          'Available variables: {username}, {key} for API key status, and {quota} for quota status.'
        )}
        placeholder={placeholder}
        rows={3}
        preview
      />
      <TemplateField
        control={props.control}
        name='verifyFailedTemplate'
        label={t('Verify failure template')}
        description={t(
          'Available variables: {username}, {key}, {quota}, and {tip} for the guidance message.'
        )}
        placeholder={placeholder}
        rows={3}
        preview
      />
      <TemplateField
        control={props.control}
        name='bindGuideTemplate'
        label={t('Unbound guidance template')}
        description={t(
          'Available variables: {username} and {host} for the community site address.'
        )}
        placeholder={placeholder}
        rows={2}
        preview
      />
    </>
  )
}

function NotificationFields(props: { control: Control<CommunityBotValues> }) {
  const { t } = useTranslation()

  return (
    <FieldGroup
      title={t('Automation & notifications')}
      description={t(
        'Room notification switches plus invite, reward, check-in, and verify message templates.'
      )}
    >
      <NotificationSwitches control={props.control} />
      <RewardTemplateFields control={props.control} />
      <CommandTemplateFields control={props.control} />
    </FieldGroup>
  )
}

export function CommunityBotFieldGroups(props: CommunityBotFieldGroupsProps) {
  return (
    <>
      <ConnectionFields {...props} />
      <RewardFields control={props.control} />
      <RealtimeFields control={props.control} />
      <AntiSpamFields control={props.control} />
      <NotificationFields control={props.control} />
    </>
  )
}
