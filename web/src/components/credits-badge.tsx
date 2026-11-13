import { Link } from '@tanstack/react-router'
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
import { Coins } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

/**
 * Header badge that shows the signed-in user's remaining credit balance.
 * Quota is stored as raw units (500,000 = $1); the product presents 10 credits
 * per USD, matching the price line used by the workbench generate bar.
 */
export function CreditsBadge({ className }: { className?: string }) {
  const { t } = useTranslation()
  const quota = useAuthStore((state) => state.auth.user?.quota)

  if (quota == null) {
    return null
  }

  const credits = Math.floor(quota / 50000)

  return (
    <Link
      to='/wallet'
      aria-label={t('Credits')}
      className={cn(
        'border-border/60 bg-muted/40 text-foreground hover:bg-accent inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-sm font-medium transition-colors',
        className
      )}
    >
      <Coins className='text-muted-foreground size-4' aria-hidden='true' />
      <span className='tabular-nums'>{credits}</span>
      <span className='text-muted-foreground hidden text-xs md:inline'>
        {t('credits')}
      </span>
    </Link>
  )
}
