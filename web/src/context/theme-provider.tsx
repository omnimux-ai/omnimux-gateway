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
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
} from 'react'

import { THEME_STORAGE_KEYS, writeThemePreference } from '@/lib/theme-storage'

type Theme = 'dark' | 'light' | 'system'
type ThemeProviderProps = {
  children: React.ReactNode
  defaultTheme?: Theme
  storageKey?: string
}
type ThemeProviderState = {
  defaultTheme: 'dark'
  resolvedTheme: 'dark'
  theme: 'dark'
  setTheme: (theme: Theme) => void
  resetTheme: () => void
}

const initialState: ThemeProviderState = {
  defaultTheme: 'dark',
  resolvedTheme: 'dark',
  theme: 'dark',
  setTheme: () => null,
  resetTheme: () => null,
}
const ThemeContext = createContext<ThemeProviderState>(initialState)

// fork: Website appearance is fixed; legacy caller preferences cannot override it.
export function ThemeProvider(props: ThemeProviderProps) {
  const storageKey = props.storageKey ?? THEME_STORAGE_KEYS.mode
  useEffect(() => {
    const root = document.documentElement
    root.classList.remove('light')
    root.classList.add('dark')
    root.style.colorScheme = 'dark'
  }, [])

  const setTheme = useCallback(() => {
    writeThemePreference(storageKey, 'dark')
  }, [storageKey])
  const resetTheme = useCallback(() => {
    writeThemePreference(storageKey, null)
  }, [storageKey])
  const value = useMemo<ThemeProviderState>(
    () => ({ ...initialState, setTheme, resetTheme }),
    [setTheme, resetTheme]
  )
  return <ThemeContext value={value}>{props.children}</ThemeContext>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = () => useContext(ThemeContext)
