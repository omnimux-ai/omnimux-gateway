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

import { ChevronDown, Shapes } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { visOpts } from '../lib/taxonomy'
import type { InfluencerTaxonomy, LookLocks, Selection } from '../types'
import { CategoryGlyph, ICON_SPECS, type IconSpec } from './category-icons'
import { OptionCard } from './option-card'

/**
 * Per-category head icon mirrored from the reference panel's sidebar
 * (`icon + name · count` accordion header). Ids match the taxonomy exactly;
 * an unmapped category still gets a neutral glyph instead of a missing one.
 */
function catIconSpec(catId: string): IconSpec | null {
  return ICON_SPECS[ICON_KEY[catId]] ?? null
}

/*
 * Taxonomy category id -> icon spec. Ids match
 * service/influencer/data/taxonomy.json. 'tier' covers the Exaggeration
 * (Character type) block, which is not a taxonomy category but shares the
 * builder's visual language.
 */
const ICON_KEY: Record<string, keyof typeof ICON_SPECS> = {
  tier: 'character',
  gender: 'gender',
  body_type: 'build',
  hair: 'hairstyle',
  hair_colour: 'hairColor',
  aesthetic: 'style',
  ethnicity_origin_base: 'ethnicity',
  age: 'age',
  skin_tone: 'skinColor',
  height: 'height',
  proportions: 'proportions',
  freak_head: 'headShape',
  freak_neck: 'neck',
  eye_shape: 'eyeShape',
  eye_color: 'eyeColor',
  freak_face: 'features',
  facial_hair: 'facialHair',
  distinctive: 'distinctive',
  accessory: 'accessories',
}

interface AccHeadProps {
  icon: IconSpec | null
  name: string
  count: number
  open: boolean
  onToggle: () => void
}

/** The accordion's clickable header: icon, `name · count`, chevron. */
function AccHead(props: AccHeadProps) {
  return (
    <Button
      variant='ghost'
      type='button'
      className='inf-acc-head'
      onClick={props.onToggle}
      aria-expanded={props.open}
    >
      <span className='inf-acc-icon'>
        {props.icon ? (
          <CategoryGlyph icon={props.icon} className='h-4 w-4' />
        ) : (
          <Shapes className='h-4 w-4' aria-hidden='true' />
        )}
      </span>
      <span className='inf-acc-name'>{props.name}</span>
      <span className='inf-acc-count'>· {props.count}</span>
      <ChevronDown
        className='inf-acc-chev h-4 w-4'
        aria-hidden='true'
        style={{ transform: props.open ? 'rotate(180deg)' : 'none' }}
      />
    </Button>
  )
}

interface Props {
  referenceSlot?: ReactNode
  taxonomy: InfluencerTaxonomy
  tier: string
  selection: Selection
  /** Categories the visitor owns by hand; their tiles carry the lock badge. */
  locks: LookLocks
  openGroups: Set<string>
  onTier: (tier: string) => void
  onPick: (catId: string, optId: string) => void
  onToggleGroup: (catId: string) => void
}

/**
 * The character builder's section stack: a column of collapsible cards, each
 * holding an inset tray of white option tiles — the card-layered arrangement
 * measured off the reference product's left panel.
 */
export function BuilderPanel(props: Props) {
  const { t } = useTranslation()
  const tax = props.taxonomy
  // The exaggeration tier is always the first card and is not part of the
  // parent's open-group set, so it carries its own disclosure state.
  const [tierOpen, setTierOpen] = useState(true)
  const orderedCats = [...tax.categories].sort((a, b) => {
    const pa = tax.category_priority.indexOf(a.id)
    const pb = tax.category_priority.indexOf(b.id)
    return (pa < 0 ? 99 : pa) - (pb < 0 ? 99 : pb)
  })

  return (
    <div className='inf-panel-pad inf-sidebar-content flex shrink-0 flex-col gap-2'>
      {/* One description node names the lock badge for every locked tile. */}
      <span id='inf-lock-desc' className='sr-only'>
        {t('Locked')}
      </span>
      {props.referenceSlot}
      {/* 夸张度与已选特征不受参考图片影响。 */}
      <div className='inf-acc inf-acc-tier'>
        <AccHead
          icon={catIconSpec('tier')}
          name={t('Character type')}
          count={tax.tier_group.options.length}
          open={tierOpen}
          onToggle={() => setTierOpen((open) => !open)}
        />
        {tierOpen ? (
          <div className='inf-acc-body'>
            {tax.tier_group.options.map((o) => (
              <OptionCard
                key={o.id}
                categoryId='tier'
                kind='media'
                option={o}
                selected={props.tier === o.id}
                onClick={() => props.onTier(o.id)}
              />
            ))}
          </div>
        ) : null}
      </div>

      {/* 2. 其余特征类别（性别、体型、发型、发色、风格、族裔等） */}
      {orderedCats.map((c) => {
        const opts = visOpts(c, props.tier)
        if (!opts.length) return null
        const open = props.openGroups.has(c.id)
        return (
          <div key={c.id} className='inf-acc'>
            <AccHead
              icon={catIconSpec(c.id)}
              name={t(c.label_en)}
              count={opts.length}
              open={open}
              onToggle={() => props.onToggleGroup(c.id)}
            />
            {open ? (
              <div className='inf-acc-body'>
                {opts.map((o) => (
                  <OptionCard
                    key={o.id}
                    categoryId={c.id}
                    kind={c.kind}
                    option={o}
                    selected={(props.selection[c.id] ?? []).includes(o.id)}
                    locked={(props.locks[c.id] ?? []).includes(o.id)}
                    lockDescriptionId='inf-lock-desc'
                    onClick={() => props.onPick(c.id, o.id)}
                  />
                ))}
              </div>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}
