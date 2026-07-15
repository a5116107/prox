/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import type { AccessControlStatus } from '@/types/access-control'
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getMyAccessControlStatus } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { AccessControlDialog } from '@/components/access-control-dialog'
import { useApiKeys } from './api-keys-provider'

export function ApiKeysPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useApiKeys()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [checking, setChecking] = useState(false)
  const [status, setStatus] = useState<AccessControlStatus | null>(null)

  const handleCreate = async () => {
    setChecking(true)
    try {
      const res = await getMyAccessControlStatus()
      const next = res.data || null
      setStatus(next)
      if (
        next?.block_token_create &&
        (next?.access_level === 'none' ||
          (Array.isArray(next?.effective_groups) &&
            next.effective_groups.length === 0))
      ) {
        setDialogOpen(true)
        return
      }
      setOpen('create')
    } finally {
      setChecking(false)
    }
  }

  return (
    <>
      <div className='flex gap-2'>
        <Button size='sm' onClick={handleCreate} disabled={checking}>
          <Plus className='h-4 w-4' />
          {checking ? t('Checking...') : t('Create API Key')}
        </Button>
      </div>
      <AccessControlDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        status={status}
        force={
          status?.access_level === 'none' ||
          (Array.isArray(status?.effective_groups) &&
            status.effective_groups.length === 0)
        }
        onStatusChanged={setStatus}
      />
    </>
  )
}
