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
import { Menu as MenuPrimitive } from '@base-ui/react/menu'
import { Check } from 'lucide-react'
import { useEffect, useState } from 'react'

import {
  LANDING_LANGS,
  landingLangOption,
  useLanding,
} from '../../i18n'

/**
 * Landing-page language switcher. Same portal + styling pattern as
 * CnAccountMenu so the popup inherits the .omc scoped tokens.
 */
export function CnLangMenu() {
  const { lang, setLang, copy } = useLanding()
  const current = landingLangOption(lang)

  const [portalRoot, setPortalRoot] = useState<HTMLElement | null>(null)
  useEffect(() => {
    setPortalRoot(document.querySelector<HTMLElement>('.omc'))
  }, [])

  return (
    <MenuPrimitive.Root modal={false}>
      <MenuPrimitive.Trigger
        render={
          <button
            type='button'
            className='cn-lang-trigger'
            title={copy.nav.langTitle}
            aria-label={`${copy.nav.langTitle}：${current.label}`}
          />
        }
      >
        <span className='cn-lang-code' lang={current.htmlLang}>
          {current.short}
        </span>
      </MenuPrimitive.Trigger>
      {portalRoot && (
        <MenuPrimitive.Portal container={portalRoot}>
          <MenuPrimitive.Positioner
            className='cn-menupos'
            align='end'
            side='bottom'
            sideOffset={8}
          >
            <MenuPrimitive.Popup className='cn-usermenu cn-langmenu'>
              {LANDING_LANGS.map((option) => (
                <MenuPrimitive.Item
                  key={option.code}
                  className='cn-item cn-lang-item'
                  onClick={() => setLang(option.code)}
                >
                  <span lang={option.htmlLang}>{option.label}</span>
                  {option.code === lang ? <Check /> : <i aria-hidden='true' />}
                </MenuPrimitive.Item>
              ))}
            </MenuPrimitive.Popup>
          </MenuPrimitive.Positioner>
        </MenuPrimitive.Portal>
      )}
    </MenuPrimitive.Root>
  )
}
