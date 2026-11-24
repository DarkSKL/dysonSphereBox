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
const {HOME_CAMERA, project, usefulZoomLimit, queryBounds, includeStar} =
  await vite.ssrLoadModule('/galaxy-camera.ts')
const {SAMPLE_STRIDE, sampleNebula} = await vite.ssrLoadModule('/galaxy-nebula.ts')
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
  assert.equal(sampleNebula([a]).length, 0)
  assert.equal(sampleNebula([a, {...b, privateLabel: '待审核'}]).length, 0)
})
