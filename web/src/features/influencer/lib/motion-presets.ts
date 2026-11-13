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

import type { MotionPreset } from '../types'
import { motionTemplatePresets } from './motion-explore'

/**
 * The built-in video template pack: the official motion presets mirrored from
 * the upstream catalogue (`data/motion-explore.json`). Each entry carries a
 * real public clip URL, so picking one fills the video slot with a link the
 * gateway fetches as the driving reference — no local download needed.
 *
 * `nameKey` is the upstream English title and doubles as the i18n key, so the
 * titles are registered in `src/i18n/static-keys.ts`.
 */
export const MOTION_PRESETS: MotionPreset[] = motionTemplatePresets()
