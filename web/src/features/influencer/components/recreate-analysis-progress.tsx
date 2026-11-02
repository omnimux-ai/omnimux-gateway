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
import { Check, ScanSearch } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Progress } from '@/components/ui/progress'
import { Spinner } from '@/components/ui/spinner'
import { cn } from '@/lib/utils'

import { PanelCard } from './panel-card'

/**
 * The decomposition is one synchronous request, so the rail advances on a
 * bounded timer instead of pretending to report server progress. It never
 * reaches 100% until the result replaces it.
 */
export function RecreateAnalysisProgress() {
  const { t } = useTranslation()
  const [progress, setProgress] = useState(6)
  useEffect(() => {
    // The queued planning turns take roughly two to three minutes; advance
    // slowly so the rail does not sit at its ceiling for the whole wait.
    const timer = setInterval(() => {
      setProgress((current) => (current >= 92 ? 92 : current + 2))
    }, 3500)
    return () => clearInterval(timer)
  }, [])

  const steps = [
    {
      title: t('Extract shot frames'),
      detail: t('Detects shot boundaries and timing.'),
    },
    {
      title: t('Understand your references'),
      detail: t('Locks the subject from your reference images.'),
    },
    {
      title: t('Decompose narrative and viral logic'),
      detail: t('Reads the hook, pacing, proof and call to action.'),
    },
    {
      title: t('Compile the Seedance prompt'),
      detail: t('Organizes beats around the reference clip duration.'),
    },
  ]
  const activeIndex = Math.min(steps.length - 1, Math.floor(progress / 25))

  return (
    <PanelCard
      icon={ScanSearch}
      label={t('Analyzing reference video')}
      action={
        <span className='text-xs text-[var(--inf-txt-3)]'>
          {t('About 2–3 minutes')}
        </span>
      }
    >
      <div role='status' aria-live='polite' className='flex flex-col gap-3'>
        <Progress
          value={progress}
          aria-label={t('Analyzing reference video')}
          className='w-full'
        />
        <ol className='flex flex-col gap-3'>
          {steps.map((step, index) => {
            const done = index < activeIndex
            const active = index === activeIndex
            return (
              <li key={step.title} className='flex items-start gap-2'>
                <span
                  aria-hidden='true'
                  className={cn(
                    'mt-0.5 grid size-5 shrink-0 place-items-center rounded-full border text-[10px]',
                    done || active
                      ? 'border-[var(--inf-brand)] text-[var(--inf-brand-light)]'
                      : 'border-[var(--inf-line-2)] text-[var(--inf-txt-3)]'
                  )}
                >
                  {done ? <Check className='size-3' /> : null}
                  {active ? <Spinner className='size-3' /> : null}
                  {done || active ? null : index + 1}
                </span>
                <div className='min-w-0'>
                  <p
                    className={cn(
                      'text-xs font-medium',
                      done || active
                        ? 'text-[var(--inf-txt)]'
                        : 'text-[var(--inf-txt-3)]'
                    )}
                  >
                    {step.title}
                  </p>
                  {active ? (
                    <p className='mt-0.5 animate-pulse text-xs text-[var(--inf-txt-2)]'>
                      {step.detail}
                    </p>
                  ) : null}
                </div>
              </li>
            )
          })}
        </ol>
        <p className='text-xs text-[var(--inf-txt-3)]'>
          {t('Keep this page open')}
        </p>
      </div>
    </PanelCard>
  )
}
