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
import { useDownload } from './download-context'
import { Icon } from './icons'

export function CnFinalCta() {
  const { open, release } = useDownload()
  const copy = useCopy()
  const finalCta = copy.finalCta
  const unavailable = release.status === 'unavailable'
  const disabledTitle = unavailable ? copy.download.unavailable : undefined
  return (
    <section className='sec final' id='download'>
      <div className='wrap'>
        <div className='final-card'>
          <h2 className='h2'>{finalCta.h2}</h2>
          <p className='lead'>{finalCta.lead}</p>
          <div className='btns'>
            <button
              className='btn btn-primary'
              type='button'
              disabled={unavailable}
              title={disabledTitle}
              onClick={() => open('mac-arm64')}
            >
              <Icon name='apple' filled />
              {finalCta.dlMac}
            </button>
            <button
              className='btn btn-ghost'
              type='button'
              disabled={unavailable}
              title={disabledTitle}
              onClick={() => open('win')}
            >
              <Icon name='win' filled />
              {finalCta.dlWin}
            </button>
          </div>
          <small>{finalCta.note}</small>
        </div>
        <div className='bigmark' aria-hidden='true'>
          OmniMux
        </div>
      </div>
    </section>
  )
}
