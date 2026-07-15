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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import type { OpsTranslate } from './ops-i18n'

export type GroupCatalogEntry = {
  value: string
  desc: string
  ratio?: number | string
}

export type AccessRuntimeView = {
  enabled: boolean
  primaryPlatform: string
  blockTokenCreate: boolean
  blockTokenEnable: boolean
  allowPaidBypass: boolean
  allowAdminBypass: boolean
  communityJoinURL: string
  primaryJoinURL: string
  denyMessage: string
  upgradeMessage: string
  rewardSoftFloorQuota: number
  rewardHardFloorQuota: number
  dailySiteRewardCap: number
  dailyUserRewardCap: number
  activeFrozenUsers: number
  activeFrozenTokens: number
}

export type AccessControlRuntimeStatus = {
  enabled?: boolean
  primary_platform?: unknown
  block_token_create?: boolean
  block_token_enable?: boolean
  allow_paid_bypass?: boolean
  allow_admin_bypass?: boolean
  community_join_url?: unknown
  primary_join_url?: unknown
  deny_message?: unknown
  upgrade_message?: unknown
  reward_soft_floor_quota?: unknown
  reward_hard_floor_quota?: unknown
  daily_site_reward_cap?: unknown
  daily_user_reward_cap?: unknown
  active_frozen_users?: unknown
  active_frozen_tokens?: unknown
  counts?: Record<string, number>
  scan_result?: unknown
}

export type SavedAccessView = {
  enabled: boolean | null
  primaryPlatform: string
  blockTokenCreate: boolean | null
  blockTokenEnable: boolean | null
  allowPaidBypass: boolean | null
  allowAdminBypass: boolean | null
  communityGroupIds: string[]
  primaryGroupIds: string[]
  communityOnlyGroups: string[]
  fullAccessGroups: string[]
  communityJoinURL: string
  primaryJoinURL: string
  denyMessage: string
  upgradeMessage: string
  rewardSoftFloorQuota: number | null
  rewardHardFloorQuota: number | null
  dailySiteRewardCap: number | null
  dailyUserRewardCap: number | null
}

type SavedFieldState = 'missing' | 'empty' | 'value'

type SavedAccessStates = {
  communityGroupIds: SavedFieldState
  primaryGroupIds: SavedFieldState
  communityOnlyGroups: SavedFieldState
  fullAccessGroups: SavedFieldState
  communityJoinURL: SavedFieldState
  primaryJoinURL: SavedFieldState
  denyMessage: SavedFieldState
  upgradeMessage: SavedFieldState
}

type BucketKey = 'communityOnlyGroups' | 'fullAccessGroups'

export type AccessControlOverviewProps = {
  t: OpsTranslate
  status: AccessControlRuntimeStatus | null
  counts: Record<string, number>
  savedView: SavedAccessView
  savedStates: SavedAccessStates
  runtimeView: AccessRuntimeView
  runtimePrimaryPlatformLabel: string
  editorPrimaryPlatformLabel: string
  accessWarnings: string[]
  groupCatalog: GroupCatalogEntry[]
  communityOnlyGroupSet: Set<string>
  fullAccessGroupSet: Set<string>
  onToggleBucketGroup: (key: BucketKey, groupName: string) => void
}

function formatSavedBool(value: boolean | null, t: OpsTranslate) {
  if (value === null) return t('Not saved')
  return value ? t('Enabled') : t('Disabled')
}

function formatSavedTextState(
  state: SavedFieldState,
  value: string,
  t: OpsTranslate
) {
  if (state === 'missing') return t('Not saved')
  if (state === 'empty') return t('Saved empty')
  return value
}

function formatSavedListState(
  items: string[],
  state: SavedFieldState,
  t: OpsTranslate
) {
  if (items.length > 0) return items.join(', ')
  return state === 'missing' ? t('Not saved') : t('Saved empty')
}

function formatSavedNumber(value: number | null, t: OpsTranslate) {
  if (value === null) return t('Not saved')
  return String(value)
}

type StatusSummaryProps = Pick<
  AccessControlOverviewProps,
  | 't'
  | 'status'
  | 'counts'
  | 'savedView'
  | 'runtimeView'
  | 'runtimePrimaryPlatformLabel'
>

function StatusSummary({
  t,
  status,
  counts,
  savedView,
  runtimeView,
  runtimePrimaryPlatformLabel,
}: StatusSummaryProps) {
  return (
    <Card className='border-primary/20'>
      <CardHeader>
        <CardTitle>
          {t('Who can create and use which keys right now')}
        </CardTitle>
        <CardDescription>
          {t(
            'This page now separates saved configuration from live user-state counters so you can immediately see who is only on community keys, who is fully unlocked, and which prompts users will actually receive.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-3 md:grid-cols-6'>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>{t('Enabled')}</div>
          <div className='font-medium'>
            {typeof status?.enabled === 'boolean'
              ? status.enabled
                ? t('Enabled')
                : t('Disabled')
              : formatSavedBool(savedView.enabled, t)}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Primary platform')}
          </div>
          <div className='font-medium'>{runtimePrimaryPlatformLabel}</div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Community-only users')}
          </div>
          <div className='font-medium'>{counts.community_only || 0}</div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('No access users')}
          </div>
          <div className='font-medium'>{counts.none || 0}</div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Saved community-only groups')}
          </div>
          <div className='font-medium'>
            {savedView.communityOnlyGroups.length}
          </div>
        </div>
        <div className='rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Frozen keys')}
          </div>
          <div className='font-medium'>{runtimeView.activeFrozenTokens}</div>
        </div>
      </CardContent>
    </Card>
  )
}

type RuntimeSummaryProps = Pick<
  AccessControlOverviewProps,
  't' | 'status' | 'savedView' | 'runtimeView' | 'runtimePrimaryPlatformLabel'
>

function RuntimeSummary({
  t,
  status,
  savedView,
  runtimeView,
  runtimePrimaryPlatformLabel,
}: RuntimeSummaryProps) {
  const boolLabel = (runtime: boolean | undefined, saved: boolean | null) =>
    typeof runtime === 'boolean'
      ? runtime
        ? t('Enabled')
        : t('Disabled')
      : formatSavedBool(saved, t)

  return (
    <div className='rounded-xl border p-4'>
      <div className='text-sm font-semibold'>{t('Live runtime summary')}</div>
      <div className='mt-3 space-y-3 text-sm'>
        <SummaryRow label={t('Primary platform')}>
          {runtimePrimaryPlatformLabel}
        </SummaryRow>
        <SummaryRow label={t('Block token creation')}>
          {boolLabel(status?.block_token_create, savedView.blockTokenCreate)}
        </SummaryRow>
        <SummaryRow label={t('Block token enable')}>
          {boolLabel(status?.block_token_enable, savedView.blockTokenEnable)}
        </SummaryRow>
        <SummaryRow label={t('Paid bypass')}>
          {boolLabel(status?.allow_paid_bypass, savedView.allowPaidBypass)}
        </SummaryRow>
        <SummaryRow label={t('Admin bypass')}>
          {boolLabel(status?.allow_admin_bypass, savedView.allowAdminBypass)}
        </SummaryRow>
        <SummaryRow label={t('Frozen users')}>
          {runtimeView.activeFrozenUsers}
        </SummaryRow>
      </div>
    </div>
  )
}

function SummaryRow({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className='flex items-start justify-between gap-3'>
      <span className='text-muted-foreground'>{label}</span>
      <span className='font-medium'>{children}</span>
    </div>
  )
}

type SavedMappingProps = Pick<
  AccessControlOverviewProps,
  't' | 'savedView' | 'savedStates'
>

function SavedMapping({ t, savedView, savedStates }: SavedMappingProps) {
  return (
    <div className='rounded-xl border p-4'>
      <div className='text-sm font-semibold'>
        {t('Saved mapping actually on record')}
      </div>
      <div className='mt-3 space-y-3 text-sm'>
        <SavedList
          label={t('Community room IDs')}
          items={savedView.communityGroupIds}
          state={savedStates.communityGroupIds}
          t={t}
        />
        <SavedList
          label={t('Primary group IDs')}
          items={savedView.primaryGroupIds}
          state={savedStates.primaryGroupIds}
          t={t}
        />
        <SavedList
          label={t('Community-only usable API groups')}
          items={savedView.communityOnlyGroups}
          state={savedStates.communityOnlyGroups}
          t={t}
        />
        <SavedList
          label={t('Full-access usable API groups')}
          items={savedView.fullAccessGroups}
          state={savedStates.fullAccessGroups}
          t={t}
        />
      </div>
    </div>
  )
}

function SavedList({
  label,
  items,
  state,
  t,
}: {
  label: string
  items: string[]
  state: SavedFieldState
  t: OpsTranslate
}) {
  return (
    <div>
      <div className='text-muted-foreground'>{label}</div>
      <div className='mt-1 font-medium'>
        {formatSavedListState(items, state, t)}
      </div>
    </div>
  )
}

type UserFacingSummaryProps = Pick<
  AccessControlOverviewProps,
  't' | 'status' | 'savedView' | 'savedStates' | 'runtimeView'
>

function UserFacingSummary({
  t,
  status,
  savedView,
  savedStates,
  runtimeView,
}: UserFacingSummaryProps) {
  return (
    <div className='rounded-xl border p-4'>
      <div className='text-sm font-semibold'>
        {t('What users will actually see')}
      </div>
      <div className='mt-3 space-y-3 text-sm'>
        <SavedText
          label={t('Denied message')}
          runtimeValue={runtimeView.denyMessage}
          savedValue={savedView.denyMessage}
          state={savedStates.denyMessage}
          t={t}
        />
        <SavedText
          label={t('Upgrade message')}
          runtimeValue={runtimeView.upgradeMessage}
          savedValue={savedView.upgradeMessage}
          state={savedStates.upgradeMessage}
          t={t}
        />
        <div className='grid gap-2 md:grid-cols-2'>
          <SavedText
            label={t('Community join URL')}
            runtimeValue={runtimeView.communityJoinURL}
            savedValue={savedView.communityJoinURL}
            state={savedStates.communityJoinURL}
            t={t}
            breakAll
          />
          <SavedText
            label={t('Primary join URL')}
            runtimeValue={runtimeView.primaryJoinURL}
            savedValue={savedView.primaryJoinURL}
            state={savedStates.primaryJoinURL}
            t={t}
            breakAll
          />
        </div>
        <RewardSummary
          t={t}
          status={status}
          savedView={savedView}
          runtimeView={runtimeView}
        />
      </div>
    </div>
  )
}

function SavedText({
  label,
  runtimeValue,
  savedValue,
  state,
  t,
  breakAll = false,
}: {
  label: string
  runtimeValue: string
  savedValue: string
  state: SavedFieldState
  t: OpsTranslate
  breakAll?: boolean
}) {
  return (
    <div>
      <div className='text-muted-foreground'>{label}</div>
      <div
        className={breakAll ? 'mt-1 font-medium break-all' : 'mt-1 leading-6'}
      >
        {runtimeValue || formatSavedTextState(state, savedValue, t)}
      </div>
    </div>
  )
}

type RewardSummaryProps = Pick<
  AccessControlOverviewProps,
  't' | 'status' | 'savedView' | 'runtimeView'
>

function RewardSummary({
  t,
  status,
  savedView,
  runtimeView,
}: RewardSummaryProps) {
  const rewards = [
    {
      label: t('Reward soft floor'),
      live: status?.reward_soft_floor_quota,
      runtime: runtimeView.rewardSoftFloorQuota,
      saved: savedView.rewardSoftFloorQuota,
    },
    {
      label: t('Reward hard floor'),
      live: status?.reward_hard_floor_quota,
      runtime: runtimeView.rewardHardFloorQuota,
      saved: savedView.rewardHardFloorQuota,
    },
    {
      label: t('Daily site reward cap'),
      live: status?.daily_site_reward_cap,
      runtime: runtimeView.dailySiteRewardCap,
      saved: savedView.dailySiteRewardCap,
    },
    {
      label: t('Daily user reward cap'),
      live: status?.daily_user_reward_cap,
      runtime: runtimeView.dailyUserRewardCap,
      saved: savedView.dailyUserRewardCap,
    },
  ]

  return (
    <div className='grid gap-2 md:grid-cols-2'>
      {rewards.map((reward) => (
        <div key={reward.label}>
          <div className='text-muted-foreground'>{reward.label}</div>
          <div className='mt-1 font-medium'>
            {typeof reward.live === 'number'
              ? reward.runtime
              : formatSavedNumber(reward.saved, t)}
          </div>
        </div>
      ))}
    </div>
  )
}

type SavedRuntimeCardProps = Pick<
  AccessControlOverviewProps,
  | 't'
  | 'status'
  | 'savedView'
  | 'savedStates'
  | 'runtimeView'
  | 'runtimePrimaryPlatformLabel'
  | 'accessWarnings'
>

function SavedRuntimeCard(props: SavedRuntimeCardProps) {
  const { t, accessWarnings } = props
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Saved policy vs live runtime')}</CardTitle>
        <CardDescription>
          {t(
            'Configured values explain what the site should do; runtime counters explain what is happening now. They are shown separately here so the page stops looking contradictory.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4 xl:grid-cols-3'>
        <RuntimeSummary {...props} />
        <SavedMapping {...props} />
        <UserFacingSummary {...props} />
      </CardContent>
      {accessWarnings.length > 0 ? (
        <CardContent className='pt-0'>
          <div className='rounded-xl border border-amber-300/60 bg-amber-500/10 p-4 text-sm'>
            <div className='font-medium'>
              {t('Current mismatches that need attention')}
            </div>
            <ul className='mt-2 list-disc space-y-1 pl-5'>
              {accessWarnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </div>
        </CardContent>
      ) : null}
    </Card>
  )
}

type GroupGuideProps = Pick<
  AccessControlOverviewProps,
  | 't'
  | 'editorPrimaryPlatformLabel'
  | 'groupCatalog'
  | 'communityOnlyGroupSet'
  | 'fullAccessGroupSet'
  | 'onToggleBucketGroup'
>

function GroupMappingGuide(props: GroupGuideProps) {
  const { t, editorPrimaryPlatformLabel, groupCatalog } = props
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Group mapping guide')}</CardTitle>
        <CardDescription>
          {t(
            'Create the package/API group first, then assign it to the right access bucket here. Saving this section updates who can create and use each group immediately.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4'>
        <div className='grid gap-3 md:grid-cols-3'>
          <GuideStep
            title={t('Step 1: Create the group')}
            description={t(
              'Add the new API group in your site’s group ratio / pricing settings first. The group must exist before access control can assign it.'
            )}
          />
          <GuideStep
            title={t('Step 2: Assign the access bucket')}
            description={t(
              'Put community fallback groups into the community-only list, and put premium / full-site groups into the full-access list.'
            )}
          />
          <GuideStep
            title={t('Step 3: Save and verify')}
            description={t(
              'After saving, users immediately see the new policy when they log in, create keys, enable keys, or hit request-time checks.'
            )}
          />
        </div>
        <div className='rounded-lg border p-4 text-sm'>
          <div className='font-medium'>
            {t('Current primary platform')}: {editorPrimaryPlatformLabel}
          </div>
          <div className='text-muted-foreground mt-1'>
            {t(
              'Final unlock rule: users unlock the fallback groups after community binding; then they unlock the full-access groups after joining the main {{platform}} group.',
              { platform: editorPrimaryPlatformLabel }
            )}
          </div>
        </div>
        {groupCatalog.length > 0 ? <GroupCatalog {...props} /> : null}
      </CardContent>
    </Card>
  )
}

function GuideStep({
  title,
  description,
}: {
  title: string
  description: string
}) {
  return (
    <div className='rounded-lg border p-4'>
      <div className='font-medium'>{title}</div>
      <div className='text-muted-foreground mt-1 text-sm'>{description}</div>
    </div>
  )
}

function GroupCatalog({
  t,
  groupCatalog,
  communityOnlyGroupSet,
  fullAccessGroupSet,
  onToggleBucketGroup,
}: GroupGuideProps) {
  return (
    <div className='grid gap-3'>
      <div>
        <div className='text-sm font-medium'>{t('Detected API groups')}</div>
        <div className='text-muted-foreground mt-1 text-sm'>
          {t(
            'Use the quick actions below to place a group into the community-only or full-access bucket.'
          )}
        </div>
      </div>
      <div className='grid gap-3 md:grid-cols-2'>
        {groupCatalog.map((group) => (
          <GroupCatalogCard
            key={group.value}
            t={t}
            group={group}
            inCommunityOnly={communityOnlyGroupSet.has(group.value)}
            inFullAccess={fullAccessGroupSet.has(group.value)}
            onToggleBucketGroup={onToggleBucketGroup}
          />
        ))}
      </div>
    </div>
  )
}

function GroupCatalogCard({
  t,
  group,
  inCommunityOnly,
  inFullAccess,
  onToggleBucketGroup,
}: {
  t: OpsTranslate
  group: GroupCatalogEntry
  inCommunityOnly: boolean
  inFullAccess: boolean
  onToggleBucketGroup: (key: BucketKey, groupName: string) => void
}) {
  return (
    <div className='rounded-lg border p-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='truncate font-medium'>{group.value}</div>
          <div className='text-muted-foreground mt-1 text-sm'>{group.desc}</div>
          <div className='mt-2 flex flex-wrap gap-2'>
            {inCommunityOnly ? (
              <Badge variant='secondary'>{t('Community available')}</Badge>
            ) : null}
            {inFullAccess ? (
              <Badge variant='secondary'>{t('Full access available')}</Badge>
            ) : null}
            {!inCommunityOnly && !inFullAccess ? (
              <Badge variant='outline'>{t('Not assigned yet')}</Badge>
            ) : null}
          </div>
        </div>
        {group.ratio !== undefined && group.ratio !== '' ? (
          <Badge variant='outline'>{group.ratio}x</Badge>
        ) : null}
      </div>
      <div className='mt-3 flex flex-wrap gap-2'>
        <Button
          type='button'
          size='sm'
          variant={inCommunityOnly ? 'default' : 'outline'}
          onClick={() =>
            onToggleBucketGroup('communityOnlyGroups', group.value)
          }
        >
          {inCommunityOnly
            ? t('Remove from community-only')
            : t('Add to community-only')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant={inFullAccess ? 'default' : 'outline'}
          onClick={() => onToggleBucketGroup('fullAccessGroups', group.value)}
        >
          {inFullAccess
            ? t('Remove from full-access')
            : t('Add to full-access')}
        </Button>
      </div>
    </div>
  )
}

export function AccessControlOverview(props: AccessControlOverviewProps) {
  return (
    <>
      <StatusSummary {...props} />
      <SavedRuntimeCard {...props} />
      <GroupMappingGuide {...props} />
    </>
  )
}
