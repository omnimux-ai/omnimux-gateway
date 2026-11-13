import { Loader2, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

export type ReferenceFilter = 'all' | 'uploaded' | 'generated'

export function ReferencePickerTabs(props: {
  value: ReferenceFilter
  onChange: (value: ReferenceFilter) => void
}) {
  const { t } = useTranslation()
  return (
    <Tabs
      value={props.value}
      onValueChange={(value) => props.onChange(value as ReferenceFilter)}
      className='inf-reference-tabs'
    >
      <TabsList aria-label={t('Reference source')}>
        <TabsTrigger value='all'>{t('All references')}</TabsTrigger>
        <TabsTrigger value='uploaded'>{t('Uploaded')}</TabsTrigger>
        <TabsTrigger value='generated'>{t('Generated')}</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}

export function ReferenceUploadCard(props: {
  label: string
  status?: string
  /** 0–100 while the presigned PUT reports progress; omitted when unknown. */
  progress?: number
  /** Local object URL shown immediately, before the upload finishes. */
  previewSrc?: string
  disabled?: boolean
  onClick: () => void
}) {
  const progress =
    typeof props.progress === 'number' && Number.isFinite(props.progress)
      ? Math.min(100, Math.max(0, Math.round(props.progress)))
      : undefined
  return (
    <Button
      type='button'
      variant='ghost'
      disabled={props.disabled}
      onClick={props.onClick}
      className='inf-character-card inf-character-upload'
      aria-label={props.label}
      aria-busy={Boolean(props.status)}
    >
      {props.previewSrc ? (
        <>
          <img
            src={props.previewSrc}
            alt=''
            aria-hidden='true'
            className='inf-upload-preview'
          />
          <span className='inf-upload-scrim' aria-hidden='true' />
        </>
      ) : null}
      <span className='inf-upload-content'>
        {props.status ? (
          <Loader2 className='size-6 animate-spin' aria-hidden='true' />
        ) : (
          <Upload className='size-6' aria-hidden='true' />
        )}
        <span role={props.status ? 'status' : undefined}>
          {props.status || props.label}
        </span>
        {props.status && progress !== undefined ? (
          <span className='inf-upload-pct'>{progress}%</span>
        ) : null}
      </span>
      {props.status && progress !== undefined ? (
        <span className='inf-upload-progress' aria-hidden='true'>
          <i style={{ width: `${progress}%` }} />
        </span>
      ) : null}
    </Button>
  )
}
