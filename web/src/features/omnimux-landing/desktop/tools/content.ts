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
// fork: single source for the TikTok tool pages' paths, locales, and copy.
// Copy lives in ./locales/<lang>.ts so each language stays isolated; the React
// page and static SEO HTML both read these plain modules (no React/JSX or
// '@/…' aliases because rsbuild.config.ts imports this with a relative path).

export {
  TOOL_LOCALE_TAGS,
  TOOL_LOCALES,
  TOOL_PATHS,
  type ToolKind,
  type ToolLocale,
  type ToolPageContent,
} from './paths'
export { TOOL_PAGES } from './locales'
