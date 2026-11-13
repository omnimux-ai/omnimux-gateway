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

export function CnProof() {
  const copy = useCopy().proof
  return (
    <section className='sec proof' id='proof'>
      <div className='wrap'>
        <h2 className='h2'>{copy.h2}</h2>
        <p className='lead' style={{ marginTop: 14 }}>
          {copy.lead}
        </p>
        <div className='facts'>
          {copy.facts.map((fact) => (
            <div key={fact.label}>
              <b>{fact.value}</b>
              <span>{fact.label}</span>
            </div>
          ))}
        </div>
        <div className='platforms' aria-label={copy.platformsAria}>
          {copy.platforms.map((platform) => (
            <span key={platform}>{platform}</span>
          ))}
        </div>
        <div className='badges'>
          <span>
            <Icon name='shield' />
            {copy.badges[0]}
          </span>
          <span>
            <Icon name='undo' />
            {copy.badges[1]}
          </span>
        </div>
      </div>
    </section>
  )
}
