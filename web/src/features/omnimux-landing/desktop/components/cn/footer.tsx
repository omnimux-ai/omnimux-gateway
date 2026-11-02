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

import { useCopy } from '../../i18n'

/** `anchorBase` (the tool pages pass `/`) points `#…` links at the homepage. */
export function CnFooter(props: { anchorBase?: string }) {
  const copy = useCopy().footer
  const anchorBase = props.anchorBase ?? ''
  return (
    <footer className='foot'>
      <div className='wrap'>
        <div className='foot-grid'>
          <div className='foot-brand'>
            <a className='brand' href={anchorBase || '#top'}>
              <img src='/logo.svg' alt='' />
              <b>OmniMux</b>
              <small>{copy.brand}</small>
            </a>
            <p>{copy.tagline}</p>
          </div>
          {copy.columns.map((column) => (
            <div key={column.title}>
              <h4>{column.title}</h4>
              <ul>
                {column.links.map((link) => (
                  <li key={link.label}>
                    {link.external || link.href.startsWith('#') ? (
                      <a
                        href={
                          link.href.startsWith('#')
                            ? `${anchorBase}${link.href}`
                            : link.href
                        }
                      >
                        {link.label}
                      </a>
                    ) : (
                      <Link to={link.href}>{link.label}</Link>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className='foot-bottom'>
          <span>© {new Date().getFullYear()} OmniMux</span>
          <a
            href='https://github.com/omnimux-ai/omnimux-ai'
            rel='noopener noreferrer'
            target='_blank'
          >
            {copy.source}
          </a>
          <a href='https://github.com/QuantumNous/new-api' rel='noopener'>
            {copy.powered}
          </a>
        </div>
      </div>
    </footer>
  )
}
