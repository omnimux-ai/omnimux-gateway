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
import { RecreateAnalysisProgress } from './recreate-analysis-progress'
import { RecreateForm } from './recreate-form'

interface Props {
  motion: MotionForm
  /** Shared reference picker anchor; missing it closes the desktop panel. */
  assetsRef?: RefObject<HTMLElement | null>
  submitting?: boolean
  onPreviewVideo?: (url: string) => void
  onPreviewImage?: (url: string) => void
  onBeforeVideoPick?: () => boolean
}

/**
 * The third motion sub-mode. Generation is the only action: clicking it starts
 * the decomposition, submits the compiled prompt and moves the task to history.
 * While the decomposition runs the form stays visible but inert, so the user
 * can still read the inputs that are being worked on.
 */
export function RecreatePanel(props: Props) {
  if (props.motion.recreateAnalyzing) {
    return (
      <>
        <RecreateAnalysisProgress />
        <div inert className='pointer-events-none opacity-50'>
          <RecreateForm
            motion={props.motion}
            assetsRef={props.assetsRef}
            submitting={props.submitting}
            onPreviewVideo={props.onPreviewVideo}
            onPreviewImage={props.onPreviewImage}
            onBeforeVideoPick={props.onBeforeVideoPick}
          />
        </div>
      </>
    )
  }
  return (
    <RecreateForm
      motion={props.motion}
      assetsRef={props.assetsRef}
      submitting={props.submitting}
      onPreviewVideo={props.onPreviewVideo}
      onPreviewImage={props.onPreviewImage}
      onBeforeVideoPick={props.onBeforeVideoPick}
    />
  )
}
