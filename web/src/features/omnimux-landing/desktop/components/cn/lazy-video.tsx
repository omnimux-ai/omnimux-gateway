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
import { useEffect, useRef } from 'react'

import { usePrefersReducedMotion } from './hooks'

/**
 * Poster-first video: the file is only fetched and played while the tile is
 * visible. `hold` keeps a tile on its poster until the scripted demo reaches
 * the generation step.
 */
export function LazyVideo(props: {
  src: string
  poster: string
  hold?: boolean
  className?: string
}) {
  const ref = useRef<HTMLVideoElement>(null)
  const reduced = usePrefersReducedMotion()
  const hold = props.hold ?? false

  useEffect(() => {
    const video = ref.current
    if (!video) return
    if (reduced || hold) {
      video.pause()
      return
    }
    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0]
        if (entry?.isIntersecting) {
          if (!video.getAttribute('src')) video.src = props.src
          void video.play().catch(() => {})
        } else {
          video.pause()
        }
      },
      { threshold: 0.25 }
    )
    observer.observe(video)
    return () => observer.disconnect()
  }, [props.src, hold, reduced])

  return (
    <video
      ref={ref}
      poster={props.poster}
      className={props.className}
      muted
      loop
      playsInline
      preload='none'
    />
  )
}
