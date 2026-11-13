import { describe, expect, test } from 'vitest'

import { resolveAuthRedirect } from '../auth-redirect'

const origin = 'https://omnimux.ai'

describe('login destination', () => {
  test.each([
    undefined,
    '',
    'https://outside.example/path',
    '//outside.example',
  ])(
    'returns to the homepage for absent or unsafe target %s',
    (target) => {
      expect(resolveAuthRedirect(target, origin)).toBe('/')
    }
  )

  test.each([
    '/dashboard/overview',
    '/cli/login?code=example',
    '/influencer?tab=history#recent',
  ])('preserves an explicitly requested internal destination %s', (target) => {
    expect(resolveAuthRedirect(target, origin)).toBe(target)
  })

  test('normalizes an explicit same-origin URL', () => {
    expect(
      resolveAuthRedirect('https://omnimux.ai/influencer?tab=history', origin)
    ).toBe('/influencer?tab=history')
  })
})
