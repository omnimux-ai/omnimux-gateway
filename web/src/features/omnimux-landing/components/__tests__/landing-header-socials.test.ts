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
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, test } from 'node:test'
import { fileURLToPath } from 'node:url'

import {
  CLI_GITHUB_URL,
  DESKTOP_GITHUB_URL,
  DISCORD_URL,
  X_URL,
} from '../../lib/landing-urls'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const headerFilePath = path.resolve(__dirname, '../landing-header.tsx')
const headerSource = readFileSync(headerFilePath, 'utf-8')

describe('LandingHeader Social & GitHub Navigation Contracts', () => {
  test('constants have valid endpoints matching specifications', () => {
    assert.equal(
      DESKTOP_GITHUB_URL,
      'https://github.com/omnimux-ai/omnimux-ai'
    )
    assert.equal(DISCORD_URL, 'https://discord.gg/uNPBDz3WNC')
    assert.equal(X_URL, 'https://x.com/OmniMuxai')
    assert.equal(CLI_GITHUB_URL, 'https://github.com/laozhong86/omnimux-cli')
  })

  test('desktop header keeps the social row in order and carries no language control', () => {
    const desktopNavStart = headerSource.indexOf('hidden items-center gap-2 lg:flex')
    assert.ok(desktopNavStart !== -1, 'desktop action container must exist')

    const desktopNavSection = headerSource.slice(desktopNavStart, desktopNavStart + 2200)

    const xIndex = desktopNavSection.indexOf('href={X_URL}')
    const discordIndex = desktopNavSection.indexOf('href={DISCORD_URL}')
    const desktopGithubIndex = desktopNavSection.indexOf('href={DESKTOP_GITHUB_URL}')

    assert.ok(xIndex !== -1, 'Twitter (X) link must exist in desktop header')
    assert.ok(discordIndex !== -1, 'Discord link must exist in desktop header')
    assert.ok(desktopGithubIndex !== -1, 'Desktop GitHub link must exist in desktop header')

    // Position verification: X < Discord < Desktop GitHub
    assert.ok(xIndex < discordIndex, 'Discord must sit to the right of Twitter (X)')
    assert.ok(
      discordIndex < desktopGithubIndex,
      'Desktop GitHub must sit to the right of Discord'
    )
  })

  test('public surfaces render no language control, so visitors follow their system language', () => {
    assert.ok(
      !headerSource.includes('LanguageSwitcher'),
      'landing header must not render a language control'
    )

    const publicHeaderSource = readFileSync(
      path.resolve(__dirname, '../../../../components/layout/components/public-header.tsx'),
      'utf-8'
    )
    assert.ok(
      !publicHeaderSource.includes('LanguageSwitcher'),
      'public header must not render a language control'
    )
  })

  test('external social links enforce security attributes and accessible labels', () => {
    const targetBlankMatches = (headerSource.match(/target="_blank"/g) || []).length
    const relNoopenerMatches = (headerSource.match(/rel="noopener noreferrer"/g) || []).length

    assert.ok(targetBlankMatches >= 3, 'Must include target="_blank" on external social links')
    assert.equal(targetBlankMatches, relNoopenerMatches, 'Every target="_blank" must have rel="noopener noreferrer"')

    assert.ok(headerSource.includes("aria-label={t('omnimux_landing.footer.x'"), 'X icon link must have aria-label')
    assert.ok(headerSource.includes("aria-label={t('omnimux_landing.footer.discord'"), 'Discord icon link must have aria-label')
    assert.ok(headerSource.includes("aria-label={t('omnimux_landing.nav.github'"), 'GitHub icon link must have aria-label')
  })

  test('mobile navigation menu provides synchronous social links', () => {
    const mobilePanelStart = headerSource.indexOf('mobileOpen && (')
    assert.ok(mobilePanelStart !== -1, 'Mobile menu panel must exist')

    assert.ok(
      headerSource.includes('href={X_URL}', mobilePanelStart),
      'Mobile menu must include Twitter (X)'
    )
    assert.ok(
      headerSource.includes('href={DISCORD_URL}', mobilePanelStart),
      'Mobile menu must include Discord'
    )
    assert.ok(
      headerSource.includes('href={DESKTOP_GITHUB_URL}', mobilePanelStart),
      'Mobile menu must include Desktop GitHub'
    )
  })
})
