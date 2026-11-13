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
import { useEffect, useState, type MouseEvent } from 'react'

import { landingLangOption, useLanding } from '../../i18n'
import type { LandingLang } from '../../i18n/types'

/** A plain click switches in place; modified clicks keep the browser's link behaviour. */
function isPlainClick(event: MouseEvent<HTMLAnchorElement>): boolean {
  return (
    event.button === 0 &&
    !event.metaKey &&
    !event.ctrlKey &&
    !event.shiftKey &&
    !event.altKey
  )
}

/**
 * Landing-page language switcher. Same portal + styling pattern as
 * CnAccountMenu so the popup inherits the .omc scoped tokens. Each language
 * is a real link to its URL; a plain click still switches through `setLang`
 * so the choice is remembered.
 */
export function CnLangMenu() {
  const { lang, setLang, copy, langOptions, langHref } = useLanding()
  const current = landingLangOption(lang)

  const choose = (event: MouseEvent<HTMLAnchorElement>, next: LandingLang) => {
    if (!isPlainClick(event)) return
    event.preventDefault()
    setLang(next)
  }

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
            aria-label={`${current.short} · ${copy.nav.langTitle}：${current.label}`}
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
              {langOptions.map((option) => (
                <MenuPrimitive.LinkItem
                  key={option.code}
                  className='cn-item cn-lang-item'
                  href={langHref(option.code)}
                  hrefLang={option.htmlLang}
                  aria-current={option.code === lang ? 'page' : undefined}
                  closeOnClick
                  onClick={(event) => choose(event, option.code)}
                >
                  <span lang={option.htmlLang}>{option.label}</span>
                  {option.code === lang ? <Check /> : <i aria-hidden='true' />}
                </MenuPrimitive.LinkItem>
              ))}
            </MenuPrimitive.Popup>
          </MenuPrimitive.Positioner>
        </MenuPrimitive.Portal>
      )}
    </MenuPrimitive.Root>
  )
}
