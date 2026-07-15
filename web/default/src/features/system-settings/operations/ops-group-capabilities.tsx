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
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { selectClassName } from './ops-group-registry'
import { type OpsTranslate } from './ops-i18n'
import {
  type OpsRenderableGroup,
  boolValue,
  capabilityPolicyOf,
  gameConfigsOf,
  groupRoleLabel,
  normalizeText,
  numberValue,
  platformLabel,
  prettifyIdentifier,
  recordValue,
  rewardPolicyOf,
} from './ops-live-foundation'
import { OpsPanel } from './ops-shared'
import {
  type OpsGroupChatOpsSavePayload,
  type OpsGroupActions,
  type OpsGroupGamesSavePayload,
} from './use-ops-registry'

type StructuredRuleFieldKind = 'text' | 'number' | 'boolean' | 'list'

type StructuredRuleField = {
  id: string
  path: string
  kind: StructuredRuleFieldKind
  value: string
}

type GroupCapabilityGameFormRow = {
  game_code: string
  enabled: boolean
  budget_pool: string
  rule_fields: StructuredRuleField[]
}

const RULE_FIELD_KIND_OPTIONS: StructuredRuleFieldKind[] = [
  'text',
  'number',
  'boolean',
  'list',
]

const CHATOPS_RULE_DEFAULT_PATHS = [
  'checkin.channel_scope',
  'checkin.require_verify',
  'checkin.reward_min',
  'checkin.reward_max',
  'verify.min_quota_required',
  'verify.community_host',
  'invite.inviter_reward_quota',
  'invite.invitee_reward_quota',
  'invite.max_per_user_day',
]

const GAME_RULE_DEFAULT_PATHS = [
  'min_bet',
  'max_bet',
  'daily_limit',
  'cooldown_seconds',
  'reward_multiplier',
  'timeout_seconds',
  'question_count',
  'question_scope',
  'question_ttl_seconds',
  'max_attempts_per_question',
  'quiz_limit_per_group',
  'max_winners_per_question',
  'reward_quota',
  'max_per_user_day',
]

const RULE_FIELD_LABELS: Record<string, string> = {
  'checkin.budget_pool': 'Check-in budget pool',
  'checkin.channel_scope': 'Channel scope',
  'checkin.enabled': 'Check-in enabled',
  'checkin.require_verify': 'Require verification before reward',
  'checkin.reward_min': 'Check-in minimum reward',
  'checkin.reward_max': 'Check-in maximum reward',
  'verify.enabled': 'Verify enabled',
  'verify.budget_pool': 'Verify budget pool',
  'verify.community_host': 'Community host',
  'verify.min_quota_required': 'Minimum quota required',
  'invite.enabled': 'Invite enabled',
  'invite.budget_pool': 'Invite budget pool',
  'invite.inviter_reward_quota': 'Inviter reward quota',
  'invite.invitee_reward_quota': 'Invitee reward quota',
  'invite.reward_quota': 'Invite reward quota',
  'invite.max_per_user_day': 'Max rewards per user per day',
  budget_pool: 'Budget pool',
  channel_scope: 'Channel scope',
  enabled: 'Enabled',
  require_verify: 'Require verification before reward',
  reward_min: 'Minimum reward',
  reward_max: 'Maximum reward',
  inviter_reward_quota: 'Inviter reward quota',
  invitee_reward_quota: 'Invitee reward quota',
  min_quota_required: 'Minimum quota required',
  min_bet: 'Minimum bet',
  max_bet: 'Maximum bet',
  daily_limit: 'Daily limit',
  cooldown_seconds: 'Cooldown seconds',
  reward_multiplier: 'Reward multiplier',
  timeout_seconds: 'Timeout seconds',
  question_count: 'Question count',
  question_scope: 'Question scope',
  question_ttl_seconds: 'Question TTL seconds',
  max_attempts_per_question: 'Max attempts per question',
  quiz_limit_per_group: 'Quiz rounds per group per day',
  max_winners_per_question: 'Max winners per question',
  reward_quota: 'Reward quota',
  max_per_user_day: 'Max rewards per user per day',
}

const RULE_FIELD_DEFAULTS: Partial<
  Record<string, { kind: StructuredRuleFieldKind; value: string }>
> = {
  budget_pool: { kind: 'text', value: 'activity' },
  channel_scope: { kind: 'text', value: 'all' },
  question_scope: { kind: 'text', value: 'per_user' },
  question_ttl_seconds: { kind: 'number', value: '600' },
  max_attempts_per_question: { kind: 'number', value: '2' },
  quiz_limit_per_group: { kind: 'number', value: '0' },
  max_winners_per_question: { kind: 'number', value: '0' },
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function createRuleFieldId(path = 'field') {
  return `${path || 'field'}-${Math.random().toString(36).slice(2, 9)}`
}

function inferRuleFieldKind(value: unknown): StructuredRuleFieldKind {
  if (typeof value === 'boolean') return 'boolean'
  if (typeof value === 'number') return 'number'
  if (Array.isArray(value)) return 'list'
  return 'text'
}

function serializeRuleFieldValue(value: unknown) {
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeText(item))
      .filter(Boolean)
      .join(', ')
  }
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') return String(value)
  return normalizeText(value)
}

function flattenRuleFields(value: unknown, prefix = ''): StructuredRuleField[] {
  const record = recordValue(value)
  return Object.entries(record).flatMap(([key, rawValue]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (isPlainRecord(rawValue)) {
      return flattenRuleFields(rawValue, path)
    }
    return [
      {
        id: createRuleFieldId(path),
        path,
        kind: inferRuleFieldKind(rawValue),
        value: serializeRuleFieldValue(rawValue),
      },
    ]
  })
}

function defaultRuleFields(paths: string[]): StructuredRuleField[] {
  return paths.map((path) => {
    const preset = RULE_FIELD_DEFAULTS[path]
    const inferredKind: StructuredRuleFieldKind =
      path.includes('enabled') || path.includes('require_')
        ? 'boolean'
        : path.includes('_scope') || path.includes('budget_pool')
          ? 'text'
          : 'number'
    return {
      id: createRuleFieldId(path),
      path,
      kind: preset?.kind || inferredKind,
      value:
        preset?.value ||
        (inferredKind === 'boolean'
          ? 'false'
          : inferredKind === 'text'
            ? ''
            : '0'),
    }
  })
}

function parseRuleListValue(raw: string) {
  return raw
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseStructuredRuleValue(field: StructuredRuleField): unknown {
  if (field.kind === 'boolean') return boolValue(field.value)
  if (field.kind === 'number') return numberValue(field.value)
  if (field.kind === 'list') return parseRuleListValue(field.value)
  return normalizeText(field.value)
}

function setNestedRuleValue(
  target: Record<string, unknown>,
  path: string,
  value: unknown
) {
  const parts = path
    .split('.')
    .map((part) => normalizeText(part))
    .filter(Boolean)
  if (!parts.length) return
  let cursor = target
  parts.forEach((part, index) => {
    if (index === parts.length - 1) {
      cursor[part] = value
      return
    }
    const next = cursor[part]
    if (!isPlainRecord(next)) {
      cursor[part] = {}
    }
    cursor = cursor[part] as Record<string, unknown>
  })
}

function ruleFromStructuredFields(fields: StructuredRuleField[]) {
  const rule: Record<string, unknown> = {}
  fields.forEach((field) => {
    const path = normalizeText(field.path)
    if (!path) return
    setNestedRuleValue(rule, path, parseStructuredRuleValue(field))
  })
  return rule
}

function labelForRuleField(path: string) {
  return (
    RULE_FIELD_LABELS[path] || prettifyIdentifier(path.split('.').pop() || path)
  )
}

function createStructuredRuleField(path = ''): StructuredRuleField {
  const preset = path ? RULE_FIELD_DEFAULTS[path] : undefined
  const inferredKind: StructuredRuleFieldKind =
    path.includes('enabled') || path.includes('require_') ? 'boolean' : 'text'
  return {
    id: createRuleFieldId(path || 'custom'),
    path,
    kind: preset?.kind || inferredKind,
    value: preset?.value || (inferredKind === 'boolean' ? 'false' : ''),
  }
}

function StructuredRuleFieldsHeader({
  missingDefaultPaths,
  onAdd,
  t,
}: {
  missingDefaultPaths: string[]
  onAdd: (path?: string) => void
  t: OpsTranslate
}) {
  return (
    <div className='flex flex-col gap-2 md:flex-row md:items-start md:justify-between'>
      <div>
        <div className='text-sm font-medium'>{t('Structured rule form')}</div>
        <div className='text-muted-foreground text-xs leading-5'>
          {t(
            'Edit rule fields as form rows. Dot paths represent nested settings and are saved back to the same live rule object.'
          )}
        </div>
      </div>
      <div className='flex flex-wrap gap-2'>
        {missingDefaultPaths.slice(0, 3).map((path) => (
          <Button
            key={path}
            type='button'
            size='sm'
            variant='outline'
            onClick={() => onAdd(path)}
          >
            {t('Add')} {t(labelForRuleField(path))}
          </Button>
        ))}
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={() => onAdd()}
        >
          {t('Add custom rule field')}
        </Button>
      </div>
    </div>
  )
}

function StructuredRuleFieldRow({
  field,
  index,
  onUpdate,
  onRemove,
  t,
}: {
  field: StructuredRuleField
  index: number
  onUpdate: (index: number, patch: Partial<StructuredRuleField>) => void
  onRemove: (index: number) => void
  t: OpsTranslate
}) {
  return (
    <div className='border-border/60 bg-background/80 grid gap-2 rounded-xl border p-2 md:grid-cols-[minmax(170px,1fr)_140px_minmax(180px,1.2fr)_auto] md:items-end'>
      <label className='space-y-1 text-xs'>
        <span className='font-medium'>{t('Rule field')}</span>
        <Input
          value={field.path}
          onChange={(event) => onUpdate(index, { path: event.target.value })}
          placeholder={t('Example: invite.max_per_user_day')}
        />
        {normalizeText(field.path) ? (
          <span className='text-muted-foreground block text-[11px]'>
            {t(labelForRuleField(field.path))}
          </span>
        ) : null}
      </label>
      <label className='space-y-1 text-xs'>
        <span className='font-medium'>{t('Value type')}</span>
        <select
          className={selectClassName()}
          value={field.kind}
          onChange={(event) =>
            onUpdate(index, {
              kind: event.target.value as StructuredRuleFieldKind,
            })
          }
        >
          {RULE_FIELD_KIND_OPTIONS.map((kind) => (
            <option key={kind} value={kind}>
              {t(`Rule type ${kind}`)}
            </option>
          ))}
        </select>
      </label>
      <label className='space-y-1 text-xs'>
        <span className='font-medium'>{t('Value')}</span>
        {field.kind === 'boolean' ? (
          <select
            className={selectClassName()}
            value={boolValue(field.value) ? 'true' : 'false'}
            onChange={(event) => onUpdate(index, { value: event.target.value })}
          >
            <option value='true'>{t('Enabled')}</option>
            <option value='false'>{t('Disabled')}</option>
          </select>
        ) : (
          <Input
            type={field.kind === 'number' ? 'number' : 'text'}
            value={field.value}
            onChange={(event) => onUpdate(index, { value: event.target.value })}
            placeholder={
              field.kind === 'list'
                ? t('Separate multiple values with commas')
                : t('Enter field value')
            }
          />
        )}
      </label>
      <Button
        type='button'
        size='sm'
        variant='outline'
        onClick={() => onRemove(index)}
      >
        {t('Remove')}
      </Button>
    </div>
  )
}

function StructuredRuleFieldsEditor({
  fields,
  defaultPaths,
  onChange,
  t,
}: {
  fields: StructuredRuleField[]
  defaultPaths: string[]
  onChange: (fields: StructuredRuleField[]) => void
  t: OpsTranslate
}) {
  const updateField = (index: number, patch: Partial<StructuredRuleField>) =>
    onChange(
      fields.map((field, fieldIndex) =>
        fieldIndex === index ? { ...field, ...patch } : field
      )
    )
  const addField = (path = '') =>
    onChange([...fields, createStructuredRuleField(path)])
  const removeField = (index: number) =>
    onChange(fields.filter((_, fieldIndex) => fieldIndex !== index))
  const missingDefaultPaths = defaultPaths.filter(
    (path) => !fields.some((field) => normalizeText(field.path) === path)
  )

  return (
    <div className='border-border/70 bg-muted/20 mt-3 space-y-3 rounded-2xl border border-dashed p-3'>
      <StructuredRuleFieldsHeader
        missingDefaultPaths={missingDefaultPaths}
        onAdd={addField}
        t={t}
      />
      {fields.length ? (
        <div className='space-y-2'>
          {fields.map((field, index) => (
            <StructuredRuleFieldRow
              key={field.id}
              field={field}
              index={index}
              onUpdate={updateField}
              onRemove={removeField}
              t={t}
            />
          ))}
        </div>
      ) : (
        <div className='border-border/60 bg-background/70 text-muted-foreground rounded-xl border p-3 text-xs leading-5'>
          {t(
            'No structured rule fields yet. Add common fields or a custom field; saving will write an empty rule object until fields are added.'
          )}
        </div>
      )}
    </div>
  )
}

function initialGameFormRows(
  group: OpsRenderableGroup
): GroupCapabilityGameFormRow[] {
  return gameConfigsOf(group).map((game) => ({
    game_code: normalizeText(game.game_code),
    enabled: boolValue(game.enabled),
    budget_pool: normalizeText(game.budget_pool) || 'game',
    rule_fields: flattenRuleFields(game.rule),
  }))
}

function initialGroupChatOpsForm(
  group: OpsRenderableGroup | null
): OpsGroupChatOpsSavePayload {
  if (!group) return {}
  const policy = capabilityPolicyOf(group)
  const reward = rewardPolicyOf(group)
  return {
    checkin_enabled: boolValue(policy.checkin_enabled),
    verify_enabled: boolValue(policy.verify_enabled),
    invite_enabled: boolValue(policy.invite_enabled),
    checkin_quota: numberValue(reward.checkin_quota),
    verify_min_quota: numberValue(reward.verify_min_quota),
    invite_reward_quota: numberValue(reward.invite_reward_quota),
    invitee_reward_quota: numberValue(reward.invitee_reward_quota),
    daily_group_reward_limit: numberValue(reward.daily_group_reward_limit),
  }
}

type GroupCapabilityEditorPanelProps = {
  groups: OpsRenderableGroup[]
  groupActions: OpsGroupActions
  t: OpsTranslate
}

export function GroupCapabilityEditorPanel({
  groups,
  groupActions,
  t,
}: GroupCapabilityEditorPanelProps) {
  const [selectedId, setSelectedId] = useState('')
  const selectedGroup =
    groups.find((group) => String(group.id) === selectedId) ?? groups[0] ?? null
  const effectiveSelectedId = selectedGroup ? String(selectedGroup.id) : ''

  return (
    <GroupCapabilityEditorPanelContent
      key={effectiveSelectedId || 'empty'}
      groups={groups}
      groupActions={groupActions}
      t={t}
      selectedGroup={selectedGroup}
      onSelectedIdChange={setSelectedId}
    />
  )
}

const CHATOPS_SWITCH_FIELDS = [
  ['checkin_enabled', 'Check-in'],
  ['verify_enabled', 'Verify'],
  ['invite_enabled', 'Invite'],
] as const

const CHATOPS_REWARD_FIELDS = [
  ['checkin_quota', 'Check-in quota'],
  ['verify_min_quota', 'Verify min quota'],
  ['invite_reward_quota', 'Inviter reward'],
  ['invitee_reward_quota', 'Invitee reward'],
  ['daily_group_reward_limit', 'Daily group cap'],
] as const

function buildGroupChatOpsSavePayload(
  form: OpsGroupChatOpsSavePayload,
  ruleFields: StructuredRuleField[]
): OpsGroupChatOpsSavePayload {
  return {
    ...form,
    checkin_quota: numberValue(form.checkin_quota),
    verify_min_quota: numberValue(form.verify_min_quota),
    invite_reward_quota: numberValue(form.invite_reward_quota),
    invitee_reward_quota: numberValue(form.invitee_reward_quota),
    daily_group_reward_limit: numberValue(form.daily_group_reward_limit),
    rule: ruleFromStructuredFields(ruleFields),
  }
}

function buildGroupGamesSavePayload(
  rows: GroupCapabilityGameFormRow[]
): OpsGroupGamesSavePayload['games'] {
  const games: OpsGroupGamesSavePayload['games'] = []
  rows.forEach((row) => {
    const gameCode = normalizeText(row.game_code).toLowerCase()
    if (!gameCode) return
    games.push({
      game_code: gameCode,
      enabled: row.enabled,
      budget_pool: normalizeText(row.budget_pool) || 'game',
      rule: ruleFromStructuredFields(row.rule_fields),
    })
  })
  return games
}

function useGroupCapabilityEditorController(
  selectedGroup: OpsRenderableGroup | null,
  groupActions: OpsGroupActions,
  t: OpsTranslate
) {
  const [chatOpsForm, setChatOpsForm] = useState<OpsGroupChatOpsSavePayload>(
    () => initialGroupChatOpsForm(selectedGroup)
  )
  const [chatOpsRuleFields, setChatOpsRuleFields] = useState<
    StructuredRuleField[]
  >(() =>
    selectedGroup
      ? flattenRuleFields(capabilityPolicyOf(selectedGroup).rule)
      : []
  )
  const [gameRows, setGameRows] = useState<GroupCapabilityGameFormRow[]>(() =>
    selectedGroup ? initialGameFormRows(selectedGroup) : []
  )
  const updateChatOps = <K extends keyof OpsGroupChatOpsSavePayload>(
    key: K,
    value: OpsGroupChatOpsSavePayload[K]
  ) => setChatOpsForm((current) => ({ ...current, [key]: value }))
  const updateGame = (
    index: number,
    patch: Partial<GroupCapabilityGameFormRow>
  ) =>
    setGameRows((current) =>
      current.map((row, rowIndex) =>
        rowIndex === index ? { ...row, ...patch } : row
      )
    )
  const saveChatOps = async () => {
    if (!selectedGroup) return
    try {
      await groupActions.saveChatOps(
        Number(selectedGroup.id),
        buildGroupChatOpsSavePayload(chatOpsForm, chatOpsRuleFields)
      )
      toast.success(t('Group features and reward settings saved.'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Group feature save failed')
      )
    }
  }
  const saveGames = async () => {
    if (!selectedGroup) return
    try {
      const games = buildGroupGamesSavePayload(gameRows)
      if (!games.length) {
        toast.error(t('Add at least one game code before saving game rules.'))
        return
      }
      await groupActions.saveGames(Number(selectedGroup.id), { games })
      toast.success(
        t('Group game and reward rules saved to the live database.')
      )
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Game rule save failed')
      )
    }
  }
  const addGame = () =>
    setGameRows((current) => [
      ...current,
      {
        game_code: '',
        enabled: true,
        budget_pool: 'game',
        rule_fields: defaultRuleFields(GAME_RULE_DEFAULT_PATHS.slice(0, 4)),
      },
    ])
  const removeGame = (index: number) =>
    setGameRows((current) =>
      current.filter((_, rowIndex) => rowIndex !== index)
    )
  return {
    chatOpsForm,
    chatOpsRuleFields,
    setChatOpsRuleFields,
    gameRows,
    updateChatOps,
    updateGame,
    saveChatOps,
    saveGames,
    addGame,
    removeGame,
  }
}

type GroupCapabilityEditorController = ReturnType<
  typeof useGroupCapabilityEditorController
>

function GroupCapabilitySelector({
  groups,
  selectedGroup,
  onSelectedIdChange,
  t,
}: {
  groups: OpsRenderableGroup[]
  selectedGroup: OpsRenderableGroup | null
  onSelectedIdChange: (selectedId: string) => void
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-background/70 space-y-3 rounded-lg border p-4'>
      <label className='space-y-1.5 text-sm'>
        <span className='font-medium'>{t('Group')}</span>
        <select
          className={selectClassName()}
          value={selectedGroup ? String(selectedGroup.id) : ''}
          onChange={(event) => onSelectedIdChange(event.target.value)}
        >
          {groups.map((group) => (
            <option key={group.id} value={group.id}>
              {group.group_name || group.group_id} · {group.group_id}
            </option>
          ))}
        </select>
      </label>
      {selectedGroup ? (
        <div className='text-muted-foreground grid gap-3 text-sm md:grid-cols-4'>
          <div>
            {t('Platform')}:{' '}
            {platformLabel(
              selectedGroup.platform_family || selectedGroup.platform,
              t
            )}
          </div>
          <div>
            {t('Role')}: {groupRoleLabel(selectedGroup.role, t)}
          </div>
          <div>
            {t('Saved game rules')}: {gameConfigsOf(selectedGroup).length}
          </div>
          <div className='border-border/70 bg-muted/20 rounded-lg border border-dashed p-3 text-xs leading-5 md:col-span-4'>
            {t(
              'The group list and status table refresh after every successful save.'
            )}
          </div>
        </div>
      ) : null}
    </div>
  )
}

function GroupChatOpsEditor({
  controller,
  saving,
  t,
}: {
  controller: GroupCapabilityEditorController
  saving: boolean
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
      <div className='mb-3 flex flex-col gap-1 md:flex-row md:items-center md:justify-between'>
        <div>
          <div className='text-sm font-semibold'>
            {t('Check-in / verify / invite')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Set switches and reward amounts for the selected group.')}
          </div>
        </div>
        <Button size='sm' onClick={controller.saveChatOps} disabled={saving}>
          {saving ? t('Saving') : t('Save group features')}
        </Button>
      </div>
      <div className='grid gap-3 md:grid-cols-3'>
        {CHATOPS_SWITCH_FIELDS.map(([key, label]) => (
          <label key={key} className='flex items-center gap-2 text-sm'>
            <input
              type='checkbox'
              className='border-border h-4 w-4 rounded'
              checked={Boolean(controller.chatOpsForm[key])}
              onChange={(event) =>
                controller.updateChatOps(key, event.target.checked)
              }
            />
            <span>{t(label)}</span>
          </label>
        ))}
      </div>
      <div className='mt-3 grid gap-3 md:grid-cols-5'>
        {CHATOPS_REWARD_FIELDS.map(([key, label]) => (
          <label key={key} className='space-y-1.5 text-sm'>
            <span className='font-medium'>{t(label)}</span>
            <Input
              type='number'
              min={0}
              value={String(controller.chatOpsForm[key] ?? 0)}
              onChange={(event) =>
                controller.updateChatOps(key, Number(event.target.value || 0))
              }
            />
          </label>
        ))}
      </div>
      <StructuredRuleFieldsEditor
        fields={controller.chatOpsRuleFields}
        defaultPaths={CHATOPS_RULE_DEFAULT_PATHS}
        onChange={controller.setChatOpsRuleFields}
        t={t}
      />
    </div>
  )
}

function GroupGameRuleRow({
  row,
  index,
  controller,
  t,
}: {
  row: GroupCapabilityGameFormRow
  index: number
  controller: GroupCapabilityEditorController
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-muted/20 rounded-xl border p-3'>
      <div className='grid gap-3 md:grid-cols-[1fr_1fr_auto_auto] md:items-end'>
        <label className='space-y-1.5 text-sm'>
          <span className='font-medium'>{t('Game code')}</span>
          <Input
            value={row.game_code}
            onChange={(event) =>
              controller.updateGame(index, { game_code: event.target.value })
            }
            placeholder={t('Example: dice')}
          />
        </label>
        <label className='space-y-1.5 text-sm'>
          <span className='font-medium'>{t('Budget pool')}</span>
          <Input
            value={row.budget_pool}
            onChange={(event) =>
              controller.updateGame(index, { budget_pool: event.target.value })
            }
            placeholder='game'
          />
        </label>
        <label className='flex items-center gap-2 text-sm'>
          <input
            type='checkbox'
            className='border-border h-4 w-4 rounded'
            checked={row.enabled}
            onChange={(event) =>
              controller.updateGame(index, { enabled: event.target.checked })
            }
          />
          <span>{t('Enabled')}</span>
        </label>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={() => controller.removeGame(index)}
        >
          {t('Remove')}
        </Button>
      </div>
      <StructuredRuleFieldsEditor
        fields={row.rule_fields}
        defaultPaths={GAME_RULE_DEFAULT_PATHS}
        onChange={(fields) =>
          controller.updateGame(index, { rule_fields: fields })
        }
        t={t}
      />
    </div>
  )
}

function GroupGamesEditor({
  controller,
  saving,
  t,
}: {
  controller: GroupCapabilityEditorController
  saving: boolean
  t: OpsTranslate
}) {
  return (
    <div className='border-border/70 bg-background/70 rounded-lg border p-4'>
      <div className='mb-3 flex flex-col gap-1 md:flex-row md:items-center md:justify-between'>
        <div>
          <div className='text-sm font-semibold'>
            {t('Games and reward pool')}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t(
              'Add one row for each game that should be available in this group.'
            )}
          </div>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button size='sm' variant='outline' onClick={controller.addGame}>
            {t('Add game rule')}
          </Button>
          <Button size='sm' onClick={controller.saveGames} disabled={saving}>
            {saving ? t('Saving') : t('Save game rules')}
          </Button>
        </div>
      </div>
      <div className='space-y-3'>
        {controller.gameRows.length ? (
          controller.gameRows.map((row, index) => (
            <GroupGameRuleRow
              key={`${index}-${row.game_code}`}
              row={row}
              index={index}
              controller={controller}
              t={t}
            />
          ))
        ) : (
          <div className='border-border/70 bg-muted/20 text-muted-foreground rounded-xl border border-dashed p-4 text-sm'>
            {t(
              'No game rows are configured for this group yet. Add a game rule to make the configuration explicit.'
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function GroupCapabilityEditorPanelContent({
  groups,
  groupActions,
  t,
  selectedGroup,
  onSelectedIdChange,
}: GroupCapabilityEditorPanelProps & {
  selectedGroup: OpsRenderableGroup | null
  onSelectedIdChange: (selectedId: string) => void
}) {
  const controller = useGroupCapabilityEditorController(
    selectedGroup,
    groupActions,
    t
  )
  if (!groups.length) return null

  return (
    <OpsPanel
      title={t('Edit group features')}
      description={t(
        'Choose a group, then edit its check-in, verification, invitation, reward, and game settings.'
      )}
    >
      <div className='space-y-4'>
        <GroupCapabilitySelector
          groups={groups}
          selectedGroup={selectedGroup}
          onSelectedIdChange={onSelectedIdChange}
          t={t}
        />
        <GroupChatOpsEditor
          controller={controller}
          saving={groupActions.saving}
          t={t}
        />
        <GroupGamesEditor
          controller={controller}
          saving={groupActions.saving}
          t={t}
        />
      </div>
    </OpsPanel>
  )
}
