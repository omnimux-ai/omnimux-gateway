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
import { GatewayTransport } from './gateway-transport'
import { MockTransport } from './mock-transport'
import type { GenerationTransport } from '../types'

export { GatewayTransport }

let shared: GenerationTransport | null = null

/**
 * Transport seam for the model page playground.
 *
 * The gateway is the default: the page's whole purpose is to submit a real
 * generation from the signed-in session, and a simulated success would be worse
 * than a visible failure. Set `VITE_MODEL_PLAYGROUND_TRANSPORT=mock` to fall back
 * to the simulated lifecycle used while building the page.
 */
export function getGenerationTransport(): GenerationTransport {
  if (shared) return shared

  const wantsMock =
    typeof import.meta !== 'undefined' &&
    import.meta.env?.VITE_MODEL_PLAYGROUND_TRANSPORT === 'mock'

  shared = wantsMock ? new MockTransport() : new GatewayTransport()
  return shared
}

/** Test seam: drops the memoised transport. */
export function resetGenerationTransport(): void {
  shared = null
}
