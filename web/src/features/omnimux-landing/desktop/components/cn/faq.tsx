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
import { useCopy } from '../../i18n'
import { Icon } from './icons'

export function CnFaq() {
  const copy = useCopy().faq
  return (
    <section className='sec' id='faq'>
      <div className='wrap'>
        <div className='sec-head'>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
        </div>
        <div className='faq-list'>
          {copy.items.map((item) => (
            <details key={item.q}>
              <summary>
                {item.q}
                <span className='pm'>
                  <Icon name='plus' />
                </span>
              </summary>
              <p>{item.a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  )
}
