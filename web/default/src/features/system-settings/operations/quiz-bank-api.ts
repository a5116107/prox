/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'
import type {
  QuizBank,
  QuizBankInput,
  QuizBankListItem,
  QuizBinding,
  QuizBindingInput,
  QuizBindingListItem,
  QuizImportResult,
  QuizQuestion,
  QuizQuestionInput,
  QuizQuestionList,
  QuizStats,
} from './quiz-bank-model'

type ListPayload<T> = { items: T[]; count: number }

function unwrap<T>(payload: unknown): T {
  if (payload && typeof payload === 'object' && 'data' in payload) {
    return (payload as { data: T }).data
  }
  return payload as T
}

function quizPath(siteId: string): string {
  return `/api/ops/quiz/${encodeURIComponent(siteId)}`
}

export async function getQuizStats(siteId: string): Promise<QuizStats> {
  const response = await api.get(`${quizPath(siteId)}/stats`)
  return unwrap<QuizStats>(response.data)
}

export async function listQuizBanks(
  siteId: string
): Promise<QuizBankListItem[]> {
  const response = await api.get(`${quizPath(siteId)}/banks`)
  return unwrap<ListPayload<QuizBankListItem>>(response.data).items ?? []
}

export async function createQuizBank(
  siteId: string,
  input: QuizBankInput
): Promise<QuizBank> {
  const response = await api.post(`${quizPath(siteId)}/banks`, input)
  return unwrap<QuizBank>(response.data)
}

export async function updateQuizBank(
  siteId: string,
  bankId: number,
  input: QuizBankInput
): Promise<QuizBank> {
  const response = await api.put(
    `${quizPath(siteId)}/banks/${encodeURIComponent(String(bankId))}`,
    input
  )
  return unwrap<QuizBank>(response.data)
}

export async function publishQuizBank(
  siteId: string,
  bankId: number
): Promise<QuizBank> {
  const response = await api.post(
    `${quizPath(siteId)}/banks/${encodeURIComponent(String(bankId))}/publish`
  )
  return unwrap<QuizBank>(response.data)
}

export async function listQuizQuestions(
  siteId: string,
  bankId: number,
  params: { status: string; search: string; offset: number; limit: number }
): Promise<QuizQuestionList> {
  const response = await api.get(
    `${quizPath(siteId)}/banks/${encodeURIComponent(String(bankId))}/questions`,
    { params }
  )
  return unwrap<QuizQuestionList>(response.data)
}

export async function saveQuizQuestion(
  siteId: string,
  bankId: number,
  questionId: number | null,
  input: QuizQuestionInput
): Promise<QuizQuestion> {
  const base = `${quizPath(siteId)}/banks/${encodeURIComponent(String(bankId))}/questions`
  const response = questionId
    ? await api.put(`${base}/${encodeURIComponent(String(questionId))}`, input)
    : await api.post(base, input)
  return unwrap<QuizQuestion>(response.data)
}

export async function setQuizQuestionStatus(
  siteId: string,
  bankId: number,
  questionId: number,
  status: string
): Promise<void> {
  await api.post(
    `${quizPath(siteId)}/banks/${encodeURIComponent(String(bankId))}/questions/${encodeURIComponent(String(questionId))}/status`,
    { status }
  )
}

export async function importQuizQuestions(
  siteId: string,
  bankId: number,
  questions: QuizQuestionInput[],
  dryRun: boolean,
  publish: boolean
): Promise<QuizImportResult> {
  const response = await api.post(
    `${quizPath(siteId)}/banks/${encodeURIComponent(String(bankId))}/import`,
    { questions, dry_run: dryRun, publish }
  )
  return unwrap<QuizImportResult>(response.data)
}

export async function listQuizBindings(
  siteId: string
): Promise<QuizBindingListItem[]> {
  const response = await api.get(`${quizPath(siteId)}/bindings`)
  return unwrap<ListPayload<QuizBindingListItem>>(response.data).items ?? []
}

export async function saveQuizBinding(
  siteId: string,
  input: QuizBindingInput
): Promise<QuizBinding> {
  const response = await api.put(`${quizPath(siteId)}/bindings`, input)
  return unwrap<QuizBinding>(response.data)
}

export async function deleteQuizBinding(
  siteId: string,
  bindingId: number
): Promise<void> {
  await api.delete(
    `${quizPath(siteId)}/bindings/${encodeURIComponent(String(bindingId))}`
  )
}

export function quizApiError(error: unknown): string {
  if (error && typeof error === 'object' && 'response' in error) {
    const response = (error as { response?: { data?: unknown } }).response
    const payload = response?.data
    if (payload && typeof payload === 'object' && 'message' in payload) {
      return String((payload as { message: unknown }).message)
    }
  }
  return error instanceof Error ? error.message : String(error)
}
