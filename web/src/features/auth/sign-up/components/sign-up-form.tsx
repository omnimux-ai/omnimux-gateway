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
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { login, register, wechatLoginByCode } from '@/features/auth/api'
import { LegalConsent } from '@/features/auth/components/legal-consent'
import { OAuthProviders } from '@/features/auth/components/oauth-providers'
import { registerFormSchema } from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { defaultAuthRedirect } from '@/features/auth/lib/auth-redirect'
import {
  getAffiliateCode,
  saveAffiliateCode,
} from '@/features/auth/lib/storage'
import { generateRandomUsername } from '@/features/auth/sign-up/lib/generate-username'
import { useStatus } from '@/hooks/use-status'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'
import { createServerError } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

export interface SignUpCredentials {
  email: string
  password: string
  username: string
}

export interface SignUpFormProps
  extends Omit<React.HTMLAttributes<HTMLFormElement>, 'onSubmit'> {
  initialValues?: {
    email?: string
    password?: string
  }
  turnstileToken: string
  turnstileSiteKey: string
  isTurnstileEnabled: boolean
  turnstileWidgetKey: number
  setTurnstileToken: (token: string) => void
  validateTurnstile: () => boolean
  onRequireVerification: (credentials: SignUpCredentials) => void
  onSendVerificationCode: (email: string) => Promise<boolean>
  isSendingCode: boolean
}

export function SignUpForm(props: SignUpFormProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [agreedToLegal, setAgreedToLegal] = useState(false)
  const [wechatCode, setWeChatCode] = useState('')
  const [isWeChatDialogOpen, setIsWeChatDialogOpen] = useState(false)
  const [isWeChatSubmitting, setIsWeChatSubmitting] = useState(false)
  const legalConsentErrorMessage = t('Please agree to the legal terms first')

  const { status } = useStatus()
  const { redirectToLogin, handleLoginResult } = useAuthRedirect()

  const form = useForm<z.infer<typeof registerFormSchema>>({
    resolver: zodResolver(registerFormSchema),
    defaultValues: {
      email: props.initialValues?.email || '',
      password: props.initialValues?.password || '',
    },
  })

  const emailVerificationRequired = !!status?.email_verification
  const requiresLegalConsent = Boolean(status?.legal_consent_required)
  const oauthRegisterEnabled =
    status?.oauth_register_enabled ??
    status?.data?.oauth_register_enabled ??
    true
  const hasWeChatLogin = Boolean(status?.wechat_login)
  const turnstileReady = !props.isTurnstileEnabled || Boolean(props.turnstileToken)

  const wechatQrCodeUrl = useMemo(() => {
    return (
      status?.wechat_qrcode ||
      status?.wechat_qr_code ||
      status?.wechat_qrcode_image_url ||
      status?.wechat_qr_code_image_url ||
      status?.wechat_account_qrcode_image_url ||
      status?.WeChatAccountQRCodeImageURL ||
      status?.data?.wechat_qrcode ||
      status?.data?.WeChatAccountQRCodeImageURL ||
      ''
    )
  }, [status])

  useEffect(() => {
    if (requiresLegalConsent) {
      setAgreedToLegal(false)
    } else {
      setAgreedToLegal(true)
    }
  }, [requiresLegalConsent])

  useEffect(() => {
    const aff = new URLSearchParams(window.location.search).get('aff')?.trim()
    if (aff) {
      saveAffiliateCode(aff)
    }
  }, [])

  async function onSubmit(data: z.infer<typeof registerFormSchema>) {
    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(legalConsentErrorMessage)
      return
    }

    if (!props.validateTurnstile()) return

    const randomUsername = generateRandomUsername()

    // 邮箱验证流程：发送验证码并切换至验证码输入页面
    if (emailVerificationRequired) {
      setIsLoading(true)
      try {
        const sent = await props.onSendVerificationCode(data.email)
        if (sent) {
          props.onRequireVerification({
            email: data.email,
            password: data.password,
            username: randomUsername,
          })
        }
      } finally {
        setIsLoading(false)
      }
      return
    }

    // 免邮箱验证流程：直接调用注册
    setIsLoading(true)
    try {
      const res = await register({
        username: randomUsername,
        password: data.password,
        email: data.email || undefined,
        aff_code: getAffiliateCode(),
        turnstile: props.turnstileToken,
      })

      if (res?.success) {
        try {
          const loginRes = await login({
            username: data.email || randomUsername,
            password: data.password,
            turnstile: props.turnstileToken,
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
      setIsLoading(false)
    }
  }

  const handleOpenWeChatDialog = () => {
    if (requiresLegalConsent && !agreedToLegal) {
      toast.error(legalConsentErrorMessage)
      return
    }
    setIsWeChatDialogOpen(true)
  }

  const handleWeChatDialogChange = (open: boolean) => {
    setIsWeChatDialogOpen(open)
    if (!open) {
      setWeChatCode('')
      setIsWeChatSubmitting(false)
    }
  }

  async function handleWeChatLogin() {
    if (!wechatCode.trim()) {
      toast.error(t('Please enter the verification code'))
      return
    }

    setIsWeChatSubmitting(true)
    try {
      const res = await wechatLoginByCode(wechatCode)
      if (res?.success) {
        handleWeChatDialogChange(false)
        if (await handleLoginResult(res.data)) {
          toast.success(t('Signed in via WeChat'))
        }
      } else {
        handleServerError(createServerError(res, t('Login failed')))
      }
    } catch (error: unknown) {
      handleServerError(
        new AuthOperationError(t('Login failed'), undefined, { cause: error })
      )
    } finally {
      setIsWeChatSubmitting(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', props.className)}
      >
        {/* Email Field */}
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Email')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('name@example.com')}
                  type='email'
                  autoComplete='email'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Password Field (单次输入，免去二次重复确认) */}
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Password')}</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder={t('Enter password (8–128 characters)')}
                  autoComplete='new-password'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Turnstile 人机验证 */}
        {props.isTurnstileEnabled && (
          <div className='mt-2'>
            <Turnstile
              key={props.turnstileWidgetKey}
              siteKey={props.turnstileSiteKey}
              onVerify={props.setTurnstileToken}
            />
          </div>
        )}

        <LegalConsent
          status={status}
          checked={agreedToLegal}
          onCheckedChange={setAgreedToLegal}
          className='mt-1'
        />

        {/* Submit Button */}
        <Button
          type='submit'
          className='mt-2 w-full justify-center gap-2'
          disabled={
            isLoading ||
            props.isSendingCode ||
            (requiresLegalConsent && !agreedToLegal) ||
            !turnstileReady
          }
        >
          {isLoading || props.isSendingCode ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : null}
          {t('Create account')}
        </Button>

        {oauthRegisterEnabled && (
          <OAuthProviders
            status={status}
            disabled={isLoading || (requiresLegalConsent && !agreedToLegal)}
            onWeChatLogin={hasWeChatLogin ? handleOpenWeChatDialog : undefined}
            isWeChatLoading={isWeChatSubmitting}
            className='pt-2'
          />
        )}
      </form>

      {hasWeChatLogin && (
        <Dialog
          open={isWeChatDialogOpen}
          onOpenChange={handleWeChatDialogChange}
          title={t('WeChat sign in')}
          description={t(
            'Scan the QR code to follow the official account and reply with “验证码” to receive your verification code.'
          )}
          contentClassName='max-w-sm'
          headerClassName='text-left'
          contentHeight='auto'
          bodyClassName='space-y-4'
          footer={
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => handleWeChatDialogChange(false)}
                disabled={isWeChatSubmitting}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                onClick={handleWeChatLogin}
                disabled={
                  isWeChatSubmitting ||
                  !wechatCode.trim() ||
                  (requiresLegalConsent && !agreedToLegal)
                }
                className='gap-2'
              >
                {isWeChatSubmitting ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : null}
                {t('Confirm')}
              </Button>
            </>
          }
        >
          {wechatQrCodeUrl ? (
            <div className='flex justify-center'>
              <img
                src={wechatQrCodeUrl}
                alt={t('WeChat login QR code')}
                className='h-40 w-40 rounded-md border object-contain'
              />
            </div>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('QR code is not configured. Please contact support.')}
            </p>
          )}
          <div className='grid gap-2'>
            <Label htmlFor='wechat-code'>{t('Verification code')}</Label>
            <Input
              id='wechat-code'
              placeholder={t('Enter the verification code')}
              value={wechatCode}
              onChange={(event) => setWeChatCode(event.target.value)}
              autoComplete='one-time-code'
            />
          </div>
        </Dialog>
      )}
    </Form>
  )
}
