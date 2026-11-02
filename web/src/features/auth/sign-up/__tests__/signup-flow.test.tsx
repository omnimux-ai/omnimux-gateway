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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as authApi from '@/features/auth/api'
import { SignUp } from '../index'

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: {
      email_verification: true,
      legal_consent_required: false,
      oauth_register_enabled: false,
      wechat_login: false,
    },
  }),
}))

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

async function renderSignUpPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const rootRoute = createRootRoute({ component: () => <Outlet /> })
  const signUpRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: SignUp,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([signUpRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const view = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await router.load()
  return view
}

describe('Streamlined Sign-up Flow', () => {
  beforeEach(() => {
    // input-otp 的焦点/选择恢复定时器（0/10/50ms，见其 ht() 辅助）由组件内部持有，
    // 卸载时不会被清理。真实定时器一旦在 jsdom 环境拆除后才触发，回调里的 React
    // 状态更新就会抛 `window is not defined`，vitest 因此「测试全过却退 1」。
    // 假定时器行为与真实时间一致（shouldAdvanceTime），但结束时可以显式取消，
    // 这样任何定时器都不可能活过本文件的环境。
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.clearAllMocks()
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    if (!document.elementFromPoint) {
      document.elementFromPoint = () => null
    }
  })

  afterEach(() => {
    cleanup()
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('renders initial form with only email and password, omitting username and confirm password', async () => {
    await renderSignUpPage()

    // 标题
    expect(screen.getByRole('heading', { name: /create an account/i })).toBeInTheDocument()

    // 字段验证：仅包含 Email 与 Password
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^password$/i)).toBeInTheDocument()

    // 严格确保已移除用户名、确认密码与首屏验证码
    expect(screen.queryByLabelText(/username/i)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/confirm password/i)).not.toBeInTheDocument()
    expect(screen.queryByPlaceholderText(/verification code/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /send code/i })).not.toBeInTheDocument()

    // 提交按钮
    expect(screen.getByRole('button', { name: /create account/i })).toBeInTheDocument()
  })

  it('transitions to verification step on valid submission and executes register with generated username', async () => {
    const user = userEvent.setup()
    const sendVerificationSpy = vi
      .spyOn(authApi, 'sendEmailVerification')
      .mockResolvedValue({ success: true, message: 'sent' })
    const registerSpy = vi
      .spyOn(authApi, 'register')
      .mockResolvedValue({ success: true, message: 'ok' })
    const loginSpy = vi
      .spyOn(authApi, 'login')
      .mockResolvedValue({
        success: true,
        message: 'ok',
        data: {
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
        },
      })

    await renderSignUpPage()

    // 第一步：输入邮箱和密码
    await user.type(screen.getByLabelText(/email/i), 'testuser@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'ValidPass1234')

    // 点击创建账户
    await user.click(screen.getByRole('button', { name: /create account/i }))

    // 验证调用发送邮件验证码接口
    await waitFor(() => {
      expect(sendVerificationSpy).toHaveBeenCalledWith('testuser@example.com', '')
    })

    // 验证切换至第二步：Check your inbox 界面（对标图 2）
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /check your inbox/i })).toBeInTheDocument()
    })
    expect(screen.getByText('testuser@example.com')).toBeInTheDocument()
    expect(screen.getByText(/check your spam folder/i)).toBeInTheDocument()

    const confirmButton = screen.getByRole('button', { name: /confirm my account/i })
    expect(confirmButton).toBeInTheDocument()
    expect(confirmButton).toBeDisabled() // 尚未输入 6 位验证码时应禁用

    // 输入 6 位验证码
    const otpInput = document.querySelector('input[data-slot="input-otp"]') as HTMLInputElement
    expect(otpInput).toBeInTheDocument()
    await user.type(otpInput, '123456')

    // 验证码填满后按钮启用
    await waitFor(() => {
      expect(confirmButton).not.toBeDisabled()
    })

    // 提交验证码
    await user.click(confirmButton)

    // 验证 register 调用携带了自动生成的唯一用户名、原密码、邮箱及验证码
    await waitFor(() => {
      expect(registerSpy).toHaveBeenCalled()
      const payload = registerSpy.mock.calls[0]?.[0]
      expect(payload?.email).toBe('testuser@example.com')
      expect(payload?.password).toBe('ValidPass1234')
      expect(payload?.verification_code).toBe('123456')
      expect(typeof payload?.username).toBe('string')
      expect(payload?.username?.startsWith('u_')).toBe(true)
      expect(payload?.username?.length).toBeLessThanOrEqual(20)
    })

    // 验证注册成功后立即调用 login 自动登录并跳转至 /influencer
    await waitFor(() => {
      expect(loginSpy).toHaveBeenCalledWith({
        username: 'testuser@example.com',
        password: 'ValidPass1234',
        turnstile: '',
      })
    })
  })

  it('allows user to navigate back to edit email and password from verification step', async () => {
    const user = userEvent.setup()
    vi.spyOn(authApi, 'sendEmailVerification').mockResolvedValue({ success: true, message: 'sent' })

    await renderSignUpPage()

    await user.type(screen.getByLabelText(/email/i), 'change@example.com')
    await user.type(screen.getByLabelText(/^password$/i), 'MySecret888')
    await user.click(screen.getByRole('button', { name: /create account/i }))

    // 确认已进入第二步
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /check your inbox/i })).toBeInTheDocument()
    })

    // 点击返回修改邮箱按钮
    const changeEmailBtn = screen.getByRole('button', { name: /change email/i })
    await user.click(changeEmailBtn)

    // 确认返回第一步，且之前填写的邮箱与密码被保留
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /create an account/i })).toBeInTheDocument()
    })
    expect(screen.getByLabelText(/email/i)).toHaveValue('change@example.com')
    expect(screen.getByLabelText(/^password$/i)).toHaveValue('MySecret888')
  })
})
