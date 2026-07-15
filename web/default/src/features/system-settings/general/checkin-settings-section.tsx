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
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  enabled: z.boolean(),
  minQuota: z.coerce.number().int().min(0),
  maxQuota: z.coerce.number().int().min(0),
  jumpMode: z.enum(['api', 'direct']),
  communityCheckinUrl: z.string().optional(),
  qqCheckinUrl: z.string().optional(),
  tgCheckinUrl: z.string().optional(),
})

type Values = z.infer<typeof schema>

export function CheckinSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    minQuota: number
    maxQuota: number
    jumpMode?: string
    communityCheckinUrl?: string
    qqCheckinUrl?: string
    tgCheckinUrl?: string
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      enabled: defaultValues.enabled,
      minQuota: defaultValues.minQuota,
      maxQuota: defaultValues.maxQuota,
      jumpMode: defaultValues.jumpMode === 'api' ? 'api' : 'direct',
      communityCheckinUrl: defaultValues.communityCheckinUrl || '',
      qqCheckinUrl: defaultValues.qqCheckinUrl || '',
      tgCheckinUrl: defaultValues.tgCheckinUrl || '',
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== defaultValues.enabled) {
      updates.push({
        key: 'checkin_setting.enabled',
        value: String(values.enabled),
      })
    }

    if (values.minQuota !== defaultValues.minQuota) {
      updates.push({
        key: 'checkin_setting.min_quota',
        value: String(values.minQuota),
      })
    }

    if (values.maxQuota !== defaultValues.maxQuota) {
      updates.push({
        key: 'checkin_setting.max_quota',
        value: String(values.maxQuota),
      })
    }

    if (
      values.jumpMode !== (defaultValues.jumpMode === 'api' ? 'api' : 'direct')
    ) {
      updates.push({
        key: 'checkin_setting.jump_mode',
        value: values.jumpMode,
      })
    }

    if (
      (values.communityCheckinUrl || '') !==
      (defaultValues.communityCheckinUrl || '')
    ) {
      updates.push({
        key: 'checkin_setting.community_checkin_url',
        value: values.communityCheckinUrl || '',
      })
    }

    if ((values.qqCheckinUrl || '') !== (defaultValues.qqCheckinUrl || '')) {
      updates.push({
        key: 'checkin_setting.qq_checkin_url',
        value: values.qqCheckinUrl || '',
      })
    }

    if ((values.tgCheckinUrl || '') !== (defaultValues.tgCheckinUrl || '')) {
      updates.push({
        key: 'checkin_setting.tg_checkin_url',
        value: values.tgCheckinUrl || '',
      })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset(values)
  }

  return (
    <SettingsSection title={t('Check-in Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save check-in settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable check-in feature')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to check in daily for random quota rewards'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <div className='grid gap-6 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='jumpMode'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>{t('Check-in mode')}</FormLabel>
                    <FormControl>
                      <select
                        className='border-input bg-background w-full rounded-md border px-3 py-2 text-sm'
                        value={field.value}
                        onChange={field.onChange}
                      >
                        <option value='direct'>
                          {t('Direct external jump')}
                        </option>
                        <option value='api'>
                          {t('Keep in-site check-in')}
                        </option>
                      </select>
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Direct mode sends the button to QQ/TG/community check-in entry; API mode keeps the old /api/user/checkin flow.'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='minQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('1000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Minimum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='maxQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('10000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Maximum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='communityCheckinUrl'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>
                      {t('Community check-in URL (prox primary)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='url'
                        placeholder='https://dc.hhhl.cc/chat/room/xxxx'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Primary entry for prox community check-in.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='qqCheckinUrl'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('QQ check-in URL (legacy fallback)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='url'
                        placeholder='https://dc.hhhl.cc/chat/room/ani6dktt6d'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Legacy fallback only; prox now prefers community check-in URL.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='tgCheckinUrl'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('TG check-in URL')}</FormLabel>
                    <FormControl>
                      <Input
                        type='url'
                        placeholder='https://dc.hhhl.cc/chat/room/ani62ryftx'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Used by the TG check-in button.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
