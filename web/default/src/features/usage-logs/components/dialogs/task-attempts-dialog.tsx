import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { formatTimestampToDate } from '@/lib/format'

import { getAdminTaskAttempts } from '../../api'
import type { AsyncTaskAttemptLog } from '../../types'

type TaskAttemptsDialogProps = {
  taskId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

function attemptVariant(attempt: AsyncTaskAttemptLog) {
  if (attempt.status === 'succeeded') return 'green' as const
  if (attempt.retryable) return 'yellow' as const
  return 'red' as const
}

export function TaskAttemptsDialog({
  taskId,
  open,
  onOpenChange,
}: TaskAttemptsDialogProps) {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['task-attempts', taskId],
    queryFn: () => getAdminTaskAttempts(taskId),
    enabled: open && Boolean(taskId),
  })
  const attempts = (data?.data ?? []) as AsyncTaskAttemptLog[]

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Async channel attempts')}
      description={taskId}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
    >
      <ScrollArea className='max-h-[560px] pr-3'>
        <div className='space-y-3 py-3'>
          {isLoading && (
            <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
          )}
          {isError && (
            <p className='text-destructive text-sm'>
              {t('Failed to load task attempts')}
            </p>
          )}
          {!isLoading && !isError && attempts.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t('No attempt records')}
            </p>
          )}
          {attempts.map((attempt) => (
            <div
              key={attempt.id}
              className='border-border bg-muted/20 rounded-lg border p-3'
            >
              <div className='flex flex-wrap items-center gap-2'>
                <span className='font-medium'>
                  Attempt {attempt.attempt_no}
                </span>
                <StatusBadge
                  label={attempt.status}
                  variant={attemptVariant(attempt)}
                  size='sm'
                  copyable={false}
                />
                <span className='text-muted-foreground font-mono text-xs'>
                  channel #{attempt.channel_id}
                </span>
              </div>
              <div className='text-muted-foreground mt-2 grid gap-1 text-xs sm:grid-cols-2'>
                <span>
                  {attempt.model} · {attempt.kind}/{attempt.action}
                </span>
                <span>
                  {attempt.duration_ms ?? 0} ms · polls{' '}
                  {attempt.poll_count ?? 0}
                </span>
                <span>
                  {attempt.stage || '-'} · {attempt.acceptance_state}
                </span>
                <span>
                  {formatTimestampToDate(attempt.started_at, 'seconds')}
                </span>
                {attempt.failure_class && (
                  <span className='text-destructive sm:col-span-2'>
                    {attempt.failure_class}
                    {attempt.http_status
                      ? ` · HTTP ${attempt.http_status}`
                      : ''}
                    {attempt.provider_code ? ` · ${attempt.provider_code}` : ''}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </Dialog>
  )
}
