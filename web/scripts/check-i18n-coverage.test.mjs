import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { afterEach, test } from 'vitest'

const script = path.resolve('scripts/check-i18n-coverage.mjs')
const fixtureRoots = []
afterEach(() => {
  for (const root of fixtureRoots.splice(0)) fs.rmSync(root, { recursive: true, force: true })
})
const locales = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi']
function check(
  _t,
  {
    source = "t('Hello'); t(flag ? 'Hello' : 'OpenAI'); t(dynamicKey)",
    change = () => {},
  } = {}
) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'omnimux-i18n-check-'))
  fixtureRoots.push(root)
  const dir = path.join(root, 'src/i18n/locales')
  fs.mkdirSync(dir, { recursive: true })
  for (const locale of locales) {
    const translation = {
      Hello: locale === 'en' ? 'Hello' : '你好',
      OpenAI: 'OpenAI',
      'Hi {{name}}': 'Hi {{name}}',
    }
    change(translation, locale)
    // Fixture writes never touch the real project locale files.
    fs.writeFileSync(
      path.join(dir, `${locale}.json`),
      JSON.stringify({ translation })
    )
  }
  fs.writeFileSync(path.join(root, 'src/example.tsx'), source)
  try {
    return {
      status: 0,
      ...JSON.parse(
        execFileSync(process.execPath, [script], {
          cwd: root,
          encoding: 'utf8',
        })
      ),
    }
  } catch (error) {
    return { status: error.status, ...JSON.parse(error.stdout) }
  }
}

test('valid literal, conditional, dynamic calls and brand names pass', (t) => {
  const result = check(t)
  assert.equal(result.status, 0)
  assert.deepEqual(result.failures, [])
})
test('missing locale key fails', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      if (locale === 'ja') delete table.Hello
    },
  })
  assert.equal(result.status, 1)
  assert.ok(
    result.failures.some(
      (item) => item.locale === 'ja' && item.issue === 'missing'
    )
  )
})
test('empty translation fails', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      if (locale === 'vi') table.Hello = '  '
    },
  })
  assert.equal(result.status, 1)
  assert.ok(
    result.failures.some(
      (item) => item.locale === 'vi' && item.issue === 'empty'
    )
  )
})
test('lost interpolation variable fails', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      if (locale === 'fr') table['Hi {{name}}'] = 'Bonjour'
    },
  })
  assert.equal(result.status, 1)
  assert.ok(
    result.failures.some(
      (item) => item.locale === 'fr' && item.issue === 'variables'
    )
  )
})
test('missing conditional and fallback branch keys fail', (t) => {
  const result = check(t, {
    source: "t(flag ? 'Hello' : 'Missing'); t(dynamicKey || 'Fallback')",
  })
  assert.equal(result.status, 1)
  assert.deepEqual(result.failures.map((item) => item.key).sort(), [
    'Fallback',
    'Missing',
  ])
})
test('broken substitution marker fails', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      if (locale === 'ru') table.Hello = '__ PH_0 __'
    },
  })
  assert.equal(result.status, 1)
  assert.ok(result.failures.some((item) => item.issue === 'broken-placeholder'))
})

test('null translation fails', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      if (locale === 'ja') table.Hello = null
    },
  })
  assert.equal(result.status, 1)
  assert.ok(result.failures.some((item) => item.issue === 'invalid-value'))
})
test('unescaped interpolation still requires the correct variable', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      table.Hello = locale === 'fr' ? '{{- wrong}}' : '{{- value}}'
    },
  })
  assert.equal(result.status, 1)
  assert.ok(
    result.failures.some(
      (item) => item.locale === 'fr' && item.issue === 'variables'
    )
  )
})
test('array fallback keys are checked', (t) => {
  const result = check(t, {
    source: "t(['Missing primary', 'Missing fallback'])",
  })
  assert.equal(result.status, 1)
  assert.deepEqual(result.failures.map((item) => item.key).sort(), [
    'Missing fallback',
    'Missing primary',
  ])
})
test('boolean, number, and object translation values fail', (t) => {
  for (const invalid of [false, 123, { label: 'Hello' }]) {
    const result = check(t, {
      change: (table, locale) => {
        if (locale === 'ja') table.Hello = invalid
      },
    })
    assert.equal(result.status, 1)
    assert.ok(result.failures.some((item) => item.issue === 'invalid-value'))
  }
})
test('formatted, spaced and repeated interpolation variables remain valid', (t) => {
  const result = check(t, {
    change: (table) => {
      table.Hello = '{{ value, number }} {{- name }} {{ value }}'
    },
  })
  assert.equal(result.status, 0)
})
test('a missing repeated interpolation occurrence fails', (t) => {
  const result = check(t, {
    change: (table, locale) => {
      table.Hello = locale === 'fr' ? '{{value}}' : '{{value}} {{value}}'
    },
  })
  assert.equal(result.status, 1)
  assert.ok(
    result.failures.some(
      (item) => item.locale === 'fr' && item.issue === 'variables'
    )
  )
})
