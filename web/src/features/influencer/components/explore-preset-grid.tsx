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

import { Sparkles, Wand2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'

import { prefetchImage } from '../lib/prefetch'
import { isCharacterTier } from '../lib/taxonomy'
import type { ExplorePreset } from '../types'
import { FadeInImage } from './fade-in-image'

interface Props {
  presets: ExplorePreset[]
  /** Opens the enlarged preview dialog for this preset. */
  onPreview: (preset: ExplorePreset) => void
  /** Fills the sidebar straight from the card, skipping the dialog. */
  onApply: (preset: ExplorePreset) => void
}

/**
 * The inspiration library: mirrored presets in a column-flow masonry, because
 * the artwork ratios differ per card and a fixed grid would crop them.
 */
export function ExplorePresetGrid(props: Props) {
  const { t } = useTranslation()

  if (!props.presets.length) {
    return (
      <EmptyState
        icon={Sparkles}
        title={t('No presets found')}
        description={t('Inspiration presets are unavailable right now')}
      />
    )
  }

  return (
    <div className='columns-2 gap-4 py-4 sm:columns-3 xl:columns-4'>
      {props.presets.map((preset, index) => (
        <ExplorePresetCard
          key={JSON.stringify([
            preset.id,
            preset.thumbnail?.path,
            preset.preview.path,
          ])}
          preset={preset}
          index={index}
          onPreview={props.onPreview}
          onApply={props.onApply}
        />
      ))}
    </div>
  )
}

interface CardProps {
  preset: ExplorePreset
  index: number
  onPreview: (preset: ExplorePreset) => void
  onApply: (preset: ExplorePreset) => void
}

/**
 * Two sibling buttons rather than one nested in the other: the whole surface
 * opens the preview, and the overlaid pill applies the preset directly. The
 * pill is revealed on hover or focus, and stays visible where hover does not
 * exist (touch) so it is never unreachable.
 */
function ExplorePresetCard(props: CardProps) {
  const { t } = useTranslation()
  const [failedSource, setFailedSource] = useState<string | null>(null)
  const preset = props.preset
  const dimensions = preset.thumbnail ?? preset.preview
  const artwork =
    preset.thumbnail && failedSource !== preset.thumbnail.path
      ? preset.thumbnail
      : preset.preview

  if (failedSource === preset.preview.path) return null

  const isPriority = props.index < 4

  const onPrefetch = () => {
    if (preset.sheet?.path) {
      prefetchImage(preset.sheet.path)
    }
  }

  return (
    <article
      onMouseEnter={onPrefetch}
      onFocusCapture={onPrefetch}
      className='group/preset relative isolate mb-4 break-inside-avoid overflow-hidden rounded-[var(--radius-card)] bg-[var(--inf-surface-2)] ring-1 ring-[var(--inf-line)] ring-inset'
    >
      <button
        type='button'
        aria-label={preset.name}
        onClick={() => props.onPreview(preset)}
        className='block w-full cursor-pointer'
      >
        <FadeInImage
          key={artwork.path}
          src={artwork.path}
          alt=''
          width={dimensions.width}
          height={dimensions.height}
          loading={isPriority ? 'eager' : 'lazy'}
          fetchPriority={isPriority ? 'high' : 'auto'}
          decoding='async'
          onError={() => setFailedSource(artwork.path)}
          style={{
            aspectRatio: `${dimensions.width} / ${dimensions.height}`,
          }}
          className='w-full bg-[var(--inf-surface-2)] object-cover'
        />
      </button>

      <button
        type='button'
        disabled={!isCharacterTier(preset.tier)}
        title={
          !isCharacterTier(preset.tier)
            ? t('This character type cannot be reused')
            : undefined
        }
        onClick={() => props.onApply(preset)}
        className='inf-abtn absolute right-2 bottom-2 inline-flex items-center gap-1.5 opacity-0 transition-opacity duration-150 group-focus-within/preset:opacity-100 group-hover/preset:opacity-100 max-md:opacity-100 [@media(hover:none)]:opacity-100'
      >
        <Wand2 className='h-3.5 w-3.5' aria-hidden='true' />
        <span>{t('Recreate')}</span>
      </button>
    </article>
  )
}
