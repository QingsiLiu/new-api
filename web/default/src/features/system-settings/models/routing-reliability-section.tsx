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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'Enter a non-negative number or leave empty')

const channelTestModes = ['scheduled_all', 'passive_recovery'] as const
type ChannelTestMode = (typeof channelTestModes)[number]

const routingReliabilitySchema = z
  .object({
    RetryTimes: z.coerce.number().min(0).max(10),
    ChannelDisableThreshold: numericString,
    AutomaticDisableChannelEnabled: z.boolean(),
    AutomaticEnableChannelEnabled: z.boolean(),
    AutomaticDisableKeywords: z.string(),
    AutomaticDisableStatusCodes: z.string(),
    AutomaticRetryStatusCodes: z.string(),
    AsyncFailoverEnabled: z.boolean(),
    AsyncFailoverMaxAttempts: z.coerce.number().int().min(1).max(3),
    AsyncPollTransientRetries: z.coerce.number().int().min(0).max(10),
    AsyncCircuitEnabled: z.boolean(),
    AsyncCircuitFailureThreshold: z.coerce.number().int().min(1).max(100),
    AsyncCircuitWindowSeconds: z.coerce.number().int().min(10).max(3600),
    AsyncCircuitMinimumSamples: z.coerce.number().int().min(1).max(10000),
    AsyncCircuitSuccessRateThreshold: z.coerce.number().int().min(1).max(100),
    AsyncCircuitInitialOpenSeconds: z.coerce.number().int().min(1).max(900),
    AsyncCircuitProbeLeaseSeconds: z.coerce.number().int().min(1).max(300),
    AsyncTaskAttemptRetentionDays: z.coerce.number().int().min(1).max(730),
    monitor_setting: z.object({
      auto_test_channel_enabled: z.boolean(),
      auto_test_channel_minutes: z.coerce
        .number()
        .int()
        .min(1, 'Interval must be at least 1 minute'),
      channel_test_mode: z.enum(channelTestModes),
    }),
  })
  .superRefine((values, ctx) => {
    const disableParsed = parseHttpStatusCodeRules(
      values.AutomaticDisableStatusCodes
    )
    if (!disableParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticDisableStatusCodes'],
        message: `Invalid status code rules: ${disableParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }

    const retryParsed = parseHttpStatusCodeRules(
      values.AutomaticRetryStatusCodes
    )
    if (!retryParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticRetryStatusCodes'],
        message: `Invalid status code rules: ${retryParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }
  })

type RoutingReliabilityFormValues = z.output<typeof routingReliabilitySchema>
type RoutingReliabilityFormInput = z.input<typeof routingReliabilitySchema>

type RoutingReliabilitySectionProps = {
  defaultValues: {
    RetryTimes: number
    ChannelDisableThreshold: string
    AutomaticDisableChannelEnabled: boolean
    AutomaticEnableChannelEnabled: boolean
    AutomaticDisableKeywords: string
    AutomaticDisableStatusCodes: string
    AutomaticRetryStatusCodes: string
    AsyncFailoverEnabled: boolean
    AsyncFailoverMaxAttempts: number
    AsyncPollTransientRetries: number
    AsyncCircuitEnabled: boolean
    AsyncCircuitFailureThreshold: number
    AsyncCircuitWindowSeconds: number
    AsyncCircuitMinimumSamples: number
    AsyncCircuitSuccessRateThreshold: number
    AsyncCircuitInitialOpenSeconds: number
    AsyncCircuitProbeLeaseSeconds: number
    AsyncTaskAttemptRetentionDays: number
    'monitor_setting.auto_test_channel_enabled': boolean
    'monitor_setting.auto_test_channel_minutes': number
    'monitor_setting.channel_test_mode': ChannelTestMode
  }
}

function normalizeLineEndings(value: string) {
  return value.replaceAll('\r\n', '\n')
}

type NormalizedRoutingReliabilityValues = {
  RetryTimes: number
  ChannelDisableThreshold: string
  AutomaticDisableChannelEnabled: boolean
  AutomaticEnableChannelEnabled: boolean
  AutomaticDisableKeywords: string
  AutomaticDisableStatusCodes: string
  AutomaticRetryStatusCodes: string
  AsyncFailoverEnabled: boolean
  AsyncFailoverMaxAttempts: number
  AsyncPollTransientRetries: number
  AsyncCircuitEnabled: boolean
  AsyncCircuitFailureThreshold: number
  AsyncCircuitWindowSeconds: number
  AsyncCircuitMinimumSamples: number
  AsyncCircuitSuccessRateThreshold: number
  AsyncCircuitInitialOpenSeconds: number
  AsyncCircuitProbeLeaseSeconds: number
  AsyncTaskAttemptRetentionDays: number
  'monitor_setting.auto_test_channel_enabled': boolean
  'monitor_setting.auto_test_channel_minutes': number
  'monitor_setting.channel_test_mode': ChannelTestMode
}

function normalizeChannelTestMode(value?: string): ChannelTestMode {
  return value === 'passive_recovery' ? 'passive_recovery' : 'scheduled_all'
}

const buildFormDefaults = (
  defaults: RoutingReliabilitySectionProps['defaultValues']
): RoutingReliabilityFormInput => ({
  RetryTimes: defaults.RetryTimes ?? 0,
  ChannelDisableThreshold: defaults.ChannelDisableThreshold ?? '',
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: defaults.AutomaticDisableStatusCodes ?? '',
  AutomaticRetryStatusCodes: defaults.AutomaticRetryStatusCodes ?? '',
  AsyncFailoverEnabled: defaults.AsyncFailoverEnabled ?? false,
  AsyncFailoverMaxAttempts: defaults.AsyncFailoverMaxAttempts ?? 3,
  AsyncPollTransientRetries: defaults.AsyncPollTransientRetries ?? 3,
  AsyncCircuitEnabled: defaults.AsyncCircuitEnabled ?? false,
  AsyncCircuitFailureThreshold: defaults.AsyncCircuitFailureThreshold ?? 3,
  AsyncCircuitWindowSeconds: defaults.AsyncCircuitWindowSeconds ?? 300,
  AsyncCircuitMinimumSamples: defaults.AsyncCircuitMinimumSamples ?? 10,
  AsyncCircuitSuccessRateThreshold:
    defaults.AsyncCircuitSuccessRateThreshold ?? 40,
  AsyncCircuitInitialOpenSeconds: defaults.AsyncCircuitInitialOpenSeconds ?? 60,
  AsyncCircuitProbeLeaseSeconds: defaults.AsyncCircuitProbeLeaseSeconds ?? 30,
  AsyncTaskAttemptRetentionDays: defaults.AsyncTaskAttemptRetentionDays ?? 180,
  monitor_setting: {
    auto_test_channel_enabled:
      defaults['monitor_setting.auto_test_channel_enabled'],
    auto_test_channel_minutes:
      defaults['monitor_setting.auto_test_channel_minutes'],
    channel_test_mode: normalizeChannelTestMode(
      defaults['monitor_setting.channel_test_mode']
    ),
  },
})

const normalizeDefaults = (
  defaults: RoutingReliabilitySectionProps['defaultValues']
): NormalizedRoutingReliabilityValues => ({
  RetryTimes: defaults.RetryTimes ?? 0,
  ChannelDisableThreshold: (defaults.ChannelDisableThreshold ?? '').trim(),
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticDisableStatusCodes ?? ''
  ).normalized,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticRetryStatusCodes ?? ''
  ).normalized,
  AsyncFailoverEnabled: defaults.AsyncFailoverEnabled ?? false,
  AsyncFailoverMaxAttempts: defaults.AsyncFailoverMaxAttempts ?? 3,
  AsyncPollTransientRetries: defaults.AsyncPollTransientRetries ?? 3,
  AsyncCircuitEnabled: defaults.AsyncCircuitEnabled ?? false,
  AsyncCircuitFailureThreshold: defaults.AsyncCircuitFailureThreshold ?? 3,
  AsyncCircuitWindowSeconds: defaults.AsyncCircuitWindowSeconds ?? 300,
  AsyncCircuitMinimumSamples: defaults.AsyncCircuitMinimumSamples ?? 10,
  AsyncCircuitSuccessRateThreshold:
    defaults.AsyncCircuitSuccessRateThreshold ?? 40,
  AsyncCircuitInitialOpenSeconds: defaults.AsyncCircuitInitialOpenSeconds ?? 60,
  AsyncCircuitProbeLeaseSeconds: defaults.AsyncCircuitProbeLeaseSeconds ?? 30,
  AsyncTaskAttemptRetentionDays: defaults.AsyncTaskAttemptRetentionDays ?? 180,
  'monitor_setting.auto_test_channel_enabled':
    defaults['monitor_setting.auto_test_channel_enabled'],
  'monitor_setting.auto_test_channel_minutes':
    defaults['monitor_setting.auto_test_channel_minutes'],
  'monitor_setting.channel_test_mode': normalizeChannelTestMode(
    defaults['monitor_setting.channel_test_mode']
  ),
})

const normalizeFormValues = (
  values: RoutingReliabilityFormValues
): NormalizedRoutingReliabilityValues => ({
  RetryTimes: values.RetryTimes,
  ChannelDisableThreshold: values.ChannelDisableThreshold.trim(),
  AutomaticDisableChannelEnabled: values.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: values.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    values.AutomaticDisableKeywords
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticDisableStatusCodes
  ).normalized,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticRetryStatusCodes
  ).normalized,
  AsyncFailoverEnabled: values.AsyncFailoverEnabled,
  AsyncFailoverMaxAttempts: values.AsyncFailoverMaxAttempts,
  AsyncPollTransientRetries: values.AsyncPollTransientRetries,
  AsyncCircuitEnabled: values.AsyncCircuitEnabled,
  AsyncCircuitFailureThreshold: values.AsyncCircuitFailureThreshold,
  AsyncCircuitWindowSeconds: values.AsyncCircuitWindowSeconds,
  AsyncCircuitMinimumSamples: values.AsyncCircuitMinimumSamples,
  AsyncCircuitSuccessRateThreshold: values.AsyncCircuitSuccessRateThreshold,
  AsyncCircuitInitialOpenSeconds: values.AsyncCircuitInitialOpenSeconds,
  AsyncCircuitProbeLeaseSeconds: values.AsyncCircuitProbeLeaseSeconds,
  AsyncTaskAttemptRetentionDays: values.AsyncTaskAttemptRetentionDays,
  'monitor_setting.auto_test_channel_enabled':
    values.monitor_setting.auto_test_channel_enabled,
  'monitor_setting.auto_test_channel_minutes':
    values.monitor_setting.auto_test_channel_minutes,
  'monitor_setting.channel_test_mode': values.monitor_setting.channel_test_mode,
})

export function RoutingReliabilitySection({
  defaultValues,
}: RoutingReliabilitySectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedRoutingReliabilityValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<
    RoutingReliabilityFormInput,
    unknown,
    RoutingReliabilityFormValues
  >({
    resolver: zodResolver(routingReliabilitySchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const autoDisableStatusCodes = form.watch('AutomaticDisableStatusCodes')
  const autoRetryStatusCodes = form.watch('AutomaticRetryStatusCodes')
  const channelTestMode = form.watch('monitor_setting.channel_test_mode')
  const autoDisableParsed = useMemo(
    () => parseHttpStatusCodeRules(autoDisableStatusCodes),
    [autoDisableStatusCodes]
  )
  const autoRetryParsed = useMemo(
    () => parseHttpStatusCodeRules(autoRetryStatusCodes),
    [autoRetryStatusCodes]
  )

  const onSubmit = async (values: RoutingReliabilityFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedRoutingReliabilityValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }

    baselineRef.current = normalized
  }

  return (
    <SettingsSection title={t('Routing Reliability')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <div className='flex min-w-0 flex-col gap-4'>
            <div className='flex flex-col gap-1'>
              <h4 className='text-sm font-medium'>{t('Request retry')}</h4>
            </div>
            <div className='grid min-w-0 gap-6 xl:grid-cols-[minmax(12rem,24rem)_minmax(0,1fr)]'>
              <FormField
                control={form.control}
                name='RetryTimes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Retry Times')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        max='10'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Number of times to retry failed requests (0-10)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AutomaticRetryStatusCodes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Auto-retry status codes')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. 401, 403, 429, 500-599')}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Accepts comma-separated status codes and inclusive ranges.'
                      )}{' '}
                      {autoRetryParsed.ok &&
                        autoRetryParsed.normalized &&
                        autoRetryParsed.normalized !== field.value.trim() && (
                          <span className='text-muted-foreground'>
                            {t('Normalized:')} {autoRetryParsed.normalized}
                          </span>
                        )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <Separator />

          <div className='flex min-w-0 flex-col gap-4'>
            <div className='flex flex-col gap-1'>
              <h4 className='text-sm font-medium'>
                {t('Async media failover')}
              </h4>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Retry image and video tasks on distinct channels without charging the user twice.'
                )}
              </p>
            </div>
            <div className='grid min-w-0 gap-6 lg:grid-cols-2 xl:grid-cols-3'>
              <FormField
                control={form.control}
                name='AsyncFailoverEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable async failover')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Disabled by default for observation-first rollout.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncFailoverMaxAttempts'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum distinct channels')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='3'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Total attempts, including the first channel.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncPollTransientRetries'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Poll transient retries')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='0'
                        max='10'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Retry polling on the same accepted upstream task.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Enforce temporary circuit breaker')}
                      </FormLabel>
                      <FormDescription>
                        {t('Temporarily skip unhealthy channel-model routes.')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitFailureThreshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Consecutive failure threshold')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='100'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitMinimumSamples'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum rolling samples')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='10000'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitSuccessRateThreshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum success rate (%)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='100'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitWindowSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Rolling window (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='10'
                        max='3600'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitInitialOpenSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Initial open duration (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='900'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncCircuitProbeLeaseSeconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Half-open probe lease (seconds)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='300'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AsyncTaskAttemptRetentionDays'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Attempt retention (days)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min='1'
                        max='730'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Raw provider-safe attempt audit retention.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <Separator />

          <div className='flex min-w-0 flex-col gap-4'>
            <div className='flex flex-col gap-1'>
              <h4 className='text-sm font-medium'>
                {t('Channel health checks')}
              </h4>
            </div>
            <div className='grid min-w-0 gap-6 lg:grid-cols-3'>
              <FormField
                control={form.control}
                name='monitor_setting.auto_test_channel_enabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Scheduled channel tests')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Automatically probe all channels in the background'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='monitor_setting.channel_test_mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Channel test mode')}</FormLabel>
                    <Select
                      items={[
                        {
                          value: 'scheduled_all',
                          label: t('Scheduled full test'),
                        },
                        {
                          value: 'passive_recovery',
                          label: t('Passive recovery only'),
                        },
                      ]}
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='scheduled_all'>
                            {t('Scheduled full test')}
                          </SelectItem>
                          <SelectItem value='passive_recovery'>
                            {t('Passive recovery only')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'Scheduled full test probes non-manually-disabled channels; passive recovery only checks auto-disabled channels after real request failures.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='monitor_setting.auto_test_channel_minutes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Test interval (minutes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {channelTestMode === 'passive_recovery'
                        ? t(
                            'How frequently the system checks auto-disabled channels for recovery'
                          )
                        : t('How frequently the system tests all channels')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AutomaticEnableChannelEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Re-enable on success')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Bring channels back online after successful checks'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </div>
          </div>

          <Separator />

          <div className='flex min-w-0 flex-col gap-4'>
            <div className='flex flex-col gap-1'>
              <h4 className='text-sm font-medium'>{t('Auto-disable rules')}</h4>
            </div>
            <div className='grid min-w-0 gap-6 lg:grid-cols-2'>
              <FormField
                control={form.control}
                name='AutomaticDisableChannelEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Disable on failure')}</FormLabel>
                      <FormDescription>
                        {t('Automatically disable channels when tests fail')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='ChannelDisableThreshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Disable threshold (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Automatically disable channels exceeding this response time'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AutomaticDisableStatusCodes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Auto-disable status codes')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. 401, 403, 429, 500-599')}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Accepts comma-separated status codes and inclusive ranges.'
                      )}{' '}
                      {autoDisableParsed.ok &&
                        autoDisableParsed.normalized &&
                        autoDisableParsed.normalized !== field.value.trim() && (
                          <span className='text-muted-foreground'>
                            {t('Normalized:')} {autoDisableParsed.normalized}
                          </span>
                        )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='AutomaticDisableKeywords'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Failure keywords')}</FormLabel>
                    <FormControl>
                      <Textarea
                        rows={6}
                        placeholder={t('one keyword per line')}
                        {...field}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'If an upstream error contains any of these keywords (case insensitive), the channel will be disabled automatically.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
