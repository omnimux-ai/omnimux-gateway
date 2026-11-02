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
import { Images, Settings2, Wand2 } from 'lucide-react'
import { useMemo, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'

import { Switch } from '@/components/ui/switch'

import {
  MOTION_RESOLUTIONS,
  RECREATE_INTENT_MAX_RUNES,
  RECREATE_MAX_IMAGES,
  type MotionForm,
} from '../hooks/use-motion'
import { toastInfluencerError } from '../lib/error-copy'
import { getAvailableMentions } from '../lib/mention'
import { CharacterSlot } from './character-slot'
import { MediaSlot } from './media-slot'
import { MentionPromptEditor } from './mention-prompt-editor'
import { PanelCard } from './panel-card'
import { SegmentedToggle } from './segmented-toggle'

interface Props {
  motion: MotionForm
  /** Anchor for the shared reference picker on desktop. */
  assetsRef?: RefObject<HTMLElement | null>
  submitting?: boolean
  onPreviewVideo?: (url: string) => void
  onPreviewImage?: (url: string) => void
  onBeforeVideoPick?: () => boolean
}

/**
 * The recreation form: one decomposition clip, the caller's own reference
 * images, an optional intent and the output quality. Generation is the only
 * action; decomposition runs as part of it. Both inputs share the same
 * one-card asset layout as the other two motion modes.
 */
export function RecreateForm(props: Props) {
  const { t } = useTranslation()
  const motion = props.motion
  const source = motion.video
  const metadata = source?.kind !== 'preset' ? source?.metadata : undefined
  let previewUrl: string | undefined
  if (source?.kind === 'upload') previewUrl = source.url
  if (source?.kind === 'remote') previewUrl = source.previewUrl ?? source.url
  const intentPrompt = motion.recreateIntentPrompt
  // The clip is decomposition material only, so @ mentions resolve to the
  // caller's reference images and never to a video input.
  const mentionItems = useMemo(
    () =>
      getAvailableMentions({
        hasVideo: false,
        characterImages: motion.recreateImages.map((image) => image.url),
      }),
    [motion.recreateImages]
  )

  const pickVideo = async (file: File) => {
    try {
      await motion.pickVideo(file)
    } catch (error) {
      toastInfluencerError(t, error, 'Upload failed. Please try again.')
    }
  }

  return (
    <>
      <PanelCard
        panelRef={props.assetsRef}
        icon={Images}
        label={t('Assets')}
      >
        <div className='grid grid-cols-1 gap-3'>
          <MediaSlot
            kind='video'
            label={t('Add a reference video')}
            hint={t('Video duration: 4 to 30 seconds')}
            accept='video/mp4,video/quicktime'
            previewUrl={previewUrl}
            badge={metadata ? `${metadata.durationSec}s` : undefined}
            busy={motion.videoPreparing || props.submitting}
            onPreviewVideo={props.onPreviewVideo}
            onBeforePick={props.onBeforeVideoPick}
            onOpenPicker={motion.openVideoPicker}
            pickerBusy={Boolean(props.submitting)}
            onPick={(file) => void pickVideo(file)}
            onClear={motion.clearVideo}
          />
          <CharacterSlot
            label={t('Add reference images')}
            hint={t('Up to 30 reference images')}
            characters={motion.recreateImages}
            max={RECREATE_MAX_IMAGES}
            busy={props.submitting}
            onOpenPicker={motion.openCharacterPicker}
            onRemoveCharacter={motion.removeCharacter}
            onPreviewImage={props.onPreviewImage}
            onPickDirectFiles={(files) => void motion.pickCharacters(files)}
          />
        </div>
      </PanelCard>

      {/* Same prompt card as the two movement modes: a switch that reveals the
          mention-aware editor, with this mode keeping its own draft. */}
      <PanelCard
        icon={Wand2}
        label={t('Prompt')}
        contentHidden={!motion.recreateIntentPromptEnabled}
        action={
          <Switch
            size='sm'
            checked={motion.recreateIntentPromptEnabled}
            onCheckedChange={motion.setRecreateIntentPromptEnabled}
            aria-label={t('Prompt')}
          />
        }
      >
        <MentionPromptEditor
          value={intentPrompt}
          items={mentionItems}
          placeholder={t(
            'Optional: describe how to adapt the structure, e.g. keep the turning reveal and end on my product.'
          )}
          maxLength={RECREATE_INTENT_MAX_RUNES}
          onChange={motion.setRecreateIntentPrompt}
        />
      </PanelCard>

      <PanelCard icon={Settings2} label={t('Parameters')}>
        <div className='flex items-center gap-2'>
          <span className='inf-param-label'>{t('Resolution')}</span>
          <SegmentedToggle
            value={motion.resolution}
            onChange={motion.setResolution}
            options={MOTION_RESOLUTIONS.map((value) => ({
              value,
              label: value.toUpperCase(),
            }))}
            ariaLabel={t('Resolution')}
          />
        </div>
      </PanelCard>
    </>
  )
}
