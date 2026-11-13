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

import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { screenMediaSelection } from '@/features/model-playground/lib/media-validation'

import { influencerErrorText } from '../lib/error-copy'
import { useUploadLibrary } from './use-upload-library'

/** Upload lifecycle only; the builder remains the sole owner of the URL, and the
 * page's upload library keeps the finished file listed for the account. */
export function useImageReference(
  setUrl: (url: string, mimeType?: string) => void
) {
  const { t } = useTranslation()
  const identity = useRef(0)
  const active = useRef(false)
  const [imageUploading, setUploading] = useState(false)
  const uploadFile = useUploadLibrary().upload

  const invalidateImage = useCallback(() => {
    identity.current += 1
    active.current = false
    setUploading(false)
  }, [])

  useEffect(
    () => () => {
      identity.current += 1
      active.current = false
    },
    []
  )

  const pickImage = useCallback(
    async (file: File): Promise<void> => {
      if (active.current) return
      const { issue } = screenMediaSelection(
        {
          accept: ['image/jpeg', 'image/png', 'image/webp'],
          maxCount: 1,
          maxBytes: 5 * 1024 * 1024,
        },
        0,
        [file]
      )
      if (issue) {
        toast.error(
          t(
            issue.code === 'too_large'
              ? 'Image must be 5 MiB or smaller'
              : 'Use a JPEG, PNG or WebP image'
          )
        )
        return
      }
      const request = ++identity.current
      active.current = true
      setUploading(true)
      try {
        const url = await uploadFile(file, 'image')
        if (request === identity.current) setUrl(url, file.type)
      } catch (error) {
        if (request === identity.current) {
          toast.error(
            influencerErrorText(
              t,
              error,
              'Image upload failed. Please try again.'
            )
          )
        }
      } finally {
        if (request === identity.current) {
          active.current = false
          setUploading(false)
        }
      }
    },
    [setUrl, t, uploadFile]
  )

  return {
    pickImage,
    invalidateImage,
    imageUploading,
    isImageUploading: () => active.current,
  }
}
