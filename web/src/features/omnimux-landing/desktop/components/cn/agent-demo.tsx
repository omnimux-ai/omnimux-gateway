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
import { REEL_MEDIA } from './data'
import { useDemoClock } from './hooks'
import { Icon } from './icons'
import { LazyVideo } from './lazy-video'

const VIDEO_READY_AT = [5200, 5600, 6000] as const
const SIDE_ICONS = [
  'chat',
  'apps',
  'flow',
  'folder',
  'bulb',
  'box',
  'send',
  'chart',
] as const
const TILE_INDICES = [0, 1, 2] as const

export function CnAgentDemo() {
  const { ref, elapsed, run } = useDemoClock(7600)
  const shown = (at: number) => (elapsed >= at ? ' in' : '')
  const ready = (index: number) => elapsed >= VIDEO_READY_AT[index]
  const tiles = TILE_INDICES.map((index) => REEL_MEDIA[index])
  const copy = useCopy().agentDemo

  return (
    <section className='sec' id='agent' style={{ paddingTop: 'clamp(56px,7vw,96px)' }}>
      <div className='wrap'>
        <div className='sec-head'>
          <span className='tag'>
            <img src='/logo.svg' alt='' />
            OmniMux Agent
          </span>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
        </div>

        <div className='win' ref={ref} aria-label={copy.windowAria}>
          <div className='win-bar'>
            <div className='dots' aria-hidden='true'>
              <i />
              <i />
              <i />
            </div>
            <div className='win-title'>
              <Icon name='chat' style={{ width: 15, height: 15 }} />
              <span>{copy.windowTitle}</span>
            </div>
            <span className={`win-status step${shown(6200)}`}>
              <Icon name='check' />
              {copy.windowStatus}
            </span>
            <button
              className='replay'
              type='button'
              aria-label={copy.replayAria}
              onClick={run}
            >
              <Icon name='undo' />
              <span>{copy.replay}</span>
            </button>
          </div>

          <div className='win-body'>
            <aside className='side' aria-hidden='true'>
              <img src='/logo.svg' alt='' />
              {copy.side.map((label, index) => (
                <button
                  key={label}
                  type='button'
                  className={index === 0 ? 'on' : undefined}
                  data-label={label}
                  tabIndex={-1}
                >
                  <Icon name={SIDE_ICONS[index]} />
                </button>
              ))}
              <span className='spacer' />
              <span className='me'>{copy.me}</span>
            </aside>

            <div className='chat'>
              <div className='chat-log'>
                <div className={`msg-user step${shown(0)}`}>
                  {copy.msgPre}{' '}
                  {copy.msgProducts.map((product) => (
                    <span className='mention-pill' key={product}>
                      {product}
                    </span>
                  ))}{' '}
                  {copy.msgPost}
                  <div className='linkcard'>
                    <img src={REEL_MEDIA[3].poster} alt='' loading='lazy' decoding='async' />
                    <span>
                      <Icon name='link' style={{ width: 13, height: 13 }} />
                      <em>{copy.linkCard}</em>
                    </span>
                  </div>
                </div>

                <div className={`step${shown(900)}`}>
                  <div className='done'>
                    <Icon name='check' />
                    {copy.doneBreakdown}
                  </div>
                  <dl className='kv' style={{ marginTop: 8 }}>
                    {copy.kv.map((row) => (
                      <div key={row.k}>
                        <dt>{row.k}</dt>
                        <dd>{row.v}</dd>
                      </div>
                    ))}
                  </dl>
                </div>

                <div className={`done step${shown(1700)}`}>
                  <Icon name='check' />
                  {copy.doneProducts}
                </div>
                <div className={`done step${shown(2300)}`}>
                  <Icon name='check' />
                  {copy.doneScripts}
                </div>

                <div className={`confirm step${shown(3000)}`}>
                  <div className='confirm-top'>
                    <b>{copy.confirmTitle}</b>
                    <span className='need'>{copy.confirmNeed}</span>
                  </div>
                  <div className='params'>
                    <span>Seedance 2.5</span>
                    <span>9:16</span>
                    <span>15s</span>
                    <span>720p</span>
                    <span className='cost'>{copy.confirmCost}</span>
                  </div>
                  <div className='confirm-actions'>
                    <button
                      className={`btn btn-primary${
                        elapsed >= 3700 ? ' pressed' : ''
                      }`}
                      type='button'
                      tabIndex={-1}
                    >
                      {copy.confirmGo}
                    </button>
                    <button className='btn btn-ghost' type='button' tabIndex={-1}>
                      {copy.confirmEdit}
                    </button>
                  </div>
                </div>
              </div>

              <div className='chat-input' aria-hidden='true'>
                <span className='ph'>{copy.inputPh}</span>
                <span className='send'>
                  <Icon name='up' style={{ width: 15, height: 15 }} />
                </span>
              </div>
            </div>

            <div className='results'>
              <div className={`res-h step${shown(4000)}`}>
                <b>{copy.resultsTitle}</b>
                <span>{copy.resultsMeta}</span>
              </div>
              <div className={`vids step${shown(4000)}`}>
                {tiles.map((tile, index) => (
                  <div
                    key={copy.vidNames[index]}
                    className={`vid${ready(index) ? ' ready' : ''}`}
                  >
                    <div className='frame'>
                      <LazyVideo
                        src={tile.src}
                        poster={tile.poster}
                        hold={!ready(index)}
                      />
                      <div className='shimmer'>{copy.generating}</div>
                    </div>
                    <p>
                      {copy.vidNames[index]}
                      <span>{copy.vidHooks[index]}</span>
                    </p>
                  </div>
                ))}
              </div>

              <div className={`sched step${shown(6900)}`}>
                <div className='sched-h'>
                  <Icon name='cal' style={{ width: 15, height: 15, color: 'var(--fg-2)' }} />
                  {copy.schedTitle}
                  <span className='ok'>
                    <Icon name='check' />
                    {copy.schedOk}
                  </span>
                </div>
                <ul>
                  {copy.vidNames.map((name, index) => (
                    <li key={name}>
                      <img src={tiles[index].poster} alt='' loading='lazy' decoding='async' />
                      {name}
                      <span className='plat p-dy'>{copy.schedPlatform}</span>
                      <time>{copy.schedTimes[index]}</time>
                    </li>
                  ))}
                </ul>
                <button className='btn btn-light' type='button' tabIndex={-1}>
                  {copy.schedAll}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
