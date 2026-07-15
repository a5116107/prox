/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { type FormEvent, useEffect, useMemo, useState } from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { OpsTranslate } from './ops-i18n'
import {
  bankToInput,
  bindingToInput,
  emptyQuizBankInput,
  emptyQuizQuestionInput,
  questionToInput,
  type QuizBank,
  type QuizBankInput,
  type QuizBankListItem,
  type QuizBindingInput,
  type QuizBindingListItem,
  type QuizImportResult,
  type QuizQuestionInput,
  type QuizQuestionListItem,
} from './quiz-bank-model'

type DialogStateProps = {
  open: boolean
  busy: boolean
  onOpenChange: (open: boolean) => void
  t: OpsTranslate
}

function Field({
  id,
  label,
  children,
  className = '',
}: {
  id: string
  label: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className={`grid gap-1.5 ${className}`}>
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  )
}

export function QuizBankDialog({
  open,
  busy,
  onOpenChange,
  t,
  initial,
  onSubmit,
}: DialogStateProps & {
  initial: QuizBank | null
  onSubmit: (input: QuizBankInput) => Promise<void>
}) {
  const [form, setForm] = useState<QuizBankInput>(emptyQuizBankInput)

  useEffect(() => {
    if (open) setForm(initial ? bankToInput(initial) : emptyQuizBankInput)
  }, [initial, open])

  const submit = (event: FormEvent) => {
    event.preventDefault()
    void onSubmit(form)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {initial ? t('Edit quiz bank') : t('Create quiz bank')}
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('Quiz bank details')}
          </DialogDescription>
        </DialogHeader>
        <form className='grid gap-4' onSubmit={submit}>
          <div className='grid gap-4 sm:grid-cols-2'>
            <Field id='quiz-bank-code' label={t('Code')}>
              <Input
                id='quiz-bank-code'
                value={form.code}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    code: event.target.value,
                  }))
                }
                disabled={busy}
                required
              />
            </Field>
            <Field id='quiz-bank-name' label={t('Name')}>
              <Input
                id='quiz-bank-name'
                value={form.name}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
                disabled={busy}
                required
              />
            </Field>
            <Field id='quiz-bank-language' label={t('Default language')}>
              <Input
                id='quiz-bank-language'
                value={form.default_language}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    default_language: event.target.value,
                  }))
                }
                disabled={busy}
              />
            </Field>
            <Field id='quiz-bank-status' label={t('Status')}>
              <Select
                value={form.status}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    status: String(value ?? 'draft'),
                  }))
                }
                disabled={busy}
              >
                <SelectTrigger id='quiz-bank-status' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='draft'>{t('Draft')}</SelectItem>
                  <SelectItem value='published'>{t('Published')}</SelectItem>
                  <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          </div>
          <Field id='quiz-bank-description' label={t('Description')}>
            <Textarea
              id='quiz-bank-description'
              value={form.description}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  description: event.target.value,
                }))
              }
              disabled={busy}
              rows={4}
            />
          </Field>
          <DialogFooter>
            <Button type='submit' disabled={busy || !form.code || !form.name}>
              {busy ? t('Saving...') : t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function QuizQuestionDialog({
  open,
  busy,
  onOpenChange,
  t,
  initial,
  onSubmit,
}: DialogStateProps & {
  initial: QuizQuestionListItem | null
  onSubmit: (input: QuizQuestionInput) => Promise<void>
}) {
  const [form, setForm] = useState<QuizQuestionInput>(emptyQuizQuestionInput)
  const [optionsText, setOptionsText] = useState('')

  useEffect(() => {
    if (!open) return
    const next = initial ? questionToInput(initial) : emptyQuizQuestionInput
    setForm(next)
    setOptionsText(next.options.join('\n'))
  }, [initial, open])

  const options = useMemo(
    () =>
      optionsText
        .split(/\r?\n/)
        .map((option) => option.trim())
        .filter(Boolean),
    [optionsText]
  )

  const submit = (event: FormEvent) => {
    event.preventDefault()
    void onSubmit({
      ...form,
      options,
      correct_index: Math.min(form.correct_index, options.length - 1),
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[92vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>
            {initial ? t('Edit question') : t('Create question')}
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('Question details')}
          </DialogDescription>
        </DialogHeader>
        <form className='grid gap-4' onSubmit={submit}>
          <div className='grid gap-4 sm:grid-cols-2'>
            <Field id='quiz-question-key' label={t('External key')}>
              <Input
                id='quiz-question-key'
                value={form.external_key}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    external_key: event.target.value,
                  }))
                }
                disabled={busy}
              />
            </Field>
            <Field id='quiz-question-category' label={t('Category code')}>
              <Input
                id='quiz-question-category'
                value={form.category_code}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    category_code: event.target.value,
                  }))
                }
                disabled={busy}
              />
            </Field>
            <Field id='quiz-question-category-name' label={t('Category name')}>
              <Input
                id='quiz-question-category-name'
                value={form.category_name}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    category_name: event.target.value,
                  }))
                }
                disabled={busy}
              />
            </Field>
            <Field id='quiz-question-language' label={t('Language')}>
              <Input
                id='quiz-question-language'
                value={form.language}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    language: event.target.value,
                  }))
                }
                disabled={busy}
              />
            </Field>
          </div>
          <Field id='quiz-question-prompt' label={t('Question')}>
            <Textarea
              id='quiz-question-prompt'
              value={form.prompt}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  prompt: event.target.value,
                }))
              }
              disabled={busy}
              rows={3}
              required
            />
          </Field>
          <div className='grid gap-4 sm:grid-cols-[minmax(0,1fr)_220px]'>
            <Field id='quiz-question-options' label={t('Answer options')}>
              <Textarea
                id='quiz-question-options'
                value={optionsText}
                onChange={(event) => {
                  setOptionsText(event.target.value)
                  const count = event.target.value
                    .split(/\r?\n/)
                    .filter((option) => option.trim()).length
                  setForm((current) => ({
                    ...current,
                    correct_index: Math.max(
                      0,
                      Math.min(current.correct_index, count - 1)
                    ),
                  }))
                }}
                disabled={busy}
                rows={6}
                required
              />
            </Field>
            <Field id='quiz-question-correct' label={t('Correct answer')}>
              <Select
                value={String(form.correct_index)}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    correct_index: Number(value ?? 0),
                  }))
                }
                disabled={busy || options.length < 2}
              >
                <SelectTrigger id='quiz-question-correct' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {options.map((option, index) => (
                    <SelectItem
                      key={`${index}-${option}`}
                      value={String(index)}
                    >
                      {String.fromCharCode(65 + index)}. {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>
          <Field id='quiz-question-explanation' label={t('Explanation')}>
            <Textarea
              id='quiz-question-explanation'
              value={form.explanation}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  explanation: event.target.value,
                }))
              }
              disabled={busy}
              rows={3}
            />
          </Field>
          <div className='grid gap-4 sm:grid-cols-4'>
            <Field id='quiz-question-difficulty' label={t('Difficulty')}>
              <Select
                value={form.difficulty}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    difficulty: String(value ?? 'normal'),
                  }))
                }
                disabled={busy}
              >
                <SelectTrigger id='quiz-question-difficulty' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='easy'>{t('Easy')}</SelectItem>
                  <SelectItem value='normal'>{t('Normal')}</SelectItem>
                  <SelectItem value='hard'>{t('Hard')}</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field id='quiz-question-status' label={t('Status')}>
              <Select
                value={form.status}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    status: String(value ?? 'draft'),
                  }))
                }
                disabled={busy}
              >
                <SelectTrigger id='quiz-question-status' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='draft'>{t('Draft')}</SelectItem>
                  <SelectItem value='published'>{t('Published')}</SelectItem>
                  <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field id='quiz-question-weight' label={t('Weight')}>
              <Input
                id='quiz-question-weight'
                type='number'
                min={1}
                value={form.weight}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    weight: Number(event.target.value),
                  }))
                }
                disabled={busy}
              />
            </Field>
            <Field id='quiz-question-source' label={t('Source')}>
              <Input
                id='quiz-question-source'
                value={form.source}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    source: event.target.value,
                  }))
                }
                disabled={busy}
              />
            </Field>
          </div>
          <DialogFooter>
            <Button
              type='submit'
              disabled={busy || !form.prompt.trim() || options.length < 2}
            >
              {busy ? t('Saving...') : t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function parseImportQuestions(raw: string): QuizQuestionInput[] {
  const parsed: unknown = JSON.parse(raw)
  const value =
    parsed && typeof parsed === 'object' && 'questions' in parsed
      ? (parsed as { questions: unknown }).questions
      : parsed
  if (!Array.isArray(value)) throw new Error('questions must be an array')
  return value as QuizQuestionInput[]
}

export function QuizImportDialog({
  open,
  busy,
  onOpenChange,
  t,
  preview,
  onResetPreview,
  onPreview,
  onImport,
}: DialogStateProps & {
  preview: QuizImportResult | null
  onResetPreview: () => void
  onPreview: (questions: QuizQuestionInput[], publish: boolean) => Promise<void>
  onImport: (questions: QuizQuestionInput[], publish: boolean) => Promise<void>
}) {
  const [raw, setRaw] = useState('[]')
  const [publish, setPublish] = useState(false)
  const [parseError, setParseError] = useState('')

  useEffect(() => {
    if (open) {
      setRaw('[]')
      setPublish(false)
      setParseError('')
    }
  }, [open])

  const run = (mode: 'preview' | 'import') => {
    try {
      const questions = parseImportQuestions(raw)
      setParseError('')
      void (mode === 'preview'
        ? onPreview(questions, publish)
        : onImport(questions, publish))
    } catch (error) {
      setParseError(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[92vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Import questions')}</DialogTitle>
          <DialogDescription className='sr-only'>
            {t('Import quiz questions from JSON')}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-4'>
          <Field id='quiz-import-json' label={t('JSON data')}>
            <Textarea
              id='quiz-import-json'
              className='min-h-72 font-mono text-xs'
              value={raw}
              onChange={(event) => {
                setRaw(event.target.value)
                setParseError('')
                onResetPreview()
              }}
              disabled={busy}
              spellCheck={false}
            />
          </Field>
          <label className='flex items-center gap-2 text-sm'>
            <Checkbox
              checked={publish}
              onCheckedChange={(checked) => {
                setPublish(checked === true)
                onResetPreview()
              }}
              disabled={busy}
            />
            {t('Publish imported questions')}
          </label>
          {parseError ? (
            <p className='text-destructive text-sm'>{parseError}</p>
          ) : null}
          {preview ? (
            <div className='grid grid-cols-2 border-y sm:grid-cols-5'>
              {[
                [t('Received'), preview.received],
                [t('Valid'), preview.valid],
                [t('New'), preview.created],
                [t('Updated'), preview.updated],
                [t('Duplicates'), preview.skipped_duplicates],
              ].map(([label, value]) => (
                <div key={String(label)} className='px-3 py-2'>
                  <div className='text-muted-foreground text-xs'>{label}</div>
                  <div className='mt-1 text-lg font-semibold'>{value}</div>
                </div>
              ))}
            </div>
          ) : null}
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={busy}
              onClick={() => run('preview')}
            >
              {t('Validate')}
            </Button>
            <Button
              type='button'
              disabled={busy || !preview || preview.valid < 1}
              onClick={() => run('import')}
            >
              {busy ? t('Importing...') : t('Import')}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  )
}

export function QuizBindingDialog({
  open,
  busy,
  onOpenChange,
  t,
  banks,
  initial,
  onSubmit,
}: DialogStateProps & {
  banks: QuizBankListItem[]
  initial: QuizBindingListItem | null
  onSubmit: (input: QuizBindingInput) => Promise<void>
}) {
  const [bankId, setBankId] = useState(0)
  const [platform, setPlatform] = useState('qq')
  const [groupId, setGroupId] = useState('*')
  const [priority, setPriority] = useState(0)
  const [enabled, setEnabled] = useState(true)
  const [rules, setRules] = useState('{}')
  const [parseError, setParseError] = useState('')

  useEffect(() => {
    if (!open) return
    const value = initial ? bindingToInput(initial) : null
    setBankId(value?.bank_id ?? banks[0]?.bank.id ?? 0)
    setPlatform(value?.platform ?? 'qq')
    setGroupId(value?.group_id ?? '*')
    setPriority(value?.priority ?? 0)
    setEnabled(value?.enabled ?? true)
    setRules(JSON.stringify(value?.rules ?? {}, null, 2))
    setParseError('')
  }, [banks, initial, open])

  const submit = (event: FormEvent) => {
    event.preventDefault()
    try {
      const parsed: unknown = JSON.parse(rules || '{}')
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error('rules must be a JSON object')
      }
      setParseError('')
      void onSubmit({
        id: initial?.binding.id,
        bank_id: bankId,
        platform,
        group_id: groupId,
        enabled,
        priority,
        rules: parsed as Record<string, unknown>,
      })
    } catch (error) {
      setParseError(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {initial ? t('Edit binding') : t('Create binding')}
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('Quiz bank binding details')}
          </DialogDescription>
        </DialogHeader>
        <form className='grid gap-4' onSubmit={submit}>
          <div className='grid gap-4 sm:grid-cols-2'>
            <Field id='quiz-binding-bank' label={t('Quiz bank')}>
              <Select
                value={String(bankId)}
                onValueChange={(value) => setBankId(Number(value ?? 0))}
                disabled={busy}
              >
                <SelectTrigger id='quiz-binding-bank' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {banks.map((item) => (
                    <SelectItem key={item.bank.id} value={String(item.bank.id)}>
                      {item.bank.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field id='quiz-binding-platform' label={t('Platform')}>
              <Select
                value={platform}
                onValueChange={(value) => setPlatform(String(value ?? '*'))}
                disabled={busy}
              >
                <SelectTrigger id='quiz-binding-platform' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='qq'>QQ</SelectItem>
                  <SelectItem value='tg'>Telegram</SelectItem>
                  <SelectItem value='*'>{t('All platforms')}</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field id='quiz-binding-group' label={t('Group ID')}>
              <Input
                id='quiz-binding-group'
                value={groupId}
                onChange={(event) => setGroupId(event.target.value)}
                disabled={busy}
                required
              />
            </Field>
            <Field id='quiz-binding-priority' label={t('Priority')}>
              <Input
                id='quiz-binding-priority'
                type='number'
                value={priority}
                onChange={(event) => setPriority(Number(event.target.value))}
                disabled={busy}
              />
            </Field>
          </div>
          <Field id='quiz-binding-rules' label={t('Rules JSON')}>
            <Textarea
              id='quiz-binding-rules'
              className='min-h-40 font-mono text-xs'
              value={rules}
              onChange={(event) => {
                setRules(event.target.value)
                setParseError('')
              }}
              disabled={busy}
              spellCheck={false}
            />
          </Field>
          <label className='flex items-center gap-2 text-sm'>
            <Checkbox
              checked={enabled}
              onCheckedChange={(checked) => setEnabled(checked === true)}
              disabled={busy}
            />
            {t('Enabled')}
          </label>
          {parseError ? (
            <p className='text-destructive text-sm'>{parseError}</p>
          ) : null}
          <DialogFooter>
            <Button type='submit' disabled={busy || bankId < 1 || !groupId}>
              {busy ? t('Saving...') : t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function DeleteQuizBindingDialog({
  item,
  busy,
  t,
  onOpenChange,
  onConfirm,
}: {
  item: QuizBindingListItem | null
  busy: boolean
  t: OpsTranslate
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<void>
}) {
  return (
    <AlertDialog open={Boolean(item)} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Delete binding')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'This removes the quiz bank assignment for {{platform}} / {{group}}.',
              {
                platform: item?.binding.platform ?? '',
                group: item?.binding.group_id ?? '',
              }
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>{t('Cancel')}</AlertDialogCancel>
          <AlertDialogAction
            variant='destructive'
            disabled={busy}
            onClick={() => void onConfirm()}
          >
            {busy ? t('Deleting...') : t('Delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
