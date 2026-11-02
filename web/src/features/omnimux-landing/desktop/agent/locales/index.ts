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
// fork: the six Agent-page locales, keyed by the same codes the landing page
// uses. Adding a language means adding its file and its row here; the
// `Record<AgentLocale, …>` type makes a missing language a typecheck failure.

import type { AgentKind, AgentLocale, AgentPageContent } from '../content'
import { en } from './en'
import { es } from './es'
import { id } from './id'
import { ja } from './ja'
import { zhCN } from './zh-CN'
import { zhTW } from './zh-TW'

export const AGENT_PAGES: Record<
  AgentLocale,
  Record<AgentKind, AgentPageContent>
> = {
  en,
  'zh-CN': zhCN,
  'zh-TW': zhTW,
  ja,
  es,
  id,
}
