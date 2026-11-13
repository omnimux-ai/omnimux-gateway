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
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { ChevronDown } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { BrandWordmark } from '@/components/brand-wordmark'
import { Dialog } from '@/components/dialog'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SystemUpdateAction } from '@/features/system-update/system-update-action'
import { useNotifications } from '@/hooks/use-notifications'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { defaultTopNavLinks } from '../config/top-nav.config'
import type { TopNavLink } from '../types'

const AUTH_PROMPT_SECONDS = 5

/*
 * 头部「展开 → 收敛」的几何量。取值与改造前的 Tailwind 类一一对应：
 * h-16 / h-12、px-2、pl-4 / pr-1.5、pt-3、rounded-2xl；展开态宽度即
 * max-w-7xl（1280）减去 md:px-6（48）。
 *
 * 收敛态宽度不在这里写死——它等于导航内容的固有宽度，会随语言文案、断点与
 * 字体变化，因此由 measureCompactWidth() 在运行时测量。
 */
const NAV_HEIGHT_EXPANDED = 64 // h-16
const NAV_HEIGHT_COMPACT = 48 // h-12
const NAV_PAD_X_EXPANDED = 8 // px-2
const NAV_PAD_LEFT_COMPACT = 16 // pl-4
const NAV_PAD_RIGHT_COMPACT = 6 // pr-1.5
const SHELL_PAD_TOP_COMPACT = 12 // pt-3
const NAV_WIDTH_EXPANDED = 1232 // max-w-7xl (1280) - md:px-6 (48)
const NAV_RADIUS_COMPACT = 'calc(var(--radius) * 1.8)' // rounded-2xl 的展开式

/*
 * 收敛从滚动 4px 处开始、164px 处走完。用一段区间而不是一个阈值：几何量随
 * 滚动位置连续插值，动画跟着手势走，反向滚动时原路返回。
 */
const COLLAPSE_START_PX = 4
const COLLAPSE_RANGE_PX = 160

const SHELL_GEOMETRY_EXPANDED: React.CSSProperties = { paddingTop: 0 }

const NAV_GEOMETRY_EXPANDED: React.CSSProperties = {
  height: NAV_HEIGHT_EXPANDED,
  maxWidth: NAV_WIDTH_EXPANDED,
  paddingLeft: NAV_PAD_X_EXPANDED,
  paddingRight: NAV_PAD_X_EXPANDED,
  borderRadius: 0,
}

const GLASS_GEOMETRY_HIDDEN: React.CSSProperties = {
  opacity: 0,
  display: 'none',
}

/**
 * 量出导航内容在收敛态下的固有宽度（含收敛态左右内边距）。
 *
 * 量的是屏外克隆而不是导航本体：直接把它切成 `w-fit` 再切回来会触发一次可见
 * 的重排。原来的 `w-fit` 之所以可靠，靠的正是这个量；在这里把它显式测出来，
 * 才能让它参与插值。
 */
function measureCompactWidth(nav: HTMLElement): number {
  const probe = nav.cloneNode(true) as HTMLElement
  probe.setAttribute('aria-hidden', 'true')
  probe.style.cssText =
    'position:fixed;top:0;left:-99999px;width:max-content;visibility:hidden;' +
    `pointer-events:none;height:${NAV_HEIGHT_COMPACT}px;` +
    `padding-left:${NAV_PAD_LEFT_COMPACT}px;` +
    `padding-right:${NAV_PAD_RIGHT_COMPACT}px`
  document.body.appendChild(probe)
  const width = Math.ceil(probe.getBoundingClientRect().width)
  probe.remove()
  return width
}

type AuthPromptTarget = {
  title: string
  href: string
}

export interface PublicHeaderProps {
  navLinks?: TopNavLink[]
  mobileLinks?: TopNavLink[]
  navContent?: React.ReactNode
  logo?: React.ReactNode
  siteName?: string
  homeUrl?: string
  leftContent?: React.ReactNode
  rightContent?: React.ReactNode
  showNavigation?: boolean
  showAuthButtons?: boolean
  showNotifications?: boolean
  className?: string
  fullWidth?: boolean
}

function HeaderNavItemWithDropdown({
  link,
  pathname,
  onNavigate,
}: {
  link: TopNavLink
  pathname: string
  onNavigate?: () => void
}) {
  const { t } = useTranslation()
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

  const items = link.items || []
  const hasActiveChild = items.some((item) => pathname === item.href)

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
          'text-muted-foreground hover:text-foreground flex cursor-pointer items-center gap-1 min-w-0 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors duration-200',
          (hasActiveChild || open) && 'text-foreground font-semibold'
        )}
        aria-expanded={open}
        aria-haspopup='true'
      >
        <span>{t(link.title)}</span>
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
                onClick={() => {
                  setOpen(false)
                  onNavigate?.()
                }}
                className={cn(
                  'text-popover-foreground hover:bg-accent hover:text-accent-foreground flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors',
                  pathname === subItem.href &&
                    'bg-accent text-accent-foreground font-semibold'
                )}
                role='menuitem'
              >
                {subItem.icon && (
                  <subItem.icon className='text-muted-foreground size-4' />
                )}
                <span>{t(subItem.title)}</span>
              </Link>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export function PublicHeader(props: PublicHeaderProps) {
  const {
    navLinks = defaultTopNavLinks,
    logo: customLogo,
    siteName: customSiteName,
    homeUrl = '/',
    showAuthButtons = true,
    showNotifications = true,
    fullWidth = false,
  } = props

  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [authPromptTarget, setAuthPromptTarget] =
    useState<AuthPromptTarget | null>(null)
  const [authPromptSecondsLeft, setAuthPromptSecondsLeft] =
    useState(AUTH_PROMPT_SECONDS)
  const { auth } = useAuthStore()
  const shellRef = useRef<HTMLDivElement>(null)
  const navRef = useRef<HTMLElement>(null)
  const glassRef = useRef<HTMLDivElement>(null)
  const { systemName, loading } = useSystemConfig()
  const dynamicLinks = useTopNavLinks()
  const notifications = useNotifications()
  const routerState = useRouterState()
  const pathname = routerState.location.pathname

  const user = auth.user
  const isAuthenticated = !!user
  const displaySiteName = customSiteName || systemName
  const links = dynamicLinks.length > 0 ? dynamicLinks : navLinks

  /*
   * 头部收敛：几何量按滚动进度连续插值，直接写进 DOM，不走 React state。
   *
   * 之前是 `window.scrollY > 20` 的二值切换 + `transition-all duration-700
   * ease-[cubic-bezier(0.16,1,0.3,1)]`。两个问题叠加成"缩放太快、不丝滑"：
   * 一是 easeOutExpo 把约 80% 的位移压进前 10% 时长（70ms 内就基本缩完）；
   * 二是宽度在 `max-w-7xl` 与 `w-fit` 之间切换，而 `fit-content` 不参与插值，
   * 于是宽度整段瞬移。改成随滚动插值后，两个问题一起消失，且反向滚动原路返回。
   *
   * 用 ref 直接写样式而不是 setState：滚动期间每帧都会变，走 React 会让整个
   * 头部（含通知、导航、系统配置等 hook）每帧重渲染，而这些量只有 6 个。
   */
  const logoContent: ReactNode = customLogo || <BrandWordmark />

  let authContent = (
    <Button
      size='sm'
      className='h-8 rounded-lg px-3.5 text-xs font-medium'
      render={<Link to='/sign-in' />}
    >
      {t('Sign in')}
    </Button>
  )
  if (isAuthenticated) authContent = <ProfileDropdown />
  if (loading) authContent = <Skeleton className='h-8 w-20 rounded-lg' />

  useEffect(() => {
    const shell = shellRef.current
    const nav = navRef.current
    const glass = glassRef.current
    if (!shell || !nav || !glass) return

    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
    const initialWidth = fullWidth
      ? shell.clientWidth || 1440
      : NAV_WIDTH_EXPANDED
    let compactWidth = initialWidth
    let applied = -1

    const lerp = (from: number, to: number, p: number) => from + (to - from) * p

    const apply = (p: number) => {
      if (Math.abs(p - applied) < 0.002) return
      applied = p

      shell.style.paddingTop = `${lerp(0, SHELL_PAD_TOP_COMPACT, p)}px`
      nav.style.height = `${lerp(NAV_HEIGHT_EXPANDED, NAV_HEIGHT_COMPACT, p)}px`
      nav.style.maxWidth =
        p === 0 && fullWidth
          ? '100%'
          : `${Math.round(lerp(initialWidth, compactWidth, p))}px`
      nav.style.paddingLeft = `${lerp(NAV_PAD_X_EXPANDED, NAV_PAD_LEFT_COMPACT, p)}px`
      nav.style.paddingRight = `${lerp(NAV_PAD_X_EXPANDED, NAV_PAD_RIGHT_COMPACT, p)}px`
      nav.style.borderRadius =
        p === 0 ? '0px' : `calc(${NAV_RADIUS_COMPACT} * ${p})`

      // 玻璃层的透明度同时充当模糊、底色、描边与阴影的淡入系数。
      // 展开态直接不渲染（display:none），避免白留一层 backdrop-filter。
      glass.style.display = p === 0 ? 'none' : ''
      glass.style.opacity = `${p}`
    }

    const read = () => {
      if (!nav.isConnected) return
      const raw = (window.scrollY - COLLAPSE_START_PX) / COLLAPSE_RANGE_PX
      // 减弱动效时不插值，直接切到终态
      const p = reduceMotion.matches
        ? Math.ceil(raw)
        : Math.min(1, Math.max(0, raw))
      apply(Math.min(1, Math.max(0, p)))
    }

    // 语言、断点、字体都会改变导航内容的固有宽度，所以收敛态宽度要重测。
    const remeasure = () => {
      const width = measureCompactWidth(nav)
      if (width > 0) compactWidth = width
      applied = -1
      read()
    }

    remeasure()
    // 直接在 scroll 回调里算，不排 rAF：滚动事件本来就按帧合并，而这里的写入
    // 只有 6 个样式属性、且不读布局。挂到 rAF 上会让头部在被节流的场景（后台
    // 标签页、窗口被遮挡）滞后甚至停在旧状态。
    window.addEventListener('scroll', read, { passive: true })
    window.addEventListener('resize', remeasure, { passive: true })
    // 字体就绪后字宽可能变化，再校一次
    document.fonts?.ready.then(remeasure).catch(() => {})

    return () => {
      window.removeEventListener('scroll', read)
      window.removeEventListener('resize', remeasure)
    }
  }, [i18n.language, displaySiteName, fullWidth])

  useEffect(() => {
    document.body.style.overflow = mobileOpen ? 'hidden' : ''
    return () => {
      document.body.style.overflow = ''
    }
  }, [mobileOpen])

  useEffect(() => {
    if (!authPromptTarget) return

    const intervalId = window.setInterval(() => {
      setAuthPromptSecondsLeft((seconds) => Math.max(seconds - 1, 0))
    }, 1000)

    const timeoutId = window.setTimeout(() => {
      const redirect = authPromptTarget.href
      setAuthPromptTarget(null)
      navigate({ to: '/sign-in', search: { redirect } })
    }, AUTH_PROMPT_SECONDS * 1000)

    return () => {
      window.clearInterval(intervalId)
      window.clearTimeout(timeoutId)
    }
  }, [authPromptTarget, navigate])

  const closeAuthPrompt = useCallback(() => {
    setAuthPromptTarget(null)
    setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
  }, [])

  const navigateToSignIn = useCallback(() => {
    const redirect = authPromptTarget?.href || '/'
    setAuthPromptTarget(null)
    navigate({ to: '/sign-in', search: { redirect } })
  }, [authPromptTarget?.href, navigate])

  const handleNavLinkClick = useCallback(
    (
      event: React.MouseEvent<HTMLAnchorElement>,
      link: TopNavLink,
      closeMobile = false
    ) => {
      if (link.disabled) {
        event.preventDefault()
        return
      }

      if (link.requiresAuth) {
        event.preventDefault()
        if (closeMobile) {
          setMobileOpen(false)
        }
        setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
        setAuthPromptTarget({
          title: t(link.title),
          href: link.href || '/',
        })
        return
      }

      if (closeMobile) {
        setMobileOpen(false)
      }
    },
    [t]
  )

  return (
    <>
      <header className='pointer-events-none fixed inset-x-0 top-0 z-50'>
        <div
          ref={shellRef}
          className={cn(
            'pointer-events-auto mx-auto flex w-full justify-center',
            fullWidth
              ? 'max-w-[1440px] px-4 sm:px-6 xl:px-8'
              : 'max-w-7xl px-4 md:px-6'
          )}
          style={SHELL_GEOMETRY_EXPANDED}
        >
          <nav
            ref={navRef}
            className='relative flex w-full min-w-0 items-center justify-between'
            style={
              fullWidth
                ? { ...NAV_GEOMETRY_EXPANDED, maxWidth: '100%' }
                : NAV_GEOMETRY_EXPANDED
            }
          >
            {/*
              玻璃层：随滚动进度淡入，模糊、底色、描边、阴影一并出现。
              `-z-10` 让它落在导航文字之下；header 是 `fixed` + `z-50` 的层叠
              上下文，所以它仍然覆盖在页面内容之上，不会沉到背景里。
            */}
            <div
              ref={glassRef}
              aria-hidden='true'
              className='bg-background/60 ring-border/50 pointer-events-none absolute inset-0 -z-10 rounded-[inherit] shadow-[0_2px_16px_-6px_rgba(0,0,0,0.08),0_0_0_0.5px_rgba(0,0,0,0.02)] ring-[0.5px] backdrop-blur-2xl dark:shadow-[0_2px_16px_-6px_rgba(0,0,0,0.4)]'
              style={GLASS_GEOMETRY_HIDDEN}
            />
            {/* Logo */}
            <div className='@container/system-brand flex min-w-0 flex-1 items-center gap-1 lg:min-w-36'>
              <Link
                to={homeUrl}
                className='group flex min-w-0 items-center gap-2.5'
              >
                {logoContent}
              </Link>
              <SystemUpdateAction presentation='version' />
            </div>

            {/* Desktop nav */}
            <div className='hidden min-w-0 items-center gap-0.5 lg:flex'>
              {links.map((link) => {
                if (link.items && link.items.length > 0) {
                  return (
                    <HeaderNavItemWithDropdown
                      key={link.title}
                      link={link}
                      pathname={pathname}
                    />
                  )
                }
                const isActive = pathname === link.href
                if (link.external) {
                  return (
                    <a
                      key={`${link.title}:${link.href}`}
                      href={link.href}
                      title={t(link.title)}
                      target='_blank'
                      rel='noopener noreferrer'
                      aria-disabled={link.disabled}
                      tabIndex={link.disabled ? -1 : undefined}
                      onClick={(event) => handleNavLinkClick(event, link)}
                      className={cn(
                        'text-muted-foreground hover:text-foreground min-w-0 truncate rounded-lg px-3 py-1.5 text-sm font-medium transition-colors duration-200',
                        link.disabled && 'pointer-events-none opacity-50'
                      )}
                    >
                      {t(link.title)}
                    </a>
                  )
                }
                return (
                  <Link
                    key={`${link.title}:${link.href}`}
                    to={link.href || '/'}
                    title={t(link.title)}
                    disabled={link.disabled}
                    onClick={(event) => handleNavLinkClick(event, link)}
                    className={cn(
                      'min-w-0 truncate rounded-lg px-3 py-1.5 text-sm font-medium transition-colors duration-200',
                      isActive
                        ? 'text-foreground'
                        : 'text-muted-foreground hover:text-foreground',
                      link.disabled && 'pointer-events-none opacity-50'
                    )}
                  >
                    {t(link.title)}
                  </Link>
                )
              })}

              {showNotifications && (
                <div className='bg-border/40 mx-2 h-4 w-px' />
              )}

              {/* No language control on public pages: visitors get their system
                  language, and only the console offers a manual choice. */}
              {showNotifications && (
                <NotificationPopover
                  open={notifications.popoverOpen}
                  onOpenChange={notifications.setPopoverOpen}
                  unreadCount={notifications.unreadCount}
                  activeTab={notifications.activeTab}
                  onTabChange={notifications.setActiveTab}
                  notice={notifications.notice}
                  announcements={notifications.announcements}
                  loading={notifications.loading}
                />
              )}

              {showAuthButtons && (
                <>
                  <div className='bg-border/40 mx-1 h-4 w-px' />
                  {authContent}
                </>
              )}
            </div>

            {/* Mobile/tablet: compact actions + hamburger */}
            <div className='flex shrink-0 items-center gap-2 lg:hidden'>
              {showAuthButtons && !loading && isAuthenticated && (
                <ProfileDropdown />
              )}
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-9'
                onClick={() => setMobileOpen((v) => !v)}
                aria-label={t('Toggle navigation menu')}
              >
                <div className='relative size-4'>
                  <span
                    className={cn(
                      'absolute inset-x-0 block h-[1.5px] origin-center rounded-full bg-current transition-all duration-300',
                      mobileOpen ? 'top-[7px] rotate-45' : 'top-[3px]'
                    )}
                  />
                  <span
                    className={cn(
                      'absolute inset-x-0 top-[7px] block h-[1.5px] rounded-full bg-current transition-all duration-300',
                      mobileOpen ? 'scale-x-0 opacity-0' : 'opacity-100'
                    )}
                  />
                  <span
                    className={cn(
                      'absolute inset-x-0 block h-[1.5px] origin-center rounded-full bg-current transition-all duration-300',
                      mobileOpen ? 'top-[7px] -rotate-45' : 'top-[11px]'
                    )}
                  />
                </div>
              </Button>
            </div>
          </nav>
        </div>
      </header>

      {/* Mobile full-screen overlay */}
      <div
        className={cn(
          'bg-background/98 fixed inset-0 z-40 backdrop-blur-2xl transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)] lg:pointer-events-none lg:hidden',
          mobileOpen
            ? 'pointer-events-auto opacity-100'
            : 'pointer-events-none opacity-0'
        )}
      >
        <div className='flex h-full flex-col justify-between px-8 pt-20 pb-10'>
          <nav className='flex flex-col gap-1'>
            {links.flatMap((link, i) => {
              if (link.items && link.items.length > 0) {
                return link.items.map((subItem) => {
                  const isActive = pathname === subItem.href
                  const linkClassName = cn(
                    'flex items-center gap-3 py-3 text-base font-medium tracking-tight transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]',
                    mobileOpen
                      ? 'translate-y-0 opacity-100'
                      : 'translate-y-4 opacity-0',
                    isActive ? 'text-foreground' : 'text-muted-foreground',
                    subItem.disabled && 'pointer-events-none opacity-50'
                  )
                  const transitionStyle = {
                    transitionDelay: mobileOpen ? `${100 + i * 50}ms` : '0ms',
                  }
                  return (
                    <Link
                      key={`${link.title}:${subItem.href}`}
                      to={subItem.href}
                      title={t(subItem.title)}
                      onClick={(event) =>
                        handleNavLinkClick(
                          event,
                          { title: subItem.title, href: subItem.href },
                          true
                        )
                      }
                      className={linkClassName}
                      style={transitionStyle}
                    >
                      {t(subItem.title)}
                    </Link>
                  )
                })
              }
              const isActive = pathname === link.href
              const linkClassName = cn(
                'flex items-center gap-3 py-3 text-base font-medium tracking-tight transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]',
                mobileOpen
                  ? 'translate-y-0 opacity-100'
                  : 'translate-y-4 opacity-0',
                isActive ? 'text-foreground' : 'text-muted-foreground',
                link.disabled && 'pointer-events-none opacity-50'
              )
              const transitionStyle = {
                transitionDelay: mobileOpen ? `${100 + i * 50}ms` : '0ms',
              }
              if (link.external) {
                return [
                  <a
                    key={`${link.title}:${link.href}`}
                    href={link.href}
                    target='_blank'
                    rel='noopener noreferrer'
                    aria-disabled={link.disabled}
                    tabIndex={link.disabled ? -1 : undefined}
                    onClick={(event) => handleNavLinkClick(event, link, true)}
                    className={linkClassName}
                    style={transitionStyle}
                  >
                    {t(link.title)}
                  </a>,
                ]
              }
              return [
                <Link
                  key={`${link.title}:${link.href}`}
                  to={link.href || '/'}
                  disabled={link.disabled}
                  onClick={(event) => handleNavLinkClick(event, link, true)}
                  className={linkClassName}
                  style={transitionStyle}
                >
                  {t(link.title)}
                </Link>,
              ]
            })}
          </nav>

          <div
            className={cn(
              'flex flex-col gap-3 transition-all duration-500',
              mobileOpen
                ? 'translate-y-0 opacity-100'
                : 'translate-y-4 opacity-0'
            )}
            style={{ transitionDelay: mobileOpen ? '250ms' : '0ms' }}
          >
            {showAuthButtons && (
              <Link
                to={isAuthenticated ? '/dashboard' : '/sign-in'}
                onClick={() => setMobileOpen(false)}
                className='bg-foreground text-background inline-flex h-10 items-center justify-center rounded-lg text-sm font-medium transition-opacity hover:opacity-90 active:opacity-80'
              >
                {isAuthenticated ? t('Go to Dashboard') : t('Sign in')}
              </Link>
            )}
          </div>
        </div>
      </div>

      <Dialog
        open={!!authPromptTarget}
        onOpenChange={(open) => {
          if (!open) {
            closeAuthPrompt()
          }
        }}
        title={t('Sign in required')}
        description={t('Please sign in to view {{module}}.', {
          module: authPromptTarget?.title || '',
        })}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        footer={
          <>
            <Button variant='outline' onClick={closeAuthPrompt}>
              {t('Cancel')}
            </Button>
            <Button onClick={navigateToSignIn}>{t('Sign in now')}</Button>
          </>
        }
      >
        <div className='bg-muted/40 text-muted-foreground rounded-lg px-3 py-2 text-sm'>
          {t('Redirecting to sign in in {{seconds}} seconds.', {
            seconds: authPromptSecondsLeft,
          })}
        </div>
      </Dialog>
    </>
  )
}
