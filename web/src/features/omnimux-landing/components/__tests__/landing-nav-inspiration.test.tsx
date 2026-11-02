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
import { cleanup, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ReactElement } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'

import { LandingHeader } from '../landing-header'

afterEach(cleanup)

async function renderHeader(element: ReactElement) {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  const root = createRootRoute({
    component: () => <I18nextProvider i18n={i18n}>{element}</I18nextProvider>,
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
}

const pricingCopy = en.translation['omnimux_landing.nav.pricing']
const inspirationCopy = en.translation['omnimux_landing.nav.inspiration']

/**
 * The Web surface keeps `/pricing` and `/inspiration` reachable but removes
 * every entry to them: the desktop surface is the only place that advertises
 * the model price catalogue and the inspiration community.
 */
describe('Web landing header route entries', () => {
  it('keeps pricing and inspiration as in-page anchors', async () => {
    await renderHeader(<LandingHeader />)

    const pricing = screen.getAllByRole('link', { name: pricingCopy })[0]
    const inspiration = screen.getAllByRole('link', { name: inspirationCopy })[0]

    expect(pricing).toHaveAttribute('href', '#pricing')
    expect(inspiration).toHaveAttribute('href', '#works')
  })

  it('never links to the /pricing or /inspiration routes', async () => {
    await renderHeader(<LandingHeader />)

    const hrefs = screen
      .getAllByRole('link')
      .map((node) => node.getAttribute('href') ?? '')

    expect(hrefs).not.toContain('/pricing')
    expect(hrefs).not.toContain('/inspiration')
  })

  it('keeps the same contract in the mobile menu', async () => {
    await renderHeader(<LandingHeader />)

    screen.getByRole('button', { name: 'Toggle navigation menu' }).click()

    const hrefs = (await screen.findAllByRole('link')).map(
      (node) => node.getAttribute('href') ?? ''
    )

    expect(hrefs).not.toContain('/pricing')
    expect(hrefs).not.toContain('/inspiration')
  })
})
