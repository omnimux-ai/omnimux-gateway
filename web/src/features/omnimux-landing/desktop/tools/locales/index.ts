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
import type { ToolKind, ToolLocale, ToolPageContent } from '../paths'
import { en } from './en'
import { es } from './es'
import { id } from './id'
import { ja } from './ja'
import { zhCN } from './zh-CN'
import { zhTW } from './zh-TW'

export const TOOL_PAGES: Record<
  ToolLocale,
  Record<ToolKind, ToolPageContent>
> = {
  en,
  'zh-CN': zhCN,
  'zh-TW': zhTW,
  ja,
  es,
  id,
}
