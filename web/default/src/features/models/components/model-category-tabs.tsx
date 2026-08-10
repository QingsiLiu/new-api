import {
  AudioLines,
  Image as ImageIcon,
  MessageSquareText,
  Video,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { ModelModal } from '../lib'

type ModelCategoryTabsProps = {
  modal: ModelModal
  onModalChange: (modal: ModelModal) => void
}

const MODAL_ITEMS = [
  { value: 'text', labelKey: 'Text models', icon: MessageSquareText },
  { value: 'image', labelKey: 'Image models', icon: ImageIcon },
  { value: 'video', labelKey: 'Video models', icon: Video },
  { value: 'audio', labelKey: 'Audio models', icon: AudioLines },
] as const

export function ModelCategoryTabs(props: ModelCategoryTabsProps) {
  const { t } = useTranslation()

  return (
    <Tabs
      value={props.modal}
      onValueChange={(value) => props.onModalChange(value as ModelModal)}
    >
      <TabsList className='!h-auto max-w-full flex-wrap justify-start'>
        {MODAL_ITEMS.map((item) => {
          const Icon = item.icon
          return (
            <TabsTrigger
              key={item.value}
              value={item.value}
              className={
                item.value === 'audio'
                  ? 'ml-1 opacity-60 data-active:opacity-100'
                  : undefined
              }
            >
              <Icon aria-hidden='true' />
              {t(item.labelKey)}
              {item.value === 'audio' ? (
                <Badge variant='outline' className='ml-1 px-1.5'>
                  {t('Compatible')}
                </Badge>
              ) : null}
            </TabsTrigger>
          )
        })}
      </TabsList>
    </Tabs>
  )
}
