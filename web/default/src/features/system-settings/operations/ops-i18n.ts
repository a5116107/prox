/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

type TranslateOptions = {
  defaultValue?: string
  [key: string]: unknown
}

export type OpsTranslate = (key: string, options?: TranslateOptions) => string

export function useOpsT(): OpsTranslate {
  const { t } = useTranslation()

  return useCallback(
    (key: string, options?: TranslateOptions) =>
      t(key, {
        ...(options ?? {}),
        defaultValue: options?.defaultValue ?? key,
      }),
    [t]
  )
}
