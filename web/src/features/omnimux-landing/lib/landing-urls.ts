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

export const DOCS_URL = 'https://docs.omnimux.ai'

/** Docs quickstart for the current interface language. Chinese UIs use /zh; all others use /en. */
export function docsQuickstartUrl(language?: string | null): string {
  const normalized = (language ?? '').trim().replaceAll('_', '-').toLowerCase()
  const locale = normalized.startsWith('zh') ? 'zh' : 'en'
  return `${DOCS_URL}/${locale}/quickstart`
}
export const CLI_GITHUB_URL = 'https://github.com/laozhong86/omnimux-cli'
export const DESKTOP_GITHUB_URL = 'https://github.com/omnimux-ai/omnimux-ai'
export const DISCORD_URL = 'https://discord.gg/uNPBDz3WNC'
export const X_URL = 'https://x.com/OmniMuxai'
export const CONTACT_EMAIL = 'hello@omnimux.ai'
