import fs from 'node:fs/promises'
import path from 'node:path'

import { parse } from '@babel/parser'

const locales = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi']
const tables = Object.fromEntries(
  await Promise.all(
    locales.map(async (locale) => [
      locale,
      JSON.parse(await fs.readFile(`src/i18n/locales/${locale}.json`, 'utf8'))
        .translation,
    ])
  )
)
const keys = new Set(
  Object.values(tables).flatMap((table) => Object.keys(table))
)
const failures = []
const variables = (text) =>
  [...String(text).matchAll(/{{\s*-?\s*([^},\s]+)[^}]*}}/g)]
    .map((match) => match[1])
    .sort()
    .join(',')
for (const locale of locales) {
  for (const key of keys) {
    const value = tables[locale][key]
    if (value === undefined) failures.push({ locale, key, issue: 'missing' })
    else if (
      value === null ||
      (typeof value !== 'string' && !Array.isArray(value)) ||
      (Array.isArray(value) &&
        value.some((item) => typeof item !== 'string' || item.trim() === ''))
    )
      {failures.push({ locale, key, issue: 'invalid-value' })}
    else if (typeof value === 'string' && value.trim() === '')
      {failures.push({ locale, key, issue: 'empty' })}
    else if (variables(tables.en[key] ?? key) !== variables(value))
      {failures.push({ locale, key, issue: 'variables' })}
    else if (typeof value === 'string' && /__\s*PH_\d+\s*__/.test(value))
      {failures.push({ locale, key, issue: 'broken-placeholder' })}
  }
}
let scanned = 0
async function inspect(dir) {
  for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
    if (['locales', 'node_modules'].includes(entry.name)) continue
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) await inspect(file)
    else if (/\.[jt]sx?$/.test(file) && !/\.(test|spec)\./.test(file)) {
      const source = await fs.readFile(file, 'utf8')
      let ast
      try {
        ast = parse(source, {
          sourceType: 'unambiguous',
          plugins: ['jsx', 'typescript'],
        })
      } catch (error) {
        failures.push({ file, issue: 'parse-error', message: error.message })
        continue
      }
      scanned++
      function walk(node) {
        if (!node || typeof node !== 'object') return
        const called =
          node.type === 'CallExpression' &&
          (node.callee?.name === 't' ||
            (node.callee?.type === 'MemberExpression' &&
              node.callee.property?.name === 't'))
        if (called) {
          const arg = node.arguments[0]
          function literalKeys(expression) {
            if (expression?.type === 'StringLiteral') return [expression.value]
            if (
              expression?.type === 'TemplateLiteral' &&
              expression.expressions.length === 0
            )
              {return [expression.quasis[0].value.cooked]}
            if (expression?.type === 'ArrayExpression')
              {return expression.elements.flatMap(literalKeys)}
            if (expression?.type === 'ConditionalExpression')
              {return [
                ...literalKeys(expression.consequent),
                ...literalKeys(expression.alternate),
              ]}
            if (expression?.type === 'LogicalExpression')
              {return [
                ...literalKeys(expression.left),
                ...literalKeys(expression.right),
              ]}
            return []
          }
          for (const key of literalKeys(arg))
            {if (!Object.hasOwn(tables.en, key))
              {failures.push({
                file,
                line: node.loc.start.line,
                key,
                issue: 'literal-call-missing',
              })}}
        }
        if (
          file.endsWith('i18n/static-keys.ts') &&
          node.type === 'StringLiteral' &&
          !Object.hasOwn(tables.en, node.value)
        )
          {failures.push({ file, key: node.value, issue: 'static-key-missing' })}
        for (const [name, value] of Object.entries(node))
          {if (!['loc', 'comments', 'tokens'].includes(name)) {
            if (Array.isArray(value)) value.forEach(walk)
            else if (value && typeof value === 'object') walk(value)
          }}
      }
      walk(ast.program)
    }
  }
}
await inspect('src')
console.log(JSON.stringify({ scanned, keyCount: keys.size, failures }, null, 2))
process.exitCode = failures.length ? 1 : 0
