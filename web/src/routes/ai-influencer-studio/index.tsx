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
import { createFileRoute } from '@tanstack/react-router'
import { lazy, Suspense } from 'react'

// fork: the character landing page keeps its own copy and media. It moved off
// `/` on 2026-10-27 when the desktop landing became the homepage, so it stays a
// separate lazy chunk and never enters the homepage's initial bundle.
const OmniMuxLanding = lazy(() =>
  import('@/features/omnimux-landing').then((m) => ({
    default: m.OmniMuxLanding,
  }))
)

function CharacterLandingRoute() {
  return (
    <Suspense fallback={<div className='min-h-screen bg-[#09090B]' />}>
      <OmniMuxLanding />
    </Suspense>
  )
}

export const Route = createFileRoute('/ai-influencer-studio/')({
  component: CharacterLandingRoute,
})
