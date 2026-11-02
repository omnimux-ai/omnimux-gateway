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
import { Crown, RefreshCw, Sparkles, Check, Lock } from 'lucide-react'
import { useState, useEffect, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StatusBadge,
  dotColorMap,
  textColorMap,
} from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
  updateBillingPreference,
} from '@/features/subscriptions/api'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import { formatDuration, formatResetPeriod } from '@/features/subscriptions/lib'
import { planTierDisplayName } from '@/features/subscriptions/lib/plan-tier'
import type {
  PlanRecord,
  SubscriptionPlan,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { formatNumber, formatQuota, quotaUnitsToDollars } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import type { PaymentMethod, TopupInfo } from '../types'

interface SubscriptionPlansCardProps {
  topupInfo: TopupInfo | null
  onAvailabilityChange?: (available: boolean) => void
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

/** The two billing periods the plan list can be sliced by. */
type BillingPeriod = 'monthly' | 'annual'

const MONTHS_PER_YEAR = 12
const DAYS_PER_MONTH = 30

function getEpayMethods(payMethods: PaymentMethod[] = []): PaymentMethod[] {
  return payMethods.filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem'
  )
}

function getBillingPreferenceLabel(
  preference: string,
  t: (key: string) => string
): string {
  switch (preference) {
    case 'subscription_first':
      return t('Subscription First')
    case 'wallet_first':
      return t('Wallet First')
    case 'subscription_only':
      return t('Subscription Only')
    case 'wallet_only':
      return t('Wallet Only')
    default:
      return preference
  }
}

/**
 * Service window of a plan record in months, used to tell a monthly record from
 * an annual one. Records are stored as separate rows (duration_unit/year), so
 * every period decision in this card goes through here.
 */
function planDurationMonths(plan: SubscriptionPlan): number {
  const value = Number(plan.duration_value || 0)
  switch (plan.duration_unit) {
    case 'year':
      return value * MONTHS_PER_YEAR
    case 'month':
      return value
    case 'day':
      return value / DAYS_PER_MONTH
    case 'hour':
      return value / (24 * DAYS_PER_MONTH)
    case 'custom':
      return Number(plan.custom_seconds || 0) / (86400 * DAYS_PER_MONTH)
    default:
      return value
  }
}

function planBillingPeriod(plan: SubscriptionPlan): BillingPeriod {
  return planDurationMonths(plan) >= MONTHS_PER_YEAR ? 'annual' : 'monthly'
}

/**
 * Tier titles carry the billing suffix (「入门版 · Starter（年付）」). The tier
 * name shown on the card is the title with that suffix removed.
 */
const TITLE_PERIOD_SUFFIX_RE =
  /\s*[（(【[]\s*[^）)】\]]*(?:年付|月付|annual|yearly|monthly|year|month)[^）)】\]]*[）)】\]]\s*$/i

function stripPeriodSuffix(title: string): string {
  const stripped = title.replace(TITLE_PERIOD_SUFFIX_RE, '').trim()
  return stripped || title.trim()
}

/**
 * Tier identity: the upgrade group when the plan carries one, otherwise the
 * period-stripped title. Monthly and annual records of the same tier share it.
 */
function planTierKey(plan: SubscriptionPlan): string {
  const group = (plan.upgrade_group || '').trim().toLowerCase()
  if (group) return `group:${group}`
  const title = stripPeriodSuffix(plan.title || '')
    .toLowerCase()
    .replaceAll(/\s+/g, '')
  return `title:${title}`
}

interface PlanTier {
  key: string
  title: string
  records: PlanRecord[]
  sortOrder: number
}

function groupPlansByTier(plans: PlanRecord[]): PlanTier[] {
  const tiers = new Map<string, PlanTier>()
  for (const record of plans) {
    const plan = record?.plan
    if (!plan) continue
    const key = planTierKey(plan)
    const sortOrder = Number(plan.sort_order || 0)
    const existing = tiers.get(key)
    if (existing) {
      existing.records.push(record)
      existing.sortOrder = Math.min(existing.sortOrder, sortOrder)
      continue
    }
    tiers.set(key, {
      key,
      title: stripPeriodSuffix(plan.title || ''),
      records: [record],
      sortOrder,
    })
  }
  return [...tiers.values()].sort(
    (a, b) => a.sortOrder - b.sortOrder || a.title.localeCompare(b.title)
  )
}

function shortestRecord(records: PlanRecord[]): PlanRecord | null {
  return records.reduce<PlanRecord | null>(
    (best, record) =>
      best === null ||
      planDurationMonths(record.plan) < planDurationMonths(best.plan)
        ? record
        : best,
    null
  )
}

/** The record a tier sells in the given period, or null when it has none. */
function pickTierRecord(
  tier: PlanTier,
  period: BillingPeriod
): PlanRecord | null {
  return shortestRecord(
    tier.records.filter((record) => planBillingPeriod(record.plan) === period)
  )
}

/** The record a tier falls back to when no period is being selected. */
function baseTierRecord(tier: PlanTier): PlanRecord | null {
  return shortestRecord(tier.records)
}

function hasBothPeriods(tier: PlanTier): boolean {
  return (
    pickTierRecord(tier, 'monthly') !== null &&
    pickTierRecord(tier, 'annual') !== null
  )
}

/** Price suffix: "/ month", "/ year", or the record's own duration. */
function planPeriodLabel(plan: SubscriptionPlan, t: TFunction): string {
  const unit = plan.duration_unit
  const value = Number(plan.duration_value || 1)
  if (unit === 'month' && value === 1) return t('month')
  if (unit === 'year' && value === 1) return t('year')
  return formatDuration(plan, t)
}

function planBilledLabel(plan: SubscriptionPlan, t: TFunction): string {
  const unit = plan.duration_unit
  const value = Number(plan.duration_value || 1)
  if (unit === 'month' && value === 1) return t('Billed monthly')
  if (unit === 'year' && value === 1) return t('Billed annually')
  return `${t('Validity Period')}: ${formatDuration(plan, t)}`
}

/** Credits granted per period, in the platform's display unit (not raw quota). */
function planCredits(plan: SubscriptionPlan): number {
  return quotaUnitsToDollars(Number(plan.total_amount || 0))
}

function planCreditsCaption(plan: SubscriptionPlan, t: TFunction): string {
  const reset = formatResetPeriod(plan, t)
  return reset === t('No Reset') ? t('Credits') : `${t('Credits')} · ${reset}`
}

/**
 * Real saving of the annual record against 12 monthly records, as a whole
 * percent. The smallest saving across tiers wins so the badge never overclaims.
 */
function computeAnnualSavingPercent(tiers: PlanTier[]): number {
  const savings: number[] = []
  for (const tier of tiers) {
    const monthly = pickTierRecord(tier, 'monthly')?.plan
    const annual = pickTierRecord(tier, 'annual')?.plan
    if (!monthly || !annual) continue
    const monthlyPrice = Number(monthly.price_amount || 0)
    const annualPrice = Number(annual.price_amount || 0)
    const months = planDurationMonths(annual)
    if (monthlyPrice <= 0 || annualPrice <= 0 || months <= 0) continue
    savings.push(1 - annualPrice / (monthlyPrice * months))
  }
  if (savings.length === 0) return 0
  return Math.floor(Math.min(...savings) * 100)
}

/**
 * The tier worth pointing at: the one giving the most credits per unit of
 * price in the period being shown. No purchase data is invented here.
 */
function pickPopularTierKey(
  items: { tier: PlanTier; plan: SubscriptionPlan }[]
): string | null {
  let bestKey: string | null = null
  let bestRatio = -1
  let bestCredits = -1
  for (const item of items) {
    const price = Number(item.plan.price_amount || 0)
    const credits = planCredits(item.plan)
    if (price <= 0 || credits <= 0) continue
    const ratio = credits / price
    if (ratio > bestRatio || (ratio === bestRatio && credits > bestCredits)) {
      bestRatio = ratio
      bestCredits = credits
      bestKey = item.tier.key
    }
  }
  return bestKey
}

interface PlanCardProps {
  plan: SubscriptionPlan
  tierTitle: string
  isPopular: boolean
  purchaseLimit: number
  purchaseCount: number
  onSelect: () => void
}

function PlanCard({
  plan,
  tierTitle,
  isPopular,
  purchaseLimit,
  purchaseCount,
  onSelect,
}: PlanCardProps) {
  const { t } = useTranslation()

  const price = Number(plan.price_amount || 0)
  const limit = purchaseLimit
  const reached = limit > 0 && purchaseCount >= limit
  const resetLabel = formatResetPeriod(plan, t)
  const benefits = [
    `${t('Validity Period')}: ${formatDuration(plan, t)}`,
    resetLabel !== t('No Reset')
      ? `${t('Quota Reset')}: ${resetLabel}`
      : null,
    limit > 0 ? `${t('Purchase Limit')}: ${limit}` : null,
    plan.upgrade_group
      ? `${t('Upgrade Group')}: ${plan.upgrade_group}`
      : null,
  ].filter(Boolean) as string[]

  return (
    <div className='relative h-full'>
      {isPopular && (
        <Badge className='absolute top-0 left-4 z-10 -translate-y-1/2 gap-1 px-3 shadow-sm'>
          <Sparkles />
          {t('Most popular')}
        </Badge>
      )}
      <Card
        data-card-hover='false'
        className={cn('h-full rounded-2xl', isPopular && 'border-primary/60')}
      >
        <CardContent className='flex h-full flex-col gap-6 p-4 sm:p-6'>
          <div>
            <h4 className='text-lg font-semibold'>{tierTitle}</h4>
            <div className='mt-2 flex flex-wrap items-end gap-2'>
              <span className='text-[32px] leading-none font-semibold tracking-tight'>
                ${price.toFixed(2)}
              </span>
              <span className='text-muted-foreground pb-0.5 text-sm'>
                / {planPeriodLabel(plan, t)}
              </span>
            </div>
            <p className='text-muted-foreground mt-2 text-xs'>
              {planBilledLabel(plan, t)}
            </p>
          </div>

          <div className='bg-muted/60 rounded-2xl px-4 py-6 text-center'>
            <div className='text-primary text-[44px] leading-none font-semibold tracking-tight'>
              {formatNumber(planCredits(plan))}
            </div>
            <div className='text-muted-foreground mt-2 text-xs'>
              {planCreditsCaption(plan, t)}
            </div>
          </div>

          <ul className='flex-1 space-y-3'>
            {benefits.map((label) => (
              <li key={label} className='flex items-start gap-2 text-sm'>
                <Check className='text-primary mt-0.5 size-3.5 shrink-0' />
                <span className='text-muted-foreground'>{label}</span>
              </li>
            ))}
          </ul>

          {reached ? (
            <Tooltip>
              <TooltipTrigger render={<div />}>
                <Button
                  variant='outline'
                  className='h-10 w-full rounded-full'
                  disabled
                >
                  <Lock className='size-3.5' />
                  {t('Limit Reached')}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {t('Purchase limit reached')} ({purchaseCount}/{limit})
              </TooltipContent>
            </Tooltip>
          ) : (
            <Button
              variant={isPopular ? 'default' : 'outline'}
              className='h-10 w-full rounded-full'
              onClick={onSelect}
            >
              <Lock className='size-3.5' />
              {t('Subscribe Now')}
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export function SubscriptionPlansCard({
  topupInfo,
  onAvailabilityChange,
  userQuota,
  onPurchaseSuccess,
}: SubscriptionPlansCardProps) {
  const { t } = useTranslation()

  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [activeSubscriptions, setActiveSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [billingPreference, setBillingPreference] =
    useState('subscription_first')
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [billingPeriod, setBillingPeriod] = useState<BillingPeriod>('monthly')

  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [selectedPlan, setSelectedPlan] = useState<PlanRecord | null>(null)

  const enableStripe = !!topupInfo?.enable_stripe_topup
  const enableCreem = !!topupInfo?.enable_creem_topup
  const enableWaffoPancake = !!topupInfo?.enable_waffo_pancake_topup
  const enableOnlineTopUp = !!topupInfo?.enable_online_topup
  const epayMethods = useMemo(
    () => getEpayMethods(topupInfo?.pay_methods),
    [topupInfo?.pay_methods]
  )

  const fetchPlans = useCallback(async () => {
    try {
      const res = requireServerSuccess(await getPublicPlans())
      if (res.success) {
        setPlans(res.data || [])
      }
    } catch (error) {
      handleServerError(error)
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = requireServerSuccess(await getSelfSubscriptionFull())
      if (res.success && res.data) {
        setBillingPreference(
          res.data.billing_preference || 'subscription_first'
        )
        setActiveSubscriptions(res.data.subscriptions || [])
        setAllSubscriptions(res.data.all_subscriptions || [])
      }
    } catch (error) {
      handleServerError(error)
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await fetchSelfSubscription()
    } finally {
      setRefreshing(false)
    }
  }

  const handlePreferenceChange = async (pref: string) => {
    const previous = billingPreference
    setBillingPreference(pref)
    try {
      const res = await updateBillingPreference(pref)
      if (res.success) {
        toast.success(t('Updated successfully'))
        const normalized = res.data?.billing_preference || pref
        setBillingPreference(normalized)
      } else {
        handleServerError(res, t('Update failed'))
        setBillingPreference(previous)
      }
    } catch (error) {
      handleServerError(error, t('Request failed'))
      setBillingPreference(previous)
    }
  }

  const hasActive = activeSubscriptions.length > 0
  const hasAny = allSubscriptions.length > 0
  const isAvailable = loading || plans.length > 0 || hasAny
  const disablePref = !hasActive
  const isSubPref =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only'

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const sub of allSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (!planId) continue
      map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [allSubscriptions])

  useEffect(() => {
    onAvailabilityChange?.(isAvailable)
  }, [isAvailable, onAvailabilityChange])

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id) {
        map.set(p.plan.id, p.plan.title || '')
      }
    }
    return map
  }, [plans])

  const getRemainingDays = (sub: UserSubscriptionRecord) => {
    const endTime = sub?.subscription?.end_time || 0
    if (!endTime) return 0
    const now = Date.now() / 1000
    return Math.max(0, Math.ceil((endTime - now) / 86400))
  }

  const getUsagePercent = (sub: UserSubscriptionRecord) => {
    const total = Number(sub?.subscription?.amount_total || 0)
    const used = Number(sub?.subscription?.amount_used || 0)
    if (total <= 0) return 0
    return Math.round((used / total) * 100)
  }

  const tiers = useMemo(() => groupPlansByTier(plans), [plans])

  // Tiers sold in both periods drive the toggle; the rest cannot be toggled.
  const { toggleTiers, singlePeriodTiers, hasToggle } = useMemo(() => {
    const toggleable: PlanTier[] = []
    const single: PlanTier[] = []
    for (const tier of tiers) {
      if (hasBothPeriods(tier)) {
        toggleable.push(tier)
      } else {
        single.push(tier)
      }
    }
    return {
      toggleTiers: toggleable,
      singlePeriodTiers: single,
      hasToggle: toggleable.length > 0,
    }
  }, [tiers])

  const annualSavingPercent = useMemo(
    () => computeAnnualSavingPercent(toggleTiers),
    [toggleTiers]
  )

  const openPurchase = (record: PlanRecord) => {
    setSelectedPlan(record)
    setPurchaseOpen(true)
  }

  const renderPlanGrid = (items: { tier: PlanTier; record: PlanRecord }[]) => {
    const popularKey = pickPopularTierKey(
      items.map((item) => ({ tier: item.tier, plan: item.record.plan }))
    )
    // fork: plan cards render cheapest-first, left to right
    const ordered = [...items].sort(
      (a, b) =>
        Number(a.record.plan.price_amount || 0) -
        Number(b.record.plan.price_amount || 0)
    )
    return (
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {ordered.map((item) => (
          <PlanCard
            key={item.tier.key}
            plan={item.record.plan}
            tierTitle={planTierDisplayName(
              item.record.plan,
              t,
              item.tier.title
            )}
            isPopular={item.tier.key === popularKey}
            purchaseLimit={Number(item.record.plan.max_purchase_per_user || 0)}
            purchaseCount={planPurchaseCountMap.get(item.record.plan.id) || 0}
            onSelect={() => openPurchase(item.record)}
          />
        ))}
      </div>
    )
  }

  const renderPeriodGrid = (period: BillingPeriod) => {
    const items = toggleTiers
      .map((tier) => ({ tier, record: pickTierRecord(tier, period) }))
      .filter(
        (item): item is { tier: PlanTier; record: PlanRecord } =>
          item.record !== null
      )
    const singles = singlePeriodTiers
      .map((tier) => ({ tier, record: baseTierRecord(tier) }))
      .filter(
        (item): item is { tier: PlanTier; record: PlanRecord } =>
          item.record !== null
      )
    if (items.length + singles.length === 0) return null
    return renderPlanGrid([...items, ...singles])
  }

  if (loading) {
    return (
      <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
          <Skeleton className='h-6 w-32' />
          <Skeleton className='h-4 w-56' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:space-y-6 sm:p-5'>
          <Skeleton className='mx-auto h-10 w-64 rounded-full' />
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
            {['first', 'second', 'third'].map((key) => (
              <Skeleton key={key} className='h-80 w-full rounded-2xl' />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (plans.length === 0 && !hasAny) {
    return null
  }

  return (
    <>
      <TitledCard
        title={
          <span className='block'>
            <span className='text-muted-foreground block text-xs font-medium tracking-[0.04em] uppercase'>
              {t('Credits')}
            </span>
            <span className='block'>{t('Subscription Plans')}</span>
          </span>
        }
        titleClassName='text-2xl sm:text-2xl'
        description={t('Subscribe to a plan for model access')}
        icon={<Crown className='h-4 w-4' />}
        iconTone='warning'
        disableHoverEffect
        contentClassName='space-y-4 sm:space-y-6'
      >
        {/* Available plans */}
        {plans.length > 0 ? (
          <div className='space-y-4 sm:space-y-6'>
            {hasToggle && (
              <Tabs
                value={billingPeriod}
                onValueChange={(value) =>
                  value && setBillingPeriod(value as BillingPeriod)
                }
                className='w-full gap-4 sm:gap-6'
              >
                <TabsList className='group-data-horizontal/tabs:h-10 self-center rounded-full p-1'>
                  <TabsTrigger
                    value='monthly'
                    className='rounded-full px-4 text-sm'
                  >
                    {t('Monthly')}
                  </TabsTrigger>
                  <TabsTrigger
                    value='annual'
                    className='gap-2 rounded-full px-4 text-sm'
                  >
                    {t('Annual')}
                    {annualSavingPercent >= 1 && (
                      <Badge variant='secondary'>
                        {t('Save {{percent}}%', {
                          percent: annualSavingPercent,
                        })}
                      </Badge>
                    )}
                  </TabsTrigger>
                </TabsList>
                <TabsContent value='monthly'>
                  {renderPeriodGrid('monthly')}
                </TabsContent>
                <TabsContent value='annual'>
                  {renderPeriodGrid('annual')}
                </TabsContent>
              </Tabs>
            )}

            {!hasToggle && renderPlanGrid(
              tiers
                .map((tier) => ({ tier, record: baseTierRecord(tier) }))
                .filter(
                  (item): item is { tier: PlanTier; record: PlanRecord } =>
                    item.record !== null
                )
            )}

          </div>
        ) : (
          <p className='text-muted-foreground py-4 text-center text-sm'>
            {t('No plans available')}
          </p>
        )}

        <Separator />

        {/* My subscriptions & billing preference */}
        <div className='rounded-2xl border p-4 sm:p-6'>
          <div className='flex flex-wrap items-center justify-between gap-2.5 sm:gap-3'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <span className='text-sm font-medium'>
                {t('My Subscriptions')}
              </span>
              <span className='flex items-center gap-1.5 text-xs font-medium'>
                <span
                  className={cn(
                    'size-1.5 shrink-0 rounded-full',
                    hasActive ? dotColorMap.success : dotColorMap.neutral
                  )}
                  aria-hidden='true'
                />
                {hasActive ? (
                  <span className={cn(textColorMap.success)}>
                    {activeSubscriptions.length} {t('active')}
                  </span>
                ) : (
                  <span className='text-muted-foreground'>
                    {t('No Active')}
                  </span>
                )}
                {allSubscriptions.length > activeSubscriptions.length && (
                  <>
                    <span className='text-muted-foreground/30'>·</span>
                    <span className='text-muted-foreground'>
                      {allSubscriptions.length - activeSubscriptions.length}{' '}
                      {t('expired')}
                    </span>
                  </>
                )}
              </span>
            </div>
            <div className='flex w-full items-center gap-2 sm:w-auto'>
              <Select
                items={[
                  {
                    value: 'subscription_first',
                    label: (
                      <>
                        {getBillingPreferenceLabel('subscription_first', t)}
                        {disablePref ? ` (${t('No Active')})` : ''}
                      </>
                    ),
                  },
                  {
                    value: 'wallet_first',
                    label: getBillingPreferenceLabel('wallet_first', t),
                  },
                  {
                    value: 'subscription_only',
                    label: (
                      <>
                        {getBillingPreferenceLabel('subscription_only', t)}
                        {disablePref ? ` (${t('No Active')})` : ''}
                      </>
                    ),
                  },
                  {
                    value: 'wallet_only',
                    label: getBillingPreferenceLabel('wallet_only', t),
                  },
                ]}
                value={billingPreference}
                onValueChange={(v) => v !== null && handlePreferenceChange(v)}
              >
                <SelectTrigger className='h-8 flex-1 text-xs sm:w-[140px] sm:flex-none'>
                  <SelectValue>
                    {getBillingPreferenceLabel(billingPreference, t)}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem
                      value='subscription_first'
                      disabled={disablePref}
                    >
                      {getBillingPreferenceLabel('subscription_first', t)}
                      {disablePref ? ` (${t('No Active')})` : ''}
                    </SelectItem>
                    <SelectItem value='wallet_first'>
                      {getBillingPreferenceLabel('wallet_first', t)}
                    </SelectItem>
                    <SelectItem
                      value='subscription_only'
                      disabled={disablePref}
                    >
                      {getBillingPreferenceLabel('subscription_only', t)}
                      {disablePref ? ` (${t('No Active')})` : ''}
                    </SelectItem>
                    <SelectItem value='wallet_only'>
                      {getBillingPreferenceLabel('wallet_only', t)}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Button
                variant='ghost'
                size='icon'
                className='h-8 w-8'
                onClick={handleRefresh}
                disabled={refreshing}
              >
                <RefreshCw
                  className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
                />
              </Button>
            </div>
          </div>

          {disablePref && isSubPref && (
            <p className='text-muted-foreground mt-2 text-xs'>
              {billingPreference === 'subscription_only'
                ? t(
                    'Preference saved as {{pref}}, but no active subscription. Requests will be rejected.',
                    { pref: t('Subscription Only') }
                  )
                : t(
                    'Preference saved as {{pref}}, but no active subscription. Wallet will be used automatically.',
                    { pref: t('Subscription First') }
                  )}
            </p>
          )}

          {hasAny && (
            <>
              <Separator className='my-3' />
              <div className='max-h-64 space-y-3 overflow-y-auto pr-1'>
                {allSubscriptions.map((sub) => {
                  const subscription = sub.subscription
                  const totalAmount = Number(subscription?.amount_total || 0)
                  const usedAmount = Number(subscription?.amount_used || 0)
                  const remainAmount =
                    totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0
                  const planTitle =
                    planTitleMap.get(subscription?.plan_id) || ''
                  const remainDays = getRemainingDays(sub)
                  const usagePercent = getUsagePercent(sub)
                  const now = Date.now() / 1000
                  const isExpired = (subscription?.end_time || 0) < now
                  const isCancelled = subscription?.status === 'cancelled'
                  const isActive =
                    subscription?.status === 'active' && !isExpired
                  const nextResetTime = subscription?.next_reset_time ?? 0
                  let statusBadge = (
                    <StatusBadge
                      label={t('Expired')}
                      variant='neutral'
                      copyable={false}
                    />
                  )
                  if (isActive) {
                    statusBadge = (
                      <StatusBadge
                        label={t('Active')}
                        variant='success'
                        copyable={false}
                      />
                    )
                  } else if (isCancelled) {
                    statusBadge = (
                      <StatusBadge
                        label={t('Cancelled')}
                        variant='neutral'
                        copyable={false}
                      />
                    )
                  }

                  let endTimeLabel = t('Expired at')
                  if (isActive) {
                    endTimeLabel = t('Until')
                  } else if (isCancelled) {
                    endTimeLabel = t('Cancelled at')
                  }

                  return (
                    <div
                      key={subscription?.id}
                      className='bg-background rounded-md border p-3 text-xs'
                    >
                      <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-2'>
                          <span className='font-medium'>
                            {planTitle
                              ? `${planTitle} · ${t('Subscription')} #${subscription?.id}`
                              : `${t('Subscription')} #${subscription?.id}`}
                          </span>
                          {statusBadge}
                        </div>
                        {isActive && (
                          <span className='text-muted-foreground'>
                            {t('{{count}} days remaining', {
                              count: remainDays,
                            })}
                          </span>
                        )}
                      </div>
                      <div className='text-muted-foreground mt-1.5'>
                        {endTimeLabel}{' '}
                        {new Date(
                          (subscription?.end_time || 0) * 1000
                        ).toLocaleString()}
                      </div>
                      {isActive && nextResetTime > 0 && (
                        <div className='text-muted-foreground mt-1'>
                          {t('Next reset')}:{' '}
                          {new Date(nextResetTime * 1000).toLocaleString()}
                        </div>
                      )}
                      <div className='text-muted-foreground mt-1'>
                        {t('Total Quota')}:{' '}
                        {totalAmount > 0 ? (
                          <Tooltip>
                            <TooltipTrigger
                              render={<span className='cursor-help' />}
                            >
                              {formatQuota(usedAmount)}/
                              {formatQuota(totalAmount)} · {t('Remaining')}{' '}
                              {formatQuota(remainAmount)}
                            </TooltipTrigger>
                            <TooltipContent>
                              {t('Raw Quota')}: {usedAmount}/{totalAmount} ·{' '}
                              {t('Remaining')} {remainAmount}
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          t('Unlimited')
                        )}
                        {totalAmount > 0 && (
                          <span className='ml-2'>
                            {t('Used')} {usagePercent}%
                          </span>
                        )}
                      </div>
                      {totalAmount > 0 && isActive && (
                        <Progress value={usagePercent} className='mt-2 h-1.5' />
                      )}
                    </div>
                  )
                })}
              </div>
            </>
          )}

          {!hasAny && (
            <p className='text-muted-foreground mt-2 text-xs'>
              {t('Subscribe to a plan for model access')}
            </p>
          )}
        </div>
      </TitledCard>

      <SubscriptionPurchaseDialog
        open={purchaseOpen}
        onOpenChange={(open) => {
          setPurchaseOpen(open)
          if (!open) {
            fetchSelfSubscription()
          }
        }}
        plan={selectedPlan}
        enableStripe={enableStripe}
        enableCreem={enableCreem}
        enableWaffoPancake={enableWaffoPancake}
        enableOnlineTopUp={enableOnlineTopUp}
        epayMethods={epayMethods}
        userQuota={userQuota}
        onPurchaseSuccess={onPurchaseSuccess}
        purchaseLimit={
          selectedPlan?.plan?.max_purchase_per_user
            ? Number(selectedPlan.plan.max_purchase_per_user)
            : undefined
        }
        purchaseCount={
          selectedPlan?.plan?.id
            ? planPurchaseCountMap.get(selectedPlan.plan.id)
            : undefined
        }
      />
    </>
  )
}
