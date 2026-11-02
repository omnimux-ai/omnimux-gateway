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
import { describe, expect, it } from 'vitest'

import { splitIntoColumns } from '../waterfall'

describe('waterfall layout distribution', () => {
  it('places all items into a single column when count is 1', () => {
    const items = [{ id: '1' }, { id: '2' }, { id: '3' }]
    const columns = splitIntoColumns(items, 1)

    expect(columns).toHaveLength(1)
    expect(columns[0].id).toBe('column-1')
    expect(columns[0].items).toEqual(items)
  })

  it('balances column heights greedily when items have mixed aspect ratios', () => {
    const items = [
      { id: '1', aspectRatio: '16 / 9' }, // landscape (short)
      { id: '2', aspectRatio: '9 / 16' },  // portrait (tall)
      { id: '3', aspectRatio: '16 / 9' }, // landscape (short)
      { id: '4', aspectRatio: '16 / 9' }, // landscape (short)
      { id: '5', aspectRatio: '16 / 9' }, // landscape (short)
    ]
    const columns = splitIntoColumns(items, 2)

    expect(columns).toHaveLength(2)
    // The tall portrait item in col 2 causes short items to be routed to col 1 to balance heights
    expect(columns[0].items.length).toBeGreaterThanOrEqual(2)
    expect(columns[1].items.length).toBeGreaterThanOrEqual(1)
    // All items must be preserved
    const allIds = columns.flatMap((col) => col.items.map((i) => i.id))
    expect(allIds.sort()).toEqual(['1', '2', '3', '4', '5'])
  })

  it('distributes items without leaving any trailing column empty', () => {
    const items = Array.from({ length: 11 }, (_, i) => ({ id: `item-${i}` }))
    const columns = splitIntoColumns(items, 5)

    expect(columns).toHaveLength(5)
    for (const col of columns) {
      expect(col.items.length).toBeGreaterThan(0)
    }
  })
})
