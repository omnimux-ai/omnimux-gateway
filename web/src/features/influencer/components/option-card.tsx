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

import { Check, Lock } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import type { InfluencerOption } from '../types'
import { CategoryGlyph, ICON_SPECS } from './category-icons'
import { getOptionLabelKey } from './option-label'

interface Props {
  categoryId?: string
  kind: 'media' | 'color' | 'text'
  option: InfluencerOption
  selected: boolean
  /** The visitor picked this tile by hand, so the dice leaves it alone. */
  locked?: boolean
  /** Id of the shared hidden node that names the lock badge. */
  lockDescriptionId?: string
  onClick: () => void
}

export function OptionCard(props: Props) {
  const { t } = useTranslation()
  const label = t(getOptionLabelKey(props.categoryId, props.option))
  const gender = props.categoryId === 'gender'
  const tier = props.categoryId === 'tier'
  const compact = ['age', 'skin_tone', 'eye_color', 'hair_colour'].includes(
    props.categoryId ?? ''
  )
  const animalText =
    props.kind === 'text' &&
    ['animal_breed', 'coat_pattern'].includes(props.categoryId ?? '')
  const cls = `inf-opt inf-opt-${props.kind}${gender ? ' inf-opt-gender' : ''}${tier ? ' inf-opt-tier' : ''}${compact ? ' inf-opt-compact' : ''}${animalText ? ' inf-opt-animal-text' : ''}${props.selected ? ' sel' : ''}${props.locked ? ' inf-opt-locked' : ''}`
  let visual
  if (props.kind === 'color') {
    visual = (
      <span
        className='inf-swatch'
        style={{ background: props.option.swatch ?? 'var(--inf-surface-soft)' }}
      />
    )
  } else if (props.categoryId === 'age') {
    visual = <CategoryGlyph icon={ICON_SPECS.age} className='h-6 w-6' />
  } else if (props.kind !== 'text') {
    visual = props.option.imageUrl ? (
      <img
        className={gender ? 'inf-gender-icon' : 'inf-thumb'}
        loading='lazy'
        src={props.option.imageUrl}
        alt=''
      />
    ) : (
      <span className='inf-thumb' />
    )
  }
  return (
    <Button
      type='button'
      variant='ghost'
      className={cls}
      onClick={props.onClick}
      aria-label={label}
      aria-pressed={props.selected}
      aria-describedby={
        props.locked ? props.lockDescriptionId : undefined
      }
      data-locked={props.locked ? 'true' : undefined}
    >
      {visual ? (
        <span className='inf-opt-visual'>
          {visual}
          {compact && props.selected && props.kind === 'color' ? (
            <span className='inf-opt-check' aria-hidden='true'>
              <Check />
            </span>
          ) : null}
          {props.locked ? (
            <span className='inf-opt-lock' aria-hidden='true'>
              <Lock />
            </span>
          ) : null}
        </span>
      ) : null}
      <span className='inf-lbl'>{label}</span>
    </Button>
  )
}
