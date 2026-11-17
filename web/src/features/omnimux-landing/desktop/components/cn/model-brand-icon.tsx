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
import DoubaoColor from '@lobehub/icons/es/Doubao/components/Color'
import GeminiColor from '@lobehub/icons/es/Gemini/components/Color'
import OpenAIMono from '@lobehub/icons/es/OpenAI/components/Mono'
import SunoMono from '@lobehub/icons/es/Suno/components/Mono'
import type { ComponentType } from 'react'

const MODEL_BRAND_ICON_COMPONENTS: Record<
  string,
  ComponentType<{ size?: number | string }>
> = {
  'Doubao.Color': DoubaoColor,
  OpenAI: OpenAIMono,
  'Gemini.Color': GeminiColor,
  Suno: SunoMono,
}

/** Landing-only static brand icons: never imports the full @lobehub/icons registry. */
export function ModelBrandIcon(props: { icon: string; size?: number }) {
  const Icon = MODEL_BRAND_ICON_COMPONENTS[props.icon]
  if (!Icon) return null
  return <Icon size={props.size ?? 18} />
}
