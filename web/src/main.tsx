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
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'

import { applyPageMetadata } from '@/features/site-metadata/dom'
import type { SearchPageManifest } from '@/features/site-metadata/metadata'
import { installBuildMetadata } from '@/lib/build-metadata'
import { applyFaviconToDom } from '@/lib/dom-utils'
import '@/lib/dayjs'
import { initializeFrontendCache } from '@/lib/frontend-cache'
import { createAppQueryClient } from '@/lib/query-client'
import { readCachedStatus, statusQueryOptions } from '@/lib/status-query'

import { DirectionProvider } from './context/direction-provider'
import { FontProvider } from './context/font-provider'
import { ThemeProvider } from './context/theme-provider'
import { i18nReady } from './i18n/config'
// Generated Routes
import { routeTree } from './routeTree.gen'

// Styles
import './styles/index.css'

// Ensure VChart theme is initialized before any chart mounts (prevents white default theme flash)
// VChart theme is driven by our ThemeProvider (html.light/html.dark) via per-chart `theme` prop.
initializeFrontendCache()
installBuildMetadata()

const queryClient = createAppQueryClient(() => {
  void router.navigate({ to: '/500' })
})

// Create a new router instance
const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Render the app
const rootElement = document.querySelector<HTMLElement>('#root')
if (!rootElement) {
  throw new Error('Root element not found')
}
// Keep page-specific search metadata through status refreshes and SPA navigation.
const searchPages: SearchPageManifest = import.meta.env.OMNIMUX_SEARCH_PAGES
const syncPageMetadata = () =>
  applyPageMetadata(searchPages, window.location.pathname)
syncPageMetadata()
router.subscribe('onResolved', syncPageMetadata)

const cachedStatus = readCachedStatus()
if (cachedStatus?.logo) applyFaviconToDom(cachedStatus.logo as string)
void queryClient
  .ensureQueryData(statusQueryOptions)
  .then((status) => {
    if (status?.logo) applyFaviconToDom(status.logo as string)
  })
  .catch(() => {
    // The shared status query owns the request failure; static page metadata remains usable.
  })

if (!rootElement.innerHTML) {
  // The active interface language may arrive as its own chunk; wait for it so a
  // non-English visitor never sees a flash of the fallback language. A locale
  // chunk that fails to load resolves anyway, leaving the bundled fallback.
  void i18nReady.finally(() => {
    const root = ReactDOM.createRoot(rootElement)
    root.render(
      <StrictMode>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider>
            <FontProvider>
              <DirectionProvider>
                <RouterProvider router={router} />
              </DirectionProvider>
            </FontProvider>
          </ThemeProvider>
        </QueryClientProvider>
      </StrictMode>
    )
  })
}
