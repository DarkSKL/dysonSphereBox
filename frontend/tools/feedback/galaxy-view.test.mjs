import assert from 'node:assert/strict'
import {createServer as createHTTPServer} from 'node:http'
import {after, test} from 'node:test'
import {fileURLToPath} from 'node:url'
import {effectScope, shallowRef} from 'vue'
import {createServer} from 'vite'

// 通过项目已有的 Vite 转译被测模块，无需额外的测试运行器。
class FeedbackError extends Error {
  constructor(reason, message = reason) { super(message); this.reason = reason }
}
const sessionKey = shallowRef('session-1')
globalThis.__mapTest = {sessionKey, FeedbackError, call: async () => ({version: 'v0', unchanged: false, points: []})}
const priorWindow = globalThis.window
const priorCAF = globalThis.cancelAnimationFrame
const priorRAF = globalThis.requestAnimationFrame
globalThis.window = Object.assign(new EventTarget(), {
  setTimeout, clearTimeout,
  matchMedia: () => ({matches: false, addEventListener() {}, removeEventListener() {}}),
})
globalThis.cancelAnimationFrame = () => {}
globalThis.requestAnimationFrame = () => 0
const vite = await createServer({
  root: fileURLToPath(new URL('../../src/composables/feedback/', import.meta.url)),
  configFile: false, logLevel: 'error', optimizeDeps: {disabled: true},
  server: {middlewareMode: true, hmr: {server: createHTTPServer()}},
  plugins: [{
    name: 'map-test-boundary', enforce: 'pre',
    resolveId(id, importer) {
      if (!importer?.endsWith('/useGalaxyMap.ts')) return
      if (id === '../../pages/services/account') return '\0test-account'
      if (id === '../../services/feedback') return '\0test-feedback'
    },
    load(id) {
      if (id === '\0test-account') return 'export const accountSessionKey=globalThis.__mapTest.sessionKey'
      if (id === '\0test-feedback') return 'export const FeedbackError=globalThis.__mapTest.FeedbackError; export const feedbackCall=(op,input)=>globalThis.__mapTest.call(op,input)'
    },
  }],
})
after(async () => {
  await vite.close()
  delete globalThis.__mapTest
  if (priorWindow === undefined) delete globalThis.window
  else globalThis.window = priorWindow
  if (priorCAF === undefined) delete globalThis.cancelAnimationFrame
  else globalThis.cancelAnimationFrame = priorCAF
  if (priorRAF === undefined) delete globalThis.requestAnimationFrame
  else globalThis.requestAnimationFrame = priorRAF
})
const {HOME_CAMERA, project, usefulZoomLimit, queryBounds, includeStar, starsNear, AMBIGUOUS_GAP, isInView, resolveClick, revalidateCandidates} =
  await vite.ssrLoadModule('/galaxy-camera.ts')
const {SAMPLE_STRIDE, sampleNebula, cloudPresence, cloudExposure, cloudPlan, easeCloudLevel, CLOUD_VISIBLE} = await vite.ssrLoadModule('/galaxy-nebula.ts')
const {galaxyStatus} = await vite.ssrLoadModule('/galaxy-status.ts')
const {haloScale} = await vite.ssrLoadModule('/galaxy-art.ts')
const {useGalaxyMap} = await vite.ssrLoadModule('/useGalaxyMap.ts')
const star = (id, x, y, z) => ({starId: String(id), x, y, z, progress: 'feedback', brightness: .4})
const tick = () => new Promise(resolve => setImmediate(resolve))
function later() {
  let resolve, reject
  const promise = new Promise((res, rej) => { resolve = res; reject = rej })
  return {promise, resolve, reject}
}
function setupMap(t, invoke) {
  const foreground = shallowRef(true)
  const calls = []
  globalThis.__mapTest.call = async (op, input) => { calls.push([op, input]); return await invoke(op, input) }
  const scope = effectScope()
  const map = scope.run(() => useGalaxyMap(foreground))
  t.after(() => scope.stop())
  return {map, foreground, calls}
}

test('任意镜头固定银心；轴对称查询包围可见厚度', () => {
  for (const size of [{width: 1060, height: 670}, {width: 490, height: 760}]) {
    for (const inclination of [-1.39, 0, .51, 1.39]) {
      for (const yaw of [-1.8, .2, 2.6]) {
        for (const zoom of [1, 2.25, 8, 32, 64]) {
          const camera = {yaw, inclination, zoom}
          assert.deepEqual(project({x: 0, y: 0, z: 0}, camera, size), {x: size.width / 2, y: size.height / 2, depth: 0})
          const b = queryBounds(camera, size)
          if (!b) continue
          assert.equal(b.minX, -b.maxX)
          assert.equal(b.minY, -b.maxY)
          for (let x = -90000; x <= 90000; x += 9000) {
            for (let y = -90000; y <= 90000; y += 9000) {
              for (const z of [-10000, 10000]) {
                const p = project({x, y, z}, camera, size)
                if (p.x >= 0 && p.x < size.width && p.y >= 0 && p.y < size.height)
                  assert.ok(x >= b.minX && x <= b.maxX && y >= b.minY && y <= b.maxY)
              }
            }
          }
        }
      }
    }
  }
})

test('最大倍率留住近星；打开外围反馈只在必要时拉远', () => {
  const size = {width: 900, height: 620}
  const points = [star(1, 5600, -2400, 1100), star(2, 1200, -8200, -4300), star(3, -6800, 7800, 4500)]
  const zoom = usefulZoomLimit(points, size)
  assert.ok(zoom > 1 && zoom < 64)
  const camera = {...HOME_CAMERA, zoom, inclination: 1.32}
  for (const point of points) assert.equal(includeStar(camera, point, size), camera)
  const outer = star(4, 87000, -4000, 6400)
  const fit = includeStar(camera, outer, size)
  assert.ok(fit.zoom < zoom)
  assert.equal(fit.yaw, camera.yaw)
  assert.equal(fit.inclination, camera.inclination)
  assert.equal(includeStar(fit, outer, size).zoom, fit.zoom)
  assert.equal(usefulZoomLimit([], size), 1)
})

test('缩放换快照时同一星的光尘可按连续区块或较低采样预算复用', () => {
  const a = star(1, -30000, 24000, 2000), b = star(2, 10000, 16000, -600)
  const full = sampleNebula([a, b], 64)
  const local = sampleNebula([b, a], 32)
  assert.deepEqual(local.subarray(0, 32 * SAMPLE_STRIDE), full.subarray(64 * SAMPLE_STRIDE, 96 * SAMPLE_STRIDE))
  assert.ok(sampleNebula([a]).length > 0, '第一颗公开星就带出云雾')
  assert.equal(sampleNebula([]).length, 0)
  assert.equal(sampleNebula([{...a, privateLabel: '待审核'}]).length, 0, '作者待审的私有星不产生云雾')
})

test('云雾亮度随数量连续：先升到峰值，密集后缓慢回落且不过曝', () => {
  const level = mass => cloudPresence(mass) * cloudExposure(mass)
  assert.equal(level(0), 0)
  // 1 到 4 条：每多一颗星云雾明显更亮，第一颗星已经可见。
  let previous = 0
  for (const mass of [1, 2, 3, 4]) {
    assert.ok(level(mass) > previous, '亮度应随数量上升到 ' + mass)
    previous = level(mass)
  }
  assert.ok(level(1) > .3, '1 条不能暗到像没有')
  // 峰值之后只缓慢回落，且 200 条仍不低于峰值的一半。
  let falling = level(4)
  for (const mass of [6, 12, 30, 60, 200, 400]) {
    assert.ok(level(mass) <= falling + 1e-9, '密集后不应回升到 ' + mass)
    falling = level(mass)
  }
  assert.ok(level(200) > .5 * level(4), '密集时不应暗到像另一套样式')
  assert.ok(level(400) <= level(4), '密集时不应比峰值更亮而过曝')
  // 相邻数量之间没有跳变。
  for (let mass = 1; mass < 400; mass++) {
    assert.ok(Math.abs(level(mass + 1) - level(mass)) < .26, '相邻数量不应跳变：' + mass)
  }
})

test('光晕只在密集时收窄，且不低于一半', () => {
  assert.equal(haloScale(1), 1)
  assert.equal(haloScale(30), 1)
  assert.ok(haloScale(200) < haloScale(60) && haloScale(60) < 1)
  assert.equal(haloScale(1e6), .5)
})

test('点击命中：明显最近的星直接打开，难以分辨时交给附近列表', () => {
  const at = (id, x, y, depth = 0) => ({x, y, depth, point: star(id, 0, 0, 0)})
  const stars = [at(1, 100, 100), at(2, 102, 100), at(3, 140, 100), at(4, 300, 300)]
  assert.deepEqual(starsNear(stars, {x: 100, y: 100}, 14).map(i => i.star.point.starId), ['1', '2'])
  assert.deepEqual(starsNear(stars, {x: 101, y: 100}, 14).map(i => i.star.point.starId), ['1', '2'], '等距时按编号稳定排序')
  assert.equal(starsNear(stars, {x: 500, y: 500}, 14).length, 0)
  const near = starsNear(stars, {x: 100, y: 100}, 14)
  assert.ok(near[1].distance - near[0].distance < AMBIGUOUS_GAP, '1 号与 2 号相距过近，不应替用户猜')
  const front = [at(7, 50, 50, 5), at(8, 50, 50, 1)]
  assert.equal(starsNear(front, {x: 50, y: 50}, 14)[0].star.point.starId, '7', '同点重叠时前面的星优先')
})

const status = (over = {}) => ({
  foreground: true, ready: true, error: '', total: 5, visible: 5, snapshotIsPanorama: true, viewIsPanorama: true, loading: false,
  selectedId: '', selectedVisible: false, selectedClickable: false, aggregated: false, ...over,
})

test('状态提示：各状态互斥，首次加载完成前不误报无反馈', () => {
  assert.equal(galaxyStatus(status()), '')
  assert.equal(galaxyStatus(status({ready: false, total: 0, visible: 0, snapshotIsPanorama: false})), 'loading')
  assert.equal(galaxyStatus(status({error: '失败'})), 'error')
  assert.equal(galaxyStatus(status({error: '失败', ready: false})), 'error', '失败优先于加载中')
  assert.equal(galaxyStatus(status({total: 0, visible: 0})), 'empty')
  assert.equal(galaxyStatus(status({foreground: false, ready: false})), '')
})

test('P1 回到全景：全景回执到达前不得显示「确实无反馈」，也不显示「画面内无星」', () => {
  // 局部放大后快照为空：画面内没有星，这是局部范围，不是整片星图为空。
  const local = status({total: 0, visible: 0, snapshotIsPanorama: false, viewIsPanorama: false})
  assert.equal(galaxyStatus(local), 'offscreen')
  // 点「回到全景」：镜头已回全景范围，但最近一次成功快照仍是局部，等待期间两种提示都不出现。
  const waiting = status({total: 0, visible: 0, snapshotIsPanorama: false, viewIsPanorama: true, loading: true})
  assert.notEqual(galaxyStatus(waiting), 'empty')
  assert.equal(galaxyStatus(waiting), '')
  assert.equal(galaxyStatus({...waiting, loading: false}), '', '请求被排队、尚未发出时同样不能误报')
  // 全景回执到达：真的为空才宣布无反馈。
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotIsPanorama: true, viewIsPanorama: true})), 'empty')
  assert.equal(galaxyStatus(status({total: 6, visible: 6})), '')
  // 反向：全景快照为空后放大，等待局部回执期间不报任何提示。
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotIsPanorama: true, viewIsPanorama: false})), '')
})

test('P6 重试等待期间：保留错误提示，不出现「确实无反馈」', () => {
  const retrying = status({error: '连接失败', ready: true, total: 0, visible: 0, loading: true})
  assert.equal(galaxyStatus(retrying), 'error')
  assert.equal(galaxyStatus(status({error: '连接失败', ready: false, total: 0, visible: 0, loading: true, snapshotIsPanorama: false})), 'error')
  assert.equal(galaxyStatus({...retrying, loading: false}), 'error', '再次失败：错误保留')
  assert.equal(galaxyStatus(status({error: '', total: 6, visible: 6})), '')
  assert.equal(galaxyStatus(status({error: '', total: 0, visible: 0})), 'empty')
})

test('P3 聚合星：所读星按自己的坐标判断是否在画面内，聚合隐藏是独立状态', () => {
  const size = {width: 950, height: 746}
  const camera = {...HOME_CAMERA, zoom: 8}
  const near = {x: 800, y: 500, z: 0}
  const far = {x: -60000, y: 30000, z: 1000}
  assert.equal(isInView(near, camera, size), true)
  assert.equal(isInView(far, camera, size), false)
  assert.equal(isInView(far, HOME_CAMERA, size), true, '全景时外围星在画面内')
  assert.equal(isInView(near, camera, {width: 0, height: 0}), false)
  // A) 成员在画面内但快照里只有已移出画面的代表星：不是移出画面，而是聚合隐藏。
  const grouped = status({selectedId: '5002', selectedVisible: true, selectedClickable: false, aggregated: true})
  assert.equal(galaxyStatus(grouped), 'selected-grouped')
  assert.notEqual(galaxyStatus(grouped), 'selected-offscreen')
  assert.equal(galaxyStatus({...grouped, aggregated: false}), '', '没有聚合证据时不归因于聚合')
  // B) 成员坐标确实移出画面：提示移出，与代表星是否可见无关。
  assert.equal(galaxyStatus(status({selectedId: '5002', selectedVisible: false, selectedClickable: true, aggregated: true})), 'selected-offscreen')
  assert.equal(galaxyStatus(status({selectedId: '5002', selectedVisible: true, selectedClickable: true, aggregated: true})), '')
})

test('P4 命中：半径内超过 12 颗星全部保留，拥挤时不截断', () => {
  const at = (id, x, y) => ({x, y, depth: 0, point: star(id, 0, 0, 0)})
  const ring = Array.from({length: 13}, (_, i) => at(String(6001 + i), 100 + 6 * Math.cos(i / 13 * 2 * Math.PI), 100 + 6 * Math.sin(i / 13 * 2 * Math.PI)))
  const result = resolveClick(ring, {x: 100, y: 100}, 12)
  assert.equal(result.open, undefined)
  assert.equal(result.crowd.length, 13, '第 13 颗不能被截掉')
  assert.ok(result.crowd.some(item => item.point.starId === '6013'))
  assert.equal(new Set(result.crowd.map(item => item.point.starId)).size, 13)
  assert.equal(resolveClick([at('1', 100, 100), at('2', 110, 100)], {x: 100, y: 100}, 12).open.point.starId, '1')
  assert.deepEqual(resolveClick([at('1', 300, 300)], {x: 100, y: 100}, 12), {})
})

test('P4 候选随快照更新重新校验：失效项移除，全部失效返回空', () => {
  const at = (id, x, y) => ({x, y, depth: 0, point: star(id, 0, 0, 0)})
  const ids = ['10', '11', '12']
  assert.deepEqual(revalidateCandidates(ids, [at('10', 1, 1), at('11', 2, 2), at('12', 3, 3)]).map(s => s.point.starId), ['10', '11', '12'])
  assert.deepEqual(revalidateCandidates(ids, [at('12', 3, 3), at('10', 1, 1)]).map(s => s.point.starId), ['10', '12'], '11 被撤回后顺序不变')
  assert.equal(revalidateCandidates(['10'], [at('10', 50, 60)])[0].x, 50, '位置取最新投影')
  assert.deepEqual(revalidateCandidates(ids, [at('99', 1, 1)]), [], '代表星被替换后旧候选失效')
  assert.deepEqual(revalidateCandidates([], [at('10', 1, 1)]), [])
})

test('P2 云雾淡出：最后一颗星消失后过渡中间帧仍绘制云雾，淡出结束才释放，始终 0 条无残留', () => {
  const samples = new Float32Array(SAMPLE_STRIDE * 4).fill(1)
  const none = new Float32Array(0)
  let level = cloudPresence(1) * cloudExposure(1)
  let plan = cloudPlan(level, samples, undefined)
  assert.equal(plan.draw, true)
  let kept = plan.keep
  const start = level
  const frames = []
  let settling = true
  for (let t = 0; t < 4000 && settling; t += 16) {
    const step = easeCloudLevel(level, 0, 16, false)
    level = step.level
    settling = step.settling
    plan = cloudPlan(level, none, kept)
    kept = plan.keep
    frames.push({t, level, draw: plan.draw, release: plan.release})
  }
  const middle = frames.filter(f => f.level > .4 * start && f.level < .6 * start)
  assert.ok(middle.length > 0, '淡出系数中间值的帧存在')
  assert.ok(middle.every(f => f.draw && !f.release), '淡出系数中间值时云雾层仍在绘制，且没有被释放')
  assert.ok(frames[0].draw, '第一帧仍绘制，不整层跳灭')
  assert.ok(frames.length > 20, '淡出持续多帧，不是一帧结束')
  const last = frames[frames.length - 1]
  assert.equal(last.level, 0)
  assert.equal(last.draw, false, '淡出结束后不再绘制')
  assert.equal(last.release, true, '淡出结束后释放云雾层')
  assert.deepEqual(cloudPlan(0, none, undefined), {draw: false, use: undefined, release: true, keep: undefined}, '始终 0 条：不绘制、无残留')
  assert.deepEqual(easeCloudLevel(.8, 0, 16, true), {level: 0, settling: false}, '减少动态效果：直接到位')
  assert.deepEqual(easeCloudLevel(-1, .6, 16, false), {level: .6, settling: false}, '首次直接到位')
  assert.ok(CLOUD_VISIBLE > 0 && CLOUD_VISIBLE < .01)
})

test('P1 组合态 useGalaxyMap：局部空快照点回到全景，全景延迟回执到达前绝不误报空态', async t => {
  const size = {width: 960, height: 640}
  const delayedPanorama = later()
  let step = 0
  const {map} = setupMap(t, (op, input) => {
    assert.equal(op, 'Map')
    step++
    // 第 1 次全景含聚合星（count>1），保证缩放后触发局部范围查询。
    if (step === 1) return {version: 'all#1', unchanged: false, points: [{...star('4001', 70000, -30000, 0), count: 5}]}
    // 第 2 次局部查询为空。
    if (input.minX !== undefined) return {version: 'local#1', unchanged: false, points: []}
    // 第 3 次回到全景：延迟返回。
    return delayedPanorama.promise
  })
  const currentStatus = () => galaxyStatus({
    foreground: true, ready: map.ready.value, error: map.error.value,
    total: map.points.value.length, visible: map.points.value.length,
    snapshotIsPanorama: map.snapshotIsPanorama.value, viewIsPanorama: map.viewIsPanorama.value,
    loading: map.loading.value, selectedId: '', selectedVisible: false, selectedClickable: false, aggregated: false,
  })
  map.updateCamera(HOME_CAMERA, size)
  await map.retry()
  assert.equal(map.snapshotIsPanorama.value, true)
  // 放大到局部空区
  map.updateCamera({...HOME_CAMERA, zoom: 12}, size)
  await map.retry()
  assert.equal(map.snapshotIsPanorama.value, false)
  assert.equal(map.viewIsPanorama.value, false)
  assert.equal(map.points.value.length, 0)
  assert.equal(currentStatus(), 'offscreen')
  // 点「回到全景」：镜头回到 HOME_CAMERA，但全景回执未到
  map.updateCamera(HOME_CAMERA, size)
  assert.equal(map.viewIsPanorama.value, true)
  assert.equal(map.snapshotIsPanorama.value, false)
  assert.equal(currentStatus(), '', '防抖等待期间不得误报「确实无反馈」')
  const pending = map.retry()
  await tick()
  assert.equal(map.loading.value, true)
  assert.equal(map.snapshotIsPanorama.value, false)
  assert.equal(currentStatus(), '', '全景回执在途期间不得误报「确实无反馈」')
  // 全景回执最终到达
  delayedPanorama.resolve({version: 'all#2', unchanged: false, points: [star('4001', 1000, 500, 0)]})
  await pending
  assert.equal(map.snapshotIsPanorama.value, true)
  assert.equal(map.points.value.length, 1)
  assert.equal(currentStatus(), '')
})

test('P6 组合态 useGalaxyMap.retry：慢请求重试指示持续到结算，轮询干扰不提前复位，支持成功与再次失败', async t => {
  const size = {width: 960, height: 640}
  let responder = async () => { throw new FeedbackError('requestFailed', '连接反馈星系失败，请重试。') }
  const {map, calls} = setupMap(t, (op, input) => responder(op, input))
  const currentStatus = () => galaxyStatus({
    foreground: true, ready: map.ready.value, error: map.error.value,
    total: map.points.value.length, visible: map.points.value.length,
    snapshotIsPanorama: map.snapshotIsPanorama.value, viewIsPanorama: map.viewIsPanorama.value,
    loading: map.loading.value, selectedId: '', selectedVisible: false, selectedClickable: false, aggregated: false,
  })
  // 1) 首次 503 失败
  map.updateCamera(HOME_CAMERA, size)
  await map.retry()
  assert.equal(map.loading.value, false)
  assert.equal(map.error.value, '连接反馈星系失败，请重试。')
  assert.equal(currentStatus(), 'error')

  // 2) 点「重试」→ 慢响应再次失败：期间即使触发轮询 refreshNow，retry 也不提前结束；失败后错误保留、可再次重试。
  const slowFail = later()
  responder = () => slowFail.promise
  let done1 = false
  const p1 = map.retry().then(() => { done1 = true })
  await tick()
  assert.equal(map.loading.value, true)
  assert.equal(done1, false)
  assert.equal(currentStatus(), 'error', '重试等待期间不得出现「确实无反馈」')
  map.refreshNow()
  await new Promise(r => setTimeout(r, 200))
  assert.equal(done1, false, '轮询或防抖期间重试指示不能提前消失')
  assert.equal(map.loading.value, true)
  slowFail.reject(new FeedbackError('requestFailed', '服务暂时不可用。'))
  await p1
  assert.equal(done1, true)
  assert.equal(map.loading.value, false)
  assert.equal(map.error.value, '服务暂时不可用。')
  assert.equal(currentStatus(), 'error', '再次失败后错误保留，不误报无反馈')

  // 3) 再次点「重试」→ 慢响应成功：期间持续加载，成功后错误清空、星点出现。
  const slowOk = later()
  responder = () => slowOk.promise
  let done2 = false
  const p2 = map.retry().then(() => { done2 = true })
  await tick()
  assert.equal(map.loading.value, true)
  assert.equal(done2, false)
  assert.equal(currentStatus(), 'error')
  slowOk.resolve({version: 'all#ok', unchanged: false, points: [star('7001', 1000, 800, 0)]})
  await p2
  assert.equal(done2, true)
  assert.equal(map.loading.value, false)
  assert.equal(map.error.value, '')
  assert.equal(map.ready.value, true)
  assert.equal(map.points.value.length, 1)
  assert.equal(currentStatus(), '')

  // 4) 点击重试时若已有旧轮询在途且失败，retry 不能在旧请求结束时提前停止，要等补发重试完成。
  responder = async () => { throw new FeedbackError('requestFailed', '旧错误') }
  await map.retry()
  assert.equal(map.error.value, '旧错误')
  const stalePoll = later()
  const freshRetry = later()
  let round = 0
  responder = () => (++round === 1 ? stalePoll.promise : freshRetry.promise)
  map.refreshNow()
  await new Promise(r => setTimeout(r, 200))
  assert.equal(map.loading.value, true, '旧轮询已在途')
  let done3 = false
  const p3 = map.retry().then(() => { done3 = true })
  stalePoll.reject(new FeedbackError('requestFailed', '旧轮询失败'))
  await tick()
  await tick()
  assert.equal(done3, false, '旧轮询失败后重试指示不得提前消失')
  assert.equal(map.loading.value, true, '已自动发起新的重试请求')
  freshRetry.resolve({version: 'all#recovered', unchanged: false, points: [star('7002', 500, 400, 0)]})
  await p3
  assert.equal(done3, true)
  assert.equal(map.loading.value, false)
  assert.equal(map.error.value, '')
  assert.ok(calls.length >= 6)
})
