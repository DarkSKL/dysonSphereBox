import {progressLabels, type DisplayStar, type StarProgress} from '../../types/feedback'
import {cameraFrame, DISK_RADIUS, HOME_CAMERA, project, type Camera, type ProjectedStar, type ViewSize} from './galaxy-camera'
import {randomFor, spiralPhase} from './galaxy-nebula'
import {GALAXY_THEMES, type GalaxyTheme} from './galaxy-theme'

// 深色星色保留为导出常量，供星点列表等只需要一套色值的调用方兜底。
export const starColors: Record<StarProgress, string> = GALAXY_THEMES.dark.starColors
const FULL_TURN = 2 * Math.PI

// =============== 静默星盘底盘（远景星尘与隐约尘埃丝缕） ===============
// 说明：在任何反馈数量下都以极克制的低频丝缕与极细远景微尘托出三维星盘景深，数量变化时背景连续过渡不跳变。
// 边界：所有远景微尘半径极小（<0.8px）、低透明且不带光晕/星核/星芒，与可点击反馈星一目了然区分；不参与列表、命中或预取。
const skyRandom = randomFor('feedback-quiet-disk-v2')

// 沿两条主对数旋臂、细分支与外围广域空间自然延续的同源星云丝缕：
// 同一套笔刷与切线拉伸向外温和扩散，保证 0 星/少星时同样衔接全域散云、不出现圆形截断。
const skyWisps = Array.from({length: 344}, (_, i) => {
  const outer = i >= 256
  const secondary = !outer && i >= 184
  if (outer) {
    const t = Math.pow(skyRandom(), .68)
    const r = .88 + 1.48 * t
    const theta = FULL_TURN * skyRandom()
    const dr = .08
    const r2 = r + dr
    const theta2 = theta + spiralPhase(r2) - spiralPhase(r)
    const x = DISK_RADIUS * r * Math.cos(theta)
    const y = DISK_RADIUS * r * Math.sin(theta)
    const z = DISK_RADIUS * (skyRandom() - .5) * (.18 + .16 * t)
    const band: 'warm' | 'nebula' | 'cool' = skyRandom() < .32 ? 'nebula' : 'cool'
    return {
      x, y, z,
      tx: DISK_RADIUS * r2 * Math.cos(theta2) - x,
      ty: DISK_RADIUS * r2 * Math.sin(theta2) - y,
      tz: 0,
      major: .058 + .064 * skyRandom(),
      minor: .018 + .022 * skyRandom(),
      opacity: (.26 + .34 * skyRandom()) * Math.max(.26, 1 - .30 * t),
      band,
    }
  }
  const idx = secondary ? i - 184 : i
  const spanCount = secondary ? 36 : 92
  const arm = idx % 2
  const t = (Math.floor(idx / 2) + skyRandom()) / spanCount
  const r = secondary ? (.18 + .72 * Math.pow(t, .86)) : (.07 + .88 * Math.pow(t, .80))
  const armOffset = secondary ? Math.PI * .48 : 0
  const spread = secondary ? (.34 + .24 * r) : (.18 + .16 * r)
  const theta = arm * Math.PI + armOffset - .65 + spiralPhase(r) + (skyRandom() - .5) * spread
  const dr = .06
  const r2 = r + dr
  const theta2 = theta + spiralPhase(r2) - spiralPhase(r)
  const x = DISK_RADIUS * r * Math.cos(theta)
  const y = DISK_RADIUS * r * Math.sin(theta)
  const z = DISK_RADIUS * (skyRandom() - .5) * (secondary ? .055 : .04)
  const band: 'warm' | 'nebula' | 'cool' = r < .24 ? 'warm' : (r < .56 && skyRandom() < .52 ? 'nebula' : 'cool')
  return {
    x, y, z,
    tx: DISK_RADIUS * r2 * Math.cos(theta2) - x,
    ty: DISK_RADIUS * r2 * Math.sin(theta2) - y,
    tz: 0,
    major: (.034 + .032 * skyRandom()) * (1 + .25 * (1 - r)) * (secondary ? .88 : 1),
    minor: (.0055 + .0065 * skyRandom()) * (secondary ? 1.05 : 1),
    opacity: (.42 + .52 * skyRandom()) * Math.max(.24, 1 - .68 * r) * (secondary ? .58 : 1),
    band,
  }
})

// 细碎盘面星尘：紧密沿三维旋臂与盘面分布，随真实相机 zoom 1:1 附着在星云上；半径严格 <0.85px 且无光晕/星芒。
const diskSkyStars = Array.from({length: 340}, (_, i) => {
  const r = .06 + .96 * Math.sqrt(skyRandom())
  const angle = (i % 2) * Math.PI - .65 + spiralPhase(Math.min(1, r)) + (skyRandom() - .5) * .76
  const tier = skyRandom()
  return {
    x: DISK_RADIUS * r * Math.cos(angle),
    y: DISK_RADIUS * r * Math.sin(angle),
    z: DISK_RADIUS * (skyRandom() - .5) * .10,
    radius: .34 + .50 * tier * tier,
    opacity: .16 + .28 * (.32 + .68 * tier),
    warm: (r < .32 && skyRandom() < .58) || skyRandom() < .12,
  }
})

// 全域极远深空背景星：均匀铺满整个矩形视场（不再局限在星盘圆形区域内），随旋转与缩放呈现轻柔远景视差。
const farSkyStars = Array.from({length: 220}, () => {
  const tier = skyRandom()
  return {
    u: skyRandom(),
    v: skyRandom(),
    parallax: .18 + .26 * skyRandom(),
    radius: .32 + .46 * tier * tier,
    opacity: .14 + .24 * (.30 + .70 * tier),
    warm: skyRandom() < .15,
  }
})

let cachedBrushMode = ''
let cachedBrushes: {
  wispCool: HTMLCanvasElement
  wispNebula: HTMLCanvasElement
  wispWarm: HTMLCanvasElement
  seedMid: HTMLCanvasElement
  seedCore: HTMLCanvasElement
} | undefined

// 静默星盘底盘离屏缓存：星盘旋臂与盘面微尘随真实相机（含 zoom）投影，确保缩放与旋转时星体始终附着在星云旋臂上；
// 静止或仅闪烁星点时直接命中缓存（2 次 drawImage），滚轮缩放时通过视口裁切只重绘画面内旋丝与盘面尘点。
let skyCacheKey = ''
let skyWispCanvas: HTMLCanvasElement | undefined
let skyWispCtx: CanvasRenderingContext2D | null = null
let skyStarCanvas: HTMLCanvasElement | undefined
let skyStarCtx: CanvasRenderingContext2D | null = null

function makeSoftBrush(color: string, radius = 32): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = radius * 2
  const ctx = canvas.getContext('2d')
  if (ctx) {
    const g = ctx.createRadialGradient(radius, radius, 0, radius, radius, radius)
    g.addColorStop(0, color)
    g.addColorStop(.28, color + '99')
    g.addColorStop(.58, color + '33')
    g.addColorStop(.82, color + '0a')
    g.addColorStop(1, color + '00')
    ctx.fillStyle = g
    ctx.fillRect(0, 0, canvas.width, canvas.height)
  }
  return canvas
}

function makeRadialSprite(size: number, stops: readonly (readonly [number, string])[]): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = size
  const ctx = canvas.getContext('2d')
  if (ctx) {
    const half = size / 2
    const g = ctx.createRadialGradient(half, half, 0, half, half, half)
    for (const [offset, color] of stops) g.addColorStop(offset, color)
    ctx.fillStyle = g
    ctx.fillRect(0, 0, size, size)
  }
  return canvas
}

function themeBrushes(theme: GalaxyTheme) {
  if (cachedBrushMode !== theme.mode || !cachedBrushes) {
    cachedBrushMode = theme.mode
    cachedBrushes = {
      wispCool: makeSoftBrush(theme.sky.wispCool, 32),
      wispNebula: makeSoftBrush(theme.sky.wispNebula, 32),
      wispWarm: makeSoftBrush(theme.sky.wispWarm, 32),
      seedMid: makeRadialSprite(96, [
        [0, theme.mode === 'dark' ? '#fff3da' : theme.sky.glow],
        [.22, theme.sky.glow + '7a'],
        [.56, theme.sky.glow + '14'],
        [1, theme.sky.glow + '00'],
      ]),
      seedCore: makeRadialSprite(64, [
        [0, theme.mode === 'dark' ? '#fffaf0' : theme.sky.glow + 'bb'],
        [.26, theme.mode === 'dark' ? '#ffe6ba' : theme.sky.glow + '66'],
        [.62, theme.sky.glow + '18'], [1, theme.sky.glow + '00'],
      ]),
    }
  }
  return cachedBrushes
}

function rebuildSkyCache(camera: Camera, size: ViewSize, theme: GalaxyTheme, dpr: number): void {
  const {width, height} = size
  const zoom = Math.max(1, camera.zoom)
  const frame = cameraFrame(camera, size)
  const minDim = Math.max(1, Math.min(width, height))
  const brushes = themeBrushes(theme)
  // 与 nebula-renderer.ts 保持同源缩放阻尼：位置严格按真实 camera.zoom 与星点 1:1 锁定，
  // 单个柔丝尺寸按 zoom^0.77 平滑成长并微调曝光，近景仍清透细腻不发糊。
  const wispZoom = Math.pow(zoom, .77)
  const wispZoomFade = 1 / (1 + .24 * Math.log2(zoom))

  // 1) 旋臂柔丝底盘（严格随真实 camera.zoom 投影，缩放时与星点完全同步，绝不脱节）。
  const wispRes = Math.min(dpr, .85, 640 / Math.max(width, height))
  const ww = Math.max(1, Math.round(width * wispRes))
  const wh = Math.max(1, Math.round(height * wispRes))
  if (!skyWispCanvas) {
    skyWispCanvas = document.createElement('canvas')
    skyWispCtx = skyWispCanvas.getContext('2d')
  }
  if (skyWispCanvas.width !== ww || skyWispCanvas.height !== wh) {
    skyWispCanvas.width = ww
    skyWispCanvas.height = wh
  }
  if (skyWispCtx) {
    const ctx = skyWispCtx
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, ww, wh)
    ctx.globalCompositeOperation = theme.mode === 'dark' ? 'screen' : 'source-over'
    const cosIncl = .52 + .48 * Math.cos(camera.inclination)
    for (const w of skyWisps) {
      const {x, y} = project(w, camera, size, frame)
      const rx = w.major * minDim * wispZoom
      const ry = w.minor * minDim * cosIncl * wispZoom
      if (x < -rx || x > width + rx || y < -rx || y > height + rx) continue
      const sx = w.tx * frame.right[0] + w.ty * frame.right[1]
      const sy = w.tx * frame.down[0] + w.ty * frame.down[1] + w.tz * frame.down[2]
      const len = Math.hypot(sx, sy)
      const ax = len > .01 ? sx / len : 1
      const ay = len > .01 ? sy / len : 0
      ctx.globalAlpha = theme.sky.wispAlpha * w.opacity * wispZoomFade
      ctx.setTransform(wispRes * ax, wispRes * ay, -wispRes * ay, wispRes * ax, x * wispRes, y * wispRes)
      const brush = w.band === 'warm' ? brushes.wispWarm : w.band === 'nebula' ? brushes.wispNebula : brushes.wispCool
      ctx.drawImage(brush, -rx, -ry, 2 * rx, 2 * ry)
    }
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.globalAlpha = 1
    ctx.globalCompositeOperation = 'source-over'
  }

  // 2) 盘面星尘（随真实 camera.zoom 与星点 1:1 附着）+ 全域极远深空背景星（覆盖整个矩形画布无圆形边界）。
  const starRes = Math.min(dpr, 2)
  const sw = Math.max(1, Math.round(width * starRes))
  const sh = Math.max(1, Math.round(height * starRes))
  if (!skyStarCanvas) {
    skyStarCanvas = document.createElement('canvas')
    skyStarCtx = skyStarCanvas.getContext('2d')
  }
  if (skyStarCanvas.width !== sw || skyStarCanvas.height !== sh) {
    skyStarCanvas.width = sw
    skyStarCanvas.height = sh
  }
  if (skyStarCtx) {
    const ctx = skyStarCtx
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, sw, sh)
    ctx.setTransform(starRes, 0, 0, starRes, 0, 0)
    ctx.globalCompositeOperation = 'source-over'
    const diskDotScale = Math.min(1.35, 1 + .10 * Math.log2(zoom))
    for (const dust of diskSkyStars) {
      const {x, y} = project(dust, camera, size, frame)
      if (x < -2 || x > width + 2 || y < -2 || y > height + 2) continue
      ctx.globalAlpha = dust.opacity
      ctx.fillStyle = dust.warm ? theme.sky.warmDust : theme.sky.dust
      ctx.beginPath()
      ctx.arc(x, y, dust.radius * diskDotScale, 0, FULL_TURN)
      ctx.fill()
    }
    const farSpan = Math.hypot(width, height) * 1.12
    const farScale = Math.pow(zoom, .18)
    const cosFar = Math.cos(camera.yaw * .24)
    const sinFar = Math.sin(camera.yaw * .24)
    const cx = width / 2
    const cy = height / 2
    for (const far of farSkyStars) {
      const pu = (((far.u + camera.yaw * .055 * far.parallax) % 1) + 1) % 1 - .5
      const pv = (((far.v + Math.sin(camera.inclination) * .16 * far.parallax) % 1) + 1) % 1 - .5
      const lx = pu * farSpan * farScale
      const ly = pv * farSpan * farScale
      const x = cx + lx * cosFar - ly * sinFar
      const y = cy + lx * sinFar + ly * cosFar
      if (x < -2 || x > width + 2 || y < -2 || y > height + 2) continue
      ctx.globalAlpha = far.opacity
      ctx.fillStyle = far.warm ? theme.sky.warmDust : theme.sky.dust
      ctx.beginPath()
      ctx.arc(x, y, far.radius, 0, FULL_TURN)
      ctx.fill()
    }
    ctx.globalAlpha = 1
  }
}

/** 沿当前相机（含缩放倍率）投影静默星盘底盘与全域深空微尘，使旋臂柔丝、盘面微尘与反馈星在任意缩放与旋转下严密附着。 */
export function drawSkyDust(ctx: CanvasRenderingContext2D, camera: Camera, size: ViewSize, theme: GalaxyTheme, cloud = 0, dpr = 1): void {
  const level = Math.max(0, Math.min(1, cloud))
  const wispFade = 1 - .78 * level
  const starFade = Math.min(1, (1 - .32 * level) * theme.sky.dustScale)
  const key = `${theme.mode}|${size.width}|${size.height}|${Math.round(dpr * 100)}|${Math.round(camera.yaw * 1000)}|${Math.round(camera.inclination * 1000)}|${Math.round(camera.zoom * 1000)}`
  if (key !== skyCacheKey || !skyWispCanvas || !skyStarCanvas) {
    rebuildSkyCache(camera, size, theme, dpr)
    skyCacheKey = key
  }
  ctx.save()
  if (wispFade > .02 && skyWispCanvas) {
    ctx.globalCompositeOperation = theme.mode === 'dark' ? 'screen' : 'source-over'
    ctx.globalAlpha = wispFade
    ctx.drawImage(skyWispCanvas, 0, 0, size.width, size.height)
  }
  if (starFade > .01 && skyStarCanvas) {
    ctx.globalCompositeOperation = 'source-over'
    ctx.globalAlpha = starFade
    ctx.drawImage(skyStarCanvas, 0, 0, size.width, size.height)
  }
  ctx.restore()
}

/** 银心收敛微光：仅保留紧致的小尺度银心核球并随真实 zoom 同步缩放，去掉向外扩散的大光圈，避免背景发散浑浊。 */
export function drawSeedGlow(ctx: CanvasRenderingContext2D, size: ViewSize, theme: GalaxyTheme, amount: number, camera: Camera = HOME_CAMERA): void {
  if (amount <= .004) return
  const x = size.width / 2, y = size.height / 2
  const minDim = Math.max(1, Math.min(size.width, size.height))
  const zoom = Math.max(1, camera.zoom)
  const coreScale = Math.pow(zoom, .76)
  const zoomFade = 1 / (1 + .30 * Math.log2(zoom))
  const mid = Math.max(24, minDim * .076) * coreScale
  const inner = Math.max(8, minDim * .022) * coreScale
  const tilt = .48 + .52 * Math.cos(camera.inclination)
  const brushes = themeBrushes(theme)
  ctx.save()
  ctx.globalCompositeOperation = theme.mode === 'dark' ? 'screen' : 'source-over'

  // 紧致倾角核球：随俯仰压扁并随 zoom 同步缩放，仅在银心内部聚拢微光，绝不外扩成大光环。
  ctx.save()
  ctx.translate(x, y)
  ctx.rotate(camera.yaw * .14)
  ctx.scale(1.12, tilt)
  ctx.globalAlpha = theme.sky.glowAlpha * .85 * amount * zoomFade
  ctx.drawImage(brushes.seedMid, -mid, -mid, 2 * mid, 2 * mid)
  ctx.restore()

  // 内核微点
  ctx.globalAlpha = Math.min(.62, theme.sky.glowAlpha * (theme.mode === 'dark' ? 2.2 : 1.25) * amount * zoomFade)
  ctx.drawImage(brushes.seedCore, x - inner, y - inner, 2 * inner, 2 * inner)
  ctx.restore()
}

export function starLabel(point: DisplayStar): string {
  return point.privateLabel || progressLabels[point.progress] || '玩家反馈'
}

// 局部视图里显出星号，始终留出选星空间；拥挤处优先使用已有列表。
export function detailLevel(zoom: number): number {
  const t = Math.max(0, Math.min(1, (Math.log2(zoom) - 1.35) / 1.85))
  return t * t * (3 - 2 * t)
}

/** 光晕随同屏星数缩小：约 30 颗以内保持原大小，密集时逐步收窄到一半，避免光晕互相淹没而无法分辨星核。 */
export function haloScale(visibleCount: number): number {
  if (visibleCount <= 30) return 1
  return Math.max(.5, 1 - .16 * Math.log2(visibleCount / 30))
}

export function starCaptions(stars: readonly ProjectedStar[], size: ViewSize, level: number) {
  const captions: {star: ProjectedStar; x: number; y: number; width: number}[] = []
  if (level < .25 || stars.length > 48) return captions
  for (const star of stars) {
    const width = Math.max(35, 16 + 8 * star.point.starId.length)
    const y = star.y - 9
    if (y < 5 || y + 18 > size.height - 5) continue
    for (const x of [star.x + 12, star.x - 12 - width]) {
      if (x < 5 || x + width > size.width - 5) continue
      if (stars.some(other => other !== star && other.x > x - 7 && other.x < x + width + 7 &&
        other.y > y - 7 && other.y < y + 25)) continue
      if (captions.some(c => Math.abs(c.y - y) < 21 && x < c.x + c.width + 3 && x + width > c.x - 3)) continue
      captions.push({star, x, y, width})
      break
    }
    if (captions.length === 12) break
  }
  return captions
}

type StarGlows = Record<StarProgress, HTMLCanvasElement> & {
  private: HTMLCanvasElement
  cushion: HTMLCanvasElement | null
}

function starPhaseSeed(id: string): number {
  let hash = 2166136261
  for (let i = 0; i < id.length; i++) hash = Math.imul(hash ^ id.charCodeAt(i), 16777619)
  return ((hash >>> 0) % 10000) / 10000
}

function privateStarTwinkle(id: string, time: number): number {
  if (time <= 0) return .78
  const seed = starPhaseSeed(id)
  const phase = time * 2.55 + seed * FULL_TURN
  return Math.max(0, Math.min(1, .50 + .34 * Math.sin(phase) + .16 * Math.sin(phase * 2.31 + 1.15)))
}

/** 预烘方案 A 的针尖高锐星核 Sprite：收细纯白亮芯、让光谱色环贴身显现并干净收束（无外扩糊光）。主题切换时重建。 */
export function createStarGlows(theme: GalaxyTheme): StarGlows {
  const light = theme.mode === 'light'
  function makeCore(color: string): HTMLCanvasElement {
    const size = 48
    const half = size / 2
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = size
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const grad = ctx.createRadialGradient(half, half, 0, half, half, half)
      if (light) {
        // 日间水墨：细锐矿墨点苔，核实边清、无浑浊外晕。
        grad.addColorStop(0, color)
        grad.addColorStop(.56, color)
        grad.addColorStop(.84, color + '88')
        grad.addColorStop(1, color + '00')
      } else {
        // 深色针尖星核：纯白针尖收窄至 0.22，外缘紧贴高饱和光谱色环并快速衰减，彻底去除白豆感与背景糊光。
        grad.addColorStop(0, '#ffffff')
        grad.addColorStop(.22, theme.coreHighlight)
        grad.addColorStop(.48, color)
        grad.addColorStop(.76, color + '52')
        grad.addColorStop(1, color + '00')
      }
      ctx.fillStyle = grad
      ctx.beginPath()
      ctx.arc(half, half, half, 0, FULL_TURN)
      ctx.fill()
    }
    return canvas
  }
  const colors = theme.starColors
  const privateTone = light ? '#46525f' : '#b4d6ff'
  return {
    feedback: makeCore(colors.feedback),
    accepted: makeCore(colors.accepted),
    implemented: makeCore(colors.implemented),
    private: makeCore(privateTone),
    cushion: theme.coreCushion ? makeSoftBrush(theme.coreCushion, 24) : null,
  }
}

/** 极细 1px 级光学微芒（无渐变分配开销，尖端几何收至 0px）：仅用于高热主星、已实现星与闪烁中的待审星。 */
function drawPinpointGlint(
  ctx: CanvasRenderingContext2D, x: number, y: number, len: number, color: string, alpha: number, light: boolean,
): void {
  if (len <= 1.2 || alpha <= .02) return
  const waist = light ? .48 : .54
  ctx.globalAlpha = alpha
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.moveTo(x - len, y)
  ctx.lineTo(x, y - waist)
  ctx.lineTo(x + len, y)
  ctx.lineTo(x, y + waist)
  ctx.closePath()
  ctx.moveTo(x, y - len)
  ctx.lineTo(x + waist, y)
  ctx.lineTo(x, y + len)
  ctx.lineTo(x - waist, y)
  ctx.closePath()
  ctx.fill()

  if (!light) {
    const inner = len * .48
    const innerWaist = .34
    ctx.globalAlpha = Math.min(1, alpha * 1.15)
    ctx.fillStyle = '#ffffff'
    ctx.beginPath()
    ctx.moveTo(x - inner, y)
    ctx.lineTo(x, y - innerWaist)
    ctx.lineTo(x + inner, y)
    ctx.lineTo(x, y + innerWaist)
    ctx.closePath()
    ctx.moveTo(x, y - inner)
    ctx.lineTo(x + innerWaist, y)
    ctx.lineTo(x, y + inner)
    ctx.lineTo(x - innerWaist, y)
    ctx.closePath()
    ctx.fill()
  }
}

// =============== 单颗反馈星（方案 A：针尖高锐星核 · 按热度分星等） ===============
// 以收窄白芯与真实星等差融入星云游离尘星：体积（半径）、亮度（透明度 + 高热炽白芯）与 1px 微芒均随点赞热度（brightness）连续拉开；
// 低赞星纤细柔和、高赞主星清亮醒目并浮现锐利十字微芒；待审私有星表现为轻柔呼吸闪烁的微星。
export function drawFeedbackStar(
  ctx: CanvasRenderingContext2D, glows: StarGlows, star: ProjectedStar, pulse: number, detail: number,
  theme: GalaxyTheme, scale = 1, sparse = 0, time = 0,
): void {
  const {point, x, y} = star
  const light = theme.mode === 'light'

  if (point.privateLabel) {
    const twinkle = privateStarTwinkle(point.starId, time)
    const bright = .34 + .66 * twinkle
    const baseRadius = (light ? 1.55 : 1.65) + detail * .65 + sparse * .32 + pulse * 2.0
    const coreRadius = baseRadius * (.82 + .34 * twinkle) * (.82 + .18 * scale)

    if (glows.cushion) {
      const cushion = coreRadius * 1.95
      ctx.globalAlpha = .40 * (.7 + .3 * twinkle)
      ctx.drawImage(glows.cushion, x - cushion, y - cushion, 2 * cushion, 2 * cushion)
    }

    ctx.globalAlpha = Math.min(1, bright)
    ctx.drawImage(glows.private, x - coreRadius, y - coreRadius, 2 * coreRadius, 2 * coreRadius)
    if (twinkle > .38) {
      const glintLen = (2.3 + 2.0 * twinkle) * (.88 + .24 * detail) + pulse * 2.4
      const privateTone = light ? '#46525f' : '#b4d6ff'
      drawPinpointGlint(ctx, x, y, glintLen, privateTone, (light ? .34 : .52) * twinkle, light)
    }
    ctx.globalAlpha = 1
    return
  }

  const heat = Math.max(0, Math.min(1, point.brightness))
  // 体积随热度按非线性的天文星等曲线平滑拉开（0 赞纤细如亮尘，高赞跃升为醒目主星）。
  const sizeCurve = Math.pow(heat, 1.18)
  const lumCurve = Math.pow(heat, .72)
  const isImplemented = point.progress === 'implemented'
  const tierBoost = isImplemented ? .18 : 0
  const density = Math.min(5, Math.log2(Math.max(1, point.count ?? 1)))
  const baseRadius = (light ? 1.24 + sizeCurve * 2.45 : 1.28 + sizeCurve * 2.85) + tierBoost + Math.min(.65, density * .18) + detail * .72 + sparse * .32
  const coreRadius = Math.max(1.12, baseRadius * (.78 + .22 * scale)) + pulse * 2.4
  const core = glows[point.progress] ?? glows.feedback
  // 亮度随点赞热度连续增强：低赞柔和融入星尘，高赞饱满透亮。
  const starAlpha = Math.min(1, (light ? .56 + .44 * lumCurve : .52 + .48 * lumCurve) + pulse * .35)

  // 浅色水墨仅保留极紧致的宣纸底衬，随热度微调衬托力度，不形成发散雾团。
  if (glows.cushion) {
    const cushion = coreRadius * 1.92
    ctx.globalAlpha = (.32 + .34 * lumCurve) * starAlpha
    ctx.drawImage(glows.cushion, x - cushion, y - cushion, 2 * cushion, 2 * cushion)
  }

  ctx.globalAlpha = starAlpha
  ctx.drawImage(core, x - coreRadius, y - coreRadius, 2 * coreRadius, 2 * coreRadius)

  // 深色高赞星追加一层极紧致的炽白内芯叠加（无外扩雾晕），让高赞主星在亮度上与低赞星一眼分明。
  if (!light && (heat > .34 || pulse > .08)) {
    const hotFactor = Math.max(0, (heat - .34) / .66)
    const hotRadius = coreRadius * (.52 + .18 * hotFactor)
    ctx.globalAlpha = Math.min(.92, hotFactor * .82 + pulse * .45)
    ctx.drawImage(core, x - hotRadius, y - hotRadius, 2 * hotRadius, 2 * hotRadius)
  }

  // 中高赞星自然浮现极短 1px 光学十字微芒，长度与明度均随点赞热度递进，既清晰体现热度层次又绝不发糊。
  if (heat > .46 || pulse > .08) {
    const color = theme.starColors[point.progress] ?? theme.starColors.feedback
    const glintFactor = Math.max(0, (heat - .46) / .54)
    const glintLen = (2.6 + glintFactor * 5.2) * (.84 + .16 * scale) * (1 + .26 * detail) + pulse * 3.2
    const glintAlpha = Math.min(.84, (light ? .28 + .42 * glintFactor : .32 + .50 * glintFactor) + pulse * .35)
    drawPinpointGlint(ctx, x, y, glintLen, color, glintAlpha, light)
  }
  ctx.globalAlpha = 1
}

/** 选中或悬停准星最后绘制：纤细精密的四段弧与细微刻度，与星图线条语言统一。 */
export function drawStarFocus(ctx: CanvasRenderingContext2D, {point, x, y}: ProjectedStar, theme: GalaxyTheme, selected = true): void {
  const r = selected ? 9.5 : 8.5
  const span = selected ? Math.PI / 3.8 : Math.PI / 4.4
  const accent = point.privateLabel ? theme.privateColor : (theme.starColors[point.progress] ?? theme.starColors.feedback)
  ctx.save()
  ctx.lineCap = 'round'
  ctx.beginPath()
  for (let i = 0; i < 4; i++) {
    const start = (i + .5) * Math.PI / 2 - span / 2
    ctx.moveTo(x + r * Math.cos(start), y + r * Math.sin(start))
    ctx.arc(x, y, r, start, start + span)
  }
  if (selected) {
    for (let i = 0; i < 4; i++) {
      const a = i * Math.PI / 2
      ctx.moveTo(x + (r + 1.5) * Math.cos(a), y + (r + 1.5) * Math.sin(a))
      ctx.lineTo(x + (r + 4.0) * Math.cos(a), y + (r + 4.0) * Math.sin(a))
    }
  }
  ctx.strokeStyle = theme.focusOutline
  ctx.lineWidth = selected ? 2.2 : 1.8
  ctx.stroke()
  ctx.globalAlpha = selected ? .92 : .72
  ctx.strokeStyle = accent
  ctx.lineWidth = selected ? 1.1 : .95
  ctx.stroke()
  ctx.restore()
}
