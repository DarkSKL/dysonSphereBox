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

let cachedWispMode = ''
let cachedWispCool: HTMLCanvasElement | undefined
let cachedWispWarm: HTMLCanvasElement | undefined

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

function wispBrushes(theme: GalaxyTheme): {cool: HTMLCanvasElement; warm: HTMLCanvasElement} {
  if (cachedWispMode !== theme.mode || !cachedWispCool || !cachedWispWarm) {
    cachedWispMode = theme.mode
    cachedWispCool = makeSoftBrush(theme.sky.wispCool, 32)
    cachedWispWarm = makeSoftBrush(theme.sky.wispWarm, 32)
  }
  return {cool: cachedWispCool, warm: cachedWispWarm}
}

/** 沿当前方向投影静默星盘底盘（隐约旋臂尘丝 + 远景极细微尘）。使用全景尺度，cloud 越高底盘尘丝越退后。 */
export function drawSkyDust(ctx: CanvasRenderingContext2D, camera: Camera, size: ViewSize, theme: GalaxyTheme, cloud = 0): void {
  const skyCamera = {...camera, zoom: 1}
  const frame = cameraFrame(skyCamera, size)
  const level = Math.max(0, Math.min(1, cloud))
  const wispFade = 1 - .78 * level
  const starFade = (1 - .32 * level) * theme.sky.dustScale
  const minDim = Math.max(1, Math.min(size.width, size.height))
  const brushes = wispBrushes(theme)
  ctx.save()
  ctx.globalCompositeOperation = theme.mode === 'dark' ? 'screen' : 'source-over'
  // 1) 隐约尘埃丝缕：沿旋臂切向拉伸的羽化柔丝，构建极克制的星盘轮廓。
  if (wispFade > .02) {
    for (const w of skyWisps) {
      const {x, y} = project(w, skyCamera, size, frame)
      const sx = w.tx * frame.right[0] + w.ty * frame.right[1]
      const sy = w.tx * frame.down[0] + w.ty * frame.down[1] + w.tz * frame.down[2]
      const len = Math.hypot(sx, sy)
      const ax = len > .01 ? sx / len : 1
      const ay = len > .01 ? sy / len : 0
      const rx = w.major * minDim
      const ry = w.minor * minDim * (.55 + .45 * Math.cos(camera.inclination))
      ctx.save()
      ctx.globalAlpha = theme.sky.wispAlpha * w.opacity * wispFade
      ctx.translate(x, y)
      ctx.rotate(Math.atan2(ay, ax))
      ctx.drawImage(w.warm ? brushes.warm : brushes.cool, -rx, -ry, 2 * rx, 2 * ry)
      ctx.restore()
    }
  }
  // 2) 细碎微弱的远景背景星尘：纯微点，无光晕与星芒，明确区分于可点反馈星。
  ctx.globalCompositeOperation = 'source-over'
  for (const dust of skyStars) {
    const {x, y} = project(dust, skyCamera, size, frame)
    if (x < -2 || x > size.width + 2 || y < -2 || y > size.height + 2) continue
    ctx.globalAlpha = dust.opacity * starFade
    ctx.fillStyle = dust.warm ? theme.sky.warmDust : theme.sky.dust
    ctx.beginPath()
    ctx.arc(x, y, dust.radius, 0, FULL_TURN)
    ctx.fill()
  }
  ctx.restore()
}

/** 银心柔和多层微光：0 条时作为静默星盘的核心呼吸点，随星云亮起柔和退隐；无生硬黑点/色块。 */
export function drawSeedGlow(ctx: CanvasRenderingContext2D, size: ViewSize, theme: GalaxyTheme, amount: number, camera: Camera = HOME_CAMERA): void {
  if (amount <= .004) return
  const x = size.width / 2, y = size.height / 2
  const minDim = Math.max(1, Math.min(size.width, size.height))
  const outer = Math.max(64, minDim * .25)
  const mid = outer * .56
  const inner = Math.max(11, minDim * .030)
  const tilt = .52 + .48 * Math.cos(camera.inclination)
  ctx.save()
  ctx.globalCompositeOperation = theme.mode === 'dark' ? 'screen' : 'source-over'
  // 外层宽雾：清透冷雾光晕，衔接外围星盘。
  const outerGrad = ctx.createRadialGradient(x, y, 0, x, y, outer)
  outerGrad.addColorStop(0, theme.sky.glowMid + '88')
  outerGrad.addColorStop(.32, theme.sky.glowMid + '38')
  outerGrad.addColorStop(.68, theme.sky.glowMid + '0c')
  outerGrad.addColorStop(1, theme.sky.glowMid + '00')
  ctx.globalAlpha = theme.sky.glowAlpha * .55 * amount
  ctx.fillStyle = outerGrad
  ctx.fillRect(x - outer, y - outer, 2 * outer, 2 * outer)

  // 中层倾角银盘光：随相机俯仰压扁，产生立体星盘感。
  ctx.save()
  ctx.translate(x, y)
  ctx.rotate(camera.yaw * .14)
  ctx.scale(1, tilt)
  const midGrad = ctx.createRadialGradient(0, 0, 0, 0, 0, mid)
  midGrad.addColorStop(0, theme.sky.glow)
  midGrad.addColorStop(.26, theme.sky.glow + '80')
  midGrad.addColorStop(.60, theme.sky.glow + '22')
  midGrad.addColorStop(1, theme.sky.glow + '00')
  ctx.globalAlpha = theme.sky.glowAlpha * .95 * amount
  ctx.fillStyle = midGrad
  ctx.fillRect(-mid, -mid, 2 * mid, 2 * mid)
  ctx.restore()

  // 内层柔光芯：柔和过渡的微光核心，浅色与深色均不画生硬实心圆点。
  const coreGrad = ctx.createRadialGradient(x, y, 0, x, y, inner)
  coreGrad.addColorStop(0, theme.mode === 'dark' ? '#fff4de' : theme.sky.glow)
  coreGrad.addColorStop(.35, theme.sky.glow + '88')
  coreGrad.addColorStop(1, theme.sky.glow + '00')
  ctx.globalAlpha = Math.min(.75, theme.sky.glowAlpha * 2.1 * amount)
  ctx.fillStyle = coreGrad
  ctx.fillRect(x - inner, y - inner, 2 * inner, 2 * inner)
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

type StarGlows = Record<StarProgress, HTMLCanvasElement> & {cushion: HTMLCanvasElement | null}

/** 只缓存三种色光与浅色柔光托底的小笔刷，画布上的每次落笔都由反馈坐标触发。主题切换时重新创建。 */
export function createStarGlows(theme: GalaxyTheme): StarGlows {
  function brush(color: string): HTMLCanvasElement {
    const canvas = document.createElement('canvas')
    const radius = 48
    canvas.width = canvas.height = radius * 2
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const glow = ctx.createRadialGradient(radius, radius, 0, radius, radius, radius)
      glow.addColorStop(0, color)
      glow.addColorStop(.14, color + 'a8')
      glow.addColorStop(.34, color + '44')
      glow.addColorStop(.64, color + '10')
      glow.addColorStop(1, color + '00')
      ctx.fillStyle = glow
      ctx.fillRect(0, 0, canvas.width, canvas.height)
    }
    return canvas
  }
  const colors = theme.starColors
  return {
    feedback: brush(colors.feedback),
    accepted: brush(colors.accepted),
    implemented: brush(colors.implemented),
    cushion: theme.coreCushion ? makeSoftBrush(theme.coreCushion, 36) : null,
  }
}

/** 纤细星芒：用两端渐隐的细光线替代粗硬实心菱形，放大看仍精致利落。 */
function drawStarSpikes(
  ctx: CanvasRenderingContext2D, x: number, y: number, length: number, thickness: number, color: string, alpha: number, diagonal: boolean,
): void {
  ctx.save()
  ctx.globalAlpha = alpha
  const drawAxis = (dx: number, dy: number, len: number, w: number) => {
    const grad = ctx.createLinearGradient(x - dx * len, y - dy * len, x + dx * len, y + dy * len)
    grad.addColorStop(0, color + '00')
    grad.addColorStop(.34, color + '55')
    grad.addColorStop(.5, color)
    grad.addColorStop(.66, color + '55')
    grad.addColorStop(1, color + '00')
    ctx.strokeStyle = grad
    ctx.lineWidth = w
    ctx.lineCap = 'round'
    ctx.beginPath()
    ctx.moveTo(x - dx * len, y - dy * len)
    ctx.lineTo(x + dx * len, y + dy * len)
    ctx.stroke()
  }
  drawAxis(1, 0, length, thickness)
  drawAxis(0, 1, length, thickness)
  if (diagonal) {
    const d = Math.SQRT1_2
    ctx.globalAlpha = alpha * .42
    drawAxis(d, d, length * .54, thickness * .78)
    drawAxis(d, -d, length * .54, thickness * .78)
  }
  ctx.restore()
}

// =============== 单颗反馈星 ===============
// 星核大小、柔光晕与纤细星芒按星等连续分级；浅色用柔光羽化托底而非硬白边，深色呈通透星光。
// 作者待审稿使用纤细空心星环，方便与公开星区分。scale 只收窄光晕，不改变星核与命中范围。
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
  const radius = (light ? 1.25 + heat * 1.55 : 1.20 + heat * 1.65) + Math.min(.6, density * .18) + detail * (light ? .95 : 1.15) + sparse * .45
  const halo = (8.5 + heat * 12.5 + density * 3.0 + detail * 2.8) * scale * (1 + sparse * .32) + pulse * 13
  const color = theme.starColors[point.progress] ?? theme.starColors.feedback

  // 1) 浅色柔光托底：把深色星核与淡雾柔和托开，完全去掉生硬白外圈。
  if (glows.cushion) {
    const cushion = Math.max(radius * 3.4, halo * .72)
    ctx.save()
    ctx.globalCompositeOperation = 'source-over'
    ctx.globalAlpha = .52 + heat * .14 + sparse * .12
    ctx.drawImage(glows.cushion, x - cushion, y - cushion, 2 * cushion, 2 * cushion)
    ctx.restore()
  }

  // 2) 柔和彩色星晕
  ctx.globalAlpha = Math.min(.95, theme.glow.base + heat * theme.glow.slope + sparse * .18 + pulse * .16)
  ctx.drawImage(glows[point.progress] ?? glows.feedback, x - halo, y - halo, 2 * halo, 2 * halo)
  ctx.globalAlpha = 1

  // 3) 纤细星芒：随星等、稀疏度与放大平滑显露，替代粗硬实心菱形。
  const spikeLevel = Math.max(0, (heat - .36) / .64, density > .6 ? .44 : 0, detail * .45, sparse * .62)
  if (spikeLevel > .04) {
    const ray = radius * (2.2 + spikeLevel * 2.8 + detail * .55) * (.68 + .32 * scale) + pulse * 4
    const thickness = light ? .76 + spikeLevel * .30 : .78 + spikeLevel * .40
    const alpha = light ? (.32 + spikeLevel * .40) : (.36 + spikeLevel * .52)
    drawStarSpikes(ctx, x, y, ray, thickness, light ? color : theme.coreHighlight, alpha, heat >= .80 || density >= 1.5 || sparse > .65)
  }

  // 4) 细腻羽化星核：深色由亮白芯自然化入彩光，浅色为清透深芯配极微弱针尖高光（无白洞珠子感）。
  const core = ctx.createRadialGradient(x, y, 0, x, y, radius * 1.15)
  if (light) {
    core.addColorStop(0, color)
    core.addColorStop(.72, color)
    core.addColorStop(.92, color + '88')
    core.addColorStop(1, color + '00')
  } else {
    core.addColorStop(0, theme.coreHighlight)
    core.addColorStop(.38 + heat * .16, theme.coreHighlight)
    core.addColorStop(.74, color)
    core.addColorStop(1, color + '00')
  }
  ctx.fillStyle = core
  ctx.beginPath()
  ctx.arc(x, y, radius * 1.15, 0, FULL_TURN)
  ctx.fill()

  if (light) {
    // 浅色极细针尖微光：让星核保持星图晶莹感，但面积极小且柔和，绝不形成空心圆环。
    const glint = Math.max(.42, radius * .24)
    ctx.save()
    ctx.globalAlpha = .30 + heat * .22
    ctx.fillStyle = theme.coreHighlight
    ctx.beginPath()
    ctx.arc(x - radius * .14, y - radius * .14, glint, 0, FULL_TURN)
    ctx.fill()
    ctx.restore()
  }
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
