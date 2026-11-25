import assert from 'node:assert/strict'
import {after, test} from 'node:test'
import {fileURLToPath} from 'node:url'
import {createServer} from 'vite'

// 通过项目已有的 Vite 转译被测模块，无需额外的测试运行器。
const vite = await createServer({
  root: fileURLToPath(new URL('../../src/composables/feedback/', import.meta.url)),
  configFile: false, logLevel: 'error', optimizeDeps: {disabled: true},
  server: {middlewareMode: true, hmr: false},
})
after(() => vite.close())
const {HOME_CAMERA, project, usefulZoomLimit, queryBounds, includeStar, starsNear, AMBIGUOUS_GAP} =
  await vite.ssrLoadModule('/galaxy-camera.ts')
const {SAMPLE_STRIDE, sampleNebula, cloudPresence, cloudExposure} = await vite.ssrLoadModule('/galaxy-nebula.ts')
const {galaxyStatus} = await vite.ssrLoadModule('/galaxy-status.ts')
const {haloScale} = await vite.ssrLoadModule('/galaxy-art.ts')
const star = (id, x, y, z) => ({starId: String(id), x, y, z, progress: 'feedback', brightness: .4})

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

test('状态提示：五种状态互斥，首次加载完成前不误报无反馈', () => {
  const base = {foreground: true, ready: true, error: '', total: 5, visible: 5, zoom: 1, selectedId: '', selectedVisible: false}
  assert.equal(galaxyStatus(base), '')
  assert.equal(galaxyStatus({...base, ready: false, total: 0, visible: 0}), 'loading')
  assert.equal(galaxyStatus({...base, error: '失败', ready: true}), 'error')
  assert.equal(galaxyStatus({...base, error: '失败', ready: false}), 'error', '失败优先于加载中')
  assert.equal(galaxyStatus({...base, total: 0, visible: 0}), 'empty')
  assert.equal(galaxyStatus({...base, total: 0, visible: 0, zoom: 4}), 'offscreen', '放大后的空画面不是「无反馈」')
  assert.equal(galaxyStatus({...base, visible: 0}), 'offscreen')
  assert.equal(galaxyStatus({...base, visible: 3, selectedId: '9', selectedVisible: false}), 'selected-offscreen')
  assert.equal(galaxyStatus({...base, selectedId: '9', selectedVisible: true}), '')
  assert.equal(galaxyStatus({...base, foreground: false, ready: false}), '')
})
