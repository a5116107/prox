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
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { type OpsTranslate } from './ops-i18n'
import {
  displayText,
  formatTime,
  normalizeText,
  releaseActionLabel,
  statusBadge,
} from './ops-live-foundation'
import { OpsPanel, OpsStatusBadge } from './ops-shared'
import {
  type OpsReleaseActions,
  type OpsReleaseImpactPreview,
  type OpsReleaseRecord,
  type OpsReleaseOverview,
} from './use-ops-registry'

function buildRollbackCandidates(
  releases: OpsReleaseOverview | null,
  currentRelease: OpsReleaseRecord | null
) {
  const recentReleases = releases?.recent_releases ?? []
  if (recentReleases.length > 0) return recentReleases
  return currentRelease ? [currentRelease] : []
}

function fallbackRollbackId(
  currentRelease: OpsReleaseRecord | null,
  rollbackCandidates: OpsReleaseRecord[]
) {
  if (currentRelease?.id) return currentRelease.id
  return rollbackCandidates[0]?.id || 0
}

function selectedRollbackRelease(
  selectedId: string,
  currentRelease: OpsReleaseRecord | null,
  rollbackCandidates: OpsReleaseRecord[]
) {
  const exactMatch = rollbackCandidates.find(
    (release) => String(release.id) === selectedId
  )
  if (exactMatch) return exactMatch
  if (currentRelease) return currentRelease
  return rollbackCandidates[0] ?? null
}

function changedOptionKeys(releaseImpact: OpsReleaseImpactPreview | null) {
  const options = releaseImpact?.changes.options
  if (!options) return []
  const changedKeys = [
    ...(options.added_keys ?? []),
    ...(options.updated_keys ?? []),
    ...(options.removed_keys ?? []),
  ]
  return Array.from(
    new Set(changedKeys.map((item) => normalizeText(item)).filter(Boolean))
  ).slice(0, 8)
}

function useReleaseSelection(
  releases: OpsReleaseOverview | null,
  releaseImpact: OpsReleaseImpactPreview | null,
  defaultOpen: boolean
) {
  const currentRelease = releases?.current_release ?? null
  const rollbackCandidates = useMemo(
    () => buildRollbackCandidates(releases, currentRelease),
    [currentRelease, releases]
  )
  const [selectedRollbackId, setSelectedRollbackId] = useState('')
  const [showTools, setShowTools] = useState(defaultOpen)
  const fallbackId = fallbackRollbackId(currentRelease, rollbackCandidates)
  const effectiveSelectedRollbackId =
    selectedRollbackId || (fallbackId > 0 ? String(fallbackId) : '')
  const selectedRelease = selectedRollbackRelease(
    effectiveSelectedRollbackId,
    currentRelease,
    rollbackCandidates
  )
  const optionChangedKeys = useMemo(
    () => changedOptionKeys(releaseImpact),
    [releaseImpact]
  )
  return {
    currentRelease,
    rollbackCandidates,
    effectiveSelectedRollbackId,
    selectedRollbackRelease: selectedRelease,
    selectRollback: (releaseId: number) =>
      setSelectedRollbackId(String(releaseId)),
    optionChangedKeys,
    showTools,
    toggleTools: () => setShowTools((value) => !value),
  }
}

function usePublishReleaseEditor(
  siteId: string,
  releases: OpsReleaseOverview | null,
  releaseActions: OpsReleaseActions,
  t: OpsTranslate
) {
  const [label, setLabel] = useState('')
  const [note, setNote] = useState('')
  const submit = async () => {
    try {
      const release = await releaseActions.publish({
        release_label: normalizeText(label) || undefined,
        note: normalizeText(note) || undefined,
      })
      toast.success(
        release?.release_label
          ? t('Published snapshot {{label}}.', {
              label: release.release_label,
            })
          : t('Published the current live snapshot.')
      )
      setLabel('')
      setNote('')
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to publish the current snapshot.')
      )
    }
  }
  return {
    label,
    setLabel,
    note,
    setNote,
    submit,
    disabled: !siteId || !releases?.publish_supported || releaseActions.busy,
    publishing: releaseActions.publishing,
  }
}

function useRollbackReleaseEditor(
  siteId: string,
  releases: OpsReleaseOverview | null,
  releaseActions: OpsReleaseActions,
  selectedRelease: OpsReleaseRecord | null,
  t: OpsTranslate
) {
  const [note, setNote] = useState('')
  const submit = async () => {
    const releaseId = Number(selectedRelease?.id || 0)
    if (!Number.isFinite(releaseId) || releaseId <= 0) {
      toast.error(t('Choose a release snapshot first.'))
      return
    }
    try {
      await releaseActions.rollback({
        release_id: releaseId,
        note: normalizeText(note) || undefined,
      })
      toast.success(
        t('Rollback finished and the live policy has been refreshed.')
      )
      setNote('')
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to rollback this release snapshot.')
      )
    }
  }
  return {
    note,
    setNote,
    submit,
    disabled:
      !siteId ||
      !releases?.rollback_supported ||
      !selectedRelease ||
      releaseActions.busy,
    rollingBack: releaseActions.rollingBack,
  }
}

type ReleaseSelection = ReturnType<typeof useReleaseSelection>
type PublishReleaseEditor = ReturnType<typeof usePublishReleaseEditor>
type RollbackReleaseEditor = ReturnType<typeof useRollbackReleaseEditor>

function ReleaseSummaryCard({
  release,
  t,
}: {
  release: OpsReleaseRecord | null
  t: OpsTranslate
}) {
  if (!release) {
    return (
      <div className='border-border/70 bg-background/70 text-muted-foreground rounded-[18px] border border-dashed p-4 text-sm leading-6'>
        {t(
          'No published snapshot exists yet. Publish once to freeze the current groups, access policy, and reward settings before wider rollout.'
        )}
      </div>
    )
  }
  return (
    <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='space-y-1'>
          <div className='text-sm font-semibold'>
            {release.release_label || t('Unnamed snapshot')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {releaseActionLabel(release.action, t)} ·{' '}
            {formatTime(release.applied_at || release.created_at)}
          </div>
        </div>
        {statusBadge(
          release.action === 'rollback',
          t('Rollback'),
          t('Publish'),
          t('Unknown')
        )}
      </div>
      <div className='text-muted-foreground mt-3 grid gap-2 text-xs md:grid-cols-2'>
        <div>
          {t('Operator')}:{' '}
          {displayText(release.actor_username || release.actor_user_id)}
        </div>
        <div>
          {t('Snapshot hash')}:{' '}
          {displayText(String(release.snapshot_hash || '').slice(0, 12))}
        </div>
        <div>
          {t('Options / groups')}: {release.option_count} /{' '}
          {release.group_count}
        </div>
        <div>
          {t('ChatOps / games')}: {release.group_chatops_count} /{' '}
          {release.group_game_count}
        </div>
      </div>
      {normalizeText(release.note) ? (
        <div className='border-border/60 bg-muted/20 text-muted-foreground mt-3 rounded-2xl border p-3 text-sm leading-6'>
          {release.note}
        </div>
      ) : null}
    </div>
  )
}

function ReleaseControlSummary({
  selection,
  releaseImpact,
  t,
}: {
  selection: ReleaseSelection
  releaseImpact: OpsReleaseImpactPreview | null
  t: OpsTranslate
}) {
  const current = selection.currentRelease
  return (
    <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='space-y-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <OpsStatusBadge tone={current ? 'success' : 'warning'}>
              {current?.release_label ||
                (current
                  ? t('Unnamed snapshot')
                  : t('No published snapshot yet'))}
            </OpsStatusBadge>
            {releaseImpact
              ? statusBadge(
                  releaseImpact.has_changes,
                  t('Changes detected'),
                  t('No changes detected'),
                  t('Unknown')
                )
              : null}
          </div>
          <div className='text-muted-foreground text-sm leading-6'>
            {t(
              'Publish and rollback stay folded by default so admins can read the live truth first. Expand only when you are ready to freeze or restore a verified snapshot.'
            )}
          </div>
          <ReleaseSummaryMetrics
            currentRelease={current}
            releaseImpact={releaseImpact}
            t={t}
          />
        </div>
        <Button
          type='button'
          variant={selection.showTools ? 'default' : 'outline'}
          onClick={selection.toggleTools}
        >
          {selection.showTools
            ? t('Hide publish tools')
            : t('Open publish tools')}
        </Button>
      </div>
    </div>
  )
}

function ReleaseSummaryMetrics({
  currentRelease,
  releaseImpact,
  t,
}: {
  currentRelease: OpsReleaseRecord | null
  releaseImpact: OpsReleaseImpactPreview | null
  t: OpsTranslate
}) {
  const currentState = releaseImpact?.current_state
  return (
    <div className='text-muted-foreground flex flex-wrap gap-2 text-xs'>
      <span>
        {t('Options / groups')}:{' '}
        {currentState
          ? `${currentState.option_count ?? 0} / ${currentState.group_count ?? 0}`
          : currentRelease
            ? `${currentRelease.option_count} / ${currentRelease.group_count}`
            : '—'}
      </span>
      <span>
        {t('ChatOps / games')}:{' '}
        {currentState
          ? `${currentState.group_chatops_count ?? 0} / ${currentState.group_game_count ?? 0}`
          : currentRelease
            ? `${currentRelease.group_chatops_count} / ${currentRelease.group_game_count}`
            : '—'}
      </span>
      {releaseImpact ? (
        <span>
          {t('Detected changes')}: {releaseImpact.diff_summary.option_changes}/
          {releaseImpact.diff_summary.group_changes}/
          {releaseImpact.diff_summary.group_chatops_changes}/
          {releaseImpact.diff_summary.group_game_changes}
        </span>
      ) : null}
    </div>
  )
}

function ReleaseImpactMetrics({
  releaseImpact,
  t,
}: {
  releaseImpact: OpsReleaseImpactPreview
  t: OpsTranslate
}) {
  const metrics = [
    [t('Option changes'), releaseImpact.diff_summary.option_changes],
    [t('Group changes'), releaseImpact.diff_summary.group_changes],
    [t('ChatOps changes'), releaseImpact.diff_summary.group_chatops_changes],
    [t('Game config changes'), releaseImpact.diff_summary.group_game_changes],
  ] as const
  return (
    <div className='mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
      {metrics.map(([label, value]) => (
        <div
          key={label}
          className='border-border/60 bg-muted/20 rounded-2xl border p-3'
        >
          <div className='text-muted-foreground text-xs tracking-wide uppercase'>
            {label}
          </div>
          <div className='mt-2 text-lg font-semibold'>{value}</div>
        </div>
      ))}
    </div>
  )
}

function ReleaseImpactCard({
  releaseImpact,
  changedKeys,
  t,
}: {
  releaseImpact: OpsReleaseImpactPreview | null
  changedKeys: string[]
  t: OpsTranslate
}) {
  if (!releaseImpact) return null
  return (
    <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='space-y-1'>
          <div className='text-sm font-semibold'>
            {t('Impact preview against latest snapshot')}
          </div>
          <div className='text-muted-foreground text-xs leading-5'>
            {releaseImpact.previous_release
              ? t(
                  'This compares the current live truth with the latest saved snapshot before you publish again.'
                )
              : t(
                  'No previous snapshot exists yet. The first publish will create the baseline for future diffs.'
                )}
          </div>
        </div>
        {statusBadge(
          releaseImpact.has_changes,
          t('Changes detected'),
          t('No changes detected'),
          t('Unknown')
        )}
      </div>
      <ReleaseImpactMetrics releaseImpact={releaseImpact} t={t} />
      <div className='text-muted-foreground mt-3 grid gap-2 text-xs md:grid-cols-2'>
        <div>
          {t('Current hash')}:{' '}
          {displayText(releaseImpact.current_hash.slice(0, 12))}
        </div>
        <div>
          {t('Previous hash')}:{' '}
          {displayText(releaseImpact.previous_hash.slice(0, 12))}
        </div>
        <div>
          {t('Current options / groups')}:{' '}
          {releaseImpact.current_state?.option_count ?? 0} /{' '}
          {releaseImpact.current_state?.group_count ?? 0}
        </div>
        <div>
          {t('Current ChatOps / games')}:{' '}
          {releaseImpact.current_state?.group_chatops_count ?? 0} /{' '}
          {releaseImpact.current_state?.group_game_count ?? 0}
        </div>
      </div>
      {changedKeys.length > 0 ? (
        <div className='border-border/60 bg-muted/20 mt-3 rounded-2xl border p-3'>
          <div className='text-muted-foreground text-xs tracking-wide uppercase'>
            {t('Changed option keys sample')}
          </div>
          <div className='text-muted-foreground mt-2 flex flex-wrap gap-2 text-xs'>
            {changedKeys.map((key) => (
              <Badge key={key} variant='outline'>
                {key}
              </Badge>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}

function ReleaseCandidateList({
  selection,
  t,
}: {
  selection: ReleaseSelection
  t: OpsTranslate
}) {
  if (selection.rollbackCandidates.length === 0) {
    return (
      <div className='border-border/70 bg-background/70 text-muted-foreground rounded-[18px] border border-dashed p-4 text-sm leading-6'>
        {t(
          'No release history yet. Publish once to create the first rollback point.'
        )}
      </div>
    )
  }
  return (
    <div className='space-y-2'>
      {selection.rollbackCandidates.slice(0, 6).map((release) => {
        const selected =
          String(release.id) === selection.effectiveSelectedRollbackId
        return (
          <div
            key={release.id}
            className={`rounded-[18px] border p-3 ${
              selected
                ? 'border-primary/50 bg-primary/5'
                : 'border-border/70 bg-background/70'
            }`}
          >
            <div className='flex flex-wrap items-start justify-between gap-3'>
              <div className='space-y-1'>
                <div className='text-sm font-medium'>
                  {release.release_label || t('Unnamed snapshot')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  #{release.id} · {releaseActionLabel(release.action, t)} ·{' '}
                  {formatTime(release.applied_at || release.created_at)}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('Groups / options')}: {release.group_count} /{' '}
                  {release.option_count}
                </div>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                {selected ? (
                  <OpsStatusBadge tone='info'>
                    {t('Rollback target')}
                  </OpsStatusBadge>
                ) : null}
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={() => selection.selectRollback(release.id)}
                >
                  {selected ? t('Selected') : t('Use for rollback')}
                </Button>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function PublishReleaseCard({
  editor,
  t,
}: {
  editor: PublishReleaseEditor
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
      <div className='text-sm font-semibold'>
        {t('Publish a release snapshot')}
      </div>
      <div className='text-muted-foreground mt-2 text-sm leading-6'>
        {t(
          'Freeze the current live groups, access-control settings, reward policies, and connector truth into one auditable snapshot.'
        )}
      </div>
      <div className='mt-4 space-y-3'>
        <div className='space-y-2'>
          <label className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('Release label')}
          </label>
          <Input
            value={editor.label}
            onChange={(event) => editor.setLabel(event.target.value)}
            placeholder={t('Optional label, for example: evening rollout')}
          />
        </div>
        <div className='space-y-2'>
          <label className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('Publish note')}
          </label>
          <Textarea
            value={editor.note}
            onChange={(event) => editor.setNote(event.target.value)}
            placeholder={t('What changed in this publish?')}
            rows={4}
          />
        </div>
        <Button
          type='button'
          onClick={() => void editor.submit()}
          disabled={editor.disabled}
        >
          {editor.publishing
            ? t('Publishing')
            : t('Publish current live truth')}
        </Button>
      </div>
    </div>
  )
}

function RollbackReleaseCard({
  selectedRelease,
  editor,
  t,
}: {
  selectedRelease: OpsReleaseRecord | null
  editor: RollbackReleaseEditor
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-background/70 rounded-[18px] border p-4'>
      <div className='text-sm font-semibold'>
        {t('Rollback to a saved snapshot')}
      </div>
      <div className='text-muted-foreground mt-2 text-sm leading-6'>
        {selectedRelease
          ? t(
              'The selected snapshot will restore saved options, group registry rows, ChatOps settings, and game settings for this site.'
            )
          : t(
              'Pick one saved snapshot from the left column before rolling back.'
            )}
      </div>
      <div className='mt-4 space-y-3'>
        <div className='border-border/60 bg-muted/20 text-muted-foreground rounded-2xl border p-3 text-sm leading-6'>
          {selectedRelease ? (
            <div className='space-y-1'>
              <div className='text-foreground font-medium'>
                {selectedRelease.release_label || t('Unnamed snapshot')}
              </div>
              <div>
                #{selectedRelease.id} ·{' '}
                {releaseActionLabel(selectedRelease.action, t)} ·{' '}
                {formatTime(
                  selectedRelease.applied_at || selectedRelease.created_at
                )}
              </div>
            </div>
          ) : (
            t('No snapshot selected yet.')
          )}
        </div>
        <div className='space-y-2'>
          <label className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('Rollback note')}
          </label>
          <Textarea
            value={editor.note}
            onChange={(event) => editor.setNote(event.target.value)}
            placeholder={t('Why are we restoring this snapshot?')}
            rows={4}
          />
        </div>
        <Button
          type='button'
          variant='outline'
          onClick={() => void editor.submit()}
          disabled={editor.disabled}
        >
          {editor.rollingBack
            ? t('Rolling back')
            : t('Rollback selected snapshot')}
        </Button>
      </div>
    </div>
  )
}

function ReleaseTools({
  selection,
  releaseImpact,
  publishEditor,
  rollbackEditor,
  t,
}: {
  selection: ReleaseSelection
  releaseImpact: OpsReleaseImpactPreview | null
  publishEditor: PublishReleaseEditor
  rollbackEditor: RollbackReleaseEditor
  t: OpsTranslate
}) {
  return (
    <div className='grid gap-4 xl:grid-cols-[minmax(0,1.08fr)_minmax(340px,.92fr)] xl:items-start'>
      <div className='space-y-3'>
        <ReleaseSummaryCard release={selection.currentRelease} t={t} />
        <ReleaseImpactCard
          releaseImpact={releaseImpact}
          changedKeys={selection.optionChangedKeys}
          t={t}
        />
        <ReleaseCandidateList selection={selection} t={t} />
      </div>
      <div className='space-y-3'>
        <PublishReleaseCard editor={publishEditor} t={t} />
        <RollbackReleaseCard
          selectedRelease={selection.selectedRollbackRelease}
          editor={rollbackEditor}
          t={t}
        />
      </div>
    </div>
  )
}

export function ReleaseControlPanel({
  siteId,
  releases,
  releaseImpact,
  releaseActions,
  t,
  defaultOpen = false,
}: {
  siteId: string
  releases: OpsReleaseOverview | null
  releaseImpact: OpsReleaseImpactPreview | null
  releaseActions: OpsReleaseActions
  t: OpsTranslate
  defaultOpen?: boolean
}) {
  const selection = useReleaseSelection(releases, releaseImpact, defaultOpen)
  const publishEditor = usePublishReleaseEditor(
    siteId,
    releases,
    releaseActions,
    t
  )
  const rollbackEditor = useRollbackReleaseEditor(
    siteId,
    releases,
    releaseActions,
    selection.selectedRollbackRelease,
    t
  )
  return (
    <OpsPanel
      title={t('Publish and rollback control')}
      description={t(
        'Use the live ops truth as the only source of release snapshots. Publish freezes the current policy state, and rollback restores one verified snapshot without touching old host-side build artifacts.'
      )}
    >
      <div className='space-y-4'>
        <ReleaseControlSummary
          selection={selection}
          releaseImpact={releaseImpact}
          t={t}
        />
        {selection.showTools ? (
          <ReleaseTools
            selection={selection}
            releaseImpact={releaseImpact}
            publishEditor={publishEditor}
            rollbackEditor={rollbackEditor}
            t={t}
          />
        ) : null}
      </div>
    </OpsPanel>
  )
}
