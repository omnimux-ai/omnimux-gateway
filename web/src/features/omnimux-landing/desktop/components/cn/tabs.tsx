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
import { useRef, useState, type ReactNode } from 'react'

export type TabItem = {
  id: string
  label: ReactNode
  content: ReactNode
  panelClassName?: string
}

export function Tabbed(props: {
  idPrefix: string
  label: string
  items: readonly TabItem[]
}) {
  const [active, setActive] = useState(props.items[0]?.id)
  const buttons = useRef<(HTMLButtonElement | null)[]>([])

  const select = (index: number) => {
    const item = props.items[index]
    if (!item) return
    setActive(item.id)
    buttons.current[index]?.focus()
  }

  return (
    <>
      <div className='tabs' role='tablist' aria-label={props.label}>
        {props.items.map((item, index) => (
          <button
            key={item.id}
            ref={(node) => {
              buttons.current[index] = node
            }}
            type='button'
            role='tab'
            id={`${props.idPrefix}-t-${item.id}`}
            aria-selected={active === item.id}
            aria-controls={`${props.idPrefix}-p-${item.id}`}
            tabIndex={active === item.id ? 0 : -1}
            onClick={() => setActive(item.id)}
            onKeyDown={(event) => {
              if (event.key === 'ArrowRight') {
                event.preventDefault()
                select((index + 1) % props.items.length)
              }
              if (event.key === 'ArrowLeft') {
                event.preventDefault()
                select(
                  (index + props.items.length - 1) % props.items.length
                )
              }
            }}
          >
            {item.label}
          </button>
        ))}
      </div>
      {props.items.map((item) => (
        <div
          key={item.id}
          role='tabpanel'
          id={`${props.idPrefix}-p-${item.id}`}
          aria-labelledby={`${props.idPrefix}-t-${item.id}`}
          className={item.panelClassName}
          hidden={active !== item.id}
        >
          {item.content}
        </div>
      ))}
    </>
  )
}
