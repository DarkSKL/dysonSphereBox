// 说明：使用项目已有 TypeScript 编译器运行纯队列契约测试；不连接 Wails、服务器或翻译供应商。
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

const source = await readFile(new URL('../src/services/descriptionTranslationQueue.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022}})
const {createDescriptionTranslationQueue} = await import(`data:text/javascript;base64,${Buffer.from(compiled.outputText).toString('base64')}`)
const tick = () => new Promise((resolve) => setTimeout(resolve, 10))
async function until(predicate) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (predicate()) return
    await tick()
  }
  assert.fail('queue did not settle')
}
const translated = (texts) => ({items: texts.map((text) => ({status: 'ready', reason: '', data: `中文:${text}`}))})

test('toggle off sends nothing; bursts skip blank texts and preserve source order with exact-text deduplication', async () => {
  const calls = []
  const queue = createDescriptionTranslationQueue(async (texts) => { calls.push(texts); return translated(texts) }, () => {}, 1)
  queue.setVisibleTexts(['a', 'b', 'a', '', '   ', ' a'])
  await tick()
  assert.equal(calls.length, 0)
  assert.equal(queue.get('a'), undefined)
  queue.setEnabled(true)
  queue.setVisibleTexts(['b', 'a', 'b', '', '   ', ' a'])
  await tick()
  assert.equal(calls.length, 0)
  queue.setContext('test-session')
  await until(() => queue.get('a')?.status === 'ready')
  assert.deepEqual(calls, [['b', 'a', ' a']])
  assert.equal(queue.get('a').data, '中文:a')
  queue.setEnabled(false)
  queue.setVisibleTexts(['not requested'])
  await tick()
  assert.equal(calls.length, 1)
  queue.dispose()
})

test('at most 20 texts per batch, only one request in flight, same text joins in-flight work', async () => {
  const calls = []
  const pending = []
  const queue = createDescriptionTranslationQueue((texts) => {
    calls.push(texts)
    return new Promise((resolve) => pending.push(() => resolve(translated(texts))))
  }, () => {}, 1)
  const texts = Array.from({length: 43}, (_, index) => `description-${index}`)
  queue.setContext('test-session')
  queue.setVisibleTexts(texts)
  queue.setEnabled(true)
  await until(() => calls.length === 1)
  queue.setVisibleTexts([...texts, texts[0]])
  await tick()
  assert.equal(calls.length, 1)
  assert.deepEqual(calls[0], texts.slice(0, 20))
  pending.shift()()
  await until(() => calls.length === 2)
  assert.deepEqual(calls[1], texts.slice(20, 40))
  pending.shift()()
  await until(() => calls.length === 3)
  assert.deepEqual(calls[2], texts.slice(40))
  pending.shift()()
  await until(() => queue.get(texts[42])?.status === 'ready')
  queue.dispose()
})

test('per-item failure keeps original text, never retries automatically, and manual retry affects only failed text', async () => {
  const calls = []
  const queue = createDescriptionTranslationQueue(async (texts) => {
    calls.push(texts)
    return calls.length === 1 ? {items: [translated(['good']).items[0], {status: 'error', reason: 'providerFailed', data: 'untrusted fallback'}]} : translated(texts)
  }, () => {}, 1)
  queue.setContext('test-session')
  queue.setVisibleTexts(['good', 'bad', 'bad'])
  queue.setEnabled(true)
  await until(() => queue.get('bad')?.status === 'error')
  assert.equal(queue.get('good').data, '中文:good')
  assert.equal(queue.get('bad').data, 'bad')
  queue.setVisibleTexts(['good', 'bad'])
  await tick()
  assert.equal(calls.length, 1)
  queue.retry('bad')
  await until(() => queue.get('bad')?.status === 'ready')
  assert.deepEqual(calls, [['good', 'bad'], ['bad']])
  queue.dispose()
})

test('mismatched result count and rejected requests fall back for whole batch until retry', async () => {
  let attempt = 0
  const queue = createDescriptionTranslationQueue(async (texts) => {
    attempt++
    if (attempt === 1) return {items: []}
    if (attempt === 2) throw new Error('offline')
    return translated(texts)
  }, () => {}, 1)
  queue.setContext('test-session')
  queue.setVisibleTexts(['a', 'b'])
  queue.setEnabled(true)
  await until(() => queue.get('b')?.status === 'error')
  assert.equal(queue.get('a').data, 'a')
  assert.equal(queue.get('b').data, 'b')
  queue.retry('a')
  await until(() => attempt === 2 && queue.get('a')?.status === 'error')
  queue.retry('a')
  await until(() => queue.get('a')?.status === 'ready')
  assert.equal(queue.get('b').status, 'error')
  queue.dispose()
})

test('context switch discards late results and old cache while serializing next-context work', async () => {
  const pending = []
  const calls = []
  const queue = createDescriptionTranslationQueue((texts, context) => {
    calls.push(context)
    return new Promise((resolve) => pending.push(() => resolve(translated(texts))))
  }, () => {}, 1)
  queue.setContext('server-a/account-a/generation-1')
  queue.setVisibleTexts(['a'])
  queue.setEnabled(true)
  await until(() => calls.length === 1)
  queue.setContext('server-b/account-b/generation-2')
  assert.equal(queue.get('a'), undefined)
  await tick()
  assert.equal(calls.length, 1)
  pending.shift()()
  await until(() => calls.length === 2)
  assert.equal(queue.get('a').status, 'loading')
  pending.shift()()
  await until(() => queue.get('a')?.status === 'ready')
  assert.deepEqual(calls, ['server-a/account-a/generation-1', 'server-b/account-b/generation-2'])
  queue.setContext('')
  queue.setEnabled(false)
  assert.equal(queue.get('a'), undefined)
  queue.dispose()
})

test('descriptions leaving viewport before flush are skipped; deactivation and disabling discard in-flight results', async () => {
  let resolve
  let calls = 0
  const queue = createDescriptionTranslationQueue((texts) => {
    calls++
    return new Promise((finish) => { resolve = () => finish(translated(texts)) })
  }, () => {}, 1)
  queue.setContext('test-session')
  queue.setEnabled(true)
  queue.setVisibleTexts(['offscreen'])
  queue.setVisibleTexts([])
  await tick()
  assert.equal(calls, 0)
  queue.setVisibleTexts(['visible'])
  await until(() => calls === 1)
  queue.setActive(false)
  resolve()
  await tick()
  assert.equal(queue.get('visible'), undefined)
  queue.setActive(true)
  await until(() => calls === 2)
  queue.setEnabled(false)
  resolve()
  await tick()
  assert.equal(queue.get('visible'), undefined)
  assert.equal(calls, 2)
  queue.dispose()
})
