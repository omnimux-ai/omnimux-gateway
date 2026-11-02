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
import { lazy, Suspense } from 'react'

import { HtmlContent, type HtmlContentVariant } from '@/components/html-content'
import { cn } from '@/lib/utils'

/**
 * The markdown renderer pulls in marked, DOMPurify and KaTeX (with its
 * stylesheet and fonts). Rich content is only ever shown on demand — an opened
 * announcement popover, a legal page, an about page — while this module itself
 * sits in the entry graph through the app header, so keep the renderer in its
 * own chunk instead of shipping it to every first visit.
 */
const Markdown = lazy(() =>
  import('@/components/ui/markdown').then((module) => ({
    default: module.Markdown,
  }))
)

type RichContentMode = 'markdown' | 'html'

interface RichContentProps {
  content: string
  mode?: RichContentMode
  breaks?: boolean
  className?: string
  htmlVariant?: HtmlContentVariant
}

export function RichContent(props: RichContentProps) {
  if (props.mode === 'html') {
    return (
      <HtmlContent
        content={props.content}
        className={props.className}
        variant={props.htmlVariant}
      />
    )
  }

  return (
    <Suspense
      fallback={
        // Plain text until the renderer arrives: the content stays readable
        // instead of flashing an empty box.
        <div className={cn('whitespace-pre-wrap', props.className)}>
          {props.content}
        </div>
      }
    >
      <Markdown breaks={props.breaks} className={props.className}>
        {props.content}
      </Markdown>
    </Suspense>
  )
}
