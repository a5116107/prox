/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useState } from 'react'
import {
  AlertTriangleIcon,
  BanIcon,
  CheckCircle2Icon,
  Clock3Icon,
  KeyRoundIcon,
  NetworkIcon,
  RotateCcwIcon,
  ShieldCheckIcon,
  UsersIcon,
} from 'lucide-react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import type { OpsTranslate } from './ops-i18n'
import { OpsStatusBadge } from './ops-shared'
import type {
  RiskActionKind,
  RiskActionPreview,
  RiskAudit,
} from './risk-control-types'
import {
  allowedRiskStatuses,
  formatRiskTime,
  parseRiskEvidence,
  parseRiskIdList,
  riskTypeLabel,
  severityTone,
  statusTone,
  tokenStatusLabel,
  tokenStatusTone,
} from './risk-control-utils'

type RiskControlDetailSheetProps = {
  open: boolean
  audit: RiskAudit | null
  reason: string
  busy: boolean
  preview: RiskActionPreview | null
  t: OpsTranslate
  onOpenChange: (open: boolean) => void
  onReasonChange: (reason: string) => void
  onPreview: (kind: RiskActionKind) => Promise<void>
  onCommit: (kind: RiskActionKind) => Promise<void>
  onStatusChange: (status: string) => Promise<void>
}

function riskPreviewHasScope(preview: RiskActionPreview | null) {
  return Boolean(
    preview &&
    (preview.result.matched_tokens > 0 ||
      (preview.kind === 'restore' &&
        (preview.result.matched_user_controls ?? 0) > 0))
  )
}

function FactBlock(props: {
  icon: typeof UsersIcon
  label: string
  value: string
}) {
  const Icon = props.icon
  return (
    <div className='min-w-0 border-b py-3 last:border-b-0'>
      <div className='text-muted-foreground flex items-center gap-2 text-xs'>
        <Icon className='size-3.5' aria-hidden='true' />
        {props.label}
      </div>
      <div className='mt-1 text-sm font-medium break-words'>{props.value}</div>
    </div>
  )
}

function PreviewTable(props: { preview: RiskActionPreview; t: OpsTranslate }) {
  const tokens = props.preview.result.tokens ?? []
  const matchedUserControls = props.preview.result.matched_user_controls ?? 0
  return (
    <section className='border-t pt-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div>
          <h3 className='text-sm font-semibold'>
            {matchedUserControls > 0
              ? props.t('Restore preview')
              : props.t('Exact key preview')}
          </h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {matchedUserControls > 0
              ? props.t(
                  '{{count}} key(s) and {{users}} account restriction(s) returned by the server',
                  {
                    count: props.preview.result.matched_tokens,
                    users: matchedUserControls,
                  }
                )
              : props.t('{{count}} key(s) returned by the server', {
                  count: props.preview.result.matched_tokens,
                })}
          </p>
        </div>
        <OpsStatusBadge
          tone={props.preview.kind === 'disable' ? 'danger' : 'success'}
        >
          {props.preview.kind === 'disable'
            ? props.t('Disable preview')
            : props.t('Restore preview')}
        </OpsStatusBadge>
      </div>
      <div className='mt-3 overflow-x-auto border'>
        <table className='w-full min-w-[520px] text-left text-xs'>
          <thead className='bg-muted/50 text-muted-foreground'>
            <tr>
              <th className='px-3 py-2 font-medium'>ID</th>
              <th className='px-3 py-2 font-medium'>{props.t('Key name')}</th>
              <th className='px-3 py-2 font-medium'>{props.t('User')}</th>
              <th className='px-3 py-2 font-medium'>{props.t('Group')}</th>
              <th className='px-3 py-2 font-medium'>{props.t('Status')}</th>
            </tr>
          </thead>
          <tbody>
            {tokens.map((token) => (
              <tr key={token.id} className='border-t'>
                <td className='px-3 py-2 font-mono'>{token.id}</td>
                <td className='max-w-48 truncate px-3 py-2' title={token.name}>
                  {token.name || '-'}
                </td>
                <td className='px-3 py-2 font-mono'>{token.user_id}</td>
                <td className='px-3 py-2'>{token.group || '-'}</td>
                <td className='px-3 py-2'>
                  <OpsStatusBadge tone={tokenStatusTone(token.status)}>
                    {tokenStatusLabel(token.status, props.t)}
                  </OpsStatusBadge>
                </td>
              </tr>
            ))}
            {tokens.length === 0 ? (
              <tr className='border-t'>
                <td
                  colSpan={5}
                  className='text-muted-foreground px-3 py-5 text-center'
                >
                  {matchedUserControls > 0
                    ? props.t(
                        'No keys matched; the account restriction will still be released.'
                      )
                    : props.t('No keys matched this action')}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </section>
  )
}

function RiskActionControls({
  audit,
  props,
}: {
  audit: RiskAudit
  props: RiskControlDetailSheetProps
}) {
  const reasonReady = props.reason.trim().length >= 8
  const canDisableKeys = ['open', 'reviewing'].includes(audit.status)
  const canRestoreKeys = ['open', 'reviewing'].includes(audit.status)
  return (
    <>
      <section className='mt-4 border-t pt-4'>
        <label
          htmlFor={`risk-action-reason-${audit.id}`}
          className='text-sm font-semibold'
        >
          {props.t('Action reason')}
        </label>
        <Textarea
          id={`risk-action-reason-${audit.id}`}
          value={props.reason}
          onChange={(event) => props.onReasonChange(event.target.value)}
          rows={3}
          className='mt-2'
          placeholder={props.t(
            'Record the evidence and scope for this operation'
          )}
        />
        {!reasonReady ? (
          <p className='text-destructive mt-1.5 text-xs'>
            {props.t('Enter at least 8 characters before changing keys')}
          </p>
        ) : null}
        <div className='mt-3 grid gap-2 sm:grid-cols-2'>
          <Button
            type='button'
            variant='outline'
            disabled={props.busy || !reasonReady || !canDisableKeys}
            onClick={() => void props.onPreview('disable')}
          >
            <BanIcon data-icon='inline-start' aria-hidden='true' />
            {props.t('Preview disable')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={props.busy || !reasonReady || !canRestoreKeys}
            onClick={() => void props.onPreview('restore')}
          >
            <RotateCcwIcon data-icon='inline-start' aria-hidden='true' />
            {props.t('Preview restore')}
          </Button>
        </div>
        <p className='text-muted-foreground mt-2 text-xs leading-5'>
          {props.t(
            'Restore is limited to keys and account restrictions owned by this audit.'
          )}
        </p>
        {audit.status === 'ignored' || audit.status === 'closed' ? (
          <p className='text-muted-foreground mt-1 text-xs leading-5'>
            {props.t(
              'Ignored and closed audits are read-only for key actions.'
            )}
          </p>
        ) : null}
      </section>
      {props.preview ? (
        <PreviewTable preview={props.preview} t={props.t} />
      ) : null}
    </>
  )
}

function RiskAuditStatusControls({
  audit,
  props,
}: {
  audit: RiskAudit
  props: RiskControlDetailSheetProps
}) {
  const statusTargets = allowedRiskStatuses(audit.status)
  return (
    <section className='mt-4 border-t pt-4'>
      <h3 className='text-sm font-semibold'>{props.t('Audit status')}</h3>
      <div className='mt-3 flex flex-wrap gap-2'>
        {statusTargets.map((status) => (
          <Button
            key={status}
            type='button'
            size='sm'
            variant='outline'
            disabled={props.busy}
            onClick={() => void props.onStatusChange(status)}
          >
            {props.t(status)}
          </Button>
        ))}
        {statusTargets.length === 0 ? (
          <span className='text-muted-foreground text-xs'>
            {props.t('Closed audits are read-only')}
          </span>
        ) : null}
      </div>
    </section>
  )
}

function RiskDetailBody({
  audit,
  props,
}: {
  audit: RiskAudit
  props: RiskControlDetailSheetProps
}) {
  const tokenIds = parseRiskIdList(audit.token_ids)
  const userIds = parseRiskIdList(audit.user_ids)
  const evidence = parseRiskEvidence(audit.evidence)
  return (
    <div className='min-h-0 flex-1 overflow-y-auto px-5 py-4'>
      <section className='grid gap-x-6 md:grid-cols-2'>
        <FactBlock
          icon={UsersIcon}
          label={props.t('Affected users')}
          value={userIds.length > 0 ? userIds.join(', ') : '-'}
        />
        <FactBlock
          icon={KeyRoundIcon}
          label={props.t('Audit-bound key IDs')}
          value={tokenIds.length > 0 ? tokenIds.join(', ') : '-'}
        />
        <FactBlock
          icon={NetworkIcon}
          label={props.t('Observed IP')}
          value={audit.ip || '-'}
        />
        <FactBlock
          icon={Clock3Icon}
          label={props.t('Last updated')}
          value={formatRiskTime(audit.updated_at)}
        />
      </section>
      <section className='border-t pt-4'>
        <div className='flex items-center gap-2'>
          <ShieldCheckIcon
            className='text-muted-foreground size-4'
            aria-hidden='true'
          />
          <h3 className='text-sm font-semibold'>{props.t('Evidence')}</h3>
        </div>
        <pre className='bg-muted/35 mt-3 max-h-64 overflow-auto border p-3 text-xs leading-5 whitespace-pre-wrap'>
          {evidence ? JSON.stringify(evidence, null, 2) : audit.evidence || '-'}
        </pre>
      </section>
      <RiskActionControls audit={audit} props={props} />
      <RiskAuditStatusControls audit={audit} props={props} />
    </div>
  )
}

function RiskDetailFooter({
  props,
  confirmKind,
  onConfirm,
}: {
  props: RiskControlDetailSheetProps
  confirmKind: RiskActionKind
  onConfirm: () => void
}) {
  const reasonReady = props.reason.trim().length >= 8
  const previewReady = riskPreviewHasScope(props.preview)
  return (
    <SheetFooter className='border-t px-5 py-3'>
      <div className='flex items-center justify-between gap-3'>
        <span className='text-muted-foreground text-xs'>
          {props.preview
            ? (props.preview.result.matched_user_controls ?? 0) > 0
              ? props.t(
                  'Preview scope: {{count}} key(s), {{users}} account restriction(s)',
                  {
                    count: props.preview.result.matched_tokens,
                    users: props.preview.result.matched_user_controls ?? 0,
                  }
                )
              : props.t('Preview scope: {{count}} key(s)', {
                  count: props.preview.result.matched_tokens,
                })
            : props.t('Run a preview before committing a key change')}
        </span>
        <Button
          type='button'
          variant={confirmKind === 'disable' ? 'destructive' : 'default'}
          disabled={props.busy || !reasonReady || !previewReady}
          onClick={onConfirm}
        >
          {confirmKind === 'disable' ? (
            <BanIcon data-icon='inline-start' aria-hidden='true' />
          ) : (
            <RotateCcwIcon data-icon='inline-start' aria-hidden='true' />
          )}
          {confirmKind === 'disable'
            ? props.t('Confirm disable')
            : props.t('Confirm restore')}
        </Button>
      </div>
    </SheetFooter>
  )
}

function RiskActionConfirmation({
  open,
  onOpenChange,
  kind,
  props,
  onCommit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  kind: RiskActionKind
  props: RiskControlDetailSheetProps
  onCommit: () => Promise<void>
}) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className='sm:max-w-lg'>
        <AlertDialogHeader>
          <AlertDialogMedia
            className={
              kind === 'disable'
                ? 'bg-destructive/10 text-destructive'
                : 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
            }
          >
            {kind === 'disable' ? (
              <AlertTriangleIcon aria-hidden='true' />
            ) : (
              <CheckCircle2Icon aria-hidden='true' />
            )}
          </AlertDialogMedia>
          <AlertDialogTitle>
            {kind === 'disable'
              ? props.t('Disable previewed keys?')
              : (props.preview?.result.matched_user_controls ?? 0) > 0
                ? props.t('Restore frozen keys')
                : props.t('Restore previewed keys?')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {(props.preview?.result.matched_user_controls ?? 0) > 0
              ? props.t(
                  'Only the {{count}} key(s) and {{users}} account restriction(s) returned by this audit preview will be changed. Other keys owned by the same users remain untouched.',
                  {
                    count: props.preview?.result.matched_tokens ?? 0,
                    users: props.preview?.result.matched_user_controls ?? 0,
                  }
                )
              : props.t(
                  'Only the {{count}} key(s) returned by this audit preview will be changed. Other keys owned by the same users remain untouched.',
                  { count: props.preview?.result.matched_tokens ?? 0 }
                )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className='bg-muted/40 border px-3 py-2 text-xs leading-5'>
          <span className='font-medium'>{props.t('Recorded reason')}:</span>{' '}
          {props.reason.trim()}
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={props.busy}>
            {props.t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            variant={kind === 'disable' ? 'destructive' : 'default'}
            disabled={props.busy}
            onClick={(event) => {
              event.preventDefault()
              void onCommit()
            }}
          >
            {props.busy ? props.t('Processing...') : props.t('Confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export function RiskControlDetailSheet(props: RiskControlDetailSheetProps) {
  const [confirmOpen, setConfirmOpen] = useState(false)
  const audit = props.audit
  const confirmKind = props.preview?.kind ?? 'disable'
  const reasonReady = props.reason.trim().length >= 8
  const previewReady = riskPreviewHasScope(props.preview)

  if (!audit) return <Sheet open={false} onOpenChange={props.onOpenChange} />

  const commit = async () => {
    if (!reasonReady || !previewReady || props.busy) return
    await props.onCommit(confirmKind)
    setConfirmOpen(false)
  }
  return (
    <>
      <Sheet
        open={props.open}
        onOpenChange={(open) => {
          if (!open) setConfirmOpen(false)
          props.onOpenChange(open)
        }}
      >
        <SheetContent className='w-full sm:max-w-2xl' side='right'>
          <SheetHeader className='border-b px-5 py-4 pr-12'>
            <div className='flex flex-wrap items-center gap-2'>
              <OpsStatusBadge tone={severityTone(audit.severity)}>
                {props.t(audit.severity || 'medium')}
              </OpsStatusBadge>
              <OpsStatusBadge tone={statusTone(audit.status)}>
                {props.t(audit.status)}
              </OpsStatusBadge>
              <span className='text-muted-foreground font-mono text-xs'>
                #{audit.id}
              </span>
            </div>
            <SheetTitle className='mt-2 pr-2 text-lg'>
              {riskTypeLabel(audit.risk_type, props.t)}
            </SheetTitle>
            <SheetDescription className='line-clamp-2'>
              {audit.subject || props.t('No subject')}
            </SheetDescription>
          </SheetHeader>
          <RiskDetailBody audit={audit} props={props} />
          <RiskDetailFooter
            props={props}
            confirmKind={confirmKind}
            onConfirm={() => {
              if (!reasonReady || !previewReady || props.busy) return
              setConfirmOpen(true)
            }}
          />
        </SheetContent>
      </Sheet>
      <RiskActionConfirmation
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        kind={confirmKind}
        props={props}
        onCommit={commit}
      />
    </>
  )
}
