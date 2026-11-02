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
import { useEffect, useState } from 'react'

import { getLobeIcon } from '@/lib/lobe-icon'

import { useCopy } from '../../i18n'
import { MODEL_BRAND_ICONS, type IconName } from './data'
import { usePrefersReducedMotion } from './hooks'
import { Icon } from './icons'

function ToolList(props: { items: readonly { icon: IconName; label: string }[] }) {
  return (
    <ul>
      {props.items.map((item) => (
        <li key={item.label}>
          <Icon name={item.icon} />
          {item.label}
        </li>
      ))}
    </ul>
  )
}

export function CnTools() {
  const [active, setActive] = useState(0)
  const reduced = usePrefersReducedMotion()
  const copy = useCopy().tools
  const groups = copy.groups

  useEffect(() => {
    if (reduced) return
    const timer = window.setInterval(
      () => setActive((index) => (index + 1) % copy.models.length),
      2000
    )
    return () => window.clearInterval(timer)
  }, [reduced, copy.models.length])

  const left = groups[0]
  const right = groups[4]
  const bottom = groups[5]
  const center = groups.slice(1, 4)

  return (
    <section className='sec' id='tools'>
      <div className='wrap'>
        <div className='sec-head'>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
        </div>

        <div className='tools-map'>
          <div className='tm tm-left'>
            <h3>{left.title}</h3>
            <p>{left.lead}</p>
            <ToolList items={left.items} />
            <span className='tm-link' aria-hidden='true' />
          </div>
          <div className='tm tm-center'>
            {center.map((group) => (
              <div className='tm-col' key={group.title}>
                <h3>{group.title}</h3>
                <p>{group.lead}</p>
                <ToolList items={group.items} />
              </div>
            ))}
            <div className='tm-label'>{copy.centerLabel}</div>
          </div>
          <div className='tm tm-right'>
            <h3>{right.title}</h3>
            <p>{right.lead}</p>
            <ToolList items={right.items} />
            <span className='tm-link' aria-hidden='true' />
          </div>
        </div>

        <div className='tm tm-bottom'>
          <div>
            <h3>{bottom.title}</h3>
            <p style={{ fontSize: 13, color: 'var(--fg-3)', marginTop: 2 }}>
              {bottom.lead}
            </p>
          </div>
          <ToolList items={bottom.items} />
        </div>

        <div className='models' id='models'>
          <h3>{copy.modelsTitle}</h3>
          <div className='models-grid'>
            <div className='wheel' aria-label={copy.modelsAria}>
              <ul>
                {copy.models.map((model, index) => (
                  <li key={model.name} className={index === active ? 'on' : undefined}>
                    <i>
                      {MODEL_BRAND_ICONS[model.name]
                        ? getLobeIcon(MODEL_BRAND_ICONS[model.name], 18)
                        : model.mark}
                    </i>
                    {model.name}
                    <span>{model.kind}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className='chiprows' aria-label={copy.chipsAria}>
              {copy.capabilityRows.map((row, rowIndex) => (
                <div
                  className={`chiprow${rowIndex === 1 ? ' rev' : ''}`}
                  key={row[0].label}
                >
                  {row.map((chip) => (
                    <span key={chip.label}>
                      <Icon name={chip.icon} />
                      {chip.label}
                    </span>
                  ))}
                  {row.map((chip) => (
                    <span key={`dup-${chip.label}`} aria-hidden='true'>
                      <Icon name={chip.icon} />
                      {chip.label}
                    </span>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
