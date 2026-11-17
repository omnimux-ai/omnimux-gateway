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
import type { RefObject } from 'react'

import type { MotionForm } from '../hooks/use-motion'
import type { ToolHandoffState } from '../hooks/use-tool-handoff'
import { RecreateForm } from './recreate-form'

interface Props {
  motion: MotionForm
  /** Shared reference picker anchor; missing it closes the desktop panel. */
  assetsRef?: RefObject<HTMLElement | null>
  submitting?: boolean
  toolHandoff?: ToolHandoffState
  onPreviewVideo?: (url: string) => void
  onPreviewImage?: (url: string) => void
  onBeforeVideoPick?: () => boolean
  /** Return false to send a guest to the login gate instead of uploading. */
  onBeforeImagePick?: () => boolean
}

/**
 * The third motion sub-mode. Generation is one action: it submits a single
 * queue task and releases the form immediately; the worker owns decomposition.
 * The form stays editable while that task runs elsewhere.
 */
export function RecreatePanel(props: Props) {
  return (
    <RecreateForm
      motion={props.motion}
      assetsRef={props.assetsRef}
      submitting={props.submitting}
      toolHandoff={props.toolHandoff}
      onPreviewVideo={props.onPreviewVideo}
      onPreviewImage={props.onPreviewImage}
      onBeforeVideoPick={props.onBeforeVideoPick}
      onBeforeImagePick={props.onBeforeImagePick}
    />
  )
}
