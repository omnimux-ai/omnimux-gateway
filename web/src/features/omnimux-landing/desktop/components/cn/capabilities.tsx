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
import { docsQuickstartUrl } from '../../../lib/landing-urls'
import { AGENT_PATHS } from '../../agent/content'
import { useLanding } from '../../i18n/context'
import { TOOL_PATHS } from '../../tools/content'
import { Icon } from './icons'

export function CnCapabilities() {
  const { lang, copy } = useLanding()
  const content = copy.capabilities
  const tools =
    TOOL_PATHS[lang === 'zh-CN' || lang === 'zh-TW' ? 'zh-CN' : 'en']
  const agents = AGENT_PATHS[lang]
  const resources = [
    { href: tools.video, label: copy.nav.toolsMenu.video },
    { href: tools.photo, label: copy.nav.toolsMenu.photo },
    { href: agents.cli, label: copy.nav.toolsMenu.cli },
    { href: agents.skills, label: copy.nav.toolsMenu.skills },
    { href: docsQuickstartUrl(lang), label: copy.nav.resourcesMenu.docs },
  ]

  return (
    <section className='sec' id='capabilities'>
      <span className='legacy-home-anchor' id='tools' aria-hidden='true' />
      <span className='legacy-home-anchor' id='ext' aria-hidden='true' />
      <div className='wrap'>
        <div className='sec-head'>
          <p className='home-section-label'>{content.kicker}</p>
          <h2 className='h2'>{content.h2}</h2>
          <p className='lead'>{content.lead}</p>
        </div>
        <ul className='home-capabilities'>
          {content.cards.map((card) => (
            <li className='card' key={card.title}>
              <h3>{card.title}</h3>
              <p>{card.body}</p>
            </li>
          ))}
        </ul>
        <nav className='home-resource-links' aria-label={copy.nav.resources}>
          {resources.map(({ href, label }) => (
            <a className='btn btn-ghost' key={href} href={href}>
              {label}
              <Icon name='right' />
            </a>
          ))}
        </nav>
      </div>
    </section>
  )
}
