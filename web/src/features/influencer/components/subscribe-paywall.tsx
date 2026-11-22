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
import type { TFunction } from 'i18next'
import { ArrowLeft, Check, CreditCard, Lock } from 'lucide-react'
import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Spinner } from '@/components/ui/spinner'
import {
  formatCredits,
  rawQuotaToUsd,
  usdToCredits,
} from '@/features/pricing/lib/credits'
import { isApiSuccess, requestStripePayment } from '@/features/wallet/api'
import { useTopupInfo } from '@/features/wallet/hooks/use-topup-info'
import { calculatePresetPricing } from '@/features/wallet/lib/format'
import type { PresetAmount } from '@/features/wallet/types'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { getSelf } from '@/lib/api'
import {
  formatCurrencyFromUSD,
  formatLocalCurrencyAmount,
} from '@/lib/currency'
import { cn } from '@/lib/utils'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { toastInfluencerError } from '../lib/error-copy'
import type { PaywallOrigin } from '../lib/insufficient-credits'

const POLL_MS = 3000
const POLL_LIMIT_MS = 10 * 60 * 1000
const RESUME_DELAY_MS = 1400
const PAYWALL_ACTION_CLASS =
  'w-full rounded-full bg-foreground text-background hover:bg-foreground/90'

type Phase = 'choose' | 'awaiting' | 'success'

interface Props {
  open: boolean
  /** Credits this generation needs, or null when the form has no quote (video). */
  requiredCredits: number | null
  origin: PaywallOrigin
  modelName?: string | null
  resumeGeneration?: boolean
  onClose: () => void
  /** Recheck eligibility before showing the receipt. */
  checkResume?: () => Promise<boolean>
  /** Payment is confirmed and the refreshed wallet is in the store. */
  onSubscribed: (canResume: boolean) => void | Promise<void>
}

function formatPayAmount(amount: number): string {
  return formatLocalCurrencyAmount(amount, {
    abbreviate: false,
    digitsLarge: 2,
    digitsSmall: 2,
  })
}

function formatCreditAmount(amountUsd: number): string {
  return formatCurrencyFromUSD(amountUsd, {
    abbreviate: false,
    digitsLarge: 0,
    digitsSmall: 2,
  })
}

interface Rung {
  preset: PresetAmount
  discount: number
  pricing: ReturnType<typeof calculatePresetPricing>
}

export function SubscribePaywall(props: Props) {
  // Mount the dialog body only while open so useTopupInfo's always-on mount
  // fetch stays gated behind `open` (the parent mounts this unconditionally).
  if (!props.open) return null
  return <PaywallDialog {...props} />
}

function PaywallDialog(props: Props) {
  const { t } = useTranslation()
  const quota = useAuthStore((s) => s.auth.user?.quota ?? 0)
  const accountId = useAuthStore((s) => s.auth.user?.id)
  const setAuthUser = useAuthStore((s) => s.auth.setUser)
  const { status } = useStatus()
  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading } = useTopupInfo()

  const quotaRef = useRef(quota)
  const baselineRef = useRef(quota)
  const checkingRef = useRef<Promise<boolean> | null>(null)
  const stripePurchase = useRef<Rung | null>(null)
  const confirmedRef = useRef(false)
  const resumedRef = useRef(false)
  const mountedRef = useRef(false)
  const subscribedRef = useRef(props.onSubscribed)

  const [phase, setPhase] = useState<Phase>('choose')
  const [canResumeAfterPayment, setCanResumeAfterPayment] = useState(true)
  const checkResume = props.checkResume
  const resumeGeneration =
    props.resumeGeneration !== false && canResumeAfterPayment
  const [checkingBalance, setCheckingBalance] = useState(true)
  const [paying, setPaying] = useState(false)
  const [addedCredits, setAddedCredits] = useState<number | null>(null)
  const [selectedUsd, setSelectedUsd] = useState<number | null>(null)

  const balanceCredits = useMemo(
    () => formatCredits(usdToCredits(rawQuotaToUsd(quota))),
    [quota],
  )
  const requiredLabel = useMemo(
    () =>
      props.requiredCredits !== null
        ? formatCredits(props.requiredCredits)
        : null,
    [props.requiredCredits],
  )
  const isVideo = props.origin === 'video'

  const priceRatio =
    typeof status?.price === 'number' && status.price > 0 ? status.price : 1
  const usdExchangeRate = useMemo(() => {
    const displayType = currency?.quotaDisplayType
    if (displayType === 'USD') return 1
    if (displayType === 'CUSTOM') {
      const rate = currency?.customCurrencyExchangeRate
      return rate && rate > 0 ? rate : 1
    }
    return currency?.usdExchangeRate && currency.usdExchangeRate > 0
      ? currency.usdExchangeRate
      : 1
  }, [
    currency?.quotaDisplayType,
    currency?.usdExchangeRate,
    currency?.customCurrencyExchangeRate,
  ])

  const rungs = useMemo<Rung[]>(() => {
    if (!presetAmounts || presetAmounts.length === 0) return []
    return presetAmounts.map((preset) => {
      const discount =
        preset.discount || topupInfo?.discount?.[preset.value] || 1.0
      const pricing = calculatePresetPricing(
        preset.value,
        priceRatio,
        discount,
        usdExchangeRate,
      )
      return { preset, discount, pricing }
    })
  }, [presetAmounts, topupInfo?.discount, priceRatio, usdExchangeRate])

  const stripeEnabled = !!topupInfo?.enable_stripe_topup

  // Default-select the smallest rung that covers the credit gap; fall back to
  // the smallest rung when nothing covers it or required is unknown.
  useEffect(() => {
    if (rungs.length === 0 || selectedUsd !== null) return
    const needCredits = props.requiredCredits ?? 0
    const covering = rungs.find(
      (r) => usdToCredits(r.preset.value) >= needCredits,
    )
    setSelectedUsd((covering ?? rungs[0]).preset.value)
  }, [rungs, props.requiredCredits, selectedUsd])

  useEffect(() => {
    quotaRef.current = quota
  }, [quota])

  useEffect(() => {
    subscribedRef.current = props.onSubscribed
  }, [props.onSubscribed])

  const refreshQuota = useCallback(async () => {
    try {
      const response: { success?: boolean; data?: AuthUser } = await getSelf()
      const user = response?.data
      if (
        mountedRef.current &&
        response?.success === true &&
        typeof accountId === 'number' &&
        user?.id === accountId &&
        useAuthStore.getState().auth.user?.id === accountId &&
        typeof user?.quota === 'number' &&
        Number.isFinite(user.quota)
      ) {
        setAuthUser(user)
        quotaRef.current = user.quota
        return user.quota
      }
    } catch {
      // Polling is best-effort.
    }
    return null
  }, [accountId, setAuthUser])

  useEffect(() => {
    let active = true
    mountedRef.current = true
    void refreshQuota().then((next) => {
      if (!active) return
      baselineRef.current = next ?? quotaRef.current
      setCheckingBalance(false)
    })
    return () => {
      active = false
      mountedRef.current = false
    }
  }, [refreshQuota])

  const resume = useCallback(() => {
    if (resumedRef.current) return
    resumedRef.current = true
    void subscribedRef.current(resumeGeneration)
  }, [resumeGeneration])

  useEffect(() => {
    if (phase !== 'success') return
    const timer = window.setTimeout(resume, RESUME_DELAY_MS)
    return () => window.clearTimeout(timer)
  }, [phase, resume])

  const detectPayment = useCallback(() => {
    if (confirmedRef.current) return Promise.resolve(true)
    if (checkingRef.current) return checkingRef.current
    const check = refreshQuota()
      .then(async (next) => {
        if (next === null || next <= baselineRef.current) return false
        const canResume = await checkResume?.()
        if (
          !mountedRef.current ||
          useAuthStore.getState().auth.user?.id !== accountId
        ) {
          return false
        }
        confirmedRef.current = true
        setCanResumeAfterPayment(canResume !== false)
        setAddedCredits(usdToCredits(rawQuotaToUsd(next - baselineRef.current)))
        setPhase('success')
        return true
      })
      .finally(() => {
        checkingRef.current = null
      })
    checkingRef.current = check
    return check
  }, [accountId, checkResume, refreshQuota])

  // Poll balance growth while the Stripe tab is open.
  useEffect(() => {
    if (phase !== 'awaiting') return
    const startedAt = Date.now()
    let intervalId: number | null = null
    const stop = () => {
      if (intervalId !== null) window.clearInterval(intervalId)
      intervalId = null
      document.removeEventListener('visibilitychange', onVisible)
      window.removeEventListener('focus', onFocus)
    }
    const tick = async () => {
      if (Date.now() - startedAt > POLL_LIMIT_MS) {
        stop()
        return
      }
      const paid = await detectPayment()
      if (paid) stop()
    }
    const onVisible = () => {
      if (document.visibilityState === 'visible') void tick()
    }
    const onFocus = () => void tick()
    intervalId = window.setInterval(() => void tick(), POLL_MS)
    document.addEventListener('visibilitychange', onVisible)
    window.addEventListener('focus', onFocus)
    void tick()
    return stop
  }, [phase, detectPayment])

  const selected = rungs.find((r) => r.preset.value === selectedUsd) ?? null
  const startPayment = useCallback(async () => {
    if (!selected || paying || checkingBalance) return
    setPaying(true)
    try {
      const response = await requestStripePayment({
        amount: Math.floor(selected.preset.value),
        payment_method: 'stripe',
      })
      const payLink = isApiSuccess(response)
        ? response.data?.pay_link
        : undefined
      if (
        !mountedRef.current ||
        useAuthStore.getState().auth.user?.id !== accountId
      ) {
        return
      }
      if (payLink) {
        baselineRef.current = quotaRef.current
        stripePurchase.current = selected
        window.open(payLink, '_blank', 'noopener')
        setPhase('awaiting')
        toast.success(t('Payment page opened'))
      } else {
        toastInfluencerError(t, undefined, 'Payment request failed')
      }
    } catch (error) {
      toastInfluencerError(t, error, 'Payment request failed')
    } finally {
      setPaying(false)
    }
  }, [accountId, checkingBalance, selected, paying, t])

  const openWalletTopUp = useCallback(() => {
    if (checkingBalance) return
    if (phase === 'choose') baselineRef.current = quotaRef.current
    window.open('/wallet', '_blank', 'noopener')
    setPhase('awaiting')
  }, [checkingBalance, phase])

  const checkManually = useCallback(async () => {
    const paid = await detectPayment()
    if (!paid && mountedRef.current) toast.info(t('Payment not completed yet'))
  }, [detectPayment, t])

  const showFallback = !loading && (!stripeEnabled || rungs.length === 0)
  const payLabel = selected
    ? t('Pay {{price}} with Stripe', {
        price: formatPayAmount(selected.pricing.actualPrice),
      })
    : t('Pay with Stripe')

  return (
    <Dialog
      open
      disablePointerDismissal
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <DialogContent
        showCloseButton={false}
        className='border-border bg-card text-foreground max-h-[92vh] gap-0 rounded-3xl border p-0 shadow-2xl ring-0 sm:max-w-3xl'
      >
        <Header
          t={t}
          phase={phase}
          resumeGeneration={resumeGeneration}
          onClose={props.onClose}
        />

        {phase === 'success' ? (
          <SuccessBody
            t={t}
            selected={stripePurchase.current}
            addedCredits={addedCredits}
            resumeGeneration={resumeGeneration}
            onDone={resume}
          />
        ) : (
          <ChooseBody
            t={t}
            resumeGeneration={resumeGeneration}
            checkingBalance={checkingBalance}
            balanceCredits={balanceCredits}
            requiredLabel={requiredLabel}
            modelName={props.modelName ?? null}
            isVideo={isVideo}
            loading={loading}
            showFallback={showFallback}
            rungs={rungs}
            selectedUsd={selectedUsd}
            onSelectUsd={setSelectedUsd}
            paying={paying}
            awaiting={phase === 'awaiting'}
            payLabel={payLabel}
            canPay={!!selected && !paying && !checkingBalance}
            onPay={() => void startPayment()}
            onWalletTopUp={openWalletTopUp}
            onCheckManually={() => void checkManually()}
            onClose={props.onClose}
          />
        )}

        {!loading && !showFallback ? (
          <div className='border-border text-muted-foreground border-t px-6 py-3.5 text-center text-[11px]'>
            {t('Payment is processed by Stripe.')}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function Header({
  t,
  phase,
  resumeGeneration,
  onClose,
}: {
  t: TFunction
  phase: Phase
  resumeGeneration: boolean
  onClose: () => void
}) {
  return (
    <div className='flex items-center gap-3 border-b border-border px-5 py-3.5 sm:px-6'>
      <div className='flex items-center gap-2 font-semibold tracking-tight'>
        <span className='grid h-5 w-5 place-items-center rounded-full bg-foreground text-[10px] font-bold text-background'>
          O
        </span>
        OmniMux
      </div>
      <span className='rounded-full border border-border px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground'>
        {t('Top up')}
      </span>
      <ol className='ml-auto hidden items-center gap-4 sm:flex'>
        <Step index={1} label={t('Choose an amount')} active={phase === 'choose'} />
        <Step index={2} label={t('Payment')} active={phase === 'awaiting'} />
        <Step
          index={3}
          label={resumeGeneration ? t('Continue generating') : t('Done')}
          active={phase === 'success'}
        />
      </ol>
      <button
        type='button'
        onClick={onClose}
        className='ml-auto inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:ml-4'
      >
        <ArrowLeft className='h-3.5 w-3.5' />
        {t('Back to editor')}
      </button>
    </div>
  )
}

type PaywallBodyProps = {
  t: TFunction
  resumeGeneration: boolean
  checkingBalance: boolean
  balanceCredits: string
  requiredLabel: string | null
  modelName: string | null
  isVideo: boolean
  loading: boolean
  showFallback: boolean
  rungs: Rung[]
  selectedUsd: number | null
  onSelectUsd: (v: number) => void
  paying: boolean
  awaiting: boolean
  payLabel: string
  canPay: boolean
  onPay: () => void
  onWalletTopUp: () => void
  onCheckManually: () => void
  onClose: () => void
}

function ChooseBody(props: PaywallBodyProps) {
  const { t } = props
  return (
    <div className='px-6 pt-6 pb-6 sm:px-8'>
      {props.resumeGeneration ? (
        <p className='text-muted-foreground text-[11px] font-semibold tracking-[0.14em] uppercase'>
          {t('Continue generating')}
        </p>
      ) : null}
      <DialogTitle className='mt-2 text-xl leading-snug font-semibold tracking-tight sm:text-2xl'>
        {props.resumeGeneration
          ? t('Top up to continue this generation')
          : t('Top up')}
      </DialogTitle>

      <div className='mt-4 flex flex-wrap items-center gap-2'>
        <span className='inline-flex items-center gap-1.5 rounded-full border border-border bg-card px-3 py-1 text-xs'>
          {t('Balance')}
          <b className='tabular-nums'>{props.balanceCredits}</b>
        </span>
        {props.requiredLabel ? (
          <span className='inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200'>
            {t('Credits needed')}
            <b className='tabular-nums'>{props.requiredLabel}</b>
          </span>
        ) : null}
      </div>

      {props.modelName ? (
        <p className='mt-3 text-xs text-muted-foreground'>
          {t('Model')}{' '}
          <b className='font-semibold text-foreground'>{props.modelName}</b>
          {' · '}
          {props.isVideo ? t('Video') : t('Image')}
        </p>
      ) : null}

      <div className='mt-6 border-t border-border pt-5'>
        {renderPaywallBody(props, t)}
      </div>
    </div>
  )
}

function renderPaywallBody(props: PaywallBodyProps, t: TFunction) {
  if (props.loading) {
    return (
      <div className='flex items-center justify-center py-14'>
        <Spinner className='h-6 w-6' />
      </div>
    )
  }
  if (props.showFallback) {
    return (
      <FallbackBody
        t={t}
        checkingBalance={props.checkingBalance}
        onWalletTopUp={props.onWalletTopUp}
        onCheckManually={props.onCheckManually}
        onClose={props.onClose}
      />
    )
  }
  return (
    <LadderBody
      t={t}
      rungs={props.rungs}
      selectedUsd={props.selectedUsd}
      onSelectUsd={props.onSelectUsd}
      paying={props.paying}
      awaiting={props.awaiting}
      payLabel={props.payLabel}
      canPay={props.canPay}
      onPay={props.onPay}
      onCheckManually={props.onCheckManually}
      onClose={props.onClose}
    />
  )
}

function FallbackBody({
  t,
  checkingBalance,
  onWalletTopUp,
  onCheckManually,
  onClose,
}: {
  t: TFunction
  checkingBalance: boolean
  onWalletTopUp: () => void
  onCheckManually: () => void
  onClose: () => void
}) {
  return (
    <div>
      <h3 className='text-base font-semibold'>{t('Top up in Wallet')}</h3>
      <p className='mt-1.5 text-sm text-muted-foreground'>
        {t('Add credits in Wallet and come back to continue.')}
      </p>
      <Button
        type='button'
        disabled={checkingBalance}
        onClick={onWalletTopUp}
        className={cn(PAYWALL_ACTION_CLASS, 'mt-5')}
      >
        {t('Add credits in Wallet')}
      </Button>
      <button
        type='button'
        disabled={checkingBalance}
        onClick={onCheckManually}
        className='mt-2 w-full rounded-full px-3 py-2 text-xs text-muted-foreground underline underline-offset-4 hover:text-foreground'
      >
        {t('I have completed the payment')}
      </button>
      <button
        type='button'
        onClick={onClose}
        className='mt-1 w-full rounded-full px-3 py-2 text-xs text-muted-foreground underline underline-offset-4 hover:text-foreground'
      >
        {t('Not now, back to editor')}
      </button>
    </div>
  )
}

function LadderBody({
  t,
  rungs,
  selectedUsd,
  onSelectUsd,
  paying,
  awaiting,
  payLabel,
  canPay,
  onPay,
  onCheckManually,
  onClose,
}: {
  t: TFunction
  rungs: Rung[]
  selectedUsd: number | null
  onSelectUsd: (v: number) => void
  paying: boolean
  awaiting: boolean
  payLabel: string
  canPay: boolean
  onPay: () => void
  onCheckManually: () => void
  onClose: () => void
}) {
  return (
    <div>
      <p className='text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground'>
        {t('Select an amount')}
      </p>
      <div className='mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3'>
        {rungs.map(({ preset, discount, pricing }) => {
          const isSelected = preset.value === selectedUsd
          const offPercent = pricing.hasDiscount
            ? Math.round((1 - discount) * 100)
            : 0
          return (
            <button
              key={preset.value}
              type='button'
              aria-pressed={isSelected}
              aria-label={
                pricing.hasDiscount
                  ? t('{{price}} top-up, get {{credits}}, save {{saved}}', {
                      price: formatPayAmount(pricing.actualPrice),
                      credits: formatCreditAmount(preset.value),
                      saved: formatPayAmount(pricing.savedAmount),
                    })
                  : t('{{price}} top-up, get {{credits}}', {
                      price: formatPayAmount(pricing.actualPrice),
                      credits: formatCreditAmount(preset.value),
                    })
              }
              onClick={() => onSelectUsd(preset.value)}
              className={cn(
                'relative flex min-h-[4.5rem] flex-col items-start rounded-lg border px-3 py-2.5 text-left transition-colors',
                isSelected
                  ? 'border-foreground bg-foreground/5 ring-1 ring-foreground/20'
                  : 'border-border hover:border-foreground/40',
              )}
            >
              {isSelected ? (
                <span className='absolute right-2 top-2 grid h-3.5 w-3.5 place-items-center rounded-full bg-foreground text-background'>
                  <Check className='h-2.5 w-2.5' />
                </span>
              ) : null}
              <span className='flex items-baseline gap-1'>
                <span className='text-base font-semibold tabular-nums sm:text-lg'>
                  {formatPayAmount(pricing.actualPrice)}
                </span>
                {pricing.hasDiscount ? (
                  <span className='text-[10px] font-semibold text-emerald-600'>
                    {t('{{off}}% OFF', { off: offPercent })}
                  </span>
                ) : null}
              </span>
              <span className='mt-1.5 text-xs text-muted-foreground'>
                {t('Get {{amount}}', {
                  amount: formatCreditAmount(preset.value),
                })}
                {pricing.hasDiscount ? (
                  <em className='text-emerald-600 not-italic'>
                    {' · '}
                    {t('Save {{amount}}', {
                      amount: formatPayAmount(pricing.savedAmount),
                    })}
                  </em>
                ) : null}
              </span>
            </button>
          )
        })}
      </div>

      <div className='mt-5 flex items-center gap-3 text-xs text-muted-foreground'>
        <CreditCard className='h-4 w-4' />
        <span>{t('Payment method')}</span>
        <b className='font-semibold text-foreground'>Stripe</b>
        <span className='h-px flex-1 bg-border' />
        <Lock className='h-3.5 w-3.5' />
        <span>{t('Secure payment')}</span>
      </div>

      <Button
        type='button'
        disabled={!canPay || awaiting}
        aria-busy={paying}
        onClick={onPay}
        className={cn(PAYWALL_ACTION_CLASS, 'mt-4')}
      >
        {paying ? <Spinner className='mr-2 h-4 w-4' /> : null}
        {paying ? t('Opening Stripe…') : payLabel}
      </Button>

      {awaiting ? (
        <div className='mt-4 flex items-start gap-3 rounded-xl border border-dashed border-border p-3.5 text-sm text-muted-foreground'>
          <Spinner className='mt-0.5 h-4 w-4 flex-none' />
          <div>
            {t(
              'Complete the payment in the new tab. This page continues automatically.',
            )}
            <button
              type='button'
              onClick={onCheckManually}
              className='ml-1.5 font-semibold text-foreground underline underline-offset-4'
            >
              {t('I have completed the payment')}
            </button>
          </div>
        </div>
      ) : (
        <button
          type='button'
          onClick={onClose}
          className='mt-3 w-full rounded-full px-3 py-2 text-xs text-muted-foreground underline underline-offset-4 hover:text-foreground'
        >
          {t('Not now, back to editor')}
        </button>
      )}
    </div>
  )
}

function SuccessBody({
  t,
  selected,
  addedCredits,
  resumeGeneration,
  onDone,
}: {
  t: TFunction
  selected: Rung | null
  addedCredits: number | null
  resumeGeneration: boolean
  onDone: () => void
}) {
  return (
    <div className='px-6 py-10 text-center sm:px-10'>
      <div className='mx-auto grid h-12 w-12 place-items-center rounded-full bg-emerald-600 text-white'>
        <Check className='h-6 w-6' />
      </div>
      <DialogTitle className='mt-5 text-xl font-semibold tracking-tight'>
        {t('Top up successful')}
      </DialogTitle>
      {resumeGeneration ? (
        <p className='text-muted-foreground mx-auto mt-2 max-w-md text-sm'>
          {t('Your credits are ready — generation resumes automatically.')}
        </p>
      ) : null}
      <dl className='mx-auto mt-6 grid max-w-xs gap-2 text-left text-sm'>
        {selected ? (
          <ReceiptRow
            label={t('Top up')}
            value={formatPayAmount(selected.pricing.actualPrice)}
          />
        ) : null}
        <ReceiptRow
          label={t('Credits added')}
          value={
            addedCredits !== null ? `+${formatCredits(addedCredits)}` : '—'
          }
        />
        {selected ? (
          <ReceiptRow label={t('Payment method')} value='Stripe' />
        ) : null}
      </dl>
      <Button
        type='button'
        onClick={onDone}
        className={cn(PAYWALL_ACTION_CLASS, 'mt-7 max-w-xs')}
      >
        {resumeGeneration ? t('Continue generating') : t('Done')}
      </Button>
    </div>
  )
}

function Step({
  index,
  label,
  active,
}: {
  index: number
  label: string
  active: boolean
}) {
  return (
    <li
      className={cn(
        'flex items-center gap-1.5 text-[11px]',
        active ? 'text-foreground' : 'text-muted-foreground',
      )}
    >
      <span
        className={cn(
          'grid h-4 w-4 place-items-center rounded-full border text-[9px] font-semibold',
          active
            ? 'border-foreground bg-foreground text-background'
            : 'border-border',
        )}
      >
        {index}
      </span>
      {label}
    </li>
  )
}

function ReceiptRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className='flex items-center justify-between gap-4'>
      <dt className='text-muted-foreground'>{label}</dt>
      <dd className='font-medium tabular-nums'>{value}</dd>
    </div>
  )
}
