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
import { useState } from 'react'

import type { DownloadCtaIntent } from '@/lib/download-cta'

import { useCopy } from '../../i18n/context'
import type { ToolWallCategory } from '../../i18n/types'
import { useDownload } from './download-context'
import { Icon } from './icons'
import { ModelBrandIcon } from './model-brand-icon'

const CATEGORY_ORDER: readonly ToolWallCategory[] = [
  'all',
  'research',
  'assets',
  'create',
  'studio',
]

const TOOLS_DETAIL_DOWNLOAD_INTENT: DownloadCtaIntent = {
  cta_id: 'tools_detail_download',
  cta_position: 'footer',
  cta_variant: 'section',
}

export function CnTools() {
  const copy = useCopy().tools
  const { open } = useDownload()
  const [category, setCategory] = useState<ToolWallCategory>('all')
  const [selectedId, setSelectedId] = useState<string>('digital-humans')

  const visibleItems =
    category === 'all'
      ? copy.items
      : copy.items.filter((item) => item.category === category)

  const selectedItem =
    visibleItems.find((item) => item.id === selectedId) ??
    visibleItems[0] ??
    copy.items[0]

  return (
    <section className='sec' id='tools'>
      <div className='wrap'>
        <div className='sec-head'>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
        </div>

        <div className='tool-topo' aria-label={copy.topologyAria}>
          <div className='tool-sources'>
            {copy.sources.map((label) => (
              <div key={label} className='tool-src'>
                {label}
              </div>
            ))}
          </div>

          <svg
            className='tool-svg tool-svg-desktop'
            viewBox='0 0 1000 56'
            preserveAspectRatio='none'
            aria-hidden='true'
          >
            <path
              className='tool-beam-base'
              d='M100 0 C100 32, 500 20, 500 56'
            />
            <path
              className='tool-beam-base'
              d='M300 0 C300 32, 500 20, 500 56'
            />
            <path className='tool-beam-base' d='M500 0 L500 56' />
            <path
              className='tool-beam-base'
              d='M700 0 C700 32, 500 20, 500 56'
            />
            <path
              className='tool-beam-base'
              d='M900 0 C900 32, 500 20, 500 56'
            />
            <path
              className='tool-beam-pulse'
              d='M100 0 C100 32, 500 20, 500 56'
            />
            <path
              className='tool-beam-pulse'
              d='M300 0 C300 32, 500 20, 500 56'
            />
            <path className='tool-beam-pulse' d='M500 0 L500 56' />
            <path
              className='tool-beam-pulse'
              d='M700 0 C700 32, 500 20, 500 56'
            />
            <path
              className='tool-beam-pulse'
              d='M900 0 C900 32, 500 20, 500 56'
            />
          </svg>

          <svg
            className='tool-svg tool-svg-mobile'
            viewBox='0 0 360 40'
            preserveAspectRatio='none'
            aria-hidden='true'
          >
            <path className='tool-beam-base' d='M90 0 C90 22, 180 14, 180 40' />
            <path
              className='tool-beam-base'
              d='M270 0 C270 22, 180 14, 180 40'
            />
            <path
              className='tool-beam-pulse'
              d='M90 0 C90 22, 180 14, 180 40'
            />
            <path
              className='tool-beam-pulse'
              d='M270 0 C270 22, 180 14, 180 40'
            />
          </svg>

          <div className='tool-hub'>
            <img src='/logo.svg' alt='' width={22} height={22} />
            <span>OmniMux</span>
          </div>

          <svg
            className='tool-svg tool-svg-desktop'
            viewBox='0 0 1000 56'
            preserveAspectRatio='none'
            aria-hidden='true'
          >
            <path
              className='tool-beam-base'
              d='M500 0 C500 34, 100 20, 100 56'
            />
            <path
              className='tool-beam-base'
              d='M500 0 C500 34, 300 20, 300 56'
            />
            <path className='tool-beam-base' d='M500 0 L500 56' />
            <path
              className='tool-beam-base'
              d='M500 0 C500 34, 700 20, 700 56'
            />
            <path
              className='tool-beam-base'
              d='M500 0 C500 34, 900 20, 900 56'
            />
            <path
              className='tool-beam-pulse'
              d='M500 0 C500 34, 100 20, 100 56'
            />
            <path
              className='tool-beam-pulse'
              d='M500 0 C500 34, 300 20, 300 56'
            />
            <path className='tool-beam-pulse' d='M500 0 L500 56' />
            <path
              className='tool-beam-pulse'
              d='M500 0 C500 34, 700 20, 700 56'
            />
            <path
              className='tool-beam-pulse'
              d='M500 0 C500 34, 900 20, 900 56'
            />
          </svg>

          <svg
            className='tool-svg tool-svg-mobile'
            viewBox='0 0 360 40'
            preserveAspectRatio='none'
            aria-hidden='true'
          >
            <path className='tool-beam-base' d='M180 0 C180 24, 90 14, 90 40' />
            <path
              className='tool-beam-base'
              d='M180 0 C180 24, 270 14, 270 40'
            />
            <path
              className='tool-beam-pulse'
              d='M180 0 C180 24, 90 14, 90 40'
            />
            <path
              className='tool-beam-pulse'
              d='M180 0 C180 24, 270 14, 270 40'
            />
          </svg>
        </div>

        <div
          className='tool-filters'
          role='tablist'
          aria-label={copy.filterAria}
        >
          {CATEGORY_ORDER.map((cat) => {
            const active = cat === category
            return (
              <button
                key={cat}
                type='button'
                role='tab'
                aria-selected={active}
                className={active ? 'tool-filter on' : 'tool-filter'}
                onClick={() => setCategory(cat)}
              >
                {copy.filters[cat]}
              </button>
            )
          })}
        </div>

        <div className='tool-wall'>
          {visibleItems.map((item) => {
            const selected = item.id === selectedItem.id
            return (
              <button
                key={item.id}
                type='button'
                aria-pressed={selected}
                className={selected ? 'tool-tile on' : 'tool-tile'}
                onClick={() => setSelectedId(item.id)}
              >
                <span className='tool-tile-ico'>
                  {item.brandIcon ? (
                    <ModelBrandIcon icon={item.brandIcon} size={18} />
                  ) : (
                    <Icon name={item.icon ?? 'spark'} />
                  )}
                </span>
                <span className='tool-tile-body'>
                  <strong>{item.name}</strong>
                  <span>{item.cap}</span>
                </span>
              </button>
            )
          })}

          {category === 'all' ? (
            <a className='tool-tile tool-tile-anchor' href='#payg-pricing'>
              <span className='tool-tile-body'>
                <strong>{copy.anchorTitle}</strong>
                <span>{copy.anchorCap}</span>
              </span>
              <Icon name='right' />
            </a>
          ) : null}
        </div>

        <div className='tool-detail' aria-live='polite'>
          <span className='tool-tile-ico'>
            {selectedItem.brandIcon ? (
              <ModelBrandIcon icon={selectedItem.brandIcon} size={18} />
            ) : (
              <Icon name={selectedItem.icon ?? 'spark'} />
            )}
          </span>
          <div className='tool-detail-info'>
            <h3>{selectedItem.name}</h3>
            <p>{selectedItem.desc}</p>
          </div>
          <button
            type='button'
            className='btn btn-sm btn-primary'
            onClick={() => open(undefined, TOOLS_DETAIL_DOWNLOAD_INTENT)}
          >
            {copy.detailCta}
          </button>
        </div>

        <div className='models' id='models'>
          <h3>{copy.modelsTitle}</h3>
          <div className='chiprows' aria-label={copy.chipsAria}>
            {copy.capabilityRows.map((row, rowIndex) => (
              <div
                className={rowIndex === 1 ? 'chiprow rev' : 'chiprow'}
                key={row[0].label}
              >
                {row.map((chip) => (
                  <span key={chip.label}>
                    <Icon name={chip.icon} />
                    {chip.label}
                  </span>
                ))}
                {row.map((chip) => (
                  <span key={`dup-${chip.label}`} aria-hidden='true'>
                    <Icon name={chip.icon} />
                    {chip.label}
                  </span>
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
