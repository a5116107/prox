/*
Copyright (C) 2023-2026 QuantumNous
*/
import { type UseFormReturn } from 'react-hook-form'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { safeNumberFieldProps } from '../utils/numeric-field'
import { type MembershipRiskValues } from './membership-risk-model'
import { type OpsTranslate } from './ops-i18n'

type MembershipRiskSettingsFormProps = {
  t: OpsTranslate
  form: UseFormReturn<MembershipRiskValues>
  onSubmit: (values: MembershipRiskValues) => Promise<void>
  saving: boolean
}

const benefitGateFields: Array<[keyof MembershipRiskValues, string]> = [
  ['freezeCommunityTokensAfterGrace', 'Freeze community tokens after grace'],
  ['revokeCommunityAccessAfterGrace', 'Revoke community access after grace'],
  ['blockCheckinOnLeft', 'Block check-in rewards after leave'],
  ['blockGameRewardOnLeft', 'Block game rewards after leave'],
  ['blockInviteRewardOnLeft', 'Block invite rewards after leave'],
  ['blockCampaignBonusOnLeft', 'Block campaign bonuses after leave'],
  ['notifyUserOnLeft', 'Notify user on leave risk'],
  ['notifyAdminOnBulkLeft', 'Notify admin on bulk leave risk'],
  ['qqEventsEnabled', 'Accept QQ membership events'],
  ['tgEventsEnabled', 'Accept Telegram membership events'],
  ['scheduledRecheckEnabled', 'Enable scheduled recheck'],
]

type RiskToggleGroupProps = Pick<MembershipRiskSettingsFormProps, 't' | 'form'>

function RiskStateToggles({ t, form }: RiskToggleGroupProps) {
  return (
    <>
      <FormField
        control={form.control}
        name='enabled'
        render={({ field }) => (
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Enable membership risk control')}</FormLabel>
              <FormDescription>
                {t(
                  'Evaluate QQ/TG membership before issuing community benefits'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />
      <FormField
        control={form.control}
        name='dryRun'
        render={({ field }) => (
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Observe only')}</FormLabel>
              <FormDescription>
                {t('Record what would happen without disabling access')}
              </FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />
    </>
  )
}

function RiskAccessToggles({ t, form }: RiskToggleGroupProps) {
  return (
    <>
      <FormField
        control={form.control}
        name='paidBypassEnabled'
        render={({ field }) => (
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Paid account exception')}</FormLabel>
              <FormDescription>
                {t(
                  'Paid subscriptions and paid groups keep paid access independent of group membership'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />
      <FormField
        control={form.control}
        name='autoRestoreOnRejoin'
        render={({ field }) => (
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t('Restore after rejoining')}</FormLabel>
              <FormDescription>
                {t(
                  'Join events restore group benefit eligibility automatically'
                )}
              </FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />
    </>
  )
}

function RiskGateCard({
  t,
  form,
}: Pick<MembershipRiskSettingsFormProps, 't' | 'form'>) {
  return (
    <Card className='lg:col-span-2'>
      <CardHeader>
        <CardTitle>{t('Risk gate')}</CardTitle>
        <CardDescription>
          {t('Control the state machine, grace window, and paid-user bypass.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-6 lg:grid-cols-2'>
        <RiskStateToggles t={t} form={form} />
        <RiskAccessToggles t={t} form={form} />
        <FormField
          control={form.control}
          name='graceHours'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Hours before restrictions')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={720}
                  step={1}
                  {...safeNumberFieldProps(field)}
                />
              </FormControl>
              <FormDescription>
                {t('Time before a leave event becomes expired')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='eventSecret'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Membership update verification key')}</FormLabel>
              <FormControl>
                <Input type='password' autoComplete='new-password' {...field} />
              </FormControl>
              <FormDescription>
                {t(
                  'Used to confirm that QQ/TG membership updates come from the trusted bot service'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </CardContent>
    </Card>
  )
}

function BenefitGatesCard({
  t,
  form,
}: Pick<MembershipRiskSettingsFormProps, 't' | 'form'>) {
  return (
    <Card className='lg:col-span-2'>
      <CardHeader>
        <CardTitle>{t('Benefit gates')}</CardTitle>
        <CardDescription>
          {t(
            'Choose which free/community benefits follow dynamic group qualification.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4 md:grid-cols-2'>
        {benefitGateFields.map(([name, label]) => (
          <FormField
            key={name}
            control={form.control}
            name={name}
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t(label)}</FormLabel>
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
        <FormField
          control={form.control}
          name='scheduledRecheckIntervalHours'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Scheduled recheck interval hours')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={720}
                  step={1}
                  {...safeNumberFieldProps(field)}
                />
              </FormControl>
              <FormDescription />
              <FormMessage />
            </FormItem>
          )}
        />
      </CardContent>
    </Card>
  )
}

export function MembershipRiskSettingsForm({
  t,
  form,
  onSubmit,
  saving,
}: MembershipRiskSettingsFormProps) {
  return (
    <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
      <SettingsPageFormActions
        onSave={form.handleSubmit(onSubmit)}
        isSaving={saving}
        isSaveDisabled={!form.formState.isDirty}
        saveLabel={t('Save membership risk settings')}
      />
      <RiskGateCard t={t} form={form} />
      <BenefitGatesCard t={t} form={form} />
    </SettingsForm>
  )
}
