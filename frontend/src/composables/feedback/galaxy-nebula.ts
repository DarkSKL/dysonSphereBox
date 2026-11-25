import type {DisplayStar} from '../../types/feedback'
import {DISK_RADIUS} from './galaxy-camera'
import {GALAXY_THEMES, type GalaxyTheme} from './galaxy-theme'

// =============== 反馈星的体积光采样 ===============
// 每粒尘埃保存实星原点、曲线系数、生命周期与暗带参数。旋臂的形状和材料均从可见反馈产生。
export type NebulaSource = Pick<DisplayStar, 'starId' | 'x' | 'y' | 'z' | 'count' | 'privateLabel'>
export const SAMPLE_STRIDE = 21
export const FLOW_PERIOD_MS = 48000
export const PALETTE_SIZE = 64

// =============== 云雾色带 ===============
// 说明：把主题里的色标插值为 64 级查找表，供 WebGL 纹理与软件笔刷共用。
// 使用：按主题取 nebulaPaletteFor；深色表保留 nebulaPalette 导出，旧调用方不受影响。
// 边界：只含颜色；明暗叠加方式与曝光在主题的 nebula.blend / nebula.gain 中。
export function buildPalette(stops: GalaxyTheme['nebula']['stops']): Uint8Array {
  const palette = new Uint8Array(PALETTE_SIZE * 4)
  for (let i = 0; i < PALETTE_SIZE; i++) {
    const r = i / (PALETTE_SIZE - 1)
    let end = 1
    while (end < stops.length - 1 && r > stops[end][0]) end++
    const left = stops[end - 1], right = stops[end]
    const t = (r - left[0]) / (right[0] - left[0])
    for (let channel = 0; channel < 3; channel++) {
      palette[i * 4 + channel] = Math.round(left[channel + 1] * (1 - t) + right[channel + 1] * t)
    }
    palette[i * 4 + 3] = 255
  }
  return palette
}

export const nebulaPalette = buildPalette(GALAXY_THEMES.dark.nebula.stops)
const palettes = {dark: nebulaPalette, light: buildPalette(GALAXY_THEMES.light.nebula.stops)}
export function nebulaPaletteFor(theme: GalaxyTheme): Uint8Array {
  return palettes[theme.mode]
}

// =============== 云雾存在感与曝光 ===============
// 说明：云雾亮度随可见反馈量连续变化。presence 让第一颗星就带出柔和的云，数量增长时平滑逼近 1；
// exposure 在数量很大时缓慢压低每颗星的光量，避免密集时过曝成另一种画面。
// 使用：galaxy-map 用可见星质量算出目标值，再按时间缓动后交给渲染器；曲线不依赖主题。
// 边界：只描述标量曲线，不读取画布或采样缓冲；0 条时 presence 为 0，不绘制云雾。
export function cloudPresence(mass: number): number {
  return 1 - Math.exp(-Math.max(0, mass) / 1.6)
}

export function cloudExposure(mass: number): number {
  return 1 / (1 + .2 * Math.log2(1 + Math.max(0, mass - 4) / 4))
}

export function randomFor(id: string): () => number {
  let seed = 2166136261
  for (let i = 0; i < id.length; i++) seed = Math.imul(seed ^ id.charCodeAt(i), 16777619)
  return () => {
    seed = (seed + 0x6d2b79f5) | 0
    let n = Math.imul(seed ^ seed >>> 15, seed | 1)
    n ^= n + Math.imul(n ^ n >>> 7, n | 61)
    return ((n ^ n >>> 14) >>> 0) / 4294967296
  }
}

export function spiralPhase(r: number): number { return 3.15 * Math.log1p(5 * r) }

export function publicMass(stars: readonly NebulaSource[]): number {
  return stars.reduce((mass, p) => mass + (p.privateLabel ? 0 : Math.max(1, p.count ?? 1)), 0)
}

/** 让星核所属的云雾在视区边缘平滑出入。屏幕坐标和边距均用 CSS 像素。 */
export function edgeTransmission(distance: number, width: number, height: number): number {
  const t = Math.max(0, Math.min(1, distance / Math.max(1, Math.min(32, .08 * Math.min(width, height)))))
  return t * t * (3 - 2 * t)
}

export function nebulaKey(stars: readonly DisplayStar[]): string {
  return stars.filter(p => !p.privateLabel).map(p => [p.starId, p.x, p.y, p.z ?? 0, p.count ?? 1].join('/')).join('|')
}

export function particleBudget(starCount: number): number {
  return Math.min(800, Math.floor(180000 / Math.max(1, starCount)))
}

/** 以对数旋臂上外、中、内三个位置拟合二次曲线。银心附近的路径逐渐过渡为短圆弧。 */
function writeTrack(out: Float32Array, i: number, r: number, angle: number, halfSpan: number, lift: number) {
  const bulge = Math.max(0, 1 - r / .23) ** 2
  const dr = Math.min(r * .31, halfSpan) * (1 - .78 * bulge)
  const outer = r + dr, inner = r - dr
  const phase = spiralPhase(r)
  const a = angle + spiralPhase(outer) - phase + bulge * .32
  const b = angle + spiralPhase(inner) - phase - bulge * .32
  const ax = DISK_RADIUS * outer * Math.cos(a), ay = DISK_RADIUS * outer * Math.sin(a)
  const bx = DISK_RADIUS * inner * Math.cos(b), by = DISK_RADIUS * inner * Math.sin(b)
  out[i + 9] = (bx - ax) / 2
  out[i + 10] = (by - ay) / 2
  out[i + 11] = lift
  out[i + 12] = (bx + ax) / 2 - out[i]
  out[i + 13] = (by + ay) / 2 - out[i + 1]
  out[i + 14] = lift * .40 * Math.cos(2 * angle)
}

export function sampleNebula(points: readonly NebulaSource[], budget = particleBudget(points.length)): Float32Array {
  const stars = points.filter(p => !p.privateLabel)
  // 第一颗公开星就带出一小片云，避免 1 条与 2 条之间出现“有云/无云”的断层。
  if (publicMass(stars) < 1) return new Float32Array()
  const samples = new Float32Array(stars.length * budget * SAMPLE_STRIDE)
  let cursor = 0
  for (const star of stars) {
    const random = randomFor(star.starId)
    const normal = () => (random() + random() + random() - 1.5) * 1.9
    const radius = Math.hypot(star.x, star.y) / DISK_RADIUS
    const theta = Math.atan2(star.y, star.x), phase = spiralPhase(radius), height = star.z ?? 0
    const armStrength = Math.exp(-3.6 * Math.sin(theta + .65 - phase) ** 2)
    const weight = 1 + Math.min(1.6, .21 * Math.log2(Math.max(1, star.count ?? 1)))
    const seedPhase = random()
    // 同一缕云丝中的颗粒共享弯曲、相位和速度，两缕错开半程交接。端部稀薄，中段保留可追踪的亮纹。
    const strands = [0, 1].map(k => ({
      offset: normal() * .029, span: .024 + .030 * random(), lane: (k ? .072 : -.095) + .032 * normal(),
      turn: random() * 6.28, height: normal() * 980, age: (seedPhase + k * .5) % 1,
      rate: k ? 6 : 5, travel: (.061 + .028 * random()) * (.75 + .25 * armStrength),
    }))

    for (let i = 0; i < budget; i++, cursor += SAMPLE_STRIDE) {
      const slot = i % 16
      const fog = slot === 0
      const silk = slot === 2 || slot === 9 || slot === 14
      const strand = strands[Math.floor(i / 16) % 2]
      const inStrand = radius > .12 && (silk || slot === 6 || slot === 12)
      const diffuse = !inStrand && random() < .23
      const u = random() * 2 - 1
      let x: number, y: number, z: number
      if (inStrand) {
        const r = Math.max(.012, radius + strand.offset + strand.span * u + normal() * .0014)
        const weave = .021 * Math.sin(3.4 * u + strand.turn) + .008 * Math.sin(7 * u + strand.turn)
        const a = theta + spiralPhase(r) - phase + strand.lane + weave + normal() * .012
        x = DISK_RADIUS * r * Math.cos(a)
        y = DISK_RADIUS * r * Math.sin(a)
        z = height + strand.height + 680 * Math.sin(2.3 * u + strand.turn) + normal() * 320
      } else if (radius < .10 || (radius < .19 && random() < (.19 - radius) / .09)) {
        const spread = DISK_RADIUS * (diffuse ? .070 + .060 * random() : .021 + .034 * random())
        x = star.x + normal() * spread
        y = star.y + normal() * spread
        z = height + normal() * spread * .76
      } else {
        const r = Math.abs(radius + normal() * (diffuse ? .12 : .061 + .037 * radius))
        const lane = random()
        const across = diffuse ? normal() * .67 : lane < .48 ? -.105 + normal() * .055 :
          lane < .77 ? .092 + normal() * .048 : normal() * .21
        const ripple = .018 * (Math.sin(28 * r + theta) + Math.sin(54 * r - theta))
        const a = theta + spiralPhase(r) - phase + across + ripple
        x = DISK_RADIUS * r * Math.cos(a)
        y = DISK_RADIUS * r * Math.sin(a)
        z = height + normal() * (diffuse ? 3700 : fog ? 1800 : 880)
      }

      const r = Math.hypot(x, y) / DISK_RADIUS, angle = Math.atan2(y, x)
      const core = Math.exp(-24 * r * r)
      const seam = Math.sin(angle + .65 - spiralPhase(r) - .11 - .055 * Math.sin(30 * r + Math.sin(3 * angle)))
      const dustLane = (1 - core) * Math.exp(-seam * seam / .008)
      const knots = .78 + .22 * Math.sin(40 * r + 2 * angle + Math.sin(17 * r - angle))
      const temp = diffuse && random() < .32 ? .42 * r : r
      const tint = Math.round(Math.max(0, Math.min(1, temp + normal() * (inStrand ? .025 : .06))) * (PALETTE_SIZE - 1))
      const diameter = fog ? (diffuse ? 12500 : 4800) + random() ** 2 * 13500 :
        silk ? 650 + random() ** 2 * 2200 : 120 + random() ** 4 * 490
      const emission = fog ? (diffuse ? .022 : .054) + .010 * core :
        silk ? .105 + .085 * random() : .13 + .65 * random() ** 3
      // 亮脊靠向前缘，外侧拖出薄尾；散落在臂间的反馈保留较松散的局部气体。
      const shoulder = Math.exp(-((u + .32) ** 2) / .085)
      const taper = inStrand ? (.22 + .63 * (1 - u * u) + .61 * shoulder) * (.38 + .62 * armStrength) : 1
      const edge = Math.max(0, Math.min(1, (1.12 - r) / .17))

      samples[cursor] = x
      samples[cursor + 1] = y
      samples[cursor + 2] = z
      samples[cursor + 3] = diameter
      samples[cursor + 4] = emission * weight * knots * taper * edge
      samples[cursor + 5] = tint
      samples[cursor + 6] = star.x
      samples[cursor + 7] = star.y
      samples[cursor + 8] = height
      const travel = inStrand ? strand.travel : fog ? (diffuse ? .025 : .043) : .037 + .022 * random()
      writeTrack(samples, cursor, r, angle, travel, (inStrand ? .6 * strand.height : 540 * Math.sin(2 * angle + 4 * r)))
      samples[cursor + 15] = inStrand ? (strand.age + .026 * u + .012 * random() + 1) % 1 : random()
      samples[cursor + 16] = inStrand ? strand.rate : fog ? (diffuse ? 2 : 3) : 4 + (i % 3)
      samples[cursor + 17] = silk ? 1.5 + (1.15 + 1.6 * random()) * armStrength : fog ? (diffuse ? 1.25 : 1.85) : 1
      const front = 24 * r + .9 * Math.sin(2 * angle - 7 * r)
      samples[cursor + 18] = Math.cos(front)
      samples[cursor + 19] = Math.sin(front)
      samples[cursor + 20] = dustLane
    }
  }
  return samples
}
