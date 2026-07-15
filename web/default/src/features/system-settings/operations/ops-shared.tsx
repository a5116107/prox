/*
Copyright (C) 2023-2026 QuantumNous
*/
import { type ReactNode } from 'react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export type OpsTone = 'neutral' | 'success' | 'warning' | 'danger' | 'info'

// kept for backward-compatible local usage
type Tone = OpsTone

type Metric = {
  label: string
  value: ReactNode
  helper?: ReactNode
}

type ActionItem = {
  id: string
  label: ReactNode
  onClick?: () => void
  href?: string
  external?: boolean
  tone?: Tone
}

type TableColumn = {
  key: string
  label: string
  className?: string
}

type TableRowData = {
  id: string
  cells: ReactNode[]
}

function joinClassNames(...values: Array<string | undefined | false | null>) {
  return values.filter(Boolean).join(' ')
}

const toneClasses: Record<Tone, string> = {
  neutral: 'border-border/70 bg-muted/50 text-foreground',
  success:
    'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  warning:
    'border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300',
  danger: 'border-rose-500/20 bg-rose-500/10 text-rose-700 dark:text-rose-300',
  info: 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300',
}

function toneButtonClass(tone: Tone) {
  switch (tone) {
    case 'success':
      return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/15 dark:text-emerald-300'
    case 'warning':
      return 'border-amber-500/20 bg-amber-500/10 text-amber-700 hover:bg-amber-500/15 dark:text-amber-300'
    case 'danger':
      return 'border-rose-500/20 bg-rose-500/10 text-rose-700 hover:bg-rose-500/15 dark:text-rose-300'
    case 'info':
      return 'border-sky-500/20 bg-sky-500/10 text-sky-700 hover:bg-sky-500/15 dark:text-sky-300'
    default:
      return 'border-border bg-background text-foreground hover:bg-muted/50'
  }
}

export function OpsStatusBadge({
  tone = 'neutral',
  children,
}: {
  tone?: Tone
  children: ReactNode
}) {
  return (
    <span
      className={joinClassNames(
        'inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-medium',
        toneClasses[tone]
      )}
    >
      {children}
    </span>
  )
}

function OpsMetricCard({ metric }: { metric: Metric }) {
  return (
    <div className='bg-background/90 rounded-2xl border p-4 shadow-sm'>
      <div className='text-muted-foreground text-xs tracking-wide'>
        {metric.label}
      </div>
      <div className='mt-2 text-2xl font-semibold tracking-tight'>
        {metric.value}
      </div>
      {metric.helper ? (
        <div className='text-muted-foreground mt-2 text-xs leading-5'>
          {metric.helper}
        </div>
      ) : null}
    </div>
  )
}

function OpsActionButton({ item }: { item: ActionItem }) {
  const className = joinClassNames(
    'inline-flex min-h-10 items-center justify-center rounded-xl border px-3 py-2 text-sm font-medium transition-colors',
    toneButtonClass(item.tone ?? 'neutral')
  )

  if (item.href) {
    return (
      <a
        href={item.href}
        target={item.external ? '_blank' : undefined}
        rel={item.external ? 'noreferrer' : undefined}
        className={className}
      >
        {item.label}
      </a>
    )
  }

  return (
    <button type='button' onClick={item.onClick} className={className}>
      {item.label}
    </button>
  )
}

function OpsActionLink({ item }: { item: ActionItem }) {
  const className =
    'inline-flex min-h-9 items-center rounded-lg px-1 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40'

  if (item.href) {
    return (
      <a
        href={item.href}
        target={item.external ? '_blank' : undefined}
        rel={item.external ? 'noreferrer' : undefined}
        className={className}
      >
        {item.label}
      </a>
    )
  }

  return (
    <button type='button' onClick={item.onClick} className={className}>
      {item.label}
    </button>
  )
}

export function OpsHeroCard({
  eyebrow,
  title,
  description,
  badges = [],
  metrics = [],
  actions = [],
  quickLinks = [],
}: {
  eyebrow?: ReactNode
  title: ReactNode
  description: ReactNode
  badges?: Array<ReactNode>
  metrics?: Metric[]
  actions?: ActionItem[]
  quickLinks?: ActionItem[]
}) {
  return (
    <Card className='border-primary/20 from-background via-background to-primary/5 overflow-hidden bg-gradient-to-br shadow-sm'>
      <CardHeader className='bg-muted/20 border-b'>
        <div className='grid gap-6 xl:grid-cols-[minmax(0,1.65fr)_minmax(320px,.85fr)] xl:items-start'>
          <div className='space-y-4'>
            {eyebrow ? (
              <div className='text-muted-foreground text-xs font-medium tracking-[0.2em] uppercase'>
                {eyebrow}
              </div>
            ) : null}
            <div className='space-y-2'>
              <CardTitle className='text-2xl tracking-tight md:text-3xl'>
                {title}
              </CardTitle>
              <CardDescription className='max-w-4xl text-sm leading-6 md:text-base'>
                {description}
              </CardDescription>
            </div>
            {badges.length > 0 ? (
              <div className='flex flex-wrap gap-2'>
                {badges.map((badge, index) => (
                  <div key={index}>{badge}</div>
                ))}
              </div>
            ) : null}
          </div>
          {actions.length > 0 || quickLinks.length > 0 ? (
            <div className='flex flex-col gap-3 xl:items-end'>
              {actions.length > 0 ? (
                <div className='flex flex-wrap items-center gap-2 xl:justify-end'>
                  {actions.map((item) => (
                    <OpsActionButton key={item.id} item={item} />
                  ))}
                </div>
              ) : null}
              {quickLinks.length > 0 ? (
                <div className='flex flex-wrap items-center gap-x-3 gap-y-1.5 xl:justify-end'>
                  {quickLinks.map((item) => (
                    <OpsActionLink key={item.id} item={item} />
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </CardHeader>
      {metrics.length > 0 ? (
        <CardContent className='grid gap-3 p-4 md:grid-cols-2 xl:grid-cols-4'>
          {metrics.map((metric) => (
            <OpsMetricCard key={String(metric.label)} metric={metric} />
          ))}
        </CardContent>
      ) : null}
    </Card>
  )
}

export function OpsSurfaceGrid({
  className,
  children,
}: {
  className?: string
  children: ReactNode
}) {
  return (
    <div className={joinClassNames('grid gap-4 md:grid-cols-2', className)}>
      {children}
    </div>
  )
}

export function OpsPanel({
  title,
  description,
  children,
  className,
}: {
  title: ReactNode
  description?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <Card className={joinClassNames('overflow-hidden shadow-sm', className)}>
      <CardHeader className='bg-muted/20 border-b pb-4'>
        <CardTitle className='text-base'>{title}</CardTitle>
        {description ? (
          <CardDescription className='leading-6'>{description}</CardDescription>
        ) : null}
      </CardHeader>
      <CardContent className='p-4'>{children}</CardContent>
    </Card>
  )
}

export function OpsInsightCard({
  title,
  value,
  description,
  badge,
}: {
  title: ReactNode
  value: ReactNode
  description: ReactNode
  badge?: ReactNode
}) {
  return (
    <div className='bg-background rounded-2xl border p-4 shadow-sm'>
      <div className='flex items-start justify-between gap-3'>
        <div className='text-sm font-medium'>{title}</div>
        {badge ? <div>{badge}</div> : null}
      </div>
      <div className='mt-3 text-xl font-semibold tracking-tight'>{value}</div>
      <div className='text-muted-foreground mt-2 text-sm leading-6'>
        {description}
      </div>
    </div>
  )
}

export function OpsStageRail({
  steps,
  columnsClassName,
}: {
  steps: Array<{
    id: string
    title: ReactNode
    description: ReactNode
    badge?: ReactNode
  }>
  columnsClassName?: string
}) {
  return (
    <div
      className={joinClassNames(
        'grid gap-3 md:grid-cols-2 xl:grid-cols-4',
        columnsClassName
      )}
    >
      {steps.map((step) => (
        <div
          key={step.id}
          className='bg-background rounded-2xl border p-4 shadow-sm'
        >
          <div className='flex items-center justify-between gap-3'>
            <div className='text-muted-foreground text-xs font-medium tracking-[0.2em]'>
              {step.id}
            </div>
            {step.badge ? <div>{step.badge}</div> : null}
          </div>
          <div className='mt-3 text-sm font-semibold'>{step.title}</div>
          <div className='text-muted-foreground mt-2 text-sm leading-6'>
            {step.description}
          </div>
        </div>
      ))}
    </div>
  )
}

export function OpsDataTable({
  title,
  description,
  columns,
  rows,
  emptyMessage,
  className,
}: {
  title: ReactNode
  description?: ReactNode
  columns: TableColumn[]
  rows: TableRowData[]
  emptyMessage: ReactNode
  className?: string
}) {
  return (
    <OpsPanel title={title} description={description} className={className}>
      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map((column) => (
                <TableHead key={column.key} className={column.className}>
                  {column.label}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length > 0 ? (
              rows.map((row) => (
                <TableRow key={row.id}>
                  {row.cells.map((cell, index) => (
                    <TableCell key={`${row.id}-${index}`}>{cell}</TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className='text-muted-foreground py-8 text-center'
                >
                  {emptyMessage}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </OpsPanel>
  )
}

const toneDot: Record<OpsTone, string> = {
  neutral: 'bg-muted-foreground/60',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-rose-500',
  info: 'bg-sky-500',
}

export function OpsModeBanner({
  tone = 'info',
  title,
  description,
  children,
}: {
  tone?: OpsTone
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
}) {
  return (
    <div
      className={joinClassNames(
        'rounded-2xl border p-4 shadow-sm',
        toneClasses[tone]
      )}
    >
      <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
        <div className='space-y-1'>
          <div className='text-base font-semibold'>{title}</div>
          {description ? (
            <div className='text-sm leading-6 opacity-90'>{description}</div>
          ) : null}
        </div>
        {children ? (
          <div className='flex flex-wrap items-center gap-2'>{children}</div>
        ) : null}
      </div>
    </div>
  )
}

type OpsMatrixCell = { label: ReactNode; tone: OpsTone }

export type OpsMatrixRow = {
  benefit: ReactNode
  active: OpsMatrixCell
  grace: OpsMatrixCell
  expired: OpsMatrixCell
  paid: OpsMatrixCell
}

function MatrixCellPill({ cell }: { cell: OpsMatrixCell }) {
  return (
    <div className='flex justify-center'>
      <span
        className={joinClassNames(
          'inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-medium',
          toneClasses[cell.tone]
        )}
      >
        {cell.label}
      </span>
    </div>
  )
}

export function OpsImpactMatrix({
  title,
  description,
  rowHeader,
  columns,
  rows,
  note,
}: {
  title: ReactNode
  description?: ReactNode
  rowHeader: ReactNode
  columns: [ReactNode, ReactNode, ReactNode, ReactNode]
  rows: OpsMatrixRow[]
  note?: ReactNode
}) {
  return (
    <OpsPanel title={title} description={description}>
      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{rowHeader}</TableHead>
              {columns.map((column, index) => (
                <TableHead key={index} className='text-center'>
                  {column}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={5}
                  className='text-muted-foreground py-8 text-center'
                >
                  —
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row, index) => (
                <TableRow key={index}>
                  <TableCell className='font-medium'>{row.benefit}</TableCell>
                  <MatrixCellPill cell={row.active} />
                  <MatrixCellPill cell={row.grace} />
                  <MatrixCellPill cell={row.expired} />
                  <MatrixCellPill cell={row.paid} />
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      {note ? (
        <div className='text-muted-foreground mt-3 text-xs leading-5'>
          {note}
        </div>
      ) : null}
    </OpsPanel>
  )
}

export type OpsTimelineItem = {
  time?: ReactNode
  type?: ReactNode
  detail?: ReactNode
  tone?: OpsTone
}

export function OpsTimeline({
  title,
  description,
  items,
  emptyMessage,
}: {
  title: ReactNode
  description?: ReactNode
  items: OpsTimelineItem[]
  emptyMessage: ReactNode
}) {
  return (
    <OpsPanel title={title} description={description}>
      {items.length === 0 ? (
        <div className='text-muted-foreground py-6 text-center text-sm'>
          {emptyMessage}
        </div>
      ) : (
        <div className='divide-border/60 divide-y'>
          {items.map((item, index) => (
            <div
              key={index}
              className='flex items-start gap-3 py-2.5 first:pt-0 last:pb-0'
            >
              {item.tone ? (
                <span
                  className={joinClassNames(
                    'mt-1.5 inline-flex h-2 w-2 shrink-0 rounded-full',
                    toneDot[item.tone]
                  )}
                  aria-hidden
                />
              ) : null}
              <div className='min-w-0 flex-1'>
                <div className='flex flex-wrap items-center gap-x-2 gap-y-0.5'>
                  {item.type ? (
                    <span className='text-sm font-semibold'>{item.type}</span>
                  ) : null}
                  {item.time ? (
                    <span className='text-muted-foreground text-xs'>
                      {item.time}
                    </span>
                  ) : null}
                </div>
                {item.detail ? (
                  <div className='text-muted-foreground mt-0.5 truncate text-xs'>
                    {item.detail}
                  </div>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      )}
    </OpsPanel>
  )
}
