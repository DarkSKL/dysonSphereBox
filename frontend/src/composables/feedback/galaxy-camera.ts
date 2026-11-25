import type {Bounds, Coordinates, DisplayStar, GalaxyPosition} from '../../types/feedback'

export const GALAXY_EDGE = 100000
export const GALAXY_HEIGHT = 10000
export const DISK_RADIUS = 96000
export const MAX_ZOOM = 64
const MAX_INCLINATION = 80 * Math.PI / 180

export interface ViewSize {width: number; height: number}
export interface Camera {zoom: number; yaw: number; inclination: number}
export interface ProjectedStar extends Coordinates {point: DisplayStar; depth: number}
export const HOME_CAMERA: Camera = {zoom: 1, yaw: -.55, inclination: .51}

// =============== 正交观测相机 ===============
// 世界原点（银心）始终投影到画布中心。相机只有朝向与倍率，星核、光尘和查询共用此投影。
export function galaxyScale(size: ViewSize, zoom: number): number {
  // 用包含整个厚星盘的球定全景尺度，回正或转到侧面都能把外缘留在画布内。
  const fitDiameter = Math.max(1, Math.min(size.width, size.height) - 24)
  return fitDiameter * zoom / (2 * Math.hypot(DISK_RADIUS, GALAXY_HEIGHT))
}

export function cameraFrame(camera: Camera, size: ViewSize) {
  const c = Math.cos(camera.yaw), s = Math.sin(camera.yaw)
  const tilt = Math.cos(camera.inclination), lift = Math.sin(camera.inclination)
  return {
    right: [c, -s, 0] as const,
    down: [s * tilt, c * tilt, -lift] as const,
    near: [s * lift, c * lift, tilt] as const,
    scale: galaxyScale(size, camera.zoom),
  }
}

type Frame = ReturnType<typeof cameraFrame>

export function project(point: GalaxyPosition, camera: Camera, size: ViewSize, frame: Frame = cameraFrame(camera, size)) {
  const {x, y, z = 0} = point
  return {
    x: size.width / 2 + frame.scale * (x * frame.right[0] + y * frame.right[1]),
    y: size.height / 2 + frame.scale * (x * frame.down[0] + y * frame.down[1] + z * frame.down[2]),
    depth: x * frame.near[0] + y * frame.near[1] + z * frame.near[2],
  }
}

/**
 * 屏幕点附近的星，按距离由近到远；距离相同时前面的星（depth 大）在前，再按编号稳定排序。
 * 用于点击命中：只有一颗明显最近的星时直接打开，两颗以上难以分辨时交给「附近的星」列表选择。
 */
export function starsNear(stars: readonly ProjectedStar[], screen: Coordinates, radius: number): {star: ProjectedStar; distance: number}[] {
  const near: {star: ProjectedStar; distance: number}[] = []
  for (const star of stars) {
    const distance = Math.hypot(star.x - screen.x, star.y - screen.y)
    if (distance <= radius) near.push({star, distance})
  }
  return near.sort((a, b) => (a.distance - b.distance) || (b.star.depth - a.star.depth) ||
    (a.star.point.starId < b.star.point.starId ? -1 : a.star.point.starId > b.star.point.starId ? 1 : 0))
}

/** 两颗最近的星相差不足该像素数时，视为点击落在拥挤处，不替用户猜测目标。 */
export const AMBIGUOUS_GAP = 4

/**
 * 点击命中：只有一颗明显最近的星时直接打开；最近两颗相差不足 AMBIGUOUS_GAP 像素时返回命中半径内的全部星，
 * 由页面用分页列表让用户选择。不截断数量：半径内第 13 颗及之后的星同样可选。
 */
export function resolveClick(stars: readonly ProjectedStar[], screen: Coordinates, radius: number):
  {open?: ProjectedStar; crowd?: ProjectedStar[]} {
  const near = starsNear(stars, screen, radius)
  if (!near.length) return {}
  if (near.length === 1 || near[1].distance - near[0].distance >= AMBIGUOUS_GAP) return {open: near[0].star}
  return {crowd: near.map(item => item.star)}
}

/**
 * 按当前快照重新校验候选：保留候选 ID 的原有顺序，丢弃已不在当前可见星中的 ID，位置取最新投影。
 * 轮询替换代表星或移除星点后，旧选项不会继续显示；全部失效时返回空数组，由调用方收起列表。
 */
export function revalidateCandidates(ids: readonly string[], stars: readonly ProjectedStar[]): ProjectedStar[] {
  const current = new Map(stars.map(star => [star.point.starId, star]))
  const kept: ProjectedStar[] = []
  for (const id of ids) {
    const star = current.get(id)
    if (star) kept.push(star)
  }
  return kept
}

export function projectVisibleStars(points: readonly DisplayStar[], camera: Camera, size: ViewSize): ProjectedStar[] {
  const frame = cameraFrame(camera, size)
  const visible: ProjectedStar[] = []
  for (const point of points) {
    const pos = project(point, camera, size, frame)
    if (pos.x >= 0 && pos.x < size.width && pos.y >= 0 && pos.y < size.height) visible.push({...pos, point})
  }
  return visible
}

/** 世界坐标当前是否投影在画布内。与 projectVisibleStars 使用同一投影与边界，供「所读星是否在画面内」判定使用。 */
export function isInView(point: GalaxyPosition, camera: Camera, size: ViewSize): boolean {
  if (!size.width || !size.height) return false
  const pos = project(point, camera, size)
  return pos.x >= 0 && pos.x < size.width && pos.y >= 0 && pos.y < size.height
}

/** 从个人列表或发布回执查看星点时，只在必要时拉远，保留当前朝向与银心。 */
export function includeStar(camera: Camera, point: GalaxyPosition, size: ViewSize): Camera {
  if (!size.width || !size.height || camera.zoom <= 1) return camera
  const pos = project(point, camera, size)
  const halfW = size.width / 2, halfH = size.height / 2
  const ratio = Math.max(
    Math.abs(pos.x - halfW) / Math.max(1, halfW - 18),
    Math.abs(pos.y - halfH) / Math.max(1, halfH - 18),
  )
  return ratio > 1 ? {...camera, zoom: Math.max(1, camera.zoom / ratio)} : camera
}

/** 用三维距离留出绕中心转动的空间。即便转到侧面，最近的小组也能留在画布内。 */
export function usefulZoomLimit(points: readonly DisplayStar[], size: ViewSize): number {
  if (!points.length || !size.width || !size.height) return 1
  const nearest = points.map(p => ({radius: Math.hypot(p.x, p.y, p.z ?? 0), count: p.count ?? 1}))
    .sort((a, b) => a.radius - b.radius)
  let needed = points.length >= 3 ? 3 : 1
  let radius = 0
  for (const star of nearest) {
    radius = star.radius
    needed -= Math.max(1, star.count)
    if (needed <= 0) break
  }
  const half = Math.min(size.width, size.height) / 2
  const space = half - Math.min(48, half * .15)
  return Math.max(1, Math.min(MAX_ZOOM, space / (galaxyScale(size, 1) * Math.max(1, radius))))
}

export function viewportBounds(camera: Camera, size: ViewSize): Bounds | null {
  if (!size.width || !size.height || camera.zoom === 1) return null
  const scale = galaxyScale(size, camera.zoom)
  const c = Math.cos(camera.yaw), s = Math.sin(camera.yaw)
  const halfU = size.width / (2 * scale)
  const halfV = size.height / (2 * scale * Math.cos(camera.inclination)) + GALAXY_HEIGHT * Math.abs(Math.tan(camera.inclination))
  const hw = Math.min(GALAXY_EDGE, Math.ceil(Math.abs(c) * halfU + Math.abs(s) * halfV))
  const hh = Math.min(GALAXY_EDGE, Math.ceil(Math.abs(s) * halfU + Math.abs(c) * halfV))
  return hw === GALAXY_EDGE && hh === GALAXY_EDGE ? null : {minX: -hw, minY: -hh, maxX: hw, maxY: hh}
}

/** 聚合图的查询边界每两倍分四档并向外取整，小幅滚动或转动可复用同一条件快照。 */
export function queryBounds(camera: Camera, size: ViewSize): Bounds | null {
  const view = viewportBounds(camera, size)
  if (!view) return null
  const expand = (extent: number) => Math.min(GALAXY_EDGE, Math.ceil(2 ** (Math.ceil(4 * Math.log2(Math.max(1, extent))) / 4)))
  const hw = expand(view.maxX), hh = expand(view.maxY)
  return hw === GALAXY_EDGE && hh === GALAXY_EDGE ? null : {minX: -hw, minY: -hh, maxX: hw, maxY: hh}
}

export function boundedCamera(camera: Camera): Camera {
  return {
    inclination: Math.max(-MAX_INCLINATION, Math.min(MAX_INCLINATION, camera.inclination)),
    yaw: camera.yaw % (2 * Math.PI),
    zoom: Math.max(1, Math.min(MAX_ZOOM, camera.zoom)),
  }
}
