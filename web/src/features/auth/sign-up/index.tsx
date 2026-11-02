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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { login, register } from '@/features/auth/api'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useEmailVerification } from '@/features/auth/hooks/use-email-verification'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'
import { defaultAuthRedirect } from '@/features/auth/lib/auth-redirect'
import { getAffiliateCode } from '@/features/auth/lib/storage'
import { useStatus } from '@/hooks/use-status'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'
import { createServerError } from '@/lib/server-error-message'

import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'
import {
  SignUpForm,
  type SignUpCredentials,
} from './components/sign-up-form'
import { VerificationStep } from './components/verification-step'

type SignUpStep = 'credentials' | 'verification'

export function SignUp() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [step, setStep] = useState<SignUpStep>('credentials')
  const [credentials, setCredentials] = useState<SignUpCredentials | null>(null)
  const [isRegistering, setIsRegistering] = useState(false)
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)

  const {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  } = useTurnstile()

  const { redirectToLogin, handleLoginResult } = useAuthRedirect()

  const {
    isSending: isSendingCode,
    secondsLeft,
    sendCode,
  } = useEmailVerification({
    turnstileToken,
    validateTurnstile,
  })

  const handleSendVerificationCode = async (email: string) => {
    const success = await sendCode(email)
    if (success) {
      setTurnstileToken('')
      setTurnstileWidgetKey((current) => current + 1)
    }
    return success
  }

  const handleRequireVerification = (newCredentials: SignUpCredentials) => {
    setCredentials(newCredentials)
    setStep('verification')
  }

  const handleConfirmVerification = async (verificationCode: string) => {
    if (!credentials) return

    setIsRegistering(true)
    try {
      const res = await register({
        username: credentials.username,
        password: credentials.password,
        email: credentials.email,
        verification_code: verificationCode,
        aff_code: getAffiliateCode(),
        turnstile: turnstileToken,
      })

      if (res?.success) {
        try {
          const loginRes = await login({
            username: credentials.email || credentials.username,
            password: credentials.password,
            turnstile: turnstileToken,
          })
          if (loginRes?.success) {
            toast.success(t('Signed in'))
            await handleLoginResult(loginRes.data, defaultAuthRedirect())
            return
          }
        } catch {
          // Fallback to manual login redirect if auto-login encounters unexpected issues
        }
        toast.success(t('Account created! Please sign in'))
        redirectToLogin()
      } else {
        handleServerError(createServerError(res, t('Failed to create account')))
      }
    } catch (error) {
      handleServerError(
        AuthOperationError.from(error, t('Failed to create account'))
      )
    } finally {
      setIsRegistering(false)
    }
  }

  const handleBackToCredentials = () => {
    setStep('credentials')
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        {step === 'credentials' ? (
          <>
            <div className='space-y-2'>
              <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
                {t('Create an account')}
              </h2>
              <p className='text-muted-foreground text-left text-sm sm:text-base'>
                {t('Already have an account?')}{' '}
                <Link
                  to='/sign-in'
                  className='hover:text-primary font-medium underline underline-offset-4'
                >
                  {t('Sign in')}
                </Link>
                .
              </p>
            </div>

            <SignUpForm
              initialValues={{
                email: credentials?.email,
                password: credentials?.password,
              }}
              turnstileToken={turnstileToken}
              turnstileSiteKey={turnstileSiteKey}
              isTurnstileEnabled={isTurnstileEnabled}
              turnstileWidgetKey={turnstileWidgetKey}
              setTurnstileToken={setTurnstileToken}
              validateTurnstile={validateTurnstile}
              onRequireVerification={handleRequireVerification}
              onSendVerificationCode={handleSendVerificationCode}
              isSendingCode={isSendingCode}
            />
          </>
        ) : (
          <VerificationStep
            email={credentials?.email || ''}
            isSubmitting={isRegistering}
            secondsLeft={secondsLeft}
            isSendingCode={isSendingCode}
            onConfirm={handleConfirmVerification}
            onResendCode={() =>
              handleSendVerificationCode(credentials?.email || '')
            }
            onBack={handleBackToCredentials}
          />
        )}

        <TermsFooter
          variant='sign-up'
          status={status}
          className='text-center'
        />
      </div>
    </AuthLayout>
  )
}
