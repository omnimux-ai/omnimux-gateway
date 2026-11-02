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
import { describe, test } from 'node:test'

import {
  CLI_GITHUB_URL,
  CONTACT_EMAIL,
  DESKTOP_GITHUB_URL,
  DISCORD_URL,
  DOCS_URL,
  X_URL,
} from '../landing-urls'

describe('landing hero and navigation urls', () => {
  test('points at the OmniMux CLI repository', () => {
    assert.equal(CLI_GITHUB_URL, 'https://github.com/laozhong86/omnimux-cli')
  })

  test('points at the public source repository', () => {
    assert.equal(
      DESKTOP_GITHUB_URL,
      'https://github.com/omnimux-ai/omnimux-ai'
    )
  })

  test('points at the Discord community', () => {
    assert.equal(DISCORD_URL, 'https://discord.gg/uNPBDz3WNC')
  })

  test('points at the official X (Twitter) account', () => {
    assert.equal(X_URL, 'https://x.com/OmniMuxai')
  })

  test('points at the official contact email and docs URL', () => {
    assert.equal(CONTACT_EMAIL, 'hello@omnimux.ai')
    assert.equal(DOCS_URL, 'https://docs.omnimux.ai')
  })
})
