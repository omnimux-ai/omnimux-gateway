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
import { Link } from '@tanstack/react-router'

import { Icon } from '../../components/cn/icons'
import { AGENT_VOICE_BARS, type AgentCard, type AgentMock } from '../content'

/**
 * The preview inside a capability card. Each variant is a static DOM mock of a
 * real product surface — no data, no interaction, so it carries no live state.
 */
export function AgentCardMock(props: { mock: AgentMock }) {
  const mock = props.mock

  if (mock.type === 'checklist') {
    return (
      <div className='agent-mock'>
        {mock.rows.map((row) => (
          <div className='agent-mrow' key={row.label}>
            <span>{row.label}</span>
            <i className={row.accent ? 'ok' : undefined}>{row.value}</i>
          </div>
        ))}
      </div>
    )
  }

  if (mock.type === 'prompt') {
    return (
      <div className='agent-prompt'>
        <span>{mock.text}</span>
        <Icon name='right' />
        <Icon name={mock.icon} />
      </div>
    )
  }

  if (mock.type === 'voice') {
    return (
      <div className='agent-voice'>
        <Icon name='mic' />
        {AGENT_VOICE_BARS.map((bar) => (
          <i key={bar.id} style={{ height: bar.height }} />
        ))}
      </div>
    )
  }

  if (mock.type === 'chips') {
    return (
      <div className='agent-chips'>
        {mock.chips.map((chip) => (
          <span className='agent-chip-tag' key={chip}>
            <Icon name='check' />
            {chip}
          </span>
        ))}
      </div>
    )
  }

  return (
    <>
      <div className='agent-pub'>
        {mock.rows.map((row) => (
          <div className='agent-pubrow' key={row.title}>
            <span className='play'>
              <Icon name='right' />
            </span>
            <span className='meta'>
              <b>{row.title}</b>
              <small>{row.meta}</small>
            </span>
            <span
              className={row.tone === 'busy' ? 'agent-pill busy' : 'agent-pill'}
            >
              <i />
              {row.status}
            </span>
          </div>
        ))}
      </div>
      <div className='agent-chips'>
        {mock.chips.map((chip) => (
          <span className='agent-chip-tag' key={chip}>
            <Icon name='check' />
            {chip}
          </span>
        ))}
      </div>
    </>
  )
}

/** The capability grid; the Skills page simply carries more cards. */
export function AgentCards(props: { cards: AgentCard[] }) {
  return (
    <div className='agent-caps'>
      {props.cards.map((card) => (
        <article className='agent-card' key={card.title}>
          <h3>{card.title}</h3>
          <p>{card.body}</p>
          <AgentCardMock mock={card.mock} />
        </article>
      ))}
    </div>
  )
}

/** The other Agent page, so each page can point at its sibling. */
export function AgentKindTabs(props: {
  current: 'cli' | 'skills'
  paths: { cli: string; skills: string }
  label: string
  labels: { cli: string; skills: string }
}) {
  const items = [
    { key: 'cli' as const, path: props.paths.cli, icon: 'term' as const },
    {
      key: 'skills' as const,
      path: props.paths.skills,
      icon: 'spark' as const,
    },
  ]
  return (
    <nav className='c-tabs agent-kind' aria-label={props.label}>
      {items.map((item) => (
        <Link
          key={item.key}
          to={item.path}
          aria-current={item.key === props.current ? 'page' : undefined}
        >
          <Icon name={item.icon} />
          {props.labels[item.key]}
        </Link>
      ))}
    </nav>
  )
}
