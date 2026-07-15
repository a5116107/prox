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
import type { Dispatch, SetStateAction } from 'react'
import { getPrimaryPlatformDisplayName } from '@/hooks/use-access-control'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import {
  AccessControlOverview,
  type AccessControlOverviewProps,
} from './access-control-overview'

export type AccessControlValues = {
  enabled: boolean
  primaryPlatform: string
  primaryGroupIds: string
  communityGroupIds: string
  communityOnlyGroups: string
  fullAccessGroups: string
  paidBypassGroups: string
  paidUserGroups: string
  allowPaidBypass: boolean
  allowAdminBypass: boolean
  checkOnLogin: boolean
  blockTokenCreate: boolean
  blockTokenEnable: boolean
  enforceRequestTime: boolean
  freezeLegacyTokens: boolean
  autoRestoreCompliantTokens: boolean
  stateCacheTTLSeconds: number
  communityJoinURL: string
  primaryJoinURL: string
  denyMessage: string
  upgradeMessage: string
  rewardSoftFloorQuota: number
  rewardHardFloorQuota: number
  dailySiteRewardCap: number
  dailyUserRewardCap: number
}

type AccessControlSetValue = <K extends keyof AccessControlValues>(
  key: K,
  value: AccessControlValues[K]
) => void

type AccessOverrideModeOption = {
  value: string
  label: string
}

type AccessControlToolState = {
  scanLoading: boolean
  runScan: (dryRun: boolean) => Promise<void>
  checkUserId: string
  setCheckUserId: Dispatch<SetStateAction<string>>
  checkUser: () => Promise<void>
  checkResult: unknown
  overrideUserId: string
  setOverrideUserId: Dispatch<SetStateAction<string>>
  overrideMode: string
  setOverrideMode: Dispatch<SetStateAction<string>>
  accessOverrideModeOptions: AccessOverrideModeOption[]
  overrideGroups: string
  setOverrideGroups: Dispatch<SetStateAction<string>>
  overrideReason: string
  setOverrideReason: Dispatch<SetStateAction<string>>
  submitOverride: () => Promise<void>
}

type AccessControlEditorViewProps = AccessControlOverviewProps & {
  embedded: boolean
  values: AccessControlValues
  setValue: AccessControlSetValue
  tools: AccessControlToolState
  saveAll: () => Promise<void>
  saving: boolean
  updateOptionsPending: boolean
}

const primaryPlatformOptions = ['qq', 'tg', 'community'] as const

function editorNumberValue(value: string) {
  const parsed = Number(value)
  return parsed ? parsed : 0
}

type PolicyEditorProps = Pick<
  AccessControlEditorViewProps,
  't' | 'embedded' | 'values' | 'setValue' | 'editorPrimaryPlatformLabel'
>

type StringField =
  | 'primaryGroupIds'
  | 'communityGroupIds'
  | 'communityOnlyGroups'
  | 'fullAccessGroups'
  | 'paidBypassGroups'
  | 'paidUserGroups'
  | 'communityJoinURL'
  | 'primaryJoinURL'
  | 'denyMessage'
  | 'upgradeMessage'

type NumberField =
  | 'rewardSoftFloorQuota'
  | 'rewardHardFloorQuota'
  | 'dailySiteRewardCap'
  | 'dailyUserRewardCap'

function ToggleSetting({
  label,
  checked,
  onCheckedChange,
}: {
  label: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <label className='flex items-center justify-between rounded-lg border p-3'>
      <span>{label}</span>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </label>
  )
}

function AccessPolicyToggles({ t, values, setValue }: PolicyEditorProps) {
  return (
    <div className='grid gap-4 md:grid-cols-2'>
      <ToggleSetting
        label={t('Enable access control')}
        checked={values.enabled}
        onCheckedChange={(checked) => setValue('enabled', checked)}
      />
      <ToggleSetting
        label={t('Allow paid-user bypass')}
        checked={values.allowPaidBypass}
        onCheckedChange={(checked) => setValue('allowPaidBypass', checked)}
      />
      <ToggleSetting
        label={t('Allow admin bypass')}
        checked={values.allowAdminBypass}
        onCheckedChange={(checked) => setValue('allowAdminBypass', checked)}
      />
      <ToggleSetting
        label={t('Force access check on login')}
        checked={values.checkOnLogin}
        onCheckedChange={(checked) => setValue('checkOnLogin', checked)}
      />
      <ToggleSetting
        label={t('Check request time')}
        checked={values.enforceRequestTime}
        onCheckedChange={(checked) => setValue('enforceRequestTime', checked)}
      />
      <ToggleSetting
        label={t('Block token creation')}
        checked={values.blockTokenCreate}
        onCheckedChange={(checked) => setValue('blockTokenCreate', checked)}
      />
      <ToggleSetting
        label={t('Block token enable')}
        checked={values.blockTokenEnable}
        onCheckedChange={(checked) => setValue('blockTokenEnable', checked)}
      />
      <ToggleSetting
        label={t('Freeze legacy keys')}
        checked={values.freezeLegacyTokens}
        onCheckedChange={(checked) => setValue('freezeLegacyTokens', checked)}
      />
      <ToggleSetting
        label={t('Auto restore compliant keys')}
        checked={values.autoRestoreCompliantTokens}
        onCheckedChange={(checked) =>
          setValue('autoRestoreCompliantTokens', checked)
        }
      />
    </div>
  )
}

function AccessPolicyIdentity({
  t,
  values,
  setValue,
  editorPrimaryPlatformLabel,
}: PolicyEditorProps) {
  return (
    <div className='grid gap-4 md:grid-cols-2'>
      <div className='grid gap-2'>
        <div className='text-sm font-medium'>{t('Primary platform')}</div>
        <Select
          value={values.primaryPlatform}
          onValueChange={(value) => value && setValue('primaryPlatform', value)}
        >
          <SelectTrigger>
            <SelectValue placeholder={t('Select primary platform')}>
              {editorPrimaryPlatformLabel}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {primaryPlatformOptions.map((platform) => (
              <SelectItem key={platform} value={platform}>
                {getPrimaryPlatformDisplayName(platform, t)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className='grid gap-2'>
        <div className='text-sm font-medium'>
          {t('State cache TTL (seconds)')}
        </div>
        <Input
          type='number'
          value={values.stateCacheTTLSeconds}
          onChange={(event) =>
            setValue(
              'stateCacheTTLSeconds',
              editorNumberValue(event.target.value)
            )
          }
        />
      </div>
    </div>
  )
}

function ListEditor({
  label,
  description,
  placeholder,
  field,
  values,
  setValue,
}: Pick<PolicyEditorProps, 'values' | 'setValue'> & {
  label: string
  description: string
  placeholder: string
  field: StringField
}) {
  return (
    <div className='grid gap-2'>
      <div className='text-sm font-medium'>{label}</div>
      <div className='text-muted-foreground text-xs'>{description}</div>
      <Textarea
        rows={4}
        value={values[field]}
        onChange={(event) => setValue(field, event.target.value)}
        placeholder={placeholder}
      />
    </div>
  )
}

function AccessGroupEditors({
  t,
  values,
  setValue,
  editorPrimaryPlatformLabel,
}: PolicyEditorProps) {
  const sharedProps = { values, setValue }
  return (
    <div className='grid gap-4 md:grid-cols-2'>
      <ListEditor
        {...sharedProps}
        field='primaryGroupIds'
        label={t('Primary group IDs')}
        description={t(
          'Bind any of these {{platform}} groups to unlock the full-access API groups.',
          { platform: editorPrimaryPlatformLabel }
        )}
        placeholder={t('One room / group ID per line')}
      />
      <ListEditor
        {...sharedProps}
        field='communityGroupIds'
        label={t('Community room IDs')}
        description={t(
          'Join any of these community rooms to unlock the community-only fallback API groups.'
        )}
        placeholder={t('One room / group ID per line')}
      />
      <ListEditor
        {...sharedProps}
        field='communityOnlyGroups'
        label={t('Community-only usable API groups')}
        description={t(
          'Users who only finish community binding can create and use only these API groups.'
        )}
        placeholder={t('One API group name per line')}
      />
      <ListEditor
        {...sharedProps}
        field='fullAccessGroups'
        label={t('Full-access usable API groups')}
        description={t(
          'Users with primary-group binding, paid bypass, admin bypass, or manual full access can use these API groups.'
        )}
        placeholder={t('Leave empty to inherit all package groups')}
      />
      <ListEditor
        {...sharedProps}
        field='paidBypassGroups'
        label={t('Paid bypass groups')}
        description={t(
          'Optional: restrict paid-bypass users to these API groups. Leave empty to follow the full-access group list.'
        )}
        placeholder={t('One API group name per line')}
      />
      <ListEditor
        {...sharedProps}
        field='paidUserGroups'
        label={t('Paid user groups')}
        description={t(
          'These are account / package groups that should bypass binding checks and receive full-access treatment.'
        )}
        placeholder={t('User package groups that unlock full access')}
      />
    </div>
  )
}

function StringInput({
  field,
  placeholder,
  values,
  setValue,
}: Pick<PolicyEditorProps, 'values' | 'setValue'> & {
  field: StringField
  placeholder: string
}) {
  return (
    <Input
      value={values[field]}
      onChange={(event) => setValue(field, event.target.value)}
      placeholder={placeholder}
    />
  )
}

function StringArea({
  field,
  placeholder,
  values,
  setValue,
}: Pick<PolicyEditorProps, 'values' | 'setValue'> & {
  field: StringField
  placeholder: string
}) {
  return (
    <Textarea
      rows={3}
      value={values[field]}
      onChange={(event) => setValue(field, event.target.value)}
      placeholder={placeholder}
    />
  )
}

function NumberInput({
  field,
  placeholder,
  values,
  setValue,
}: Pick<PolicyEditorProps, 'values' | 'setValue'> & {
  field: NumberField
  placeholder: string
}) {
  return (
    <Input
      type='number'
      value={values[field]}
      onChange={(event) =>
        setValue(field, editorNumberValue(event.target.value))
      }
      placeholder={placeholder}
    />
  )
}

function AccessPolicyMessages({ t, values, setValue }: PolicyEditorProps) {
  const sharedProps = { values, setValue }
  return (
    <>
      <div className='grid gap-4 md:grid-cols-2'>
        <StringInput
          {...sharedProps}
          field='communityJoinURL'
          placeholder={t('Community join URL')}
        />
        <StringInput
          {...sharedProps}
          field='primaryJoinURL'
          placeholder={t('Primary group join URL')}
        />
      </div>
      <div className='grid gap-4 md:grid-cols-2'>
        <StringArea
          {...sharedProps}
          field='denyMessage'
          placeholder={t('Denied message')}
        />
        <StringArea
          {...sharedProps}
          field='upgradeMessage'
          placeholder={t('Upgrade message')}
        />
      </div>
      <div className='grid gap-4 md:grid-cols-4'>
        <NumberInput
          {...sharedProps}
          field='rewardSoftFloorQuota'
          placeholder={t('Soft floor')}
        />
        <NumberInput
          {...sharedProps}
          field='rewardHardFloorQuota'
          placeholder={t('Hard floor')}
        />
        <NumberInput
          {...sharedProps}
          field='dailySiteRewardCap'
          placeholder={t('Daily site reward cap')}
        />
        <NumberInput
          {...sharedProps}
          field='dailyUserRewardCap'
          placeholder={t('Daily user reward cap')}
        />
      </div>
    </>
  )
}

function AccessPolicyEditor(props: PolicyEditorProps) {
  const { t, embedded } = props
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {embedded ? t('Access rules for this site') : t('Edit access policy')}
        </CardTitle>
        <CardDescription>
          {t(
            'Decide which groups stay community-only, which groups require the main field, and what users should see when a required binding is still missing.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4'>
        <AccessPolicyToggles {...props} />
        <AccessPolicyIdentity {...props} />
        <AccessGroupEditors {...props} />
        <AccessPolicyMessages {...props} />
      </CardContent>
    </Card>
  )
}

type AccessToolsProps = AccessControlToolState &
  Pick<AccessControlOverviewProps, 't' | 'status'>

function AccessControlTools(props: AccessToolsProps) {
  const {
    t,
    status,
    scanLoading,
    runScan,
    checkUserId,
    setCheckUserId,
    checkUser,
    checkResult,
    overrideUserId,
    setOverrideUserId,
    overrideMode,
    setOverrideMode,
    accessOverrideModeOptions,
    overrideGroups,
    setOverrideGroups,
    overrideReason,
    setOverrideReason,
    submitOverride,
  } = props
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Check live users and handle exceptions')}</CardTitle>
        <CardDescription>
          {t(
            'Run a live scan before freezing old keys, inspect one user when something looks wrong, and use manual override only for rare exceptions.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4 md:grid-cols-2'>
        <div className='grid gap-3 rounded-lg border p-4'>
          <div className='font-medium'>{t('Scan & freeze')}</div>
          <div className='flex flex-wrap gap-2'>
            <Button
              variant='outline'
              disabled={scanLoading}
              onClick={() => void runScan(true)}
            >
              {t('Dry run')}
            </Button>
            <Button disabled={scanLoading} onClick={() => void runScan(false)}>
              {t('Apply scan')}
            </Button>
          </div>
          {status?.scan_result != null ? (
            <pre className='bg-muted overflow-auto rounded-md p-3 text-xs'>
              {JSON.stringify(status.scan_result, null, 2)}
            </pre>
          ) : null}
        </div>
        <div className='grid gap-3 rounded-lg border p-4'>
          <div className='font-medium'>{t('Check user')}</div>
          <div className='flex gap-2'>
            <Input
              value={checkUserId}
              onChange={(event) => setCheckUserId(event.target.value)}
              placeholder={t('User ID')}
            />
            <Button variant='outline' onClick={() => void checkUser()}>
              {t('Check')}
            </Button>
          </div>
          {checkResult != null ? (
            <pre className='bg-muted overflow-auto rounded-md p-3 text-xs'>
              {JSON.stringify(checkResult, null, 2)}
            </pre>
          ) : null}
        </div>
        <div className='grid gap-3 rounded-lg border p-4 md:col-span-2'>
          <div className='font-medium'>{t('Manual override')}</div>
          <div className='grid gap-3 md:grid-cols-2'>
            <Input
              value={overrideUserId}
              onChange={(event) => setOverrideUserId(event.target.value)}
              placeholder={t('Target user ID')}
            />
            <Select
              value={overrideMode}
              onValueChange={(value) => value && setOverrideMode(value)}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Access override mode')}>
                  {
                    accessOverrideModeOptions.find(
                      (option) => option.value === overrideMode
                    )?.label
                  }
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {accessOverrideModeOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Textarea
              rows={3}
              value={overrideGroups}
              onChange={(event) => setOverrideGroups(event.target.value)}
              placeholder={t('Custom groups, one per line')}
            />
            <Textarea
              rows={3}
              value={overrideReason}
              onChange={(event) => setOverrideReason(event.target.value)}
              placeholder={t('Reason')}
            />
          </div>
          <div>
            <Button onClick={() => void submitOverride()}>
              {t('Apply override')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function AccessControlEditorView({
  t,
  embedded,
  status,
  counts,
  savedView,
  savedStates,
  runtimeView,
  runtimePrimaryPlatformLabel,
  editorPrimaryPlatformLabel,
  accessWarnings,
  groupCatalog,
  communityOnlyGroupSet,
  fullAccessGroupSet,
  onToggleBucketGroup: toggleBucketGroup,
  values,
  setValue,
  tools,
  saveAll,
  saving,
  updateOptionsPending,
}: AccessControlEditorViewProps) {
  return (
    <SettingsSection title={t('Access Control')}>
      {!embedded ? (
        <AccessControlOverview
          t={t}
          status={status}
          counts={counts}
          savedView={savedView}
          savedStates={savedStates}
          runtimeView={runtimeView}
          runtimePrimaryPlatformLabel={runtimePrimaryPlatformLabel}
          editorPrimaryPlatformLabel={editorPrimaryPlatformLabel}
          accessWarnings={accessWarnings}
          groupCatalog={groupCatalog}
          communityOnlyGroupSet={communityOnlyGroupSet}
          fullAccessGroupSet={fullAccessGroupSet}
          onToggleBucketGroup={toggleBucketGroup}
        />
      ) : null}
      <AccessPolicyEditor
        t={t}
        embedded={embedded}
        values={values}
        setValue={setValue}
        editorPrimaryPlatformLabel={editorPrimaryPlatformLabel}
      />
      {!embedded ? (
        <AccessControlTools t={t} status={status} {...tools} />
      ) : null}
      <SettingsPageFormActions
        onSave={saveAll}
        isSaving={saving || updateOptionsPending}
        isSaveDisabled={saving || updateOptionsPending}
        saveLabel={embedded ? t('Save access rules') : undefined}
        inline={embedded}
        className={embedded ? 'justify-start' : undefined}
      />
    </SettingsSection>
  )
}
