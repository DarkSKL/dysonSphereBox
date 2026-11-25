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

// 沿两条对数旋臂与微弱银盘分布的极淡尘埃丝缕：用切向拉伸的羽化柔光椭圆平滑拼接，消除孤立圆斑噪点。
const skyWisps = Array.from({length: 132}, (_, i) => {
  const arm = i % 2
  const t = (Math.floor(i / 2) + skyRandom()) / 66
  const r = .07 + .88 * Math.pow(t, .82)
  const theta = arm * Math.PI - .65 + spiralPhase(r) + (skyRandom() - .5) * (.32 + .24 * r)
  const dr = .06
  const r2 = r + dr
  const theta2 = theta + spiralPhase(r2) - spiralPhase(r)
  const x = DISK_RADIUS * r * Math.cos(theta)
  const y = DISK_RADIUS * r * Math.sin(theta)
  const z = DISK_RADIUS * (skyRandom() - .5) * .055
  return {
    x, y, z,
    tx: DISK_RADIUS * r2 * Math.cos(theta2) - x,
    ty: DISK_RADIUS * r2 * Math.sin(theta2) - y,
    tz: 0,
    major: (.042 + .038 * skyRandom()) * (1 + .35 * (1 - r)),
    minor: .012 + .013 * skyRandom(),
    opacity: (.38 + .52 * skyRandom()) * Math.max(.22, 1 - .72 * r),
    warm: r < .30,
  }
})

// 细碎微弱的远景背景星尘：分布在全天球与微弱盘面，带三维视差，但亮度与尺寸严格压在反馈星之下。
const skyStars = Array.from({length: 250}, (_, i) => {
  const disk = i < 140
  const r = disk ? .08 + .96 * Math.sqrt(skyRandom()) : .24 + 1.05 * Math.sqrt(skyRandom())
  const angle = disk
    ? (i % 2) * Math.PI - .65 + spiralPhase(Math.min(1, r)) + (skyRandom() - .5) * 1.15
    : FULL_TURN * skyRandom()
  const tier = skyRandom()
  return {
    x: DISK_RADIUS * r * Math.cos(angle),
    y: DISK_RADIUS * r * Math.sin(angle),
    z: DISK_RADIUS * (skyRandom() - .5) * (disk ? .14 : .72),
    // 严格保持极细微尘（0.40–0.80px），杜绝交互星核感。
    radius: .40 + .40 * tier * tier,
    opacity: .11 + .18 * (disk ? .82 : 1) * (.38 + .62 * tier),
    warm: disk && r < .34 && skyRandom() < .55,
  }
})

let cachedBrushMode = ''
let cachedBrushes: {
  wispCool: HTMLCanvasElement
  wispWarm: HTMLCanvasElement
  seedOuter: HTMLCanvasElement
  seedMid: HTMLCanvasElement
  seedCore: HTMLCanvasElement
} | undefined

// 静默星盘底盘离屏缓存：底盘只随朝向（yaw/inclination）、视口尺寸与主题变化，不随 zoom 变化。
// 连续滚轮缩放与云雾流动期间直接命中缓存，仅需 2 次 drawImage，避免每帧重复画 132 缕柔丝与 250 颗微尘。
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
      wispWarm: makeSoftBrush(theme.sky.wispWarm, 32),
      seedOuter: makeRadialSprite(96, [
        [0, theme.sky.glowMid + '88'], [.32, theme.sky.glowMid + '38'],
        [.68, theme.sky.glowMid + '0c'], [1, theme.sky.glowMid + '00'],
      ]),
      seedMid: makeRadialSprite(96, [
        [0, theme.sky.glow], [.26, theme.sky.glow + '80'],
        [.60, theme.sky.glow + '22'], [1, theme.sky.glow + '00'],
      ]),
      seedCore: makeRadialSprite(64, [
        [0, theme.mode === 'dark' ? '#fff4de' : theme.sky.glow + 'cc'],
        [.35, theme.sky.glow + '66'], [1, theme.sky.glow + '00'],
      ]),
    }
  }
  return cachedBrushes
}

function rebuildSkyCache(camera: Camera, size: ViewSize, theme: GalaxyTheme, dpr: number): void {
  const {width, height} = size
  const skyCamera = {...camera, zoom: 1}
  const frame = cameraFrame(skyCamera, size)
  const minDim = Math.max(1, Math.min(width, height))
  const brushes = themeBrushes(theme)

  // 1) 旋臂柔丝底盘（低频柔和层使用适度降采样离屏画布，平滑缩放极细腻且重建飞快）。
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
    const cosIncl = .55 + .45 * Math.cos(camera.inclination)
    for (const w of skyWisps) {
      const {x, y} = project(w, skyCamera, size, frame)
      const sx = w.tx * frame.right[0] + w.ty * frame.right[1]
      const sy = w.tx * frame.down[0] + w.ty * frame.down[1] + w.tz * frame.down[2]
      const len = Math.hypot(sx, sy)
      const ax = len > .01 ? sx / len : 1
      const ay = len > .01 ? sy / len : 0
      const rx = w.major * minDim
      const ry = w.minor * minDim * cosIncl
      ctx.globalAlpha = theme.sky.wispAlpha * w.opacity
      ctx.setTransform(wispRes * ax, wispRes * ay, -wispRes * ay, wispRes * ax, x * wispRes, y * wispRes)
      ctx.drawImage(w.warm ? brushes.wispWarm : brushes.wispCool, -rx, -ry, 2 * rx, 2 * ry)
    }
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.globalAlpha = 1
    ctx.globalCompositeOperation = 'source-over'
  }

  // 2) 远景极细微尘底盘（保持屏幕清晰度，与可点击反馈星清晰区分）。
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
    for (const dust of skyStars) {
      const {x, y} = project(dust, skyCamera, size, frame)
      if (x < -2 || x > width + 2 || y < -2 || y > height + 2) continue
      ctx.globalAlpha = dust.opacity
      ctx.fillStyle = dust.warm ? theme.sky.warmDust : theme.sky.dust
      ctx.beginPath()
      ctx.arc(x, y, dust.radius, 0, FULL_TURN)
      ctx.fill()
    }
    ctx.globalAlpha = 1
  }
}

/** 沿当前方向投影静默星盘底盘（隐约旋臂尘丝 + 远景极细微尘）。使用全景尺度，cloud 越高底盘尘丝越退后。 */
export function drawSkyDust(ctx: CanvasRenderingContext2D, camera: Camera, size: ViewSize, theme: GalaxyTheme, cloud = 0, dpr = 1): void {
  const level = Math.max(0, Math.min(1, cloud))
  const wispFade = 1 - .78 * level
  const starFade = Math.min(1, (1 - .32 * level) * theme.sky.dustScale)
  const key = `${theme.mode}|${size.width}|${size.height}|${Math.round(dpr * 100)}|${Math.round(camera.yaw * 1000)}|${Math.round(camera.inclination * 1000)}`
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

/** 银心柔和多层微光：0 条时作为静默星盘的核心呼吸点，随星云亮起柔和退隐；全部使用预烘离屏笔刷，零每帧渐变创建。 */
export function drawSeedGlow(ctx: CanvasRenderingContext2D, size: ViewSize, theme: GalaxyTheme, amount: number, camera: Camera = HOME_CAMERA): void {
  if (amount <= .004) return
  const x = size.width / 2, y = size.height / 2
  const minDim = Math.max(1, Math.min(size.width, size.height))
  const outer = Math.max(64, minDim * .25)
  const mid = outer * .56
  const inner = Math.max(11, minDim * .030)
  const tilt = .52 + .48 * Math.cos(camera.inclination)
  const brushes = themeBrushes(theme)
  ctx.save()
  ctx.globalCompositeOperation = theme.mode === 'dark' ? 'screen' : 'source-over'

  // 外层宽雾：清透微晕，衔接外围星盘。
  ctx.globalAlpha = theme.sky.glowAlpha * .55 * amount
  ctx.drawImage(brushes.seedOuter, x - outer, y - outer, 2 * outer, 2 * outer)

  // 中层倾角银盘光：随相机俯仰压扁，产生立体星盘感。
  ctx.save()
  ctx.translate(x, y)
  ctx.rotate(camera.yaw * .14)
  ctx.scale(1, tilt)
  ctx.globalAlpha = theme.sky.glowAlpha * .95 * amount
  ctx.drawImage(brushes.seedMid, -mid, -mid, 2 * mid, 2 * mid)
  ctx.restore()

  // 内层柔光芯：柔和过渡的微光核心（浅色为极淡清墨微晕，深色为暖金柔芯）。
  ctx.globalAlpha = Math.min(.75, theme.sky.glowAlpha * (theme.mode === 'dark' ? 2.1 : 1.35) * amount)
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

type StarSpriteSet = {
  glow: HTMLCanvasElement
  core: HTMLCanvasElement
  spike4: HTMLCanvasElement
  spike8: HTMLCanvasElement
}
type StarGlows = Record<StarProgress, StarSpriteSet> & {cushion: HTMLCanvasElement | null}

/** 预烘单颗星的柔晕、羽化星核与纤细星芒 Sprite，彻底消除绘制 200 颗星时的每帧渐变创建。主题切换时重建。 */
export function createStarGlows(theme: GalaxyTheme): StarGlows {
  const light = theme.mode === 'light'
  function makeGlow(color: string): HTMLCanvasElement {
    return makeRadialSprite(64, [
      [0, color], [.14, color + 'a8'], [.34, color + '44'], [.64, color + '10'], [1, color + '00'],
    ])
  }
  function makeCore(color: string): HTMLCanvasElement {
    const size = 32
    const half = size / 2
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = size
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const grad = ctx.createRadialGradient(half, half, 0, half, half, half)
      if (light) {
        // 日间水墨：沉稳羽化的焦墨/青黛/淡朱砂星核，搭配极轻柔的针尖亮感，不形成白洞珠子。
        grad.addColorStop(0, color)
        grad.addColorStop(.72, color)
        grad.addColorStop(.92, color + '88')
        grad.addColorStop(1, color + '00')
      } else {
        grad.addColorStop(0, theme.coreHighlight)
        grad.addColorStop(.46, theme.coreHighlight)
        grad.addColorStop(.74, color)
        grad.addColorStop(1, color + '00')
      }
      ctx.fillStyle = grad
      ctx.beginPath()
      ctx.arc(half, half, half, 0, FULL_TURN)
      ctx.fill()
      if (light) {
        ctx.globalAlpha = .20
        ctx.fillStyle = theme.coreHighlight
        ctx.beginPath()
        ctx.arc(half - half * .14, half - half * .14, half * .22, 0, FULL_TURN)
        ctx.fill()
      }
    }
    return canvas
  }
  function makeSpike(color: string, diagonal: boolean): HTMLCanvasElement {
    const size = 96
    const c = size / 2
    const len = c - 2
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = size
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const drawAxis = (dx: number, dy: number, axisLen: number, lineWidth: number, alpha: number) => {
        ctx.save()
        ctx.globalAlpha = alpha
        const grad = ctx.createLinearGradient(c - dx * axisLen, c - dy * axisLen, c + dx * axisLen, c + dy * axisLen)
        grad.addColorStop(0, color + '00')
        grad.addColorStop(.34, color + '55')
        grad.addColorStop(.5, color)
        grad.addColorStop(.66, color + '55')
        grad.addColorStop(1, color + '00')
        ctx.strokeStyle = grad
        ctx.lineWidth = lineWidth
        ctx.lineCap = 'round'
        ctx.beginPath()
        ctx.moveTo(c - dx * axisLen, c - dy * axisLen)
        ctx.lineTo(c + dx * axisLen, c + dy * axisLen)
        ctx.stroke()
        ctx.restore()
      }
      drawAxis(1, 0, len, 2.2, 1)
      drawAxis(0, 1, len, 2.2, 1)
      if (diagonal) {
        const d = Math.SQRT1_2
        drawAxis(d, d, len * .54, 1.7, .42)
        drawAxis(d, -d, len * .54, 1.7, .42)
      }
    }
    return canvas
  }
  const darkSpike4 = !light ? makeSpike(theme.coreHighlight, false) : undefined
  const darkSpike8 = !light ? makeSpike(theme.coreHighlight, true) : undefined
  const setFor = (color: string): StarSpriteSet => ({
    glow: makeGlow(color),
    core: makeCore(color),
    spike4: darkSpike4 ?? makeSpike(color, false),
    spike8: darkSpike8 ?? makeSpike(color, true),
  })
  const colors = theme.starColors
  return {
    feedback: setFor(colors.feedback),
    accepted: setFor(colors.accepted),
    implemented: setFor(colors.implemented),
    cushion: theme.coreCushion ? makeSoftBrush(theme.coreCushion, 32) : null,
  }
}

// =============== 单颗反馈星 ===============
// 星核大小、柔光晕与纤细星芒按星等连续分级；全部复用预烘 Sprite，零每帧渐变创建。
// 浅色用柔和宣纸羽化托底配焦墨星核，深色呈通透星光；作者待审稿使用纤细空心星环区分。
// sparse 为 0..1 的稀疏聚焦度（1 条时接近 1，随星数连续衰减到 0），让首颗星天然带出轻柔星芒并成为画面焦点。
export function drawFeedbackStar(
  ctx: CanvasRenderingContext2D, glows: StarGlows, star: ProjectedStar, pulse: number, detail: number,
  theme: GalaxyTheme, scale = 1, sparse = 0,
): void {
  const {point, x, y} = star
  if (point.privateLabel) {
    ctx.save()
    ctx.globalCompositeOperation = 'source-over'
    ctx.strokeStyle = theme.privateColor
    ctx.lineWidth = 1.2
    ctx.beginPath()
    ctx.arc(x, y, 3.6 + detail * .9, 0, FULL_TURN)
    ctx.stroke()
    ctx.globalAlpha = .55
    ctx.fillStyle = theme.privateColor
    ctx.beginPath()
    ctx.arc(x, y, .9, 0, FULL_TURN)
    ctx.fill()
    ctx.restore()
    return
  }

  const light = theme.mode === 'light'
  const heat = point.progress === 'implemented' ? .34 : Math.max(0, Math.min(1, point.brightness))
  const density = Math.min(5, Math.log2(Math.max(1, point.count ?? 1)))
  const radius = (light ? 1.32 + heat * 1.55 : 1.20 + heat * 1.65) + Math.min(.6, density * .18) + detail * (light ? .95 : 1.15) + sparse * .45
  const halo = (8.5 + heat * 12.5 + density * 3.0 + detail * 2.8) * scale * (1 + sparse * .32) + pulse * 13
  const sprites = glows[point.progress] ?? glows.feedback

  // 1) 浅色宣纸柔光托底：把焦墨星核与松烟墨雾自然托开，无硬白圈。
  if (glows.cushion) {
    const cushion = Math.max(radius * 3.6, halo * .76)
    ctx.globalAlpha = .58 + heat * .14 + sparse * .12
    ctx.drawImage(glows.cushion, x - cushion, y - cushion, 2 * cushion, 2 * cushion)
  }

  // 2) 柔和星晕 / 浅色淡墨晕
  ctx.globalAlpha = Math.min(.95, theme.glow.base + heat * theme.glow.slope + sparse * .16 + pulse * .16)
  ctx.drawImage(sprites.glow, x - halo, y - halo, 2 * halo, 2 * halo)

  // 3) 纤细星芒：按星等、稀疏度与放大平滑显露（直接贴预烘 Sprite）。
  const spikeLevel = Math.max(0, (heat - .36) / .64, density > .6 ? .44 : 0, detail * .45, sparse * .62)
  if (spikeLevel > .04) {
    const ray = radius * (2.2 + spikeLevel * 2.8 + detail * .55) * (.68 + .32 * scale) + pulse * 4
    ctx.globalAlpha = light ? (.28 + spikeLevel * .36) : (.36 + spikeLevel * .52)
    const diagonal = heat >= .80 || density >= 1.5 || sparse > .65
    ctx.drawImage(diagonal ? sprites.spike8 : sprites.spike4, x - ray, y - ray, 2 * ray, 2 * ray)
  }

  // 4) 细腻羽化星核（预烘 Sprite，零每帧渐变创建）。
  const coreRadius = radius * 1.15
  ctx.globalAlpha = 1
  ctx.drawImage(sprites.core, x - coreRadius, y - coreRadius, 2 * coreRadius, 2 * coreRadius)
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
