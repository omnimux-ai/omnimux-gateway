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
import { Spinner } from '@/components/ui/spinner'
import {
  formatCredits,
  rawQuotaToUsd,
  usdToCredits,
} from '@/features/pricing/lib/credits'
import {
  getPublicPlans,
  paySubscriptionStripe,
} from '@/features/subscriptions/api'
import { formatDuration } from '@/features/subscriptions/lib'
import { planTierDisplayName } from '@/features/subscriptions/lib/plan-tier'
import type {
  PlanRecord,
  SubscriptionPlan,
} from '@/features/subscriptions/types'
import { getSelf } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { toastInfluencerError } from '../lib/error-copy'
import type { PaywallOrigin } from '../lib/insufficient-credits'

const POLL_MS = 3000
const POLL_LIMIT_MS = 10 * 60 * 1000
const RESUME_DELAY_MS = 1400

type Phase = 'choose' | 'awaiting' | 'success'

interface Props {
  open: boolean
  /** Credits this generation needs, or null when the form has no quote (video). */
  requiredCredits: number | null
  origin: PaywallOrigin
  modelName?: string | null
  onClose: () => void
  /** Payment is confirmed: the caller refreshes the wallet and resumes. */
  onSubscribed: () => void | Promise<void>
}

function isPurchasable(record: PlanRecord): boolean {
  const plan = record?.plan
  return (
    Boolean(plan) && plan.enabled !== false && Boolean(plan.stripe_price_id)
  )
}

function planCreditsOf(plan: SubscriptionPlan): number {
  return usdToCredits(rawQuotaToUsd(Number(plan.total_amount || 0)))
}

function formatPrice(plan: SubscriptionPlan): string {
  return `$${Number(plan.price_amount || 0).toFixed(2)}`
}

/** Billing cadence, without repeating the reset period next to it. */
function planPeriodLabel(plan: SubscriptionPlan, t: TFunction): string {
  const unit = plan.duration_unit
  const value = Number(plan.duration_value || 1)
  if (value === 1 && unit === 'month') return t('Monthly')
  if (value === 1 && unit === 'year') return t('Annual')
  return formatDuration(plan, t)
}

/**
 * The plan worth showing first: a monthly plan with the best credits per unit
 * of price (annual records only when no monthly one exists). The database
 * title stays untouched; only the preselection is a product decision.
 */
function defaultPlanId(plans: PlanRecord[]): number | null {
  const monthly = plans.filter(
    (record) =>
      record.plan.duration_unit === 'month' &&
      Number(record.plan.duration_value || 1) === 1
  )
  const pool = monthly.length > 0 ? monthly : plans
  let best: PlanRecord | null = null
  let bestRatio = -1
  for (const record of pool) {
    const price = Number(record.plan.price_amount || 0)
    const credits = planCreditsOf(record.plan)
    if (price <= 0 || credits <= 0) continue
    const ratio = credits / price
    if (ratio > bestRatio) {
      bestRatio = ratio
      best = record
    }
  }
  return (best ?? plans[0])?.plan.id ?? null
}

function PlanRow(props: {
  plan: SubscriptionPlan
  selected: boolean
  onSelect: () => void
}) {
  const { t } = useTranslation()
  const plan = props.plan
  return (
    <button
      type='button'
      role='radio'
      aria-checked={props.selected}
      onClick={props.onSelect}
      className={cn(
        'flex w-full items-center gap-3 rounded-2xl border px-4 py-3.5 text-left transition-colors sm:gap-4 sm:px-5 sm:py-4',
        props.selected
          ? 'border-primary bg-primary/10'
          : 'border-border bg-card/40 hover:bg-card/70'
      )}
    >
      <span
        aria-hidden='true'
        className={cn(
          'relative size-4 shrink-0 rounded-full border',
          props.selected ? 'border-primary' : 'border-muted-foreground/50'
        )}
      >
        {props.selected ? (
          <span className='bg-primary absolute inset-1 rounded-full' />
        ) : null}
      </span>
      <span className='flex min-w-0 flex-1 flex-col gap-1'>
        <span className='text-sm font-semibold'>
          {planTierDisplayName(plan, t, plan.title)}
        </span>
        <span className='text-muted-foreground text-xs'>
          {formatCredits(planCreditsOf(plan))} {t('credits')} ·{' '}
          {planPeriodLabel(plan, t)}
        </span>
      </span>
      <span className='shrink-0 text-base font-semibold tabular-nums'>
        {formatPrice(plan)}
      </span>
    </button>
  )
}

/**
 * Full-screen subscription takeover shown when the account cannot pay for the
 * current generation. It is intentionally not the shared `Dialog`: a checkout
 * needs the whole viewport, its own step header and a receipt state, which the
 * centered dialog shell cannot express. Stripe is the only payment method the
 * site sells today, so it is stated rather than offered as a choice.
 */
export function SubscribePaywall(props: Props) {
  const { t } = useTranslation()
  const quota = useAuthStore((state) => state.auth.user?.quota ?? 0)
  const setAuthUser = useAuthStore((state) => state.auth.setUser)
  const quotaRef = useRef(quota)
  const panelRef = useRef<HTMLDivElement | null>(null)
  const baselineRef = useRef(0)
  const settledRef = useRef(false)
  const subscribedRef = useRef(props.onSubscribed)
  const [plans, setPlans] = useState<PlanRecord[] | null>(null)
  const [plansFailed, setPlansFailed] = useState(false)
  const [planId, setPlanId] = useState<number | null>(null)
  const [phase, setPhase] = useState<Phase>('choose')
  const [paying, setPaying] = useState(false)
  const [addedCredits, setAddedCredits] = useState(0)

  const balanceCredits = formatCredits(usdToCredits(rawQuotaToUsd(quota)))
  const requiredLabel =
    props.requiredCredits === null ? null : formatCredits(props.requiredCredits)
  const isVideo = props.origin === 'video'
  const requestClose = props.onClose

  useEffect(() => {
    quotaRef.current = quota
  }, [quota])

  useEffect(() => {
    subscribedRef.current = props.onSubscribed
  }, [props.onSubscribed])

  const refreshQuota = useCallback(async (): Promise<number> => {
    try {
      const res = (await getSelf()) as { success?: boolean; data?: AuthUser }
      if (res?.success && res.data) {
        setAuthUser(res.data)
        return res.data.quota ?? 0
      }
    } catch {
      // A failed refresh keeps the last known balance; polling retries it.
    }
    return quotaRef.current
  }, [setAuthUser])

  // Opening resets the flow and pins the balance the payment is measured against.
  useEffect(() => {
    if (!props.open) return
    setPhase('choose')
    setPaying(false)
    setAddedCredits(0)
    setPlans(null)
    setPlansFailed(false)
    settledRef.current = false
    baselineRef.current = quotaRef.current
    void refreshQuota().then((next) => {
      baselineRef.current = Math.max(baselineRef.current, next)
    })
  }, [props.open, refreshQuota])

  useEffect(() => {
    if (!props.open) return
    let cancelled = false
    void (async () => {
      try {
        const res = await getPublicPlans()
        if (cancelled) return
        const available = Array.isArray(res.data) ? res.data : []
        const purchasable = available.filter(isPurchasable)
        setPlans(purchasable)
        setPlanId(defaultPlanId(purchasable))
      } catch (error) {
        if (cancelled) return
        setPlansFailed(true)
        setPlans([])
        toastInfluencerError(t, error, 'Something went wrong!')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [props.open, t])

  useEffect(() => {
    if (!props.open) return
    panelRef.current?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') requestClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [props.open, requestClose])

  // The caller resumes exactly once, whether by the timer or the button.
  useEffect(() => {
    if (phase !== 'success' || settledRef.current) return
    settledRef.current = true
    const timer = setTimeout(() => {
      void subscribedRef.current()
    }, RESUME_DELAY_MS)
    return () => clearTimeout(timer)
  }, [phase])

  const selectedPlan = useMemo(
    () => plans?.find((record) => record.plan.id === planId)?.plan ?? null,
    [plans, planId]
  )

  const detectPayment = useCallback(async (): Promise<boolean> => {
    const next = await refreshQuota()
    if (next <= baselineRef.current) return false
    setAddedCredits(usdToCredits(rawQuotaToUsd(next - baselineRef.current)))
    setPhase('success')
    return true
  }, [refreshQuota])

  // Poll while Stripe's hosted checkout (or the Wallet top-up page) is open.
  useEffect(() => {
    if (!props.open || phase !== 'awaiting') return
    const startedAt = Date.now()
    let stopped = false
    let timer: ReturnType<typeof setInterval> | null = null
    const check = async () => {
      if (stopped) return
      const paid = await detectPayment()
      if (paid) stopped = true
    }
    timer = setInterval(() => {
      if (Date.now() - startedAt > POLL_LIMIT_MS) {
        if (timer) clearInterval(timer)
        return
      }
      if (document.visibilityState === 'visible') void check()
    }, POLL_MS)
    const onReturn = () => {
      if (document.visibilityState === 'visible') void check()
    }
    window.addEventListener('focus', onReturn)
    document.addEventListener('visibilitychange', onReturn)
    return () => {
      stopped = true
      if (timer) clearInterval(timer)
      window.removeEventListener('focus', onReturn)
      document.removeEventListener('visibilitychange', onReturn)
    }
  }, [props.open, phase, detectPayment])

  const startPayment = useCallback(async () => {
    if (!selectedPlan || paying) return
    setPaying(true)
    try {
      const res = await paySubscriptionStripe({ plan_id: selectedPlan.id })
      if (res.message === 'success' && res.data?.pay_link) {
        window.open(res.data.pay_link, '_blank', 'noopener')
        setPhase('awaiting')
        toast.success(t('Payment page opened'))
      } else {
        toastInfluencerError(t, res, 'Payment request failed')
      }
    } catch (error) {
      toastInfluencerError(t, error, 'Payment request failed')
    } finally {
      setPaying(false)
    }
  }, [selectedPlan, paying, t])

  const openWalletTopUp = useCallback(() => {
    window.open('/wallet', '_blank', 'noopener')
    setPhase('awaiting')
  }, [])

  const checkManually = useCallback(async () => {
    const paid = await detectPayment()
    if (!paid) toast.info(t('Payment not completed yet'))
  }, [detectPayment, t])

  if (!props.open) return null

  const showFallback = plans !== null && plans.length === 0
  let payLabel = t('Choose a plan')
  if (phase === 'awaiting') {
    payLabel = t('Waiting for payment…')
  } else if (selectedPlan) {
    payLabel = t('Pay {{price}} · Subscribe to {{plan}}', {
      price: formatPrice(selectedPlan),
      plan: planTierDisplayName(selectedPlan, t, selectedPlan.title),
    })
  }

  return (
    <div
      ref={panelRef}
      tabIndex={-1}
      className='bg-background text-foreground fixed inset-0 z-[100] flex flex-col overflow-y-auto outline-none'
      role='dialog'
      aria-modal='true'
      aria-labelledby='subscribe-paywall-title'
    >
      <header className='border-border/70 bg-background/90 sticky top-0 z-10 flex h-14 shrink-0 items-center gap-3 border-b px-4 backdrop-blur sm:h-16 sm:gap-4 sm:px-7'>
        <span className='text-base font-bold tracking-tight sm:text-lg'>
          OmniMux
        </span>
        <span className='border-border text-muted-foreground hidden border-l pl-4 text-xs sm:inline'>
          {t('Subscription')}
        </span>
        <ol className='text-muted-foreground ml-2 hidden items-center gap-4 text-[11px] md:flex'>
          <Step
            index={1}
            active={phase === 'choose'}
            label={t('Choose a plan')}
          />
          <Step index={2} active={phase === 'awaiting'} label={t('Payment')} />
          <Step
            index={3}
            active={phase === 'success'}
            label={t('Continue generating')}
          />
        </ol>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='ml-auto rounded-full'
          onClick={props.onClose}
        >
          <ArrowLeft className='size-4' aria-hidden='true' />
          {t('Back to editor')}
        </Button>
      </header>

      <div className='flex flex-1 items-center justify-center px-4 py-8 sm:px-7 sm:py-12'>
        {phase === 'success' ? (
          <div className='flex w-full max-w-md flex-col items-center text-center'>
            <div className='bg-primary/15 text-primary flex size-16 items-center justify-center rounded-full'>
              <Check className='size-7' aria-hidden='true' />
            </div>
            <h1
              id='subscribe-paywall-title'
              className='mt-4 text-2xl font-semibold tracking-tight sm:text-3xl'
            >
              {t('Subscription active')}
            </h1>
            <p className='text-muted-foreground mt-3 text-sm leading-relaxed'>
              {t(
                '{{credits}} credits were added to your balance. Resuming this generation…',
                { credits: formatCredits(addedCredits) }
              )}
            </p>
            <dl className='border-border bg-card/40 mt-6 w-full rounded-2xl border px-5 py-1 text-left'>
              <ReceiptRow label={t('Plan')}>
                {selectedPlan
                  ? planTierDisplayName(selectedPlan, t, selectedPlan.title)
                  : t('Subscription')}
              </ReceiptRow>
              <ReceiptRow label={t('Credits added')}>
                {formatCredits(addedCredits)}
              </ReceiptRow>
              <ReceiptRow label={t('Payment method')} last>
                Stripe
              </ReceiptRow>
            </dl>
            <Button
              type='button'
              className='bg-foreground text-background hover:bg-foreground/90 mt-6 h-12 w-full max-w-xs rounded-full'
              onClick={() => void subscribedRef.current()}
            >
              {t('Continue generating')}
            </Button>
          </div>
        ) : (
          <div className='grid w-full max-w-5xl items-start gap-8 md:grid-cols-2 md:gap-12 md:pt-4'>
            <section className='order-2 md:order-1'>
              <span className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
                {t('Continue generating')}
              </span>
              <h1
                id='subscribe-paywall-title'
                className='mt-2 text-3xl leading-tight font-semibold tracking-tight sm:text-4xl'
              >
                {t('Subscribe to continue this generation')}
              </h1>
              <p className='text-muted-foreground mt-4 text-sm leading-relaxed'>
                {props.requiredCredits === null
                  ? t(
                      'Your balance is {{balance}} credits. Subscribe and this generation continues automatically.',
                      { balance: balanceCredits }
                    )
                  : t(
                      'This generation needs {{required}} credits and your balance is {{balance}}. Subscribe and this generation continues automatically.',
                      { required: requiredLabel, balance: balanceCredits }
                    )}
              </p>
              <div className='border-border bg-card/40 mt-6 flex items-center justify-between gap-4 rounded-xl border px-4 py-3 text-xs'>
                <span className='text-muted-foreground'>
                  {props.requiredCredits === null
                    ? t('Balance')
                    : t('Credits needed')}
                </span>
                <span className='flex items-center gap-3 font-semibold tabular-nums'>
                  {requiredLabel ? (
                    <span className='text-muted-foreground decoration-destructive/60 line-through'>
                      {requiredLabel}
                    </span>
                  ) : null}
                  <span>{balanceCredits}</span>
                  <span className='text-muted-foreground font-normal'>
                    {t('credits')}
                  </span>
                </span>
              </div>
              {props.modelName ? (
                <p className='text-muted-foreground mt-3 text-xs'>
                  {t('Model')}:{' '}
                  <span className='text-foreground'>{props.modelName}</span>
                  {isVideo ? ` · ${t('Video')}` : ''}
                </p>
              ) : null}
              <ul className='text-muted-foreground mt-6 space-y-2.5 text-xs'>
                <Benefit>
                  {t(
                    'Credits are added immediately and usable for this generation'
                  )}
                </Benefit>
                <Benefit>
                  {t('Generation resumes automatically after payment')}
                </Benefit>
                <Benefit>
                  {t('Your character settings are kept while you pay')}
                </Benefit>
              </ul>
            </section>

            <section className='order-1 md:order-2'>
              {plans === null && (
                <div className='flex items-center justify-center gap-2 py-20 text-sm'>
                  <Spinner />
                  <span className='text-muted-foreground'>{t('Loading')}</span>
                </div>
              )}
              {plans !== null && showFallback && (
                <div className='flex flex-col gap-4'>
                  <h2 className='text-lg font-semibold'>
                    {t('No subscription plans available')}
                  </h2>
                  <p className='text-muted-foreground text-sm leading-relaxed'>
                    {t(
                      'This site does not sell subscription plans right now. Add credits in Wallet and come back to continue.'
                    )}
                  </p>
                  {plansFailed ? (
                    <p className='text-muted-foreground text-xs'>
                      {t('Could not load plans')}
                    </p>
                  ) : null}
                  <Button
                    type='button'
                    className='bg-foreground text-background hover:bg-foreground/90 h-12 w-full rounded-full'
                    onClick={openWalletTopUp}
                  >
                    {t('Add credits in Wallet')}
                  </Button>
                </div>
              )}
              {plans !== null && !showFallback && (
                <div className='flex flex-col'>
                  <h2 className='text-lg font-semibold'>
                    {t('Choose a plan')}
                  </h2>
                  <p className='text-muted-foreground mt-1.5 mb-4 text-xs'>
                    {t(
                      'Plans grant credits every period. Manage or cancel anytime in Wallet.'
                    )}
                  </p>
                  <div
                    className='flex flex-col gap-2.5'
                    role='radiogroup'
                    aria-label={t('Subscription plans')}
                  >
                    {plans?.map((record) => (
                      <PlanRow
                        key={record.plan.id}
                        plan={record.plan}
                        selected={record.plan.id === planId}
                        onSelect={() => setPlanId(record.plan.id)}
                      />
                    ))}
                  </div>
                  <div className='border-border bg-card/40 text-muted-foreground mt-4 mb-4 flex items-center gap-2.5 rounded-xl border px-4 py-2.5 text-xs'>
                    <CreditCard
                      className='size-4 shrink-0'
                      aria-hidden='true'
                    />
                    <span>{t('Payment method')}</span>
                    <span className='text-foreground font-semibold'>
                      Stripe
                    </span>
                    <span className='ml-auto flex items-center gap-1'>
                      <Lock className='size-3' aria-hidden='true' />
                      {t('Secure payment')}
                    </span>
                  </div>
                  <Button
                    type='button'
                    className='bg-foreground text-background hover:bg-foreground/90 h-12 w-full rounded-full'
                    disabled={!selectedPlan || paying || phase === 'awaiting'}
                    aria-busy={paying}
                    onClick={() => void startPayment()}
                  >
                    {paying || phase === 'awaiting' ? <Spinner /> : null}
                    {payLabel}
                  </Button>
                  {phase === 'awaiting' ? (
                    <div className='mt-3 flex flex-col items-center gap-2 text-center'>
                      <p className='text-muted-foreground text-xs'>
                        {t(
                          'Complete the payment in the new tab. This page continues automatically.'
                        )}
                      </p>
                      <button
                        type='button'
                        className='text-primary text-xs underline underline-offset-4'
                        onClick={() => void checkManually()}
                      >
                        {t('I have completed the payment')}
                      </button>
                    </div>
                  ) : (
                    <button
                      type='button'
                      className='text-muted-foreground hover:text-foreground mt-3 text-center text-xs underline underline-offset-4'
                      onClick={props.onClose}
                    >
                      {t('Not now, back to editor')}
                    </button>
                  )}
                </div>
              )}
            </section>
          </div>
        )}
      </div>

      <footer className='border-border/70 text-muted-foreground shrink-0 border-t px-4 py-3 text-center text-[11px] sm:px-7'>
        {t(
          'Subscriptions can be managed or cancelled in Wallet. Payment is processed by Stripe.'
        )}
      </footer>
    </div>
  )
}

function Step(props: { index: number; active: boolean; label: string }) {
  return (
    <li
      className={cn(
        'flex items-center gap-1.5',
        props.active && 'text-foreground'
      )}
    >
      <span className='border-border flex size-4.5 items-center justify-center rounded-full border text-[10px]'>
        {props.index}
      </span>
      {props.label}
    </li>
  )
}

function Benefit(props: { children: ReactNode }) {
  return (
    <li className='flex items-start gap-2'>
      <Check
        className='text-primary mt-0.5 size-3.5 shrink-0'
        aria-hidden='true'
      />
      {props.children}
    </li>
  )
}

function ReceiptRow(props: {
  label: string
  children: ReactNode
  last?: boolean
}) {
  return (
    <div
      className={cn(
        'flex items-center justify-between py-3 text-xs',
        !props.last && 'border-border/60 border-b'
      )}
    >
      <dt className='text-muted-foreground'>{props.label}</dt>
      <dd className='font-semibold'>{props.children}</dd>
    </div>
  )
}
