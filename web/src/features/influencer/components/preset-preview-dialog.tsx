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

import { Trash2, Wand2 } from 'lucide-react'
import { useEffect, useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'

import { presetParamRows, resolvePresetSelection } from '../lib/presets'
import { isCharacterTier } from '../lib/taxonomy'
import type { ExplorePreset, InfluencerTaxonomy } from '../types'
import { FullscreenPreviewDialog } from './fullscreen-preview-dialog'
import { PreviewDetails } from './preview-details'

interface Props {
  preset: ExplorePreset | null
  taxonomy: InfluencerTaxonomy | null
  onClose: () => void
  onApply: (preset: ExplorePreset) => void
  isAdmin?: boolean
  onRemove?: (preset: ExplorePreset) => void
}

export function PresetPreviewDialog(props: Props) {
  const { t } = useTranslation()
  const applyBlockedId = useId()
  const [confirmRemove, setConfirmRemove] = useState(false)
  const preset = props.preset
  const taxonomy = props.taxonomy
  useEffect(() => setConfirmRemove(false), [preset?.id])
  const rows = useMemo(
    () => (preset && taxonomy ? presetParamRows(preset, taxonomy) : []),
    [preset, taxonomy]
  )
  const tags = useMemo(() => rows.flatMap((row) => row.options), [rows])
  const canApply = Boolean(
    preset && taxonomy && resolvePresetSelection(preset, taxonomy)
  )
  const canRemove = Boolean(
    preset?.published && props.isAdmin && props.onRemove
  )
  const remove = props.onRemove
  if (!preset) return null
  // Blocked-reason copy lives on the disabled control (tooltip + description),
  // not in a paragraph that changes the action row's height.
  let applyBlockedReason = ''
  if (!canApply) {
    applyBlockedReason = isCharacterTier(preset.tier)
      ? t('Preset settings are being updated. Please try again later.')
      : t('This character type cannot be reused')
  }
  return (
    <>
      <FullscreenPreviewDialog
        open
        onOpenChange={(open) => {
          if (!open) props.onClose()
        }}
        title={preset.name}
        backgroundUrl={preset.preview.path}
        media={
          <img
            src={preset.sheet.path}
            alt={preset.name}
            width={preset.sheet.width}
            height={preset.sheet.height}
            className='size-full object-contain'
          />
        }
        actions={
          <>
            {applyBlockedReason ? (
              <span id={applyBlockedId} className='sr-only'>
                {applyBlockedReason}
              </span>
            ) : null}
            <Button
              type='button'
              className='inf-cta min-h-11'
              disabled={!canApply}
              title={applyBlockedReason || undefined}
              aria-describedby={applyBlockedReason ? applyBlockedId : undefined}
              onClick={() => props.onApply(preset)}
            >
              <Wand2 className='size-4' aria-hidden='true' />
              <span>{t('Recreate')}</span>
            </Button>
            {canRemove ? (
              <Button
                type='button'
                variant='secondary'
                className='min-h-11'
                onClick={() => setConfirmRemove(true)}
              >
                <Trash2 className='size-4' aria-hidden='true' />
                <span>{t('Remove')}</span>
              </Button>
            ) : null}
          </>
        }
      >
        <PreviewDetails
          key={preset.id}
          brief={preset.brief}
          tags={tags.map((tag) => t(tag))}
        />
      </FullscreenPreviewDialog>
      {canRemove && remove ? (
        <ConfirmDialog
          open={confirmRemove}
          onOpenChange={setConfirmRemove}
          title={t('Remove from the inspiration library?')}
          desc={t(
            'Everyone loses access to this character once it is removed. This cannot be undone.'
          )}
          destructive
          confirmText={t('Remove')}
          handleConfirm={() => {
            setConfirmRemove(false)
            remove(preset)
          }}
        />
      ) : null}
    </>
  )
}
