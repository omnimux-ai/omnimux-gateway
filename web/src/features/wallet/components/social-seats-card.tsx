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
import { Link2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { toIntlLocale } from '@/i18n/languages'
import { api } from '@/lib/api'

type VacantSeatItem = {
  id: number
  expires_at: string
  swaps_remaining: number
  same_account_only: boolean
}

type SocialSeatsView = {
  billing_mode?: string
  seats_purchased?: number
  seats_occupied?: number
  seats_vacant?: number
  seats_cap?: number
  next_expiry?: string | null
  seat_price_usd?: number
  seat_price_credits?: number
  vacant_seats?: VacantSeatItem[]
}

type ApiEnvelope = {
  success?: boolean
  data?: SocialSeatsView
}

async function fetchSocialSeats(): Promise<SocialSeatsView | null> {
  try {
    const res = await api.get('/api/social/v1/seats', {
      skipErrorHandler: true,
      skipBusinessError: true,
    })
    const body = res.data as ApiEnvelope | SocialSeatsView | undefined
    if (!body) return null
    if (typeof body === 'object' && 'success' in body) {
      if (body.success === false || !body.data) return null
      return body.data
    }
    return body as SocialSeatsView
  } catch {
    return null
  }
}

function formatSeatDate(raw: string, language: string): string {
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) {
    return raw.slice(0, 10)
  }
  return new Intl.DateTimeFormat(toIntlLocale(language), {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  }).format(parsed)
}

export function SocialSeatsCard() {
  const { t, i18n } = useTranslation()
  const [view, setView] = useState<SocialSeatsView | null>(null)

  useEffect(() => {
    let cancelled = false
    const loadSeats = async () => {
      const next = await fetchSocialSeats()
      if (!cancelled) setView(next)
    }
    void loadSeats()
    return () => {
      cancelled = true
    }
  }, [])

  if (!view || view.billing_mode !== 'seat') {
    return null
  }

  const purchased = view.seats_purchased ?? 0
  const seatsCap = view.seats_cap ?? 20
  const stats = [
    { label: t('Purchased'), value: `${purchased} / ${seatsCap}` },
    { label: t('Occupied'), value: String(view.seats_occupied ?? 0) },
    { label: t('Vacant'), value: String(view.seats_vacant ?? 0) },
    {
      label: t('Next expiry'),
      value: view.next_expiry ? view.next_expiry : '—',
    },
  ]

  const credits = view.seat_price_credits ?? 20
  const usd = view.seat_price_usd ?? 2
  const vacantSeats = view.vacant_seats ?? []

  return (
    <Card data-card-hover='false' className='bg-muted/20 py-0'>
      <CardContent className='flex flex-col gap-3 p-3 sm:p-4'>
        <div className='flex min-w-0 flex-wrap items-center justify-between gap-2.5'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <IconBadge tone='info'>
              <Link2 />
            </IconBadge>
            <h3 className='truncate text-sm font-semibold'>
              {t('Social account binding seats')}
            </h3>
          </div>
          <Badge variant='secondary' className='font-mono tabular-nums'>
            {t('{{credits}} credits (${{usd}} / 30d)', { credits, usd })}
          </Badge>
        </div>

        <div className='grid grid-cols-2 gap-2 rounded-lg border sm:grid-cols-4 sm:gap-0 sm:divide-x'>
          {stats.map((item) => (
            <div key={item.label} className='min-w-0 px-2.5 py-2.5 sm:px-4'>
              <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
                {item.label}
              </div>
              <div className='text-foreground mt-1 font-mono text-sm font-bold tracking-tight break-all tabular-nums sm:text-lg'>
                {item.value}
              </div>
            </div>
          ))}
        </div>

        {vacantSeats.length > 0 ? (
          <div className='flex flex-wrap gap-2'>
            {vacantSeats.map((slot) => (
              <div
                key={slot.id}
                className='bg-background inline-flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs'
              >
                <span className='font-mono tabular-nums'>
                  {t('Vacant until {{date}}', {
                    date: formatSeatDate(slot.expires_at, i18n.language),
                  })}
                </span>
                <Badge
                  variant={slot.same_account_only ? 'warning' : 'secondary'}
                >
                  {slot.same_account_only
                    ? t('Same account only')
                    : t('{{count}} swaps left', {
                        count: slot.swaps_remaining,
                      })}
                </Badge>
              </div>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
