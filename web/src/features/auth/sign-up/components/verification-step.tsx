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
import { Loader2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp'
import { Button } from '@/components/ui/button'

export interface VerificationStepProps {
  email: string
  isSubmitting: boolean
  secondsLeft: number
  isSendingCode: boolean
  onConfirm: (code: string) => Promise<void>
  onResendCode: () => Promise<boolean>
  onBack: () => void
}

export function VerificationStep(props: VerificationStepProps) {
  const { t } = useTranslation()
  const [code, setCode] = useState('')

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (code.length !== 6 || props.isSubmitting) return
    await props.onConfirm(code)
  }

  const handleResend = async () => {
    if (props.isSubmitting || props.isSendingCode || props.secondsLeft > 0) return
    await props.onResendCode()
  }

  const renderResendContent = () => {
    if (props.isSendingCode) {
      return (
        <span className='inline-flex items-center gap-1.5'>
          <Loader2 className='h-3.5 w-3.5 animate-spin' />
          {t('Sending...')}
        </span>
      )
    }
    if (props.secondsLeft > 0) {
      return t('Resend ({{seconds}}s)', { seconds: props.secondsLeft })
    }
    return t('Resend code')
  }

  return (
    <div className='w-full space-y-8'>
      {/* 标题与说明区域（对标图 2） */}
      <div className='space-y-3 text-center'>
        <h2 className='text-3xl font-bold tracking-tight sm:text-4xl'>
          {t('Check your inbox')}
        </h2>
        <div className='text-muted-foreground text-sm sm:text-base leading-relaxed space-y-1'>
          <p>
            {t('Enter the code we sent to')}{' '}
            <span className='font-semibold text-foreground break-all'>
              {props.email}
            </span>
            .
          </p>
          <p>{t("Can't find it? Check your spam folder.")}</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className='space-y-6'>
        {/* 6位验证码输入框：紫色外发光与居中数字槽（对标图 2） */}
        <div className='flex justify-center'>
          <InputOTP
            maxLength={6}
            value={code}
            onChange={setCode}
            autoFocus
            containerClassName='w-full justify-center'
          >
            <InputOTPGroup className='w-full max-w-sm h-14 sm:h-16 rounded-2xl border-2 border-primary/80 bg-input/20 focus-within:ring-2 focus-within:ring-primary/40 focus-within:border-primary transition-all flex items-center justify-center gap-1 sm:gap-2 px-3 shadow-inner'>
              {[0, 1, 2, 3, 4, 5].map((index) => (
                <InputOTPSlot
                  key={index}
                  index={index}
                  className='size-9 sm:size-10 border-0 text-xl sm:text-2xl font-mono font-medium focus:ring-0 shadow-none'
                />
              ))}
            </InputOTPGroup>
          </InputOTP>
        </div>

        {/* 提交按钮：Confirm my account（对标图 2） */}
        <Button
          type='submit'
          className='w-full h-12 text-base font-semibold rounded-2xl justify-center gap-2'
          disabled={props.isSubmitting || code.length !== 6}
        >
          {props.isSubmitting ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : null}
          {t('Confirm my account')}
        </Button>

        {/* 辅助操作：返回修改邮箱与重新发送验证码 */}
        <div className='flex items-center justify-between text-sm text-muted-foreground pt-1'>
          <button
            type='button'
            onClick={props.onBack}
            className='hover:text-primary transition-colors underline underline-offset-4 cursor-pointer'
            disabled={props.isSubmitting}
          >
            {t('Change email')}
          </button>

          <button
            type='button'
            onClick={handleResend}
            disabled={props.isSubmitting || props.isSendingCode || props.secondsLeft > 0}
            className='hover:text-primary transition-colors disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer'
          >
            {renderResendContent()}
          </button>
        </div>
      </form>
    </div>
  )
}
