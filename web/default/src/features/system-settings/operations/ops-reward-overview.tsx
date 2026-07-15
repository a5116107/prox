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
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  DailyBudgetSettingsPanel,
  dailyBudgetDraftFingerprint,
} from './ops-daily-budget'
import { type OpsTranslate } from './ops-i18n'
import {
  displayText,
  formatTime,
  normalizeText,
  numberValue,
  recordValue,
} from './ops-live-foundation'
import {
  claimStatusLabel,
  claimStatusTone,
  degradationLabel,
  degradationTone,
  inviteEventLabel,
  quotaDisplay,
  sourceTypeLabel,
} from './ops-live-truth'
import {
  OpsDataTable,
  OpsInsightCard,
  OpsPanel,
  OpsStatusBadge,
  OpsSurfaceGrid,
} from './ops-shared'
import {
  type OpsInviteJourneyOverview,
  type OpsRewardFundOverview,
} from './use-ops-registry'

function useRewardCapacityRestore(siteId: string, t: OpsTranslate) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const [reason, setReason] = useState('')
  const [selectedPools, setSelectedPools] = useState<string[]>(['activity'])
  const poolOptions = [
    ['activity', t('Check-in and redemption rewards')],
    ['growth', t('Registration and invitation rewards')],
    ['game', t('Game rewards')],
    ['community', t('Community rewards')],
    ['ops_comp', t('Manual compensation')],
  ] as const

  const togglePool = (poolType: string) => {
    setSelectedPools((current) =>
      current.includes(poolType)
        ? current.filter((item) => item !== poolType)
        : [...current, poolType]
    )
  }
  const restore = async () => {
    if (!siteId || selectedPools.length === 0 || !reason.trim()) {
      toast.error(t('Choose at least one use and enter a reason.'))
      return
    }
    setRestoring(true)
    try {
      await api.post(
        `/api/ops/fund/${encodeURIComponent(siteId)}/restore-capacity`,
        {
          pool_types: selectedPools,
          request_id: crypto.randomUUID(),
          reason: reason.trim(),
        }
      )
      await queryClient.invalidateQueries({
        queryKey: ['ops-registry-fund', siteId],
      })
      toast.success(
        t('Available rewards and the matching fund balance were restored.')
      )
      setReason('')
      setOpen(false)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to restore available rewards.')
      )
    } finally {
      setRestoring(false)
    }
  }
  return {
    open,
    toggleOpen: () => setOpen((value) => !value),
    restoring,
    reason,
    setReason,
    selectedPools,
    poolOptions,
    togglePool,
    restore,
  }
}

function RewardFundSummaryCards({
  rewardFund,
  inviteJourney,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  t: OpsTranslate
}) {
  const fund = recordValue(rewardFund?.fund)
  const degradation = recordValue(rewardFund?.degradation)
  const commissionAudit = recordValue(rewardFund?.commission_audit)
  const funnel = recordValue(inviteJourney?.funnel)
  const budgetPools = rewardFund?.budget_pools_today ?? []
  const claimStatuses = inviteJourney?.claim_statuses ?? []
  const remainingBudget = budgetPools.reduce(
    (sum, row) => sum + numberValue(row.remaining_quota),
    0
  )
  const totalBudget = budgetPools.reduce(
    (sum, row) => sum + numberValue(row.total_quota),
    0
  )
  const blockedClaims =
    numberValue(funnel.blocked_claims) ||
    claimStatuses
      .filter((row) => claimStatusTone(row.status) !== 'success')
      .reduce((sum, row) => sum + numberValue(row.count), 0)
  return (
    <OpsSurfaceGrid className='xl:grid-cols-4'>
      <OpsInsightCard
        title={t('Operating fund balance')}
        value={
          rewardFund ? quotaDisplay(fund.balance_quota, fund.balance_usd) : '—'
        }
        description={
          normalizeText(fund.exists) === 'false'
            ? t(
                'No fund account exists yet. Fund-backed rewards should stay closed.'
              )
            : t('Real balance from the operations fund account.')
        }
        badge={
          <OpsStatusBadge tone={degradationTone(degradation.state)}>
            {degradationLabel(degradation.state, t)}
          </OpsStatusBadge>
        }
      />
      <OpsInsightCard
        title={t('Today reward budget')}
        value={
          rewardFund
            ? numberValue(rewardFund.effective_available_quota).toLocaleString()
            : '—'
        }
        description={`${t('Remaining today')}: ${remainingBudget.toLocaleString()} / ${totalBudget.toLocaleString()} · ${t(
          'Effective available amount is the lower of today’s remaining budgets and the operating fund balance.'
        )}`}
      />
      <OpsInsightCard
        title={t('Invite rewards paid / blocked')}
        value={
          inviteJourney
            ? `${numberValue(funnel.paid_claims).toLocaleString()} / ${blockedClaims.toLocaleString()}`
            : '—'
        }
        description={
          inviteJourney
            ? `${t('Paid quota')}: ${quotaDisplay(
                funnel.paid_quota,
                funnel.paid_quota_usd
              )}`
            : t('Invite journey endpoint has not returned data yet.')
        }
      />
      <OpsInsightCard
        title={t('Commission into fund')}
        value={
          rewardFund
            ? quotaDisplay(
                commissionAudit.fund_net_quota ||
                  commissionAudit.commission_quota,
                commissionAudit.fund_net_usd
              )
            : '—'
        }
        description={t(
          'Game commission evidence is reconciled with fund ledger rows; it is not a manual top-up.'
        )}
      />
    </OpsSurfaceGrid>
  )
}

function RewardCapacityRestorePanel({
  siteId,
  t,
}: {
  siteId: string
  t: OpsTranslate
}) {
  const restore = useRewardCapacityRestore(siteId, t)
  return (
    <OpsPanel
      title={t('Restore available rewards')}
      description={t(
        'Use this when check-in, redemption, invitation, or game rewards stop because today’s available amount or fund balance is too low. Existing payouts and reserved rewards are kept.'
      )}
    >
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='text-muted-foreground text-sm'>
          {t(
            'The change is recorded. Repeating the same request will not add the amount twice.'
          )}
        </div>
        <Button type='button' onClick={restore.toggleOpen}>
          {restore.open ? t('Cancel') : t('Restore amount')}
        </Button>
      </div>
      {restore.open ? (
        <div className='border-border/70 mt-4 space-y-4 border-t pt-4'>
          <div>
            <div className='mb-2 text-sm font-medium'>
              {t('Select affected uses')}
            </div>
            <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-5'>
              {restore.poolOptions.map(([poolType, label]) => (
                <label
                  key={poolType}
                  className='border-border flex min-h-11 cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm'
                >
                  <input
                    type='checkbox'
                    checked={restore.selectedPools.includes(poolType)}
                    onChange={() => restore.togglePool(poolType)}
                  />
                  <span>{label}</span>
                </label>
              ))}
            </div>
          </div>
          <div>
            <label
              htmlFor='ops-budget-restore-reason'
              className='mb-2 block text-sm font-medium'
            >
              {t('Reason for this change')}
            </label>
            <Textarea
              id='ops-budget-restore-reason'
              value={restore.reason}
              onChange={(event) => restore.setReason(event.target.value)}
              placeholder={t(
                'Example: rewards were paused after the daily amount was used up; reviewed by the site administrator.'
              )}
              rows={3}
            />
          </div>
          <div className='flex justify-end'>
            <Button
              type='button'
              disabled={
                restore.restoring ||
                restore.selectedPools.length === 0 ||
                !restore.reason.trim()
              }
              onClick={() => void restore.restore()}
            >
              {restore.restoring ? t('Restoring...') : t('Confirm restore')}
            </Button>
          </div>
        </div>
      ) : null}
    </OpsPanel>
  )
}

function RewardEvidenceToggle({
  rewardFund,
  open,
  onToggle,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  open: boolean
  onToggle: () => void
  t: OpsTranslate
}) {
  const degradation = recordValue(rewardFund?.degradation)
  return (
    <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='space-y-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <OpsStatusBadge tone={degradationTone(degradation.state)}>
              {degradationLabel(degradation.state, t)}
            </OpsStatusBadge>
            <span className='text-muted-foreground text-xs'>
              {t('Generated at')}: {formatTime(rewardFund?.generated_at)}
            </span>
          </div>
          <div className='text-muted-foreground text-sm leading-6'>
            {t(
              'The cards above already show the live fund balance, today budget, claim volume, and commission result. Expand the detailed evidence only when you need ledger rows or payout drill-down.'
            )}
          </div>
        </div>
        <Button
          type='button'
          variant={open ? 'default' : 'outline'}
          onClick={onToggle}
        >
          {open ? t('Hide detailed evidence') : t('Open detailed evidence')}
        </Button>
      </div>
    </div>
  )
}

function RewardSafetyDecision({
  rewardFund,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  t: OpsTranslate
}) {
  const degradation = recordValue(rewardFund?.degradation)
  const rewardPolicy = recordValue(rewardFund?.reward_policy)
  const inviteRewardAudit = recordValue(rewardFund?.invite_reward_audit)
  return (
    <OpsPanel
      title={t('Reward safety decision')}
      description={t(
        'This is the operator-facing answer: whether rewards can continue, why the system thinks so, and what should be done before more invite benefits are sent.'
      )}
    >
      <div className='grid gap-3 xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,.9fr)]'>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='flex flex-wrap items-center gap-2'>
            <OpsStatusBadge tone={degradationTone(degradation.state)}>
              {degradationLabel(degradation.state, t)}
            </OpsStatusBadge>
            <span className='text-muted-foreground text-xs'>
              {t('Generated at')}: {formatTime(rewardFund?.generated_at)}
            </span>
          </div>
          <div className='text-muted-foreground mt-3 text-sm leading-6'>
            {displayText(degradation.reason)}
          </div>
          <div className='border-border/60 bg-muted/20 mt-3 rounded-2xl border p-3 text-sm leading-6'>
            <span className='font-semibold'>{t('Operator action')}:</span>{' '}
            <span className='text-muted-foreground'>
              {displayText(degradation.operator_action)}
            </span>
          </div>
        </div>
        <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
          <div className='text-sm font-semibold'>
            {t('Configured reward exposure')}
          </div>
          <div className='text-muted-foreground mt-3 space-y-2 text-sm leading-6'>
            <div>
              {t('Configured reward groups')}:{' '}
              {displayText(rewardPolicy.configured_groups)}
            </div>
            <div>
              {t('Invite-enabled groups')}:{' '}
              {displayText(rewardPolicy.invite_enabled_groups)}
            </div>
            <div>
              {t('One invite pair needs')}:{' '}
              {quotaDisplay(rewardPolicy.invite_pair_reward_quota)}
            </div>
            <div>
              {t('Planned daily reward cap')}:{' '}
              {quotaDisplay(rewardPolicy.planned_daily_reward_quota)}
            </div>
            <div>
              {t('Paid quota')}:{' '}
              {quotaDisplay(
                inviteRewardAudit.paid_claim_quota,
                inviteRewardAudit.paid_claim_usd
              )}
            </div>
            <div>
              {t('Claims without ledger')}:{' '}
              {displayText(inviteRewardAudit.paid_without_ledger_count)}
            </div>
          </div>
        </div>
      </div>
    </OpsPanel>
  )
}

function InviteAndBudgetTables({
  rewardFund,
  inviteJourney,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  t: OpsTranslate
}) {
  const stateMachine = inviteJourney?.state_machine ?? []
  const budgetPools = rewardFund?.budget_pools_today ?? []
  return (
    <OpsSurfaceGrid className='xl:grid-cols-2'>
      <OpsDataTable
        title={t('Invite funnel state machine')}
        description={t(
          'The invite path should read left to right: link, join, membership verified, reward claim, then paid or blocked with a visible reason.'
        )}
        columns={[
          { key: 'step', label: t('Step') },
          { key: 'count', label: t('Count') },
          { key: 'next', label: t('Next state') },
        ]}
        rows={stateMachine.map((row, index) => ({
          id: `${row.step || index}`,
          cells: [
            <div className='space-y-1'>
              <div className='font-medium'>
                {inviteEventLabel(row.step || row.label, t)}
              </div>
              <div className='text-muted-foreground text-xs'>
                {displayText(row.step)}
              </div>
            </div>,
            numberValue(row.count).toLocaleString(),
            <span className='text-muted-foreground text-sm'>
              {displayText(row.next)}
            </span>,
          ],
        }))}
        emptyMessage={t('No invite journey evidence yet.')}
      />
      <OpsDataTable
        title={t('Today budget pools')}
        description={t(
          'Each row is a real budget pool for today. Low or exhausted pools explain why rewards are downgraded or queued.'
        )}
        columns={[
          { key: 'pool', label: t('Budget pool') },
          { key: 'effective', label: t('Effective available') },
          { key: 'remaining', label: t('Remaining') },
          { key: 'used', label: t('Used / total') },
          { key: 'state', label: t('State') },
        ]}
        rows={budgetPools.map((row, index) => ({
          id: `${row.id || row.pool_type || index}`,
          cells: [
            <div className='space-y-1'>
              <div className='font-medium'>{displayText(row.pool_type)}</div>
              <div className='text-muted-foreground text-xs'>
                {displayText(row.budget_date)}
              </div>
            </div>,
            quotaDisplay(
              row.effective_available_quota,
              row.effective_available_usd
            ),
            quotaDisplay(row.remaining_quota, row.remaining_usd),
            `${numberValue(row.used_quota).toLocaleString()} / ${numberValue(
              row.total_quota
            ).toLocaleString()}`,
            <OpsStatusBadge tone={degradationTone(row.degrade_state)}>
              {degradationLabel(row.degrade_state, t)}
            </OpsStatusBadge>,
          ],
        }))}
        emptyMessage={t('No budget pool was created for today.')}
      />
    </OpsSurfaceGrid>
  )
}

function ClaimAndFundTables({
  rewardFund,
  inviteJourney,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  t: OpsTranslate
}) {
  const claimStatuses = inviteJourney?.claim_statuses ?? []
  const recentLedgers = rewardFund?.recent_ledgers ?? []
  return (
    <OpsSurfaceGrid className='xl:grid-cols-2'>
      <OpsDataTable
        title={t('Claim payout reasons')}
        description={t(
          'This table explains whether invite rewards were paid, blocked by missing membership, delayed by low fund, or delayed by today’s budget.'
        )}
        columns={[
          { key: 'status', label: t('Status') },
          { key: 'stage', label: t('Reward side') },
          { key: 'count', label: t('Count') },
          { key: 'quota', label: t('Quota') },
        ]}
        rows={claimStatuses.map((row, index) => ({
          id: `${row.status || index}-${row.reward_stage || ''}`,
          cells: [
            <OpsStatusBadge tone={claimStatusTone(row.status)}>
              {claimStatusLabel(row.status, t)}
            </OpsStatusBadge>,
            displayText(row.reward_stage),
            numberValue(row.count).toLocaleString(),
            quotaDisplay(row.quota, row.quota_usd),
          ],
        }))}
        emptyMessage={t('No invite reward claim status rows yet.')}
      />
      <OpsDataTable
        title={t('Recent fund ledger')}
        description={t(
          'Newest real fund ledger rows. This shows whether game commission, top-up, and invite reward deductions are really landing.'
        )}
        columns={[
          { key: 'source', label: t('Source') },
          { key: 'delta', label: t('Delta') },
          { key: 'balance', label: t('Balance after') },
          { key: 'time', label: t('Time') },
        ]}
        rows={recentLedgers.slice(0, 8).map((row, index) => ({
          id: `${row.id || index}`,
          cells: [
            <div className='space-y-1'>
              <div className='font-medium'>
                {sourceTypeLabel(row.source_type, t)}
              </div>
              <div className='text-muted-foreground text-xs'>
                {displayText(row.source_pool_type || row.idempotency_key)}
              </div>
            </div>,
            quotaDisplay(row.delta_quota, row.delta_usd),
            numberValue(row.balance_after).toLocaleString(),
            formatTime(row.created_at),
          ],
        }))}
        emptyMessage={t('No fund ledger rows yet.')}
      />
    </OpsSurfaceGrid>
  )
}

function ClaimsAndProblemsTables({
  inviteJourney,
  t,
}: {
  inviteJourney: OpsInviteJourneyOverview | null
  t: OpsTranslate
}) {
  const recentClaims = inviteJourney?.recent_claims ?? []
  const problems = inviteJourney?.problems ?? []
  return (
    <OpsSurfaceGrid className='xl:grid-cols-2'>
      <OpsDataTable
        title={t('Recent invite claims')}
        description={t(
          'Newest invite claim rows, including the fund ledger id and the human-readable error when payout was blocked.'
        )}
        columns={[
          { key: 'user', label: t('Reward user') },
          { key: 'status', label: t('Status') },
          { key: 'quota', label: t('Quota') },
          { key: 'reason', label: t('Reason') },
          { key: 'time', label: t('Time') },
        ]}
        rows={recentClaims.slice(0, 8).map((row, index) => ({
          id: `${row.id || index}`,
          cells: [
            <div className='space-y-1'>
              <div className='font-medium'>
                UID {displayText(row.reward_user_id)}
              </div>
              <div className='text-muted-foreground text-xs'>
                {displayText(row.reward_stage)}
              </div>
            </div>,
            <OpsStatusBadge tone={claimStatusTone(row.status)}>
              {claimStatusLabel(row.status, t)}
            </OpsStatusBadge>,
            quotaDisplay(row.quota, row.quota_usd),
            <div className='space-y-1'>
              <div>{displayText(row.error)}</div>
              <div className='text-muted-foreground text-xs'>
                {t('Fund ledger')}: {displayText(row.ops_fund_ledger_id)}
              </div>
            </div>,
            formatTime(row.updated_at || row.created_at),
          ],
        }))}
        emptyMessage={t('No recent invite claims yet.')}
      />
      <OpsDataTable
        title={t('Invite problems needing operator review')}
        description={t(
          'Only real detected problems are listed here. An empty table means the backend did not detect a review item in current data.'
        )}
        columns={[
          { key: 'code', label: t('Problem') },
          { key: 'severity', label: t('Severity') },
          { key: 'count', label: t('Count') },
          { key: 'message', label: t('Message') },
        ]}
        rows={problems.map((row, index) => ({
          id: `${row.code || index}`,
          cells: [
            displayText(row.code),
            <OpsStatusBadge tone={degradationTone(row.severity)}>
              {displayText(row.severity)}
            </OpsStatusBadge>,
            numberValue(row.count).toLocaleString(),
            displayText(row.message),
          ],
        }))}
        emptyMessage={t('No invite problems detected.')}
      />
    </OpsSurfaceGrid>
  )
}

function RewardEvidencePanels({
  rewardFund,
  inviteJourney,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  t: OpsTranslate
}) {
  return (
    <>
      <RewardSafetyDecision rewardFund={rewardFund} t={t} />
      <InviteAndBudgetTables
        rewardFund={rewardFund}
        inviteJourney={inviteJourney}
        t={t}
      />
      <ClaimAndFundTables
        rewardFund={rewardFund}
        inviteJourney={inviteJourney}
        t={t}
      />
      <ClaimsAndProblemsTables inviteJourney={inviteJourney} t={t} />
    </>
  )
}

export function RewardFundInviteOverview({
  rewardFund,
  inviteJourney,
  siteId,
  t,
}: {
  rewardFund: OpsRewardFundOverview | null
  inviteJourney: OpsInviteJourneyOverview | null
  siteId: string
  t: OpsTranslate
}) {
  const [showEvidence, setShowEvidence] = useState(false)
  const budgetSettings = recordValue(rewardFund?.budget_settings)
  return (
    <div className='space-y-6'>
      <RewardFundSummaryCards
        rewardFund={rewardFund}
        inviteJourney={inviteJourney}
        t={t}
      />
      <DailyBudgetSettingsPanel
        key={`${siteId}:${dailyBudgetDraftFingerprint(budgetSettings)}`}
        settings={budgetSettings}
        siteId={siteId}
        t={t}
      />
      <RewardCapacityRestorePanel siteId={siteId} t={t} />
      <RewardEvidenceToggle
        rewardFund={rewardFund}
        open={showEvidence}
        onToggle={() => setShowEvidence((value) => !value)}
        t={t}
      />
      {showEvidence ? (
        <RewardEvidencePanels
          rewardFund={rewardFund}
          inviteJourney={inviteJourney}
          t={t}
        />
      ) : null}
    </div>
  )
}
