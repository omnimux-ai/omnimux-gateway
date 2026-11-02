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
import { ASSETS } from './data'
import { useDownload } from './download-context'
import { Icon } from './icons'

const ROW_MEDIA = [
  { image: ASSETS.d1, used: 14, total: 20, percent: '70%' },
  { image: ASSETS.durabilityProduct, used: 9, total: 15, percent: '60%' },
  { image: ASSETS.tryonProduct, used: 6, total: 12, percent: '50%' },
  { image: ASSETS.canSq, used: 2, total: 10, percent: '20%' },
] as const

const LIST_ICONS = ['box', 'folder', 'bulb', 'send'] as const

export function CnWorkspace() {
  const { open } = useDownload()
  const copy = useCopy().workspace

  return (
    <section className='sec' id='workspace'>
      <div className='wrap ws'>
        <div>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
          <ul className='ws-list'>
            {copy.list.map((item, index) => (
              <li key={item.name}>
                <Icon name={LIST_ICONS[index]} />
                <span>
                  <b>{item.name}</b>
                  {item.desc}
                </span>
              </li>
            ))}
          </ul>
          <small>{copy.note}</small>
        </div>
        <div className='card tbl'>
          <div className='tbl-h'>
            <b>{copy.tableTitle}</b>
            <button className='btn btn-sm btn-light' type='button' onClick={() => open()}>
              <Icon name='plus' />
              {copy.addProduct}
            </button>
          </div>
          <table>
            <thead>
              <tr>
                <th>{copy.cols[0]}</th>
                <th>{copy.cols[1]}</th>
                <th>{copy.cols[2]}</th>
              </tr>
            </thead>
            <tbody>
              {copy.rows.map((row, index) => {
                const media = ROW_MEDIA[index]
                return (
                  <tr key={row.name}>
                    <td>
                      <div className='pn'>
                        <img src={media.image} alt='' loading='lazy' decoding='async' />
                        {row.name}
                        <span>{row.category}</span>
                      </div>
                    </td>
                    <td>{row.points}</td>
                    <td>
                      <div className='prog'>
                        <div>
                          {media.used}
                          <span>/ {media.total}</span>
                        </div>
                        <i style={{ '--v': media.percent } as React.CSSProperties} />
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  )
}
