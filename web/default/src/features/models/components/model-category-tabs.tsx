import type { TFunction } from 'i18next'
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
import { TEXT_MODEL_CATEGORIES, type TextModelCategory } from '../types'

type ModelCategoryTabsProps = {
  modal: ModelModal
  textCategory: TextModelCategory
  onModalChange: (modal: ModelModal) => void
  onTextCategoryChange: (category: TextModelCategory) => void
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
    <div className='space-y-2'>
      <Tabs
        value={props.modal}
        onValueChange={(value) => props.onModalChange(value as ModelModal)}
      >
        <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
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

      {props.modal === 'text' ? (
        <Tabs
          value={props.textCategory}
          onValueChange={(value) =>
            props.onTextCategoryChange(value as TextModelCategory)
          }
        >
          <TabsList
            variant='line'
            className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'
          >
            {TEXT_MODEL_CATEGORIES.map((category) => (
              <TabsTrigger key={category} value={category}>
                {getTextCategoryLabel(category, t)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      ) : null}
    </div>
  )
}

function getTextCategoryLabel(
  category: TextModelCategory,
  t: TFunction
): string {
  if (category === 'unclassified') return t('Unclassified')
  return category.toUpperCase()
}
