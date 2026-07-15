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
import { useMemo, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { type OpsTranslate } from './ops-i18n'
import {
  type SavedOpsContext,
  normalizeText,
  recordValue,
} from './ops-live-foundation'
import { OpsPanel } from './ops-shared'
import {
  type OpsGroupActions,
  type OpsGroupSavePayload,
  type OpsRegistryGroup,
} from './use-ops-registry'

type GroupRegistryFormMode = 'create' | 'edit' | 'clone'

type GroupRegistryFormState = {
  mode: GroupRegistryFormMode
  sourceGroupId: string
  platform: string
  groupId: string
  groupName: string
  role: string
  status: string
  joinUrl: string
  inviteTargetGroupId: string
  notes: string
  copyChatOps: boolean
  copyGameRule: boolean
}

function normalizeOpsPlatformForForm(value: string | undefined) {
  const normalized = normalizeText(value).toLowerCase()
  if (
    normalized === 'tg' ||
    normalized === 'telegram' ||
    normalized === 'tg_group'
  ) {
    return 'tg'
  }
  if (normalized === 'community' || normalized === 'community_room')
    return 'community'
  return 'qq'
}

function buildInitialGroupForm(
  defaultPlatform: string
): GroupRegistryFormState {
  return {
    mode: 'create',
    sourceGroupId: '',
    platform: normalizeOpsPlatformForForm(defaultPlatform),
    groupId: '',
    groupName: '',
    role: 'ops_secondary',
    status: 'disabled',
    joinUrl: '',
    inviteTargetGroupId: '',
    notes: '',
    copyChatOps: true,
    copyGameRule: true,
  }
}

function buildGroupEditForm(
  group: OpsRegistryGroup,
  defaultPlatform: string
): GroupRegistryFormState {
  const config = recordValue(group.config)
  return {
    ...buildInitialGroupForm(defaultPlatform),
    mode: 'edit',
    sourceGroupId: String(group.id),
    platform: normalizeOpsPlatformForForm(
      group.platform_family || group.platform
    ),
    groupId: normalizeText(group.group_id),
    groupName: normalizeText(group.group_name),
    role: normalizeText(group.role) || 'ops_secondary',
    status: normalizeText(group.status) || 'disabled',
    joinUrl: normalizeText(config.join_url),
    inviteTargetGroupId: normalizeText(group.invite_target_group_id),
    notes: normalizeText(config.notes),
  }
}

export function selectClassName() {
  return 'border-input bg-background text-foreground focus-visible:border-ring focus-visible:ring-ring/50 h-8 w-full rounded-lg border px-2.5 py-1 text-sm outline-none transition-colors focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50'
}

function compactConfig(joinUrl: string, notes: string) {
  const config: Record<string, unknown> = {}
  const normalizedJoinUrl = normalizeText(joinUrl)
  const normalizedNotes = normalizeText(notes)
  if (normalizedJoinUrl) config.join_url = normalizedJoinUrl
  if (normalizedNotes) config.notes = normalizedNotes
  config.updated_from = 'ops-control-plane-ui'
  return config
}

type GroupRegistryBulkRow = {
  groupId: string
  groupName: string
}

function parseGroupRegistryBulkLine(line: string): GroupRegistryBulkRow | null {
  if (line.startsWith('{') && line.endsWith('}')) {
    try {
      const parsed = recordValue(JSON.parse(line))
      const groupId = normalizeText(
        parsed.group_id ?? parsed.groupId ?? parsed.id
      )
      if (!groupId) return null
      const groupName = normalizeText(
        parsed.group_name ?? parsed.groupName ?? parsed.name
      )
      return { groupId, groupName: groupName || groupId }
    } catch {
      return null
    }
  }

  const parts = line
    .split(/[,\t|]/)
    .map((part) => normalizeText(part))
    .filter(Boolean)
  const groupId = parts[0] || ''
  if (!groupId) return null
  const groupName = parts.slice(1).join(' ')
  return { groupId, groupName: groupName || groupId }
}

function parseGroupRegistryBulkDraft(draft: string) {
  const seen = new Set<string>()
  const rows: GroupRegistryBulkRow[] = []
  for (const rawLine of draft.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const row = parseGroupRegistryBulkLine(line)
    if (!row || seen.has(row.groupId)) continue
    seen.add(row.groupId)
    rows.push(row)
  }
  return rows
}

type GroupRegistryFormUpdate = <K extends keyof GroupRegistryFormState>(
  key: K,
  value: GroupRegistryFormState[K]
) => void

function buildGroupRegistryPayload(
  form: GroupRegistryFormState,
  siteId: string,
  saved: SavedOpsContext
): OpsGroupSavePayload {
  const groupId = normalizeText(form.groupId)
  const payload: OpsGroupSavePayload = {
    site_id: siteId || saved.siteId,
    group_id: groupId,
    group_name: normalizeText(form.groupName) || groupId,
    invite_target_group_id:
      normalizeText(form.inviteTargetGroupId) || undefined,
    role: form.role,
    status: form.status,
    language: 'zh-CN',
    timezone: 'Asia/Shanghai',
    config: compactConfig(form.joinUrl, form.notes),
  }
  if (form.mode === 'clone') {
    payload.copy_chatops = form.copyChatOps
    payload.copy_game_rule = form.copyGameRule
  } else {
    payload.platform = form.platform
  }
  return payload
}

async function saveGroupRegistryForm(
  form: GroupRegistryFormState,
  source: OpsRegistryGroup | undefined,
  payload: OpsGroupSavePayload,
  actions: OpsGroupActions
) {
  if (form.mode === 'edit') return actions.update(Number(source?.id), payload)
  if (form.mode === 'clone') return actions.clone(Number(source?.id), payload)
  return actions.create(payload)
}

function useGroupRegistryFormController({
  siteId,
  groups,
  saved,
  groupActions,
  defaultPlatform,
  t,
}: {
  siteId: string
  groups: OpsRegistryGroup[]
  saved: SavedOpsContext
  groupActions: OpsGroupActions
  defaultPlatform: string
  t: OpsTranslate
}) {
  const [form, setForm] = useState<GroupRegistryFormState>(() =>
    buildInitialGroupForm(defaultPlatform)
  )
  const selectedSource = groups.find(
    (group) => String(group.id) === form.sourceGroupId
  )
  const update: GroupRegistryFormUpdate = (key, value) =>
    setForm((current) => ({ ...current, [key]: value }))
  const selectSource = (sourceGroupId: string) => {
    const source = groups.find((group) => String(group.id) === sourceGroupId)
    if (form.mode === 'edit' && source) {
      setForm(buildGroupEditForm(source, defaultPlatform))
      return
    }
    update('sourceGroupId', sourceGroupId)
  }
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const groupId = normalizeText(form.groupId)
    if (!groupId) {
      toast.error(t('Group ID is required.'))
      return
    }
    if ((form.mode === 'clone' || form.mode === 'edit') && !selectedSource) {
      toast.error(
        form.mode === 'clone'
          ? t('Choose a source group before cloning.')
          : t('Choose a group before editing.')
      )
      return
    }
    try {
      const result = await saveGroupRegistryForm(
        form,
        selectedSource,
        buildGroupRegistryPayload(form, siteId, saved),
        groupActions
      )
      toast.success(
        t('Group saved and live truth refreshed: {{group}}', {
          group: result?.group_name || result?.group_id || groupId,
        })
      )
      setForm(buildInitialGroupForm(defaultPlatform))
    } catch (error) {
      toast.error(
        t('Group save failed: {{reason}}', {
          reason: error instanceof Error ? error.message : t('Unknown error'),
        })
      )
    }
  }
  return {
    form,
    update,
    selectedSource,
    selectSource,
    submit,
    reset: () => setForm(buildInitialGroupForm(defaultPlatform)),
  }
}

function useGroupBulkImport({
  siteId,
  saved,
  form,
  groupActions,
  t,
}: {
  siteId: string
  saved: SavedOpsContext
  form: GroupRegistryFormState
  groupActions: OpsGroupActions
  t: OpsTranslate
}) {
  const [draft, setDraft] = useState('')
  const [importing, setImporting] = useState(false)
  const rows = useMemo(() => parseGroupRegistryBulkDraft(draft), [draft])
  const submit = async () => {
    if (!rows.length) {
      toast.error(t('Paste at least one group ID before bulk import.'))
      return
    }
    const payloadBase = buildGroupRegistryPayload(form, siteId, saved)
    setImporting(true)
    try {
      const result = await groupActions.createBulk({
        site_id: siteId || saved.siteId,
        continue_on_error: true,
        items: rows.map((row) => ({
          ...payloadBase,
          group_id: row.groupId,
          group_name: row.groupName,
        })),
      })
      const successCount = Number(result?.created_count || 0)
      const failures = result?.failed || []
      if (!failures.length) {
        toast.success(
          t(
            'Bulk import completed: {{count}} groups were added to the live registry.',
            { count: String(successCount) }
          )
        )
        setDraft('')
      } else {
        toast.warning(
          t(
            'Bulk import finished with partial failures: {{success}} succeeded, {{failed}} failed.',
            {
              success: String(successCount),
              failed: String(failures.length),
            }
          )
        )
      }
    } catch (error) {
      toast.error(
        t('Group save failed: {{reason}}', {
          reason: error instanceof Error ? error.message : t('Unknown error'),
        })
      )
    } finally {
      setImporting(false)
    }
  }
  return { draft, setDraft, rows, importing, submit }
}

type GroupRegistryController = ReturnType<typeof useGroupRegistryFormController>
type GroupBulkImport = ReturnType<typeof useGroupBulkImport>

function GroupRegistryStats({
  groups,
  t,
}: {
  groups: OpsRegistryGroup[]
  t: OpsTranslate
}) {
  const activeGroups = groups.filter((group) => group.enabled).length
  const metrics = [
    [t('Registered'), groups.length],
    [t('Active groups'), activeGroups],
    [t('Safe default'), t('Disabled after save')],
    [t('After saving'), t('Live truth refreshes')],
  ] as const
  return (
    <div className='grid gap-3 md:grid-cols-4'>
      {metrics.map(([label, value]) => (
        <div
          key={label}
          className='border-border/70 bg-background/70 rounded-2xl border p-3'
        >
          <div className='text-muted-foreground text-xs font-medium'>
            {label}
          </div>
          <div
            className={
              typeof value === 'number'
                ? 'mt-1 text-xl font-semibold'
                : 'mt-1 text-sm font-semibold'
            }
          >
            {value}
          </div>
        </div>
      ))}
    </div>
  )
}

function GroupRegistryModeFields({
  controller,
  groups,
  t,
}: {
  controller: GroupRegistryController
  groups: OpsRegistryGroup[]
  t: OpsTranslate
}) {
  const { form, update, selectedSource, selectSource } = controller
  const editMode = form.mode === 'edit'
  const sourceMode = editMode || form.mode === 'clone'
  return (
    <div className='grid gap-3 md:grid-cols-3'>
      <label className='space-y-1.5 text-sm'>
        <span className='font-medium'>{t('Action')}</span>
        <select
          className={selectClassName()}
          value={form.mode}
          onChange={(event) =>
            update('mode', event.target.value as GroupRegistryFormMode)
          }
        >
          <option value='create'>{t('Create new group')}</option>
          <option value='edit'>{t('Edit existing group')}</option>
          <option value='clone'>{t('Clone from existing group')}</option>
        </select>
      </label>

      {sourceMode ? (
        <label className='space-y-1.5 text-sm md:col-span-2'>
          <span className='font-medium'>
            {editMode ? t('Group to edit') : t('Source group')}
          </span>
          <select
            className={selectClassName()}
            value={form.sourceGroupId}
            onChange={(event) => selectSource(event.target.value)}
          >
            <option value=''>{t('Choose source group')}</option>
            {groups.map((group) => (
              <option key={group.id} value={group.id}>
                {group.group_name || group.group_id} · {group.group_id}
              </option>
            ))}
          </select>
          <span className='text-muted-foreground text-xs leading-5'>
            {editMode
              ? selectedSource
                ? t(
                    'Loaded from the real group registry. Saving here updates the existing group directly.'
                  )
                : t(
                    'Pick a registered group to edit its role, status, invite target, join link, and notes.'
                  )
              : selectedSource
                ? t(
                    'Clone will copy the selected group identity template and optional capability rules.'
                  )
                : t(
                    'Pick a configured group to reuse its safe template and reduce manual entry.'
                  )}
          </span>
        </label>
      ) : (
        <label className='space-y-1.5 text-sm md:col-span-2'>
          <span className='font-medium'>{t('Platform')}</span>
          <select
            className={selectClassName()}
            value={form.platform}
            onChange={(event) => update('platform', event.target.value)}
          >
            <option value='qq'>{t('QQ group')}</option>
            <option value='tg'>{t('TG group')}</option>
            <option value='community'>{t('Community room')}</option>
          </select>
          <span className='text-muted-foreground text-xs leading-5'>
            {t(
              'A site can register multiple QQ, TG, and community groups independently.'
            )}
          </span>
        </label>
      )}
    </div>
  )
}

function GroupRegistryIdentityFields({
  controller,
  t,
}: {
  controller: GroupRegistryController
  t: OpsTranslate
}) {
  const { form, update } = controller
  const editMode = form.mode === 'edit'
  return (
    <div className='grid gap-3 md:grid-cols-2'>
      <label className='space-y-1.5 text-sm'>
        <span className='font-medium'>
          {editMode ? t('Group ID') : t('New group ID')}
        </span>
        <Input
          value={form.groupId}
          onChange={(event) => update('groupId', event.target.value)}
          placeholder={t('Example: 123456789 or -1001234567890')}
          disabled={editMode}
        />
      </label>
      <label className='space-y-1.5 text-sm'>
        <span className='font-medium'>{t('Display name')}</span>
        <Input
          value={form.groupName}
          onChange={(event) => update('groupName', event.target.value)}
          placeholder={t('Example: A site QQ main group 2')}
        />
      </label>
    </div>
  )
}

function GroupRegistryPolicyFields({
  controller,
  t,
}: {
  controller: GroupRegistryController
  t: OpsTranslate
}) {
  const { form, update } = controller
  return (
    <>
      <div className='grid gap-3 md:grid-cols-4'>
        <label className='space-y-1.5 text-sm'>
          <span className='font-medium'>{t('Role')}</span>
          <select
            className={selectClassName()}
            value={form.role}
            onChange={(event) => update('role', event.target.value)}
          >
            <option value='primary_mainfield'>{t('Primary main field')}</option>
            <option value='community_intake'>{t('Community intake')}</option>
            <option value='ops_secondary'>{t('Ops secondary')}</option>
            <option value='campaign'>{t('Campaign')}</option>
            <option value='backup'>{t('Backup')}</option>
          </select>
        </label>
        <label className='space-y-1.5 text-sm'>
          <span className='font-medium'>{t('Status')}</span>
          <select
            className={selectClassName()}
            value={form.status}
            onChange={(event) => update('status', event.target.value)}
          >
            <option value='disabled'>{t('Disabled')}</option>
            <option value='active'>{t('Active')}</option>
            <option value='archived'>{t('Archived')}</option>
          </select>
        </label>
        <label className='space-y-1.5 text-sm'>
          <span className='font-medium'>{t('Join link')}</span>
          <Input
            value={form.joinUrl}
            onChange={(event) => update('joinUrl', event.target.value)}
            placeholder={t('Optional join link shown to admins')}
          />
        </label>
        <label className='space-y-1.5 text-sm'>
          <span className='font-medium'>{t('Invite target group ID')}</span>
          <Input
            value={form.inviteTargetGroupId}
            onChange={(event) =>
              update('inviteTargetGroupId', event.target.value)
            }
            placeholder={t('Optional target group unlocked by invites')}
          />
        </label>
      </div>
      <label className='space-y-1.5 text-sm'>
        <span className='font-medium'>{t('Admin notes')}</span>
        <Textarea
          value={form.notes}
          onChange={(event) => update('notes', event.target.value)}
          placeholder={t(
            'Why this group exists, which unlock lane it belongs to, and what to verify after saving.'
          )}
        />
      </label>
    </>
  )
}

function GroupBulkImportEditor({
  bulk,
  busy,
  t,
}: {
  bulk: GroupBulkImport
  busy: boolean
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-muted/20 space-y-3 rounded-2xl border p-3'>
      <div className='flex flex-col gap-1 md:flex-row md:items-center md:justify-between'>
        <div>
          <div className='text-sm font-medium'>
            {t('Bulk add groups with the same defaults')}
          </div>
          <div className='text-muted-foreground text-xs leading-5'>
            {t(
              'Paste one group per line. Supported formats: group_id, group_id,display_name, or JSON like {"group_id":"123","group_name":"Main QQ 2"}.'
            )}
          </div>
        </div>
        <Badge variant='secondary'>
          {t('Preview {{count}} groups', {
            count: String(bulk.rows.length),
          })}
        </Badge>
      </div>
      <Textarea
        value={bulk.draft}
        onChange={(event) => bulk.setDraft(event.target.value)}
        placeholder={t(
          'Example:\n925249987,A site QQ main group 2\n-1001234567890,B site TG backup\ncommunity-room-2,Community intake room 2'
        )}
      />
      <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
        <span>
          {t(
            'The current platform / role / status / join link / invite target / notes fields are applied to every imported row.'
          )}
        </span>
        {bulk.rows.length ? (
          <span>
            {t('Preview: {{items}}', {
              items: bulk.rows
                .slice(0, 3)
                .map((row) => `${row.groupName} · ${row.groupId}`)
                .join(' / '),
            })}
          </span>
        ) : null}
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => bulk.setDraft('')}
          disabled={busy || !bulk.draft}
        >
          {t('Clear bulk draft')}
        </Button>
        <Button
          type='button'
          size='sm'
          onClick={bulk.submit}
          disabled={busy || !bulk.rows.length}
        >
          {bulk.importing ? t('Importing') : t('Bulk create groups')}
        </Button>
      </div>
    </div>
  )
}

function GroupCloneOptions({
  controller,
  t,
}: {
  controller: GroupRegistryController
  t: OpsTranslate
}) {
  const { form, update } = controller
  return (
    <div className='border-border/70 bg-muted/20 grid gap-3 rounded-2xl border p-3 md:grid-cols-2'>
      <label className='flex items-start gap-3 text-sm'>
        <input
          type='checkbox'
          className='border-border mt-1 h-4 w-4 rounded'
          checked={form.copyChatOps}
          onChange={(event) => update('copyChatOps', event.target.checked)}
        />
        <span>
          <span className='block font-medium'>
            {t('Copy check-in / verify / invite settings')}
          </span>
          <span className='text-muted-foreground text-xs leading-5'>
            {t(
              'Copies group_chatops_configs so the cloned group starts with the same operation rules.'
            )}
          </span>
        </span>
      </label>
      <label className='flex items-start gap-3 text-sm'>
        <input
          type='checkbox'
          className='border-border mt-1 h-4 w-4 rounded'
          checked={form.copyGameRule}
          onChange={(event) => update('copyGameRule', event.target.checked)}
        />
        <span>
          <span className='block font-medium'>
            {t('Copy game rule settings')}
          </span>
          <span className='text-muted-foreground text-xs leading-5'>
            {t(
              'Copies group_game_configs so game availability and reward rules stay aligned.'
            )}
          </span>
        </span>
      </label>
    </div>
  )
}

function GroupRegistryFormActions({
  controller,
  busy,
  t,
}: {
  controller: GroupRegistryController
  busy: boolean
  t: OpsTranslate
}) {
  const { form, reset } = controller
  return (
    <div className='border-border/70 bg-background/70 flex flex-col gap-3 rounded-2xl border p-3 md:flex-row md:items-center md:justify-between'>
      <div className='text-muted-foreground text-xs leading-5'>
        {t(
          'Saving here writes to the real group registry. Keep new groups disabled until connector, unlock policy, and capability checks all match.'
        )}
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={reset}
          disabled={busy}
        >
          {t('Reset')}
        </Button>
        <Button type='submit' size='sm' disabled={busy}>
          {busy
            ? t('Saving')
            : form.mode === 'edit'
              ? t('Save group changes')
              : form.mode === 'clone'
                ? t('Clone group')
                : t('Create group')}
        </Button>
      </div>
    </div>
  )
}

export function GroupRegistryActionsPanel({
  siteId,
  groups,
  saved,
  groupActions,
  defaultPlatform,
  title,
  description,
  t,
}: {
  siteId: string
  groups: OpsRegistryGroup[]
  saved: SavedOpsContext
  groupActions: OpsGroupActions
  defaultPlatform: string
  title: string
  description: string
  t: OpsTranslate
}) {
  const controller = useGroupRegistryFormController({
    siteId,
    groups,
    saved,
    groupActions,
    defaultPlatform,
    t,
  })
  const bulk = useGroupBulkImport({
    siteId,
    saved,
    form: controller.form,
    groupActions,
    t,
  })
  const busy = groupActions.saving || bulk.importing

  return (
    <OpsPanel title={title} description={description}>
      <form className='space-y-4' onSubmit={controller.submit}>
        <GroupRegistryStats groups={groups} t={t} />
        <GroupRegistryModeFields
          controller={controller}
          groups={groups}
          t={t}
        />
        <GroupRegistryIdentityFields controller={controller} t={t} />
        <GroupRegistryPolicyFields controller={controller} t={t} />
        {controller.form.mode === 'create' ? (
          <GroupBulkImportEditor bulk={bulk} busy={busy} t={t} />
        ) : null}
        {controller.form.mode === 'clone' ? (
          <GroupCloneOptions controller={controller} t={t} />
        ) : null}
        <GroupRegistryFormActions controller={controller} busy={busy} t={t} />
      </form>
    </OpsPanel>
  )
}
