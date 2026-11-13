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

import { formatGroupShortName, getCheapestAvailableGroup } from '../model-helpers'
import type { PricingModel } from '../../types'

function mockModel(enableGroups: string[]): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: enableGroups,
  }
}

describe('getCheapestAvailableGroup', () => {
  const usableGroup = {
    default: 'Default',
    'gpt-image-2.5-economy': 'Economy',
    'gpt-image-2.5-pro': 'Pro',
    vip: 'VIP',
  }

  const groupRatio = {
    default: 1,
    'gpt-image-2.5-economy': 0.112455,
    'gpt-image-2.5-pro': 0.281248,
    vip: 0.5,
  }

  it('returns "default" when model has no available groups', () => {
    const model = mockModel([])
    const cheapest = getCheapestAvailableGroup(model, usableGroup, groupRatio)
    expect(cheapest).toBe('default')
  })

  it('returns the only group when model has exactly one available group', () => {
    const model = mockModel(['default'])
    const cheapest = getCheapestAvailableGroup(model, usableGroup, groupRatio)
    expect(cheapest).toBe('default')
  })

  it('picks the group with the lowest ratio for multi-group models', () => {
    const model = mockModel([
      'default',
      'gpt-image-2.5-economy',
      'gpt-image-2.5-pro',
    ])
    const cheapest = getCheapestAvailableGroup(model, usableGroup, groupRatio)
    expect(cheapest).toBe('gpt-image-2.5-economy')
  })

  it('stabilizes ties with alphabetical ordering', () => {
    const model = mockModel(['group-b', 'group-a'])
    const tiesUsable = {
      'group-a': 'A',
      'group-b': 'B',
    }
    const tiesRatio = {
      'group-a': 1,
      'group-b': 1,
    }
    const cheapest = getCheapestAvailableGroup(model, tiesUsable, tiesRatio)
    expect(cheapest).toBe('group-a')
  })

  describe('formatGroupShortName', () => {
    it('strips model prefix when group starts with model name', () => {
      expect(formatGroupShortName('gpt-image-2.5-economy', 'gpt-image-2.5')).toBe('economy')
      expect(formatGroupShortName('gpt-image-2.5-pro', 'gpt-image-2.5')).toBe('pro')
      expect(formatGroupShortName('minimax-h3-video-fast', 'minimax-h3-video')).toBe('fast')
    })

    it('returns raw group name when no prefix matches', () => {
      expect(formatGroupShortName('default', 'gpt-image-2.5')).toBe('default')
      expect(formatGroupShortName('vip', 'gpt-image-2.5')).toBe('vip')
      expect(formatGroupShortName('grok-imagine-2.0-ext', 'other')).toBe('grok-imagine-2.0-ext')
    })

    it('handles empty or missing parameters gracefully', () => {
      expect(formatGroupShortName('')).toBe('')
      expect(formatGroupShortName('default')).toBe('default')
    })
  })
})
