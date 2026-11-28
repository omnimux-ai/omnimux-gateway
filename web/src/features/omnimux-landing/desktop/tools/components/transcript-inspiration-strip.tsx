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
import { Link } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useEffect, useState } from 'react'

import { Icon } from '../../components/cn/icons'
import { useCopy } from '../../i18n/context'
import {
  fetchPublicTikTokInspirations,
  type PublicInspirationItem,
} from '../lib/api'

const ITEMS_PER_PAGE = 2

function formatCompactDuration(seconds: number): string {
  const s = Math.max(0, Math.round(seconds))
  const m = String(Math.floor(s / 60)).padStart(2, '0')
  const rem = String(s % 60).padStart(2, '0')
  return `${m}:${rem}`
}

function formatCompactViews(views: number): string {
  if (!views || views <= 0) return ''
  if (views >= 1_000_000) return `${(views / 1_000_000).toFixed(1)}M`
  if (views >= 1_000) return `${Math.round(views / 1_000)}K`
  return String(views)
}

function isTikTokUrl(raw: string | null | undefined): raw is string {
  if (!raw) return false
  try {
    const parsed = new URL(raw)
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
      return false
    }
    const host = parsed.hostname.toLowerCase()
    return host === 'tiktok.com' || host.endsWith('.tiktok.com')
  } catch {
    return false
  }
}

export function TranscriptInspirationStrip(props: {
  onPickUrl: (url: string) => void
}) {
  const copy = useCopy().toolPage
  const [items, setItems] = useState<PublicInspirationItem[]>([])
  const [page, setPage] = useState(0)

  useEffect(() => {
    let cancelled = false
    void fetchPublicTikTokInspirations(12).then((loaded) => {
      if (cancelled || loaded.length === 0) return
      setItems(loaded)
      setPage(0)
    })
    return () => {
      cancelled = true
    }
  }, [])

  if (items.length === 0) return null

  const pageCount = Math.max(1, Math.ceil(items.length / ITEMS_PER_PAGE))
  const safePage = Math.min(page, pageCount - 1)
  const start = safePage * ITEMS_PER_PAGE
  const visible = items.slice(start, start + ITEMS_PER_PAGE)
  const rangeLabel = copy.transcriptInspRange
    .replace('{from}', String(start + 1))
    .replace('{to}', String(start + visible.length))
    .replace('{total}', String(items.length))

  return (
    <section className='tool-sec tool-sec-tight'>
      <div className='wrap'>
        <div className='tx-insp-head'>
          <div>
            <h2 className='h2'>{copy.transcriptInspTitle}</h2>
            <p className='tx-insp-sub'>{copy.transcriptInspSub}</p>
          </div>
          <Link className='btn btn-sm btn-ghost' to='/inspiration/tiktok'>
            {copy.transcriptInspAll}
            <Icon name='right' />
          </Link>
        </div>

        <div className='tx-insp-grid'>
          {visible.map((item) => {
            const viewsLabel = formatCompactViews(item.views)
            const handle = item.creator?.handle
            const tiktokUrl = isTikTokUrl(item.sourceUrl)
              ? item.sourceUrl
              : null
            return (
              <article className='tx-insp-card' key={item.id}>
                <div className='tx-insp-media'>
                  {item.coverUrl ? (
                    <img
                      src={item.coverUrl}
                      alt={item.title}
                      loading='lazy'
                      decoding='async'
                    />
                  ) : (
                    <div className='tx-insp-empty' aria-hidden='true'>
                      <Icon name='video' />
                    </div>
                  )}
                  <div className='tx-insp-badges'>
                    {item.duration > 0 ? (
                      <span>{formatCompactDuration(item.duration)}</span>
                    ) : null}
                    {viewsLabel ? <span>{viewsLabel}</span> : null}
                  </div>
                </div>

                <div className='tx-insp-info'>
                  <div className='tx-insp-top'>
                    {handle ? (
                      <span className='tx-insp-author'>@{handle}</span>
                    ) : null}
                    {item.category ? (
                      <span className='tx-insp-cat'>{item.category}</span>
                    ) : null}
                  </div>
                  <h3>{item.title}</h3>
                  {item.caption ? <p>{item.caption}</p> : null}
                  <div className='tx-insp-acts'>
                    {tiktokUrl ? (
                      <button
                        type='button'
                        className='btn btn-sm btn-light'
                        onClick={() => props.onPickUrl(tiktokUrl)}
                      >
                        {copy.transcriptInspLoad}
                      </button>
                    ) : null}
                    <Link
                      className='btn btn-sm btn-ghost'
                      to='/inspiration/tiktok'
                    >
                      {copy.transcriptInspOpen}
                    </Link>
                  </div>
                </div>
              </article>
            )
          })}
        </div>

        {pageCount > 1 ? (
          <div className='tx-insp-pager'>
            <span className='tx-insp-range'>{rangeLabel}</span>
            <div className='tx-insp-nav'>
              <button
                type='button'
                className='tx-insp-nav-btn'
                aria-label={copy.transcriptInspPrev}
                onClick={() =>
                  setPage((prev) => (prev - 1 + pageCount) % pageCount)
                }
              >
                <ChevronLeft className='ico' aria-hidden='true' />
              </button>
              <div className='tx-insp-dots'>
                {Array.from({ length: pageCount }, (_, i) => (
                  <button
                    key={i}
                    type='button'
                    className='tx-insp-dot'
                    aria-current={i === safePage ? 'true' : undefined}
                    aria-label={String(i + 1)}
                    onClick={() => setPage(i)}
                  />
                ))}
              </div>
              <button
                type='button'
                className='tx-insp-nav-btn'
                aria-label={copy.transcriptInspNext}
                onClick={() => setPage((prev) => (prev + 1) % pageCount)}
              >
                <ChevronRight className='ico' aria-hidden='true' />
              </button>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  )
}
