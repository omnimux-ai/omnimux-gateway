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
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { api } from '@/lib/http-client'
import { getServerErrorMessage } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import { AuthLayout } from '../auth-layout'

type DeviceGrantResponse = {
  success: boolean
  message?: string
  data?: {
    verification_scope: 'cli.device.approve'
    verification_context: {
      device_flow_id: number
      user_code: string
      scopes: string[]
      expires_at: number
    }
  }
}

type DeviceActionResponse = {
  success: boolean
  message?: string
  data?: { approved?: boolean; denied?: boolean; user_id?: number }
}

export function CliLogin() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ from: '/cli/login' })
  const user = useAuthStore((s) => s.auth.user)
  const sessionId = useAuthStore((s) => s.auth.session?.sid)
  const [userCode, setUserCode] = useState(
    typeof search.user_code === 'string' ? search.user_code : ''
  )
  const [status, setStatus] = useState<'idle' | 'approved' | 'denied'>('idle')
  const [submitting, setSubmitting] = useState(false)
  const currentOperation = useRef<AbortController | null>(null)
  const verification = useSecureVerification()
  const cancelVerification = verification.cancel

  useEffect(
    () => () => {
      const current = currentOperation.current
      currentOperation.current = null
      current?.abort()
      cancelVerification()
      setSubmitting(false)
      setStatus('idle')
    },
    [cancelVerification, user?.id, sessionId, userCode, search.user_code]
  )

  const redirectToSignIn = () => {
    const code = userCode.trim()
    const redirect = code
      ? `/cli/login?user_code=${encodeURIComponent(code)}`
      : '/cli/login'
    void navigate({
      to: '/sign-in',
      search: { redirect },
    })
  }

  const runAction = async (action: 'approve' | 'deny') => {
    if (currentOperation.current) return
    if (!user) {
      redirectToSignIn()
      return
    }
    const code = userCode.trim()
    if (!code) {
      toast.error(t('Enter the code shown in your terminal'))
      return
    }
    const controller = new AbortController()
    const approvingUserId = user.id
    const approvingSessionId = sessionId
    currentOperation.current = controller
    setSubmitting(true)
    try {
      let proof = null
      if (action === 'approve') {
        const grant = await api.post<DeviceGrantResponse>(
          '/api/user/device/code',
          { user_code: code },
          { skipBusinessError: true, signal: controller.signal }
        )
        const metadata = grant.data.data
        if (
          !grant.data.success ||
          metadata?.verification_scope !== 'cli.device.approve'
        ) {
          toast.error(
            getServerErrorMessage(grant.data, t('Invalid or expired code'))
          )
          return
        }
        const liveAuth = useAuthStore.getState().auth
        if (
          currentOperation.current !== controller ||
          liveAuth.user?.id !== approvingUserId ||
          liveAuth.session?.sid !== approvingSessionId
        ) {
          return
        }
        proof = await verification.requestVerification({
          scope: metadata.verification_scope,
          context: metadata.verification_context,
        })
      }
      const currentAuth = useAuthStore.getState().auth
      if (
        currentOperation.current !== controller ||
        (action === 'approve' && !proof) ||
        currentAuth.user?.id !== approvingUserId ||
        currentAuth.session?.sid !== approvingSessionId
      ) {
        return
      }
      const res = await api.post<DeviceActionResponse>(
        `/api/user/device/${action}`,
        { user_code: code },
        {
          skipBusinessError: true,
          signal: controller.signal,
          ...(proof && {
            headers: { 'X-Security-Proof': proof.proof_token },
            singleUseAuthorization: true,
          }),
        }
      )
      if (currentOperation.current !== controller) return
      if (!res.data?.success) {
        toast.error(
          getServerErrorMessage(res.data, t('Invalid or expired code'))
        )
        return
      }
      setStatus(action === 'approve' ? 'approved' : 'denied')
      toast.success(
        action === 'approve'
          ? t('CLI authorization approved')
          : t('CLI authorization denied')
      )
    } catch {
      if (currentOperation.current !== controller) return
      toast.error(t('Invalid or expired code'))
    } finally {
      if (currentOperation.current === controller) {
        currentOperation.current = null
        setSubmitting(false)
      }
    }
  }

  let statusContent
  if (status === 'approved') {
    statusContent = (
      <div className='space-y-4 rounded-lg border p-4'>
        <p className='font-medium'>{t('You can return to the terminal')}</p>
        <p className='text-muted-foreground text-sm'>
          {t(
            'The CLI will finish signing in automatically. You can close this page.'
          )}
        </p>
        <Button variant='outline' render={<Link to='/dashboard' />}>
          {t('Go to dashboard')}
        </Button>
      </div>
    )
  } else if (status === 'denied') {
    statusContent = (
      <div className='space-y-4 rounded-lg border p-4'>
        <p className='font-medium'>{t('Authorization denied')}</p>
        <p className='text-muted-foreground text-sm'>
          {t('No credentials were issued to the CLI.')}
        </p>
      </div>
    )
  } else {
    statusContent = (
      <div className='space-y-6'>
        <div className='space-y-2'>
          <Label htmlFor='user-code'>{t('Device code')}</Label>
          <Input
            id='user-code'
            value={userCode}
            disabled={submitting}
            onChange={(e) => setUserCode(e.target.value.toUpperCase())}
            placeholder='XXXX-XXXX'
            autoComplete='one-time-code'
            className='text-center font-mono text-2xl tracking-[0.3em]'
          />
        </div>

        <div className='space-y-3 text-sm'>
          <ul className='text-muted-foreground list-disc space-y-2 ps-4'>
            <li>
              {t('Create characters and videos, including multiview images.')}
            </li>
            <li>
              {t(
                'Read and download your character and video creations and their task history.'
              )}
            </li>
          </ul>
          <p className='font-medium'>
            {t(
              'Generation uses your account credits or subscription allowance, just like the website.'
            )}
          </p>
        </div>

        {!user ? (
          <div className='space-y-3'>
            <p className='text-muted-foreground text-sm'>
              {t('Sign in to confirm this CLI authorization request.')}
            </p>
            <Button className='w-full' onClick={redirectToSignIn}>
              {t('Sign in to continue')}
            </Button>
          </div>
        ) : (
          <div className='flex flex-col gap-3 sm:flex-row'>
            <Button
              className='flex-1'
              disabled={submitting}
              onClick={() => void runAction('approve')}
            >
              {t('Authorize CLI')}
            </Button>
            <Button
              className='flex-1'
              variant='outline'
              disabled={submitting}
              onClick={() => void runAction('deny')}
            >
              {t('Deny')}
            </Button>
          </div>
        )}
      </div>
    )
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('Authorize OmniMux CLI')}
          </h2>
          <p className='text-muted-foreground text-sm sm:text-base'>
            {t(
              'Authorize this device to use OmniMux CLI for 30 days. This replaces only your previous CLI device token; other access tokens stay active.'
            )}
          </p>
        </div>

        {statusContent}
        <SecureVerificationDialog {...verification.dialogProps} />
      </div>
    </AuthLayout>
  )
}
