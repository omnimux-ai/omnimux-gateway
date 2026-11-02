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
import type { LucideIcon } from 'lucide-react'
import type { ReactNode, RefObject } from 'react'

export interface PanelCardProps {
  panelRef?: RefObject<HTMLElement | null>
  icon: LucideIcon
  label: string
  /** Optional trailing control pinned to the right edge of the label row. */
  action?: ReactNode
  children: ReactNode
  contentHidden?: boolean
}

/**
 * A non-collapsible card in the same layered shape as the builder's
 * accordions: outer card + label head + darker inset box for the content.
 * The inset box only renders when there is content, so a card can stay
 * a compact single row while its body is switched off.
 */
export function PanelCard(props: PanelCardProps) {
  const Icon = props.icon
  return (
    <section ref={props.panelRef} className='inf-acc'>
      <div className='inf-acc-label'>
        <span className='inf-acc-icon'>
          <Icon className='h-4 w-4' aria-hidden='true' />
        </span>
        <span className='inf-acc-name'>{props.label}</span>
        {props.action ? (
          <span className='inf-acc-action'>{props.action}</span>
        ) : null}
      </div>
      {props.children ? (
        <div className='inf-acc-box' hidden={props.contentHidden}>
          {props.children}
        </div>
      ) : null}
    </section>
  )
}
