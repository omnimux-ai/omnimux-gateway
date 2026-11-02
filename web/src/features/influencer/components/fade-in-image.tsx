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

import { useEffect, useRef, useState, type ComponentProps } from 'react'

import { cn } from '@/lib/utils'

export type FadeInImageProps = ComponentProps<'img'> & {
  /** Lets a card swap its skeleton for the decoded image without reading state. */
  onLoadedChange?: (loaded: boolean) => void
}

/**
 * Shared image wrapper that renders with opacity-0 and transitions to opacity-100
 * via `.inf-card-img.loaded` once the image bytes are decoded.
 * Checks `complete` on mount so browser-cached images never stay invisible.
 */
export function FadeInImage(props: FadeInImageProps) {
  const { className, onLoad, onError, onLoadedChange, ...rest } = props
  const [loaded, setLoaded] = useState(false)
  const imgRef = useRef<HTMLImageElement | null>(null)
  const loadedChangeRef = useRef(onLoadedChange)
  loadedChangeRef.current = onLoadedChange

  useEffect(() => {
    if (imgRef.current?.complete && imgRef.current.naturalWidth > 0) {
      setLoaded(true)
    }
  }, [])

  useEffect(() => {
    loadedChangeRef.current?.(loaded)
  }, [loaded])

  return (
    <img
      ref={imgRef}
      {...rest}
      className={cn('inf-card-img', loaded && 'loaded', className)}
      onLoad={(e) => {
        setLoaded(true)
        onLoad?.(e)
      }}
      onError={(e) => {
        onError?.(e)
      }}
    />
  )
}
