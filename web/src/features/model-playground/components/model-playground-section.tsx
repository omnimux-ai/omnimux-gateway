import type { TFunction } from 'i18next'
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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { formatCredits, usdToCredits } from '@/features/pricing/lib/credits'
import { getConfiguredGroupRatio } from '@/features/pricing/lib/model-helpers'
import { priceLadder } from '@/features/pricing/lib/model-page-pricing'
import type { PricingModel } from '@/features/pricing/types'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { useGeneration } from '../hooks/use-generation'
import { usePlaygroundHistory } from '../hooks/use-playground-history'
import {
  intentSuffix,
  needsIntentHint,
  promptHasIntent,
} from '../lib/intent-precheck'
import {
  acceptTypes,
  referenceLimitsFrom,
  screenMediaSelection,
  validateOperationMedia,
  type MediaSelectionIssue,
} from '../lib/media-validation'
import { operationMedia, operationsFor } from '../lib/operation-matrix'
import {
  buildOperationSchema,
  buildParamSchema,
  defaultParams,
} from '../lib/param-schema'
import { estimateQuote } from '../lib/quote'
import { toSubmitBody } from '../lib/submit-body'
import { tierGroupForResolution } from '../lib/tier-group'
import { getGenerationTransport } from '../transport'
import type {
  GenerationTask,
  PlaygroundOperation,
  PlaygroundParams,
} from '../types'
import { PlaygroundForm } from './playground-form'
import { PlaygroundHistory } from './playground-history'
import {
  PlaygroundModeSelector,
  type ModePriceLine,
} from './playground-mode-selector'
import { PlaygroundPreview } from './playground-preview'

/**
 * The slot's refusal as one sentence.
 *
 * A helper rather than a nested ternary in the markup: three reasons need three
 * sentences, and the count one needs the model's own documented ceiling.
 */
function selectionIssueMessage(
  issue: MediaSelectionIssue,
  t: TFunction
): string {
  if (issue.code === 'too_many') {
    return t(
      'This model accepts {{count}} reference file(s); the rest were skipped.',
      { count: issue.limit }
    )
  }
  if (issue.code === 'too_large') {
    return t('Each reference file must be under {{size}}MB; it was skipped.', {
      size: Math.round(issue.limitBytes / (1024 * 1024)),
    })
  }
  return t('Unsupported file type {{type}}; it was skipped.', {
    type: issue.mimeType,
  })
}

export type ModelPlaygroundSectionProps = {
  model: PricingModel
  /** Drives the submit label; unauthenticated users see "登录后生成". */
  authenticated: boolean
  /** Optional: invoked instead of submitting when the visitor is signed out. */
  onRequestSignIn?: () => void
  /** Optional: pre-populated prompt from inspiration community or external link. */
  initialPrompt?: string
  /** Selected group/tier */
  selectedGroup?: string
  /** Configured group ratios */
  groupRatio?: Record<string, number>
}

function formatUsd(usd: number): string {
  if (usd === 0) return '0'
  if (usd < 0.01) return usd.toFixed(4)
  return usd.toFixed(3)
}

function promptOf(params: PlaygroundParams): string {
  return typeof params.prompt === 'string' ? params.prompt : ''
}

export function ModelPlaygroundSection(props: ModelPlaygroundSectionProps) {
  const { t } = useTranslation()
  const transport = getGenerationTransport()
  const modelId = props.model.model_name
  const operations = useMemo(() => operationsFor(modelId), [modelId])

  const [operation, setOperation] = useState<PlaygroundOperation | undefined>(
    () => operations[0]
  )

  const schema = useMemo(
    () =>
      operation
        ? buildOperationSchema(props.model, operation)
        : buildParamSchema(props.model),
    [operation, props.model]
  )

  const [params, setParams] = useState<PlaygroundParams>(() => {
    const initial = defaultParams(schema)
    if (props.initialPrompt) {
      initial.prompt = props.initialPrompt
    }
    return initial
  })

  // The mode list belongs to the model, so a model change can invalidate the
  // chosen mode. Fall back to the new model's first mode and rebuild the form.
  // Adjusting during render (rather than in an effect) keeps the stale mode from
  // ever reaching the wire.
  const [lastModelId, setLastModelId] = useState(modelId)
  const droppedModeRef = useRef<PlaygroundOperation | undefined>(undefined)
  if (lastModelId !== modelId) {
    const dropped = operation !== undefined && !operations.includes(operation)
    setLastModelId(modelId)
    setOperation(operations[0])
    setParams(
      defaultParams(
        operations[0]
          ? buildOperationSchema(props.model, operations[0])
          : buildParamSchema(props.model)
      )
    )
    droppedModeRef.current = dropped ? operation : undefined
  }

  useEffect(() => {
    const dropped = droppedModeRef.current
    if (!dropped) return
    droppedModeRef.current = undefined
    toast.info(t('The previous mode is not available on this model.'))
  }, [modelId, t])

  useEffect(() => {
    const prompt = props.initialPrompt
    if (prompt) {
      setParams((previous) => ({ ...previous, prompt }))
      const el = document.querySelector('#playground')
      if (el) {
        el.scrollIntoView({ behavior: 'smooth' })
      }
    }
  }, [props.initialPrompt])

  const [mode, setMode] = useState<'form' | 'json'>('form')
  const [previewView, setPreviewView] = useState<'preview' | 'json'>('preview')
  const [uploading, setUploading] = useState(false)
  const [selectedTask, setSelectedTask] = useState<GenerationTask | null>(null)
  // The last selection the slot refused, so the reason survives the render that
  // follows a rejected pick.
  const [selectionIssue, setSelectionIssue] =
    useState<MediaSelectionIssue | null>(null)

  /**
   * The token group the request's resolution selects, for a model whose quality
   * tiers are groups. Undefined for every other model, and for a resolution the
   * model's provisioned tiers do not cover — the request then keeps whatever
   * group the page already selected.
   */
  const tierGroup = useMemo(
    () => tierGroupForResolution(props.model, params),
    [params, props.model]
  )

  const currentGroupRatio = useMemo(() => {
    const group = tierGroup ?? props.selectedGroup
    if (!group || !props.groupRatio) return 1
    return getConfiguredGroupRatio(props.groupRatio, group)
  }, [props.groupRatio, props.selectedGroup, tierGroup])

  const resolveEstimate = useCallback(
    (next: PlaygroundParams) => {
      const quote = estimateQuote(props.model, next, schema, currentGroupRatio)
      return quote ? quote.usd : null
    },
    [currentGroupRatio, props.model, schema]
  )

  const generation = useGeneration({ transport, resolveEstimate })
  const history = usePlaygroundHistory(
    transport,
    modelId,
    generation.task,
    props.authenticated
  )

  /**
   * Starting rate for the mode picker.
   *
   * Read from the same catalog ladder the pricing section renders, so the bar
   * can never quote a number the rest of the page does not. It is a starting
   * price, deliberately not the estimate for the configured request: that one
   * belongs to the footer.
   */
  const modePrice = useCallback((): ModePriceLine | null => {
    const entry = priceLadder(props.model, 1).at(0)
    if (!entry) return null
    const effectiveUsd = entry.usd * currentGroupRatio
    const unit =
      entry.label === 'per_request' ? t('per request') : t('per second')
    return {
      amount: `${t('from')} ~$${formatUsd(effectiveUsd)} / ${unit}`,
      credits: `${formatCredits(usdToCredits(effectiveUsd))} ${t('credits')}`,
    }
  }, [currentGroupRatio, props.model, t])

  const quote = useMemo(
    () => estimateQuote(props.model, params, schema, currentGroupRatio),
    [currentGroupRatio, params, props.model, schema]
  )

  // Resolved at paint time so the message follows a language switch. The server
  // reason is preferred; the fallback only covers a failure that carries none.
  const submitError = generation.error
    ? getServerErrorMessage(
        generation.error,
        t('Generation failed. Please try again.')
      )
    : null

  const onParamChange = useCallback(
    (key: string, value: string | number | boolean) => {
      setParams((previous) => ({ ...previous, [key]: value }))
    },
    []
  )

  const onMediaFiles = useCallback(
    (key: string, files: FileList | null) => {
      if (!files?.length) return
      const control = schema.controls.find((entry) => entry.key === key)
      const existing = Array.isArray(params[key])
        ? (params[key] as string[]).length
        : 0
      // The slot refuses what the model cannot take before anything is
      // uploaded: the gateway precharges, so a rejected file must never leave
      // the browser.
      const { accepted, issue } = screenMediaSelection(
        {
          accept: acceptTypes(control?.accept),
          maxCount: control?.maxCount,
          maxBytes: control?.maxBytes,
        },
        existing,
        [...files]
      )
      setSelectionIssue(issue)
      if (accepted.length === 0) return

      setUploading(true)
      void Promise.all(
        accepted.map((file) =>
          transport.uploadReference(file).then((result) => result.url)
        )
      )
        .then((urls) => {
          setParams((previous) => {
            const current = Array.isArray(previous[key])
              ? (previous[key] as string[])
              : []
            return { ...previous, [key]: [...current, ...urls] }
          })
        })
        .finally(() => setUploading(false))
    },
    [params, schema, transport]
  )

  const onMediaRemove = useCallback((key: string, url: string) => {
    setParams((previous) => {
      const current = Array.isArray(previous[key])
        ? (previous[key] as string[])
        : []
      return { ...previous, [key]: current.filter((item) => item !== url) }
    })
  }, [])

  /**
   * Mode switch keeps the prompt and drops every media slot.
   *
   * The two media families are mutually exclusive upstream, so carrying an
   * upload across a switch guarantees a rejected request; the prompt is the one
   * thing that means the same in every mode.
   */
  const onOperationChange = useCallback(
    (next: PlaygroundOperation) => {
      setOperation(next)
      const nextSchema = buildOperationSchema(props.model, next)
      setParams((previous) => {
        const rebuilt = defaultParams(nextSchema)
        const prompt = promptOf(previous)
        if (prompt) rebuilt.prompt = prompt
        return rebuilt
      })
      setSelectedTask(null)
      generation.reset()
    },
    [generation, props.model]
  )

  const reset = useCallback(() => {
    setParams(defaultParams(schema))
    setSelectedTask(null)
    generation.reset()
  }, [generation, schema])

  // The model's declared reference limits, checked here so an over-full set is
  // refused by the form instead of by the gateway after submission.
  const mediaIssue = operation
    ? validateOperationMedia(
        operation,
        params,
        referenceLimitsFrom(props.model.contract)
      )
    : null

  const intentMissing =
    operation !== undefined &&
    needsIntentHint(operation) &&
    !promptHasIntent(operation, promptOf(params))

  const completePrompt = useCallback(() => {
    if (!operation) return
    const suffix = intentSuffix(operation)
    if (!suffix) return
    setParams((previous) => {
      const prompt = promptOf(previous)
      if (prompt.startsWith(suffix)) return previous
      return { ...previous, prompt: `${suffix}${prompt}` }
    })
  }, [operation])

  const buildBody = useCallback(() => {
    const body = toSubmitBody(modelId, operation, schema, params)
    // A resolved tier group travels as `X-Omnimux-Group`, so the body never
    // carries a second group that could disagree with the resolution.
    if (
      !tierGroup &&
      props.selectedGroup &&
      props.selectedGroup !== 'default'
    ) {
      body.group = props.selectedGroup
    }
    return body
  }, [modelId, operation, params, props.selectedGroup, schema, tierGroup])

  const submit = useCallback(() => {
    if (!props.authenticated) {
      props.onRequestSignIn?.()
      return
    }
    setSelectedTask(null)
    const body = buildBody()
    const targetModel =
      typeof body.model === 'string' && body.model ? body.model : modelId
    void generation.submit(targetModel, body, schema.modality, tierGroup)
  }, [buildBody, generation, modelId, props, schema.modality, tierGroup])

  const onSelectHistoryTask = useCallback((task: GenerationTask) => {
    setSelectedTask(task)
    setPreviewView('preview')
  }, [])

  // When no task is currently running and user hasn't explicitly selected one,
  // default to previewing the latest completed task with media from history.
  const latestCompletedHistoryTask = useMemo(() => {
    if (!history.tasks || history.tasks.length === 0) return null
    return (
      history.tasks.find(
        (t) =>
          t.status === 'succeeded' &&
          Array.isArray(t.media) &&
          t.media.length > 0
      ) ??
      history.tasks[0] ??
      null
    )
  }, [history.tasks])

  const activeTask =
    generation.task ?? selectedTask ?? latestCompletedHistoryTask
  const previewParams = activeTask ? activeTask.params : params

  const jsonText = useMemo(() => {
    const body = buildBody()
    const displayModel =
      typeof body.model === 'string' && body.model ? body.model : modelId
    return JSON.stringify({ model: displayModel, ...body }, null, 2)
  }, [buildBody, modelId])

  const applyJson = useCallback((text: string) => {
    try {
      const parsed = JSON.parse(text) as Record<string, unknown>
      setParams((previous) => {
        const next = { ...previous }
        for (const [key, value] of Object.entries(parsed)) {
          if (key === 'model' || key === 'operation') continue
          if (
            typeof value === 'string' ||
            typeof value === 'number' ||
            typeof value === 'boolean' ||
            Array.isArray(value)
          ) {
            next[key] = value as string | number | boolean | string[]
          }
        }
        return next
      })
    } catch {
      // A partially typed document must not clobber the form parameters.
    }
  }, [])

  let submitLabel = t('Sign in to generate')
  if (props.authenticated) {
    submitLabel = generation.submitting ? t('Generating…') : t('Generate')
  }

  const blockedReason = useMemo(() => {
    if (!mediaIssue) return null
    if (mediaIssue.code === 'audio_alone') {
      return t('Reference audio needs at least one reference image or video.')
    }
    const spec = operationMedia(operation as PlaygroundOperation).find(
      (item) => item.key === mediaIssue.slotKey
    )
    return t('Add the required input: {{slot}}', {
      slot: t(spec?.labelKey ?? 'Input'),
    })
  }, [mediaIssue, operation, t])

  return (
    <section id='playground' className='scroll-mt-[176px] space-y-6'>
      {/* Lead-in spans the full content width, as on the benchmark page. */}
      <div className='space-y-2'>
        <h2
          data-parity='playground.intro.title'
          className='text-[32px] leading-[1.25] font-bold tracking-[-0.02em]'
        >
          {t('Before integrating the API, try {{name}} first.', {
            name: props.model.model_name,
          })}
        </h2>
        <p
          data-parity='playground.intro.desc'
          className='text-muted-foreground text-sm leading-[1.7]'
        >
          {t(
            'Test {{name}} output quality, estimate credit usage, and generate one sample before going to production.',
            { name: props.model.model_name }
          )}
        </p>
      </div>

      {/*
        Benchmark proportions: a narrow fixed form column beside a much wider
        preview/history column (measured 428px vs 766px inside a 1216px content
        width), not an even split.
      */}
      <div className='grid gap-5 lg:grid-cols-[428px_minmax(0,1fr)]'>
        <div className='space-y-3'>
          {/*
            The mode picker leads the column. The lead-in above the grid already
            states what the panel is for, so a repeated title plus helper line
            only pushed the controls down.
          */}
          <div className='space-y-4 rounded-(--radius-media) border p-4'>
            {operations.length > 0 && (
              <PlaygroundModeSelector
                operations={operations}
                value={operation ?? operations[0]}
                disabled={generation.submitting}
                priceFor={modePrice}
                onChange={onOperationChange}
              />
            )}

            <ToggleGroup
              data-parity='playground.mode.switch'
              value={[mode]}
              onValueChange={(values) => {
                const next = values.at(0)
                if (next === 'form' || next === 'json') setMode(next)
              }}
              variant='outline'
              size='sm'
              className='bg-muted/40 w-fit rounded-lg border p-0.5'
              aria-label={t('Input mode')}
            >
              <ToggleGroupItem value='form'>{t('Form mode')}</ToggleGroupItem>
              <ToggleGroupItem value='json'>{t('JSON mode')}</ToggleGroupItem>
            </ToggleGroup>

            {mode === 'form' ? (
              <PlaygroundForm
                schema={schema}
                params={params}
                uploading={uploading}
                disabled={generation.submitting}
                onParamChange={onParamChange}
                onMediaFiles={onMediaFiles}
                onMediaRemove={onMediaRemove}
              />
            ) : (
              <Textarea
                value={jsonText}
                rows={18}
                spellCheck={false}
                className='font-mono text-[12px]'
                aria-label={t('Request JSON')}
                onChange={(event) => applyJson(event.target.value)}
              />
            )}

            {blockedReason && (
              <p
                data-parity='playground.form.blocked'
                role='status'
                className='text-muted-foreground text-[12px] leading-5'
              >
                {blockedReason}
              </p>
            )}

            {selectionIssue && (
              <p
                data-parity='playground.form.selection-issue'
                role='alert'
                className='text-muted-foreground text-[12px] leading-5'
              >
                {selectionIssueMessage(selectionIssue, t)}
              </p>
            )}

            {intentMissing && (
              <div
                data-parity='playground.form.intent-hint'
                role='status'
                className='flex flex-wrap items-center gap-2'
              >
                <p className='text-muted-foreground text-[12px] leading-5'>
                  {t(
                    'This prompt does not state what to edit or how to continue, so the model may reject the task after it is queued.'
                  )}
                </p>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='h-7 rounded-full px-3 text-[12px]'
                  onClick={completePrompt}
                >
                  {t('Complete the prompt')}
                </Button>
              </div>
            )}

            {/*
              Benchmark behaviour: Reset and submit stick to the bottom of the
              viewport while the parameters scroll, so the primary action stays
              reachable. It sits last in the card so it never covers a field.
            */}
            <div className='bg-background/95 sticky bottom-0 z-10 -mx-4 -mb-4 flex flex-wrap items-center justify-between gap-2.5 rounded-b-(--radius-media) border-t px-4 pt-3 pb-4 backdrop-blur'>
              {submitError ? (
                <p
                  data-parity='playground.footer.error'
                  role='alert'
                  className='text-destructive basis-full text-[13px] leading-5'
                >
                  {submitError}
                </p>
              ) : null}
              <p
                data-parity='playground.footer.quote'
                className='text-[13px] tabular-nums'
              >
                {quote
                  ? `${t('Estimate')}: ~$${formatUsd(quote.usd)} · ${formatCredits(quote.credits)} ${t('credits')}`
                  : `${t('Estimate')}: —`}
              </p>
              <div className='flex items-center gap-2'>
                <Button
                  data-parity='playground.footer.reset'
                  variant='outline'
                  className='h-9 rounded-full px-4 text-[13px] font-medium'
                  onClick={reset}
                >
                  {t('Reset')}
                </Button>
                <Button
                  data-parity='playground.footer.submit'
                  className='bg-foreground text-background hover:bg-foreground/90 h-10 rounded-full px-[22px] text-sm font-semibold'
                  disabled={generation.submitting || mediaIssue !== null}
                  onClick={submit}
                >
                  {submitLabel}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <div className='space-y-6'>
          <PlaygroundPreview
            task={activeTask}
            params={previewParams}
            view={previewView}
            onViewChange={setPreviewView}
          />
          <PlaygroundHistory
            tasks={history.tasks}
            loading={history.loading}
            selectedTaskId={activeTask?.taskId ?? null}
            onSelectTask={onSelectHistoryTask}
          />
        </div>
      </div>
    </section>
  )
}
