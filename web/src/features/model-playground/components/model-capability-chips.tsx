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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { PricingModel } from '@/features/pricing/types'

import { buildParamSchema } from '../lib/param-schema'

export type ModelCapabilityChipsProps = {
  model: PricingModel
  /** Selecting a chip prefills the matching parameter combination. */
  onPick?: (capability: string) => void
}

/**
 * Capability chips for the model page header.
 *
 * Chips describe input shapes, not model IDs: OmniMux exposes one public ID per
 * capability and lets request parameters pick the route, so no derivative ID is
 * ever shown here.
 */
export function ModelCapabilityChips(props: ModelCapabilityChipsProps) {
  const { t } = useTranslation()
  const [selected, setSelected] = useState<string[]>([])

  const capabilities = useMemo(
    () => buildParamSchema(props.model).capabilities,
    [props.model]
  )

  if (capabilities.length === 0) return null

  return (
    <ToggleGroup
      data-parity='capabilities.chips'
      value={selected}
      onValueChange={(values) => {
        setSelected(values)
        const next = values.at(-1)
        if (next) props.onPick?.(next)
      }}
      variant='outline'
      size='sm'
      aria-label={t('Model capabilities')}
      className='flex-wrap'
    >
      {capabilities.map((capability) => (
        <ToggleGroupItem key={capability} value={capability}>
          {t(capability)}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
