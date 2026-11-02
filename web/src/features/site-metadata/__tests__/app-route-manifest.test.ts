import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { inferFullPath } from '@tanstack/router-generator'
import { describe, expect, it } from 'vitest'

import { legacyRoutePaths, resolveLegacyRoute } from '../../../lib/legacy-route'
import { createAppRouteManifest } from '../app-route-manifest'

describe('application route manifest', () => {
  it('uses effective paths for route groups, pathless layouts and dynamic pages', () => {
    const generatedPaths = [
      { routePath: '/' },
      { routePath: '/(auth)/sign-in' },
      { routePath: '/_authenticated/dashboard/' },
      { routePath: '/pricing/$modelId/' },
      { routePath: '/oauth/$provider' },
      { routePath: '/_authenticated/dashboard' },
    ].map((node) =>
      inferFullPath({
        ...node,
        filePath: '',
        fullPath: '',
        variableName: 'testRoute',
        _fsRouteType: 'static',
      })
    )
    expect(createAppRouteManifest(generatedPaths)).toEqual({
      version: 1,
      paths: [
        '/',
        '/dashboard',
        '/oauth/$provider',
        '/pricing/$modelId',
        '/sign-in',
      ],
      legacy: legacyRoutePaths,
    })
  })

  it.each([
    '/s/$',
    '/s/*',
    '/s/{-$shareId}',
    '/s/prefix{$shareId}',
    '/s/$shareId.json',
    '/s//detail',
    'relative',
  ])(
    'blocks unsupported route shape %s instead of broadening server matches',
    (path) => {
      expect(() => createAppRouteManifest(['/', path])).toThrow(
        'Unsupported application route shape'
      )
    }
  )

  it('blocks empty or incomplete route trees', () => {
    expect(() => createAppRouteManifest([])).toThrow('route tree is empty')
    expect(() => createAppRouteManifest(['/dashboard'])).toThrow('no homepage')
  })

  it('keeps the browser matching policy mirrored by the Go route matcher', () => {
    const root = createRootRoute()
    root.addChildren([
      createRoute({ getParentRoute: () => root, path: '/influencer' }),
      createRoute({ getParentRoute: () => root, path: '/pricing' }),
      createRoute({ getParentRoute: () => root, path: '/pricing/$modelId' }),
    ])
    const router = createRouter({
      routeTree: root,
      history: createMemoryHistory({ initialEntries: ['/'] }),
    })
    for (const path of [
      '/INFLUENCER/',
      '/influencer///',
      '/pricing/vendor%2Fmodel',
    ]) {
      expect(
        router.matchRoutes(path).some((match) => match.globalNotFound),
        path
      ).toBe(false)
    }
    for (const path of [
      '/%69nfluencer',
      '/%70ricing',
      '/pricing//model',
      '/influencer/extra',
    ]) {
      expect(
        router.matchRoutes(path).some((match) => match.globalNotFound),
        path
      ).toBe(true)
    }
  })

  it('derives legacy entries from the same policy that resolves incoming links', () => {
    for (const path of legacyRoutePaths.exactPaths) {
      expect(resolveLegacyRoute(path)).not.toBeNull()
    }
    for (const prefix of legacyRoutePaths.prefixPaths) {
      expect(resolveLegacyRoute(`${prefix}unknown-historical-entry`)).toBe(
        '/dashboard'
      )
    }
    expect(resolveLegacyRoute('/LOGIN')).toBeNull()
    expect(resolveLegacyRoute('/console2/unknown')).toBeNull()
  })
})
