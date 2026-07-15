/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ChevronLeft,
  ChevronRight,
  Edit3,
  Plus,
  RefreshCw,
  Send,
  Trash2,
  Upload,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SettingsSection } from '../components/settings-section'
import type { OperationsSettings } from '../types'
import { useOpsT } from './ops-i18n'
import {
  createQuizBank,
  deleteQuizBinding,
  getQuizStats,
  importQuizQuestions,
  listQuizBanks,
  listQuizBindings,
  listQuizQuestions,
  publishQuizBank,
  quizApiError,
  saveQuizBinding,
  saveQuizQuestion,
  setQuizQuestionStatus,
  updateQuizBank,
} from './quiz-bank-api'
import {
  DeleteQuizBindingDialog,
  QuizBankDialog,
  QuizBindingDialog,
  QuizImportDialog,
  QuizQuestionDialog,
} from './quiz-bank-dialogs'
import type {
  QuizBank,
  QuizBankInput,
  QuizBindingInput,
  QuizBindingListItem,
  QuizImportResult,
  QuizQuestionInput,
  QuizQuestionListItem,
} from './quiz-bank-model'

const PAGE_SIZE = 20

function statusVariant(status: string) {
  if (status === 'published' || status === 'active') return 'default' as const
  if (status === 'disabled') return 'destructive' as const
  return 'secondary' as const
}

function statusLabel(status: string, t: ReturnType<typeof useOpsT>) {
  if (status === 'published') return t('Published')
  if (status === 'disabled') return t('Disabled')
  if (status === 'active') return t('Active')
  return t('Draft')
}

function LoadingRows({ columns }: { columns: number }) {
  return (
    <>
      {[0, 1, 2].map((row) => (
        <TableRow key={row}>
          <TableCell colSpan={columns} className='h-12'>
            <div className='bg-muted h-3 animate-pulse rounded' />
          </TableCell>
        </TableRow>
      ))}
    </>
  )
}

export function QuizBankSection({
  defaultValues,
}: {
  defaultValues: OperationsSettings
}) {
  const t = useOpsT()
  const siteId =
    String(defaultValues['agent_setting.site_id'] ?? '').trim() || 'prox'
  const [selectedBankId, setSelectedBankId] = useState(0)
  const [status, setStatus] = useState('all')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [busyAction, setBusyAction] = useState('')
  const [bankDialogOpen, setBankDialogOpen] = useState(false)
  const [questionDialogOpen, setQuestionDialogOpen] = useState(false)
  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [bindingDialogOpen, setBindingDialogOpen] = useState(false)
  const [editingBank, setEditingBank] = useState<QuizBank | null>(null)
  const [editingQuestion, setEditingQuestion] =
    useState<QuizQuestionListItem | null>(null)
  const [editingBinding, setEditingBinding] =
    useState<QuizBindingListItem | null>(null)
  const [deletingBinding, setDeletingBinding] =
    useState<QuizBindingListItem | null>(null)
  const [importPreview, setImportPreview] = useState<QuizImportResult | null>(
    null
  )

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim())
      setPage(1)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [searchInput])

  const statsQuery = useQuery({
    queryKey: ['ops-quiz-stats', siteId],
    queryFn: () => getQuizStats(siteId),
    staleTime: 30_000,
    retry: 1,
  })
  const banksQuery = useQuery({
    queryKey: ['ops-quiz-banks', siteId],
    queryFn: () => listQuizBanks(siteId),
    staleTime: 30_000,
    retry: 1,
  })
  const bindingsQuery = useQuery({
    queryKey: ['ops-quiz-bindings', siteId],
    queryFn: () => listQuizBindings(siteId),
    staleTime: 30_000,
    retry: 1,
  })

  const banks = useMemo(() => banksQuery.data ?? [], [banksQuery.data])
  useEffect(() => {
    if (!banks.length) {
      setSelectedBankId(0)
      return
    }
    if (!banks.some((item) => item.bank.id === selectedBankId)) {
      setSelectedBankId(banks[0].bank.id)
    }
  }, [banks, selectedBankId])

  const selectedBank =
    banks.find((item) => item.bank.id === selectedBankId) ?? null
  const questionQuery = useQuery({
    queryKey: [
      'ops-quiz-questions',
      siteId,
      selectedBankId,
      status,
      search,
      page,
    ],
    enabled: selectedBankId > 0,
    queryFn: () =>
      listQuizQuestions(siteId, selectedBankId, {
        status: status === 'all' ? '' : status,
        search,
        offset: (page - 1) * PAGE_SIZE,
        limit: PAGE_SIZE,
      }),
    staleTime: 15_000,
    retry: 1,
  })
  const totalPages = Math.max(
    1,
    Math.ceil((questionQuery.data?.total ?? 0) / PAGE_SIZE)
  )

  useEffect(() => {
    if (page > totalPages) setPage(totalPages)
  }, [page, totalPages])

  const refresh = async () => {
    await Promise.all([
      statsQuery.refetch(),
      banksQuery.refetch(),
      bindingsQuery.refetch(),
      selectedBankId > 0 ? questionQuery.refetch() : Promise.resolve(),
    ])
  }

  const run = async (
    action: string,
    operation: () => Promise<void>,
    successMessage: string
  ) => {
    setBusyAction(action)
    try {
      await operation()
      toast.success(successMessage)
      await refresh()
    } catch (error) {
      toast.error(quizApiError(error))
    } finally {
      setBusyAction('')
    }
  }

  const openCreateBank = () => {
    setEditingBank(null)
    setBankDialogOpen(true)
  }
  const openEditBank = () => {
    if (!selectedBank) return
    setEditingBank(selectedBank.bank)
    setBankDialogOpen(true)
  }
  const openCreateQuestion = () => {
    setEditingQuestion(null)
    setQuestionDialogOpen(true)
  }
  const openCreateBinding = () => {
    setEditingBinding(null)
    setBindingDialogOpen(true)
  }

  const stats = statsQuery.data
  const statsItems = [
    [t('Quiz banks'), stats?.banks ?? 0],
    [t('Questions'), stats?.questions ?? 0],
    [t('Bindings'), stats?.bindings ?? 0],
    [t('Total rounds'), stats?.draws ?? 0],
    [t('Open rounds'), stats?.open_draws ?? 0],
  ]
  const loading =
    statsQuery.isFetching || banksQuery.isFetching || bindingsQuery.isFetching

  return (
    <SettingsSection title={t('Quiz bank management')}>
      <div className='flex flex-col gap-4'>
        <header className='flex flex-col gap-3 border-b pb-4 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex min-w-0 items-center gap-2'>
            <Badge variant='outline'>{siteId}</Badge>
            {loading ? (
              <span className='text-muted-foreground text-xs'>
                {t('Refreshing...')}
              </span>
            ) : null}
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='outline'
              onClick={() => void refresh()}
              disabled={loading || Boolean(busyAction)}
            >
              <RefreshCw aria-hidden='true' />
              {t('Refresh')}
            </Button>
            <Button type='button' onClick={openCreateBank}>
              <Plus aria-hidden='true' />
              {t('New quiz bank')}
            </Button>
          </div>
        </header>

        <div className='grid grid-cols-2 border-y sm:grid-cols-5'>
          {statsItems.map(([label, value]) => (
            <div key={String(label)} className='min-w-0 px-3 py-3'>
              <div className='text-muted-foreground truncate text-xs'>
                {label}
              </div>
              <div className='mt-1 text-xl font-semibold'>{value}</div>
            </div>
          ))}
        </div>

        {banksQuery.isError ? (
          <div className='border-destructive/40 text-destructive border px-3 py-2 text-sm'>
            {quizApiError(banksQuery.error)}
          </div>
        ) : null}

        {banks.length ? (
          <div className='grid gap-3 border-b pb-4 lg:grid-cols-[minmax(240px,1fr)_auto] lg:items-end'>
            <div className='grid gap-1.5'>
              <span className='text-sm font-medium'>{t('Quiz bank')}</span>
              <Select
                value={String(selectedBankId)}
                onValueChange={(value) => {
                  setSelectedBankId(Number(value ?? 0))
                  setPage(1)
                }}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {banks.map((item) => (
                    <SelectItem key={item.bank.id} value={String(item.bank.id)}>
                      {item.bank.name} ({item.published_question_count}/
                      {item.question_count})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-wrap gap-2'>
              {selectedBank ? (
                <Badge variant={statusVariant(selectedBank.bank.status)}>
                  {statusLabel(selectedBank.bank.status, t)}
                </Badge>
              ) : null}
              <Button type='button' variant='outline' onClick={openEditBank}>
                <Edit3 aria-hidden='true' />
                {t('Edit')}
              </Button>
              <Button
                type='button'
                variant='outline'
                disabled={
                  !selectedBank ||
                  selectedBank.published_question_count < 1 ||
                  Boolean(busyAction)
                }
                onClick={() => {
                  if (!selectedBank) return
                  void run(
                    'publish-bank',
                    async () => {
                      await publishQuizBank(siteId, selectedBank.bank.id)
                    },
                    t('Quiz bank published')
                  )
                }}
              >
                <Send aria-hidden='true' />
                {t('Publish')}
              </Button>
            </div>
          </div>
        ) : (
          <div className='border-y py-12 text-center'>
            <p className='font-medium'>{t('No quiz banks')}</p>
            <Button type='button' className='mt-3' onClick={openCreateBank}>
              <Plus aria-hidden='true' />
              {t('Create quiz bank')}
            </Button>
          </div>
        )}

        <Tabs defaultValue='questions'>
          <TabsList variant='line'>
            <TabsTrigger value='questions'>{t('Questions')}</TabsTrigger>
            <TabsTrigger value='bindings'>{t('Bindings')}</TabsTrigger>
          </TabsList>

          <TabsContent value='questions' className='mt-3'>
            <div className='border'>
              <div className='flex flex-col gap-3 border-b p-3 lg:flex-row lg:items-center lg:justify-between'>
                <div className='flex flex-1 flex-col gap-2 sm:flex-row'>
                  <Input
                    value={searchInput}
                    onChange={(event) => setSearchInput(event.target.value)}
                    placeholder={t('Search questions')}
                    className='w-full sm:max-w-sm'
                  />
                  <Select
                    value={status}
                    onValueChange={(value) => {
                      setStatus(String(value ?? 'all'))
                      setPage(1)
                    }}
                  >
                    <SelectTrigger className='w-full sm:w-40'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='all'>{t('All statuses')}</SelectItem>
                      <SelectItem value='draft'>{t('Draft')}</SelectItem>
                      <SelectItem value='published'>
                        {t('Published')}
                      </SelectItem>
                      <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='flex flex-wrap gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    disabled={!selectedBank}
                    onClick={() => {
                      setImportPreview(null)
                      setImportDialogOpen(true)
                    }}
                  >
                    <Upload aria-hidden='true' />
                    {t('Import')}
                  </Button>
                  <Button
                    type='button'
                    disabled={!selectedBank}
                    onClick={openCreateQuestion}
                  >
                    <Plus aria-hidden='true' />
                    {t('New question')}
                  </Button>
                </div>
              </div>
              <Table className='min-w-[980px] table-fixed'>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-[410px]'>{t('Question')}</TableHead>
                    <TableHead className='w-[140px]'>{t('Category')}</TableHead>
                    <TableHead className='w-[110px]'>
                      {t('Difficulty')}
                    </TableHead>
                    <TableHead className='w-[110px]'>{t('Status')}</TableHead>
                    <TableHead className='w-[110px]'>{t('Weight')}</TableHead>
                    <TableHead className='w-[160px] text-right'>
                      {t('Actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {questionQuery.isLoading ? <LoadingRows columns={6} /> : null}
                  {!questionQuery.isLoading &&
                  (questionQuery.data?.items.length ?? 0) === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className='h-36 text-center'>
                        {t('No matching questions')}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {(questionQuery.data?.items ?? []).map((item) => (
                    <TableRow key={item.question.id}>
                      <TableCell className='whitespace-normal'>
                        <div className='line-clamp-2 font-medium'>
                          {item.question.prompt}
                        </div>
                        <div className='text-muted-foreground mt-1 truncate text-xs'>
                          {item.question.external_key}
                        </div>
                      </TableCell>
                      <TableCell>{item.category?.name || '-'}</TableCell>
                      <TableCell>{item.question.difficulty}</TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(item.question.status)}>
                          {statusLabel(item.question.status, t)}
                        </Badge>
                      </TableCell>
                      <TableCell>{item.question.weight}</TableCell>
                      <TableCell>
                        <div className='flex justify-end gap-1'>
                          <Button
                            type='button'
                            size='sm'
                            variant='ghost'
                            onClick={() => {
                              setEditingQuestion(item)
                              setQuestionDialogOpen(true)
                            }}
                          >
                            <Edit3 aria-hidden='true' />
                            {t('Edit')}
                          </Button>
                          <Button
                            type='button'
                            size='sm'
                            variant='outline'
                            disabled={Boolean(busyAction)}
                            onClick={() =>
                              void run(
                                `question-status-${item.question.id}`,
                                async () => {
                                  await setQuizQuestionStatus(
                                    siteId,
                                    selectedBankId,
                                    item.question.id,
                                    item.question.status === 'published'
                                      ? 'disabled'
                                      : 'published'
                                  )
                                },
                                t('Question status updated')
                              )
                            }
                          >
                            {item.question.status === 'published'
                              ? t('Disable')
                              : t('Publish')}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <footer className='flex flex-col gap-3 border-t p-3 sm:flex-row sm:items-center sm:justify-between'>
                <span className='text-muted-foreground text-xs'>
                  {t('{{total}} questions', {
                    total: questionQuery.data?.total ?? 0,
                  })}
                </span>
                <div className='flex items-center gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    disabled={page <= 1 || questionQuery.isFetching}
                    onClick={() => setPage((current) => current - 1)}
                  >
                    <ChevronLeft aria-hidden='true' />
                    {t('Previous')}
                  </Button>
                  <span className='min-w-16 text-center text-xs font-medium'>
                    {page} / {totalPages}
                  </span>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    disabled={page >= totalPages || questionQuery.isFetching}
                    onClick={() => setPage((current) => current + 1)}
                  >
                    {t('Next')}
                    <ChevronRight aria-hidden='true' />
                  </Button>
                </div>
              </footer>
            </div>
          </TabsContent>

          <TabsContent value='bindings' className='mt-3'>
            <div className='border'>
              <div className='flex justify-end border-b p-3'>
                <Button
                  type='button'
                  disabled={!banks.length}
                  onClick={openCreateBinding}
                >
                  <Plus aria-hidden='true' />
                  {t('New binding')}
                </Button>
              </div>
              <Table className='min-w-[860px] table-fixed'>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-[210px]'>
                      {t('Quiz bank')}
                    </TableHead>
                    <TableHead className='w-[130px]'>{t('Platform')}</TableHead>
                    <TableHead className='w-[220px]'>{t('Group ID')}</TableHead>
                    <TableHead className='w-[100px]'>{t('Priority')}</TableHead>
                    <TableHead className='w-[100px]'>{t('Status')}</TableHead>
                    <TableHead className='w-[190px] text-right'>
                      {t('Actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {bindingsQuery.isLoading ? <LoadingRows columns={6} /> : null}
                  {!bindingsQuery.isLoading &&
                  (bindingsQuery.data?.length ?? 0) === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className='h-36 text-center'>
                        {t('No quiz bank bindings')}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {(bindingsQuery.data ?? []).map((item) => (
                    <TableRow key={item.binding.id}>
                      <TableCell className='font-medium'>
                        {item.bank.name || `#${item.binding.bank_id}`}
                      </TableCell>
                      <TableCell>{item.binding.platform}</TableCell>
                      <TableCell>{item.binding.group_id}</TableCell>
                      <TableCell>{item.binding.priority}</TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            item.binding.enabled ? 'default' : 'secondary'
                          }
                        >
                          {item.binding.enabled ? t('Enabled') : t('Disabled')}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className='flex justify-end gap-1'>
                          <Button
                            type='button'
                            size='sm'
                            variant='ghost'
                            onClick={() => {
                              setEditingBinding(item)
                              setBindingDialogOpen(true)
                            }}
                          >
                            <Edit3 aria-hidden='true' />
                            {t('Edit')}
                          </Button>
                          <Button
                            type='button'
                            size='sm'
                            variant='outline'
                            disabled={Boolean(busyAction)}
                            onClick={() =>
                              void run(
                                `binding-toggle-${item.binding.id}`,
                                async () => {
                                  await saveQuizBinding(siteId, {
                                    id: item.binding.id,
                                    bank_id: item.binding.bank_id,
                                    platform: item.binding.platform,
                                    group_id: item.binding.group_id,
                                    enabled: !item.binding.enabled,
                                    priority: item.binding.priority,
                                    rules: item.rules,
                                  })
                                },
                                t('Binding status updated')
                              )
                            }
                          >
                            {item.binding.enabled ? t('Disable') : t('Enable')}
                          </Button>
                          <Button
                            type='button'
                            size='icon-sm'
                            variant='destructive'
                            title={t('Delete')}
                            aria-label={t('Delete')}
                            onClick={() => setDeletingBinding(item)}
                          >
                            <Trash2 aria-hidden='true' />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </TabsContent>
        </Tabs>
      </div>

      <QuizBankDialog
        open={bankDialogOpen}
        busy={busyAction === 'save-bank'}
        onOpenChange={setBankDialogOpen}
        t={t}
        initial={editingBank}
        onSubmit={async (input: QuizBankInput) => {
          await run(
            'save-bank',
            async () => {
              const saved = editingBank
                ? await updateQuizBank(siteId, editingBank.id, input)
                : await createQuizBank(siteId, input)
              setSelectedBankId(saved.id)
              setBankDialogOpen(false)
            },
            editingBank ? t('Quiz bank updated') : t('Quiz bank created')
          )
        }}
      />
      <QuizQuestionDialog
        open={questionDialogOpen}
        busy={busyAction === 'save-question'}
        onOpenChange={setQuestionDialogOpen}
        t={t}
        initial={editingQuestion}
        onSubmit={async (input: QuizQuestionInput) => {
          await run(
            'save-question',
            async () => {
              await saveQuizQuestion(
                siteId,
                selectedBankId,
                editingQuestion?.question.id ?? null,
                input
              )
              setQuestionDialogOpen(false)
            },
            editingQuestion ? t('Question updated') : t('Question created')
          )
        }}
      />
      <QuizImportDialog
        open={importDialogOpen}
        busy={
          busyAction === 'import-questions' || busyAction === 'import-preview'
        }
        onOpenChange={setImportDialogOpen}
        t={t}
        preview={importPreview}
        onResetPreview={() => setImportPreview(null)}
        onPreview={async (questions, publish) => {
          setBusyAction('import-preview')
          try {
            setImportPreview(
              await importQuizQuestions(
                siteId,
                selectedBankId,
                questions,
                true,
                publish
              )
            )
          } catch (error) {
            toast.error(quizApiError(error))
          } finally {
            setBusyAction('')
          }
        }}
        onImport={async (questions, publish) => {
          await run(
            'import-questions',
            async () => {
              await importQuizQuestions(
                siteId,
                selectedBankId,
                questions,
                false,
                publish
              )
              setImportDialogOpen(false)
              setImportPreview(null)
            },
            t('Questions imported')
          )
        }}
      />
      <QuizBindingDialog
        open={bindingDialogOpen}
        busy={busyAction === 'save-binding'}
        onOpenChange={setBindingDialogOpen}
        t={t}
        banks={banks}
        initial={editingBinding}
        onSubmit={async (input: QuizBindingInput) => {
          await run(
            'save-binding',
            async () => {
              await saveQuizBinding(siteId, input)
              setBindingDialogOpen(false)
            },
            editingBinding ? t('Binding updated') : t('Binding created')
          )
        }}
      />
      <DeleteQuizBindingDialog
        item={deletingBinding}
        busy={busyAction === 'delete-binding'}
        t={t}
        onOpenChange={(open) => {
          if (!open) setDeletingBinding(null)
        }}
        onConfirm={async () => {
          if (!deletingBinding) return
          await run(
            'delete-binding',
            async () => {
              await deleteQuizBinding(siteId, deletingBinding.binding.id)
              setDeletingBinding(null)
            },
            t('Binding deleted')
          )
        }}
      />
    </SettingsSection>
  )
}
