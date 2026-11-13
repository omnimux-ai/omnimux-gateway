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

import type { PricingModel } from '@/features/pricing/types'

import {
  provisionedTierGroups,
  tierGroupForResolution,
  tierResolutions,
} from '../tier-group'

function subject(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'wan-3.0',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['wan-3.0-720p', 'wan-3.0-1080p'],
    ...overrides,
  }
}

describe('provisionedTierGroups', () => {
  test('returns only the tier groups the model actually enables', () => {
    const groups = provisionedTierGroups(
      subject({ enable_groups: ['default', 'wan-3.0-720p'] })
    )
    expect(groups).toEqual({ '720p': 'wan-3.0-720p' })
  })

  test('returns nothing while no tier group is provisioned', () => {
    expect(provisionedTierGroups(subject({ enable_groups: ['default'] }))).toBe(
      undefined
    )
  })

  test('returns nothing for a model that declares no resolution tiers', () => {
    expect(
      provisionedTierGroups(
        subject({
          model_name: 'seedance-2-5',
          enable_groups: ['seedance-2-5-economy'],
        })
      )
    ).toBe(undefined)
  })
})

describe('tierResolutions', () => {
  test('lists only the tiers whose group is provisioned', () => {
    expect(tierResolutions(subject({}))).toEqual(['720p', '1080p'])
    expect(
      tierResolutions(subject({ enable_groups: ['wan-3.0-1080p'] }))
    ).toEqual(['1080p'])
  })

  test('is absent when no tier group is provisioned', () => {
    expect(tierResolutions(subject({ enable_groups: [] }))).toBe(undefined)
    expect(
      tierResolutions(subject({ enable_groups: ['default'] }))
    ).toBe(undefined)
  })
})

describe('tierGroupForResolution', () => {
  test('maps a declared resolution onto its group regardless of case or padding', () => {
    expect(tierGroupForResolution(subject({}), { resolution: '720p' })).toBe(
      'wan-3.0-720p'
    )
    expect(
      tierGroupForResolution(subject({}), { resolution: ' 1080P ' })
    ).toBe('wan-3.0-1080p')
  })

  test('returns no group for a resolution the provisioned tiers do not cover', () => {
    for (const resolution of ['480p', '1280x720', '', '  ']) {
      expect(
        tierGroupForResolution(subject({}), { resolution }),
        resolution
      ).toBe(undefined)
    }
    expect(tierGroupForResolution(subject({}), {})).toBe(undefined)
    expect(
      tierGroupForResolution(subject({}), { resolution: 720 })
    ).toBe(undefined)
  })

  test('returns no group while the tiers are unprovisioned', () => {
    expect(
      tierGroupForResolution(subject({ enable_groups: ['default'] }), {
        resolution: '720p',
      })
    ).toBe(undefined)
  })
})
