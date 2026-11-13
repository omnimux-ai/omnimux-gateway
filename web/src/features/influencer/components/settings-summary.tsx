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

import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

import { filterSelectionForTier, selectedLabels } from '../lib/taxonomy'
import type { InfluencerTaxonomy, Selection } from '../types'
import { getOptionLabelKey } from './option-label'

interface SummaryProps {
  labels: string[]
  disabled?: boolean
  onReset?: () => void
}

function SettingsSummary(props: SummaryProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  if (!props.labels.length) return null
  const text = props.labels.join(' · ')
  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className='inf-settings-summary'
      aria-label={t('Selected settings')}
    >
      <div className='inf-settings-head'>
        <span className='inf-settings-title'>{t('Selected settings')}</span>
        <CollapsibleTrigger
          render={<Button variant='ghost' className='inf-settings-action' />}
        >
          {open ? t('Show less') : t('Show more')}
        </CollapsibleTrigger>
        {props.onReset ? (
          <Button
            variant='ghost'
            className='inf-settings-action'
            disabled={props.disabled}
            onClick={props.onReset}
          >
            {t('Reset features')}
          </Button>
        ) : null}
      </div>
      {!open ? (
        <p className='inf-settings-preview' aria-live='polite'>
          {text}
        </p>
      ) : null}
      <CollapsibleContent>
        <p className='inf-settings-expanded' aria-live='polite'>
          {text}
        </p>
      </CollapsibleContent>
    </Collapsible>
  )
}

interface CharacterProps {
  taxonomy: InfluencerTaxonomy
  tier: string
  selection: Selection
  disabled?: boolean
  onReset: () => void
}

export function CharacterSettingsSummary(props: CharacterProps) {
  const { t } = useTranslation()
  const valid = filterSelectionForTier(
    props.taxonomy.categories,
    props.tier,
    props.selection
  ).selection
  const labelCategories = props.taxonomy.categories.map((category) => ({
    ...category,
    options: category.options.map((option) => ({
      ...option,
      label_en: getOptionLabelKey(category.id, option),
    })),
  }))
  const labels = selectedLabels(labelCategories, valid).map((label) => t(label))
  return (
    <SettingsSummary
      labels={labels}
      disabled={props.disabled}
      onReset={props.onReset}
    />
  )
}
