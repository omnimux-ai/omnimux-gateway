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
import { Fragment, useEffect, useRef, useState, type ReactNode } from 'react'

import { useCopy } from '../../i18n'
import type { FlowStepCopy } from '../../i18n/types'
import {
  ASSETS,
  POSTS_MEDIA,
  TRENDING_MEDIA,
  type IconName,
} from './data'
import { useDownload } from './download-context'
import { usePrefersReducedMotion } from './hooks'
import { Icon } from './icons'
import { LazyVideo } from './lazy-video'
import { Tabbed } from './tabs'

const STEP_ICONS: readonly IconName[] = [
  'flame',
  'scan',
  'clap',
  'send',
  'chart',
]

function Secondary(props: {
  index: number
  label?: string
  onOpen: () => void
}) {
  if (!props.label) return null
  if (props.index === 0) {
    return (
      <a className='btn btn-text' href='#ext'>
        {props.label}
      </a>
    )
  }
  if (props.index === 2) {
    return (
      <button className='btn btn-text' type='button' onClick={props.onOpen}>
        {props.label}
      </button>
    )
  }
  return <span className='btn btn-text'>{props.label}</span>
}

function Head(props: { index: number; step: FlowStepCopy }) {
  return (
    <div className='fp-head'>
      <span className='st-num'>{`0${props.index + 1}`}</span>
      <span className='st-k'>{props.step.kind}</span>
      <h3>{props.step.title}</h3>
      <p>{props.step.desc}</p>
    </div>
  )
}

export function CnFlow() {
  const [active, setActive] = useState(0)
  const rootRef = useRef<HTMLDivElement>(null)
  const { open } = useDownload()
  const reduced = usePrefersReducedMotion()
  const copy = useCopy().flow
  const STEPS = copy.steps

  useEffect(() => {
    const root = rootRef.current
    if (!root) return
    const panels = root.querySelectorAll<HTMLElement>('.fp')
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setActive(Number((entry.target as HTMLElement).dataset.i))
          }
        })
      },
      { rootMargin: '-42% 0px -52% 0px' }
    )
    panels.forEach((panel) => observer.observe(panel))
    return () => observer.disconnect()
  }, [])

  const goTo = (index: number) => {
    document.querySelector<HTMLElement>(`#fp${index}`)?.scrollIntoView({
      behavior: reduced ? 'auto' : 'smooth',
      block: 'center',
    })
  }

  const panel = (index: number, content: ReactNode) => (
    <div className='fp' id={`fp${index}`} data-i={index}>
      <Head index={index} step={STEPS[index]} />
      {content}
    </div>
  )

  return (
    <section className='sec' id='flow' ref={rootRef}>
      <div className='wrap'>
        <div className='sec-head'>
          <h2 className='h2'>{copy.h2}</h2>
          <div className='flow-line' aria-label={copy.lineAria}>
            {STEPS.map((step, index) => (
              <Fragment key={step.kind}>
                <span>{step.kind}</span>
                {index < STEPS.length - 1 ? <i /> : null}
              </Fragment>
            ))}
          </div>
        </div>

        <div className='flow-grid'>
          <ol className='steps' id='steps'>
            {STEPS.map((step, index) => (
              <li
                key={step.kind}
                className={`st${active === index ? ' on' : ''}`}
                data-i={index}
              >
                <button
                  className='st-head'
                  type='button'
                  onClick={() => goTo(index)}
                >
                  <span className='st-ico'>
                    <Icon name={STEP_ICONS[index]} />
                  </span>
                  <span>
                    <span className='st-num'>{`0${index + 1}`}</span>
                    <span className='st-k'>{step.kind}</span>
                    <span className='st-t'>{step.title}</span>
                  </span>
                </button>
                <div className='st-body'>
                  <div>
                    <p>{step.desc}</p>
                    {step.chips ? (
                      <div className='st-chips'>
                        {step.chips.map((chip) => (
                          <span key={chip}>{chip}</span>
                        ))}
                      </div>
                    ) : null}
                    <div className='st-cta'>
                      <button
                        className='btn btn-sm btn-light'
                        type='button'
                        onClick={() => open()}
                      >
                        {step.primary}
                        <Icon name='right' />
                      </button>
                      <Secondary index={index} label={step.secondary} onOpen={open} />
                    </div>
                  </div>
                </div>
              </li>
            ))}
          </ol>

          <div className='panels'>
            {panel(
              0,
              <div className='card'>
                <div className='card-h'>
                  <Icon name='flame' />
                  {copy.discover.title}
                  <span className='meta'>{copy.discover.meta}</span>
                </div>
                <div className='trend'>
                  {copy.discover.items.map((item, index) => {
                    const media = TRENDING_MEDIA[index]
                    return (
                      <article className='tv' key={item.title}>
                        <div className='frame'>
                          {media.video ? (
                            <LazyVideo src={media.video} poster={media.poster ?? ''} />
                          ) : (
                            <img src={media.image ?? ASSETS.polka} alt='' loading='lazy' />
                          )}
                          <div className='who'>
                            <i style={{ '--av': media.avatar } as React.CSSProperties} />
                            {item.who}
                          </div>
                          <div className='ov'>
                            <span>
                              <Icon name={media.video ? 'eye' : 'heart'} />
                              {item.views}
                            </span>
                            <span>{item.length}</span>
                          </div>
                        </div>
                        <h4>{item.title}</h4>
                        <div className='st2'>
                          <span>♥ {item.likes}</span>
                          <b>
                            {copy.discover.savesPrefix} {item.saves}
                          </b>
                        </div>
                      </article>
                    )
                  })}
                </div>
              </div>
            )}

            {panel(
              1,
              <div className='card'>
                <Tabbed
                  idPrefix='cn-breakdown'
                  label={copy.breakdown.tabsAria}
                  items={[
                    {
                      id: 'video',
                      label: copy.breakdown.tabs.video,
                      content: (
                        <div className='bd'>
                          <div className='frame'>
                            <LazyVideo
                              src={ASSETS.ugcDemo}
                              poster={ASSETS.ugcPoster}
                            />
                            <div className='stats'>
                              <span>{copy.breakdown.statsViews}</span>
                              <span>{copy.breakdown.statsLikes}</span>
                            </div>
                          </div>
                          <div className='why'>
                            <h4>{copy.breakdown.why}</h4>
                            <ul>
                              {copy.breakdown.rows.map((row) => (
                                <li key={row.label}>
                                  <div className='row'>
                                    {row.label}
                                    <span>{row.time}</span>
                                  </div>
                                  <p>{row.desc}</p>
                                </li>
                              ))}
                            </ul>
                            <div className='reuse'>
                              <small>{copy.breakdown.reuse}</small>
                              <div className='chain'>
                                <b>{copy.breakdown.chain[0]}</b>
                                <Icon name='right' />
                                <b>{copy.breakdown.chain[1]}</b>
                                <Icon name='right' />
                                <b className='cart'>{copy.breakdown.chain[2]}</b>
                              </div>
                            </div>
                          </div>
                        </div>
                      ),
                    },
                    {
                      id: 'account',
                      label: copy.breakdown.tabs.account,
                      content: (
                        <>
                          <div className='acct-top'>
                            <i />
                            <div>
                              <b>{copy.breakdown.accountName}</b>
                              <span>{copy.breakdown.accountMeta}</span>
                            </div>
                          </div>
                          <div className='thumbs'>
                            {[
                              ASSETS.polka,
                              ASSETS.pizza,
                              ASSETS.kitchenB,
                              ASSETS.redB,
                            ].map((src, index) => (
                              <div key={src}>
                                <img src={src} alt='' loading='lazy' />
                                <span>
                                  <Icon name='eye' />
                                  {copy.breakdown.thumbs[index]}
                                </span>
                              </div>
                            ))}
                          </div>
                          <div className='metrics'>
                            {copy.breakdown.metrics.map((metric) => (
                              <div key={metric.label}>
                                <small>{metric.label}</small>
                                <b>{metric.value}</b>
                              </div>
                            ))}
                          </div>
                          <ul className='insights'>
                            {copy.breakdown.insights.map((insight) => (
                              <li key={insight.k}>
                                <b>{insight.k}</b>
                                {insight.v}
                              </li>
                            ))}
                          </ul>
                          <button
                            className='btn btn-sm btn-light go'
                            type='button'
                            onClick={() => open()}
                          >
                            {copy.breakdown.accountCta}
                          </button>
                        </>
                      ),
                    },
                    {
                      id: 'category',
                      label: copy.breakdown.tabs.category,
                      content: (
                        <>
                          <div className='card-h' style={{ marginBottom: 12 }}>
                            {copy.breakdown.catTitle}
                            <span className='meta'>{copy.breakdown.catMeta}</span>
                          </div>
                          <div className='bars'>
                            {copy.breakdown.bars.map((bar) => (
                              <div className='bar' key={bar.label}>
                                {bar.label}
                                <i style={{ '--v': bar.value } as React.CSSProperties} />
                                <span>{bar.value}</span>
                              </div>
                            ))}
                          </div>
                          <ul className='insights'>
                            {copy.breakdown.catInsights.map((insight) => (
                              <li key={insight.k}>
                                <b>{insight.k}</b>
                                {insight.v}
                              </li>
                            ))}
                          </ul>
                          <button
                            className='btn btn-sm btn-light go'
                            type='button'
                            onClick={() => open()}
                          >
                            {copy.breakdown.catCta}
                          </button>
                        </>
                      ),
                    },
                  ]}
                />
              </div>
            )}

            {panel(
              2,
              <div className='card'>
                <Tabbed
                  idPrefix='cn-make'
                  label={copy.make.tabsAria}
                  items={[
                    {
                      id: 'generate',
                      label: copy.make.tabs.generate,
                      content: (
                        <>
                          <div className='make'>
                            <figure>
                              <figcaption>{copy.make.capProduct}</figcaption>
                              <div className='frame'>
                                <img
                                  src={ASSETS.durabilityProduct}
                                  alt={copy.make.altProduct} loading='lazy' decoding='async' />
                              </div>
                            </figure>
                            <div className='arrow'>
                              <i>
                                <Icon name='right' />
                              </i>
                              Seedance 2.5
                              <br />
                              12s
                            </div>
                            <figure>
                              <figcaption>{copy.make.capResult}</figcaption>
                              <div className='frame'>
                                <LazyVideo
                                  src={ASSETS.durability}
                                  poster={ASSETS.durabilityPoster}
                                />
                                <span className='badge'>{copy.make.badgeGenerated}</span>
                              </div>
                            </figure>
                          </div>
                          <div className='prompt'>
                            <small>{copy.make.promptLabel}</small>
                            <span>{copy.make.promptGenerate}</span>
                          </div>
                        </>
                      ),
                    },
                    {
                      id: 'replicate',
                      label: copy.make.tabs.replicate,
                      content: (
                        <>
                          <div className='make three'>
                            <figure>
                              <figcaption>{copy.make.capRef}</figcaption>
                              <div className='frame'>
                                <img src={ASSETS.pizza} alt={copy.make.altRef} loading='lazy' />
                                <span
                                  className='badge'
                                  style={{ color: 'var(--fg)', borderColor: 'var(--hair-3)' }}
                                >
                                  {copy.make.badgeRef}
                                </span>
                              </div>
                            </figure>
                            <figure>
                              <figcaption>{copy.make.capMine}</figcaption>
                              <div className='frame'>
                                <img src={ASSETS.can} alt={copy.make.altCan} loading='lazy' decoding='async' />
                              </div>
                            </figure>
                            <div className='arrow'>
                              <i>
                                <Icon name='right' />
                              </i>
                              {copy.make.arrowReplicateA}
                              <br />
                              {copy.make.arrowReplicateB}
                            </div>
                            <figure>
                              <figcaption>{copy.make.capResult}</figcaption>
                              <div className='frame'>
                                <LazyVideo
                                  src={ASSETS.selfie}
                                  poster={ASSETS.selfiePoster}
                                />
                                <span className='badge'>{copy.make.badgeGenerated}</span>
                              </div>
                            </figure>
                          </div>
                          <div className='prompt'>
                            <small>{copy.make.intentLabel}</small>
                            <span>{copy.make.promptReplicate}</span>
                          </div>
                        </>
                      ),
                    },
                  ]}
                />
              </div>
            )}

            {panel(
              3,
              <>
                <div className='card'>
                  <div className='card-h'>
                    <Icon name='cal' />
                    {copy.publish.title}{' '}
                    <span style={{ color: 'var(--fg-3)', fontWeight: 400 }}>
                      {copy.publish.posts.length}
                    </span>
                    <span className='meta'>{copy.publish.meta}</span>
                  </div>
                  <ul className='posts'>
                    {copy.publish.posts.map((post, index) => {
                      const media = POSTS_MEDIA[index]
                      return (
                        <li key={post.title}>
                          <img src={media.poster} alt='' loading='lazy' />
                          <div className='t'>
                            <b>{post.title}</b>
                            <span>
                              <span className={`plat p-${media.platform}`}>
                                {post.platformLabel}
                              </span>
                              {post.account} · {post.time}
                            </span>
                          </div>
                          <span className={`state ${media.stateClass}`}>
                            {post.state}
                          </span>
                        </li>
                      )
                    })}
                  </ul>
                </div>
                <div className='feat'>
                  {copy.publish.feats.map((feat, index) => (
                    <div key={feat.title}>
                      <h4>
                        <Icon name={(['layers', 'users', 'shield', 'clock'] as const)[index]} />
                        {feat.title}
                      </h4>
                      <p>{feat.desc}</p>
                    </div>
                  ))}
                </div>
              </>
            )}

            {panel(
              4,
              <div className='card'>
                <div className='card-h'>
                  <Icon name='chart' />
                  {copy.review.title}{' '}
                  <span
                    style={{
                      color: 'var(--ok)',
                      fontWeight: 400,
                      fontSize: 12.5,
                    }}
                  >
                    {copy.review.synced}
                  </span>
                  <span className='meta'>{copy.review.meta}</span>
                </div>
                <div className='kpis'>
                  {copy.review.kpis.map((kpi) => (
                    <div key={kpi.label}>
                      <small>{kpi.label}</small>
                      <b>{kpi.value}</b>
                      <em>{kpi.delta}</em>
                    </div>
                  ))}
                </div>
                <div className='chart' aria-hidden='true'>
                  {[38, 52, 44, 61, 58, 79, 94].map((height) => (
                    <i key={height} style={{ '--h': `${height}%` } as React.CSSProperties} />
                  ))}
                </div>
                <div className='days' aria-hidden='true'>
                  {copy.review.days.map((day) => (
                    <span key={day}>{day}</span>
                  ))}
                </div>
                <div className='split'>
                  <small>{copy.review.splitTitle}</small>
                  <div className='split-bar'>
                    <i />
                    <i />
                  </div>
                  <div className='split-leg'>
                    <span>{copy.review.splitLegs[0]}</span>
                    <span>{copy.review.splitLegs[1]}</span>
                  </div>
                </div>
                <p className='note'>{copy.review.note}</p>
                <button
                  className='btn btn-sm btn-light go'
                  type='button'
                  onClick={() => open()}
                >
                  {copy.review.cta}
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
