import assert from 'node:assert/strict'
import {createServer as createHTTPServer} from 'node:http'
import {after, test} from 'node:test'
import {fileURLToPath} from 'node:url'
import {effectScope, shallowRef} from 'vue'
import {createServer} from 'vite'

// 只替换会话与桥接，实际运行页面使用的预览调度器和 Vue 响应式状态。
class FeedbackError extends Error {constructor(reason) {super(reason); this.reason = reason}}
const sessionKey = shallowRef('first')
globalThis.__previewTest = {sessionKey, FeedbackError}
const priorWindow = globalThis.window
globalThis.window = Object.assign(new EventTarget(), {setTimeout, clearTimeout})
const vite = await createServer({
  root: fileURLToPath(new URL('../../src/composables/feedback/', import.meta.url)),
  configFile: false, logLevel: 'error', optimizeDeps: {disabled: true},
  server: {middlewareMode: true, hmr: {server: createHTTPServer()}},
  plugins: [{name: 'preview-test-boundary', enforce: 'pre',
    resolveId(id, importer) {
      if (!importer?.endsWith('/useFeedbackPreview.ts')) return
      if (id === '../../pages/services/account') return '\0test-account'
      if (id === '../../services/feedback') return '\0test-feedback'
    },
    load(id) {
      if (id === '\0test-account') return 'export const accountSessionKey=globalThis.__previewTest.sessionKey'
      if (id === '\0test-feedback') return 'export const FeedbackError=globalThis.__previewTest.FeedbackError; export const feedbackCall=(op,input)=>globalThis.__previewTest.call(op,input)'
    },
  }],
})
const {useFeedbackPreview} = await vite.ssrLoadModule('/useFeedbackPreview.ts')
after(async () => {
  await vite.close()
  delete globalThis.__previewTest
  if (priorWindow === undefined) delete globalThis.window
  else globalThis.window = priorWindow
})
const tick = () => new Promise(resolve => setImmediate(resolve))
const point = (id, version) => ({starId: String(id), previewVersion: version, x: 0, y: 0, progress: 'feedback', brightness: .3})
const item = id => ({starId: id, preview: '反馈 ' + id, displayName: '匿名'})
function later() {
  let release
  const promise = new Promise(resolve => {release = resolve})
  return {promise, release}
}
function setup(t, invoke) {
  const active = shallowRef(true), missing = [], calls = []
  globalThis.__previewTest.call = async (op, input) => {calls.push([op, input]); return await invoke(op, input)}
  const scope = effectScope(), p = scope.run(() => useFeedbackPreview(active, id => missing.push(id)))
  t.after(() => scope.stop())
  const load = (stars, version = 'map1', mine = new Map()) => {
    p.reconcile(stars, mine, version)
    p.prewarm(stars.map(point => ({point})))
  }
  return {p, load, active, missing, calls}
}

test('128 个公开摘要批量预取，复悬与仅热度变化不再读取', async t => {
  const {p, load, calls} = setup(t, (op, input) => {
    assert.equal(op, 'Previews')
    return {items: input.starIds.map(item)}
  })
  const stars = Array.from({length: 130}, (_, i) => point(9007199254740993n + BigInt(i), '32'))
  load(stars)
  await tick()
  assert.equal(calls.length, 1)
  assert.deepEqual(calls[0][1].starIds, stars.slice(0,128).map(s => s.starId))
  p.select(stars[0], {x: 30, y: 20})
  assert.deepEqual(p.quote.value, item(stars[0].starId))
  p.clear()
  p.select(stars[0])
  load(stars.map(s => ({...s, brightness: 1})), 'map2')
  await tick()
  assert.equal(calls.length, 1)
  p.select(stars[129])
  await tick()
  assert.deepEqual(p.quote.value, item(stars[129].starId))
  assert.deepEqual(calls[1][1].starIds, [stars[129].starId])
})

test('旧服务器缺批量接口时只读意图目标，正文先到，真实署名补齐后共用缓存', async t => {
  const name = later()
  const {p, load, calls} = setup(t, (op, {starId}) => {
    if (op === 'Previews') throw new FeedbackError('unsupported')
    if (op === 'Preview') return {starId, preview: '旧版首行'}
    assert.equal(op, 'Detail')
    return name.promise
  })
  const a = point(1), b = point(2)
  load([a,b]); await tick()
  assert.equal(calls.length, 1)
  p.approach(a); p.select(a); p.select(a)
  await tick()
  assert.equal(p.quote.value.preview, '旧版首行')
  assert.equal(p.quote.value.displayName, undefined)
  assert.deepEqual(calls.map(c => c[0]), ['Previews','Preview','Detail'])
  name.release({starId:'1',withdrawn:false,original:{body:'旧版首行\n正文',displayName:'对外署名',reviewState:'approved'}})
  await tick()
  p.clear(); p.select(a)
  assert.equal(p.quote.value.displayName, '对外署名')
  await tick()
  assert.equal(calls.length, 3)
})

test('切星和撤回时，迟到的回执不会覆盖新引文或重新显示旧星', async t => {
  const slow = later()
  const {p, load, calls} = setup(t, (op, input) => op === 'Previews' ? slow.promise : item(input.starId))
  const a = point(1,'v1'), b = point(2,'v1')
  p.reconcile([a,b], new Map(),'map1')
  p.select(a); await tick()
  p.select(b); await tick()
  assert.equal(p.quote.value.starId, '2')
  load([b])
  slow.release({items:[item('1')]}); await tick()
  assert.equal(p.quote.value.starId, '2')
  p.select(a)
  assert.equal(p.quote.value, null)
  load([]);assert.equal(p.hover.value,null);assert.equal(p.quote.value,null)
  assert.equal(calls.filter(c=>c[0]==='Preview').length,1)
})

test('本人待审引文不进入公共批量，审核版本和会话切换立刻收回旧正文', async t => {
  const first = later(), second = later()
  let batches = 0
  const {p, load, calls, active} = setup(t, () => (++batches === 1 ? first : second).promise)
  const a = point(7,'20')
  load([a], 'm', new Map([['7',{starId:'7',reviewState:'pending',bodyVersion:'18',preview:'待审原文',displayName:'匿名'}]]))
  p.select(a); await tick();assert.equal(p.quote.value.preview,'待审原文');assert.equal(calls.length,0)
  load([a],'m2');assert.equal(p.quote.value,null);await tick()
  const approved = point(7,'21')
  load([approved],'m3');await tick()
  first.release({items:[{...item('7'),preview:'旧审核版本'}]});await tick()
  assert.equal(p.quote.value,null)
  sessionKey.value='second';active.value=false
  second.release({items:[item('7')]});await tick()
  assert.equal(p.quote.value,null);assert.equal(p.hover.value,null)
})

test('明确略过的公开 ID 清除预览，字符串形式的 HTML 保持为普通文本', async t => {
  const {p, load, missing} = setup(t, () => ({items:[{starId:'9',preview:'<em>原文</em>',displayName:'<署名>'}]}))
  const hidden=point(8,'8'), publicStar=point(9,'9')
  load([hidden,publicStar]);p.select(hidden);await tick()
  assert.deepEqual(missing,['8']);assert.equal(p.quote.value,null)
  p.select(publicStar);assert.equal(p.quote.value.preview,'<em>原文</em>');assert.equal(p.quote.value.displayName,'<署名>')
})

test('缓存到期重新核验署名，重新进入同一视野不会重复启动在途读取', async t => {
  let clock = performance.now(), name = '原署名'
  t.mock.method(performance, 'now', () => clock)
  const {p, load, calls} = setup(t, (_, {starIds}) => ({
    items: starIds.map(id => ({...item(id), displayName: name})),
  }))
  const a = point(15,'64')
  load([a]);load([a]);p.select(a);await tick()
  assert.equal(calls.length,1)
  assert.equal(p.quote.value.displayName,'原署名')
  p.clear();clock += 61000;name = '更新署名'
  p.select(a)
  assert.equal(p.quote.value,null)
  await tick()
  assert.equal(calls.length,2)
  assert.equal(p.quote.value.displayName,'更新署名')
})
