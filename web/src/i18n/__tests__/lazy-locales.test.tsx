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
import { render, screen, waitFor } from '@testing-library/react'
import { useTranslation } from 'react-i18next'
import { describe, expect, it } from 'vitest'

import en from '../locales/en.json'

import i18n, { ensureLocaleLoaded, i18nReady } from '../config'

/**
 * Only the fallback language is bundled; every other interface language is
 * fetched as its own chunk. Startup waits for that chunk — the app does not
 * render until the promise settles — and a language that never arrives must
 * leave the fallback in place.
 *
 * `src/test-setup.ts` re-initializes the shared i18next instance with an empty
 * English bundle, so these tests assert the chunk-loading behaviour rather than
 * the bundled English strings.
 */
describe('lazy interface languages', () => {
  it('settles startup so the app is never left unrendered', async () => {
    await expect(i18nReady).resolves.toBeUndefined()
  })

  it('does not carry a non-bundled locale before it is selected', () => {
    expect(i18n.hasResourceBundle('zhCN', 'translation')).toBe(false)
  })

  it('loads a non-bundled locale chunk on demand and translates with it', async () => {
    await i18nReady

    await ensureLocaleLoaded('zhCN')
    await i18n.changeLanguage('zhCN')

    expect(i18n.hasResourceBundle('zhCN', 'translation')).toBe(true)
    expect(i18n.t('Action confirmation')).toBe('操作确认')
  })

  it('leaves the fallback in place for a language the app does not ship', async () => {
    await i18nReady

    await expect(ensureLocaleLoaded('xx-YY')).resolves.toBeUndefined()

    expect(i18n.hasResourceBundle('xx-YY', 'translation')).toBe(false)
  })

  it('re-renders mounted components when a locale bundle reaches the store', async () => {
    await i18nReady
    await i18n.changeLanguage('en')

    function Probe() {
      const { t } = useTranslation()
      return <span data-testid='probe'>{t('Action confirmation')}</span>
    }

    render(<Probe />)
    // The test setup ships an empty English bundle, so the key itself renders.
    expect(screen.getByTestId('probe')).toHaveTextContent('Action confirmation')

    // A locale chunk arriving mid-session is only reported through the store, so
    // that event is what has to update components that are already mounted.
    i18n.addResourceBundle('en', 'translation', en.translation)

    await waitFor(() =>
      expect(screen.getByTestId('probe')).toHaveTextContent('Action Confirmation')
    )
  })
})
