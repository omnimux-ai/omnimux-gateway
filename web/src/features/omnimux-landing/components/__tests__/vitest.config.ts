import { defineConfig } from 'vitest/config'

import baseConfig from '../../../../../vitest.config'

export default defineConfig({
  ...baseConfig,
  test: {
    ...baseConfig.test,
    include: [
      'src/features/omnimux-landing/components/__tests__/landing-start.test.tsx',
      'src/features/omnimux-landing/components/__tests__/landing-nav-inspiration.test.tsx',
      'src/features/omnimux-landing/components/__tests__/landing-influencer.test.tsx',
      'src/features/omnimux-landing/components/__tests__/landing-nav-tools.test.tsx',
      'src/features/omnimux-landing/components/__tests__/landing-nav-resources.test.tsx',
    ],
    exclude: [],
  },
})
