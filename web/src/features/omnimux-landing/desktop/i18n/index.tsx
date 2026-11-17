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
/* eslint-disable react-refresh/only-export-components */
import {
  LandingLangProviderBase,
  resolveLandingLang,
  type LandingLangProviderProps,
} from './context'
import { LANDING_COPIES } from './static-copies'
import type { LandingCopy } from './types'

export * from './context'
export { LANDING_COPIES } from './static-copies'

/**
 * Synchronous provider for app surfaces (tool/agent pages and tests).
 * The landing entry uses the base provider with an explicitly preloaded copy,
 * so it never imports every locale dictionary into the first-screen bundle.
 */
export function LandingLangProvider(
  props: LandingLangProviderProps & { initialCopy?: LandingCopy }
) {
  const lang = props.lang ?? resolveLandingLang()
  return (
    <LandingLangProviderBase
      {...props}
      initialCopy={props.initialCopy ?? LANDING_COPIES[lang]}
    />
  )
}
