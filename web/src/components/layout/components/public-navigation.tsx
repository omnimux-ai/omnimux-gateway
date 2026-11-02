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
import { ChevronDown } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { cn } from '@/lib/utils'

import { defaultTopNavLinks } from '../config/top-nav.config'
import type { TopNavDropdownItem, TopNavLink } from '../types'

function PublicNavItemWithDropdown({
  title,
  items,
}: {
  title: string
  items: TopNavDropdownItem[]
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
        className='text-muted-foreground hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground inline-flex h-9 w-max cursor-pointer items-center justify-center gap-1 rounded-md bg-transparent px-4 py-2 text-sm font-medium transition-colors focus:outline-none'
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

interface PublicNavigationProps {
  /**
   * Custom navigation links
   * If not provided, will use dynamic links from backend or defaults
   */
  links?: TopNavLink[]
  /**
   * Additional className
   */
  className?: string
}

/**
 * Public navigation component that matches Launch UI template styling
 * Used in PublicHeader for desktop navigation
 */
export function PublicNavigation({
  links: providedLinks,
  className,
}: PublicNavigationProps = {}) {
  // Use the same logic as AppHeader: prioritize dynamic links from backend
  const dynamicLinks = useTopNavLinks()
  const defaultLinks = providedLinks || defaultTopNavLinks
  const links = dynamicLinks.length > 0 ? dynamicLinks : defaultLinks

  return (
    <nav className={cn('hidden items-center gap-1 lg:flex', className)}>
      {links.map((link) => {
        if (link.items && link.items.length > 0) {
          return (
            <PublicNavItemWithDropdown
              key={`${link.title}:${link.href || 'dropdown'}`}
              title={link.title}
              items={link.items}
            />
          )
        }
        // Handle external links
        if (link.external) {
          return (
            <a
              key={`${link.title}:${link.href}`}
              href={link.href}
              target='_blank'
              rel='noopener noreferrer'
              className={cn(
                'text-muted-foreground hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground inline-flex h-9 w-max items-center justify-center rounded-md bg-transparent px-4 py-2 text-sm font-medium transition-colors focus:outline-none',
                link.disabled && 'pointer-events-none opacity-50'
              )}
            >
              {link.title}
            </a>
          )
        }
        // Handle internal links
        return (
          <Link
            key={`${link.title}:${link.href}`}
            to={link.href || '/'}
            className={cn(
              'text-muted-foreground hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground inline-flex h-9 w-max items-center justify-center rounded-md bg-transparent px-4 py-2 text-sm font-medium transition-colors focus:outline-none',
              link.disabled && 'pointer-events-none opacity-50'
            )}
          >
            {link.title}
          </Link>
        )
      })}
    </nav>
  )
}
