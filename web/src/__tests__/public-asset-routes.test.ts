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

import { readdirSync } from 'node:fs'
import { join } from 'node:path'

import { describe, expect, it } from 'vitest'

/**
 * `web/public` is served by the gateway's static middleware before the SPA
 * fallback, and Go's file server answers a directory path with a 301 to its
 * trailing-slash form — while the router normalises the trailing slash back
 * away. A public entry named after a page therefore turns that page into an
 * infinite redirect (`/influencer` ↔ `/influencer/`) that only reproduces on
 * the gateway, never on the dev server.
 *
 * Route segments come from the file tree: a parenthesised name is a group and
 * an underscore-prefixed name is a pathless layout, so neither reaches the URL.
 */
const ROUTES_DIR = join(process.cwd(), 'src/routes')
const PUBLIC_DIR = join(process.cwd(), 'public')

function routeSegments(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true })
    .map((entry) => entry.name)
    .filter((name) => !name.startsWith('_') && !name.startsWith('('))
    .map((name) => name.replace(/\.(tsx|ts)$/, ''))
    .filter((name) => name !== 'route' && name !== 'index' && name !== '__root')
}

function urlSegments(): Set<string> {
  const nested = join(ROUTES_DIR, '_authenticated')
  return new Set([
    ...routeSegments(ROUTES_DIR),
    ...readdirSync(nested, { withFileTypes: true })
      .map((entry) => entry.name)
      .filter((name) => !name.startsWith('_') && !name.startsWith('('))
      .map((name) => name.replace(/\.(tsx|ts)$/, ''))
      .filter((name) => name !== 'route' && name !== 'index'),
  ])
}

describe('web/public entries', () => {
  it('never shadow a page route', () => {
    const routes = urlSegments()
    const shadowing = readdirSync(PUBLIC_DIR)
      .map((name) => name.replace(/\.[^.]+$/, ''))
      .filter((name) => routes.has(name))

    expect(shadowing).toEqual([])
  })
})
