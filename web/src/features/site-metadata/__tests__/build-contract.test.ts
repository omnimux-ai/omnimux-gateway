import { describe, expect, it } from 'vitest'

import { assertGuideDocuments, assertSearchPageShell } from '../build-contract'

const pageHead =
  '<!--omnimux-page-head--><title>OmniMux</title><!--/omnimux-page-head-->'

describe('built search page shell', () => {
  it('refuses missing tutorial documents and noscript-only shells', () => {
    expect(() => assertGuideDocuments({})).toThrow(
      'Missing complete guide document'
    )
    expect(() =>
      assertGuideDocuments({
        '/guides': {
          documentHtml:
            '<!doctype html><html><body><noscript><main><h1>Only fallback</h1></main></noscript></body></html>',
        },
      })
    ).toThrow()
  })
  it('accepts one replaceable page title after asset injection', () => {
    expect(() =>
      assertSearchPageShell(
        `<html><head>${pageHead}<script src="/app.js"></script></head></html>`
      )
    ).not.toThrow()
  })

  it('rejects an extra title injected by the bundler outside the page-head block', () => {
    expect(() =>
      assertSearchPageShell(
        `<html><head>${pageHead}<title>Rsbuild App</title></head></html>`
      )
    ).toThrow('exactly one title')
  })

  it('rejects a missing title or a title outside the replacement block', () => {
    expect(() =>
      assertSearchPageShell(
        '<head><!--omnimux-page-head--><!--/omnimux-page-head--></head>'
      )
    ).toThrow()
    expect(() =>
      assertSearchPageShell(
        '<head><title>Wrong</title><!--omnimux-page-head--><!--/omnimux-page-head--></head>'
      )
    ).toThrow()
  })
})
