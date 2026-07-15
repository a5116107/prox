/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useEffect } from 'react'
import * as z from 'zod'
import { useForm, type UseFormReturn } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
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

const createSchema = () =>
  z.object({
    enabled: z.boolean(),
    highRiskKeyRecreateRequired: z.boolean(),
    highRiskActivationRequired: z.boolean(),
    highRiskActivationSource: z.string().min(1),
    activationCodeTTLMinutes: z.number().min(1).max(120),
    sameIPSameDayOAuthRegisterEnabled: z.boolean(),
    sameIPSameDayOAuthRegisterLimit: z.number().min(1).max(10),
    sameIPRegisterBlockMessage: z.string().min(1),
    requestIPTrackingEnabled: z.boolean(),
    sameIPMultiAccountUsageEnabled: z.boolean(),
    sameIPMultiAccountUsageWindowMinutes: z.number().min(1).max(1440),
    sameIPMultiAccountUsageUserLimit: z.number().min(1).max(50),
    sameIPMultiAccountUsageBlockMessage: z.string().min(1),
    dynamicIPChurnEnabled: z.boolean(),
    dynamicIPChurnWindowMinutes: z.number().min(1).max(1440),
    dynamicIPChurnDistinctIPLimit: z.number().min(2).max(100),
    dynamicIPChurnBlockMessage: z.string().min(1),
    burstRegisterEnabled: z.boolean(),
    burstRegisterWindowMinutes: z.number().min(1).max(1440),
    burstRegisterLimit: z.number().min(2).max(100),
    burstRegisterBlockMessage: z.string().min(1),
    inactiveTokenDisableEnabled: z.boolean(),
    inactiveTokenDisableDays: z.number().min(1).max(365),
    inactiveTokenDisableReason: z.string().min(1),
  })

type Values = z.infer<ReturnType<typeof createSchema>>

type Props = {
  defaultValues: {
    'risk_control_setting.enabled': boolean
    'risk_control_setting.high_risk_key_recreate_required': boolean
    'risk_control_setting.high_risk_activation_required': boolean
    'risk_control_setting.high_risk_activation_source': string
    'risk_control_setting.activation_code_ttl_minutes': number
    'risk_control_setting.same_ip_same_day_oauth_register_enabled': boolean
    'risk_control_setting.same_ip_same_day_oauth_register_limit': number
    'risk_control_setting.same_ip_register_block_message': string
    'risk_control_setting.request_ip_tracking_enabled': boolean
    'risk_control_setting.same_ip_multi_account_usage_enabled': boolean
    'risk_control_setting.same_ip_multi_account_usage_window_minutes': number
    'risk_control_setting.same_ip_multi_account_usage_user_limit': number
    'risk_control_setting.same_ip_multi_account_usage_block_message': string
    'risk_control_setting.dynamic_ip_churn_enabled': boolean
    'risk_control_setting.dynamic_ip_churn_window_minutes': number
    'risk_control_setting.dynamic_ip_churn_distinct_ip_limit': number
    'risk_control_setting.dynamic_ip_churn_block_message': string
    'risk_control_setting.burst_register_enabled': boolean
    'risk_control_setting.burst_register_window_minutes': number
    'risk_control_setting.burst_register_limit': number
    'risk_control_setting.burst_register_block_message': string
    'risk_control_setting.inactive_token_disable_enabled': boolean
    'risk_control_setting.inactive_token_disable_days': number
    'risk_control_setting.inactive_token_disable_reason': string
  }
}

const optionKeyMap: Record<keyof Values, string> = {
  enabled: 'risk_control_setting.enabled',
  highRiskKeyRecreateRequired:
    'risk_control_setting.high_risk_key_recreate_required',
  highRiskActivationRequired:
    'risk_control_setting.high_risk_activation_required',
  highRiskActivationSource: 'risk_control_setting.high_risk_activation_source',
  activationCodeTTLMinutes: 'risk_control_setting.activation_code_ttl_minutes',
  sameIPSameDayOAuthRegisterEnabled:
    'risk_control_setting.same_ip_same_day_oauth_register_enabled',
  sameIPSameDayOAuthRegisterLimit:
    'risk_control_setting.same_ip_same_day_oauth_register_limit',
  sameIPRegisterBlockMessage:
    'risk_control_setting.same_ip_register_block_message',
  requestIPTrackingEnabled: 'risk_control_setting.request_ip_tracking_enabled',
  sameIPMultiAccountUsageEnabled:
    'risk_control_setting.same_ip_multi_account_usage_enabled',
  sameIPMultiAccountUsageWindowMinutes:
    'risk_control_setting.same_ip_multi_account_usage_window_minutes',
  sameIPMultiAccountUsageUserLimit:
    'risk_control_setting.same_ip_multi_account_usage_user_limit',
  sameIPMultiAccountUsageBlockMessage:
    'risk_control_setting.same_ip_multi_account_usage_block_message',
  dynamicIPChurnEnabled: 'risk_control_setting.dynamic_ip_churn_enabled',
  dynamicIPChurnWindowMinutes:
    'risk_control_setting.dynamic_ip_churn_window_minutes',
  dynamicIPChurnDistinctIPLimit:
    'risk_control_setting.dynamic_ip_churn_distinct_ip_limit',
  dynamicIPChurnBlockMessage:
    'risk_control_setting.dynamic_ip_churn_block_message',
  burstRegisterEnabled: 'risk_control_setting.burst_register_enabled',
  burstRegisterWindowMinutes:
    'risk_control_setting.burst_register_window_minutes',
  burstRegisterLimit: 'risk_control_setting.burst_register_limit',
  burstRegisterBlockMessage:
    'risk_control_setting.burst_register_block_message',
  inactiveTokenDisableEnabled:
    'risk_control_setting.inactive_token_disable_enabled',
  inactiveTokenDisableDays: 'risk_control_setting.inactive_token_disable_days',
  inactiveTokenDisableReason:
    'risk_control_setting.inactive_token_disable_reason',
}

const parseNumber = (value: string, fallback: number) =>
  Number.parseInt(value || String(fallback), 10)

function buildDefaults(input: Props['defaultValues']): Values {
  return {
    enabled: Boolean(input['risk_control_setting.enabled'] ?? true),
    highRiskKeyRecreateRequired: Boolean(
      input['risk_control_setting.high_risk_key_recreate_required'] ?? true
    ),
    highRiskActivationRequired: Boolean(
      input['risk_control_setting.high_risk_activation_required'] ?? true
    ),
    highRiskActivationSource: String(
      input['risk_control_setting.high_risk_activation_source'] || 'qq'
    ),
    activationCodeTTLMinutes: Number(
      input['risk_control_setting.activation_code_ttl_minutes'] || 10
    ),
    sameIPSameDayOAuthRegisterEnabled: Boolean(
      input['risk_control_setting.same_ip_same_day_oauth_register_enabled'] ??
      true
    ),
    sameIPSameDayOAuthRegisterLimit: Number(
      input['risk_control_setting.same_ip_same_day_oauth_register_limit'] || 1
    ),
    sameIPRegisterBlockMessage: String(
      input['risk_control_setting.same_ip_register_block_message'] ||
        '检测到同一 IP 当天已注册过账号，请更换 IP 后再试。'
    ),
    requestIPTrackingEnabled: Boolean(
      input['risk_control_setting.request_ip_tracking_enabled'] ?? true
    ),
    sameIPMultiAccountUsageEnabled: Boolean(
      input['risk_control_setting.same_ip_multi_account_usage_enabled'] ?? false
    ),
    sameIPMultiAccountUsageWindowMinutes: Number(
      input[
        'risk_control_setting.same_ip_multi_account_usage_window_minutes'
      ] || 60
    ),
    sameIPMultiAccountUsageUserLimit: Number(
      input['risk_control_setting.same_ip_multi_account_usage_user_limit'] || 1
    ),
    sameIPMultiAccountUsageBlockMessage: String(
      input['risk_control_setting.same_ip_multi_account_usage_block_message'] ||
        '检测到同一 IP 在短时间内切换多个账号访问，当前 Key 已触发风控，请更换独立网络后重试。'
    ),
    dynamicIPChurnEnabled: Boolean(
      input['risk_control_setting.dynamic_ip_churn_enabled'] ?? false
    ),
    dynamicIPChurnWindowMinutes: Number(
      input['risk_control_setting.dynamic_ip_churn_window_minutes'] || 30
    ),
    dynamicIPChurnDistinctIPLimit: Number(
      input['risk_control_setting.dynamic_ip_churn_distinct_ip_limit'] || 6
    ),
    dynamicIPChurnBlockMessage: String(
      input['risk_control_setting.dynamic_ip_churn_block_message'] ||
        '检测到当前 Key 在短时间内频繁切换 IP，已触发风控，请完成账号校验后再试。'
    ),
    burstRegisterEnabled: Boolean(
      input['risk_control_setting.burst_register_enabled'] ?? false
    ),
    burstRegisterWindowMinutes: Number(
      input['risk_control_setting.burst_register_window_minutes'] || 10
    ),
    burstRegisterLimit: Number(
      input['risk_control_setting.burst_register_limit'] || 3
    ),
    burstRegisterBlockMessage: String(
      input['risk_control_setting.burst_register_block_message'] ||
        '检测到该 IP 在短时间内注册过多账号，请稍后更换 IP 后再试。'
    ),
    inactiveTokenDisableEnabled: Boolean(
      input['risk_control_setting.inactive_token_disable_enabled'] ?? false
    ),
    inactiveTokenDisableDays: Number(
      input['risk_control_setting.inactive_token_disable_days'] || 7
    ),
    inactiveTokenDisableReason: String(
      input['risk_control_setting.inactive_token_disable_reason'] ||
        '长时间未活跃的账号需重新创建 Key 并完成校验后再使用。'
    ),
  }
}

type Translate = (key: string) => string

type RiskFormProps = {
  form: UseFormReturn<Values>
  t: Translate
}

type SwitchFieldName =
  | 'enabled'
  | 'highRiskKeyRecreateRequired'
  | 'highRiskActivationRequired'
  | 'sameIPSameDayOAuthRegisterEnabled'
  | 'requestIPTrackingEnabled'
  | 'sameIPMultiAccountUsageEnabled'
  | 'dynamicIPChurnEnabled'
  | 'burstRegisterEnabled'
  | 'inactiveTokenDisableEnabled'

type NumberFieldName =
  | 'activationCodeTTLMinutes'
  | 'sameIPSameDayOAuthRegisterLimit'
  | 'sameIPMultiAccountUsageWindowMinutes'
  | 'sameIPMultiAccountUsageUserLimit'
  | 'dynamicIPChurnWindowMinutes'
  | 'dynamicIPChurnDistinctIPLimit'
  | 'burstRegisterWindowMinutes'
  | 'burstRegisterLimit'
  | 'inactiveTokenDisableDays'

type TextFieldName =
  | 'highRiskActivationSource'
  | 'sameIPRegisterBlockMessage'
  | 'sameIPMultiAccountUsageBlockMessage'
  | 'dynamicIPChurnBlockMessage'
  | 'burstRegisterBlockMessage'
  | 'inactiveTokenDisableReason'

function RiskSwitchField({
  form,
  t,
  name,
  label,
  description,
}: RiskFormProps & {
  name: SwitchFieldName
  label: string
  description: string
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <SettingsSwitchItem>
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

function RiskNumberField({
  form,
  t,
  name,
  label,
  fallback,
  min,
  max,
}: RiskFormProps & {
  name: NumberFieldName
  label: string
  fallback: number
  min: number
  max: number
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
              type='number'
              min={min}
              max={max}
              value={Number(field.value ?? fallback)}
              onChange={(e) =>
                field.onChange(parseNumber(e.target.value, fallback))
              }
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function RiskTextField({
  form,
  t,
  name,
  label,
  placeholder,
  textarea = true,
}: RiskFormProps & {
  name: TextFieldName
  label: string
  placeholder?: string
  textarea?: boolean
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem className={textarea ? 'lg:col-span-2' : undefined}>
          <FormLabel>{t(label)}</FormLabel>
          <FormControl>
            {textarea ? (
              <Textarea
                rows={3}
                placeholder={placeholder ? t(placeholder) : undefined}
                {...field}
              />
            ) : (
              <Input placeholder={placeholder} {...field} />
            )}
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function RiskMasterFields(props: RiskFormProps) {
  return (
    <>
      <RiskSwitchField
        {...props}
        name='enabled'
        label='Enable risk control'
        description='Apply account-risk gating, request-IP tracking, and OAuth registration limits.'
      />
      <RiskSwitchField
        {...props}
        name='highRiskKeyRecreateRequired'
        label='Force high-risk users to recreate keys'
        description='Previously disabled old keys stay invalid and users must create new keys.'
      />
      <RiskSwitchField
        {...props}
        name='highRiskActivationRequired'
        label='Require QQ activation for high-risk keys'
        description='New keys created by high-risk users stay disabled until QQ bind code activation completes.'
      />
      <RiskTextField
        {...props}
        name='highRiskActivationSource'
        label='Activation source'
        placeholder='qq'
        textarea={false}
      />
      <RiskNumberField
        {...props}
        name='activationCodeTTLMinutes'
        label='Bind code TTL (minutes)'
        fallback={10}
        min={1}
        max={120}
      />
    </>
  )
}

function RiskRegistrationFields(props: RiskFormProps) {
  return (
    <>
      <RiskSwitchField
        {...props}
        name='sameIPSameDayOAuthRegisterEnabled'
        label='Block same-IP second OAuth registration'
        description='Only affects new account creation. Existing users can still log in normally.'
      />
      <RiskNumberField
        {...props}
        name='sameIPSameDayOAuthRegisterLimit'
        label='Same-IP daily registration limit'
        fallback={1}
        min={1}
        max={10}
      />
      <RiskTextField
        {...props}
        name='sameIPRegisterBlockMessage'
        label='Registration block message'
      />
      <RiskSwitchField
        {...props}
        name='burstRegisterEnabled'
        label='Enable burst registration blocking'
        description='Stop repeated OAuth registrations from the same IP inside a short time window.'
      />
      <RiskNumberField
        {...props}
        name='burstRegisterWindowMinutes'
        label='Burst registration window (minutes)'
        fallback={10}
        min={1}
        max={1440}
      />
      <RiskNumberField
        {...props}
        name='burstRegisterLimit'
        label='Burst registration threshold'
        fallback={3}
        min={2}
        max={100}
      />
      <RiskTextField
        {...props}
        name='burstRegisterBlockMessage'
        label='Burst registration block message'
      />
    </>
  )
}

function RiskRequestIpFields(props: RiskFormProps) {
  return (
    <>
      <RiskSwitchField
        {...props}
        name='requestIPTrackingEnabled'
        label='Track request IP fingerprints'
        description='Persist per-key IP fingerprints for risk audits and inactivity checks.'
      />
      <RiskSwitchField
        {...props}
        name='sameIPMultiAccountUsageEnabled'
        label='Block same-IP multi-account token usage'
        description='Use this only when you want to treat short-window account switching on the same IP as high risk.'
      />
      <RiskNumberField
        {...props}
        name='sameIPMultiAccountUsageWindowMinutes'
        label='Same-IP account switching window (minutes)'
        fallback={60}
        min={1}
        max={1440}
      />
      <RiskNumberField
        {...props}
        name='sameIPMultiAccountUsageUserLimit'
        label='Allowed other accounts on the same IP'
        fallback={1}
        min={1}
        max={50}
      />
      <RiskTextField
        {...props}
        name='sameIPMultiAccountUsageBlockMessage'
        label='Same-IP multi-account block message'
      />
    </>
  )
}

function RiskDynamicAndInactiveFields(props: RiskFormProps) {
  return (
    <>
      <RiskSwitchField
        {...props}
        name='dynamicIPChurnEnabled'
        label='Enable dynamic IP churn detection'
        description='Freeze keys that rotate across too many source IPs within a short window.'
      />
      <RiskNumberField
        {...props}
        name='dynamicIPChurnWindowMinutes'
        label='Dynamic IP window (minutes)'
        fallback={30}
        min={1}
        max={1440}
      />
      <RiskNumberField
        {...props}
        name='dynamicIPChurnDistinctIPLimit'
        label='Distinct IP threshold per key'
        fallback={6}
        min={2}
        max={100}
      />
      <RiskTextField
        {...props}
        name='dynamicIPChurnBlockMessage'
        label='Dynamic IP block message'
      />
      <RiskSwitchField
        {...props}
        name='inactiveTokenDisableEnabled'
        label='Require re-verification after long inactivity'
        description='Periodically mark long-inactive users as high risk and force them to recreate keys.'
      />
      <RiskNumberField
        {...props}
        name='inactiveTokenDisableDays'
        label='Inactive threshold (days)'
        fallback={7}
        min={1}
        max={365}
      />
      <RiskTextField
        {...props}
        name='inactiveTokenDisableReason'
        label='Inactive-user re-verification reason'
      />
    </>
  )
}

export function RiskControlSection({ defaultValues }: Props) {
  const { t } = useTranslation()
  const form = useForm<Values>({
    resolver: zodResolver(createSchema()),
    defaultValues: buildDefaults(defaultValues),
  })
  const updateOption = useUpdateOption()

  useEffect(() => {
    form.reset(buildDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (values: Values) => {
    for (const [field, optionKey] of Object.entries(optionKeyMap) as Array<
      [keyof Values, string]
    >) {
      await updateOption.mutateAsync({
        key: optionKey,
        value: String(values[field] ?? ''),
      })
    }
  }

  return (
    <SettingsSection title={t('Risk Control')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel={t('Save risk control settings')}
          />
          <RiskMasterFields form={form} t={t} />
          <RiskRegistrationFields form={form} t={t} />
          <RiskRequestIpFields form={form} t={t} />
          <RiskDynamicAndInactiveFields form={form} t={t} />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
