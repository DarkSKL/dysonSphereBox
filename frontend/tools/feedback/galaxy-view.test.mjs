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
const {HOME_CAMERA, project, usefulZoomLimit, queryBounds, queryRegionKey, normalizeQueryScope, includeStar, starsNear, AMBIGUOUS_GAP, isInView, resolveClick, revalidateCandidates, resolveFocusFallback} =
  await vite.ssrLoadModule('/galaxy-camera.ts')
const {SAMPLE_STRIDE, sampleNebula, cloudPresence, cloudExposure, cloudPlan, easeCloudLevel, CLOUD_VISIBLE} = await vite.ssrLoadModule('/galaxy-nebula.ts')
const {galaxyStatus, isPublicFeedback, isCoveredByAggregate, aggregateCellOf, aggregateGridBounds, nextSnapshotOrder, AGGREGATE_GRID_WIDTH} = await vite.ssrLoadModule('/galaxy-status.ts')
const {haloScale} = await vite.ssrLoadModule('/galaxy-art.ts')
const {GALAXY_THEMES} = await vite.ssrLoadModule('/galaxy-theme.ts')
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
  const stop = () => scope.stop()
  t.after(stop)
  return {map, foreground, calls, stop}
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
  foreground: true, ready: true, error: '', total: 5, visible: 5, snapshotRegion: 'all', viewRegion: 'all', loading: false,
  selectedId: '', selectedPublic: false, selectedVisible: false, selectedClickable: false, selectedGrouped: false, ...over,
})

test('状态提示：各状态互斥，首次加载完成前不误报无反馈', () => {
  assert.equal(galaxyStatus(status()), '')
  assert.equal(galaxyStatus(status({ready: false, total: 0, visible: 0, snapshotRegion: ''})), 'loading')
  assert.equal(galaxyStatus(status({error: '失败'})), 'error')
  assert.equal(galaxyStatus(status({error: '失败', ready: false})), 'error', '失败优先于加载中')
  assert.equal(galaxyStatus(status({total: 0, visible: 0})), 'empty')
  assert.equal(galaxyStatus(status({foreground: false, ready: false})), '')
})

test('P1 回到全景与 P2-2 局部视区切换：按实际查询范围比较并归一化精度，等待新回执时不沿用旧空态', () => {
  // 精度归一化：微小浮点抖动和 -0 视为同一视区。
  assert.equal(normalizeQueryScope({minX: -8192.00001, minY: -0, maxX: 8191.99999, maxY: 8192}), '-8192,0,8192,8192')
  assert.equal(normalizeQueryScope('-8192.00001,-8191.99999,8192.00001,8191.99999'), '-8192,-8192,8192,8192')
  assert.equal(normalizeQueryScope(null), 'all')
  assert.equal(normalizeQueryScope('all'), 'all')
  assert.equal(normalizeQueryScope(''), '')
  // 同一局部空视区静止（含微小浮点抖动）：正常显示「当前画面里没有星」。
  const localA = '-8192,-8192,8192,8192'
  const localB = '-32768,-32768,32768,32768'
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: localA, viewRegion: localA})), 'offscreen')
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: localA, viewRegion: {minX: -8192.00002, minY: -8191.99998, maxX: 8192.00001, maxY: 8191.99999}})), 'offscreen', '同视区微小浮点抖动仍判同一范围')
  // 从空局部视区 A 拉远/平移到另一局部视区 B：160ms 请求合并窗口（loading=false）和请求在途（loading=true）均不得沿用旧空态。
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: localA, viewRegion: localB, loading: false})), '', '160ms 请求合并窗口期不得沿用旧局部空提示')
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: localA, viewRegion: localB, loading: true})), '', '局部新范围在途期不得沿用旧局部空提示')
  // 新局部视区 B 到达后：若无星继续报 offscreen，若有星则隐藏。
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: localB, viewRegion: localB})), 'offscreen')
  assert.equal(galaxyStatus(status({total: 3, visible: 3, snapshotRegion: localB, viewRegion: localB})), '')
  // 点「回到全景」：镜头已回全景范围，但最近一次成功快照仍是局部，等待期间两种提示都不出现。
  const waiting = status({total: 0, visible: 0, snapshotRegion: localA, viewRegion: 'all', loading: true})
  assert.notEqual(galaxyStatus(waiting), 'empty')
  assert.equal(galaxyStatus(waiting), '')
  assert.equal(galaxyStatus({...waiting, loading: false}), '', '请求被排队、尚未发出时同样不能误报')
  // 全景回执到达：真的为空才宣布无反馈。
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: 'all', viewRegion: 'all'})), 'empty')
  assert.equal(galaxyStatus(status({total: 6, visible: 6})), '')
  // 反向：全景快照为空后放大，等待局部回执期间不报任何提示。
  assert.equal(galaxyStatus(status({total: 0, visible: 0, snapshotRegion: 'all', viewRegion: localA})), '')
})

test('P6 重试等待期间：保留错误提示，不出现「确实无反馈」', () => {
  const retrying = status({error: '连接失败', ready: true, total: 0, visible: 0, loading: true})
  assert.equal(galaxyStatus(retrying), 'error')
  assert.equal(galaxyStatus(status({error: '连接失败', ready: false, total: 0, visible: 0, loading: true, snapshotRegion: ''})), 'error')
  assert.equal(galaxyStatus({...retrying, loading: false}), 'error', '再次失败：错误保留')
  assert.equal(galaxyStatus(status({error: '', total: 6, visible: 6})), '')
  assert.equal(galaxyStatus(status({error: '', total: 0, visible: 0})), 'empty')
})

test('P3 与 P2-4 聚合星与私有审核状态：仅公开且有明确聚合依据的星才显示聚合提示，私有待审/转人工绝不误报', () => {
  const size = {width: 950, height: 746}
  const camera = {...HOME_CAMERA, zoom: 8}
  const near = {x: 800, y: 500, z: 0}
  const far = {x: -60000, y: 30000, z: 1000}
  assert.equal(isInView(near, camera, size), true)
  assert.equal(isInView(far, camera, size), false)
  assert.equal(isInView(far, HOME_CAMERA, size), true, '全景时外围星在画面内')
  assert.equal(isInView(near, camera, {width: 0, height: 0}), false)

  const detailOf = (id, x, y, reviewState = 'approved', withdrawn = false) => ({
    starId: String(id), x, y, z: 0, withdrawn, original: {reviewState},
  })
  const nearGroup = [{...star('5001', 1800, 900, 0), count: 6}]
  const farGroup = [{...star('5001', 70000, -30000, 0), count: 6}]
  const localBounds = {minX: -16384, minY: -16384, maxX: 16384, maxY: 16384}

  // 1) 待审私有详情 + 画面内未出现在个人星点列表 + 快照中紧邻/同坐标有聚合点 → 不报 selected-grouped。
  const pendingDetail = detailOf('9101', 1800, 900, 'pending')
  assert.equal(isPublicFeedback(pendingDetail.original.reviewState, pendingDetail.withdrawn), false)
  assert.equal(isCoveredByAggregate(pendingDetail, nearGroup, null), false)
  assert.equal(galaxyStatus(status({
    selectedId: '9101', selectedPublic: false, selectedVisible: true, selectedClickable: false, selectedGrouped: true,
  })), '', '待审私有反馈即使传了聚合标记也绝不报 selected-grouped')

  // 2) 转人工私有详情 + 画面内未出现在个人星点列表 + 快照有聚合点 → 不报 selected-grouped。
  const manualDetail = detailOf('9102', 1800, 900, 'manual')
  assert.equal(isPublicFeedback(manualDetail.original.reviewState, manualDetail.withdrawn), false)
  assert.equal(isCoveredByAggregate(manualDetail, nearGroup, null), false)
  assert.equal(galaxyStatus(status({
    selectedId: '9102', selectedPublic: false, selectedVisible: true, selectedClickable: false, selectedGrouped: false,
  })), '', '转人工私有反馈不报 selected-grouped')

  // 驳回 / 撤回同样不归入公开聚合体系。
  assert.equal(isCoveredByAggregate(detailOf('9103', 1800, 900, 'rejected'), nearGroup, null), false)
  assert.equal(isCoveredByAggregate(detailOf('9104', 1800, 900, 'approved', true), nearGroup, null), false)

  // 3) 公开详情未直接列出，但聚合点在无关远处（全景与局部均无聚合依据）→ 不报 selected-grouped。
  const approvedNear = detailOf('5002', 1500, 1000, 'approved')
  assert.equal(isCoveredByAggregate(approvedNear, farGroup, null), false, '全景下远处无关聚合点不构成聚合依据')
  assert.equal(isCoveredByAggregate(approvedNear, [{...star('5001', -9000, 6000, 0), count: 6}], localBounds), false, '局部跨度下远处聚合点不构成聚合依据')
  assert.equal(galaxyStatus(status({
    selectedId: '5002', selectedPublic: true, selectedVisible: true, selectedClickable: false,
    selectedGrouped: isCoveredByAggregate(approvedNear, farGroup, null),
  })), '')

  // 4) 公开详情确有近邻聚合依据：在视区内报 selected-grouped；移出视区报 selected-offscreen 且 includeStar 按成员坐标拉回。
  assert.equal(isCoveredByAggregate(approvedNear, nearGroup, null), true, '全景近邻聚合点构成明确聚合依据')
  assert.equal(isCoveredByAggregate(approvedNear, nearGroup, localBounds), true, '局部近邻聚合点构成明确聚合依据')
  assert.equal(galaxyStatus(status({
    selectedId: '5002', selectedPublic: true, selectedVisible: true, selectedClickable: false, selectedGrouped: true,
  })), 'selected-grouped')
  const approvedFar = detailOf('5009', -60000, 30000, 'approved')
  const farCluster = [{...star('5008', -58500, 29000, 0), count: 5}]
  assert.equal(isCoveredByAggregate(approvedFar, farCluster, null), true)
  assert.equal(galaxyStatus(status({
    selectedId: '5009', selectedPublic: true, selectedVisible: false, selectedClickable: false, selectedGrouped: true,
  })), 'selected-offscreen')
  const pulled = includeStar(camera, approvedFar, size)
  assert.equal(isInView(approvedFar, pulled, size), true, '回到这颗星按成员自己的坐标把星拉回画面')
  assert.equal(galaxyStatus(status({
    selectedId: '5002', selectedPublic: true, selectedVisible: true, selectedClickable: true, selectedGrouped: true,
  })), '', '直接可点选时不提示聚合隐藏')
})

test('P2-3 候选列表焦点接替：移除聚焦项后按原索引或末项接替，翻页回退落到有效项，列表外不抢焦点', () => {
  // 移除中间项：接替同索引后继项。
  assert.equal(resolveFocusFallback(['6001', '6002', '6003'], ['6001', '6003'], '6002'), '6003')
  // 移除末项：超出范围则落到当前页最后一项。
  assert.equal(resolveFocusFallback(['6001', '6002', '6003'], ['6001', '6002'], '6003'), '6002')
  // 第 2 页唯一项被移除、页码回退到第 1 页：落到第 1 页有效候选按钮。
  assert.equal(resolveFocusFallback(['6013'], ['6001', '6002', '6003'], '6013'), '6001')
  // 聚焦项仍在当前页：不移动焦点。
  assert.equal(resolveFocusFallback(['6001', '6002'], ['6001', '6002', '6003'], '6002'), null)
  // 焦点原本不在候选按钮上：绝不抢焦点。
  assert.equal(resolveFocusFallback(['6001', '6002'], ['6002'], ''), null)
  // 当前页全部移除：返回空串，由列表/外层回退容器或星图。
  assert.equal(resolveFocusFallback(['6001'], [], '6001'), '')
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

test('P1 与 P2-2 组合态 useGalaxyMap：空局部视区切换另一局部视区或回全景，防抖合并期与在途期均不沿用旧空态', async t => {
  const size = {width: 960, height: 640}
  const delayedLocalB = later()
  const delayedLocalC = later()
  const delayedPanorama = later()
  let step = 0
  const {map} = setupMap(t, (op, input) => {
    assert.equal(op, 'Map')
    step++
    // 1) 全景含聚合星（count>1），保证缩放后触发局部范围查询。
    if (step === 1) return {version: 'all#1', unchanged: false, points: [{...star('4001', 70000, -30000, 0), count: 5}]}
    // 2) 局部视区 A（zoom=20）为空。
    if (step === 2) return {version: 'localA#1', unchanged: false, points: []}
    // 3) 局部视区 B（zoom=6）延迟返回有星。
    if (step === 3) return delayedLocalB.promise
    // 4) 局部视区 C（zoom=12）延迟返回仍无星。
    if (step === 4) return delayedLocalC.promise
    // 5) 回到全景延迟返回有星。
    assert.equal(input.minX, undefined)
    return delayedPanorama.promise
  })
  const currentStatus = () => galaxyStatus({
    foreground: true, ready: map.ready.value, error: map.error.value,
    total: map.points.value.length, visible: map.points.value.length,
    snapshotRegion: map.snapshotRegion.value, viewRegion: map.viewRegion.value,
    loading: map.loading.value, selectedId: '', selectedPublic: false, selectedVisible: false, selectedClickable: false, selectedGrouped: false,
  })
  map.updateCamera(HOME_CAMERA, size)
  await map.retry()
  assert.equal(map.snapshotRegion.value, 'all')

  // 放大到空局部视区 A（zoom=20）
  map.updateCamera({...HOME_CAMERA, zoom: 20}, size)
  await map.retry()
  const regionA = map.snapshotRegion.value
  assert.notEqual(regionA, 'all')
  assert.equal(map.viewRegion.value, regionA)
  assert.equal(map.points.value.length, 0)
  assert.equal(currentStatus(), 'offscreen', '空局部视区静止时正常提示当前画面里没有星')

  // 拉远到另一局部视区 B（zoom=6，仍非全景）：160ms 防抖合并窗口期与请求在途期均不沿用旧空态。
  map.updateCamera({...HOME_CAMERA, zoom: 6}, size)
  const regionB = map.viewRegion.value
  assert.notEqual(regionB, 'all')
  assert.notEqual(regionB, regionA)
  assert.equal(map.loading.value, false, '刚切换镜头仍在 160ms 请求合并窗口内')
  assert.equal(currentStatus(), '', '160ms 请求合并窗口内绝不沿用旧视区空提示')
  // 等 160ms 合并定时器真正发出局部视区 B 请求
  await new Promise(r => setTimeout(r, 200))
  assert.equal(map.loading.value, true)
  assert.equal(currentStatus(), '', '局部视区 B 回执在途期绝不沿用旧空提示')
  delayedLocalB.resolve({version: 'localB#1', unchanged: false, points: [star('4002', 5000, 3000, 0)]})
  await tick()
  await tick()
  assert.equal(map.snapshotRegion.value, regionB)
  assert.equal(map.points.value.length, 1)
  assert.equal(currentStatus(), '')

  // 再切到局部视区 C（zoom=12）且回执确实无星：新快照到达后正常恢复 offscreen。
  map.updateCamera({...HOME_CAMERA, zoom: 12}, size)
  assert.equal(currentStatus(), '')
  const pendingC = map.retry()
  await tick()
  assert.equal(currentStatus(), '')
  delayedLocalC.resolve({version: 'localC#1', unchanged: false, points: []})
  await pendingC
  assert.equal(map.snapshotRegion.value, map.viewRegion.value)
  assert.equal(currentStatus(), 'offscreen', '新局部视区确实无星时回执到达后正常显示 offscreen')

  // 点「回到全景」：镜头回到 HOME_CAMERA，但全景回执未到（原有 P1 不退化）。
  map.updateCamera(HOME_CAMERA, size)
  assert.equal(map.viewRegion.value, 'all')
  assert.notEqual(map.snapshotRegion.value, 'all')
  assert.equal(currentStatus(), '', '防抖等待期间不得误报「确实无反馈」')
  const pending = map.retry()
  await tick()
  assert.equal(map.loading.value, true)
  assert.notEqual(map.snapshotRegion.value, 'all')
  assert.equal(currentStatus(), '', '全景回执在途期间不得误报「确实无反馈」')
  delayedPanorama.resolve({version: 'all#2', unchanged: false, points: [star('4001', 1000, 500, 0)]})
  await pending
  assert.equal(map.snapshotRegion.value, 'all')
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
    snapshotRegion: map.snapshotRegion.value, viewRegion: map.viewRegion.value,
    loading: map.loading.value, selectedId: '', selectedPublic: false, selectedVisible: false, selectedClickable: false, selectedGrouped: false,
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

test('P2-1 组合态 useGalaxyMap.retry：慢重试期间连续切换 3 次视区，保留最新视区补查且转圈持续到有效请求结算（覆盖成功与再次失败）', async t => {
  const size = {width: 960, height: 640}
  const regionOf = (input) => input.minX === undefined ? 'all' : `${input.minX},${input.minY},${input.maxX},${input.maxY}`
  // 分支 A：连续切换 3 次视区后，第 4 个最新视区请求延迟成功。
  {
    const pending = []
    let failInitial = true
    const {map, calls} = setupMap(t, (_op, input) => {
      if (failInitial) return Promise.reject(new FeedbackError('requestFailed', '连接反馈星系失败，请重试。'))
      const d = later()
      pending.push({region: regionOf(input), d})
      return d.promise
    })
    // 首次 503 失败（无完整目录，每次缩放均触发不同局部边界）。
    map.updateCamera(HOME_CAMERA, size)
    await map.retry()
    assert.equal(map.error.value, '连接反馈星系失败，请重试。')

    failInitial = false
    let retrySettled = false
    const retryPromise = map.retry().then(() => { retrySettled = true })
    await tick()
    assert.equal(pending.length, 1)
    assert.equal(pending[0].region, 'all')
    assert.equal(map.loading.value, true)
    assert.equal(retrySettled, false)

    // 切视区 1（zoom=4）使第 1 次在途请求作废；前序作废回执到达时 retry 绝不结束，立即补发视区 1。
    map.updateCamera({...HOME_CAMERA, zoom: 4}, size)
    const region1 = map.viewRegion.value
    assert.notEqual(region1, 'all')
    pending[0].d.resolve({version: 'stale#0', unchanged: false, points: []})
    await tick(); await tick()
    assert.equal(retrySettled, false, '第 1 次作废后 retry 仍未结束')
    assert.equal(map.loading.value, true, '第 1 次作废后 loading 保持为真')
    assert.equal(pending.length, 2, '已自动补发第 1 次新视区查询')
    assert.equal(pending[1].region, region1)

    // 切视区 2（zoom=8）后再在防抖窗口内紧接着切视区 3（zoom=16），使第 2 次在途请求作废。
    map.updateCamera({...HOME_CAMERA, zoom: 8}, size)
    map.updateCamera({...HOME_CAMERA, zoom: 16}, size)
    const region3 = map.viewRegion.value
    assert.notEqual(region3, region1)
    pending[1].d.reject(new FeedbackError('requestFailed', '已作废的旧错'))
    await tick(); await tick()
    assert.equal(retrySettled, false, '第 2 次作废后 retry 仍未结束（不再在 2 次后提前退出）')
    assert.equal(map.loading.value, true, '第 2 次作废后 loading 保持为真')
    assert.equal(pending.length, 3, '最新视区补查必须发出且未被清掉')
    assert.equal(pending[2].region, region3)

    // 再切一次视区 4（zoom=32），使第 3 次在途请求再次作废。
    map.updateCamera({...HOME_CAMERA, zoom: 32}, size)
    const region4 = map.viewRegion.value
    assert.notEqual(region4, region3)
    pending[2].d.resolve({version: 'stale#2', unchanged: false, points: []})
    await tick(); await tick()
    assert.equal(retrySettled, false, '第 3 次作废后 retry 仍持续等待最新视区')
    assert.equal(map.loading.value, true)
    assert.equal(pending.length, 4, '第 4 个最新视区请求已发出')
    assert.equal(pending[3].region, region4)

    // 最新视区 4 延迟成功返回：错误清除、星图展示、retry 正常结算。
    pending[3].d.resolve({version: 'local4#ok', unchanged: false, points: [star('8001', 200, 150, 0)]})
    await retryPromise
    assert.equal(retrySettled, true)
    assert.equal(map.loading.value, false)
    assert.equal(map.error.value, '')
    assert.equal(map.snapshotRegion.value, region4)
    assert.deepEqual(map.points.value.map(p => p.starId), ['8001'])
    assert.equal(calls.length, 5)
  }

  // 分支 B：连续切换 3 次视区后，最新视区有效请求延迟再次失败 → 转圈结束、错误保留、可再次重试成功。
  {
    const pending = []
    let failInitial = true
    const {map} = setupMap(t, (_op, input) => {
      if (failInitial) return Promise.reject(new FeedbackError('requestFailed', '初始 503'))
      const d = later()
      pending.push({region: regionOf(input), d})
      return d.promise
    })
    map.updateCamera(HOME_CAMERA, size)
    await map.retry()
    assert.equal(map.error.value, '初始 503')

    failInitial = false
    let retrySettled = false
    const retryPromise = map.retry().then(() => { retrySettled = true })
    await tick()
    assert.equal(pending.length, 1)

    // 连续 3 次在途作废（超越旧 2 次上限）
    for (const zoom of [4, 10, 24]) {
      map.updateCamera({...HOME_CAMERA, zoom}, size)
      const prev = pending[pending.length - 1]
      prev.d.resolve({version: 'stale', unchanged: false, points: []})
      await tick(); await tick()
      assert.equal(retrySettled, false)
      assert.equal(map.loading.value, true)
    }
    const latestRegion = map.viewRegion.value
    assert.equal(pending.length, 4)
    assert.equal(pending[3].region, latestRegion)

    // 最新视区请求再次 503 失败：retry 结束、loading 复位、保留最新错误提示、允许继续点击重试。
    pending[3].d.reject(new FeedbackError('requestFailed', '最新视区再次 503'))
    await retryPromise
    assert.equal(retrySettled, true)
    assert.equal(map.loading.value, false)
    assert.equal(map.error.value, '最新视区再次 503')

    // 再次点击重试可正常恢复。
    const recovery = map.retry()
    await tick()
    assert.equal(map.loading.value, true)
    assert.equal(pending.length, 5)
    assert.equal(pending[4].region, latestRegion)
    pending[4].d.resolve({version: 'recovered#1', unchanged: false, points: [star('8002', 100, 100, 0)]})
    await recovery
    assert.equal(map.loading.value, false)
    assert.equal(map.error.value, '')
    assert.equal(map.snapshotRegion.value, latestRegion)
    assert.deepEqual(map.points.value.map(p => p.starId), ['8002'])
  }
})

test('日间水墨风调色规格：松烟墨灰阶近单色且控制峰值墨深，深色主题保持不变', () => {
  const {light, dark} = GALAXY_THEMES
  // 日间云雾色标均为低饱和近单色松烟墨灰（通道最大差 <= 16），且最深色标保持中浅灰阶避免 200 条糊黑。
  for (const [, r, g, b] of light.nebula.stops) {
    assert.ok(Math.max(r, g, b) - Math.min(r, g, b) <= 16, `日间水墨色标应为低饱和松烟灰：${r},${g},${b}`)
    assert.ok(Math.min(r, g, b) >= 100, `日间水墨色标峰值不得过黑：${r},${g},${b}`)
  }
  assert.equal(light.nebula.blend, 'over')
  assert.ok(light.coreCushion, '日间焦墨星核保留宣纸柔光托底')
  // 深色主题保持原有调色与发光混合模式不变。
  assert.equal(dark.stage, '#03060d')
  assert.equal(dark.nebula.blend, 'screen')
  assert.deepEqual(dark.starColors, {feedback: '#8ad8ff', accepted: '#c4a4ff', implemented: '#ffca80'})
})

test('复审 P2-1 换账号交接：旧账号慢重试/普通请求在途切账号、连切账号与登出再登录均不吞新账号首次查询', async t => {
  t.after(() => { sessionKey.value = 'session-1' })
  const size = {width: 960, height: 640}
  const waitMacro = (ms = 25) => new Promise(resolve => setTimeout(resolve, ms))

  // 1) 旧账号慢重试在途 → 切换到新账号 A：旧重试立即退场、绝不吞掉新账号 A 首次查询，无需等轮询立即发出并结算。
  {
    sessionKey.value = 'acct-old'
    const pendingBySession = []
    let failOldInitial = true
    const {map, stop} = setupMap(t, () => {
      const current = sessionKey.value
      if (current === 'acct-old' && failOldInitial) {
        return Promise.reject(new FeedbackError('requestFailed', '旧账号初始 503'))
      }
      const d = later()
      pendingBySession.push({session: current, d})
      return d.promise
    })
    map.updateCamera(HOME_CAMERA, size)
    await map.retry()
    assert.equal(map.error.value, '旧账号初始 503')

    failOldInitial = false
    let oldRetrySettled = false
    const oldRetry = map.retry().then(() => { oldRetrySettled = true })
    await tick()
    assert.equal(pendingBySession.length, 1)
    assert.equal(pendingBySession[0].session, 'acct-old')
    assert.equal(map.loading.value, true)

    // 模拟旧账号曾在放大态，确保换账号重置镜头也不会被 160ms 防抖或旧重试吞掉首次查询。
    map.updateCamera({...HOME_CAMERA, zoom: 8}, size)
    // 旧重试在途时切换到新账号 A：
    sessionKey.value = 'acct-a'
    await tick()
    assert.equal(oldRetrySettled, true, '换账号后旧账号的 retry Promise 应立即结束，不阻塞新账号')
    assert.equal(map.error.value, '', '换账号后旧错误立即清空')
    assert.equal(map.points.value.length, 0)
    // 等待 0ms 调度宏任务（远小于 2s 轮询周期）：新账号 A 必须立即发出首次查询，且在旧请求尚未回执时也不被阻塞。
    await waitMacro(30)
    assert.equal(pendingBySession.length, 2, '新账号 A 必须立即发出首次地图查询（不能被旧重试吞掉）')
    assert.equal(pendingBySession[1].session, 'acct-a')
    assert.equal(map.loading.value, true)

    // 旧账号慢请求此时才返回错误（或旧数据）：绝不覆盖新账号状态，也不提前复位新账号 loading。
    pendingBySession[0].d.reject(new FeedbackError('stale', '旧账号迟到错误'))
    await tick(); await tick()
    assert.equal(map.error.value, '', '旧账号迟到错误不得污染新账号 A')
    assert.equal(map.loading.value, true, '旧账号收尾不得提前复位新账号 A 的 loading')

    // 新账号 A 首次查询返回：正常渲染星图且 loading 复位。
    pendingBySession[1].d.resolve({version: 'a#1', unchanged: false, points: [star('9001', 120, 80, 0)]})
    await oldRetry
    await tick(); await tick()
    assert.equal(map.loading.value, false)
    assert.equal(map.ready.value, true)
    assert.equal(map.error.value, '')
    assert.equal(map.snapshotRegion.value, 'all')
    assert.deepEqual(map.points.value.map(p => p.starId), ['9001'])
    stop()
  }

  // 2) 旧账号慢重试在途时连续切换账号（旧 → A → B，含已发出 A 后再切 B）与先切空会话再切回 B：
  // 绝不对空会话发请求，最终活跃账号 B 稳定发起并结算首次查询，无残留旧数据或错误。
  {
    sessionKey.value = 'acct-1'
    const requests = []
    let failInitial = true
    const {map, stop} = setupMap(t, () => {
      const current = sessionKey.value
      if (current === 'acct-1' && failInitial) {
        return Promise.reject(new FeedbackError('requestFailed', 'acct-1 503'))
      }
      const d = later()
      requests.push({session: current, d})
      return d.promise
    })
    map.updateCamera(HOME_CAMERA, size)
    await map.retry()
    failInitial = false
    const pOld = map.retry()
    await tick()
    assert.equal(requests.length, 1)
    assert.equal(requests[0].session, 'acct-1')

    // 同步连切 acct-1 → acct-a → ''（登出空会话）：不得对空会话或被取消的 acct-a 发请求。
    sessionKey.value = 'acct-a'
    sessionKey.value = ''
    await waitMacro(30)
    assert.equal(requests.length, 1, '切到空会话期间绝不发起任何地图请求')
    assert.equal(map.loading.value, false, '空会话下 loading 保持为 false')
    // 旧请求在空会话期间返回旧数据：同样不得写入或发起空会话查询。
    requests[0].d.resolve({version: 'old#stale', unchanged: false, points: [star('1', 0, 0, 0)]})
    await pOld
    await waitMacro(30)
    assert.equal(requests.length, 1, '旧请求在空会话期间结算后仍不得对空会话发请求')
    assert.equal(map.points.value.length, 0)
    assert.equal(map.loading.value, false)

    // 再切回新账号 B：立即发起首次查询；在 B 在途时再切到账号 C，B 的迟到结果不得污染 C。
    sessionKey.value = 'acct-b'
    await waitMacro(30)
    assert.equal(requests.length, 2)
    assert.equal(requests[1].session, 'acct-b')
    assert.equal(map.loading.value, true)

    sessionKey.value = 'acct-c'
    await waitMacro(30)
    assert.equal(requests.length, 3)
    assert.equal(requests[2].session, 'acct-c')
    assert.equal(map.loading.value, true)

    // acct-b 迟到成功返回：丢弃；acct-c 结算后正常显示 acct-c 数据。
    requests[1].d.resolve({version: 'b#stale', unchanged: false, points: [star('9100', 500, 500, 0)]})
    await tick(); await tick()
    assert.equal(map.points.value.length, 0, '被取代的 acct-b 数据不得残留到 acct-c')
    assert.equal(map.loading.value, true)

    requests[2].d.resolve({version: 'c#1', unchanged: false, points: [star('9200', -300, 240, 0)]})
    await tick(); await tick()
    assert.equal(map.loading.value, false)
    assert.equal(map.ready.value, true)
    assert.equal(map.error.value, '')
    assert.deepEqual(map.points.value.map(p => p.starId), ['9200'])
    assert.ok(requests.every(r => r.session !== ''), '所有已发请求均不包含空会话')
    stop()
  }

  // 3) 旧账号普通请求（非重试）在途时切换账号：新账号首次查询同样正常衔接且不被旧请求阻塞或污染。
  {
    sessionKey.value = 'acct-normal-1'
    const requests = []
    const {map, stop} = setupMap(t, () => {
      const d = later()
      requests.push({session: sessionKey.value, d})
      return d.promise
    })
    map.updateCamera(HOME_CAMERA, size)
    await waitMacro(30)
    assert.equal(requests.length, 1)
    assert.equal(requests[0].session, 'acct-normal-1')
    assert.equal(map.loading.value, true)

    // 普通请求在途期间切换账号：
    sessionKey.value = 'acct-normal-2'
    await waitMacro(30)
    assert.equal(requests.length, 2, '普通请求在途切账号后新账号立即发出首次查询')
    assert.equal(requests[1].session, 'acct-normal-2')
    assert.equal(map.loading.value, true)

    requests[0].d.resolve({version: 'n1#late', unchanged: false, points: [star('100', 0, 0, 0)]})
    await tick(); await tick()
    assert.equal(map.points.value.length, 0)
    assert.equal(map.loading.value, true)

    requests[1].d.resolve({version: 'n2#ok', unchanged: false, points: [star('200', 50, 60, 0)]})
    await tick(); await tick()
    assert.equal(map.loading.value, false)
    assert.equal(map.ready.value, true)
    assert.deepEqual(map.points.value.map(p => p.starId), ['200'])
    stop()
  }

  // 4) 同账号内并发/重复调用 retry() 复用同一在途重试，不发出重复请求。
  {
    sessionKey.value = 'acct-dup-retry'
    let firstFail = true
    const pending = []
    const {map, stop} = setupMap(t, () => {
      if (firstFail) return Promise.reject(new FeedbackError('requestFailed', '503'))
      const d = later()
      pending.push(d)
      return d.promise
    })
    map.updateCamera(HOME_CAMERA, size)
    await map.retry()
    firstFail = false
    const r1 = map.retry()
    const r2 = map.retry()
    assert.equal(r1, r2, '同账号并发调用 retry 复用同一个 Promise')
    await tick()
    assert.equal(pending.length, 1, '同账号并发 retry 只发出一次请求')
    pending[0].resolve({version: 'dup#ok', unchanged: false, points: [star('300', 10, 10, 0)]})
    await Promise.all([r1, r2])
    assert.equal(map.loading.value, false)
    assert.equal(map.error.value, '')
    stop()
  }
})

test('复审 P2-2 精确 32×32 网格分桶与快照因果证据：相邻桶绝不误报，刚过审未入快照不误报，快照更新后准确切换', async t => {
  t.after(() => { sessionKey.value = 'session-1' })
  assert.equal(AGGREGATE_GRID_WIDTH, 32)
  const detailOf = (id, x, y, reviewState = 'approved', withdrawn = false) => ({
    starId: String(id), x, y, z: 0, withdrawn, original: {reviewState},
  })

  // 1) 与 Go 服务端 map_geometry.go:48-52 完全一致的 32×32 网格分桶边界核对：
  // 1a) 服务端单测同款局部边界 {0, 0, 3199, 3199}：每格恰好 100×100，(99, 99) 与 (100, 99) 仅差 1 单位但分属相邻桶。
  const local3200 = {minX: 0, minY: 0, maxX: 3199, maxY: 3199}
  assert.deepEqual(aggregateCellOf({x: 23, y: 48}, local3200), {col: 0, row: 0, index: 0})
  assert.deepEqual(aggregateCellOf({x: 74, y: 87}, local3200), {col: 0, row: 0, index: 0})
  assert.deepEqual(aggregateCellOf({x: 99, y: 99}, local3200), {col: 0, row: 0, index: 0})
  assert.deepEqual(aggregateCellOf({x: 100, y: 99}, local3200), {col: 1, row: 0, index: 1})
  assert.deepEqual(aggregateCellOf({x: 99, y: 100}, local3200), {col: 0, row: 1, index: 32})
  assert.deepEqual(aggregateCellOf({x: 3199, y: 3199}, local3200), {col: 31, row: 31, index: 1023})
  assert.equal(aggregateCellOf({x: 3200, y: 0}, local3200), null, '超出闭区间右界不在任何桶内')
  assert.equal(aggregateCellOf({x: -1, y: 0}, local3200), null, '超出闭区间左界不在任何桶内')

  // 1b) 全景边界 [-100000, 100000]（span = 200001）：x=0 落第 15 列，x=1 落第 16 列，跨网格线仅差 1 单位。
  assert.deepEqual(aggregateCellOf({x: -100000, y: -100000}, null), {col: 0, row: 0, index: 0})
  assert.deepEqual(aggregateCellOf({x: 100000, y: 100000}, null), {col: 31, row: 31, index: 1023})
  assert.equal(aggregateCellOf({x: 0, y: 0}, null).col, 15)
  assert.equal(aggregateCellOf({x: 1, y: 0}, null).col, 16)
  assert.equal(aggregateCellOf({x: 6250, y: 0}, null).col, 16)
  assert.equal(aggregateCellOf({x: 6251, y: 0}, null).col, 17)
  // 超出 GALAXY_EDGE 的输入按服务端 intersectGalaxy 截断到 [-100000, 100000]。
  assert.deepEqual(aggregateGridBounds({minX: -120000, minY: -120000, maxX: 120000, maxY: 120000}),
    {minX: -100000, minY: -100000, maxX: 100000, maxY: 100000})

  // 2) 全景与局部视区下：同桶 count > 1 返回 true；跨线 1 单位的相邻桶 count > 1 绝不误报；同桶 count === 1 / 省略 count 不误报。
  const panoAgg = [{...star('7001', 0, 0, 0), count: 5}] // col 15, row 15
  assert.equal(isCoveredByAggregate(detailOf('7002', -1000, -500), panoAgg, null), true, '全景同桶 (15,15) count>1 正确匹配')
  assert.equal(isCoveredByAggregate(detailOf('7003', 1, 0), panoAgg, null), false, '全景跨线 1 单位相邻桶 (16,15) 绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7004', 0, 1), panoAgg, null), false, '全景跨线 1 单位相邻桶 (15,16) 绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7005', -1000, -500), [{...star('7001', 0, 0, 0), count: 1}], null), false, '同桶 count===1 独立星绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7006', -1000, -500), [star('7001', 0, 0, 0)], null), false, '同桶未聚合单星（省略 count）绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7001', 0, 0), panoAgg, null), false, '代表星自身已在快照中绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7002', -1000, -500), [], null), false, '空快照绝不误报')

  // 局部视区 {0,0,3199,3199} 与对称局部 {-16384,-16384,16384,16384}：
  const localAgg = [{...star('7101', 99, 99, 0), count: 4}] // col 0, row 0
  assert.equal(isCoveredByAggregate(detailOf('7102', 23, 48), localAgg, local3200), true, '局部同桶 (0,0) count>1 正确匹配')
  assert.equal(isCoveredByAggregate(detailOf('7103', 100, 99), localAgg, local3200), false, '局部跨线 1 单位相邻桶 (1,0) 绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7104', 99, 100), localAgg, local3200), false, '局部跨线 1 单位相邻桶 (0,1) 绝不误报')
  const symLocal = {minX: -16384, minY: -16384, maxX: 16384, maxY: 16384}
  assert.equal(isCoveredByAggregate(detailOf('7202', 0, 0), [{...star('7201', 1, 0, 0), count: 3}], symLocal), false, '对称局部边界跨线 x=0 vs x=1 绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7203', 100, 100), [{...star('7201', 1, 0, 0), count: 3}], symLocal), false, '对称局部边界跨行 y=0(row=15) vs y=100(row=16) 绝不误报')
  assert.equal(isCoveredByAggregate(detailOf('7204', 100, -100), [{...star('7201', 1, 0, 0), count: 3}], symLocal), true, '对称局部同桶 (16,15) 正确匹配')
  assert.equal(isCoveredByAggregate(detailOf('7205', 100, 100), [{...star('7206', 1, 1, 0), count: 3}], symLocal), true, '对称局部同桶 (16,16) 正确匹配')

  // 3) 私有审态与已撤回反馈在同桶 count > 1 且快照已确认下同样绝不误报。
  for (const [state, withdrawn] of [['pending', false], ['manual', false], ['rejected', false], ['approved', true]]) {
    assert.equal(
      isCoveredByAggregate(detailOf('7300', -1000, -500, state, withdrawn), panoAgg, null, {snapshotConfirmed: true}),
      false,
      `私有/撤回状态 (${state}, withdrawn=${withdrawn}) 绝不误报聚合提示`,
    )
  }

  // 4) 刚过审详情已可见但地图快照更新前 vs 更新后（结合 useGalaxyMap.snapshotSyncedOrder 真实因果链路）：
  // 即使刚过审的星与已有 count>1 聚合星落在完全相同的 32×32 桶内，
  // 只要当前快照（含早已在途的旧快照）发起早于过审观测序号，一律不报 selected-grouped；
  // 直到在过审观测之后发起的新 Map 快照结算后，才按「独立星直接可点」或「同桶聚合星代表」准确切换。
  {
    sessionKey.value = 'acct-causal'
    const size = {width: 960, height: 640}
    const queue = []
    const {map} = setupMap(t, () => {
      const d = later()
      queue.push(d)
      return d.promise
    })
    const statusFor = (target, approvedOrder) => {
      const clickable = map.points.value.some(p => p.starId === target.starId)
      const confirmed = approvedOrder > 0 && map.snapshotSyncedOrder.value >= approvedOrder
      return galaxyStatus({
        foreground: true, ready: map.ready.value, error: map.error.value,
        total: map.points.value.length, visible: map.points.value.length,
        snapshotRegion: map.snapshotRegion.value, viewRegion: map.viewRegion.value,
        loading: map.loading.value,
        selectedId: target.starId, selectedPublic: true, selectedVisible: true,
        selectedClickable: clickable,
        selectedGrouped: isCoveredByAggregate(target, map.points.value, map.snapshotBounds.value, {snapshotConfirmed: confirmed}),
      })
    }

    // 4a) 初始快照含同桶聚合星 8001 (0, 0, count=6)；随后发起一轮旧轮询（在过审之前启动）。
    map.updateCamera(HOME_CAMERA, size)
    await new Promise(r => setTimeout(r, 25))
    queue[0].resolve({version: 'v1', unchanged: false, points: [{...star('8001', 0, 0, 0), count: 6}]})
    await tick(); await tick()
    assert.ok(map.snapshotSyncedOrder.value > 0)

    map.refreshNow()
    await new Promise(r => setTimeout(r, 25))
    assert.equal(queue.length, 2, '过审前的旧轮询请求已在途')

    // 4b) 新星 8002 (-500, -500，与 8001 同属全景桶 15,15) 刚通过审核、详情已可见：
    const newlyApproved = detailOf('8002', -500, -500, 'approved')
    const approvedOrder = nextSnapshotOrder()
    assert.equal(statusFor(newlyApproved, approvedOrder), '', '旧快照早于过审观测：即使同桶存在 count>1 也绝不误报聚合提示')

    // 过审前已发起的旧轮询晚于详情到达：其 requestOrder 仍小于 approvedOrder，仍不得误报！
    queue[1].resolve({version: 'v1', unchanged: true, points: []})
    await tick(); await tick()
    assert.ok(map.snapshotSyncedOrder.value < approvedOrder, '旧在途请求结算后快照因果序号仍早于过审观测')
    assert.equal(statusFor(newlyApproved, approvedOrder), '', '旧在途请求晚到不得触发误报')

    // 4c) 过审观测之后发起新快照查询 → 分支 A：快照将其收录进同桶聚合星（count 变为 7），此时才显示 selected-grouped。
    map.refreshNow()
    await new Promise(r => setTimeout(r, 25))
    assert.equal(queue.length, 3)
    queue[2].resolve({version: 'v2', unchanged: false, points: [{...star('8001', 0, 0, 0), count: 7}]})
    await tick(); await tick()
    assert.ok(map.snapshotSyncedOrder.value >= approvedOrder)
    assert.equal(statusFor(newlyApproved, approvedOrder), 'selected-grouped', '过审后快照确认同桶聚合覆盖：准确显示 selected-grouped')

    // 4d) 分支 B：再次过审的新星 8003 (-800, -800) 快照更新后以独立星现身：更新前不误报，更新后直接可点且无聚合提示。
    const standalone = detailOf('8003', -800, -800, 'approved')
    const order3 = nextSnapshotOrder()
    assert.equal(statusFor(standalone, order3), '', '独立星快照更新前不误报聚合提示')
    map.refreshNow()
    await new Promise(r => setTimeout(r, 25))
    queue[3].resolve({version: 'v3', unchanged: false, points: [star('8001', 0, 0, 0), star('8003', -800, -800, 0)]})
    await tick(); await tick()
    assert.equal(statusFor(standalone, order3), '', '快照更新收录为独立星后直接可点，无聚合提示')
  }
})
