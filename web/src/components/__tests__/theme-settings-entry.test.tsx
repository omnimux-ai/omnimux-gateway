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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'

import { ConfigDrawer } from '@/components/config-drawer'
import { SidebarProvider } from '@/components/ui/sidebar'
import { DirectionProvider } from '@/context/direction-provider'
import { LayoutProvider } from '@/context/layout-provider'
import { ThemeCustomizationProvider } from '@/context/theme-customization-provider'
import { ThemeProvider } from '@/context/theme-provider'

afterEach(cleanup)

it('does not expose the appearance settings button in the website shell', () => {
  render(
    <DirectionProvider>
      <ThemeProvider>
        <ThemeCustomizationProvider>
          <LayoutProvider>
            <SidebarProvider>
              <ConfigDrawer />
            </SidebarProvider>
          </LayoutProvider>
        </ThemeCustomizationProvider>
      </ThemeProvider>
    </DirectionProvider>
  )

  expect(
    screen.queryByRole('button', { name: 'Open theme settings' })
  ).toBeNull()
})
