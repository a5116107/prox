/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

export type QuizBank = {
  id: number
  site_id: string
  code: string
  name: string
  description: string
  default_language: string
  status: string
  created_by: number
  created_at: number
  updated_at: number
}

export type QuizBankListItem = {
  bank: QuizBank
  question_count: number
  published_question_count: number
}

export type QuizQuestion = {
  id: number
  bank_id: number
  category_id: number
  external_key: string
  prompt: string
  options_json: string
  correct_index: number
  explanation: string
  difficulty: string
  language: string
  status: string
  weight: number
  source: string
  content_hash: string
  created_by: number
  created_at: number
  updated_at: number
}

export type QuizCategory = {
  id: number
  bank_id: number
  code: string
  name: string
  status: string
}

export type QuizQuestionListItem = {
  question: QuizQuestion
  options: string[]
  category?: QuizCategory
}

export type QuizQuestionList = {
  items: QuizQuestionListItem[]
  total: number
  offset: number
  limit: number
}

export type QuizBinding = {
  id: number
  site_id: string
  bank_id: number
  platform: string
  group_id: string
  enabled: boolean
  priority: number
  rules_json: string
  created_at: number
  updated_at: number
}

export type QuizBindingListItem = {
  binding: QuizBinding
  bank: QuizBank
  rules: Record<string, unknown>
}

export type QuizStats = {
  site_id: string
  banks: number
  questions: number
  bindings: number
  draws: number
  open_draws: number
}

export type QuizBankInput = {
  code: string
  name: string
  description: string
  default_language: string
  status: string
}

export type QuizQuestionInput = {
  external_key: string
  category_code: string
  category_name: string
  prompt: string
  options: string[]
  correct_index: number
  explanation: string
  difficulty: string
  language: string
  status: string
  weight: number
  source: string
}

export type QuizBindingInput = {
  id?: number
  bank_id: number
  platform: string
  group_id: string
  enabled: boolean
  priority: number
  rules: Record<string, unknown>
}

export type QuizImportResult = {
  received: number
  valid: number
  created: number
  updated: number
  skipped_duplicates: number
  dry_run: boolean
}

export const emptyQuizBankInput: QuizBankInput = {
  code: '',
  name: '',
  description: '',
  default_language: 'zh-CN',
  status: 'draft',
}

export const emptyQuizQuestionInput: QuizQuestionInput = {
  external_key: '',
  category_code: 'general',
  category_name: '',
  prompt: '',
  options: ['', ''],
  correct_index: 0,
  explanation: '',
  difficulty: 'normal',
  language: 'zh-CN',
  status: 'draft',
  weight: 100,
  source: 'manual',
}

export function bankToInput(bank: QuizBank): QuizBankInput {
  return {
    code: bank.code,
    name: bank.name,
    description: bank.description,
    default_language: bank.default_language,
    status: bank.status,
  }
}

export function questionToInput(item: QuizQuestionListItem): QuizQuestionInput {
  return {
    external_key: item.question.external_key,
    category_code: item.category?.code ?? '',
    category_name: item.category?.name ?? '',
    prompt: item.question.prompt,
    options: item.options,
    correct_index: item.question.correct_index,
    explanation: item.question.explanation,
    difficulty: item.question.difficulty,
    language: item.question.language,
    status: item.question.status,
    weight: item.question.weight,
    source: item.question.source,
  }
}

export function bindingToInput(item: QuizBindingListItem): QuizBindingInput {
  return {
    id: item.binding.id,
    bank_id: item.binding.bank_id,
    platform: item.binding.platform,
    group_id: item.binding.group_id,
    enabled: item.binding.enabled,
    priority: item.binding.priority,
    rules: item.rules,
  }
}
