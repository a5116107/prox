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
import { useEffect, useMemo, useState } from 'react'
import * as z from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Edit, Trash2, Save, MessageCircle, Send } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
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
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { StaticDataTable } from '@/components/data-table'
import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type Announcement = {
  id: number
  content: string
  publishDate: string
  type: 'default' | 'ongoing' | 'success' | 'warning' | 'error'
  extra?: string
  forcePopup?: boolean
  communityType?: string
  communityValue?: string
  communityLink?: string
  qqGroup?: string
  qqGroupLink?: string
  tgGroup?: string
  tgGroupLink?: string
  telegram?: string
  telegramLink?: string
}

type AnnouncementsSectionProps = {
  enabled: boolean
  data: string
}

const announcementSchema = z.object({
  content: z
    .string()
    .min(1, 'Content is required')
    .max(500, 'Content must be less than 500 characters'),
  publishDate: z.string().min(1, 'Publish date is required'),
  type: z.enum(['default', 'ongoing', 'success', 'warning', 'error']),
  forcePopup: z.boolean().optional().default(true),
  communityType: z.enum(['qq', 'tg']).optional().default('qq'),
  communityValue: z
    .string()
    .max(300, 'Community value must be less than 300 characters')
    .optional(),
  communityLink: z
    .string()
    .max(300, 'Community link must be less than 300 characters')
    .optional(),
  qqGroup: z
    .string()
    .max(20, 'QQ group number must be less than 20 characters')
    .optional(),
  qqGroupLink: z
    .string()
    .max(300, 'QQ group link must be less than 300 characters')
    .optional(),
  tgGroup: z
    .string()
    .max(300, 'Telegram group must be less than 300 characters')
    .optional(),
  tgGroupLink: z
    .string()
    .max(300, 'Telegram link must be less than 300 characters')
    .optional(),
  extra: z
    .string()
    .max(100, 'Extra must be less than 100 characters')
    .optional(),
})

type AnnouncementFormValues = z.infer<typeof announcementSchema>

const SITE_DEFAULT_COMMUNITY = {
  forcePopup: true,
  communityType: 'qq' as const,
  communityValue: '925249987',
  communityLink: '',
}

function getDefaultCommunityFields(
  communityType: 'qq' | 'tg' = SITE_DEFAULT_COMMUNITY.communityType
) {
  const type = normalizeCommunityType(communityType)
  const useSiteDefault = type === SITE_DEFAULT_COMMUNITY.communityType
  const value = useSiteDefault ? SITE_DEFAULT_COMMUNITY.communityValue : ''
  const link = useSiteDefault ? SITE_DEFAULT_COMMUNITY.communityLink : ''

  return {
    forcePopup: true,
    communityType: type,
    communityValue: value,
    communityLink: link,
    qqGroup: type === 'qq' ? value : '',
    qqGroupLink: type === 'qq' ? link : '',
    tgGroup: type === 'tg' ? value : '',
    tgGroupLink: type === 'tg' ? link : '',
  }
}

function createDefaultAnnouncementForm(): AnnouncementFormValues {
  return {
    content: '',
    publishDate: new Date().toISOString(),
    type: 'default',
    ...getDefaultCommunityFields(),
    extra: '',
  }
}

const ANNOUNCEMENT_FORM_ID = 'announcement-form'

const typeOptions = [
  {
    value: 'default',
    label: 'Default',
    color: 'bg-gray-500',
    badgeVariant: 'neutral' as const,
  },
  {
    value: 'ongoing',
    label: 'Ongoing',
    color: 'bg-blue-500',
    badgeVariant: 'info' as const,
  },
  {
    value: 'success',
    label: 'Success',
    color: 'bg-green-500',
    badgeVariant: 'success' as const,
  },
  {
    value: 'warning',
    label: 'Warning',
    color: 'bg-orange-500',
    badgeVariant: 'warning' as const,
  },
  {
    value: 'error',
    label: 'Error',
    color: 'bg-red-500',
    badgeVariant: 'danger' as const,
  },
]

const communityTypeOptions = [
  { value: 'qq', label: 'QQ群', icon: MessageCircle },
  { value: 'tg', label: 'Telegram', icon: Send },
]

function extractQqGroup(text = ''): string {
  return (
    String(text || '').match(
      /(?:Q群|QQ群|qq群|qq\s*群|QQ\s*群|qq|QQ)\s*[：: ]\s*(\d{5,12})/i
    )?.[1] || ''
  )
}

function normalizeCommunityType(value: unknown): 'qq' | 'tg' {
  const normalized = String(value || '').toLowerCase()
  return normalized === 'tg' || normalized === 'telegram' ? 'tg' : 'qq'
}

function getCommunityInfo(announcement: Partial<Announcement> = {}) {
  const inferredType =
    announcement.tgGroup || announcement.telegram || announcement.telegramLink
      ? 'tg'
      : 'qq'
  const type = normalizeCommunityType(
    announcement.communityType || inferredType
  )
  const value =
    announcement.communityValue ||
    (type === 'tg'
      ? announcement.tgGroup || announcement.telegram || ''
      : announcement.qqGroup || extractQqGroup(announcement.content) || '')
  const link =
    announcement.communityLink ||
    (type === 'tg'
      ? announcement.tgGroupLink || announcement.telegramLink || ''
      : announcement.qqGroupLink || '')
  return { type, value, link }
}

export function AnnouncementsSection({
  enabled,
  data,
}: AnnouncementsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [announcements, setAnnouncements] = useState<Announcement[]>([])
  const [isEnabled, setIsEnabled] = useState(enabled)
  const [hasChanges, setHasChanges] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingAnnouncement, setEditingAnnouncement] =
    useState<Announcement | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<'single' | 'batch'>('single')

  const form = useForm<AnnouncementFormValues>({
    resolver: zodResolver(
      announcementSchema
    ) as unknown as Resolver<AnnouncementFormValues>,
    defaultValues: createDefaultAnnouncementForm(),
  })

  useEffect(() => {
    try {
      const parsed = JSON.parse(data || '[]')
      if (Array.isArray(parsed)) {
        setAnnouncements(
          parsed.map((item, idx) => ({
            ...item,
            id: item.id || idx + 1,
          }))
        )
      }
    } catch {
      setAnnouncements([])
    }
  }, [data])

  useEffect(() => {
    setIsEnabled(enabled)
  }, [enabled])

  const handleToggleEnabled = async (checked: boolean) => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.announcements_enabled',
        value: checked,
      })
      setIsEnabled(checked)
      toast.success(t('Setting saved'))
    } catch {
      toast.error(t('Failed to update setting'))
    }
  }

  const handleAdd = () => {
    setEditingAnnouncement(null)
    form.reset(createDefaultAnnouncementForm())
    setShowDialog(true)
  }

  const handleEdit = (announcement: Announcement) => {
    setEditingAnnouncement(announcement)
    form.reset({
      content: announcement.content,
      publishDate: announcement.publishDate,
      type: announcement.type,
      forcePopup: announcement.forcePopup === true,
      communityType: getCommunityInfo(announcement).type,
      communityValue: getCommunityInfo(announcement).value,
      communityLink: getCommunityInfo(announcement).link,
      qqGroup: announcement.qqGroup || '',
      qqGroupLink: announcement.qqGroupLink || '',
      tgGroup: announcement.tgGroup || announcement.telegram || '',
      tgGroupLink: announcement.tgGroupLink || announcement.telegramLink || '',
      extra: announcement.extra || '',
    })
    setShowDialog(true)
  }

  const handleDelete = (announcement: Announcement) => {
    setEditingAnnouncement(announcement)
    setDeleteTarget('single')
    setShowDeleteDialog(true)
  }

  const handleBatchDelete = () => {
    if (selectedIds.length === 0) {
      toast.error(t('Please select items to delete'))
      return
    }
    setDeleteTarget('batch')
    setShowDeleteDialog(true)
  }

  const confirmDelete = () => {
    if (deleteTarget === 'single' && editingAnnouncement) {
      setAnnouncements((prev) =>
        prev.filter((item) => item.id !== editingAnnouncement.id)
      )
      setHasChanges(true)
      toast.success(t('Announcement deleted. Click "Save Settings" to apply.'))
    } else if (deleteTarget === 'batch') {
      setAnnouncements((prev) =>
        prev.filter((item) => !selectedIds.includes(item.id))
      )
      setSelectedIds([])
      setHasChanges(true)
      toast.success(
        t('{{count}} announcements deleted. Click "Save Settings" to apply.', {
          count: selectedIds.length,
        })
      )
    }
    setShowDeleteDialog(false)
    setEditingAnnouncement(null)
  }

  const handleSubmitForm = (values: AnnouncementFormValues) => {
    const communityType = normalizeCommunityType(values.communityType)
    const defaultCommunity = getDefaultCommunityFields(communityType)
    const communityValue =
      values.communityValue?.trim() || defaultCommunity.communityValue
    const communityLink =
      values.communityLink?.trim() || defaultCommunity.communityLink
    const normalizedValues = {
      ...values,
      forcePopup: !!values.forcePopup,
      communityType,
      communityValue,
      communityLink,
      // Keep legacy QQ fields for older frontend/runtime compatibility.
      qqGroup: communityType === 'qq' ? communityValue : '',
      qqGroupLink: communityType === 'qq' ? communityLink : '',
      tgGroup: communityType === 'tg' ? communityValue : '',
      tgGroupLink: communityType === 'tg' ? communityLink : '',
    }

    if (editingAnnouncement) {
      setAnnouncements((prev) =>
        prev.map((item) =>
          item.id === editingAnnouncement.id
            ? { ...item, ...normalizedValues }
            : item
        )
      )
      toast.success(t('Announcement updated. Click "Save Settings" to apply.'))
    } else {
      const newId = Math.max(...announcements.map((item) => item.id), 0) + 1
      setAnnouncements((prev) => [...prev, { id: newId, ...normalizedValues }])
      toast.success(t('Announcement added. Click "Save Settings" to apply.'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleSaveAll = async () => {
    try {
      await updateOption.mutateAsync({
        key: 'console_setting.announcements',
        value: JSON.stringify(announcements),
      })
      setHasChanges(false)
      toast.success(t('Announcements saved successfully'))
    } catch {
      toast.error(t('Failed to save announcements'))
    }
  }

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? announcements.map((item) => item.id) : [])
  }

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id)
    )
  }

  const sortedAnnouncements = useMemo(() => {
    return [...announcements].sort((a, b) => {
      return (
        new Date(b.publishDate).getTime() - new Date(a.publishDate).getTime()
      )
    })
  }, [announcements])

  const getRelativeTime = (date: string) => {
    const now = new Date()
    const past = new Date(date)
    const diffMs = now.getTime() - past.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    const diffHours = Math.floor(diffMins / 60)
    const diffDays = Math.floor(diffHours / 24)

    if (diffMins < 60) return `${diffMins}m ago`
    if (diffHours < 24) return `${diffHours}h ago`
    return `${diffDays}d ago`
  }

  const selectedCommunityType = normalizeCommunityType(
    form.watch('communityType')
  )

  return (
    <SettingsSection title={t('Announcements')}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add Announcement')}
            </Button>
            <Button
              onClick={handleBatchDelete}
              size='sm'
              variant='destructive'
              disabled={selectedIds.length === 0}
            >
              <Trash2 className='mr-2 h-4 w-4' />
              {t('Delete (')}
              {selectedIds.length})
            </Button>
            <Button
              onClick={handleSaveAll}
              size='sm'
              variant='secondary'
              disabled={!hasChanges || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
          </div>
          <SettingsSwitchField
            checked={isEnabled}
            onCheckedChange={handleToggleEnabled}
            label={t('Enabled')}
            className='border-b-0 py-0'
          />
        </div>

        <StaticDataTable
          data={sortedAnnouncements}
          getRowKey={(announcement) => announcement.id}
          emptyContent={t(
            'No announcements yet. Click "Add Announcement" to create one.'
          )}
          columns={[
            {
              id: 'select',
              header: (
                <Checkbox
                  checked={
                    selectedIds.length === announcements.length &&
                    announcements.length > 0
                  }
                  onCheckedChange={toggleSelectAll}
                />
              ),
              className: 'w-12',
              cell: (announcement) => (
                <Checkbox
                  checked={selectedIds.includes(announcement.id)}
                  onCheckedChange={(checked) =>
                    toggleSelectOne(announcement.id, checked as boolean)
                  }
                />
              ),
            },
            {
              id: 'content',
              header: t('Content'),
              cellClassName: 'max-w-xs truncate',
              cell: (announcement) => announcement.content,
            },
            {
              id: 'publish-date',
              header: t('Publish Date'),
              cell: (announcement) => (
                <div className='flex flex-col gap-1'>
                  <span className='text-sm font-medium'>
                    {getRelativeTime(announcement.publishDate)}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {dayjs(announcement.publishDate).format(
                      'YYYY-MM-DD HH:mm:ss'
                    )}
                  </span>
                </div>
              ),
            },
            {
              id: 'type',
              header: t('Type'),
              cell: (announcement) => (
                <StatusBadge
                  label={
                    typeOptions.find((opt) => opt.value === announcement.type)
                      ?.label
                  }
                  variant={
                    typeOptions.find((opt) => opt.value === announcement.type)
                      ?.badgeVariant ?? 'neutral'
                  }
                  copyable={false}
                />
              ),
            },
            {
              id: 'force-popup',
              header: t('Force Popup'),
              cell: (announcement) => (
                <StatusBadge
                  label={announcement.forcePopup ? t('Yes') : t('No')}
                  variant={announcement.forcePopup ? 'danger' : 'neutral'}
                  copyable={false}
                />
              ),
            },
            {
              id: 'community',
              header: t('Community'),
              cellClassName: 'text-muted-foreground max-w-xs truncate',
              cell: (announcement) => {
                const info = getCommunityInfo(announcement)
                return info.value || info.link
                  ? `${info.type === 'tg' ? 'TG' : 'QQ'} ${info.value || t('Link configured')}`
                  : '-'
              },
            },
            {
              id: 'extra',
              header: t('Extra'),
              cellClassName: 'text-muted-foreground max-w-xs truncate',
              cell: (announcement) => announcement.extra || '-',
            },
            {
              id: 'actions',
              header: t('Actions'),
              className: 'w-32',
              cell: (announcement) => (
                <div className='flex gap-2'>
                  <Button
                    onClick={() => handleEdit(announcement)}
                    size='sm'
                    variant='ghost'
                  >
                    <Edit className='h-4 w-4' />
                  </Button>
                  <Button
                    onClick={() => handleDelete(announcement)}
                    size='sm'
                    variant='ghost'
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>
              ),
            },
          ]}
        />
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={setShowDialog}
        title={
          editingAnnouncement ? t('Edit Announcement') : t('Add Announcement')
        }
        description={t(
          'Create or update system announcements for the dashboard'
        )}
        contentClassName='max-w-2xl'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => setShowDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit' form={ANNOUNCEMENT_FORM_ID}>
              {editingAnnouncement ? t('Update') : t('Add')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={ANNOUNCEMENT_FORM_ID}
            onSubmit={form.handleSubmit(handleSubmitForm)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='content'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Content')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t(
                        'Enter announcement content (supports Markdown/HTML)'
                      )}
                      rows={4}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Maximum 500 characters. Supports Markdown and HTML.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='publishDate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Publish Date')}</FormLabel>
                  <FormControl>
                    <DateTimePicker
                      value={field.value ? new Date(field.value) : undefined}
                      onChange={(date) =>
                        field.onChange(date ? date.toISOString() : '')
                      }
                      placeholder={t('Select publish date')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Date and time when this announcement should be displayed'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Type')}</FormLabel>
                  <Select
                    items={[
                      ...typeOptions.map((option) => ({
                        value: option.value,
                        label: (
                          <div className='flex items-center gap-2'>
                            <div
                              className={`h-3 w-3 rounded-full ${option.color}`}
                            />
                            {option.label}
                          </div>
                        ),
                      })),
                    ]}
                    onValueChange={field.onChange}
                    value={field.value}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue
                          placeholder={t('Select announcement type')}
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {typeOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            <div className='flex items-center gap-2'>
                              <div
                                className={`h-3 w-3 rounded-full ${option.color}`}
                              />
                              {option.label}
                            </div>
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='forcePopup'
              render={({ field }) => (
                <FormItem className='flex flex-row items-start gap-3 rounded-md border p-3'>
                  <FormControl>
                    <Checkbox
                      checked={!!field.value}
                      onCheckedChange={(checked) =>
                        field.onChange(checked === true)
                      }
                    />
                  </FormControl>
                  <div className='space-y-1 leading-none'>
                    <FormLabel>{t('Force Popup')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Ignore Close Today and auto popup again when this announcement changes.'
                      )}
                    </FormDescription>
                  </div>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='communityType'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>加群类型</FormLabel>
                  <Select
                    items={communityTypeOptions.map((option) => ({
                      value: option.value,
                      label: option.label,
                    }))}
                    onValueChange={(value) => {
                      field.onChange(value)
                      const defaultCommunity = getDefaultCommunityFields(
                        value as 'qq' | 'tg'
                      )
                      form.setValue('forcePopup', defaultCommunity.forcePopup)
                      form.setValue(
                        'communityValue',
                        defaultCommunity.communityValue
                      )
                      form.setValue(
                        'communityLink',
                        defaultCommunity.communityLink
                      )
                      form.setValue('qqGroup', defaultCommunity.qqGroup)
                      form.setValue('qqGroupLink', defaultCommunity.qqGroupLink)
                      form.setValue('tgGroup', defaultCommunity.tgGroup)
                      form.setValue('tgGroupLink', defaultCommunity.tgGroupLink)
                    }}
                    value={field.value || 'qq'}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder='选择加群类型' />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {communityTypeOptions.map((option) => {
                          const Icon = option.icon
                          return (
                            <SelectItem key={option.value} value={option.value}>
                              <div className='flex items-center gap-2'>
                                <Icon className='h-4 w-4' />
                                {option.label}
                              </div>
                            </SelectItem>
                          )
                        })}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    选择此公告前台展示 QQ 加群按钮还是 Telegram 加群按钮。
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='communityValue'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {selectedCommunityType === 'tg'
                      ? 'Telegram 群/频道'
                      : 'QQ群号'}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder={
                        selectedCommunityType === 'tg'
                          ? 'your_group or https://t.me/your_group'
                          : '925249987'
                      }
                      {...field}
                      value={field.value || ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {selectedCommunityType === 'tg'
                      ? '可选。可填写 @name、name 或 https://t.me/xxx，前台会生成 TG 加群按钮。'
                      : '可选。填写群号后，前台会生成一键 QQ 加群按钮。'}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='communityLink'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {selectedCommunityType === 'tg'
                      ? 'Telegram 链接'
                      : 'QQ 加群链接'}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder={
                        selectedCommunityType === 'tg'
                          ? 'https://t.me/your_group'
                          : 'https://qm.qq.com/...'
                      }
                      {...field}
                      value={field.value || ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {selectedCommunityType === 'tg'
                      ? '可选。填写后优先使用这个 Telegram 链接。'
                      : '可选。填写 qm.qq.com 官方链接时优先使用该链接。'}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='extra'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Extra Notes (Optional)')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Additional information')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional supplementary information (max 100 characters)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget === 'single'
                ? 'This announcement will be removed from the list.'
                : `${selectedIds.length} announcements will be removed from the list.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
