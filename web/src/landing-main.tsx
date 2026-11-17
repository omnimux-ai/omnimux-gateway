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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { StrictMode, useEffect } from 'react'
import ReactDOM from 'react-dom/client'

import { DirectionProvider } from './context/direction-provider'
import { FontProvider } from './context/font-provider'
import { ThemeProvider } from './context/theme-provider'
import { loadEntry, type EntryWindow } from './entry-loader'
import { DesktopLanding } from './features/omnimux-landing/desktop'
import {
  loadLandingCopy,
  resolveLandingLang,
} from './features/omnimux-landing/desktop/i18n/context'
import type {
  LandingCopy,
  LandingLang,
} from './features/omnimux-landing/desktop/i18n/types'
import { installBuildMetadata } from './lib/build-metadata'
import { applyFaviconToDom } from './lib/dom-utils'
import { captureFirstTouch } from './lib/growth-tracking'

installBuildMetadata()
captureFirstTouch()

let landingInitial: { copy: LandingCopy; lang: LandingLang } | null = null

function LandingPage() {
  return (
    <DesktopLanding
      initialCopy={landingInitial?.copy}
      initialLang={landingInitial?.lang}
    />
  )
}

function AppHandoff() {
  useEffect(() => {
    loadEntry('app')
  }, [])
  return <div className='min-h-screen bg-[#0C0B11]' />
}

const rootRoute = createRootRoute({ component: () => <Outlet /> })
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: LandingPage,
})
const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/$',
  component: AppHandoff,
})
const routeTree = rootRoute.addChildren([indexRoute, appRoute])
const router = createRouter({ routeTree })

function readCachedStatusLogo(): string | null {
  try {
    const raw = window.localStorage.getItem('status')
    if (!raw) return null
    const status = JSON.parse(raw) as { logo?: string }
    return status.logo ?? null
  } catch {
    return null
  }
}

let landingRoot: ReactDOM.Root | null = null

export async function mountLanding() {
  const rootElement = document.querySelector<HTMLElement>('#root')
  if (!rootElement) throw new Error('Root element not found')
  const lang = resolveLandingLang()
  const copy = await loadLandingCopy(lang)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false },
    },
  })
  const logo = readCachedStatusLogo()
  if (logo) applyFaviconToDom(logo)

  landingInitial = { copy, lang }
  if (!rootElement.innerHTML) {
    landingRoot = ReactDOM.createRoot(rootElement)
    landingRoot.render(
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
  }
  ;(window as EntryWindow).__omnimuxUnmountLanding = () => {
    landingRoot?.unmount()
    landingRoot = null
    rootElement.innerHTML = ''
  }
}
