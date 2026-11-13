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

export type HostedStat = {
  labelKey: string
  value: string | number
}

export function HostedStatGrid(props: { items: HostedStat[] }) {
  const { t } = useTranslation()

  return (
    <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
      {props.items.map((item) => (
        <div
          key={item.labelKey}
          className='bg-card rounded-lg border px-3 py-2.5 shadow-xs'
        >
          <p className='text-muted-foreground text-xs'>{t(item.labelKey)}</p>
          <p className='mt-1 text-lg font-semibold tabular-nums'>{item.value}</p>
        </div>
      ))}
    </div>
  )
}
