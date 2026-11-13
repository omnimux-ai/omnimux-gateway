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

import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { api } from '@/lib/api'

type SocialSeatsView = {
  billing_mode?: string
  seats_purchased?: number
  seats_occupied?: number
  seats_vacant?: number
  next_expiry?: string | null
  seat_price_usd?: number
  seat_price_credits?: number
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

export function SocialSeatsCard() {
  const [view, setView] = useState<SocialSeatsView | null>(null)

  useEffect(() => {
    let cancelled = false
    void fetchSocialSeats().then((next) => {
      if (!cancelled) setView(next)
    })
    return () => {
      cancelled = true
    }
  }, [])

  if (!view || view.billing_mode !== 'seat') {
    return null
  }

  const stats = [
    { label: '已购买', value: String(view.seats_purchased ?? 0) },
    { label: '占用中', value: String(view.seats_occupied ?? 0) },
    { label: '可绑空闲', value: String(view.seats_vacant ?? 0) },
    { label: '最近到期', value: view.next_expiry ? view.next_expiry : '—' },
  ]

  const credits = view.seat_price_credits ?? 0
  const usd = view.seat_price_usd ?? 0

  return (
    <Card data-card-hover='false' className='bg-muted/20 py-0'>
      <CardContent className='flex flex-col gap-3 p-3 sm:p-4'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <IconBadge tone='info'>
            <Link2 />
          </IconBadge>
          <div className='min-w-0'>
            <h3 className='truncate text-sm font-semibold'>社媒绑定席位</h3>
            <p className='text-muted-foreground line-clamp-1 text-xs'>
              解绑不退积分；无空闲席位时新开一席在账号落库时扣费
            </p>
          </div>
        </div>

        <div className='grid grid-cols-2 gap-2 sm:grid-cols-4 sm:divide-x sm:gap-0 rounded-lg border'>
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

        <div className='text-muted-foreground text-xs'>
          新开一席价：{credits} 积分（${usd} / 30 天）
        </div>
      </CardContent>
    </Card>
  )
}
