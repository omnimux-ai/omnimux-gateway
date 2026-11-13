import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { AuthBundle } from '@/stores/auth-store'

import { useAuthRedirect } from '../use-auth-redirect'

const mocks = vi.hoisted(() => ({
  navigate: vi.fn().mockResolvedValue(undefined),
  applyAuthBundle: vi.fn(),
  state: { auth: { session: undefined as { sid: string } | undefined } },
}))

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => mocks.navigate }))
vi.mock('@/lib/api', () => ({
  applyAuthBundle: mocks.applyAuthBundle,
  isAuthBundle: () => true,
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof mocks.state) => unknown) => selector(mocks.state),
    { getState: () => mocks.state }
  ),
}))

const bundle = { user: { id: 1, username: 'user', role: 1 } } as AuthBundle

describe('successful login navigation', () => {
  beforeEach(() => {
    mocks.state.auth.session = undefined
  })

  test('successful login without a return destination returns to the homepage', async () => {
    const { result } = renderHook(() => useAuthRedirect())
    await act(async () => {
      await result.current.handleLoginSuccess(bundle)
    })
    expect(mocks.navigate).toHaveBeenCalledWith({
      href: '/',
      replace: true,
    })
  })

  test('successful login preserves an explicit dashboard destination', async () => {
    const { result } = renderHook(() => useAuthRedirect())
    await act(async () => {
      await result.current.handleLoginSuccess(bundle, '/dashboard/overview')
    })
    expect(mocks.navigate).toHaveBeenCalledWith({
      href: '/dashboard/overview',
      replace: true,
    })
  })

  test('successful login rejects an external return destination', async () => {
    const { result } = renderHook(() => useAuthRedirect())
    await act(async () => {
      await result.current.handleLoginSuccess(bundle, 'https://outside.example')
    })
    expect(mocks.navigate).toHaveBeenCalledWith({
      href: '/',
      replace: true,
    })
  })

  test('a stale login response does not navigate after the session changes', async () => {
    const { result } = renderHook(() => useAuthRedirect())
    mocks.state.auth.session = { sid: 'new-session' }
    await act(async () => {
      await result.current.handleLoginSuccess(bundle)
    })
    expect(mocks.navigate).not.toHaveBeenCalled()
  })
})
