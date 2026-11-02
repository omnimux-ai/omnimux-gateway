import { describe, expect, it } from 'vitest'

import * as catalog from '../lib/catalog'
import type { PricingRow } from '../types'

const economy = 'gpt-image-2.5-sunburst-image'
const row: PricingRow = {
  model_name: 'gpt-image-2.5-sunburst',
  vendor_id: 1,
  quota_type: 1,
  model_price: 0.3,
  enable_groups: ['default', economy],
}

describe('reference-image group selection', () => {
  it('uses the reference-compatible supplier group rather than a cheaper text-only group', () => {
    expect(
      catalog.pickImageQuoteGroup(row, {
        default: { ratio: 0.01 },
        [economy]: { ratio: 0.1 },
      })
    ).toBe(economy)
  })

  it('refuses reference generation when the caller cannot use the supplier group', () => {
    expect(
      catalog.pickImageQuoteGroup(row, { default: { ratio: 1 } })
    ).toBeNull()
  })

  it('does not infer authenticated group permission before it has loaded', () => {
    expect(catalog.pickImageQuoteGroup(row, undefined)).toBeNull()
    expect(catalog.pickImageQuoteGroup(row, {})).toBeNull()
  })

  it('refuses a supplier group that the gateway does not advertise for this model', () => {
    expect(
      catalog.pickImageQuoteGroup(
        { ...row, enable_groups: ['default'] },
        {
          [economy]: { ratio: 0.1 },
        }
      )
    ).toBeNull()
  })

  it('does not redirect a different image model to the GPT reference supplier', () => {
    expect(
      catalog.pickImageQuoteGroup(
        { ...row, model_name: 'another-image-model' },
        {
          [economy]: { ratio: 0.1 },
        }
      )
    ).toBeNull()
  })
})
