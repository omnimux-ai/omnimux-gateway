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
import { describe, expect, test } from 'vitest'

import {
  intentSuffix,
  needsIntentHint,
  promptHasIntent,
} from '../intent-precheck'

describe('needsIntentHint', () => {
  test('asks for wording only on the two inferred modes', () => {
    expect(needsIntentHint('video_edit')).toBe(true)
    expect(needsIntentHint('video_extend')).toBe(true)
    expect(needsIntentHint('text_to_video')).toBe(false)
    expect(needsIntentHint('first_frame')).toBe(false)
    expect(needsIntentHint('video_multi_ref')).toBe(false)
  })
})

describe('promptHasIntent', () => {
  test('recognises an English edit instruction', () => {
    expect(promptHasIntent('video_edit', 'Remove the logo from the cup')).toBe(true)
  })

  test('recognises a Chinese edit instruction', () => {
    expect(promptHasIntent('video_edit', '把杯子上的 logo 去掉')).toBe(true)
  })

  test('ignores letter case', () => {
    expect(promptHasIntent('video_edit', 'REPLACE the background')).toBe(true)
  })

  test('flags a prompt that never states the edit', () => {
    expect(promptHasIntent('video_edit', 'a girl dancing on a stage')).toBe(false)
  })

  test('accepts continuation wording for extend', () => {
    expect(promptHasIntent('video_extend', 'Continue the story forward')).toBe(true)
    expect(promptHasIntent('video_extend', '继续向前延长')).toBe(true)
  })

  test('treats the modes without rules as already satisfied', () => {
    expect(promptHasIntent('text_to_video', 'anything at all')).toBe(true)
    expect(promptHasIntent('first_frame', '')).toBe(true)
  })
})

describe('intentSuffix', () => {
  test('gives each inferred mode a prefix that states the task', () => {
    expect(intentSuffix('video_edit')).toContain('Edit')
    expect(intentSuffix('video_extend')).toContain('Continue')
  })

  test('returns nothing for a mode that needs no hint', () => {
    expect(intentSuffix('text_to_video')).toBe('')
  })
})
