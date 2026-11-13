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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  isRedirect,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { z } from 'zod'

import * as authApi from '@/features/auth/api'
import { Route as SignUpRoute } from '@/routes/(auth)/sign-up'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'

import { SignIn } from '../../sign-in'
import { SignUp } from '../index'

/**
 * Sign-up honours `?redirect=` like sign-in: every way out of the page —
 * verified or direct registration, the sign-in fallback, the 2FA hand-off,
 * OAuth, WeChat and the sign-in ⇄ sign-up links — keeps the same-origin
 * destination, and anything else falls back to the default landing.
 */
const { status } = vi.hoisted(() => ({
  status: {
    email_verification: false,
    legal_consent_required: false,
    oauth_register_enabled: false,
    wechat_login: false,
    register_enabled: true,
  } as Record<string, unknown>,
}))

vi.mock('@/hooks/use-status', () => ({ useStatus: () => ({ status }) }))
vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    systemName: 'OmniMux',
    systemLogo: '',
    isLoading: false,
  }),
}))
vi.mock('@/features/auth/hooks/use-turnstile', () => ({
  useTurnstile: () => ({
    isTurnstileEnabled: false,
    turnstileSiteKey: '',
    turnstileToken: '',
    setTurnstileToken: vi.fn(),
    validateTurnstile: () => true,
  }),
}))
// The provider buttons are out of scope; the stub exposes what they receive.
vi.mock('@/features/auth/components/oauth-providers', () => ({
  OAuthProviders: (props: {
    redirectTo?: string
    onWeChatLogin?: () => void
  }) => (
    <div data-testid='oauth' data-redirect={props.redirectTo ?? ''}>
      {props.onWeChatLogin ? (
        <button type='button' onClick={props.onWeChatLogin}>
          stub wechat
        </button>
      ) : null}
    </div>
  ),
}))
vi.mock('@/features/auth/sign-in/components/user-auth-form', () => ({
  UserAuthForm: (props: { redirectTo?: string }) => (
    <div data-testid='sign-in-form' data-redirect={props.redirectTo ?? ''} />
  ),
}))

const BUNDLE = {
  access_token: 'valid-token',
  token_type: 'Bearer',
  access_expires_at: 9999999999,
  user: { id: 101, username: 'new-user', role: 1 },
  session: {
    sid: 'session-id',
    current: true,
    login_method: 'password',
    ip: '',
    user_agent: '',
    created_at: 1,
    last_active_at: 1,
    expires_at: 9999999999,
  },
} as unknown as AuthBundle

const OPEN_REDIRECTS = [
  '//evil.example/influencer',
  '///evil.example',
  '/\\evil.example',
  '\\\\evil.example',
  'https://evil.example/influencer',
  'http:evil.example',
  'javascript:alert(1)',
  'data:text/html,hi',
]

const searchSchema = z.object({ redirect: z.string().optional() })

function renderAuth(initialEntry: string) {
  const rootRoute = createRootRoute({ component: () => <Outlet /> })
  // Same ids as the file routes, so `useSearch({ from })` resolves.
  const authRoute = createRoute({
    getParentRoute: () => rootRoute,
    id: '(auth)',
    component: () => <Outlet />,
  })
  const signUpRoute = createRoute({
    getParentRoute: () => authRoute,
    path: 'sign-up',
    validateSearch: searchSchema,
    component: SignUp,
  })
  const signInRoute = createRoute({
    getParentRoute: () => authRoute,
    path: 'sign-in',
    validateSearch: searchSchema,
    component: SignIn,
  })
  const page = (path: string, text: string) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => <p>{text}</p>,
    })
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      authRoute.addChildren([signUpRoute, signInRoute]),
      page('/', 'home page'),
      page('/influencer', 'influencer page'),
      page('/otp', 'otp page'),
    ]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return router
}

async function submitCredentials() {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText(/email/i), 'new@example.com')
  await user.type(screen.getByLabelText(/^password$/i), 'ValidPass1234')
  await user.click(screen.getByRole('button', { name: /create account/i }))
  return user
}

function mockDirectRegistration() {
  vi.spyOn(authApi, 'register').mockResolvedValue({
    success: true,
    message: 'ok',
  })
  return vi
    .spyOn(authApi, 'login')
    .mockResolvedValue({ success: true, message: 'ok', data: BUNDLE })
}

async function expectLocation(
  router: ReturnType<typeof renderAuth>,
  pathname: string,
  search: Record<string, unknown> = {}
) {
  await waitFor(() => expect(router.state.location.pathname).toBe(pathname))
  expect(router.state.location.search).toEqual(search)
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  if (!document.elementFromPoint) document.elementFromPoint = () => null
  status.email_verification = false
  status.oauth_register_enabled = false
  status.wechat_login = false
  useAuthStore.getState().auth.reset()
})

afterEach(() => {
  cleanup()
  vi.clearAllTimers()
  vi.useRealTimers()
})

describe('sign-up returns to the requested page', () => {
  it('after email verification', async () => {
    status.email_verification = true
    vi.spyOn(authApi, 'sendEmailVerification').mockResolvedValue({
      success: true,
      message: 'sent',
    })
    mockDirectRegistration()
    const router = renderAuth('/sign-up?redirect=%2Finfluencer')

    const user = await submitCredentials()
    await screen.findByRole('heading', { name: /check your inbox/i })
    const otpInput = document.querySelector(
      'input[data-slot="input-otp"]'
    ) as HTMLInputElement
    await user.type(otpInput, '123456')
    const confirm = screen.getByRole('button', { name: /confirm my account/i })
    await waitFor(() => expect(confirm).not.toBeDisabled())
    await user.click(confirm)

    await expectLocation(router, '/influencer')
  })

  it('after direct registration', async () => {
    mockDirectRegistration()
    const router = renderAuth('/sign-up?redirect=%2Finfluencer')
    await submitCredentials()
    await expectLocation(router, '/influencer')
  })

  it('through sign-in when the automatic sign-in fails', async () => {
    vi.spyOn(authApi, 'register').mockResolvedValue({
      success: true,
      message: 'ok',
    })
    vi.spyOn(authApi, 'login').mockRejectedValue(new Error('network'))
    const router = renderAuth('/sign-up?redirect=%2Finfluencer')
    await submitCredentials()
    await expectLocation(router, '/sign-in', { redirect: '/influencer' })
    expect(screen.getByTestId('sign-in-form')).toHaveAttribute(
      'data-redirect',
      '/influencer'
    )
  })

  it('through the 2FA step', async () => {
    vi.spyOn(authApi, 'register').mockResolvedValue({
      success: true,
      message: 'ok',
    })
    vi.spyOn(authApi, 'login').mockResolvedValue({
      success: true,
      message: 'ok',
      data: {
        require_verification: true,
        flow_token: 'flow',
        expires_at: Math.floor(Date.now() / 1000) + 300,
        methods: [{ method: '2fa', available: true }],
      },
    } as never)
    const router = renderAuth('/sign-up?redirect=%2Finfluencer')
    await submitCredentials()
    await expectLocation(router, '/otp')
    expect(
      useAuthStore.getState().auth.pendingLoginVerification?.redirectTo
    ).toBe('/influencer')
  })

  it('through OAuth', async () => {
    status.oauth_register_enabled = true
    renderAuth('/sign-up?redirect=%2Finfluencer')
    expect(await screen.findByTestId('oauth')).toHaveAttribute(
      'data-redirect',
      '/influencer'
    )
  })

  it('through WeChat', async () => {
    status.oauth_register_enabled = true
    status.wechat_login = true
    vi.spyOn(authApi, 'wechatLoginByCode').mockResolvedValue({
      success: true,
      message: 'ok',
      data: BUNDLE,
    } as never)
    const router = renderAuth('/sign-up?redirect=%2Finfluencer')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'stub wechat' }))
    await user.type(
      await screen.findByPlaceholderText(/enter the verification code/i),
      '654321'
    )
    await user.click(screen.getByRole('button', { name: /confirm/i }))
    await expectLocation(router, '/influencer')
  })
})

describe('sign-up without a destination keeps the current behavior', () => {
  it('lands on the homepage', async () => {
    mockDirectRegistration()
    const router = renderAuth('/sign-up')
    await submitCredentials()
    await expectLocation(router, '/')
  })

  it('falls back to a plain sign-in page', async () => {
    vi.spyOn(authApi, 'register').mockResolvedValue({
      success: true,
      message: 'ok',
    })
    vi.spyOn(authApi, 'login').mockRejectedValue(new Error('network'))
    const router = renderAuth('/sign-up')
    await submitCredentials()
    await expectLocation(router, '/sign-in')
  })

  it('gives OAuth no destination', async () => {
    status.oauth_register_enabled = true
    renderAuth('/sign-up')
    expect(await screen.findByTestId('oauth')).toHaveAttribute(
      'data-redirect',
      ''
    )
  })
})

describe('switching between sign-in and sign-up', () => {
  it('keeps the destination from sign-up to sign-in', async () => {
    const router = renderAuth('/sign-up?redirect=%2Finfluencer')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('link', { name: /^sign in$/i }))
    await expectLocation(router, '/sign-in', { redirect: '/influencer' })
  })

  it('keeps the destination from sign-in to sign-up', async () => {
    const router = renderAuth('/sign-in?redirect=%2Finfluencer')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('link', { name: /^sign up$/i }))
    await expectLocation(router, '/sign-up', { redirect: '/influencer' })
  })

  it.each(OPEN_REDIRECTS)('drops %s when switching', async (target) => {
    const router = renderAuth(`/sign-in?redirect=${encodeURIComponent(target)}`)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('link', { name: /^sign up$/i }))
    await expectLocation(router, '/sign-up')
    await user.click(await screen.findByRole('link', { name: /^sign in$/i }))
    await expectLocation(router, '/sign-in')
  })
  it.each(OPEN_REDIRECTS)(
    'drops %s from sign-up to sign-in',
    async (target) => {
      const router = renderAuth(
        `/sign-up?redirect=${encodeURIComponent(target)}`
      )
      const user = userEvent.setup()
      await user.click(await screen.findByRole('link', { name: /^sign in$/i }))
      await expectLocation(router, '/sign-in')
    }
  )
})

describe('sign-up redirect stays on this site', () => {
  it.each(OPEN_REDIRECTS)('lands on the homepage for %s', async (target) => {
    mockDirectRegistration()
    const router = renderAuth(`/sign-up?redirect=${encodeURIComponent(target)}`)
    await submitCredentials()
    await expectLocation(router, '/')
  })
})

describe('signed-in visit to /sign-up', () => {
  function beforeLoadTarget(redirect?: string): string {
    const beforeLoad = SignUpRoute.options.beforeLoad as (ctx: {
      search: { redirect?: string }
    }) => Promise<void>
    return beforeLoad({ search: { redirect } }).then(
      () => 'no redirect',
      (error: unknown) => {
        if (!isRedirect(error)) throw error
        return String(error.options.href)
      }
    ) as unknown as string
  }

  beforeEach(() => {
    useAuthStore.getState().auth.setUser(BUNDLE.user)
  })

  it('goes to the requested page', async () => {
    expect(await beforeLoadTarget('/influencer')).toBe('/influencer')
  })

  it('goes to the homepage without a destination', async () => {
    expect(await beforeLoadTarget()).toBe('/')
  })

  it.each(OPEN_REDIRECTS)('goes to the homepage for %s', async (target) => {
    expect(await beforeLoadTarget(target)).toBe('/')
  })
})
