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
import { useTranslation } from 'react-i18next'

import { DEMO_BANNER_BODY, DEMO_BANNER_TITLE } from '../constants'

export function HostedDemoBanner() {
  const { t } = useTranslation()

  return (
    <div
      role='status'
      className='border-warning/40 bg-warning/10 text-warning rounded-lg border px-3 py-2 text-sm'
    >
      <p className='font-medium'>{t(DEMO_BANNER_TITLE)}</p>
      <p className='text-warning/90 mt-0.5'>{t(DEMO_BANNER_BODY)}</p>
    </div>
  )
}
