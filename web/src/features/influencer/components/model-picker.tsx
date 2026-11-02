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

import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { availableGroups, imageModels, vendorName } from '../lib/catalog'
import type { PricingResponse } from '../types'

interface Props {
  pricing: PricingResponse | undefined
  userGroups: Record<string, { ratio: number; desc?: string }>
  vendorId: number | null
  model: string
  group: string
  onVendor: (id: number | null) => void
  onModel: (name: string) => void
  onGroup: (name: string) => void
}

const labelCls = 'mb-1 block text-[11px] text-[var(--inf-txt-3)]'

export function ModelPicker(props: Props) {
  const { t } = useTranslation()
  const models = useMemo(() => imageModels(props.pricing), [props.pricing])
  const vendors = useMemo(() => {
    const ids = [...new Set(models.map((m) => m.vendor_id))]
    return ids.map((id) => ({ id, name: vendorName(props.pricing, id) }))
  }, [models, props.pricing])

  const filtered = useMemo(() => {
    if (props.vendorId == null) return models
    return models.filter((m) => m.vendor_id === props.vendorId)
  }, [models, props.vendorId])

  const row = useMemo(
    () => models.find((m) => m.model_name === props.model),
    [models, props.model]
  )
  const groups = useMemo(
    () => availableGroups(row, props.userGroups, props.pricing?.group_ratio),
    [row, props.userGroups, props.pricing]
  )

  return (
    <div className='flex flex-col gap-2.5'>
      <div>
        <label className={labelCls} htmlFor='inf-vendor'>
          {t('Brand')}
        </label>
        <NativeSelect
          id='inf-vendor'
          className='w-full'
          value={props.vendorId == null ? '' : String(props.vendorId)}
          onChange={(e) => {
            const next = e.target.value === '' ? null : Number(e.target.value)
            if (next !== props.vendorId) {
              props.onVendor(next)
              props.onModel('')
              props.onGroup('')
            }
          }}
        >
          <NativeSelectOption value=''>{t('All brands')}</NativeSelectOption>
          {vendors.map((v) => (
            <NativeSelectOption key={v.id} value={v.id}>
              {v.name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <div>
        <label className={labelCls} htmlFor='inf-model'>
          {t('Model')}
        </label>
        <NativeSelect
          id='inf-model'
          className='w-full'
          value={props.model}
          onChange={(e) => {
            props.onModel(e.target.value)
            props.onGroup('')
          }}
        >
          <NativeSelectOption value=''>{t('Pick a model')}</NativeSelectOption>
          {filtered.map((m) => (
            <NativeSelectOption key={m.model_name} value={m.model_name}>
              {m.model_name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <div>
        <label className={labelCls} htmlFor='inf-group'>
          {t('Group')}
        </label>
        <NativeSelect
          id='inf-group'
          className='w-full'
          value={props.group}
          onChange={(e) => props.onGroup(e.target.value)}
        >
          <NativeSelectOption value=''>{t('Default group')}</NativeSelectOption>
          {groups.map((g) => (
            <NativeSelectOption key={g.name} value={g.name}>
              {g.name}
              {g.ratio !== 1 ? ` ×${g.ratio}` : ''}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
    </div>
  )
}
