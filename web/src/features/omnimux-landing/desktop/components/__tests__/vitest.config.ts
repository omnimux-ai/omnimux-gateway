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
import { defineConfig } from 'vitest/config'

import baseConfig from '../../../../../../vitest.config'

export default defineConfig({
  ...baseConfig,
  test: {
    ...baseConfig.test,
    include: [
      'src/features/omnimux-landing/desktop/components/__tests__/cn-i18n.test.tsx',
      'src/features/omnimux-landing/desktop/components/__tests__/cn-landing.test.tsx',
      'src/features/omnimux-landing/desktop/components/__tests__/cn-nav-auth.test.tsx',
      'src/features/omnimux-landing/desktop/components/__tests__/detect-platform.test.ts',
    ],
    exclude: [],
  },
})
