/*
Copyright (C) 2023-2026 QuantumNous
*/
import { ShieldAlert, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useApiKeys } from './api-keys-provider'

export function CommunityGateBanner() {
  const { t } = useTranslation()
  const {
    communityGateStatus,
    communityGateLoading,
    setCommunityGateDialogOpen,
    refreshCommunityGate,
    restoreCommunityGateKeys,
  } = useApiKeys()

  const gate = communityGateStatus?.gate
  if (!gate?.enabled) return null
  const frozenCount = communityGateStatus?.active_frozen_keys ?? 0
  if (communityGateStatus?.compliant && frozenCount <= 0) return null

  return (
    <Alert className='mb-4 border-amber-200 bg-amber-50 text-amber-950 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-100'>
      <ShieldAlert className='size-4' />
      <AlertTitle>
        {communityGateStatus?.compliant
          ? t('API keys need recovery')
          : t('Community verification required')}
      </AlertTitle>
      <AlertDescription className='mt-1 space-y-3'>
        <p>
          {communityGateStatus?.denied_message ||
            t(
              'Please bind your dc.hhhl.cc community account and join the required room before creating or enabling API keys.'
            )}
          {frozenCount > 0 &&
            t(' {{count}} key(s) are temporarily disabled.', {
              count: frozenCount,
            })}
        </p>
        <div className='flex flex-wrap gap-2'>
          <Button
            size='sm'
            onClick={() => setCommunityGateDialogOpen(true)}
            className='gap-2'
          >
            <Users className='size-4' />
            {t('Complete community verification')}
          </Button>
          {communityGateStatus?.can_restore && (
            <Button
              size='sm'
              variant='outline'
              onClick={restoreCommunityGateKeys}
            >
              {t('Restore disabled keys')}
            </Button>
          )}
          <Button
            size='sm'
            variant='ghost'
            disabled={communityGateLoading}
            onClick={() => void refreshCommunityGate(true)}
          >
            {t('Re-check')}
          </Button>
        </div>
      </AlertDescription>
    </Alert>
  )
}
