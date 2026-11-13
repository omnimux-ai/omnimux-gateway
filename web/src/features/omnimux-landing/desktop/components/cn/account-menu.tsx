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
import { useNavigate } from '@tanstack/react-router'
import {
  LayoutGrid,
  LogOut,
  Settings,
  ShieldCheck,
  User,
  Wallet,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { useIsSidebarModuleVisible } from '@/hooks/use-sidebar-config'
import { useUserDisplay } from '@/hooks/use-user-display'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useCopy } from '../../i18n'

/**
 * 桌面端落地页的账户菜单。结构与 ProfileDropdown 保持一致（用户头、
 * 控制台/资料/安全/钱包/系统设置、退出登录），视觉使用落地页自己的深色
 * 样式，因此弹层通过 Portal 固定挂到 .omc 根内，沿用其作用域变量。
 */
export function CnAccountMenu(props: { onSignOut: () => void }) {
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.auth.user)
  const { displayName, roleLabel } = useUserDisplay(user)
  const isSuperAdmin = user?.role === ROLE.SUPER_ADMIN
  const isWalletVisible = useIsSidebarModuleVisible('/wallet')
  const isSecurityVisible = useIsSidebarModuleVisible('/security')
  const avatarName = user?.username || displayName
  const avatarFallback = getUserAvatarFallback(avatarName)
  const avatarFallbackStyle = useMemo(
    () => getUserAvatarStyle(avatarName),
    [avatarName]
  )
  const account = useCopy().nav.account

  const [portalRoot, setPortalRoot] = useState<HTMLElement | null>(null)
  useEffect(() => {
    setPortalRoot(document.querySelector<HTMLElement>('.omc'))
  }, [])

  return (
    <MenuPrimitive.Root modal={false}>
      <MenuPrimitive.Trigger
        render={
          <button type='button' className='cn-avatar' aria-label={account.aria} />
        }
      >
        <Avatar size='sm'>
          <AvatarFallback style={avatarFallbackStyle}>
            {avatarFallback}
          </AvatarFallback>
        </Avatar>
      </MenuPrimitive.Trigger>
      {portalRoot && (
        <MenuPrimitive.Portal container={portalRoot}>
          <MenuPrimitive.Positioner
            className='cn-menupos'
            align='end'
            side='bottom'
            sideOffset={8}
          >
            <MenuPrimitive.Popup className='cn-usermenu'>
              <div className='cn-who'>
                <Avatar>
                  <AvatarFallback style={avatarFallbackStyle}>
                    {avatarFallback}
                  </AvatarFallback>
                </Avatar>
                <div className='cn-who-meta'>
                  <b>{displayName}</b>
                  <span>
                    {roleLabel}
                    {user?.group ? ` · ${String(user.group)}` : ''}
                  </span>
                </div>
              </div>
              <div className='cn-sep' />
              <MenuPrimitive.Item
                className='cn-item'
                onClick={() => navigate({ to: '/influencer' })}
              >
                <LayoutGrid />
                {account.dashboard}
              </MenuPrimitive.Item>
              <MenuPrimitive.Item
                className='cn-item'
                onClick={() => navigate({ to: '/profile' })}
              >
                <User />
                {account.profile}
              </MenuPrimitive.Item>
              {isSecurityVisible && (
                <MenuPrimitive.Item
                  className='cn-item'
                  onClick={() => navigate({ to: '/security' })}
                >
                  <ShieldCheck />
                  {account.security}
                </MenuPrimitive.Item>
              )}
              {isWalletVisible && (
                <MenuPrimitive.Item
                  className='cn-item'
                  onClick={() => navigate({ to: '/wallet' })}
                >
                  <Wallet />
                  {account.wallet}
                </MenuPrimitive.Item>
              )}
              {isSuperAdmin && (
                <MenuPrimitive.Item
                  className='cn-item'
                  onClick={() =>
                    navigate({
                      to: '/system-settings/site/$section',
                      params: { section: 'system-info' },
                    })
                  }
                >
                  <Settings />
                  {account.systemSettings}
                </MenuPrimitive.Item>
              )}
              <div className='cn-sep' />
              <MenuPrimitive.Item
                className='cn-item cn-danger'
                onClick={props.onSignOut}
              >
                <LogOut />
                {account.signOut}
              </MenuPrimitive.Item>
            </MenuPrimitive.Popup>
          </MenuPrimitive.Positioner>
        </MenuPrimitive.Portal>
      )}
    </MenuPrimitive.Root>
  )
}
