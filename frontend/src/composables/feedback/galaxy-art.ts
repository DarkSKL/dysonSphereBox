import {progressLabels, type DisplayStar, type StarProgress} from '../../types/feedback'
import {cameraFrame, DISK_RADIUS, project, type Camera, type ProjectedStar, type ViewSize} from './galaxy-camera'
import {randomFor, spiralPhase} from './galaxy-nebula'
import {GALAXY_THEMES, type GalaxyTheme} from './galaxy-theme'

// 深色星色保留为导出常量，供星点列表等只需要一套色值的调用方兜底。
export const starColors: Record<StarProgress, string> = GALAXY_THEMES.dark.starColors
const FULL_TURN = 2 * Math.PI

// =============== 远景尘点与银心种子光 ===============
// 说明：远景尘点在任何反馈数量下都绘制，固定采样让同一视角下背景一致，数量变化时背景不跳变。
// 银心种子光只在反馈很少时出现，作为 0 条时的视觉焦点，随云雾亮起而淡出。尘点只用于绘制，不进入列表、命中或预取。
const skyRandom = randomFor('feedback-empty-sky')
const emptySky = Array.from({length: 48}, (_, i) => {
  const halo = i % 5 === 4
  const r = halo ? .38 + .9 * skyRandom() : .06 + .91 * (Math.floor(i / 2) + skyRandom()) / 24
  const angle = halo ? FULL_TURN * skyRandom() : (i % 2) * Math.PI - .65 + spiralPhase(r) + (skyRandom() - .5) * .52
  return {
    x: DISK_RADIUS * r * Math.cos(angle), y: DISK_RADIUS * r * Math.sin(angle),
    z: DISK_RADIUS * (skyRandom() - .5) * (halo ? .48 : .12),
    radius: .55 + .57 * skyRandom(), opacity: .20 + .19 * skyRandom(),
    warm: !halo && r < .32,
    mist: !halo && i % 3 !== 1,
  }
})

/** 沿当前方向投影稀疏的远景尘点。使用全景尺度，高倍率下也有轻微层次；cloud 为 0..1 的云雾亮度，云越亮尘点越淡。 */
export function drawSkyDust(ctx: CanvasRenderingContext2D, camera: Camera, size: ViewSize, theme: GalaxyTheme, cloud = 0): void {
  const skyCamera = {...camera, zoom: 1}
  const frame = cameraFrame(skyCamera, size)
  const spread = Math.min(40, Math.max(12, Math.min(size.width, size.height) * .042))
  const fade = 1 - .45 * Math.max(0, Math.min(1, cloud))
  ctx.save()
  for (const dust of emptySky) {
    const {x, y} = project(dust, skyCamera, size, frame)
    const color = dust.warm ? theme.sky.warmDust : theme.sky.dust
    if (dust.mist) {
      const mist = ctx.createRadialGradient(x, y, 0, x, y, spread)
      mist.addColorStop(0, color)
      mist.addColorStop(1, color + '00')
      ctx.globalAlpha = theme.sky.mistAlpha * fade
      ctx.fillStyle = mist
      ctx.fillRect(x - spread, y - spread, 2 * spread, 2 * spread)
    }
    ctx.globalAlpha = dust.opacity * fade
    ctx.fillStyle = color
    ctx.beginPath()
    ctx.arc(x, y, dust.radius, 0, FULL_TURN)
    ctx.fill()
  }
  ctx.restore()
}

/** 银心种子光：0 条时画面唯一的柔和焦点，amount 为 0..1。银心始终投影在画布中心，因此焦点位置不随视角漂移。 */
export function drawSeedGlow(ctx: CanvasRenderingContext2D, size: ViewSize, theme: GalaxyTheme, amount: number): void {
  if (amount <= .004) return
  const x = size.width / 2, y = size.height / 2
  const radius = Math.max(36, Math.min(size.width, size.height) * .2)
  const glow = ctx.createRadialGradient(x, y, 0, x, y, radius)
  glow.addColorStop(0, theme.sky.glow)
  glow.addColorStop(.35, theme.sky.glow + '55')
  glow.addColorStop(1, theme.sky.glow + '00')
  ctx.save()
  ctx.globalAlpha = theme.sky.glowAlpha * amount
  ctx.fillStyle = glow
  ctx.fillRect(x - radius, y - radius, 2 * radius, 2 * radius)
  ctx.globalAlpha = Math.min(1, theme.sky.glowAlpha * 4.2 * amount)
  ctx.fillStyle = theme.sky.glow
  ctx.beginPath()
  ctx.arc(x, y, 1.8, 0, FULL_TURN)
  ctx.fill()
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

type StarGlows = Record<StarProgress, HTMLCanvasElement>

/** 只缓存三种色光的小笔刷，画布上的每次落笔都由反馈坐标触发。主题切换时重新创建。 */
export function createStarGlows(theme: GalaxyTheme): StarGlows {
  function brush(color: string): HTMLCanvasElement {
    const canvas = document.createElement('canvas')
    const radius = 48
    canvas.width = canvas.height = radius * 2
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const glow = ctx.createRadialGradient(radius, radius, 0, radius, radius, radius)
      glow.addColorStop(0, color)
      glow.addColorStop(.16, color + 'aa')
      glow.addColorStop(.42, color + '42')
      glow.addColorStop(.72, color + '0d')
      glow.addColorStop(1, color + '00')
      ctx.fillStyle = glow
      ctx.fillRect(0, 0, canvas.width, canvas.height)
    }
    return canvas
  }
  const colors = theme.starColors
  return {feedback: brush(colors.feedback), accepted: brush(colors.accepted), implemented: brush(colors.implemented)}
}

// =============== 单颗反馈星 ===============
// 星核和光晕表达热度，聚合计数只加宽现有光晕。作者待审稿使用空心标记，方便与公开星区分。
// scale 为 haloScale 的结果，只缩小光晕，不改变星核大小，命中范围不受影响。
export function drawFeedbackStar(
  ctx: CanvasRenderingContext2D, glows: StarGlows, star: ProjectedStar, pulse: number, detail: number,
  theme: GalaxyTheme, scale = 1,
): void {
  const {point, x, y} = star
  if (point.privateLabel) {
    ctx.strokeStyle = theme.privateColor
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.arc(x, y, 4 + detail, 0, FULL_TURN)
    ctx.stroke()
    return
  }

  const heat = point.progress === 'implemented' ? .24 : Math.max(0, Math.min(1, point.brightness))
  const density = Math.min(5, Math.log2(Math.max(1, point.count ?? 1)))
  const radius = 1.6 + heat * 1.6 + detail * 1.6
  const halo = (9 + heat * 12 + density * 3 + detail * 3) * scale + pulse * 12
  const color = theme.starColors[point.progress] ?? theme.starColors.feedback
  ctx.globalAlpha = theme.glow.base + heat * theme.glow.slope + pulse * .18
  ctx.drawImage(glows[point.progress] ?? glows.feedback, x - halo, y - halo, 2 * halo, 2 * halo)
  ctx.globalAlpha = 1
  if (theme.coreRing) {
    // 浅色下深色星核落在灰色云雾上，先垫一圈白边保证星核轮廓清楚。
    ctx.fillStyle = theme.coreRing
    ctx.beginPath()
    ctx.arc(x, y, radius + 1.4, 0, FULL_TURN)
    ctx.fill()
  }
  ctx.fillStyle = color
  if (heat >= .86) {
    // 热门星的细星芒穿过本身的星核，仍然只有一个可命中中心。
    const ray = 5 + heat * 3
    ctx.beginPath()
    ctx.moveTo(x - ray, y)
    ctx.quadraticCurveTo(x, y, x, y - ray)
    ctx.quadraticCurveTo(x, y, x + ray, y)
    ctx.quadraticCurveTo(x, y, x, y + ray)
    ctx.quadraticCurveTo(x, y, x - ray, y)
    ctx.fill()
  }
  ctx.beginPath()
  ctx.arc(x, y, radius, 0, FULL_TURN)
  ctx.fill()
  ctx.fillStyle = theme.coreHighlight
  ctx.beginPath()
  ctx.arc(x, y, radius * (.36 + heat * .24), 0, FULL_TURN)
  ctx.fill()
}

/** 选中或悬停标记最后绘制，密集区域里的目标始终可辨。 */
export function drawStarFocus(ctx: CanvasRenderingContext2D, {point, x, y}: ProjectedStar, theme: GalaxyTheme): void {
  ctx.beginPath()
  for (let i = 0; i < 4; i++) {
    const angle = (i + .15) * Math.PI / 2
    ctx.moveTo(x + 10 * Math.cos(angle), y + 10 * Math.sin(angle))
    ctx.arc(x, y, 10, angle, angle + Math.PI / 3)
  }
  ctx.strokeStyle = theme.focusOutline
  ctx.lineWidth = 3
  ctx.stroke()
  ctx.strokeStyle = point.privateLabel ? theme.privateColor : (theme.starColors[point.progress] ?? theme.starColors.feedback)
  ctx.lineWidth = 1.3
  ctx.stroke()
}
