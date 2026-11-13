import type { Rspack } from '@rsbuild/core'
import { inferFullPath, type GeneratorPlugin } from '@tanstack/router-generator'

import { legacyRoutePaths } from '../../lib/legacy-route'

export interface AppRouteManifest {
  version: 1
  paths: string[]
  legacy: typeof legacyRoutePaths
}

/** Only route shapes supported by the Go matcher may enter a release. */
export function createAppRouteManifest(paths: string[]): AppRouteManifest {
  if (paths.length === 0) throw new Error('Application route tree is empty')
  const normalized = paths.map((routePath) => {
    if (
      !routePath.startsWith('/') ||
      (routePath !== '/' &&
        !routePath
          .replace(/\/$/, '')
          .slice(1)
          .split('/')
          .every((segment) =>
            /^(?:[A-Za-z0-9._~-]+|\$[A-Za-z_][A-Za-z0-9_]*)$/.test(segment)
          ))
    ) {
      throw new Error(`Unsupported application route shape: ${routePath}`)
    }
    return routePath === '/' ? routePath : routePath.replace(/\/$/, '')
  })
  if (!normalized.includes('/')) {
    throw new Error('Application route tree has no homepage')
  }
  return {
    version: 1,
    paths: [...new Set(normalized)].sort(),
    legacy: legacyRoutePaths,
  }
}

// fork: export effective generated paths, never a second hand-maintained route list.
export function appRouteManifestBuild(): {
  generator: GeneratorPlugin
  rspack: Rspack.Plugin
} {
  let manifest: AppRouteManifest | undefined
  return {
    generator: {
      name: 'omnimux-app-route-manifest',
      onRouteTreeChanged({ routeNodes }) {
        manifest = undefined
        // A pathless route-group layout can have an empty inferred full path.
        manifest = createAppRouteManifest(
          routeNodes.map(inferFullPath).filter(Boolean)
        )
      },
    },
    rspack: {
      apply(compiler: Rspack.Compiler) {
        compiler.hooks.thisCompilation.tap(
          'OmniMuxAppRoutes',
          (compilation) => {
            compilation.hooks.processAssets.tap(
              {
                name: 'OmniMuxAppRoutes',
                stage:
                  compiler.webpack.Compilation.PROCESS_ASSETS_STAGE_ADDITIONAL,
              },
              () => {
                if (!manifest) {
                  throw new Error(
                    'Application route manifest was not generated'
                  )
                }
                compilation.emitAsset(
                  'app-routes.json',
                  new compiler.webpack.sources.RawSource(
                    JSON.stringify(manifest)
                  )
                )
              }
            )
          }
        )
      },
    },
  }
}
