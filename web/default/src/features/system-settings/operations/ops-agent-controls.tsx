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
import {
  Bot,
  CircleGauge,
  Gamepad2,
  RefreshCw,
  Settings2,
  UsersRound,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { GroupCapabilityEditorPanel } from './ops-group-capabilities'
import { GroupRegistryActionsPanel } from './ops-group-registry'
import { type OpsTranslate } from './ops-i18n'
import {
  AGENT_OPS_UI_REVISION,
  type SavedOpsContext,
  type ViewMode,
  countEnabledGameRules,
  platformLabel,
  resolveCapabilityRows,
} from './ops-live-foundation'
import { releaseModeBadge, scrollToOpsAnchor } from './ops-live-truth'
import { OpsStatusBadge } from './ops-shared'
import {
  type OpsGroupCapabilityMatrixOverview,
  type OpsGroupActions,
  type OpsRegistryGroup,
  type OpsReleaseOverview,
} from './use-ops-registry'

type AgentOperationsHeaderProps = {
  saved: SavedOpsContext
  effectiveSiteId: string
  releases: OpsReleaseOverview | null
  recentAuditCount: number
  mode: ViewMode
  loading: boolean
  onSetMode: (mode: ViewMode) => void
  onRefresh: () => void
  onEditGroups: () => void
  onEditCapabilities: () => void
  t: OpsTranslate
}

export function AgentOperationsHeader({
  saved,
  effectiveSiteId,
  releases,
  recentAuditCount,
  mode,
  loading,
  onSetMode,
  onRefresh,
  onEditGroups,
  onEditCapabilities,
  t,
}: AgentOperationsHeaderProps) {
  return (
    <section
      data-ui-revision={AGENT_OPS_UI_REVISION}
      className='bg-card overflow-hidden rounded-lg border shadow-sm'
    >
      <div className='grid gap-4 px-5 py-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='bg-muted/40 text-foreground flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border'>
            <Bot className='h-5 w-5' aria-hidden='true' />
          </div>
          <div className='min-w-0 space-y-1.5'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('QQ / TG group service')}
            </div>
            <h2 className='text-xl font-semibold'>
              {t('Current bot service status')}
            </h2>
            <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
              {t(
                'Check connected groups, available group features, reward balances, and recent handling results.'
              )}
            </p>
          </div>
        </div>

        <div className='flex flex-wrap items-center gap-2 lg:justify-end'>
          <div
            className='bg-muted/30 inline-flex rounded-lg border p-1'
            aria-label={t('Page view')}
          >
            <Button
              type='button'
              size='sm'
              variant={mode === 'overview' ? 'secondary' : 'ghost'}
              aria-pressed={mode === 'overview'}
              onClick={() => onSetMode('overview')}
            >
              <CircleGauge className='h-4 w-4' aria-hidden='true' />
              {t('Status overview')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant={mode === 'editor' ? 'secondary' : 'ghost'}
              aria-pressed={mode === 'editor'}
              onClick={() => onSetMode('editor')}
            >
              <Settings2 className='h-4 w-4' aria-hidden='true' />
              {t('Edit settings')}
            </Button>
          </div>
          <Button
            type='button'
            size='sm'
            variant='outline'
            onClick={onRefresh}
            disabled={loading}
            title={t('Refresh current data')}
          >
            <RefreshCw
              className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`}
              aria-hidden='true'
            />
            {loading ? t('Refreshing') : t('Refresh')}
          </Button>
        </div>
      </div>

      <div className='bg-muted/15 flex flex-col gap-3 border-t px-5 py-3 xl:flex-row xl:items-center xl:justify-between'>
        <div className='flex flex-wrap items-center gap-2'>
          <Badge variant='outline'>
            {t('Site ID')}: {effectiveSiteId || t('Unknown')}
          </Badge>
          {saved.siteName ? (
            <Badge variant='outline'>
              {t('Site name')}: {saved.siteName}
            </Badge>
          ) : null}
          <Badge variant='outline'>
            {t('Main channel')}:{' '}
            {saved.primaryPlatform
              ? platformLabel(saved.primaryPlatform, t)
              : t('Not configured')}
          </Badge>
          {releaseModeBadge(releases?.release_mode, t)}
          <OpsStatusBadge tone='info'>
            {t('Recent changes')}: {recentAuditCount}
          </OpsStatusBadge>
        </div>

        <div className='flex flex-wrap items-center gap-2'>
          <Button type='button' size='sm' onClick={onEditGroups}>
            <UsersRound className='h-4 w-4' aria-hidden='true' />
            {t('Manage groups and rooms')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='secondary'
            onClick={onEditCapabilities}
          >
            <Settings2 className='h-4 w-4' aria-hidden='true' />
            {t('Set group features and rewards')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='outline'
            render={<a href='/game-admin' />}
          >
            <Gamepad2 className='h-4 w-4' aria-hidden='true' />
            {t('Open game settings')}
          </Button>
        </div>
      </div>
    </section>
  )
}

export function AgentConfigurationEntryPoints({
  groupCount,
  capabilityRowCount,
  enabledGameRules,
  onEditGroups,
  onEditCapabilities,
  t,
}: {
  groupCount: number
  capabilityRowCount: number
  enabledGameRules: number
  onEditGroups: () => void
  onEditCapabilities: () => void
  t: OpsTranslate
}) {
  const entries = [
    {
      key: 'groups',
      icon: UsersRound,
      title: t('Groups and rooms'),
      description: t(
        'Add, edit, copy, or import QQ groups, TG groups, and community rooms.'
      ),
      meta: t('{{count}} groups / rooms', { count: groupCount }),
      button: t('Manage groups'),
      onClick: onEditGroups,
    },
    {
      key: 'capabilities',
      icon: Settings2,
      title: t('Group features and rewards'),
      description: t(
        'Set check-in, verification, invitations, reward amounts, and games for each group.'
      ),
      meta: t('{{count}} groups configured · {{games}} game rules enabled', {
        count: capabilityRowCount,
        games: enabledGameRules,
      }),
      button: t('Edit group features'),
      onClick: onEditCapabilities,
    },
    {
      key: 'game-admin',
      icon: Gamepad2,
      title: t('Game settings'),
      description: t(
        'Manage detailed game rules and handle game-specific administration.'
      ),
      meta: t('Separate game settings page'),
      button: t('Open game settings'),
      href: '/game-admin',
    },
  ]

  return (
    <section className='space-y-3'>
      <div>
        <h3 className='text-base font-semibold'>{t('Common actions')}</h3>
        <p className='text-muted-foreground mt-1 text-sm leading-6'>
          {t(
            'Choose what you want to change. Each save updates the current site directly.'
          )}
        </p>
      </div>
      <div className='bg-card grid overflow-hidden rounded-lg border lg:grid-cols-3'>
        {entries.map((entry) => (
          <div
            key={entry.key}
            className='flex min-h-[154px] flex-col justify-between gap-4 border-b p-4 last:border-b-0 lg:border-r lg:border-b-0 lg:last:border-r-0'
          >
            <div className='space-y-3'>
              <div className='flex items-center justify-between gap-3'>
                <div className='bg-muted text-foreground flex h-8 w-8 items-center justify-center rounded-lg'>
                  <entry.icon className='h-4 w-4' aria-hidden='true' />
                </div>
                <span className='text-muted-foreground text-xs'>
                  {entry.meta}
                </span>
              </div>
              <div className='space-y-1'>
                <div className='text-sm font-semibold'>{entry.title}</div>
                <p className='text-muted-foreground text-sm leading-6'>
                  {entry.description}
                </p>
              </div>
            </div>
            <div>
              {entry.href ? (
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  render={<a href={entry.href} />}
                >
                  {entry.button}
                </Button>
              ) : (
                <Button type='button' size='sm' onClick={entry.onClick}>
                  {entry.button}
                </Button>
              )}
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

export function AgentLiveEditingPanels({
  siteId,
  groups,
  groupCapabilities,
  saved,
  groupActions,
  t,
}: {
  siteId: string
  groups: OpsRegistryGroup[]
  groupCapabilities: OpsGroupCapabilityMatrixOverview | null
  saved: SavedOpsContext
  groupActions: OpsGroupActions
  t: OpsTranslate
}) {
  const capabilityRows = resolveCapabilityRows(groupCapabilities, groups)
  const enabledGameRules = countEnabledGameRules(groupCapabilities, groups)

  return (
    <div className='space-y-6' data-ui-revision={AGENT_OPS_UI_REVISION}>
      <AgentConfigurationEntryPoints
        groupCount={groups.length}
        capabilityRowCount={capabilityRows.length}
        enabledGameRules={enabledGameRules}
        onEditGroups={() => scrollToOpsAnchor('ops-group-registry-editor')}
        onEditCapabilities={() =>
          scrollToOpsAnchor('ops-group-capability-editor')
        }
        t={t}
      />

      <div id='ops-group-registry-editor' className='scroll-mt-28'>
        <GroupRegistryActionsPanel
          siteId={siteId}
          groups={groups}
          saved={saved}
          groupActions={groupActions}
          defaultPlatform={saved.primaryPlatform || 'qq'}
          title={t('Add or edit a QQ / TG group')}
          description={t(
            'Register, edit, bulk import, or clone a primary or secondary chat group, then reuse its capability template so check-in, verify, invite, rewards, and game settings do not drift.'
          )}
          t={t}
        />
      </div>

      <div id='ops-group-capability-editor' className='scroll-mt-28'>
        <GroupCapabilityEditorPanel
          groups={capabilityRows}
          groupActions={groupActions}
          t={t}
        />
      </div>
    </div>
  )
}
