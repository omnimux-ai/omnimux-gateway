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
import { Link, useNavigate } from '@tanstack/react-router'
import { Lock } from 'lucide-react'

import { Icon } from '../../components/cn/icons'
import { useCopy } from '../../i18n'
import type { ToolResult } from '../lib/api'

/** `TikTok · 2025-02-13`: the post date in UTC, independent of UI language. */
function postedLine(result: ToolResult, extra?: string): string {
  const parts = ['TikTok']
  if (result.created_at > 0) {
    parts.push(new Date(result.created_at * 1000).toISOString().slice(0, 10))
  }
  if (extra) parts.push(extra)
  return parts.join(' · ')
}

function ToolAuthor(props: { result: ToolResult; extra?: string }) {
  return (
    <div className='tool-who'>
      <i aria-hidden='true' />
      <div>
        <b>@{props.result.author.unique_id}</b>
        <span>{postedLine(props.result, props.extra)}</span>
      </div>
    </div>
  )
}

function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const minutes = String(Math.floor(total / 60)).padStart(2, '0')
  const seconds = String(total % 60).padStart(2, '0')
  return `${minutes}:${seconds}`
}

function formatFrame(width: number, height: number): string {
  if (!width || !height) return ''
  if (Math.abs(width / height - 9 / 16) < 0.02) return '9:16'
  return `${width}×${height}`
}

export function ToolVideoResult(props: { result: ToolResult }) {
  const copy = useCopy().toolPage
  const video = props.result.video
  if (!video) return null
  const frame = formatFrame(video.width, video.height)
  return (
    <div className='tool-result' aria-live='polite'>
      <div className='tool-vid'>
        <video
          controls
          playsInline
          preload='metadata'
          poster={props.result.cover_url || undefined}
          src={video.preview_url}
        />
        <div>
          <ToolAuthor result={props.result} />
          {props.result.description ? (
            <p className='tool-desc'>{props.result.description}</p>
          ) : null}
          <div className='tool-chips'>
            {video.duration_ms > 0 ? (
              <span>{formatDuration(video.duration_ms)}</span>
            ) : null}
            {frame ? <span>{frame}</span> : null}
            <span>MP4</span>
          </div>
          <div className='tool-acts'>
            <a className='btn btn-light' href={video.download_url} download>
              <Icon name='dl' />
              {copy.downloadMp4}
            </a>
          </div>
          <div className='tool-divider' />
          <div className='tool-acts'>
            <Link className='btn btn-primary' to='/influencer'>
              <Icon name='spark' />
              {copy.recreateMine}
            </Link>
            <Link className='btn btn-ghost' to='/influencer'>
              {copy.breakdown}
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}

export function ToolPhotoResult(props: {
  result: ToolResult
  loggedIn: boolean
}) {
  const copy = useCopy().toolPage
  const navigate = useNavigate()
  const images = props.result.images ?? []
  const count = copy.photoCount.replace('{count}', String(images.length))
  const zipUrl = props.result.zip_url
  return (
    <div className='tool-result' aria-live='polite'>
      <div className='tool-photo-head'>
        <ToolAuthor result={props.result} extra={count} />
        <span className='tool-spacer' />
        {props.loggedIn && zipUrl ? (
          <a className='btn btn-sm btn-ghost' href={zipUrl} download>
            {copy.downloadZip}
          </a>
        ) : null}
        {props.loggedIn ? null : (
          <>
            <span className='tool-lock' id='tool-zip-lock'>
              <Lock className='ico' aria-hidden='true' />
              {copy.zipLocked}
            </span>
            <button
              type='button'
              className='btn btn-sm btn-ghost'
              aria-describedby='tool-zip-lock'
              onClick={() =>
                void navigate({
                  to: '/sign-in',
                  search: {
                    redirect: `${window.location.pathname}${window.location.search}`,
                  },
                })
              }
            >
              {copy.downloadZip}
            </button>
          </>
        )}
      </div>
      <div className='tool-imgs'>
        {images.map((image, position) => {
          const n = String(position + 1)
          return (
            <div className='tool-img' key={image.index}>
              <img
                src={image.preview_url}
                alt={copy.imageAlt.replace('{n}', n)}
                loading='lazy'
                decoding='async'
              />
              <span className='tool-n'>{n.padStart(2, '0')}</span>
              <a
                href={image.download_url}
                download
                aria-label={copy.downloadImage.replace('{n}', n)}
              >
                <Icon name='dl' />
              </a>
            </div>
          )
        })}
      </div>
      <div className='tool-divider' />
      <div className='tool-btns'>
        <Link className='btn btn-primary' to='/influencer'>
          {copy.recreatePhotos}
        </Link>
        <Link className='btn btn-ghost' to='/influencer'>
          {copy.breakdown}
        </Link>
      </div>
    </div>
  )
}
