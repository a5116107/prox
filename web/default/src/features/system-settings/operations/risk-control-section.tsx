/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertCircleIcon,
  ChevronRightIcon,
  KeyRoundIcon,
  RefreshCcwIcon,
  SearchIcon,
  ShieldAlertIcon,
  UserRoundCheckIcon,
  UsersRoundIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { RiskControlSection as RiskControlSettingsEditor } from '../security/risk-control-section'
import { useOpsT, type OpsTranslate } from './ops-i18n'
import { OpsStatusBadge } from './ops-shared'
import { RiskControlDetailSheet } from './risk-control-detail-sheet'
import {
  pageSizeOptions,
  riskTypeOptions,
  severityOptions,
  statusOptions,
  type RiskActionKind,
  type RiskActionPreview,
  type RiskActionResult,
  type RiskAudit,
  type RiskFilters,
  type RiskListResult,
} from './risk-control-types'
import {
  compactRiskEvidence,
  defaultRiskActionReason,
  formatRiskTime,
  parseRiskIdList,
  riskErrorMessage,
  riskTypeLabel,
  severityTone,
  statusTone,
} from './risk-control-utils'

const DEFAULT_FILTERS: RiskFilters = {
  riskType: 'all',
  severity: 'all',
  status: 'active',
  keyword: '',
}

type Props = {
  defaultValues: Record<string, boolean | number | string>
}

function useDebouncedValue(value: string, delay: number) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(timer)
  }, [delay, value])
  return debounced
}

function RiskMetric(props: {
  icon: typeof ShieldAlertIcon
  label: string
  value: number | string
  tone: 'danger' | 'warning' | 'info' | 'success'
}) {
  const Icon = props.icon
  const toneClass = {
    danger: 'text-rose-600 bg-rose-500/10 dark:text-rose-300',
    warning: 'text-amber-700 bg-amber-500/10 dark:text-amber-300',
    info: 'text-cyan-700 bg-cyan-500/10 dark:text-cyan-300',
    success: 'text-emerald-700 bg-emerald-500/10 dark:text-emerald-300',
  }[props.tone]
  return (
    <div className='bg-background flex min-h-24 items-center gap-3 rounded-lg border px-4 py-3'>
      <span
        className={`flex size-9 shrink-0 items-center justify-center rounded-md ${toneClass}`}
      >
        <Icon className='size-4' aria-hidden='true' />
      </span>
      <div className='min-w-0'>
        <div className='text-muted-foreground truncate text-xs'>
          {props.label}
        </div>
        <div className='mt-1 text-xl font-semibold'>{props.value}</div>
      </div>
    </div>
  )
}

function RiskFilterBar(props: {
  filters: RiskFilters
  loading: boolean
  t: OpsTranslate
  onChange: (filters: RiskFilters) => void
  onRefresh: () => void
}) {
  const update = (patch: Partial<RiskFilters>) => {
    props.onChange({ ...props.filters, ...patch })
  }
  return (
    <div className='bg-muted/20 grid gap-2 border-b p-3 lg:grid-cols-[minmax(180px,1fr)_150px_150px_minmax(240px,1.4fr)_auto]'>
      <Select
        value={props.filters.riskType}
        onValueChange={(value) => update({ riskType: value ?? 'all' })}
      >
        <SelectTrigger aria-label={props.t('Risk type')}>
          <SelectValue placeholder={props.t('Risk type')} />
        </SelectTrigger>
        <SelectContent>
          {riskTypeOptions.map((item) => (
            <SelectItem key={item} value={item}>
              {item === 'all'
                ? props.t('All risk types')
                : riskTypeLabel(item, props.t)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={props.filters.severity}
        onValueChange={(value) => update({ severity: value ?? 'all' })}
      >
        <SelectTrigger aria-label={props.t('Severity level')}>
          <SelectValue placeholder={props.t('Severity level')} />
        </SelectTrigger>
        <SelectContent>
          {severityOptions.map((item) => (
            <SelectItem key={item} value={item}>
              {item === 'all' ? props.t('All severity levels') : props.t(item)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={props.filters.status}
        onValueChange={(value) => update({ status: value ?? 'all' })}
      >
        <SelectTrigger aria-label={props.t('Status')}>
          <SelectValue placeholder={props.t('Status')} />
        </SelectTrigger>
        <SelectContent>
          {statusOptions.map((item) => (
            <SelectItem key={item} value={item}>
              {item === 'all'
                ? props.t('All statuses')
                : item === 'active'
                  ? props.t('Active audits')
                  : props.t(item)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <div className='relative'>
        <SearchIcon
          className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2'
          aria-hidden='true'
        />
        <Input
          value={props.filters.keyword}
          onChange={(event) => update({ keyword: event.target.value })}
          className='pl-8'
          aria-label={props.t('Search risk audits')}
          placeholder={props.t('IP / username / User ID / Token ID')}
        />
      </div>
      <Button
        type='button'
        variant='outline'
        onClick={props.onRefresh}
        disabled={props.loading}
      >
        <RefreshCcwIcon
          data-icon='inline-start'
          className={props.loading ? 'animate-spin' : undefined}
          aria-hidden='true'
        />
        {props.t('Refresh')}
      </Button>
    </div>
  )
}

function LoadingRows() {
  return Array.from({ length: 6 }, (_, index) => (
    <TableRow key={index}>
      {Array.from({ length: 7 }, (__, cell) => (
        <TableCell key={cell}>
          <div className='bg-muted h-4 animate-pulse rounded-sm' />
        </TableCell>
      ))}
    </TableRow>
  ))
}

function useRiskAuditQueue(t: OpsTranslate) {
  const [filters, setFilters] = useState<RiskFilters>(DEFAULT_FILTERS)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [data, setData] = useState<RiskListResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const requestSequence = useRef(0)
  const debouncedKeyword = useDebouncedValue(filters.keyword, 350)
  const params = useMemo(
    () => ({
      p: page,
      page_size: pageSize,
      risk_type: filters.riskType === 'all' ? '' : filters.riskType,
      severity: filters.severity === 'all' ? '' : filters.severity,
      status: filters.status === 'all' ? '' : filters.status,
      keyword: debouncedKeyword.trim(),
    }),
    [debouncedKeyword, filters, page, pageSize]
  )
  const load = useCallback(async () => {
    const sequence = ++requestSequence.current
    setLoading(true)
    setLoadError('')
    try {
      const response = await api.get('/api/risk-control/audits', {
        params,
        disableDuplicate: true,
      })
      if (!response.data?.success) {
        throw new Error(
          response.data?.message || t('Failed to load risk audits')
        )
      }
      if (sequence !== requestSequence.current) return
      const result = response.data.data as RiskListResult
      const maxPage = Math.max(1, Math.ceil(result.total / pageSize))
      if (page > maxPage) setPage(maxPage)
      else setData(result)
    } catch (error) {
      if (sequence === requestSequence.current) {
        setLoadError(riskErrorMessage(error, t('Failed to load risk audits')))
      }
    } finally {
      if (sequence === requestSequence.current) setLoading(false)
    }
  }, [page, pageSize, params, t])
  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(timer)
  }, [load])
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / pageSize))
  return {
    filters,
    page,
    pageSize,
    data,
    loading,
    loadError,
    totalPages,
    firstItem: data?.total ? (page - 1) * pageSize + 1 : 0,
    lastItem: data?.total ? Math.min(page * pageSize, data.total) : 0,
    load,
    setPage,
    setPageSize,
    changeFilters: (next: RiskFilters) => {
      setFilters(next)
      setPage(1)
    },
  }
}

async function requestRiskKeyAction(
  auditId: number,
  kind: RiskActionKind,
  reason: string,
  dryRun: boolean,
  t: OpsTranslate
) {
  const endpoint = kind === 'disable' ? 'disable-keys' : 'restore-keys'
  const response = await api.post(
    `/api/risk-control/audits/${auditId}/${endpoint}`,
    { reason: reason.trim(), dry_run: dryRun }
  )
  if (!response.data?.success) {
    throw new Error(response.data?.message || t('Risk action failed'))
  }
  return response.data.data as RiskActionResult
}

async function requestRiskStatus(
  auditId: number,
  status: string,
  reason: string,
  t: OpsTranslate
) {
  const response = await api.post(
    `/api/risk-control/audits/${auditId}/status`,
    { status, reason: reason.trim() }
  )
  if (!response.data?.success) {
    throw new Error(
      response.data?.message || t('Failed to update audit status')
    )
  }
  return response.data.data as RiskAudit
}

function showRiskActionResult(
  kind: RiskActionKind,
  result: RiskActionResult,
  dryRun: boolean,
  t: OpsTranslate
) {
  if (dryRun) {
    if (kind === 'restore' && (result.matched_user_controls ?? 0) > 0) {
      toast.info(
        t(
          'Preview matched {{count}} key(s) and {{users}} account restriction(s)',
          {
            count: result.matched_tokens ?? 0,
            users: result.matched_user_controls ?? 0,
          }
        )
      )
      return
    }
    toast.info(
      t('Preview matched {{count}} key(s)', {
        count: result.matched_tokens ?? 0,
      })
    )
    return
  }
  const count =
    kind === 'disable'
      ? (result.disabled_tokens ?? 0)
      : (result.restored_tokens ?? 0)
  if (kind === 'restore' && (result.released_user_controls ?? 0) > 0) {
    toast.success(
      t(
        'Restored {{count}} key(s) and released {{users}} account restriction(s)',
        {
          count,
          users: result.released_user_controls ?? 0,
        }
      )
    )
    return
  }
  toast.success(
    kind === 'disable'
      ? t('Disabled {{count}} key(s)', { count })
      : t('Restored {{count}} key(s)', { count })
  )
}

function useRiskAuditActions(t: OpsTranslate, reload: () => Promise<void>) {
  const [selectedAudit, setSelectedAudit] = useState<RiskAudit | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const [actionReason, setActionReason] = useState('')
  const [actionBusy, setActionBusy] = useState(false)
  const [preview, setPreview] = useState<RiskActionPreview | null>(null)
  const openAudit = (audit: RiskAudit) => {
    setSelectedAudit(audit)
    setActionReason(defaultRiskActionReason(audit, t))
    setPreview(null)
    setDetailOpen(true)
  }
  const closeDetail = (open: boolean) => {
    setDetailOpen(open)
    if (!open && !actionBusy) {
      setPreview(null)
      setSelectedAudit(null)
      setActionReason('')
    }
  }
  const runKeyAction = async (kind: RiskActionKind, dryRun: boolean) => {
    if (!selectedAudit) return
    setActionBusy(true)
    try {
      const result = await requestRiskKeyAction(
        selectedAudit.id,
        kind,
        actionReason,
        dryRun,
        t
      )
      showRiskActionResult(kind, result, dryRun, t)
      if (dryRun) {
        setPreview({ kind, result })
      } else {
        setPreview(null)
        setDetailOpen(false)
        setSelectedAudit(null)
        await reload()
      }
    } catch (error) {
      toast.error(riskErrorMessage(error, t('Risk action failed')))
    } finally {
      setActionBusy(false)
    }
  }
  const updateStatus = async (status: string) => {
    if (!selectedAudit) return
    setActionBusy(true)
    try {
      const updated = await requestRiskStatus(
        selectedAudit.id,
        status,
        actionReason,
        t
      )
      toast.success(t('Marked as {{status}}', { status: t(status) }))
      setSelectedAudit(updated)
      setPreview(null)
      await reload()
    } catch (error) {
      toast.error(riskErrorMessage(error, t('Failed to update audit status')))
    } finally {
      setActionBusy(false)
    }
  }
  return {
    selectedAudit,
    detailOpen,
    actionReason,
    actionBusy,
    preview,
    openAudit,
    closeDetail,
    runKeyAction,
    updateStatus,
    changeReason: (reason: string) => {
      setActionReason(reason)
      setPreview(null)
    },
  }
}

function RiskQueueSummary({
  summary,
  t,
}: {
  summary?: RiskListResult['summary']
  t: OpsTranslate
}) {
  return (
    <>
      <header className='flex flex-col gap-3 border-b pb-4 lg:flex-row lg:items-start lg:justify-between'>
        <div className='min-w-0'>
          <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium uppercase'>
            <ShieldAlertIcon className='size-4' aria-hidden='true' />
            {t('Account Risk Control')}
          </div>
          <h2 className='mt-1 text-xl font-semibold'>
            {t('Account and key review')}
          </h2>
          <p className='text-muted-foreground mt-1 max-w-3xl text-sm leading-5'>
            {t(
              'Community qualification, account linkage, invite relationships, IP changes, and key distribution are evaluated in one site-scoped queue.'
            )}
          </p>
        </div>
        <div className='flex flex-wrap gap-2'>
          <OpsStatusBadge tone='danger'>
            {t('No paid exceptions')}
          </OpsStatusBadge>
          <OpsStatusBadge tone='info'>{t('Only affected keys')}</OpsStatusBadge>
          <OpsStatusBadge tone='success'>
            {t('Admin accounts ignored')}
          </OpsStatusBadge>
        </div>
      </header>
      <section className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <RiskMetric
          icon={ShieldAlertIcon}
          label={t('Open risks')}
          value={summary?.total_open ?? '-'}
          tone='warning'
        />
        <RiskMetric
          icon={AlertCircleIcon}
          label={t('High risk')}
          value={summary?.high_risk_open ?? '-'}
          tone='danger'
        />
        <RiskMetric
          icon={KeyRoundIcon}
          label={t('Keys needing action')}
          value={summary?.key_risk_open ?? '-'}
          tone='info'
        />
        <RiskMetric
          icon={UserRoundCheckIcon}
          label={t('Admin false positives')}
          value={summary?.admin_candidates ?? '-'}
          tone='success'
        />
      </section>
      <div className='border-l-2 border-amber-500 bg-amber-500/5 px-4 py-3 text-sm'>
        <span className='font-medium'>{t('Current rule')}:</span>{' '}
        <span className='text-muted-foreground'>
          {t(
            'Accounts without matching community or group qualification cannot keep active keys. Restore actions are limited to keys disabled by the same audit.'
          )}
        </span>
      </div>
    </>
  )
}

function RiskAuditRow({
  audit,
  loading,
  t,
  onOpen,
}: {
  audit: RiskAudit
  loading: boolean
  t: OpsTranslate
  onOpen: (audit: RiskAudit) => void
}) {
  const userCount = parseRiskIdList(audit.user_ids).length
  const tokenCount = parseRiskIdList(audit.token_ids).length
  return (
    <TableRow className={loading ? 'opacity-60' : undefined}>
      <TableCell className='align-top'>
        <div className='font-medium'>{riskTypeLabel(audit.risk_type, t)}</div>
        <div className='mt-2 flex flex-wrap gap-1.5'>
          <OpsStatusBadge tone={severityTone(audit.severity)}>
            {t(audit.severity || 'medium')}
          </OpsStatusBadge>
          <OpsStatusBadge tone={statusTone(audit.status)}>
            {t(audit.status)}
          </OpsStatusBadge>
        </div>
      </TableCell>
      <TableCell className='align-top'>
        <div className='line-clamp-2 font-medium' title={audit.subject}>
          {audit.subject || '-'}
        </div>
        <div className='text-muted-foreground mt-1 font-mono text-xs'>
          #{audit.id}
        </div>
      </TableCell>
      <TableCell className='align-top text-xs'>
        <div>{t('{{count}} user(s)', { count: userCount })}</div>
        <div className='text-muted-foreground mt-1'>
          {t('{{count}} key(s)', { count: tokenCount })}
        </div>
      </TableCell>
      <TableCell className='align-top font-mono text-xs'>
        {audit.ip || '-'}
      </TableCell>
      <TableCell className='align-top'>
        <p
          className='text-muted-foreground line-clamp-3 text-xs leading-5'
          title={audit.evidence}
        >
          {compactRiskEvidence(audit.evidence, t)}
        </p>
      </TableCell>
      <TableCell className='text-muted-foreground align-top text-xs'>
        {formatRiskTime(audit.updated_at)}
      </TableCell>
      <TableCell className='text-right align-top'>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={() => onOpen(audit)}
        >
          {t('Review')}
          <ChevronRightIcon data-icon='inline-end' aria-hidden='true' />
        </Button>
      </TableCell>
    </TableRow>
  )
}

function RiskAuditTable({
  data,
  loading,
  t,
  onOpen,
}: {
  data: RiskListResult | null
  loading: boolean
  t: OpsTranslate
  onOpen: (audit: RiskAudit) => void
}) {
  return (
    <div className='overflow-x-auto'>
      <Table className='min-w-[1120px] table-fixed'>
        <TableHeader>
          <TableRow>
            <TableHead className='w-[190px]'>{t('Risk')}</TableHead>
            <TableHead className='w-[250px]'>{t('Subject')}</TableHead>
            <TableHead className='w-[170px]'>{t('Users / keys')}</TableHead>
            <TableHead className='w-[170px]'>{t('IP')}</TableHead>
            <TableHead>{t('Why flagged')}</TableHead>
            <TableHead className='w-[150px]'>{t('Updated')}</TableHead>
            <TableHead className='w-[110px] text-right'>
              {t('Action')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && !data ? <LoadingRows /> : null}
          {!loading && (data?.items.length ?? 0) === 0 ? (
            <TableRow>
              <TableCell colSpan={7} className='h-44 text-center'>
                <UsersRoundIcon
                  className='text-muted-foreground mx-auto size-6'
                  aria-hidden='true'
                />
                <p className='mt-2 font-medium'>
                  {t('No matching risk audits')}
                </p>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Change filters or refresh the current site queue.')}
                </p>
              </TableCell>
            </TableRow>
          ) : null}
          {(data?.items ?? []).map((audit) => (
            <RiskAuditRow
              key={audit.id}
              audit={audit}
              loading={loading}
              t={t}
              onOpen={onOpen}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function RiskQueuePagination({
  queue,
  t,
}: {
  queue: ReturnType<typeof useRiskAuditQueue>
  t: OpsTranslate
}) {
  return (
    <footer className='bg-muted/15 flex flex-col gap-3 border-t px-3 py-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='text-muted-foreground text-xs'>
        {t('Showing {{first}}-{{last}} of {{total}}', {
          first: queue.firstItem,
          last: queue.lastItem,
          total: queue.data?.total ?? 0,
        })}
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <Select
          value={String(queue.pageSize)}
          onValueChange={(value) => {
            queue.setPageSize(Number(value ?? 20))
            queue.setPage(1)
          }}
        >
          <SelectTrigger className='w-28' aria-label={t('Rows per page')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {pageSizeOptions.map((size) => (
              <SelectItem key={size} value={String(size)}>
                {t('{{count}} rows', { count: size })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className='min-w-20 text-center text-xs font-medium'>
          {t('{{page}} / {{pages}}', {
            page: queue.page,
            pages: queue.totalPages,
          })}
        </span>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={queue.page <= 1 || queue.loading}
          onClick={() => queue.setPage((current) => Math.max(1, current - 1))}
        >
          {t('Previous')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={queue.page >= queue.totalPages || queue.loading}
          onClick={() =>
            queue.setPage((current) => Math.min(queue.totalPages, current + 1))
          }
        >
          {t('Next')}
        </Button>
      </div>
    </footer>
  )
}

function RiskQueuePanel({
  queue,
  actions,
  t,
}: {
  queue: ReturnType<typeof useRiskAuditQueue>
  actions: ReturnType<typeof useRiskAuditActions>
  t: OpsTranslate
}) {
  return (
    <section className='bg-background overflow-hidden rounded-lg border'>
      <RiskFilterBar
        filters={queue.filters}
        loading={queue.loading}
        t={t}
        onChange={queue.changeFilters}
        onRefresh={() => void queue.load()}
      />
      {queue.loadError ? (
        <div className='flex min-h-48 flex-col items-center justify-center gap-3 p-6 text-center'>
          <AlertCircleIcon
            className='text-destructive size-6'
            aria-hidden='true'
          />
          <div>
            <p className='font-medium'>{t('Risk audits unavailable')}</p>
            <p className='text-muted-foreground mt-1 text-sm'>
              {queue.loadError}
            </p>
          </div>
          <Button
            type='button'
            variant='outline'
            onClick={() => void queue.load()}
          >
            <RefreshCcwIcon data-icon='inline-start' aria-hidden='true' />
            {t('Retry')}
          </Button>
        </div>
      ) : (
        <RiskAuditTable
          data={queue.data}
          loading={queue.loading}
          t={t}
          onOpen={actions.openAudit}
        />
      )}
      <RiskQueuePagination queue={queue} t={t} />
    </section>
  )
}

export function RiskControlSection({ defaultValues }: Props) {
  const t = useOpsT()
  const queue = useRiskAuditQueue(t)
  const actions = useRiskAuditActions(t, queue.load)
  return (
    <div className='space-y-4'>
      <section id='risk-settings-editor' className='scroll-mt-24'>
        <RiskControlSettingsEditor defaultValues={defaultValues as never} />
      </section>
      <RiskQueueSummary summary={queue.data?.summary} t={t} />
      <RiskQueuePanel queue={queue} actions={actions} t={t} />
      <RiskControlDetailSheet
        open={actions.detailOpen}
        audit={actions.selectedAudit}
        reason={actions.actionReason}
        busy={actions.actionBusy}
        preview={actions.preview}
        t={t}
        onOpenChange={actions.closeDetail}
        onReasonChange={actions.changeReason}
        onPreview={(kind) => actions.runKeyAction(kind, true)}
        onCommit={(kind) => actions.runKeyAction(kind, false)}
        onStatusChange={actions.updateStatus}
      />
    </div>
  )
}
