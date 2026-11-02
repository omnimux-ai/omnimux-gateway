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
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { Toaster, toast } from 'sonner'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/http-client'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { CliLogin } from '..'

const bundle: AuthBundle = {
  access_token: 'local-test-token',
  token_type: 'Bearer',
  access_expires_at: 9999999999,
  user: { id: 901, username: 'local-account', role: 1 },
  session: {
    sid: 'local-session',
    current: true,
    login_method: 'password',
    ip: '',
    user_agent: '',
    created_at: 1,
    last_active_at: 1,
    expires_at: 9999999999,
  },
}

function grantResponse(userCode = 'ABCD-EFGH') {
  return {
    data: {
      success: true,
      data: {
        verification_scope: 'cli.device.approve',
        verification_context: {
          device_flow_id: 41,
          user_code: userCode.replace('-', ''),
          scopes: ['profile:read'],
          expires_at: 9999999999,
        },
      },
    },
  }
}

function proofResponse() {
  return {
    data: {
      success: true,
      data: {
        proof_token: 'local-one-use-proof',
        scope: 'cli.device.approve',
        method: 'password',
        expires_at: 9999999999,
      },
    },
  }
}

function renderAuthorization() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const root = createRootRoute({ component: Outlet })
  const cli = createRoute({
    getParentRoute: () => root,
    path: '/cli/login',
    validateSearch: (search: Record<string, unknown>) => ({
      user_code:
        typeof search.user_code === 'string' ? search.user_code : undefined,
    }),
    component: CliLogin,
  })
  const signIn = createRoute({
    getParentRoute: () => root,
    path: '/sign-in',
    component: () => <p>Sign-in destination</p>,
  })
  const router = createRouter({
    routeTree: root.addChildren([cli, signIn]),
    history: createMemoryHistory({
      initialEntries: ['/cli/login?user_code=ABCD-EFGH'],
    }),
  })
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>
  )
  return { ...view, router, client }
}

async function openVerification(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: 'Authorize CLI' }))
  return screen.findByLabelText('Password', { selector: 'input' })
}

async function submitPassword(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    await screen.findByLabelText('Password', { selector: 'input' }),
    'local-password'
  )
  await user.click(screen.getByRole('button', { name: 'Verify' }))
}

function holdVerification() {
  let resolve!: (value: ReturnType<typeof proofResponse>) => void
  const promise = new Promise<ReturnType<typeof proofResponse>>((finish) => {
    resolve = finish
  })
  vi.mocked(api.post).mockImplementation(async (url) => {
    if (url === '/api/user/device/code') return grantResponse()
    if (url === '/api/verify') return promise
    return { data: { success: true } }
  })
  return { promise, resolve }
}

beforeEach(() => {
  useAuthStore.getState().auth.setBundle(structuredClone(bundle))
  useSystemConfigStore.getState().setLoading(false)
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url !== '/api/verify/methods') throw new Error(`Unexpected GET ${url}`)
    return {
      data: {
        success: true,
        data: {
          scope: 'cli.device.approve',
          methods: [{ method: 'password', available: true }],
          oauth_providers: [],
          password_encryption_enabled: false,
        },
      },
    }
  })
  vi.spyOn(api, 'post').mockImplementation(async (url, body) => {
    if (url === '/api/user/device/code') {
      return grantResponse((body as { user_code: string }).user_code)
    }
    if (url === '/api/verify') return proofResponse()
    if (url === '/api/user/device/approve' || url === '/api/user/device/deny') {
      return { data: { success: true } }
    }
    throw new Error(`Unexpected POST ${url}`)
  })
})

afterEach(async () => {
  await i18next.changeLanguage('en')
  cleanup()
  useAuthStore.getState().auth.reset()
  toast.dismiss()
  vi.restoreAllMocks()
})

it('cancelling native verification sends no approval and allows a new attempt', async () => {
  renderAuthorization()
  const user = userEvent.setup()
  await openVerification(user)
  expect(
    await screen.findByRole('dialog', { name: 'Security verification' })
  ).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(
    vi
      .mocked(api.post)
      .mock.calls.filter(
        ([url]) => url === '/api/verify' || url === '/api/user/device/approve'
      )
  ).toHaveLength(0)
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Authorize CLI' })).toBeEnabled()
})

it('valid verification approves the entered code with one proof and shows success', async () => {
  renderAuthorization()
  const user = userEvent.setup()
  await openVerification(user)
  await submitPassword(user)
  expect(
    await screen.findByText('You can return to the terminal')
  ).toBeVisible()
  expect(api.post).toHaveBeenCalledWith(
    '/api/verify',
    {
      method: 'password',
      password: 'local-password',
      scope: 'cli.device.approve',
      context: grantResponse().data.data.verification_context,
    },
    expect.anything()
  )
  expect(api.post).toHaveBeenCalledWith(
    '/api/user/device/approve',
    { user_code: 'ABCD-EFGH' },
    expect.objectContaining({
      headers: { 'X-Security-Proof': 'local-one-use-proof' },
    })
  )
})

it('denying a code requires no verification and shows that no credentials were issued', async () => {
  renderAuthorization()
  await userEvent.click(await screen.findByRole('button', { name: 'Deny' }))
  expect(
    await screen.findByText('No credentials were issued to the CLI.')
  ).toBeVisible()
  expect(api.get).not.toHaveBeenCalled()
  expect(api.post).toHaveBeenCalledExactlyOnceWith(
    '/api/user/device/deny',
    { user_code: 'ABCD-EFGH' },
    expect.not.objectContaining({ headers: expect.anything() })
  )
})

it('duplicate authorization clicks open one verification and send one approval', async () => {
  renderAuthorization()
  const user = userEvent.setup()
  await user.dblClick(
    await screen.findByRole('button', { name: 'Authorize CLI' })
  )
  await screen.findByLabelText('Password', { selector: 'input' })
  expect(api.get).toHaveBeenCalledTimes(1)
  await submitPassword(user)
  await screen.findByText('You can return to the terminal')
  expect(
    vi
      .mocked(api.post)
      .mock.calls.filter(([url]) => url === '/api/user/device/approve')
  ).toHaveLength(1)
})

it('the code stays locked during verification and a new code requires a fresh attempt', async () => {
  renderAuthorization()
  const user = userEvent.setup()
  await openVerification(user)
  const input = screen.getByPlaceholderText('XXXX-XXXX')
  expect(input).toBeDisabled()
  await user.type(input, 'OTHER')
  expect(input).toHaveValue('ABCD-EFGH')
  await user.click(screen.getByRole('button', { name: 'Cancel' }))
  await user.clear(input)
  await user.type(input, 'JKLM-NOPQ')
  await openVerification(user)
  await submitPassword(user)
  await screen.findByText('You can return to the terminal')
  expect(api.post).toHaveBeenCalledWith(
    '/api/user/device/approve',
    { user_code: 'JKLM-NOPQ' },
    expect.anything()
  )
})

it.each(['account', 'session', 'route-code', 'unmount', 'cancel'] as const)(
  '%s change during pending verification ignores the late proof and sends no approval',
  async (change) => {
    const pending = holdVerification()
    const view = renderAuthorization()
    const user = userEvent.setup()
    await openVerification(user)
    await submitPassword(user)
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith(
        '/api/verify',
        expect.anything(),
        expect.anything()
      )
    )
    if (change === 'cancel') {
      await user.click(screen.getByRole('button', { name: 'Cancel' }))
    } else {
      await act(async () => {
        if (change === 'account') {
          useAuthStore
            .getState()
            .auth.setUser({ id: 902, username: 'other-account', role: 1 })
        }
        if (change === 'session') {
          useAuthStore.getState().auth.setBundle({
            ...bundle,
            session: { ...bundle.session, sid: 'other-session' },
          })
        }
        if (change === 'route-code') {
          await view.router.navigate({
            to: '/cli/login',
            search: { user_code: 'JKLM-NOPQ' },
          })
        }
        if (change === 'unmount') view.unmount()
      })
    }
    await act(async () => {
      pending.resolve(proofResponse())
      await pending.promise
    })
    expect(
      vi
        .mocked(api.post)
        .mock.calls.filter(([url]) => url === '/api/user/device/approve')
    ).toHaveLength(0)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(
      screen.queryByText('You can return to the terminal')
    ).not.toBeInTheDocument()
  }
)

it('a rejected approval keeps the page retryable and never reports success', async () => {
  vi.mocked(api.post).mockImplementation(async (url) => {
    if (url === '/api/user/device/code') return grantResponse()
    if (url === '/api/verify') return proofResponse()
    return { data: { success: false, message: 'Invalid or expired code' } }
  })
  renderAuthorization()
  const user = userEvent.setup()
  await openVerification(user)
  await submitPassword(user)
  expect(await screen.findByText('Invalid or expired code')).toBeVisible()
  expect(
    screen.queryByText('You can return to the terminal')
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Authorize CLI' })).toBeEnabled()
})

it('signed-out authorization keeps the device code in the sign-in redirect', async () => {
  useAuthStore.getState().auth.reset()
  const { router } = renderAuthorization()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Sign in to continue' })
  )
  expect(await screen.findByText('Sign-in destination')).toBeVisible()
  expect(router.state.location.search).toEqual({
    redirect: '/cli/login?user_code=ABCD-EFGH',
  })
  expect(api.post).not.toHaveBeenCalled()
})

it('the authorization page discloses creation, owned history, and account credit consumption before approval', async () => {
  renderAuthorization()
  expect(
    await screen.findByText(
      'Create characters and videos, including multiview images.'
    )
  ).toBeVisible()
  expect(
    screen.getByText(
      'Read and download your character and video creations and their task history.'
    )
  ).toBeVisible()
  expect(
    screen.getByText(
      'Generation uses your account credits or subscription allowance, just like the website.'
    )
  ).toBeVisible()
  expect(screen.getByRole('button', { name: 'Authorize CLI' })).toBeEnabled()
  expect(api.post).not.toHaveBeenCalled()
})

it.each(['grant', 'approval'] as const)(
  'localizes a failed %s response with source parameters and does not approve the device',
  async (stage) => {
    const key = 'Device code expires in {{minutes}} minutes'
    i18next.addResourceBundle('vi', 'translation', {
      [key]: 'Mã thiết bị hết hạn sau {{minutes}} phút',
    })
    await i18next.changeLanguage('vi')
    const notify = vi.spyOn(toast, 'error')
    const success = vi.spyOn(toast, 'success')
    const failure = {
      data: {
        success: false,
        message: 'Device code expires in 4 minutes',
        message_key: key,
        message_params: { minutes: 4 },
      },
    }
    vi.mocked(api.post).mockImplementation(async (url) => {
      if (url === '/api/user/device/code') {
        return stage === 'grant' ? failure : grantResponse()
      }
      if (url === '/api/verify') return proofResponse()
      return failure
    })
    renderAuthorization()
    const user = userEvent.setup()
    await user.click(
      await screen.findByRole('button', { name: 'Authorize CLI' })
    )
    if (stage === 'approval') await submitPassword(user)

    await waitFor(() => {
      expect(notify).toHaveBeenCalledWith('Mã thiết bị hết hạn sau 4 phút')
    })
    expect(success).not.toHaveBeenCalled()
    expect(
      screen.queryByText('You can return to the terminal')
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Authorize CLI' })).toBeEnabled()
    if (stage === 'grant') {
      expect(vi.mocked(api.post).mock.calls.map(([url]) => url)).toEqual([
        '/api/user/device/code',
      ])
    }
  }
)
