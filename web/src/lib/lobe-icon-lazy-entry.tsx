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
import { lazy } from 'react'

const LazyIcon = lazy(async () => {
  const { getLobeIcon } = await import('./lobe-icon')
  function LazyIconEntry(props: { name: string; size: number }) {
    return <>{getLobeIcon(props.name, props.size)}</>
  }
  return { default: LazyIconEntry }
})

/** Loads the full icon registry only when an icon is first rendered. */
export default function LobeIconLazyEntry(props: { name: string; size: number }) {
  return <LazyIcon name={props.name} size={props.size} />
}
