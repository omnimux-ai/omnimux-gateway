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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, fireEvent } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import { useAuthStore } from '@/stores/auth-store'

import { LandingHeader } from '../landing-header'

afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.setUser(null)
})

async function renderHeader(authenticated: boolean) {
  useAuthStore
    .getState()
    .auth.setUser(
      authenticated ? { id: 1, username: 'visitor', role: 1 } : null
    )
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en } })
  const root = createRootRoute({
    component: () => (
      <I18nextProvider i18n={i18n}>
        <LandingHeader />
      </I18nextProvider>
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
}

it('sends guests to registration from desktop and mobile Start links', async () => {
  await renderHeader(false)
  expect(screen.getByRole('link', { name: 'Start' })).toHaveAttribute(
    'href',
    '/sign-up'
  )
  fireEvent.click(
    screen.getByRole('button', { name: 'Toggle navigation menu' })
  )
  expect(screen.getAllByRole('link', { name: 'Start' })).toHaveLength(2)
  for (const link of screen.getAllByRole('link', { name: 'Start' }))
    {expect(link).toHaveAttribute('href', '/sign-up')}
})

it('sends signed-in users to the studio from desktop and mobile Start links', async () => {
  await renderHeader(true)
  expect(screen.getByRole('link', { name: 'Start' })).toHaveAttribute(
    'href',
    '/influencer'
  )
  fireEvent.click(
    screen.getByRole('button', { name: 'Toggle navigation menu' })
  )
  expect(screen.getAllByRole('link', { name: 'Start' })).toHaveLength(2)
  for (const link of screen.getAllByRole('link', { name: 'Start' }))
    {expect(link).toHaveAttribute('href', '/influencer')}
})
