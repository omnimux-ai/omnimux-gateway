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
import { Link } from '@tanstack/react-router'
import { Astroid, ChevronDown, Menu } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

import type { TopNavLink } from '../types'

type TopNavProps = React.HTMLAttributes<HTMLElement> & {
  links: TopNavLink[]
}

function TopNavItemWithDropdown({
  title,
  items,
  isActive,
}: {
  title: string
  items: NonNullable<TopNavLink['items']>
  isActive?: boolean
}) {
  const [open, setOpen] = useState(false)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleMouseEnter = () => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current)
    setOpen(true)
  }

  const handleMouseLeave = () => {
    timeoutRef.current = setTimeout(() => {
      setOpen(false)
    }, 150)
  }

  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
    }
  }, [])

  return (
    <div
      className='relative flex items-center'
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <button
        type='button'
        onClick={() => setOpen((prev) => !prev)}
        className={cn(
          'hover:text-primary flex cursor-pointer items-center gap-1 text-sm font-medium transition-colors',
          isActive ? 'text-primary' : 'text-muted-foreground'
        )}
        aria-expanded={open}
        aria-haspopup='true'
      >
        <span>{title}</span>
        <ChevronDown
          className={cn(
            'size-3.5 opacity-70 transition-transform duration-200',
            open && 'rotate-180'
          )}
        />
      </button>

      {open && (
        <div
          className='animate-in fade-in-0 zoom-in-95 absolute top-full left-0 z-50 pt-2 duration-150'
          role='menu'
        >
          <div className='border-border bg-popover/95 text-popover-foreground min-w-44 rounded-lg border p-1 shadow-lg backdrop-blur-sm'>
            {items.map((subItem) => (
              <Link
                key={subItem.href}
                to={subItem.href}
                onClick={() => setOpen(false)}
                className='text-popover-foreground hover:bg-accent hover:text-accent-foreground flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors'
                role='menuitem'
              >
                {subItem.icon && (
                  <subItem.icon className='text-muted-foreground size-4' />
                )}
                <span className='font-medium'>{subItem.title}</span>
              </Link>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

/**
 * 顶部导航栏组件
 * 在大屏幕显示水平导航，在小屏幕显示下拉菜单
 */
export function TopNav({ className, links, ...props }: TopNavProps) {
  // 规范化链接，确保所有可选属性都有默认值
  const normalizedLinks = useMemo(
    () =>
      links.map((link) => ({
        isActive: false,
        disabled: false,
        external: false,
        ...link,
      })),
    [links]
  )

  return (
    <>
      {/* 移动端下拉菜单 */}
      <div className='lg:hidden'>
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger
            render={<Button size='icon' variant='outline' className='size-7' />}
          >
            <Menu />
          </DropdownMenuTrigger>
          <DropdownMenuContent side='bottom' align='start'>
            {normalizedLinks.flatMap((link) => {
              if (link.items && link.items.length > 0) {
                return link.items.map((subItem) => (
                  <DropdownMenuItem
                    key={`${link.title}-${subItem.href}`}
                    render={
                      <Link
                        to={subItem.href}
                        className='text-muted-foreground'
                      >
                        {subItem.title}
                      </Link>
                    }
                  />
                ))
              }
              return [
                <DropdownMenuItem
                  key={`${link.title}-${link.href}`}
                  render={
                    link.external ? (
                      <a
                        href={link.href}
                        target='_blank'
                        rel='noopener noreferrer'
                        className={!link.isActive ? 'text-muted-foreground' : ''}
                      >
                        {link.title}
                      </a>
                    ) : (
                      <Link
                        to={link.href || '/'}
                        className={!link.isActive ? 'text-muted-foreground' : ''}
                        disabled={link.disabled}
                      >
                        {link.title}
                      </Link>
                    )
                  }
                />,
              ]
            })}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* 桌面端水平导航 */}
      <nav
        className={cn(
          'hidden items-center space-x-4 lg:flex lg:space-x-4 xl:space-x-6',
          className
        )}
        {...props}
      >
        {normalizedLinks.map((link) => {
          if (link.items && link.items.length > 0) {
            return (
              <TopNavItemWithDropdown
                key={link.title}
                title={link.title}
                items={link.items}
                isActive={link.isActive}
              />
            )
          }
          return link.external ? (
            <a
              key={`${link.title}-${link.href}`}
              href={link.href}
              target='_blank'
              rel='noopener noreferrer'
              className={`hover:text-primary text-sm font-medium transition-colors ${link.isActive ? '' : 'text-muted-foreground'}`}
            >
              {link.title}
            </a>
          ) : (
            <Link
              key={`${link.title}-${link.href}`}
              to={link.href || '/'}
              disabled={link.disabled}
              className={`hover:text-primary inline-flex items-center gap-2 text-sm font-medium transition-colors ${link.isActive ? '' : 'text-muted-foreground'}`}
            >
              {link.href === '/influencer' && (
                <Astroid size={24} aria-hidden='true' className='shrink-0' />
              )}
              {link.title}
            </Link>
          )
        })}
      </nav>
    </>
  )
}
