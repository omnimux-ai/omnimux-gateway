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
import { describe, expect, test } from 'vitest'

import type { BillingUsageSchema, PricingModel } from '@/features/pricing/types'

import {
  buildParamSchema,
  defaultParams,
  resolveModality,
  submitParams,
} from '../param-schema'

function model(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

const durationSchema: BillingUsageSchema = {
  duration: { type: 'number', unit: 'second' },
}

describe('resolveModality', () => {
  test('prefers the declared output modality over the tags', () => {
    const resolved = resolveModality(
      model({ output_modalities: ['text', 'video'], tags: 'Image Generation' })
    )
    expect(resolved).toBe('video')
  })

  test('falls back to the Video Generation tag when output modalities are absent', () => {
    expect(
      resolveModality(model({ tags: 'Video Generation,async,task' }))
    ).toBe('video')
  })

  test('falls back to the Image Generation tag when output modalities are absent', () => {
    expect(resolveModality(model({ tags: 'Image Generation' }))).toBe('image')
  })

  test('defaults to image when the catalog carries no modality signal', () => {
    expect(resolveModality(model({}))).toBe('image')
  })
})

describe('buildParamSchema', () => {
  test('orders image controls prompt → size mode → ratio → resolution → quality', () => {
    const schema = buildParamSchema(model({ tags: 'Image Generation' }))
    const keys = schema.controls.map((control) => control.key)
    expect(keys.indexOf('prompt')).toBe(0)
    expect(keys.indexOf('size_mode')).toBeLessThan(keys.indexOf('aspect_ratio'))
    expect(keys.indexOf('aspect_ratio')).toBeLessThan(
      keys.indexOf('resolution')
    )
    expect(keys.indexOf('resolution')).toBeLessThan(keys.indexOf('quality'))
  })

  test('counts advanced controls so the collapsed group can show its size', () => {
    const image = buildParamSchema(model({ tags: 'Image Generation' }))
    const video = buildParamSchema(model({ tags: 'Video Generation' }))
    expect(image.advancedCount).toBe(2)
    expect(video.advancedCount).toBe(1)
  })

  test('adds a reference media control only for models that accept non-text input', () => {
    const taskModel = buildParamSchema(
      model({
        tags: 'Video Generation,async,task',
        input_modalities: ['text', 'image'],
      })
    )
    const plainModel = buildParamSchema(
      model({ tags: 'Image Generation', input_modalities: ['text'] })
    )
    expect(taskModel.controls.some((c) => c.key === 'reference_media')).toBe(
      true
    )
    expect(plainModel.controls.some((c) => c.key === 'reference_media')).toBe(
      false
    )
  })

  test('derives video duration options from the declared tier bounds', () => {
    const schema = buildParamSchema(
      model({
        model_name: 'seedance-2-5',
        tags: 'Video Generation',
        billing_usage_schema: durationSchema,
      })
    )
    const duration = schema.controls.find((c) => c.key === 'duration')
    expect(duration?.options?.map((option) => option.value)).toEqual([
      '4',
      '5',
      '6',
      '8',
      '10',
      '12',
      '15',
      '30',
    ])
  })

  test('offers only the provisioned resolution tiers for wan-3.0, and no quality tier', () => {
    const schema = buildParamSchema(
      model({
        model_name: 'wan-3.0',
        tags: 'Video Generation,async,task',
        enable_groups: ['wan-3.0-720p', 'wan-3.0-1080p'],
      })
    )

    expect(schema.controls.some((c) => c.key === 'quality_tier')).toBe(false)
    const resolution = schema.controls.find((c) => c.key === 'resolution')
    expect(resolution?.options?.map((option) => option.value)).toEqual([
      '720p',
      '1080p',
    ])
    expect(resolution?.defaultValue).toBe('720p')
  })

  test('keeps the declared resolutions while the wan-3.0 tiers are unprovisioned', () => {
    const schema = buildParamSchema(
      model({
        model_name: 'wan-3.0',
        tags: 'Video Generation,async,task',
        enable_groups: ['default'],
        contract: {
          model_id: 'wan-3.0',
          family: 'video',
          resolutions: ['480P', '720P', '1080P'],
        },
      })
    )

    const resolution = schema.controls.find((c) => c.key === 'resolution')
    expect(resolution?.options?.map((option) => option.value)).toEqual([
      '480P',
      '720P',
      '1080P',
    ])
  })

  test('keeps every derived parameter at its declared default', () => {
    const schema = buildParamSchema(model({ tags: 'Image Generation' }))
    const params = defaultParams(schema)
    for (const control of schema.controls) {
      expect(params[control.key]).toEqual(control.defaultValue)
    }
  })
})

describe('submitParams', () => {
  const videoSchema = () =>
    buildParamSchema(
      model({
        model_name: 'seedance-2-5',
        tags: 'Video Generation',
        billing_usage_schema: durationSchema,
      })
    )

  test('sends the duration control as a number so the gateway accepts the body', () => {
    const schema = videoSchema()
    const params = defaultParams(schema)

    // The form holds the option value as a string because it doubles as an i18n
    // key; the submit body must still carry a JSON number.
    expect(typeof params.duration).toBe('string')

    const shaped = submitParams(schema, params)
    expect(typeof shaped.duration).toBe('number')
    expect(shaped.duration).toBe(Number(params.duration))
  })

  test('leaves string-typed controls untouched', () => {
    const schema = videoSchema()
    const shaped = submitParams(schema, {
      ...defaultParams(schema),
      resolution: '480p',
    })

    expect(shaped.resolution).toBe('480p')
  })

  test('sends the vendor term for the automatic aspect ratio', () => {
    const schema = videoSchema()
    const params = defaultParams(schema)

    // The chip reads "auto"; the video providers only accept `adaptive`.
    expect(params.aspect_ratio).toBe('auto')
    expect(submitParams(schema, params).aspect_ratio).toBe('adaptive')
  })
})

describe('buildParamSchema for a per-call video model', () => {
  const perCall = (modelName: string) =>
    model({ model_name: modelName, tags: 'Video Generation,task' })

  const PER_CALL_IDS = [
    'seedance-2-5-task',
    'seedance-2-0-task',
    'seedance-2-0-fast-task',
    'minimax-h3-task',
    'gemini-omni-1.1-flash-task',
    'grok-imagine-video-1-5-task',
  ]

  // The envelope now reaches the form through the contract rather than the
  // register, so these carry the contract the catalog publishes.
  const published = {
    model_id: 'seedance-2-5-task',
    family: 'video' as const,
    durations_seconds: { allowed: [30] },
    resolutions: ['720p'],
    aspect_ratios: ['16:9', '9:16'],
  }

  test('pins a declared fixed length to a single locked value instead of a ladder', () => {
    const schema = buildParamSchema({
      ...perCall('seedance-2-5-task'),
      contract: published,
    })
    const duration = schema.controls.find(
      (control) => control.key === 'duration'
    )

    expect(duration?.options?.map((option) => option.value)).toEqual(['30'])
    expect(duration?.lockedReasonKey).toBeTruthy()
  })

  test('never renders the per-second billing claim for any per-call model', () => {
    for (const id of PER_CALL_IDS) {
      const schema = buildParamSchema(perCall(id))
      for (const control of schema.controls) {
        expect(control.hintKey, id).not.toBe(
          'Duration is billed per output second.'
        )
      }
    }
  })

  test('offers no seconds adjuster to any per-call model', () => {
    for (const id of PER_CALL_IDS) {
      const duration = buildParamSchema(perCall(id)).controls.find(
        (control) => control.key === 'duration'
      )
      expect(duration?.options?.length ?? 0, id).toBeLessThan(2)
    }
  })

  test('omits the length control when the envelope is unestablished', () => {
    const schema = buildParamSchema(perCall('gemini-omni-1.1-flash-task'))
    expect(schema.controls.some((control) => control.key === 'duration')).toBe(
      false
    )
  })

  test('narrows a per-call model to the resolutions and ratios its line accepts', () => {
    const schema = buildParamSchema({
      ...perCall('seedance-2-5-task'),
      contract: published,
    })
    const resolution = schema.controls.find(
      (control) => control.key === 'resolution'
    )
    const ratio = schema.controls.find(
      (control) => control.key === 'aspect_ratio'
    )

    expect(resolution?.options?.map((option) => option.value)).toEqual(['720p'])
    expect(ratio?.options?.map((option) => option.value)).toEqual([
      '16:9',
      '9:16',
    ])
  })

  test('sends the declared fixed length as a number', () => {
    const schema = buildParamSchema({
      ...perCall('seedance-2-5-task'),
      contract: published,
    })
    expect(submitParams(schema, defaultParams(schema)).duration).toBe(30)
  })

  test('sends no length at all when the envelope is unestablished', () => {
    const schema = buildParamSchema(perCall('grok-imagine-video-1-5-task'))
    expect('duration' in submitParams(schema, defaultParams(schema))).toBe(
      false
    )
  })

  test('leaves a per-second model its seconds ladder and its per-second claim', () => {
    const schema = buildParamSchema(
      model({ model_name: 'seedance-2-5', tags: 'Video Generation' })
    )
    const duration = schema.controls.find(
      (control) => control.key === 'duration'
    )

    expect(duration?.options?.length).toBeGreaterThan(1)
    expect(duration?.hintKey).toBe('Duration is billed per output second.')
    expect(duration?.lockedReasonKey).toBeUndefined()
  })
})

describe('buildParamSchema with a published contract', () => {
  const contracted = (contract: PricingModel['contract']) =>
    model({
      model_name: 'some-video-model',
      tags: 'Video Generation',
      contract,
    })

  test('takes the declared resolutions instead of the modality default', () => {
    const schema = buildParamSchema(
      contracted({ model_id: 'm', family: 'video', resolutions: ['720p'] })
    )
    const resolution = schema.controls.find((c) => c.key === 'resolution')
    expect(resolution?.options?.map((option) => option.value)).toEqual(['720p'])
  })

  test('takes the declared aspect ratios instead of the modality default', () => {
    const schema = buildParamSchema(
      contracted({
        model_id: 'm',
        family: 'video',
        aspect_ratios: ['16:9', '9:16'],
      })
    )
    const ratio = schema.controls.find((c) => c.key === 'aspect_ratio')
    expect(ratio?.options?.map((option) => option.value)).toEqual([
      '16:9',
      '9:16',
    ])
  })

  test('takes the declared output formats instead of the modality default', () => {
    const schema = buildParamSchema(
      contracted({ model_id: 'm', family: 'video', output_formats: ['mp4'] })
    )
    const format = schema.controls.find((c) => c.key === 'output_format')
    expect(format?.options?.map((option) => option.value)).toEqual(['mp4'])
  })

  test('keeps the modality defaults when the contract declares no such axis', () => {
    const schema = buildParamSchema(
      contracted({ model_id: 'm', family: 'video' })
    )
    const resolution = schema.controls.find((c) => c.key === 'resolution')
    const format = schema.controls.find((c) => c.key === 'output_format')
    expect(resolution?.options?.map((option) => option.value)).toEqual([
      '480p',
      '720p',
      '1080p',
    ])
    expect(format?.options?.map((option) => option.value)).toEqual([
      'mp4',
      'mov',
    ])
  })

  test('keeps the modality defaults when no contract was published', () => {
    const schema = buildParamSchema(
      model({ model_name: 'some-video-model', tags: 'Video Generation' })
    )
    const resolution = schema.controls.find((c) => c.key === 'resolution')
    expect(resolution?.options?.map((option) => option.value)).toEqual([
      '480p',
      '720p',
      '1080p',
    ])
  })
})

describe('buildParamSchema with a declared duration or audio envelope', () => {
  const contracted = (contract: PricingModel['contract']) =>
    model({
      model_name: 'some-video-model',
      tags: 'Video Generation',
      contract,
    })

  const valuesOf = (schema: ReturnType<typeof buildParamSchema>, key: string) =>
    schema.controls.find((c) => c.key === key)?.options?.map((o) => o.value)

  test('takes the declared duration range instead of the modality ladder', () => {
    const schema = buildParamSchema(
      contracted({
        model_id: 'm',
        family: 'video',
        durations_seconds: { min: 5, max: 10 },
      })
    )
    expect(valuesOf(schema, 'duration')).toEqual(['5', '6', '8', '10'])
  })

  test('takes the declared allowed durations exactly', () => {
    const schema = buildParamSchema(
      contracted({
        model_id: 'm',
        family: 'video',
        durations_seconds: { allowed: [5, 10] },
      })
    )
    expect(valuesOf(schema, 'duration')).toEqual(['5', '10'])
  })

  test('withholds the audio switch from a model that declares no audio', () => {
    const schema = buildParamSchema(
      contracted({ model_id: 'm', family: 'video', audio_supported: false })
    )
    expect(schema.controls.some((c) => c.key === 'generate_audio')).toBe(false)
  })

  test('keeps the audio switch when audio is declared', () => {
    const schema = buildParamSchema(
      contracted({ model_id: 'm', family: 'video', audio_supported: true })
    )
    expect(schema.controls.some((c) => c.key === 'generate_audio')).toBe(true)
  })

  test('keeps the audio switch when the contract establishes no audio axis', () => {
    const schema = buildParamSchema(
      contracted({ model_id: 'm', family: 'video' })
    )
    expect(schema.controls.some((c) => c.key === 'generate_audio')).toBe(true)
  })
})

describe('per-call duration supplied by the contract', () => {
  // A per-call id with no register entry: the envelope can only come from the
  // contract, which is the point of this fallback.
  const unregistered = (contract: PricingModel['contract']) =>
    model({
      model_name: 'percall-unregistered-task',
      tags: 'Video Generation,task',
      contract,
    })

  test('locks a single declared length even without a register entry', () => {
    const schema = buildParamSchema(
      unregistered({
        model_id: 'm',
        family: 'video',
        durations_seconds: { allowed: [12] },
      })
    )
    const duration = schema.controls.find((c) => c.key === 'duration')
    expect(duration?.options?.map((o) => o.value)).toEqual(['12'])
    expect(duration?.lockedReasonKey).toBeTruthy()
  })

  test('sends the contract-supplied length as a number', () => {
    const schema = buildParamSchema(
      unregistered({
        model_id: 'm',
        family: 'video',
        durations_seconds: { allowed: [12] },
      })
    )
    expect(submitParams(schema, defaultParams(schema)).duration).toBe(12)
  })

  test('offers a chooser when the contract declares several lengths', () => {
    const schema = buildParamSchema(
      unregistered({
        model_id: 'm',
        family: 'video',
        durations_seconds: { allowed: [10, 15] },
      })
    )
    const duration = schema.controls.find((c) => c.key === 'duration')
    expect(duration?.options?.map((o) => o.value)).toEqual(['10', '15'])
    expect(duration?.lockedReasonKey).toBeUndefined()
  })
})

describe('the prompt control carries the declared length limit', () => {
  const withContract = (contract: PricingModel['contract']) =>
    model({
      model_name: 'some-video-model',
      tags: 'Video Generation',
      contract,
    })

  const promptControl = (subject: PricingModel) =>
    buildParamSchema(subject).controls.find(
      (control) => control.key === 'prompt'
    )

  test('uses the declared ceiling', () => {
    expect(
      promptControl(
        withContract({ model_id: 'm', family: 'video', prompt_max_chars: 2500 })
      )?.maxChars
    ).toBe(2500)
  })

  test('stays uncapped when no contract is published', () => {
    expect(
      promptControl(
        model({ model_name: 'some-video-model', tags: 'Video Generation' })
      )?.maxChars
    ).toBeUndefined()
  })

  test('stays uncapped when the contract declares no limit', () => {
    expect(
      promptControl(withContract({ model_id: 'm', family: 'video' }))?.maxChars
    ).toBeUndefined()
  })

  test('ignores a non-positive declaration', () => {
    expect(
      promptControl(
        withContract({ model_id: 'm', family: 'video', prompt_max_chars: 0 })
      )?.maxChars
    ).toBeUndefined()
  })
})
