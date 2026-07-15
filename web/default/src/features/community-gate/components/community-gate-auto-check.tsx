/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getCommunityGateMe, restoreCommunityGateSelf } from '../api'
import { maybeAutoOpenCommunityGate } from '../lib'
import type { CommunityGateMeStatus } from '../types'
import { CommunityGateDialog } from './community-gate-dialog'

export function CommunityGateAutoCheck() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const [status, setStatus] = useState<CommunityGateMeStatus | null>(null)
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const checkedRef = useRef<number | null>(null)

  const refresh = useCallback(
    async (force = false) => {
      setLoading(true)
      try {
        const res = await getCommunityGateMe(force)
        if (res.success && res.data) {
          setStatus(res.data)
          const autoOpened = maybeAutoOpenCommunityGate(res.data, user?.id)
          if (
            res.data.gate?.enabled &&
            (!res.data.compliant || res.data.has_active_frozen_keys) &&
            !autoOpened
          ) {
            setOpen(true)
          }
          return res.data
        }
        return null
      } finally {
        setLoading(false)
      }
    },
    [user?.id]
  )

  const restore = useCallback(async () => {
    const res = await restoreCommunityGateSelf()
    if (res.success) {
      toast.success(t('Disabled API keys restored'))
      await refresh(true)
    } else {
      toast.error(res.message || t('Failed to restore disabled keys'))
    }
  }, [refresh, t])

  useEffect(() => {
    if (!user?.id || checkedRef.current === user.id) return
    checkedRef.current = user.id
    void refresh(false)
  }, [refresh, user?.id])

  return (
    <CommunityGateDialog
      open={open}
      onOpenChange={setOpen}
      status={status}
      loading={loading}
      onRefresh={refresh}
      onRestore={restore}
    />
  )
}
