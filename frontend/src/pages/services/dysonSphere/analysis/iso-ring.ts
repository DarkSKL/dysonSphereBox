// =============== 等距占比环 ===============
// 职责：把整体占比的数值转换为与台地同一等距视角下的环形块面片（顶面、内外侧壁、端面）、地面刻度与画家算法顺序；
// 使用：整体占比卡片的 SVG 环形图；
// 边界：纯几何与受光系数，不读取主题、统计口径或 DOM；角度严格按数值比例分配，块间缝隙只从两端等量内缩，不挪动占比。

/** 台地正交相机的俯角（真等距）：环与左侧台地处在同一视角 */
const ELEVATION = Math.atan(1 / Math.SQRT2)
/** 水平圆投影为椭圆后的纵横比 */
export const RING_SQUASH = Math.sin(ELEVATION)
/** 竖直高度投影到画面的比例 */
export const RING_RISE = Math.cos(ELEVATION)
/** 起点：正后方（画面 12 点钟），顺时针排布，与地面刻度的 0% 对齐 */
export const RING_START = -Math.PI / 2
const TURN = Math.PI * 2
// 以外半径为单位：内半径、块高、悬停抬升、底座扫描环与环顶余量（后排抬升块及站在块上的图标）。
const INNER = 0.58
const HEIGHT = 0.2
const LIFT = 0.15
/** 底座扫描环半径，刻度落在它以内 */
export const RING_SCAN = 1.25
const REACH = 1.28
const HEADROOM = 0.92

export interface RingFrame {
  /** 地面圆心（离地高度 0）的画面坐标 */
  readonly cx: number
  readonly cy: number
  readonly outer: number
  readonly inner: number
  /** 块的基础高度，世界单位与半径相同（像素） */
  readonly height: number
  /** 悬停时追加的抬升高度 */
  readonly lift: number
}

/** 在画幅内放下环、抬升余量与底座刻度并居中；画幅过小返回 null，由调用方暂不绘制 */
export function fitRing(width: number, height: number, pad = 6): RingFrame | null {
  const span = HEADROOM + REACH * RING_SQUASH
  const outer = Math.min((width - pad * 2) / (REACH * 2), (height - pad * 2) / span)
  if (!Number.isFinite(outer) || outer < 24) return null
  return {cx: width / 2, cy: (height - span * outer) / 2 + HEADROOM * outer, outer, inner: outer * INNER, height: outer * HEIGHT, lift: outer * LIFT}
}

// =============== 投影与路径 ===============
const fixed = (value: number): string => String(Math.round(value * 100) / 100)
/** 地面坐标投影：angle 为方位角，radius 为半径，z 为离地高度 */
export const ringPoint = (frame: RingFrame, angle: number, radius: number, z = 0): [number, number] =>
  [frame.cx + Math.cos(angle) * radius, frame.cy + Math.sin(angle) * radius * RING_SQUASH - z * RING_RISE]
const at = (frame: RingFrame, angle: number, radius: number, z: number): string => {
  const [x, y] = ringPoint(frame, angle, radius, z)
  return `${fixed(x)} ${fixed(y)}`
}
/** 沿投影椭圆从 from 走到 to：每段不超过半圈，避免 SVG 椭圆弧在 180° 处翻到另一侧 */
const arc = (frame: RingFrame, radius: number, z: number, from: number, to: number): string => {
  const steps = Math.max(1, Math.ceil(Math.abs(to - from) / Math.PI - 1e-9))
  const radii = `${fixed(radius)} ${fixed(radius * RING_SQUASH)}`
  let path = ''
  for (let step = 1; step <= steps; step++) path += ` A${radii} 0 0 ${to > from ? 1 : 0} ${at(frame, from + (to - from) * step / steps, radius, z)}`
  return path
}
const topFace = (frame: RingFrame, from: number, to: number, z: number): string =>
  `M${at(frame, from, frame.outer, z)}${arc(frame, frame.outer, z, from, to)} L${at(frame, to, frame.inner, z)}${arc(frame, frame.inner, z, to, from)} Z`
const wallFace = (frame: RingFrame, radius: number, from: number, to: number, z: number): string =>
  `M${at(frame, from, radius, z)}${arc(frame, radius, z, from, to)} L${at(frame, to, radius, 0)}${arc(frame, radius, 0, to, from)} Z`
const capFace = (frame: RingFrame, angle: number, z: number): string =>
  `M${at(frame, angle, frame.inner, 0)} L${at(frame, angle, frame.outer, 0)} L${at(frame, angle, frame.outer, z)} L${at(frame, angle, frame.inner, z)} Z`

// 受光：光从画面左前上方照来（地面平面内的方向，x 向右、y 朝向观察者）；返回 0–1 的朗伯系数，由调用方换算侧面明暗。
const lambert = (nx: number, ny: number): number => Math.max(0, nx * -0.8 + ny * 0.6)

// =============== 分块与面片 ===============
export interface RingSpan {
  /** 绘制用起止方位角，已扣除块间缝隙 */
  readonly from: number
  readonly to: number
  /** 未扣缝隙的中线方位角，图标与读数以它为准 */
  readonly mid: number
}
/** 按数值分配方位角并在块间留出 gap 像素的缝隙；只有一项时画成完整圆环。小于缝隙的块保留真实弧度的 40%，仍可见、可悬停。 */
export function ringSpans(frame: RingFrame, values: readonly number[], gap = 2.6): RingSpan[] {
  const total = values.reduce((sum, value) => sum + Math.max(0, value), 0)
  if (!(total > 0)) return []
  const inset = values.length > 1 ? gap / 2 / ((frame.outer + frame.inner) / 2) : 0
  let cursor = RING_START
  return values.map(value => {
    const sweep = Math.max(0, value) / total * TURN
    const from = cursor
    cursor += sweep
    const shrink = Math.min(inset, sweep * 0.3)
    return {from: from + shrink, to: from + sweep - shrink, mid: from + sweep / 2}
  })
}

export interface RingBlock {
  /** 条目下标，原样带回每个面片，供命中与联动 */
  readonly entry: number
  readonly from: number
  readonly to: number
  /** 顶面离地高度 */
  readonly z: number
  /** 整圈单块：没有端面 */
  readonly whole: boolean
}
export interface RingFace {
  readonly d: string
  /** 0–1 的受光系数 */
  readonly light: number
}
export interface RingPiece {
  readonly key: string
  readonly entry: number
  readonly top: string
  /** 顶面外沿，用作描边高光 */
  readonly rim: string
  /** 前半圈露出外壁；后半圈透过环孔露出内壁 */
  readonly wall: RingFace & {readonly inner: boolean}
  readonly caps: readonly RingFace[]
}
// 在最左（π）与最右（0）处切开：切开后每片完全位于前半圈或后半圈，可见面固定，画家算法只需按远近排序。
const SPLITS = [0, Math.PI]

/** 生成按画家算法排好序的面片：后半圈先画，内壁透过环孔可见；前半圈后画；同半圈内由远到近。 */
export function ringPieces(frame: RingFrame, blocks: readonly RingBlock[]): RingPiece[] {
  const pieces: {piece: RingPiece; depth: number}[] = []
  for (const block of blocks) {
    if (!(block.to > block.from)) continue
    const cuts = [block.from, ...SPLITS.filter(angle => angle > block.from + 1e-6 && angle < block.to - 1e-6), block.to]
    for (let part = 1; part < cuts.length; part++) {
      const from = cuts[part - 1]
      const to = cuts[part]
      const mid = (from + to) / 2
      const front = Math.sin(mid) > 0
      // 端面只在块的真实首尾、且朝向观察者时可见；切分点处的截面属于块内部。
      const caps: RingFace[] = []
      if (!block.whole && part === 1 && Math.cos(from) < 0) caps.push({d: capFace(frame, from, block.z), light: lambert(Math.sin(from), -Math.cos(from))})
      if (!block.whole && part === cuts.length - 1 && Math.cos(to) > 0) caps.push({d: capFace(frame, to, block.z), light: lambert(-Math.sin(to), Math.cos(to))})
      pieces.push({depth: Math.sin(mid), piece: {
        key: `${block.entry}:${part}`, entry: block.entry, caps,
        top: topFace(frame, from, to, block.z),
        rim: `M${at(frame, from, frame.outer, block.z)}${arc(frame, frame.outer, block.z, from, to)}`,
        wall: front
          ? {d: wallFace(frame, frame.outer, from, to, block.z), light: lambert(Math.cos(mid), Math.sin(mid)), inner: false}
          : {d: wallFace(frame, frame.inner, from, to, block.z), light: lambert(-Math.cos(mid), -Math.sin(mid)), inner: true},
      }})
    }
  }
  return pieces.sort((a, b) => a.depth - b.depth).map(item => item.piece)
}

// =============== 测量与底座 ===============
/** 投影后沿半径 radius 的弧长（像素），用于判断块的顶面放不放得下图标 */
export function ringArcLength(frame: RingFrame, radius: number, from: number, to: number): number {
  const steps = Math.max(2, Math.ceil(Math.abs(to - from) / (Math.PI / 24)))
  let [px, py] = ringPoint(frame, from, radius)
  let length = 0
  for (let step = 1; step <= steps; step++) {
    const [x, y] = ringPoint(frame, from + (to - from) * step / steps, radius)
    length += Math.hypot(x - px, y - py)
    px = x
    py = y
  }
  return length
}
/** 地面刻度：count 等分，每 majorEvery 格一根长刻度；0% 落在环的起点，刻度与块角度一一对应，可直接估读占比 */
export function ringTicks(frame: RingFrame, count = 20, majorEvery = 5): {minor: string; major: string} {
  let minor = ''
  let major = ''
  for (let index = 0; index < count; index++) {
    const angle = RING_START + index / count * TURN
    const isMajor = index % majorEvery === 0
    const [x1, y1] = ringPoint(frame, angle, frame.outer * (isMajor ? 1.08 : 1.12))
    const [x2, y2] = ringPoint(frame, angle, frame.outer * (isMajor ? 1.21 : 1.17))
    const segment = `M${fixed(x1)} ${fixed(y1)}L${fixed(x2)} ${fixed(y2)}`
    if (isMajor) major += segment
    else minor += segment
  }
  return {minor, major}
}
