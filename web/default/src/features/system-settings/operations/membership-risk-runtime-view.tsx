/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useMemo, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
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
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  formatMembershipTime,
  membershipStatusOpsTone,
  membershipStatusOptions,
  membershipStatusTone,
  translateMembershipStatus,
  translateUnresolvedAction,
  unresolvedActionTone,
  type MembershipOverview,
  type MembershipState,
  type MembershipUnresolvedOverview,
  type MembershipUnresolvedRecord,
} from './membership-risk-model'
import { type OpsTranslate } from './ops-i18n'
import {
  OpsImpactMatrix,
  OpsModeBanner,
  OpsTimeline,
  type OpsMatrixRow,
  type OpsTimelineItem,
} from './ops-shared'

export type MembershipStateAction = 'restore' | 'bypass' | 'clear-bypass'
export type MembershipBatchAction = 'backfill' | 'demote'

type MembershipRuntimeSnapshot = {
  overview: MembershipOverview | null
  unresolved: MembershipUnresolvedOverview | null
  states: MembershipState[]
  loading: boolean
  isDryRun: boolean
  totalTracked: number
  benefitRows: OpsMatrixRow[]
}

type MembershipRuntimeControls = {
  statusFilter: string
  onStatusFilterChange: (status: string) => void
  manualActionReason: string
  onManualActionReasonChange: (reason: string) => void
  manualBypassHours: number
  onManualBypassHoursChange: (hours: number) => void
  actionLoading: number | null
  batchLoading: MembershipBatchAction | null
}

type MembershipRuntimeActions = {
  refresh: (status: string) => Promise<void>
  refreshWithFeedback: (status: string) => Promise<void>
  runStateAction: (
    state: MembershipState,
    action: MembershipStateAction
  ) => Promise<void>
  runBatchAction: (action: MembershipBatchAction) => Promise<void>
}

type MembershipRiskRuntimeViewProps = {
  t: OpsTranslate
  snapshot: MembershipRuntimeSnapshot
  controls: MembershipRuntimeControls
  actions: MembershipRuntimeActions
}

function StatusBadge({ status, t }: { status: string; t: OpsTranslate }) {
  return (
    <span
      className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-medium ${membershipStatusTone(status)}`}
    >
      {translateMembershipStatus(status, t)}
    </span>
  )
}

function StatTile({
  label,
  value,
  hint,
}: {
  label: string
  value: ReactNode
  hint?: ReactNode
}) {
  return (
    <div className='bg-background/70 rounded-xl border p-3'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 text-xl font-semibold'>{value}</div>
      {hint ? (
        <div className='text-muted-foreground mt-1 text-xs'>{hint}</div>
      ) : null}
    </div>
  )
}

type SummaryProps = Pick<MembershipRiskRuntimeViewProps, 't' | 'snapshot'> & {
  onRefresh: () => void
}

function buildTimelineItems(
  states: MembershipState[],
  t: OpsTranslate
): OpsTimelineItem[] {
  return [...states]
    .sort((left, right) => (right.updated_at || 0) - (left.updated_at || 0))
    .slice(0, 8)
    .map((state) => ({
      time: formatMembershipTime(state.updated_at),
      type: translateMembershipStatus(state.status, t),
      detail: `${(state.source || '—').toUpperCase()} · ${state.room_id || '—'} · ${
        state.new_api_user_id
          ? `#${state.new_api_user_id}`
          : state.external_user_id || '—'
      }`,
      tone: membershipStatusOpsTone(state.status),
    }))
}

function MembershipStatGrid({
  t,
  snapshot,
}: Pick<MembershipRiskRuntimeViewProps, 't' | 'snapshot'>) {
  if (snapshot.loading && !snapshot.overview) {
    return (
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-6'>
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton key={index} className='h-20 rounded-xl' />
        ))}
      </div>
    )
  }

  return (
    <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-6'>
      <StatTile
        label={t('Tracked users')}
        value={snapshot.totalTracked}
        hint={t('All known memberships')}
      />
      <StatTile
        label={t('Active')}
        value={snapshot.overview?.counts?.active || 0}
        hint={t('Can receive group benefits')}
      />
      <StatTile
        label={t('Grace')}
        value={snapshot.overview?.counts?.grace || 0}
        hint={t('Left but still in grace')}
      />
      <StatTile
        label={t('Expired')}
        value={snapshot.overview?.counts?.left_expired || 0}
        hint={t('Group benefits revoked')}
      />
      <StatTile
        label={t('Observed but not linked')}
        value={snapshot.overview?.counts?.unbound_observed || 0}
        hint={t('Observed in the group but not linked to a site account yet')}
      />
      <StatTile
        label={t('Need manual identity mapping')}
        value={snapshot.unresolved?.unresolved_state_count || 0}
        hint={t('Still missing a reliable user mapping')}
      />
    </div>
  )
}

function MembershipRuntimeSummary({ t, snapshot, onRefresh }: SummaryProps) {
  const timelineItems = useMemo(
    () => buildTimelineItems(snapshot.states, t),
    [snapshot.states, t]
  )

  return (
    <>
      <OpsModeBanner
        tone={snapshot.isDryRun ? 'warning' : 'danger'}
        title={
          snapshot.isDryRun
            ? t('Dry run mode is on')
            : t('Live enforcement is on')
        }
        description={
          snapshot.isDryRun
            ? t(
                'Only risk decisions are recorded right now. No check-in, invite, game, or community key benefits are actually revoked.'
              )
            : t(
                'Risk decisions are enforced. Leaving a group can revoke free community benefits after the grace window. Paid benefits stay protected by the paid bypass.'
              )
        }
      >
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={snapshot.loading}
          onClick={onRefresh}
        >
          {t('Refresh')}
        </Button>
      </OpsModeBanner>

      <MembershipStatGrid t={t} snapshot={snapshot} />

      <OpsImpactMatrix
        title={t('Benefit impact by membership state')}
        description={t(
          'What happens to each benefit under the currently saved rules. Derived from the saved risk toggles, not a live feed.'
        )}
        rowHeader={t('Benefit')}
        columns={[t('Active'), t('Grace'), t('Expired'), t('Paid bypass')]}
        rows={snapshot.benefitRows}
        note={
          snapshot.isDryRun
            ? t('In dry run mode none of these revocations actually execute.')
            : t(
                'Paid bypass keeps paid benefits independent of group membership.'
              )
        }
      />

      <OpsTimeline
        title={t('Recent state changes')}
        description={t(
          'Latest membership state updates, derived from the states list above. Not a full event log.'
        )}
        items={timelineItems}
        emptyMessage={
          snapshot.loading ? t('Loading...') : t('No membership state records')
        }
      />
    </>
  )
}

function ManualActionCard({
  t,
  controls,
}: Pick<MembershipRiskRuntimeViewProps, 't' | 'controls'>) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Manual action defaults and impact preview')}</CardTitle>
        <CardDescription>
          {t(
            'Restore, bypass, and clear-bypass actions use the reason and bypass window below. The preview matrix updates from the current form values before you save.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_220px]'>
        <FormItem>
          <FormLabel>{t('Reason written to logs')}</FormLabel>
          <FormControl>
            <Textarea
              rows={4}
              value={controls.manualActionReason}
              onChange={(event) =>
                controls.onManualActionReasonChange(event.target.value)
              }
              placeholder={t(
                'Example: user rejoined the QQ/TG primary group and passed binding verification'
              )}
            />
          </FormControl>
          <FormDescription>
            {t(
              'Avoid hard-coded reasons. Every manual restore or bypass should explain what changed and who verified it.'
            )}
          </FormDescription>
        </FormItem>
        <FormItem>
          <FormLabel>{t('Temporary exception hours')}</FormLabel>
          <FormControl>
            <Input
              type='number'
              min={1}
              max={720}
              step={1}
              value={controls.manualBypassHours}
              onChange={(event) =>
                controls.onManualBypassHoursChange(
                  Number(event.target.value || 72)
                )
              }
            />
          </FormControl>
          <FormDescription>
            {t(
              'Used only when clicking bypass. Restore and clear-bypass use the same reason but no duration.'
            )}
          </FormDescription>
        </FormItem>
      </CardContent>
    </Card>
  )
}

function MembershipStatesToolbar({
  t,
  snapshot,
  controls,
  actions,
}: MembershipRiskRuntimeViewProps) {
  return (
    <div className='flex flex-wrap items-center gap-2'>
      <Button
        type='button'
        size='sm'
        variant='outline'
        disabled={controls.batchLoading !== null}
        onClick={() => void actions.runBatchAction('backfill')}
      >
        {controls.batchLoading === 'backfill'
          ? t('Processing...')
          : t('Repair missing membership records')}
      </Button>
      <Button
        type='button'
        size='sm'
        variant='outline'
        disabled={controls.batchLoading !== null}
        onClick={() => void actions.runBatchAction('demote')}
      >
        {controls.batchLoading === 'demote'
          ? t('Processing...')
          : t('Demote pseudo-active states')}
      </Button>
      <Select
        value={controls.statusFilter || 'all'}
        onValueChange={(value) => {
          const selected = value || 'all'
          const nextStatus = selected === 'all' ? '' : selected
          controls.onStatusFilterChange(nextStatus)
          void actions.refreshWithFeedback(nextStatus)
        }}
      >
        <SelectTrigger className='w-44'>
          <SelectValue placeholder={t('All statuses')} />
        </SelectTrigger>
        <SelectContent>
          {membershipStatusOptions.map((status) => (
            <SelectItem key={status || 'all'} value={status || 'all'}>
              {status || t('All statuses')}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button
        type='button'
        variant='outline'
        disabled={snapshot.loading}
        onClick={() => void actions.refresh(controls.statusFilter)}
      >
        {t('Reload')}
      </Button>
    </div>
  )
}

type MembershipStateRowProps = Pick<
  MembershipRiskRuntimeViewProps,
  't' | 'controls' | 'actions'
> & { state: MembershipState }

function MembershipStateRow({
  t,
  controls,
  actions,
  state,
}: MembershipStateRowProps) {
  return (
    <TableRow>
      <TableCell>
        <div className='font-medium'>
          {state.new_api_user_id ? `#${state.new_api_user_id}` : '—'}
        </div>
        <div className='text-muted-foreground text-xs'>
          {state.external_user_id}
        </div>
      </TableCell>
      <TableCell className='uppercase'>{state.source}</TableCell>
      <TableCell>{state.room_id}</TableCell>
      <TableCell>
        <StatusBadge status={state.status} t={t} />
        {state.bypass_until ? (
          <div className='text-muted-foreground mt-1 text-xs'>
            {t('Bypass until')}: {formatMembershipTime(state.bypass_until)}
          </div>
        ) : null}
      </TableCell>
      <TableCell>{formatMembershipTime(state.grace_until)}</TableCell>
      <TableCell>
        <div>{state.risk_score}</div>
        <div className='text-muted-foreground text-xs'>
          {t('Leaves 30d')}: {state.leave_count_30d || 0}
        </div>
      </TableCell>
      <TableCell>
        <div className='flex flex-wrap justify-end gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={controls.actionLoading === state.id}
            onClick={() => void actions.runStateAction(state, 'restore')}
          >
            {t('Restore')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={controls.actionLoading === state.id}
            onClick={() => void actions.runStateAction(state, 'bypass')}
          >
            {t('Allow temporarily for {{hours}}h', {
              hours: String(controls.manualBypassHours || 72),
            })}
          </Button>
          {state.status === 'manual_bypass' ? (
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={controls.actionLoading === state.id}
              onClick={() => void actions.runStateAction(state, 'clear-bypass')}
            >
              {t('Clear bypass')}
            </Button>
          ) : null}
        </div>
      </TableCell>
    </TableRow>
  )
}

function MembershipStateRows({
  t,
  snapshot,
  controls,
  actions,
}: MembershipRiskRuntimeViewProps) {
  if (snapshot.states.length === 0) {
    return (
      <TableBody>
        <TableRow>
          <TableCell
            colSpan={7}
            className='text-muted-foreground py-8 text-center'
          >
            {snapshot.loading
              ? t('Loading...')
              : t('No membership state records')}
          </TableCell>
        </TableRow>
      </TableBody>
    )
  }

  return (
    <TableBody>
      {snapshot.states.map((state) => (
        <MembershipStateRow
          key={state.id}
          t={t}
          state={state}
          controls={controls}
          actions={actions}
        />
      ))}
    </TableBody>
  )
}

function MembershipStatesCard(props: MembershipRiskRuntimeViewProps) {
  const { t } = props
  return (
    <Card>
      <CardHeader>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <CardTitle>{t('Membership states')}</CardTitle>
            <CardDescription>
              {t('Recent QQ/TG membership qualifications and manual actions.')}
            </CardDescription>
          </div>
          <MembershipStatesToolbar {...props} />
        </div>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Source')}</TableHead>
              <TableHead>{t('Room')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Grace until')}</TableHead>
              <TableHead>{t('Risk')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <MembershipStateRows {...props} />
        </Table>
      </CardContent>
    </Card>
  )
}

type UnresolvedRecordProps = {
  record: MembershipUnresolvedRecord
  t: OpsTranslate
}

function IdentityHints({ record, t }: UnresolvedRecordProps) {
  if (!record.identity_hints?.length) {
    return (
      <span className='text-muted-foreground text-xs'>
        {t('No nickname, card, or username hint was captured yet')}
      </span>
    )
  }

  return (
    <div className='flex flex-wrap gap-2'>
      {record.identity_hints.map((hint) => (
        <span
          key={`${record.state_id}-hint-${hint}`}
          className='bg-muted rounded-full px-2 py-0.5 text-xs'
        >
          {hint}
        </span>
      ))}
    </div>
  )
}

function CandidateUsers({ record, t }: UnresolvedRecordProps) {
  if (record.match_candidates?.length) {
    return (
      <div className='space-y-2'>
        {record.match_candidates.map((candidate) => (
          <div
            key={`${record.state_id}-candidate-${candidate.user_id}`}
            className='bg-muted/40 rounded-lg border px-3 py-2 text-xs'
          >
            <div className='font-medium'>
              #{candidate.user_id} · {candidate.username || '—'}
            </div>
            <div className='text-muted-foreground'>
              {t('Matched by')}: {candidate.matched_by || '—'}
            </div>
            <div className='text-muted-foreground'>
              {t('Community ID')}: {candidate.community_external_user_id || '—'}
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (record.existing_bindings?.length) {
    return (
      <div className='space-y-2'>
        {record.existing_bindings.map((binding, index) => (
          <div
            key={`${record.state_id}-binding-${binding.room_id}-${index}`}
            className='bg-muted/40 rounded-lg border px-3 py-2 text-xs'
          >
            <div className='font-medium'>
              {binding.source?.toUpperCase()} · {binding.room_id || '—'}
            </div>
            <div className='text-muted-foreground'>
              #{binding.new_api_user_id || 0} · {binding.username || '—'}
            </div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <span className='text-muted-foreground text-xs'>
      {t('No unique candidate user yet')}
    </span>
  )
}

function UnresolvedMembershipRow({ record, t }: UnresolvedRecordProps) {
  return (
    <TableRow>
      <TableCell>
        <div className='font-medium uppercase'>{record.source}</div>
        <div className='text-muted-foreground text-xs'>
          {record.room_id || '—'}
        </div>
        <div className='text-muted-foreground text-xs'>
          {t('Latest event')}: {record.latest_event_type || '—'} ·{' '}
          {formatMembershipTime(record.latest_event_at || record.updated_at)}
        </div>
      </TableCell>
      <TableCell>
        <div className='font-mono text-sm'>{record.external_user_id}</div>
        <div className='text-muted-foreground text-xs'>
          {t('Current status')}: {translateMembershipStatus(record.status, t)}
        </div>
      </TableCell>
      <TableCell>
        <IdentityHints record={record} t={t} />
      </TableCell>
      <TableCell>
        <CandidateUsers record={record} t={t} />
      </TableCell>
      <TableCell>
        <OpsModeBanner
          tone={unresolvedActionTone(record.suggested_action)}
          title={translateUnresolvedAction(record.suggested_action, t)}
          description={record.reason || t('No operator note')}
        />
      </TableCell>
    </TableRow>
  )
}

function UnresolvedMembershipContent({
  t,
  snapshot,
}: Pick<MembershipRiskRuntimeViewProps, 't' | 'snapshot'>) {
  if (!snapshot.unresolved?.records?.length) {
    return (
      <div className='text-muted-foreground rounded-xl border border-dashed px-4 py-8 text-center text-sm'>
        {snapshot.loading
          ? t('Loading...')
          : t('No unresolved identity records at the moment')}
      </div>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Source / room')}</TableHead>
          <TableHead>{t('External ID')}</TableHead>
          <TableHead>{t('Identity hints')}</TableHead>
          <TableHead>{t('Candidate users')}</TableHead>
          <TableHead>{t('Operator action')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {snapshot.unresolved.records.map((record) => (
          <UnresolvedMembershipRow
            key={`${record.source}-${record.room_id}-${record.external_user_id}`}
            record={record}
            t={t}
          />
        ))}
      </TableBody>
    </Table>
  )
}

function UnresolvedMembershipCard({
  t,
  snapshot,
}: Pick<MembershipRiskRuntimeViewProps, 't' | 'snapshot'>) {
  return (
    <Card>
      <CardHeader>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <CardTitle>{t('Unresolved identity queue')}</CardTitle>
            <CardDescription>
              {t(
                'These membership records are real but still missing a reliable site user mapping. Use the hints below to decide whether to backfill, ask the user to bind, or collect more proof.'
              )}
            </CardDescription>
          </div>
          <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-sm'>
            <span>
              {t('Unresolved states')}:{' '}
              {snapshot.unresolved?.unresolved_state_count || 0}
            </span>
            <span>
              {t('Unresolved events')}:{' '}
              {snapshot.unresolved?.unresolved_event_count || 0}
            </span>
          </div>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <UnresolvedMembershipContent t={t} snapshot={snapshot} />
      </CardContent>
    </Card>
  )
}

export function MembershipRiskRuntimeView({
  t,
  snapshot,
  controls,
  actions,
}: MembershipRiskRuntimeViewProps) {
  return (
    <>
      <MembershipRuntimeSummary
        t={t}
        snapshot={snapshot}
        onRefresh={() => void actions.refresh(controls.statusFilter)}
      />
      <ManualActionCard t={t} controls={controls} />
      <MembershipStatesCard
        t={t}
        snapshot={snapshot}
        controls={controls}
        actions={actions}
      />
      <UnresolvedMembershipCard t={t} snapshot={snapshot} />
    </>
  )
}
