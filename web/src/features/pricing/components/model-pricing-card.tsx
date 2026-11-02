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
import { ArrowRight, Check, Copy } from 'lucide-react'
import { useCallback, useMemo, type KeyboardEvent, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { resolveCardSavingBadge } from '../lib/saving-display'
import type { PublicModelCardItem } from '../types'
import { SavingBadge } from './saving-badge'
import { TierPriceTable } from './tier-price-table'

export type ModelPricingCardProps = {
  card: PublicModelCardItem
  className?: string
  onCopyModelId?: (modelId: string) => void
  onCardClick?: (modelId: string) => void
}

export function ModelPricingCard(props: ModelPricingCardProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard()

  let navigate: ReturnType<typeof useNavigate> | null = null
  try {
    navigate = useNavigate()
  } catch {
    navigate = null
  }

  const modelId = props.card.modelId
  const isCopied = copiedText === modelId
  const tierCount =
    props.card.tierCount > 0 ? props.card.tierCount : props.card.tiers.length
  const vendorIcon = props.card.vendorIcon
    ? getLobeIcon(props.card.vendorIcon, 20)
    : getLobeIcon(props.card.provider, 20)

  const cardSavingBadge = useMemo(
    () => resolveCardSavingBadge(props.card.tiers),
    [props.card.tiers]
  )

  const handleCopy = useCallback(
    async (event: MouseEvent<HTMLButtonElement>) => {
      event.stopPropagation()
      const success = await copyToClipboard(modelId)
      if (success && props.onCopyModelId) {
        props.onCopyModelId(modelId)
      }
    },
    [copyToClipboard, modelId, props]
  )

  const handleCardClick = useCallback(() => {
    if (props.onCardClick) {
      props.onCardClick(modelId)
      return
    }
    if (navigate) {
      try {
        navigate({
          to: '/pricing/$modelId',
          params: { modelId },
        })
        return
      } catch {
        // Fallback to window navigation if router is not mounted
      }
    }
    if (typeof window !== 'undefined') {
      window.location.assign(`/pricing/${encodeURIComponent(modelId)}`)
    }
  }, [navigate, modelId, props])

  const handleKeyDown = useCallback(
    (event: KeyboardEvent<HTMLElement>) => {
      if (event.key === 'Enter' || event.key === ' ') {
        if (event.target === event.currentTarget) {
          event.preventDefault()
          handleCardClick()
        }
      }
    },
    [handleCardClick]
  )

  const tierBadgeLabel =
    tierCount === 1
      ? t('1 PRICE TIER')
      : t('{{count}} PRICE TIERS', { count: tierCount })

  const modelPageUrl = `/pricing/${encodeURIComponent(modelId)}`

  return (
    <section
      role='link'
      tabIndex={0}
      onClick={handleCardClick}
      onKeyDown={handleKeyDown}
      className={cn(
        'group bg-card border-border hover:border-foreground/30 hover:shadow-xs w-full cursor-pointer border transition-all',
        props.className
      )}
      data-model-id={modelId}
      aria-label={t('View model {{name}} and generate', {
        name: props.card.name,
        defaultValue: `View ${props.card.name}`,
      })}
    >
      <header className='flex items-start justify-between gap-4 px-4 py-3 sm:px-5 sm:py-3.5'>
        <div className='flex min-w-0 flex-1 flex-col gap-1 overflow-hidden'>
          <h3 className='leading-[1.2] tracking-tight'>
            <span className='grid w-full min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-2.5'>
              <span
                className='flex size-5 shrink-0 items-center justify-center sm:size-6'
                aria-hidden='true'
              >
                {vendorIcon}
              </span>
              <span
                className='text-foreground group-hover:text-primary block min-w-0 truncate font-mono text-[16px] font-medium tracking-tight transition-colors sm:text-[18px]'
                title={props.card.name}
              >
                {props.card.name}
              </span>
            </span>
          </h3>

          <div className='text-muted-foreground flex flex-wrap items-center gap-1.5 font-mono text-[13px] leading-[16px]'>
            <span>({modelId})</span>
            <button
              type='button'
              onClick={handleCopy}
              title={t('Copy model ID')}
              aria-label={
                isCopied ? t('Copied model ID') : t('Copy model ID')
              }
              className='text-muted-foreground hover:text-foreground inline-flex cursor-pointer items-center p-0.5 transition-colors'
            >
              {isCopied ? (
                <Check
                  className='size-3 text-emerald-500 dark:text-emerald-400'
                  aria-hidden='true'
                />
              ) : (
                <Copy className='size-3' aria-hidden='true' />
              )}
            </button>
            {cardSavingBadge.kind !== 'none' ? (
              <SavingBadge badge={cardSavingBadge} variant='capsule' />
            ) : null}
          </div>
        </div>

        <div className='flex shrink-0 flex-wrap items-center gap-2 pt-0.5'>
          <span className='border-border bg-background/80 text-muted-foreground inline-flex items-center border px-2.5 py-0.5 font-mono text-[11px] uppercase select-none'>
            {tierBadgeLabel}
          </span>
          <a
            href={modelPageUrl}
            onClick={(event) => {
              // Let user Cmd+click / middle click open in new tab normally; otherwise handle via card click
              if (!event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey) {
                event.preventDefault()
                event.stopPropagation()
                handleCardClick()
              }
            }}
            className='border-border bg-background/80 hover:bg-foreground hover:text-background text-foreground inline-flex items-center gap-1 border px-2.5 py-0.5 font-mono text-[11px] font-medium transition-colors'
          >
            <span>{t('Generate')}</span>
            <ArrowRight className='size-3' aria-hidden='true' />
          </a>
        </div>
      </header>

      <TierPriceTable tiers={props.card.tiers} />
    </section>
  )
}
