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
import { useEffect, useRef } from 'react'

import { useCopy } from '../../i18n'
import { ASSETS } from './data'
import { useDownload } from './download-context'
import { useDemoClock } from './hooks'
import { Icon } from './icons'
import { LazyVideo } from './lazy-video'
import { Tabbed } from './tabs'

const SET_TILE_MEDIA = [
  ASSETS.d1,
  ASSETS.d3,
  ASSETS.d5,
  ASSETS.d6,
  ASSETS.d4,
  ASSETS.d2,
  ASSETS.d7,
  ASSETS.d9,
] as const

const NODE_PAIRS = [
  ['n-img', 'n-ai'],
  ['n-txt', 'n-ai'],
  ['n-ai', 'n-vid'],
  ['n-vid', 'n-cut'],
] as const

export function CnCreate() {
  const { ref, elapsed } = useDemoClock(7000)
  const { open } = useDownload()
  const shown = (at: number) => (elapsed >= at ? ' in' : '')
  const tileShown = (index: number) => elapsed >= 3700 + index * 220
  const copy = useCopy().create

  return (
    <section className='sec' id='create'>
      <div className='wrap'>
        <div className='sec-head'>
          <span className='tag'>
            <img src='/logo.svg' alt='' />
            {copy.tag}
          </span>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
        </div>

        <div className='card cr' ref={ref}>
          <div className='chat'>
            <div className='chat-log'>
              <div className={`msg-user step${shown(0)}`}>
                <div className='linkcard' style={{ margin: '0 0 8px' }}>
                  <img src={ASSETS.d1} alt='' loading='lazy' decoding='async' />
                  <span>{copy.msgProductCard}</span>
                </div>
                {copy.msgAsk}
              </div>
              <div className={`done step${shown(800)}`}>
                <Icon name='check' />
                {copy.doneAnalyze}
              </div>
              <div className={`done step${shown(1400)}`}>
                <Icon name='check' />
                {copy.donePlan}
              </div>
              <div className={`done step${shown(2000)}`}>
                <Icon name='check' />
                {copy.doneConsistent}
              </div>
              <div className={`confirm step${shown(2600)}`}>
                <div className='confirm-top'>
                  <b>{copy.confirmTitle}</b>
                  <span className='need'>{copy.confirmNeed}</span>
                </div>
                <div className='params'>
                  <span>GPT Image 2.5</span>
                  <span>1:1</span>
                  <span>{copy.paramCount}</span>
                </div>
                <div className='confirm-actions'>
                  <button
                    className={`btn btn-primary${
                      elapsed >= 3300 ? ' pressed' : ''
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
              <div className={`done step${shown(5600)}`}>
                <Icon name='check' />
                {copy.doneResult}
              </div>
            </div>
          </div>

          <div className='canvas'>
            <Tabbed
              idPrefix='cn-canvas'
              label={copy.tabsAria}
              items={[
                {
                  id: 'canvas',
                  label: (
                    <>
                      <Icon name='grid' />
                      {copy.tabs.canvas}
                    </>
                  ),
                  panelClassName: 'dotgrid',
                  content: (
                    <>
                      <span className='grp'>
                        <Icon name='scan' />
                        {copy.groupLabel}
                      </span>
                      <div className='set'>
                        {SET_TILE_MEDIA.map((src, index) => (
                          <figure
                            key={src}
                            className={tileShown(index) ? 'in' : undefined}
                          >
                            <img src={src} alt='' loading='lazy' decoding='async' />
                            <figcaption>{copy.setCaptions[index]}</figcaption>
                          </figure>
                        ))}
                      </div>
                      <div className='zoom' aria-hidden='true'>
                        <Icon name='pointer' />
                        <Icon name='hand' />
                        <i />−<span>100%</span>+<i />
                        <Icon name='dl' />
                      </div>
                    </>
                  ),
                },
                {
                  id: 'assets',
                  label: (
                    <>
                      <Icon name='folder' />
                      {copy.tabs.assets}
                    </>
                  ),
                  panelClassName: 'dotgrid',
                  content: (
                    <div className='lib'>
                      {[
                        ASSETS.polka,
                        ASSETS.kitchen,
                        ASSETS.d8,
                        ASSETS.tryonProduct,
                      ].map((src, index) => (
                        <figure key={src}>
                          <img src={src} alt='' loading='lazy' decoding='async' />
                          <figcaption>
                            {copy.libAssets[index].label}
                            <span>{copy.libAssets[index].sub}</span>
                          </figcaption>
                        </figure>
                      ))}
                    </div>
                  ),
                },
                {
                  id: 'inspiration',
                  label: (
                    <>
                      <Icon name='bulb' />
                      {copy.tabs.inspiration}
                    </>
                  ),
                  panelClassName: 'dotgrid',
                  content: (
                    <div className='lib'>
                      {[
                        ASSETS.ugcPoster,
                        ASSETS.redPoster,
                        ASSETS.serumPoster,
                        ASSETS.ugcPoster,
                      ].map((src, index) => (
                        <figure key={copy.libInspiration[index].label}>
                          <img src={src} alt='' loading='lazy' decoding='async' />
                          <figcaption>
                            {copy.libInspiration[index].label}
                            <span>{copy.libInspiration[index].sub}</span>
                          </figcaption>
                        </figure>
                      ))}
                    </div>
                  ),
                },
                {
                  id: 'products',
                  label: (
                    <>
                      <Icon name='box' />
                      {copy.tabs.products}
                    </>
                  ),
                  panelClassName: 'dotgrid',
                  content: (
                    <ul className='plist'>
                      {[ASSETS.d1, ASSETS.durabilityProduct, ASSETS.canSq].map(
                        (src, index) => (
                          <li key={src}>
                            <img src={src} alt='' loading='lazy' decoding='async' />
                            <div>
                              {copy.libProducts[index].name}
                              <span>{copy.libProducts[index].sub}</span>
                            </div>
                            <em>{copy.libProducts[index].count}</em>
                          </li>
                        )
                      )}
                    </ul>
                  ),
                },
              ]}
            />
          </div>
        </div>

        <div className='cap'>
          <h3>{copy.capAgentTitle}</h3>
          <p>{copy.capAgentDesc}</p>
          <button className='btn btn-light' type='button' onClick={() => open()}>
            {copy.capAgentCta}
            <Icon name='right' />
          </button>
        </div>

        <CanvasGraph aria={copy.canvasAria} copy={copy} />

        <div className='cap'>
          <h3>{copy.capCanvasTitle}</h3>
          <p>{copy.capCanvasDesc}</p>
          <button className='btn btn-light' type='button' onClick={() => open()}>
            {copy.capCanvasCta}
            <Icon name='right' />
          </button>
        </div>
      </div>
    </section>
  )
}

function CanvasGraph(props: {
  aria: string
  copy: ReturnType<typeof useCopy>['create']
}) {
  const nodesRef = useRef<HTMLDivElement>(null)
  const linksRef = useRef<SVGSVGElement>(null)
  const copy = props.copy

  useEffect(() => {
    const nodes = nodesRef.current
    const links = linksRef.current
    if (!nodes || !links) return

    const draw = () => {
      const base = nodes.getBoundingClientRect()
      if (!base.width || !base.height) return
      links.setAttribute('viewBox', `0 0 ${base.width} ${base.height}`)
      links.innerHTML = NODE_PAIRS.map(([from, to]) => {
        const start = document.querySelector<HTMLElement>(`#${from}`)?.getBoundingClientRect()
        const end = document.querySelector<HTMLElement>(`#${to}`)?.getBoundingClientRect()
        if (!start || !end) return ''
        const x1 = start.right - base.left
        const y1 = start.top + start.height / 2 - base.top
        const x2 = end.left - base.left
        const y2 = end.top + end.height / 2 - base.top
        const curve = Math.max(30, (x2 - x1) / 2)
        return `<path d="M${x1} ${y1} C ${x1 + curve} ${y1}, ${
          x2 - curve
        } ${y2}, ${x2} ${y2}"/><circle cx="${x1}" cy="${y1}" r="4.5"/><circle cx="${x2}" cy="${y2}" r="4.5"/>`
      }).join('')
    }

    draw()
    const observer = new ResizeObserver(draw)
    observer.observe(nodes)
    window.addEventListener('load', draw)
    return () => {
      observer.disconnect()
      window.removeEventListener('load', draw)
    }
  }, [])

  return (
    <div
      className='card flowcanvas'
      style={{ marginTop: 'clamp(56px,7vw,88px)' }}
      aria-label={props.aria}
    >
      <div className='dotgrid' />
      <div className='nodes' ref={nodesRef}>
        <svg className='links' ref={linksRef} aria-hidden='true' />
        <div className='node' id='n-img' style={{ left: '3%', top: 34 }}>
          <header>
            <Icon name='img' />
            {copy.nodeImage}
          </header>
          <div className='nb'>
            <img src={ASSETS.d1} alt='' style={{ aspectRatio: '4 / 3' }} loading='lazy' decoding='async' />
          </div>
        </div>
        <div className='node' id='n-txt' style={{ left: '3%', top: 262 }}>
          <header>
            <Icon name='type' />
            {copy.nodeText}
          </header>
          <div className='nb'>
            <p>{copy.nodeTextBody}</p>
          </div>
        </div>
        <div
          className='node n-sel'
          id='n-ai'
          style={{ left: '30%', top: 80 }}
        >
          <header>
            <Icon name='spark' />
            {copy.nodeAIImage}
            <span>{copy.nodeDone}</span>
          </header>
          <div className='nb'>
            <img src={ASSETS.d4} alt='' loading='lazy' decoding='async' />
          </div>
          <span className='model'>GPT Image 2.5</span>
        </div>
        <div className='node' id='n-vid' style={{ left: '56%', top: 80 }}>
          <header>
            <Icon name='video' />
            {copy.nodeAIVideo}
            <span>{copy.nodeDone}</span>
          </header>
          <div className='nb'>
            <LazyVideo src={ASSETS.drinkAd} poster={ASSETS.drinkPoster} />
          </div>
          <span className='model'>Seedance 2.5 · 12s</span>
        </div>
        <div className='node' id='n-cut' style={{ left: '80.5%', top: 150 }}>
          <header>
            <Icon name='cut' />
            {copy.nodeCut}
            <span>{copy.nodeDone}</span>
          </header>
          <div className='nb'>
            <p>
              {copy.nodeCutBodyA}
              <br />
              {copy.nodeCutBodyB}
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
