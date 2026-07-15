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
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { type OpsTranslate } from './ops-i18n'
import { boolValue, numberValue } from './ops-live-foundation'
import { quotaDisplay } from './ops-live-truth'
import { OpsPanel } from './ops-shared'

type DailyBudgetDraft = {
  dailyBudgetQuota: number
  growthBudgetQuota: number
  activityBudgetQuota: number
  gameBudgetQuota: number
  opsCompBudgetQuota: number
  communityBudgetQuota: number
  opsFundDailyTargetQuota: number
  dailyBudgetResetEnabled: boolean
  dailyFundResetEnabled: boolean
}

function dailyBudgetDraftFromRecord(
  settings: Record<string, unknown>
): DailyBudgetDraft {
  return {
    dailyBudgetQuota: numberValue(settings.daily_budget_quota),
    growthBudgetQuota: numberValue(settings.growth_budget_quota),
    activityBudgetQuota: numberValue(settings.activity_budget_quota),
    gameBudgetQuota: numberValue(settings.game_budget_quota),
    opsCompBudgetQuota: numberValue(settings.ops_comp_budget_quota),
    communityBudgetQuota: numberValue(settings.community_budget_quota),
    opsFundDailyTargetQuota: numberValue(settings.ops_fund_daily_target_quota),
    dailyBudgetResetEnabled: boolValue(settings.daily_budget_reset_enabled),
    dailyFundResetEnabled: boolValue(settings.daily_fund_reset_enabled),
  }
}

export function dailyBudgetDraftFingerprint(settings: Record<string, unknown>) {
  return JSON.stringify(dailyBudgetDraftFromRecord(settings))
}

type DailyBudgetNumberKey = keyof Pick<
  DailyBudgetDraft,
  | 'dailyBudgetQuota'
  | 'growthBudgetQuota'
  | 'activityBudgetQuota'
  | 'gameBudgetQuota'
  | 'opsCompBudgetQuota'
  | 'communityBudgetQuota'
  | 'opsFundDailyTargetQuota'
>

type DailyBudgetFlagKey = keyof Pick<
  DailyBudgetDraft,
  'dailyBudgetResetEnabled' | 'dailyFundResetEnabled'
>

const DAILY_BUDGET_FIELDS = [
  ['dailyBudgetQuota', 'Default daily budget'],
  ['growthBudgetQuota', 'Registration and invitation daily budget'],
  ['activityBudgetQuota', 'Check-in daily budget'],
  ['gameBudgetQuota', 'Game daily budget'],
  ['opsCompBudgetQuota', 'Manual compensation daily budget'],
  ['communityBudgetQuota', 'Community daily budget'],
] as const satisfies ReadonlyArray<readonly [DailyBudgetNumberKey, string]>

function useDailyBudgetSettings(
  settings: Record<string, unknown>,
  siteId: string,
  t: OpsTranslate
) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<DailyBudgetDraft>(() =>
    dailyBudgetDraftFromRecord(settings)
  )
  const [reason, setReason] = useState('')
  const [saving, setSaving] = useState(false)
  const updateNumber = (key: DailyBudgetNumberKey, value: string) => {
    const numeric = Number(value)
    setDraft((current) => ({
      ...current,
      [key]: Number.isFinite(numeric) ? Math.max(0, Math.trunc(numeric)) : 0,
    }))
  }
  const updateFlag = (key: DailyBudgetFlagKey, value: boolean) =>
    setDraft((current) => ({ ...current, [key]: value }))
  const save = async (applyToToday: boolean) => {
    if (!siteId) return
    if (applyToToday && !reason.trim()) {
      toast.error(t('Enter a reason before applying new limits to today.'))
      return
    }
    setSaving(true)
    try {
      const requestId = applyToToday
        ? typeof crypto !== 'undefined' && 'randomUUID' in crypto
          ? crypto.randomUUID()
          : `budget-settings-${Date.now()}`
        : ''
      await api.put(`/api/ops/fund/${encodeURIComponent(siteId)}/settings`, {
        daily_budget_quota: draft.dailyBudgetQuota,
        growth_budget_quota: draft.growthBudgetQuota,
        activity_budget_quota: draft.activityBudgetQuota,
        game_budget_quota: draft.gameBudgetQuota,
        ops_comp_budget_quota: draft.opsCompBudgetQuota,
        community_budget_quota: draft.communityBudgetQuota,
        ops_fund_daily_target_quota: draft.opsFundDailyTargetQuota,
        daily_budget_reset_enabled: draft.dailyBudgetResetEnabled,
        daily_fund_reset_enabled: draft.dailyFundResetEnabled,
        apply_to_today: applyToToday,
        request_id: requestId,
        reason: applyToToday ? reason.trim() : '',
      })
      await queryClient.invalidateQueries({
        queryKey: ['ops-registry-fund', siteId],
      })
      toast.success(
        applyToToday
          ? t('Daily budgets were saved and applied to today.')
          : t('Daily budgets were saved for the next reset.')
      )
      if (applyToToday) setReason('')
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to save daily budgets.')
      )
    } finally {
      setSaving(false)
    }
  }
  return { draft, reason, setReason, saving, updateNumber, updateFlag, save }
}

type DailyBudgetSettingsController = ReturnType<typeof useDailyBudgetSettings>

function DailyBudgetQuotaFields({
  controller,
  t,
}: {
  controller: DailyBudgetSettingsController
  t: OpsTranslate
}) {
  return (
    <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
      {DAILY_BUDGET_FIELDS.map(([key, label]) => (
        <label key={key} className='space-y-2'>
          <span className='text-sm font-medium'>{t(label)}</span>
          <Input
            type='number'
            min={0}
            step={1}
            value={controller.draft[key]}
            onChange={(event) =>
              controller.updateNumber(key, event.target.value)
            }
          />
          <span className='text-muted-foreground block text-xs'>
            {quotaDisplay(controller.draft[key])}
          </span>
        </label>
      ))}
    </div>
  )
}

function DailyBudgetResetControls({
  controller,
  effectiveFundTarget,
  t,
}: {
  controller: DailyBudgetSettingsController
  effectiveFundTarget: unknown
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 mt-4 grid gap-4 border-t pt-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,.75fr)]'>
      <div className='space-y-3'>
        <label className='border-border flex min-h-12 items-start gap-3 rounded-md border px-3 py-2.5 text-sm'>
          <input
            type='checkbox'
            className='mt-1'
            checked={controller.draft.dailyBudgetResetEnabled}
            onChange={(event) =>
              controller.updateFlag(
                'dailyBudgetResetEnabled',
                event.target.checked
              )
            }
          />
          <span>
            <span className='block font-medium'>
              {t('Recreate daily budget pools automatically')}
            </span>
            <span className='text-muted-foreground mt-1 block'>
              {t(
                'A new dated pool is created once after midnight; previous days are never rewritten.'
              )}
            </span>
          </span>
        </label>
        <label className='border-border flex min-h-12 items-start gap-3 rounded-md border px-3 py-2.5 text-sm'>
          <input
            type='checkbox'
            className='mt-1'
            checked={controller.draft.dailyFundResetEnabled}
            onChange={(event) =>
              controller.updateFlag(
                'dailyFundResetEnabled',
                event.target.checked
              )
            }
          />
          <span>
            <span className='block font-medium'>
              {t('Replenish the operating fund once per day')}
            </span>
            <span className='text-muted-foreground mt-1 block'>
              {t(
                'The fund is raised to the daily target once, with an auditable ledger entry; it is not refilled after every reward.'
              )}
            </span>
          </span>
        </label>
      </div>
      <label className='space-y-2'>
        <span className='text-sm font-medium'>
          {t('Operating fund daily target')}
        </span>
        <Input
          type='number'
          min={0}
          step={1}
          value={controller.draft.opsFundDailyTargetQuota}
          onChange={(event) =>
            controller.updateNumber(
              'opsFundDailyTargetQuota',
              event.target.value
            )
          }
        />
        <span className='text-muted-foreground block text-xs leading-5'>
          {controller.draft.opsFundDailyTargetQuota > 0
            ? quotaDisplay(controller.draft.opsFundDailyTargetQuota)
            : `${t('Automatic')}: ${quotaDisplay(effectiveFundTarget)}`}
          {' · '}
          {t('Use 0 to match the sum of all concrete daily budgets.')}
        </span>
      </label>
    </div>
  )
}

function DailyBudgetSaveActions({
  controller,
  t,
}: {
  controller: DailyBudgetSettingsController
  t: OpsTranslate
}) {
  const canApplyToday =
    controller.reason.trim() &&
    (controller.draft.dailyBudgetResetEnabled ||
      controller.draft.dailyFundResetEnabled)
  return (
    <div className='border-border/70 mt-4 space-y-3 border-t pt-4'>
      <label htmlFor='ops-daily-budget-reason' className='block space-y-2'>
        <span className='text-sm font-medium'>
          {t('Reason for applying limits to today')}
        </span>
        <Textarea
          id='ops-daily-budget-reason'
          value={controller.reason}
          onChange={(event) => controller.setReason(event.target.value)}
          placeholder={t(
            'Required only when applying the new limits to today.'
          )}
          rows={2}
        />
      </label>
      <div className='flex flex-wrap justify-end gap-2'>
        <Button
          type='button'
          variant='outline'
          disabled={controller.saving}
          onClick={() => void controller.save(false)}
        >
          {controller.saving ? t('Saving...') : t('Save for the next reset')}
        </Button>
        <Button
          type='button'
          disabled={controller.saving || !canApplyToday}
          onClick={() => void controller.save(true)}
        >
          {controller.saving ? t('Saving...') : t('Save and apply to today')}
        </Button>
      </div>
    </div>
  )
}

export function DailyBudgetSettingsPanel({
  settings,
  siteId,
  t,
}: {
  settings: Record<string, unknown>
  siteId: string
  t: OpsTranslate
}) {
  const controller = useDailyBudgetSettings(settings, siteId, t)
  return (
    <OpsPanel
      title={t('Daily budget settings')}
      description={t(
        'Set the amount recreated for each reward use every day. The accounting day changes at midnight in Asia/Shanghai.'
      )}
    >
      <DailyBudgetQuotaFields controller={controller} t={t} />
      <DailyBudgetResetControls
        controller={controller}
        effectiveFundTarget={settings.effective_fund_target_quota}
        t={t}
      />
      <DailyBudgetSaveActions controller={controller} t={t} />
    </OpsPanel>
  )
}
