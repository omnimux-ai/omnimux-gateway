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

export function CnSurfaces() {
  const { open, downloadDirect } = useDownload()
  const copy = useCopy().surfaces

  return (
    <section className='sec' id='surfaces'>
      <div className='wrap'>
        <div className='sec-head'>
          <h2 className='h2'>{copy.h2}</h2>
          <p className='lead'>{copy.lead}</p>
        </div>
        <div className='surf'>
          <article className='sc'>
            <div className='sc-icons'>
              <span>
                <img src='/logo.svg' alt='' />
              </span>
            </div>
            <h3>{copy.desktop.title}</h3>
            <p>{copy.desktop.desc}</p>
            <div className='sc-mock'>
              <div>
                <div className='mini-bar'>
                  <span className='dots'>
                    <i />
                    <i />
                    <i />
                  </span>
                  OmniMux
                </div>
                <div className='mini-in'>
                  {copy.desktop.input}
                  <i>
                    <Icon name='up' />
                  </i>
                </div>
                <div className='mini-vids'>
                  <span>
                    <img src={ASSETS.durabilityPoster} alt='' loading='lazy' decoding='async' />
                  </span>
                  <span>
                    <img src={ASSETS.ugcPoster} alt='' loading='lazy' decoding='async' />
                  </span>
                  <span>
                    <img src={ASSETS.selfiePoster} alt='' loading='lazy' decoding='async' />
                  </span>
                  <span>
                    <img src={ASSETS.redPoster} alt='' loading='lazy' decoding='async' />
                  </span>
                </div>
              </div>
            </div>
            <button className='sc-cta' type='button' onClick={() => {
                if (!downloadDirect()) open()
              }}>
              {copy.desktop.cta}
              <Icon name='right' />
            </button>
          </article>

          <article className='sc'>
            <div className='sc-icons'>
              <span>
                <Icon name='term' />
              </span>
              <span>Codex</span>
              <span>Claude Code</span>
            </div>
            <h3>{copy.cli.title}</h3>
            <p>{copy.cli.desc}</p>
            <div className='sc-mock'>
              <div className='term'>
                <div>
                  <b>$</b> npm install -g @omnimux/cli
                </div>
                <div>
                  <b>$</b> omnimux skill install --user
                </div>
                <div className='okl'>✓ Skills installed to ~/.agents/skills</div>
              </div>
            </div>
            <a className='sc-cta' href='https://docs.omnimux.ai'>
              {copy.cli.cta}
              <Icon name='right' />
            </a>
          </article>

          <article className='sc' id='ext'>
            <div className='sc-icons'>
              <span>
                <Icon name='puzzle' />
              </span>
            </div>
            <h3>{copy.ext.title}</h3>
            <p>{copy.ext.desc}</p>
            <div className='sc-mock'>
              <div style={{ padding: 0, overflow: 'hidden' }}>
                <div className='ext'>
                  <img src={ASSETS.polkaB} alt='' loading='lazy' decoding='async' />
                  <div className='capsule'>
                    <span>
                      <Icon name='bulb' />
                      {copy.ext.capSave}
                    </span>
                    <span>
                      <Icon name='scan' />
                      {copy.ext.capScan}
                    </span>
                  </div>
                  <div className='ext-toast'>
                    <Icon name='check' />
                    {copy.ext.toast}
                  </div>
                </div>
              </div>
            </div>
            <button className='sc-cta' type='button' onClick={() => {
                if (!downloadDirect()) open()
              }}>
              {copy.ext.cta}
              <Icon name='right' />
            </button>
          </article>
        </div>
      </div>
    </section>
  )
}
