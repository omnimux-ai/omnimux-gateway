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
import { Images, Palette, Settings2, Wand2 } from 'lucide-react'
import {
  useEffect,
  useMemo,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Switch } from '@/components/ui/switch'

import { fetchMotionOptions } from '../api'
import { MOTION_RESOLUTIONS, type MotionForm } from '../hooks/use-motion'
import { toastInfluencerError } from '../lib/error-copy'
import { getAvailableMentions } from '../lib/mention'
import { MOTION_MODES, presetsForMode } from '../lib/motion'
import { MOTION_PRESETS } from '../lib/motion-presets'
import { MOTION_VISUAL_STYLES } from '../lib/motion-visual-styles'
import { motionVideoCaption } from '../lib/video-caption'
import type { MotionAspectRatio, MotionMode } from '../types'
import { CharacterSlot } from './character-slot'
import { MediaSlot } from './media-slot'
import { MentionPromptEditor } from './mention-prompt-editor'
import { MotionStyleDialog } from './motion-style-dialog'
import { MotionStyleRow } from './motion-style-row'
import { PanelCard } from './panel-card'
import { RecreatePanel } from './recreate-panel'
import { SegmentedToggle } from './segmented-toggle'

/**
 * The modes read differently to the person filling the form even though the
 * controls are shared, so the wording around the video slot is what changes:
 * a pose source, a scene to keep, or a clip that is only decomposed. Values are
 * i18n keys resolved through `t()` here, and registered in
 * `src/i18n/static-keys.ts`.
 */
const MODE_COPY: Record<
  MotionMode,
  {
    switchLabel: string
    /** Compact label for the three-up switch; the full name stays the
     * accessible name so screen readers and tooltips keep the meaning. */
    switchLabelCompact: string
    preview: string
    title: string
    description: string
    videoLabel: string
    videoHint: string
    placeholder: string
  }
> = {
  motion: {
    switchLabel: 'Motion transfer',
    switchLabelCompact: 'Motion transfer short',
    preview: '/images/influencer/motion-transfer-preview.webp',
    title: 'Motion Control',
    description: 'Keep the motion. Reimagine the character and scene.',
    videoLabel: 'Add a reference video to extract motion',
    videoHint: 'Video duration: 4 to 30 seconds',
    placeholder:
      'Optional: what to change in the scene, props, motion, style or sound. Leave empty for a same-style remake…',
  },
  replace: {
    switchLabel: 'Object swap',
    switchLabelCompact: 'Object swap short',
    preview: '/images/influencer/object-swap-preview.webp',
    title: 'Object Swap',
    description: 'Replace only what you choose. Keep the scene and motion.',
    videoLabel: 'Add a reference video to extract motion',
    videoHint: 'Video duration: 4 to 30 seconds',
    placeholder: 'Describe the scene to keep…',
  },
  recreate: {
    switchLabel: 'Viral recreate',
    switchLabelCompact: 'Viral recreate short',
    preview: '/images/influencer/recreate-preview.webp',
    title: 'Viral Recreate',
    description:
      'Decompose a viral clip into beats, then generate with your own product.',
    videoLabel: 'Add a reference recreate video',
    videoHint: 'Video duration: 4 to 15 seconds',
    placeholder:
      'Optional: describe how to adapt the structure to your own product…',
  },
}

/** Icons stay inline so all three segments share one visual weight. */
const MODE_ICONS: Record<MotionMode, ReactNode> = {
  motion: (
    <svg
      width='24'
      height='24'
      viewBox='0 0 24 24'
      fill='none'
      xmlns='http://www.w3.org/2000/svg'
      className='size-4 shrink-0'
      aria-hidden='true'
    >
      <path
        stroke='currentColor'
        strokeWidth='1.5'
        d='M15 19.25a7.25 7.25 0 1 0 0-14.5 7.25 7.25 0 0 0 0 14.5Z'
      />
      <path
        stroke='currentColor'
        strokeDasharray='0.01 2.5'
        strokeLinecap='round'
        strokeWidth='1.5'
        d='M9 19.25a7.25 7.25 0 1 0 0-14.5 7.25 7.25 0 0 0 0 14.5Z'
      />
    </svg>
  ),
  replace: (
    <svg
      width='24'
      height='24'
      viewBox='0 0 24 24'
      fill='none'
      xmlns='http://www.w3.org/2000/svg'
      className='size-4 shrink-0'
      aria-hidden='true'
    >
      <path
        stroke='currentColor'
        strokeLinecap='round'
        strokeLinejoin='round'
        strokeWidth='1.5'
        d='M12.25 8.75h2a1 1 0 0 1 1 1v2m-6.5-3h-5a1 1 0 0 0-1 1v10.5a1 1 0 0 0 1 1h10.5a1 1 0 0 0 1-1v-5m-6.5-11.5a1 1 0 0 1 1-1h10.5a1 1 0 0 1 1 1v10.5a1 1 0 0 1-1 1H9.75a1 1 0 0 1-1-1z'
      />
    </svg>
  ),
  recreate: (
    <svg
      width='24'
      height='24'
      viewBox='0 0 24 24'
      fill='none'
      xmlns='http://www.w3.org/2000/svg'
      className='size-4 shrink-0'
      aria-hidden='true'
    >
      <path
        stroke='currentColor'
        strokeLinecap='round'
        strokeLinejoin='round'
        strokeWidth='1.5'
        d='M11 16.15V18.85C11 21.1 10.1 22 7.85 22H5.15C2.9 22 2 21.1 2 18.85V16.15C2 13.9 2.9 13 5.15 13H7.85C10.1 13 11 13.9 11 16.15Z'
      />
      <path
        stroke='currentColor'
        strokeLinecap='round'
        strokeLinejoin='round'
        strokeWidth='1.5'
        d='M22 15C22 18.87 18.87 22 15 22L16.05 20.25'
      />
      <path
        stroke='currentColor'
        strokeLinecap='round'
        strokeLinejoin='round'
        strokeWidth='1.5'
        d='M2 9C2 5.13 5.13 2 9 2L7.95 3.75'
      />
      <path
        stroke='currentColor'
        strokeLinecap='round'
        strokeLinejoin='round'
        strokeWidth='1.5'
        d='M17.5 11C19.9853 11 22 8.98528 22 6.5C22 4.01472 19.9853 2 17.5 2C15.0147 2 13 4.01472 13 6.5C13 8.98528 15.0147 11 17.5 11Z'
      />
    </svg>
  ),
}

interface Props {
  assetsRef?: RefObject<HTMLElement | null>
  motion: MotionForm
  onBeforeVideoPick?: () => boolean
  /** Opens a selected reference in the shared playback window. */
  onPreviewVideo?: (url: string) => void
  /** Opens a selected image in the same shared preview window. */
  onPreviewImage?: (url: string) => void
  submitting?: boolean
}

/** Video controls over the existing motion form and upload state. */
export function MotionPanel(props: Props) {
  const { t } = useTranslation()
  const motion = props.motion
  const copy = MODE_COPY[motion.mode]
  const [stylePickerOpen, setStylePickerOpen] = useState(false)
  const setOutputCapabilities = motion.setOutputCapabilities
  useEffect(() => {
    let active = true
    const load =
      typeof fetchMotionOptions === 'function'
        ? fetchMotionOptions()
        : Promise.resolve(null)
    void Promise.resolve(load)
      .then((options) => {
        if (!active || !options) return
        const map: Record<string, MotionAspectRatio[]> = {}
        for (const [key, value] of Object.entries(options.resolutions)) {
          map[key] = value.aspect_ratios
        }
        setOutputCapabilities?.(map)
      })
      .catch(() => undefined)
    return () => {
      active = false
    }
  }, [setOutputCapabilities])

  const presets = useMemo(
    () =>
      motion.mode === 'motion'
        ? MOTION_VISUAL_STYLES
        : presetsForMode(MOTION_PRESETS, 'replace'),
    [motion.mode]
  )

  const pickVideo = async (file: File) => {
    try {
      await motion.pickVideo(file)
    } catch (error) {
      toastInfluencerError(t, error, 'Upload failed. Please try again.')
    }
  }

  // Hoisted so the narrowing survives into the callbacks below; reading
  // `motion.video` inside one would widen it back to the whole union.
  const videoSource = motion.video
  // Catalogue clips keep their localized name; file names are never shown.
  const videoCaption = motionVideoCaption(videoSource, t)

  const metadata =
    videoSource?.kind !== 'preset' ? videoSource?.metadata : undefined
  const videoBadge = metadata ? `${metadata.durationSec}s` : undefined

  /** The clip the slot previews: a local upload or a library clip's URL. */
  let videoPreviewUrl: string | undefined
  if (videoSource?.kind === 'upload') {
    videoPreviewUrl = videoSource.url
  } else if (videoSource?.kind === 'remote') {
    videoPreviewUrl = videoSource.previewUrl ?? videoSource.url
  }

  const mentionItems = useMemo(() => {
    return getAvailableMentions({
      hasVideo: videoSource !== null,
      videoThumbnail: videoPreviewUrl,
      videoUrl:
        videoPreviewUrl ??
        (videoSource?.kind === 'preset' ? videoSource.presetId : undefined),
      characterImages: motion.characters.map((character) => character.url),
    })
  }, [motion.characters, videoSource, videoPreviewUrl])

  return (
    <div className='inf-panel-pad inf-sidebar-content flex shrink-0 flex-col gap-2'>
      <figure className='inf-mode-preview' aria-label={t(copy.title)}>
        <img
          src={copy.preview}
          alt={t(copy.title)}
          width={304}
          height={130}
          className='inf-mode-preview-image'
        />
        <figcaption className='inf-mode-preview-copy' aria-live='polite'>
          <h2 className='inf-mode-preview-title'>{t(copy.title)}</h2>
          <p className='inf-mode-preview-description'>{t(copy.description)}</p>
        </figcaption>
      </figure>
      {/* 1. 子模式：两种模式共用同一表单，差别落在视频槽的语义上。
          与顶栏同款胶囊，但带图标——对标竞品 Motion transfer / Objects swap */}
      <SegmentedToggle
        value={motion.mode}
        onChange={motion.setMode}
        options={MOTION_MODES.map((mode) => ({
          value: mode,
          label: t(MODE_COPY[mode].switchLabelCompact),
          ariaLabel: t(MODE_COPY[mode].switchLabel),
          icon: MODE_ICONS[mode],
        }))}
        ariaLabel={t('Mode')}
        className='inf-pill'
        block
      />

      {motion.mode === 'recreate' ? (
        <RecreatePanel
          motion={motion}
          assetsRef={props.assetsRef}
          submitting={props.submitting}
          onPreviewVideo={props.onPreviewVideo}
          onPreviewImage={props.onPreviewImage}
          onBeforeVideoPick={props.onBeforeVideoPick}
        />
      ) : (
        <>
          {/* 2. 素材：角色形象（锁定身份）+ 驱动视频（动作源或场景底板） */}
          <PanelCard
            panelRef={props.assetsRef}
            icon={Images}
            label={t('Assets')}
          >
            <div className='grid grid-cols-1 gap-3'>
              <MediaSlot
                kind='video'
                label={t(copy.videoLabel)}
                hint={t(copy.videoHint)}
                accept='video/mp4,video/quicktime'
                previewUrl={videoPreviewUrl}
                badge={videoBadge}
                caption={videoCaption}
                busy={motion.videoPreparing || props.submitting}
                onPreviewVideo={props.onPreviewVideo}
                onBeforePick={props.onBeforeVideoPick}
                onOpenPicker={motion.openVideoPicker}
                pickerBusy={Boolean(props.submitting)}
                onPick={(file) => void pickVideo(file)}
                onClear={motion.clearVideo}
              />
              <CharacterSlot
                label={t('Add character, product or clothing images')}
                hint={t('Up to 30 reference images')}
                characters={motion.characters}
                busy={props.submitting}
                onOpenPicker={motion.openCharacterPicker}
                onRemoveCharacter={motion.removeCharacter}
                onPreviewImage={props.onPreviewImage}
                onPickDirectFiles={(files) => void motion.pickCharacters(files)}
              />
            </div>
          </PanelCard>

          {/* 3. 动作迁移风格仅提供视觉文字；对象替换保留原模板目录。 */}
          <PanelCard icon={Palette} label={t('Style preset')}>
            <MotionStyleRow
              presets={presets}
              style={motion.style}
              onOpen={() => setStylePickerOpen(true)}
            />
          </PanelCard>

          <MotionStyleDialog
            open={stylePickerOpen}
            presets={presets}
            selectedId={motion.style?.id ?? null}
            onOpenChange={setStylePickerOpen}
            onApply={motion.applyStyle}
          />

          {/* 4. 提示词：默认关闭，仅呈现单行标题 + 开关；
          开启后才展开输入框并随请求提交。关闭时草稿保留在 state 中。 */}
          <PanelCard
            icon={Wand2}
            label={t('Prompt')}
            contentHidden={!motion.promptEnabled}
            action={
              <Switch
                size='sm'
                checked={motion.promptEnabled}
                onCheckedChange={motion.setPromptEnabled}
                aria-label={t('Prompt')}
              />
            }
          >
            <MentionPromptEditor
              value={motion.prompt}
              items={mentionItems}
              placeholder={t(copy.placeholder)}
              onChange={motion.setPrompt}
            />
          </PanelCard>

          {/* 5. 参数：清晰度决定档位；时长与画幅只读跟随原片，不重复展示。 */}
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
      )}
    </div>
  )
}
