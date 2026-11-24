import {progressLabels, type DisplayStar, type StarProgress} from '../../types/feedback'
import {cameraFrame, DISK_RADIUS, project, type Camera, type ProjectedStar, type ViewSize} from './galaxy-camera'
import {randomFor, spiralPhase} from './galaxy-nebula'

export const starColors: Record<StarProgress, string> = {
  feedback: '#91dbff',
  accepted: '#c3a0ff',
  implemented: '#ffc982',
}
const PRIVATE_COLOR = '#b5c1d0'
const FULL_TURN = 2 * Math.PI

// =============== 空视野远景 ===============
// 固定采样让再次进入空视野时得到同一片天空。尘点只用于绘制，不进入反馈列表、命中或预取。
const skyRandom = randomFor('feedback-empty-sky')
const emptySky = Array.from({length: 48}, (_, i) => {
  const halo = i % 5 === 4
  const r = halo ? .38 + .9 * skyRandom() : .06 + .91 * (Math.floor(i / 2) + skyRandom()) / 24
  const angle = halo ? FULL_TURN * skyRandom() : (i % 2) * Math.PI - .65 + spiralPhase(r) + (skyRandom() - .5) * .52
  return {
    x: DISK_RADIUS * r * Math.cos(angle), y: DISK_RADIUS * r * Math.sin(angle),
    z: DISK_RADIUS * (skyRandom() - .5) * (halo ? .48 : .12),
    radius: .55 + .57 * skyRandom(), opacity: .20 + .19 * skyRandom(),
    color: !halo && r < .32 ? '#b5a69b' : '#9eaec7',
    mist: !halo && i % 3 !== 1,
  }
})

/** 沿当前方向投影稀疏的远景尘点。使用全景尺度，空视野在高倍率下也有轻微层次。 */
export function drawEmptySky(ctx: CanvasRenderingContext2D, camera: Camera, size: ViewSize): void {
  const skyCamera = {...camera, zoom: 1}
  const frame = cameraFrame(skyCamera, size)
  const spread = Math.min(40, Math.max(12, Math.min(size.width, size.height) * .042))
  ctx.save()
  for (const dust of emptySky) {
    const {x, y} = project(dust, skyCamera, size, frame)
    if (dust.mist) {
      const mist = ctx.createRadialGradient(x, y, 0, x, y, spread)
      mist.addColorStop(0, dust.color)
      mist.addColorStop(1, dust.color + '00')
      ctx.globalAlpha = .045
      ctx.fillStyle = mist
      ctx.fillRect(x - spread, y - spread, 2 * spread, 2 * spread)
    }
    ctx.globalAlpha = dust.opacity
    ctx.fillStyle = dust.color
    ctx.beginPath()
    ctx.arc(x, y, dust.radius, 0, FULL_TURN)
    ctx.fill()
  }
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

/** 只缓存三种色光的小笔刷，画布上的每次落笔都由反馈坐标触发。 */
export function createStarGlows(): StarGlows {
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
  return {feedback: brush(starColors.feedback), accepted: brush(starColors.accepted), implemented: brush(starColors.implemented)}
}

// =============== 单颗反馈星 ===============
// 星核和光晕表达热度，聚合计数只加宽现有光晕。作者待审稿使用空心标记，方便与公开星区分。
export function drawFeedbackStar(ctx: CanvasRenderingContext2D, glows: StarGlows, star: ProjectedStar, pulse: number, detail: number): void {
  const {point, x, y} = star
  if (point.privateLabel) {
    ctx.strokeStyle = PRIVATE_COLOR
    ctx.lineWidth = 1.5
    ctx.beginPath()
    ctx.arc(x, y, 4 + detail, 0, FULL_TURN)
    ctx.stroke()
    return
  }

  const heat = point.progress === 'implemented' ? .24 : Math.max(0, Math.min(1, point.brightness))
  const density = Math.min(5, Math.log2(Math.max(1, point.count ?? 1)))
  const radius = 1.6 + heat * 1.6 + detail * 1.6
  const halo = 9 + heat * 12 + density * 3 + pulse * 12 + detail * 3
  const color = starColors[point.progress] ?? starColors.feedback
  ctx.globalAlpha = .43 + heat * .32 + pulse * .18
  ctx.drawImage(glows[point.progress] ?? glows.feedback, x - halo, y - halo, 2 * halo, 2 * halo)
  ctx.globalAlpha = 1
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
  ctx.fillStyle = '#f3f7ff'
  ctx.beginPath()
  ctx.arc(x, y, radius * (.36 + heat * .24), 0, FULL_TURN)
  ctx.fill()
}

/** 选中或悬停标记最后绘制，密集区域里的目标始终可辨。 */
export function drawStarFocus(ctx: CanvasRenderingContext2D, {point, x, y}: ProjectedStar): void {
  ctx.beginPath()
  for (let i = 0; i < 4; i++) {
    const angle = (i + .15) * Math.PI / 2
    ctx.moveTo(x + 10 * Math.cos(angle), y + 10 * Math.sin(angle))
    ctx.arc(x, y, 10, angle, angle + Math.PI / 3)
  }
  ctx.strokeStyle = '#111724'
  ctx.lineWidth = 3
  ctx.stroke()
  ctx.strokeStyle = point.privateLabel ? PRIVATE_COLOR : (starColors[point.progress] ?? starColors.feedback)
  ctx.lineWidth = 1.3
  ctx.stroke()
}
