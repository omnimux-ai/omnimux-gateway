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
import { createInstance } from 'i18next'
import { describe, expect, it } from 'vitest'

import type { TextTranslator } from '../translator'

describe('text translation', () => {
  it('accepts the native translator for plain and interpolated text', async () => {
    const i18n = createInstance()
    await i18n.init({
      lng: 'fr',
      fallbackLng: 'en',
      resources: {
        fr: {
          translation: {
            Retry: 'Réessayer',
            'Ratio: {{value}}': 'Ratio : {{value}}',
          },
        },
      },
    })
    const translate: TextTranslator = i18n.t

    expect(translate('Retry')).toBe('Réessayer')
    expect(translate('Ratio: {{value}}', { value: 1.5 })).toBe('Ratio : 1.5')
  })

  it('uses the French other form before English when many is missing', async () => {
    const i18n = createInstance()
    await i18n.init({
      lng: 'fr',
      fallbackLng: { default: ['en'] },
      resources: {
        fr: {
          translation: {
            task_one: 'Une tâche',
            task_other: '{{count}} tâches',
          },
        },
        en: {
          translation: { task_one: 'One task', task_other: '{{count}} tasks' },
        },
      },
    })
    const count = 1_000_000
    expect(new Intl.PluralRules('fr').select(count)).toBe('many')
    expect(i18n.t('task', { count })).toBe('1000000 tâches')
  })
})
