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
export type EntryName = 'landing' | 'app'

export type EntryManifest = Record<EntryName, readonly string[]>

export type EntryWindow = Window & {
  __OMNIMUX_ENTRIES__?: EntryManifest
  __omnimuxUnmountLanding?: () => void
}

const loadedEntries = new Set<EntryName>()

function readManifest(): EntryManifest {
  const manifest = (window as EntryWindow).__OMNIMUX_ENTRIES__
  if (!manifest?.landing?.length || !manifest.app?.length) {
    throw new Error('OmniMux entry manifest is not available')
  }
  return manifest
}

/**
 * Loads an entry's initial scripts in the order emitted by the bundler. The
 * entry chunks rely on their shared initial chunks being present before their
 * runtime starts, and `async = false` keeps the browser's classic-script order.
 */
export function loadEntry(name: EntryName): void {
  if (loadedEntries.has(name)) return
  loadedEntries.add(name)
  for (const url of readManifest()[name]) {
    if (url.endsWith('.css')) {
      const link = document.createElement('link')
      link.rel = 'stylesheet'
      link.href = url
      document.head.appendChild(link)
      continue
    }
    const script = document.createElement('script')
    script.async = false
    script.src = url
    document.head.appendChild(script)
  }
}
