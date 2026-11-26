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

/** 云雾亮度降到该值以下才视为淡出结束。 */
export const CLOUD_VISIBLE = .003

/**
 * 云雾亮度向目标缓动一步。reducedMotion 或首次调用（level 为负）时直接到位；dt 以毫秒计，上限 250 防止切后台后一步跳过。
 * 返回新亮度与是否仍在趋近目标，调用方据此决定是否继续排帧。
 */
export function easeCloudLevel(level: number, target: number, dt: number, reducedMotion: boolean): {level: number; settling: boolean} {
  if (level < 0 || reducedMotion) return {level: target, settling: false}
  const next = level + (target - level) * (1 - Math.exp(-Math.min(250, Math.max(0, dt)) / 160))
  const done = Math.abs(target - next) < .004
  return {level: done ? target : next, settling: !done}
}

/**
 * 本帧是否绘制云雾层，以及应使用哪一批采样。
 * 采样先于亮度清空：最后一颗公开星消失时 samples 立刻为空，但亮度还在淡出，此时用上一批非空采样（previous）继续绘制，
 * 亮度降到 CLOUD_VISIBLE 以下才停绘并要求释放云雾层。亮度为 0 且采样为空（始终 0 条）时不绘制，不会有残留。
 */
export function cloudPlan<T extends {length: number}>(level: number, samples: T, previous: T | undefined):
  {draw: boolean; use: T | undefined; release: boolean; keep: T | undefined} {
  const keep = samples.length ? samples : previous
  const draw = level > CLOUD_VISIBLE && !!keep?.length
  return {draw, use: draw ? keep : undefined, release: !draw && !samples.length, keep: draw ? keep : undefined}
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
    // 三缕交错的细纤维丝缕（端部柔和渐隐，避免孤星出现生硬划痕感，密集时织出通透多层丝缕）。
    const strands = [0, 1, 2].map(k => ({
      offset: normal() * .034, span: .032 + .038 * random(),
      lane: ((k - 1) * .082) + .038 * normal(),
      turn: random() * 6.28, height: normal() * 1050, age: (seedPhase + k / 3) % 1,
      rate: 4.5 + k * .8, travel: (.056 + .026 * random()) * (.72 + .28 * armStrength),
    }))

    for (let i = 0; i < budget; i++, cursor += SAMPLE_STRIDE) {
      const slot = i % 16
      const fog = slot === 0 || slot === 8
      const cocoon = slot === 4 || slot === 11
      const silk = slot === 2 || slot === 9 || slot === 14
      const strand = strands[Math.floor(i / 16) % strands.length]
      const inStrand = !cocoon && radius > .12 && (silk || slot === 6 || slot === 12)
      const diffuse = !inStrand && !cocoon && random() < .46
      // 将星云原有的弥散散云中约一半自然扩散至星盘外围更广区域，保持同源体积粒子材质与旋涡切线方向，不另起炉灶。
      const farDiffuse = diffuse && random() < .56
      const farWisp = farDiffuse && !fog && random() < .32
      const u = random() * 2 - 1
      let x: number, y: number, z: number
      if (cocoon) {
        // 贴合星体自身的局部柔和气晕：单星时自然托住反馈星，避免孤零零的尖锐月牙划痕。
        const spread = DISK_RADIUS * (.020 + .050 * Math.sqrt(random()))
        const drift = random() * 6.2832
        x = star.x + Math.cos(drift) * spread + normal() * spread * .30
        y = star.y + Math.sin(drift) * spread + normal() * spread * .30
        z = height + normal() * spread * .40
      } else if (inStrand) {
        const r = Math.max(.012, radius + strand.offset + strand.span * u + normal() * .0024)
        const weave = .026 * Math.sin(3.2 * u + strand.turn) + .010 * Math.sin(6.8 * u + strand.turn)
        const a = theta + spiralPhase(r) - phase + strand.lane + weave + normal() * .020
        x = DISK_RADIUS * r * Math.cos(a)
        y = DISK_RADIUS * r * Math.sin(a)
        z = height + strand.height + 680 * Math.sin(2.3 * u + strand.turn) + normal() * 380
      } else if (farDiffuse) {
        // 同源散云向外自然铺展至整个视野（宽屏左右与四角），沿对数旋涡切线延展并保留 3D 厚度，彻底消除星盘圆形边界。
        const reach = Math.pow(random(), .60)
        const r = Math.max(.20, radius * .42 + (.20 + 2.24 * reach) + normal() * .16)
        const ripple = .028 * (Math.sin(18 * r + theta) + Math.sin(34 * r - theta))
        const a = theta + spiralPhase(r) - phase + normal() * 2.35 + ripple
        x = DISK_RADIUS * r * Math.cos(a)
        y = DISK_RADIUS * r * Math.sin(a)
        z = height + normal() * (5600 + 16000 * Math.min(1.6, r))
      } else if (radius < .10 || (radius < .19 && random() < (.19 - radius) / .09)) {
        const spread = DISK_RADIUS * (diffuse ? .095 + .085 * random() : .024 + .038 * random())
        x = star.x + normal() * spread
        y = star.y + normal() * spread
        z = height + normal() * spread * .76
      } else {
        const r = Math.abs(radius + normal() * (diffuse ? .25 : .074 + .046 * radius))
        const lane = random()
        const across = diffuse ? normal() * 1.15 : lane < .46 ? -.116 + normal() * .076 :
          lane < .76 ? .104 + normal() * .068 : normal() * .30
        const ripple = .022 * (Math.sin(26 * r + theta) + Math.sin(50 * r - theta))
        const a = theta + spiralPhase(r) - phase + across + ripple
        x = DISK_RADIUS * r * Math.cos(a)
        y = DISK_RADIUS * r * Math.sin(a)
        z = height + normal() * (diffuse ? 4800 : fog ? 2200 : 1020)
      }

      const r = Math.hypot(x, y) / DISK_RADIUS, angle = Math.atan2(y, x)
      const core = Math.exp(-20 * r * r)
      const seam = Math.sin(angle + .65 - spiralPhase(r) - .11 - .055 * Math.sin(30 * r + Math.sin(3 * angle)))
      const laneFade = Math.max(0, Math.min(1, (1.28 - r) / .42))
      const dustLane = (1 - core) * laneFade * Math.exp(-seam * seam / .009)
      const knots = .74 + .26 * Math.sin(38 * r + 2 * angle + Math.sin(17 * r - angle))
      // 广域散云以低频三维波雕出自然的聚散丝团与深空暗缝，避免整片糊成均匀雾气。
      let farPatch = 1
      if (farDiffuse) {
        const nx = x / DISK_RADIUS, ny = y / DISK_RADIUS
        const wave =
          .50 * Math.sin(2.5 * nx + 1.9 * ny + 1.05 * Math.sin(1.8 * ny - 1.3 * nx)) +
          .32 * Math.sin(4.6 * ny - 3.2 * nx + .85 * Math.cos(2.6 * nx + 2.1 * ny)) +
          .18 * Math.cos(7.1 * nx + 5.7 * ny)
        const clump = Math.max(0, Math.min(1, (wave + .38) / 1.08))
        farPatch = clump * clump
      }
      const temp = farDiffuse
        ? .64 + .36 * Math.min(1, r / 2.2) - (random() < .28 ? .16 : 0)
        : diffuse && random() < .36 ? .46 * r : r
      const tint = Math.round(Math.max(0, Math.min(1, temp + normal() * (inStrand ? .028 : .065))) * (PALETTE_SIZE - 1))
      const diameter = cocoon ? 3600 + random() ** 2 * 6600 :
        farDiffuse ? (fog ? 11800 + random() ** 2 * 13600 : farWisp ? 1600 + random() ** 2 * 3400 : 140 + random() ** 4 * 460) :
        fog ? (diffuse ? 9800 : 4200) + random() ** 2 * 10200 :
        silk ? 540 + random() ** 2 * 2000 : 110 + random() ** 4 * 420
      const emission = cocoon ? .072 + .030 * (1 - Math.min(1, Math.hypot(x - star.x, y - star.y) / (DISK_RADIUS * .095))) :
        farDiffuse ? (fog ? .034 : farWisp ? .052 : .15 * random() ** 2) * farPatch :
        fog ? (diffuse ? .019 : .034) + .008 * core :
        silk ? .130 + .105 * random() : .12 + .58 * random() ** 3
      // 纤维端部按平滑钟形衰减，消除生硬截断；广域散云不再按圆形半径硬截断，仅做极缓远距衰减。
      const shoulder = Math.exp(-((u + .26) ** 2) / .11)
      const bell = Math.max(0, 1 - u * u) ** 1.35
      const taper = inStrand ? (.14 + .70 * bell + .52 * shoulder * bell) * (.40 + .60 * armStrength) : 1
      const edge = farDiffuse
        ? Math.max(.26, 1 - .28 * Math.max(0, r - .75))
        : (() => {
            const rawEdge = Math.max(0, Math.min(1, (1.42 - r) / .54))
            return rawEdge * rawEdge * (3 - 2 * rawEdge)
          })()

      samples[cursor] = x
      samples[cursor + 1] = y
      samples[cursor + 2] = z
      samples[cursor + 3] = diameter
      samples[cursor + 4] = emission * weight * knots * taper * edge
      samples[cursor + 5] = tint
      samples[cursor + 6] = star.x
      samples[cursor + 7] = star.y
      samples[cursor + 8] = height
      const travel = cocoon ? .020 + .014 * random() : inStrand ? strand.travel : farDiffuse ? .026 + .018 * random() : fog ? (diffuse ? .024 : .040) : .036 + .022 * random()
      writeTrack(samples, cursor, r, angle, travel, (inStrand ? .6 * strand.height : 540 * Math.sin(2 * angle + 4 * r)))
      samples[cursor + 15] = inStrand ? (strand.age + .024 * u + .012 * random() + 1) % 1 : random()
      samples[cursor + 16] = inStrand ? strand.rate : (fog || cocoon || farDiffuse) ? (diffuse ? 2 : 2.5) : 4 + (i % 3)
      samples[cursor + 17] = silk ? 1.85 + (1.45 + 1.85 * random()) * armStrength : cocoon ? 1.35 + .45 * random() : farWisp ? 2.1 + 1.4 * random() : fog ? (diffuse ? 1.75 : 2.25) : 1.05
      const front = 24 * r + .9 * Math.sin(2 * angle - 7 * r)
      samples[cursor + 18] = Math.cos(front)
      samples[cursor + 19] = Math.sin(front)
      samples[cursor + 20] = dustLane
    }
  }
  return samples
}
