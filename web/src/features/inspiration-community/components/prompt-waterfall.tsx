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
import { cn } from '@/lib/utils'

import {
  splitIntoColumns,
  useWaterfallColumnCount,
  type WaterfallLadder,
} from '../lib/waterfall'
import type { PromptItem } from '../types'
import { PromptDetailCard } from './prompt-detail-card'

export interface PromptWaterfallProps {
  items: PromptItem[]
  /**
   * Opening a tile hands over the ordered list it came from, so the detail
   * layer can flip to the neighbours the visitor is actually looking at.
   */
  onOpenPreview: (item: PromptItem, items: PromptItem[]) => void
  /** Breakpoint ladder of the level rendering the batch. */
  ladder: WaterfallLadder
  /** Column and inter-tile gap; the L1 teaser is denser than the L2 listing. */
  gapClassName: string
  testId?: string
}

/**
 * The batch of tiles as a waterfall, shared by the L1 section preview and the
 * L2 listing so both levels lay the same tile out the same way.
 *
 * The only differences between the two call sites are the ladder (a ten-tile
 * teaser is denser than a library listing), the gap, and the test id.
 */
export function PromptWaterfall(props: PromptWaterfallProps) {
  const columns = splitIntoColumns(
    props.items,
    useWaterfallColumnCount(props.ladder)
  )
  const openPreview = (item: PromptItem) =>
    props.onOpenPreview(item, props.items)

  return (
    <div
      className={cn('flex items-start', props.gapClassName)}
      data-testid={props.testId}
    >
      {columns.map((column) => (
        <div
          key={column.id}
          className={cn('flex min-w-0 flex-1 flex-col', props.gapClassName)}
        >
          {column.items.map((item) => (
            <PromptDetailCard
              key={item.id}
              item={item}
              onOpenPreview={openPreview}
            />
          ))}
        </div>
      ))}
    </div>
  )
}
