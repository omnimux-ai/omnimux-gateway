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
import { z } from 'zod'

// fork: public TikTok photo downloader tool page (en). The URL pins the page
// language for SEO; the feature stays a lazy chunk like the homepage.
const TikTokToolPage = lazy(() =>
  import('@/features/omnimux-landing/desktop/tools').then((m) => ({
    default: m.TikTokToolPage,
  }))
)

const searchSchema = z.object({
  url: z.string().optional(),
})

function ToolRoute() {
  const search = Route.useSearch()
  return (
    <Suspense fallback={<div className='min-h-screen bg-[#0C0B11]' />}>
      <TikTokToolPage kind='photo' locale='en' initialUrl={search.url} />
    </Suspense>
  )
}

export const Route = createFileRoute('/tools/tiktok-photo-downloader')({
  validateSearch: searchSchema,
  component: ToolRoute,
})
