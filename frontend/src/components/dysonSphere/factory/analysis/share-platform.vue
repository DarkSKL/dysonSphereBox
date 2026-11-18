<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, shallowRef, watch} from 'vue'
import * as THREE from 'three'
import {LocateFixed, RotateCcw, RotateCw, ScanEye} from '@lucide/vue'
import ItemIcon from '../item-icon.vue'
import {analysisReducedMotion, rampAt, shareEntryTone, type AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {ensureFactoryIcons, factoryIconCanvas, factoryIconImageReady, factoryIconsReady} from '../../../../pages/services/dysonSphere/factory-icons'
import {SHARE_METRICS, allocateTiles, categoryLabel, formatInteger, formatPower, formatShare, formatValue, layoutPlatform, regionAnchor, shareMetricsOf, shareValueOf, type ShareEntry, type ShareMetric} from '../../../../pages/services/dysonSphere/factory-analysis'

// =============== 整体占比台地（three.js） ===============
// 说明：纪念碑谷式 10×10 浮空平台代表整个工厂，一根立柱 = 当前口径的 1%（最大余数法分配）；每种建筑占一片连续区域，
//       颜色深浅随占比升高，整个场地共用阶梯高度；建筑和电力展示设施插画，生产展示各产出物品插画。
//       空间口径的 100 格即蓝图包络：建筑占地与运输线路升起为立柱，空地只铺一层地砖，看得出包络里还剩多少地方。
// 交互：悬停区域时阶梯平滑升起至平齐、移开恢复，其余区域淡化并联动右侧占比环；点击固定后镜头聚焦对应设施或物品区域；
//       拖动旋转（松手吸附等距方位）、滚轮缩放、双击复位，工具栏可旋转 90°、复位与开关自动聚焦。
// 边界：只表达整体构成，不表示建筑在蓝图中的实际位置；只在动画进行时逐帧渲染，容器不可见时不绘制；卸载时释放全部 GPU 资源。
const props = defineProps<{entries: readonly ShareEntry[]; metric: ShareMetric; palette: AnalysisPalette; hovered: number | null; pinned: number | null}>()
const emit = defineEmits<{(event: 'hover', index: number | null): void; (event: 'pick', index: number | null): void}>()
ensureFactoryIcons()

const COLUMNS = 10
const ROWS = 10
const FOOTPRINT = 0.92
const BASE_HEIGHT = 0.3
const RISE_HEIGHT = 1.6
const HOVER_LIFT = 0.8
const TERRACE_STEPS = 10
const TOP_HEIGHT = BASE_HEIGHT + RISE_HEIGHT
/** 空地地砖厚度：只比平台面略高，与立柱拉开明显落差 */
const FLAT_HEIGHT = 0.12
const SLAB_DEPTH = 0.7
const SLAB_HALF = 5.3
const ELEVATION = Math.atan(1 / Math.SQRT2)
/** 默认从西南方俯视：大份额区域位于画面后方（东北角），与平台排布方向一致 */
const HOME_THETA = -Math.PI / 4
const QUARTER = Math.PI / 2
const BASE_HALF = 7
const reducedMotion = analysisReducedMotion()

// =============== 立柱分配 ===============
// 说明：其余口径按份额从大到小铺设；空间口径按「建筑 → 线路 → 空地」固定顺序，占用区域在后排连成一片，空地留在前排。
const values = computed(() => props.entries.map(entry => shareValueOf(entry, props.metric)))
const order = computed(() => {
  const indices = values.value.map((_value, index) => index).filter(index => values.value[index] > 0)
  return props.metric === 'space' ? indices : indices.sort((a, b) => values.value[b] - values.value[a] || a - b)
})
const tileCounts = computed(() => allocateTiles(values.value))
const placed = computed(() => layoutPlatform(tileCounts.value, order.value, COLUMNS, ROWS))
const maxShare = computed(() => props.entries.reduce((max, entry) => Math.max(max, entry.shares[props.metric]), 0))
const shareRatio = (entry: number): number => (maxShare.value > 0 ? (props.entries[entry]?.shares[props.metric] ?? 0) / maxShare.value : 0)
/** 空间口径中的空地：只铺地砖，不立插画 */
const isEmptyLand = (entry: number): boolean => {
  const item = props.entries[entry]
  return item?.kind === 'space' && item.role === 'empty'
}
const worldX = (column: number): number => column - (COLUMNS - 1) / 2
const worldZ = (row: number): number => (ROWS - 1) / 2 - row
const spriteSizeOf = (entry: number): number => 1.25 + 1.35 * Math.sqrt((tileCounts.value[entry] ?? 0) / 100)
// 抬升与淡化跟随悬停（悬停优先），没有悬停时保持固定项抬起；镜头聚焦只跟随固定项与环图停留。
const highlight = computed(() => props.hovered ?? props.pinned)

// =============== 场景对象 ===============
// 说明：立柱共用一个实例网格，几何体按面写入明暗系数（顶面最亮、两侧依受光方向压暗），实例颜色给出热力色，不依赖光照即得纪念碑谷的平涂三面。
const shadedBox = (shades: readonly number[]): THREE.BoxGeometry => {
  const geometry = new THREE.BoxGeometry(1, 1, 1)
  geometry.translate(0, 0.5, 0)
  const colors = new Float32Array(geometry.attributes.position.count * 3)
  for (let face = 0; face < 6; face++) for (let vertex = 0; vertex < 4; vertex++) colors.fill(shades[face], (face * 4 + vertex) * 3, (face * 4 + vertex) * 3 + 3)
  geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3))
  return geometry
}
// 面顺序：+X（东）、-X（西）、+Y（顶）、-Y（底）、+Z（南）、-Z（北）
const tileGeometry = shadedBox([0.45, 0.6, 1, 0.2, 0.36, 0.3])
const tileMaterial = new THREE.MeshBasicMaterial({vertexColors: true})
const tileMesh = new THREE.InstancedMesh(tileGeometry, tileMaterial, COLUMNS * ROWS)
tileMesh.count = 0
const slabGeometry = shadedBox([0.62, 0.8, 1, 0.3, 0.52, 0.45])
const slabMaterial = new THREE.MeshBasicMaterial({vertexColors: true})
const slab = new THREE.Mesh(slabGeometry, slabMaterial)
slab.scale.set(SLAB_HALF * 2, SLAB_DEPTH, SLAB_HALF * 2)
slab.position.y = -SLAB_DEPTH - 0.001
const rimGeometry = new THREE.BufferGeometry().setFromPoints([[-1, -1], [1, -1], [1, 1], [-1, 1]].map(([x, z]) => new THREE.Vector3(x * SLAB_HALF, 0.002, z * SLAB_HALF)))
const rimMaterial = new THREE.LineBasicMaterial({transparent: true, opacity: 0.85})
const rim = new THREE.LineLoop(rimGeometry, rimMaterial)
const shadowGeometry = new THREE.CircleGeometry(0.42, 28).rotateX(-Math.PI / 2)
const scene = new THREE.Scene()
scene.add(slab, rim, tileMesh)
const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 400)
const probe = new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 400)

interface TileState { entry: number; column: number; row: number; terraceDrop: number; delay: number; grow: number; flat: boolean }
interface RegionVisual { entry: number; column: number; row: number; size: number; sprite: THREE.Sprite | null; texture: THREE.Texture | null; shadow: THREE.Mesh; pop: number; delay: number }
interface RegionMotion { lift: number; dim: number; flatten: number }
let tileStates: TileState[] = []
let visuals: RegionVisual[] = []
const motion = new Map<number, RegionMotion>()
let buildStart = 0
const baseColors = new Map<number, THREE.Color>()
const dimColor = new THREE.Color()
const scratchColor = new THREE.Color()
const matrix = new THREE.Matrix4()
const position = new THREE.Vector3()
const scale = new THREE.Vector3()
const identity = new THREE.Quaternion()

// =============== 主题配色 ===============
// 说明：立柱颜色与右侧占比环同源（shareEntryTone），同一条目在两处颜色一致；空地取口径明度梯最浅的一级，像未建设的地面。
const applyPalette = (): void => {
  const p = props.palette
  baseColors.clear()
  for (const entry of order.value) {
    const item = props.entries[entry]
    if (item) baseColors.set(entry, new THREE.Color(shareEntryTone(p, props.metric, item, shareRatio(entry)) ?? rampAt(p.shareRamps[props.metric], p.dark ? 0.16 : 0.06)))
  }
  dimColor.set(p.tileB)
  slabMaterial.color.set(p.cliffLeft)
  rimMaterial.color.set(p.dark ? '#4cc2ff' : '#3f97dc')
  rimMaterial.opacity = p.dark ? 0.6 : 0.7
  for (const visual of visuals) (visual.shadow.material as THREE.MeshBasicMaterial).opacity = p.dark ? 0.32 : 0.18
}

// =============== 建造与回收 ===============
// 说明：口径或数据变化时重建立柱与插画：立柱从后往前依次升起，插画随后弹出，呼应平面预览的平台升起动画。
const disposeVisuals = (): void => {
  for (const visual of visuals) {
    if (visual.sprite) { scene.remove(visual.sprite); (visual.sprite.material as THREE.SpriteMaterial).dispose() }
    visual.texture?.dispose()
    scene.remove(visual.shadow)
    ;(visual.shadow.material as THREE.MeshBasicMaterial).dispose()
  }
  visuals = []
}
const buildVisuals = (): void => {
  disposeVisuals()
  order.value.forEach((entry, rank) => {
    const anchor = regionAnchor(placed.value, entry)
    const item = props.entries[entry]
    if (!anchor || !item || isEmptyLand(entry)) return
    const canvas = factoryIconCanvas(item.icon, 64)
    const texture = canvas ? new THREE.CanvasTexture(canvas) : null
    if (texture) { texture.colorSpace = THREE.SRGBColorSpace; texture.anisotropy = 4 }
    const sprite = texture ? new THREE.Sprite(new THREE.SpriteMaterial({map: texture, transparent: true, alphaTest: 0.08, depthWrite: false})) : null
    if (sprite) { sprite.center.set(0.5, 0.05); sprite.userData.entry = entry; sprite.renderOrder = 2; scene.add(sprite) }
    const shadow = new THREE.Mesh(shadowGeometry, new THREE.MeshBasicMaterial({color: 0x000000, transparent: true, opacity: 0.25, depthWrite: false}))
    shadow.renderOrder = 1
    scene.add(shadow)
    visuals.push({entry, column: anchor.column, row: anchor.row, size: spriteSizeOf(entry), sprite, texture, shadow, pop: reducedMotion ? 1 : 0, delay: 360 + rank * 55})
  })
}
const rebuild = (): void => {
  const depthRank = [...placed.value].map((tile, index) => ({index, depth: tile.column + tile.row})).sort((a, b) => b.depth - a.depth)
  const delays = new Map(depthRank.map((item, rank) => [item.index, rank * 6]))
  // 全场按对角深度划分十级台阶，跨建筑区域连续延伸；占比继续由方块数量和颜色表达。
  tileStates = placed.value.map((tile, index) => {
    const progress = (tile.column + tile.row) / (COLUMNS + ROWS - 2)
    const tier = Math.min(TERRACE_STEPS - 1, Math.floor(progress * TERRACE_STEPS))
    const terraceDrop = RISE_HEIGHT * (1 - tier / (TERRACE_STEPS - 1))
    return {entry: tile.entry, column: tile.column, row: tile.row, terraceDrop, delay: delays.get(index) ?? 0, grow: reducedMotion ? 1 : 0, flat: isEmptyLand(tile.entry)}
  })
  for (const key of [...motion.keys()]) if (!order.value.includes(key)) motion.delete(key)
  for (const entry of order.value) if (!motion.has(entry)) motion.set(entry, {lift: 0, dim: 0, flatten: 0})
  tileMesh.count = tileStates.length
  buildVisuals()
  applyPalette()
  buildStart = performance.now()
  focusEntry = undefined
  focusOn(props.pinned, false)
}

// =============== 相机 ===============
// 说明：正交等距相机；视角由方位角、缩放与观察点描述，切换时按缓动插值，方位角走最短路径。
interface View { theta: number; zoom: number; x: number; y: number; z: number }
const view: View = {theta: HOME_THETA, zoom: 1, x: 0, y: 0, z: 0}
let tween: {from: View; to: View; start: number; duration: number} | null = null
let focusEntry: number | null | undefined
let width = 0
let height = 0
const aspect = (): number => (height > 0 ? width / height : 1)
const placeCamera = (target: THREE.OrthographicCamera, theta: number, x: number, y: number, z: number, zoom: number): void => {
  const half = BASE_HALF
  target.left = -half * aspect()
  target.right = half * aspect()
  target.top = half
  target.bottom = -half
  target.zoom = zoom
  target.position.set(x + 60 * Math.cos(ELEVATION) * Math.sin(theta), y + 60 * Math.sin(ELEVATION), z + 60 * Math.cos(ELEVATION) * Math.cos(theta))
  target.up.set(0, 1, 0)
  target.lookAt(x, y, z)
  target.updateProjectionMatrix()
  target.updateMatrixWorld(true)
}
const applyView = (): void => placeCamera(camera, view.theta, view.x, view.y, view.z, view.zoom)
const viewPoint = new THREE.Vector3()
const axisRight = new THREE.Vector3()
const axisUp = new THREE.Vector3()
// 取景求解：把一组世界点投到该方位的相机平面，取包围盒中心作为观察点、按留白算缩放。
const solveView = (points: readonly THREE.Vector3[], theta: number, margin: number): View => {
  placeCamera(probe, theta, 0, 0, 0, 1)
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  for (const point of points) {
    viewPoint.copy(point).applyMatrix4(probe.matrixWorldInverse)
    minX = Math.min(minX, viewPoint.x)
    maxX = Math.max(maxX, viewPoint.x)
    minY = Math.min(minY, viewPoint.y)
    maxY = Math.max(maxY, viewPoint.y)
  }
  const zoom = Math.min((2 * BASE_HALF * aspect()) / (Math.max(0.5, maxX - minX) * margin), (2 * BASE_HALF) / (Math.max(0.5, maxY - minY) * margin))
  axisRight.setFromMatrixColumn(probe.matrixWorld, 0)
  axisUp.setFromMatrixColumn(probe.matrixWorld, 1)
  const center = new THREE.Vector3().addScaledVector(axisRight, (minX + maxX) / 2).addScaledVector(axisUp, (minY + maxY) / 2)
  return {theta, zoom, x: center.x, y: center.y, z: center.z}
}
const REGION_TOP = TOP_HEIGHT + HOVER_LIFT
/** 插画顶端的世界坐标：插画始终正对相机，顶端沿相机上方向而不是竖直方向延伸 */
const spriteTop = (visual: RegionVisual, theta: number, extra: number): THREE.Vector3 => {
  const reach = visual.size + extra
  return new THREE.Vector3(
    worldX(visual.column) - Math.sin(ELEVATION) * Math.sin(theta) * reach,
    REGION_TOP + Math.cos(ELEVATION) * reach,
    worldZ(visual.row) - Math.sin(ELEVATION) * Math.cos(theta) * reach,
  )
}
const overviewPoints = (theta: number): THREE.Vector3[] => {
  const points: THREE.Vector3[] = []
  for (const x of [-SLAB_HALF, SLAB_HALF]) for (const z of [-SLAB_HALF, SLAB_HALF]) points.push(new THREE.Vector3(x, -SLAB_DEPTH, z), new THREE.Vector3(x, BASE_HEIGHT + RISE_HEIGHT, z))
  for (const visual of visuals) points.push(spriteTop(visual, theta, 0.9))
  return points
}
const regionPoints = (entry: number, theta: number): THREE.Vector3[] => {
  const points: THREE.Vector3[] = []
  for (const tile of tileStates) {
    if (tile.entry !== entry) continue
    for (const dx of [-0.5, 0.5]) for (const dz of [-0.5, 0.5]) points.push(new THREE.Vector3(worldX(tile.column) + dx, 0, worldZ(tile.row) + dz), new THREE.Vector3(worldX(tile.column) + dx, REGION_TOP, worldZ(tile.row) + dz))
  }
  const visual = visuals.find(item => item.entry === entry)
  if (visual) points.push(spriteTop(visual, theta, 0.7))
  return points
}
const snapTheta = (theta: number): number => HOME_THETA + Math.round((theta - HOME_THETA) / QUARTER) * QUARTER
// 自动旋转：选能把该建筑群放到画面最前方的等距方位（相机与区域重心同侧）；区域位于中央时保持当前方位。
const bestTheta = (entry: number): number => {
  const own = tileStates.filter(tile => tile.entry === entry)
  if (!own.length) return snapTheta(view.theta)
  const cx = own.reduce((sum, tile) => sum + worldX(tile.column), 0) / own.length
  const cz = own.reduce((sum, tile) => sum + worldZ(tile.row), 0) / own.length
  if (Math.hypot(cx, cz) < 1.5) return snapTheta(view.theta)
  const base = snapTheta(view.theta)
  let best = base
  let score = -Infinity
  for (let k = -1; k <= 2; k++) {
    const theta = base + k * QUARTER
    const value = cx * Math.sin(theta) + cz * Math.cos(theta)
    if (value > score + 1e-6) { score = value; best = theta }
  }
  return best
}
const overviewView = (theta: number): View => solveView(overviewPoints(theta), theta, 1.06)
const startTween = (to: View, duration: number): void => {
  if (reducedMotion || duration <= 0) { Object.assign(view, to); tween = null; kick(); return }
  tween = {from: {...view}, to, start: performance.now(), duration}
  kick()
}
// 聚焦：null 为全景；建筑群聚焦时自动旋转到最前方并放大，放大倍率限制在全景的 1.25–3.2 倍之间。
const focusOn = (entry: number | null, animate = true): void => {
  if (!width || !height) { focusEntry = undefined; return }
  if (entry === focusEntry) return
  focusEntry = entry
  const overview = overviewView(entry === null ? snapTheta(view.theta) : bestTheta(entry))
  if (entry === null || !tileStates.some(tile => tile.entry === entry)) { startTween(overview, animate ? 650 : 0); return }
  const target = solveView(regionPoints(entry, overview.theta), overview.theta, 1.85)
  target.zoom = Math.min(overview.zoom * 3.2, Math.max(overview.zoom * 1.25, target.zoom))
  startTween(target, animate ? 750 : 0)
}
const shortestAngle = (from: number, to: number): number => {
  const delta = ((to - from) % (Math.PI * 2) + Math.PI * 3) % (Math.PI * 2) - Math.PI
  return from + delta
}
const easeInOut = (t: number): number => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2)
const stepView = (now: number): boolean => {
  if (!tween) return false
  const t = Math.min(1, (now - tween.start) / tween.duration)
  const e = easeInOut(t)
  const {from, to} = tween
  view.theta = from.theta + (shortestAngle(from.theta, to.theta) - from.theta) * e
  view.zoom = from.zoom + (to.zoom - from.zoom) * e
  view.x = from.x + (to.x - from.x) * e
  view.y = from.y + (to.y - from.y) * e
  view.z = from.z + (to.z - from.z) * e
  if (t >= 1) { view.theta = to.theta; tween = null }
  return tween !== null
}

// =============== 逐帧更新 ===============
// 说明：默认保留每根立柱的阶梯落差，仅悬停时消除落差；固定项保持抬升与聚焦，但移开指针仍恢复阶梯。空地地砖不参与阶梯，悬停时只微微浮起。
const easeOut = (t: number): number => 1 - (1 - Math.min(1, Math.max(0, t))) ** 3
const tileHeight = (tile: TileState): number => {
  const state = motion.get(tile.entry)
  if (tile.flat) return FLAT_HEIGHT * tile.grow + (state?.lift ?? 0) * 0.35
  return (TOP_HEIGHT - tile.terraceDrop * (1 - (state?.flatten ?? 0))) * tile.grow + (state?.lift ?? 0)
}
const stepScene = (now: number, dt: number): boolean => {
  let busy = false
  const smoothing = reducedMotion ? 1 : 1 - Math.exp(-dt * 12)
  for (const [entry, state] of motion) {
    const lift = highlight.value === entry ? HOVER_LIFT : 0
    const dim = highlight.value !== null && highlight.value !== entry ? 1 : 0
    const flatten = props.hovered === entry ? 1 : 0
    state.lift += (lift - state.lift) * smoothing
    state.dim += (dim - state.dim) * smoothing
    state.flatten += (flatten - state.flatten) * smoothing
    if (Math.abs(lift - state.lift) > 0.002 || Math.abs(dim - state.dim) > 0.004 || Math.abs(flatten - state.flatten) > 0.002) busy = true
    else { state.lift = lift; state.dim = dim; state.flatten = flatten }
  }
  tileStates.forEach((tile, index) => {
    if (tile.grow < 1) {
      tile.grow = easeOut((now - buildStart - tile.delay) / 520)
      busy = true
    }
    position.set(worldX(tile.column), 0, worldZ(tile.row))
    scale.set(FOOTPRINT, Math.max(0.001, tileHeight(tile)), FOOTPRINT)
    tileMesh.setMatrixAt(index, matrix.compose(position, identity, scale))
    const base = baseColors.get(tile.entry)
    if (base) tileMesh.setColorAt(index, scratchColor.copy(base).lerp(dimColor, (motion.get(tile.entry)?.dim ?? 0) * 0.62))
  })
  tileMesh.instanceMatrix.needsUpdate = true
  if (tileMesh.instanceColor) tileMesh.instanceColor.needsUpdate = true
  tileMesh.boundingSphere = null
  for (const visual of visuals) {
    if (visual.pop < 1) {
      visual.pop = easeOut((now - buildStart - visual.delay) / 380)
      busy = true
    }
    const anchor = tileStates.find(tile => tile.column === visual.column && tile.row === visual.row)
    const top = anchor ? tileHeight(anchor) : TOP_HEIGHT
    const dim = motion.get(visual.entry)?.dim ?? 0
    if (visual.sprite) {
      visual.sprite.position.set(worldX(visual.column), top + 0.01, worldZ(visual.row))
      visual.sprite.scale.set(visual.size * visual.pop, visual.size * visual.pop, 1)
      ;(visual.sprite.material as THREE.SpriteMaterial).opacity = 1 - dim * 0.6
    }
    visual.shadow.position.set(worldX(visual.column), top + 0.004, worldZ(visual.row))
    visual.shadow.scale.setScalar(Math.max(0.001, visual.size * 0.8 * visual.pop))
  }
  return busy
}

// =============== 渲染循环 ===============
const canvas = ref<HTMLCanvasElement | null>(null)
const host = ref<HTMLElement | null>(null)
const supported = ref(true)
let renderer: THREE.WebGLRenderer | null = null
const renderSize = new THREE.Vector2()
let frame = 0
let lastTime = 0
let disposed = false
const kick = (): void => { if (!frame && renderer && !disposed) frame = requestAnimationFrame(tick) }
const tick = (now: number): void => {
  frame = 0
  if (!renderer || disposed || !width || !height) { lastTime = 0; return }
  const dt = lastTime ? Math.min(0.05, (now - lastTime) / 1000) : 1 / 60
  lastTime = now
  const viewBusy = stepView(now)
  const sceneBusy = stepScene(now, dt)
  applyView()
  // 改写 WebGL 画布尺寸会清空绘图缓冲区，必须在同一帧立即绘制，避免连续缩放时合成出透明帧。
  renderer.getSize(renderSize)
  if (renderSize.x !== width || renderSize.y !== height) renderer.setSize(width, height, false)
  renderer.render(scene, camera)
  if (viewBusy || sceneBusy) kick()
  else lastTime = 0
}

// =============== 指针交互 ===============
const raycaster = new THREE.Raycaster()
const pointer = new THREE.Vector2()
const tip = shallowRef<{entry: number; x: number; y: number} | null>(null)
let pointerEntry: number | null = null
let drag: {x: number; y: number; theta: number; moved: boolean} | null = null
const pickAt = (clientX: number, clientY: number): number | null => {
  const element = canvas.value
  if (!element || !renderer) return null
  const rect = element.getBoundingClientRect()
  pointer.set((clientX - rect.left) / rect.width * 2 - 1, -((clientY - rect.top) / rect.height) * 2 + 1)
  raycaster.setFromCamera(pointer, camera)
  const targets: THREE.Object3D[] = [tileMesh, ...visuals.flatMap(visual => (visual.sprite ? [visual.sprite] : []))]
  for (const hit of raycaster.intersectObjects(targets, false)) {
    if (hit.object === tileMesh && hit.instanceId !== undefined) return tileStates[hit.instanceId]?.entry ?? null
    const entry = hit.object.userData.entry
    if (typeof entry === 'number') return entry
  }
  return null
}
const setPointerEntry = (entry: number | null, clientX = 0, clientY = 0): void => {
  const rect = host.value?.getBoundingClientRect()
  tip.value = entry === null || !rect ? null : {entry, x: clientX - rect.left, y: clientY - rect.top}
  if (entry === pointerEntry) return
  pointerEntry = entry
  emit('hover', entry)
}
const onPointerDown = (event: PointerEvent): void => {
  if (event.button !== 0) return
  drag = {x: event.clientX, y: event.clientY, theta: view.theta, moved: false}
  canvas.value?.setPointerCapture(event.pointerId)
}
const onPointerMove = (event: PointerEvent): void => {
  if (drag) {
    if (Math.abs(event.clientX - drag.x) + Math.abs(event.clientY - drag.y) > 4) drag.moved = true
    if (drag.moved) {
      tween = null
      view.theta = drag.theta - (event.clientX - drag.x) * 0.009
      tip.value = null
      kick()
      return
    }
  }
  setPointerEntry(pickAt(event.clientX, event.clientY), event.clientX, event.clientY)
}
const onPointerUp = (event: PointerEvent): void => {
  if (!drag) return
  const moved = drag.moved
  drag = null
  canvas.value?.releasePointerCapture(event.pointerId)
  if (moved) {
    // 松手吸附到最近的等距方位；聚焦中的建筑群按新方位重新取景。
    const theta = snapTheta(view.theta)
    const entry = focusEntry ?? null
    startTween(entry === null ? overviewView(theta) : {...solveView(regionPoints(entry, theta), theta, 1.85), zoom: view.zoom}, 320)
    return
  }
  emit('pick', pickAt(event.clientX, event.clientY))
}
const onPointerLeave = (): void => { if (!drag) setPointerEntry(null) }
const onWheel = (event: WheelEvent): void => {
  if (!width || !height) return
  const base = overviewView(snapTheta(view.theta)).zoom
  tween = null
  view.zoom = Math.min(base * 5, Math.max(base * 0.7, view.zoom * Math.exp(-event.deltaY * 0.0015)))
  kick()
}

// =============== 工具栏 ===============
const autoFocus = ref(true)
// 旋转 90°：全景时重新取景；聚焦建筑群时保持当前放大倍率，只按新方位重新对准该建筑群。
const rotate = (step: number): void => {
  const theta = snapTheta(view.theta) + step * QUARTER
  const entry = focusEntry ?? null
  startTween(entry === null ? overviewView(theta) : {...solveView(regionPoints(entry, theta), theta, 1.85), zoom: view.zoom}, 650)
}
const resetView = (): void => {
  focusEntry = null
  startTween(overviewView(HOME_THETA), 650)
  if (props.pinned !== null) emit('pick', null)
}

// =============== 联动与自动聚焦 ===============
// 说明：点击固定条目立即聚焦，取消固定立即回到全景；在环图中停留 0.45 秒后自动聚焦，移开 0.9 秒后回到全景。
//       平台自身的悬停只抬升立柱，不移动镜头，避免画面在指针下方漂移。
let dwell: number | undefined
let lastPinned: number | null = null
const scheduleFocus = (): void => {
  window.clearTimeout(dwell)
  if (props.pinned !== null) { lastPinned = props.pinned; focusOn(props.pinned); return }
  const unpinned = lastPinned !== null
  lastPinned = null
  if (unpinned) { focusOn(null); return }
  const external = props.hovered !== null && props.hovered !== pointerEntry
  if (external && autoFocus.value) {
    const target = props.hovered
    dwell = window.setTimeout(() => { if (props.hovered === target && props.pinned === null) focusOn(target) }, 450)
  } else if (props.hovered === null && focusEntry !== null && focusEntry !== undefined) {
    dwell = window.setTimeout(() => { if (props.hovered === null && props.pinned === null) focusOn(null) }, 900)
  }
}
watch(() => [props.pinned, props.hovered] as const, () => { scheduleFocus(); kick() })
watch(autoFocus, value => { if (!value && props.pinned === null) focusOn(null) })
watch(() => [props.entries, props.metric] as const, () => { tip.value = null; rebuild(); kick() })
watch(() => props.palette, () => { applyPalette(); kick() })
// 精灵图迟到时补上插画。
watch(() => [factoryIconsReady.value, factoryIconImageReady.value] as const, () => { buildVisuals(); applyPalette(); kick() })

// =============== 浮层内容 ===============
// 说明：设施展示建筑与电力，生产物品只展示生产，空间构成只展示空间利用率。
const tipEntry = computed(() => (tip.value ? props.entries[tip.value.entry] ?? null : null))
const tipMetrics = computed(() => {
  const entry = tipEntry.value
  return entry ? SHARE_METRICS.filter(item => shareMetricsOf(entry).includes(item.key)) : []
})
const kindLabel = (entry: ShareEntry): string => (entry.kind === 'product' ? '生产物品' : entry.kind === 'space' ? '空间构成' : categoryLabel(entry.category))
const tipHint = (entry: ShareEntry): string => (entry.kind === 'product' ? '点击聚焦这个物品区域' : entry.kind === 'space' ? '点击聚焦这片区域' : '点击聚焦这个建筑群')
const subject = computed(() => (props.metric === 'production' ? '物品产出' : props.metric === 'space' ? '空间占用' : '建筑'))
const tipStyle = computed(() => {
  const value = tip.value
  if (!value) return {}
  const flip = value.x > width - 240
  return {left: `${flip ? value.x - 14 : value.x + 14}px`, top: `${Math.min(value.y + 14, Math.max(8, height - 150))}px`, transform: flip ? 'translateX(-100%)' : 'none'}
})

// =============== 生命周期 ===============
let resizeObserver: ResizeObserver | null = null
onMounted(() => {
  if (!canvas.value || !host.value) return
  try {
    renderer = new THREE.WebGLRenderer({canvas: canvas.value, antialias: true, alpha: true, powerPreference: 'low-power'})
  } catch {
    supported.value = false
    return
  }
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.setClearColor(0x000000, 0)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  resizeObserver = new ResizeObserver(([entry]) => {
    const nextWidth = Math.round(entry.contentRect.width)
    const nextHeight = Math.round(entry.contentRect.height)
    if (nextWidth === width && nextHeight === height) return
    width = nextWidth
    height = nextHeight
    if (!renderer || !width || !height) return
    // 全景状态随尺寸重新取景；聚焦中保持当前镜头，只更新画幅比例。
    if (focusEntry === undefined) { rebuildFocus(); return }
    if (focusEntry === null && !tween) Object.assign(view, overviewView(snapTheta(view.theta)))
    kick()
  })
  resizeObserver.observe(host.value)
  rebuild()
})
const rebuildFocus = (): void => {
  focusEntry = undefined
  focusOn(props.pinned, false)
  kick()
}
onBeforeUnmount(() => {
  disposed = true
  window.clearTimeout(dwell)
  if (frame) cancelAnimationFrame(frame)
  resizeObserver?.disconnect()
  disposeVisuals()
  tileGeometry.dispose()
  tileMaterial.dispose()
  tileMesh.dispose()
  slabGeometry.dispose()
  slabMaterial.dispose()
  rimGeometry.dispose()
  rimMaterial.dispose()
  shadowGeometry.dispose()
  renderer?.dispose()
  renderer?.forceContextLoss()
  renderer = null
})
</script>

<template>
  <!-- ~ 整体占比台地 -->
  <!-- * 说明：10×10 阶梯浮空平台，一根立柱 = 当前口径的 1%；生产口径按物品划分区域与插画，空间口径的空地只铺地砖 -->
  <!-- * 交互：悬停区域平齐并联动占比环；点击固定聚焦；支持拖动旋转、滚轮缩放和双击复位 -->
  <!-- * 依赖：three.js 正交等距相机；插画取自图鉴精灵 -->
  <!-- * 边界：只表达整体构成，不表示蓝图中的实际位置；WebGL 不可用时仍可查看右侧占比环 -->
  <div ref="host" class="platform">
    <canvas ref="canvas" class="platform-canvas" role="img" :aria-label="`全厂${subject}构成等距平台`" @pointerdown="onPointerDown" @pointermove="onPointerMove" @pointerup="onPointerUp" @pointerleave="onPointerLeave" @wheel.prevent="onWheel" @dblclick="resetView" />

    <!-- 悬停浮层：设施展示建筑和电力，生产物品展示产出，空间构成展示面积与占包络比例。 -->
    <div v-if="tip && tipEntry" class="platform-tip" :style="tipStyle" role="tooltip">
      <header><ItemIcon :name="tipEntry.icon" :size="24" /><span><strong>{{ tipEntry.name }}</strong><small>{{ kindLabel(tipEntry) }}</small></span></header>
      <dl :class="{'is-single': tipMetrics.length === 1}">
        <div v-for="item in tipMetrics" :key="item.key" :class="{'is-current': item.key === metric}"><dt>{{ item.label }}</dt><dd class="type-display">{{ formatShare(tipEntry.shares[item.key]) }}</dd></div>
      </dl>
      <p v-if="tipEntry.kind === 'building'">建筑 {{ formatInteger(tipEntry.count) }} 座 · 额定 {{ formatPower(tipEntry.powerMw) }}</p>
      <p v-else-if="tipEntry.kind === 'product'">产出 {{ formatValue(tipEntry.productionRate) }} 件/min</p>
      <p v-else>面积 {{ formatInteger(tipEntry.areaGrid2) }} 格²（格网近似）</p>
      <p class="tip-hint">{{ tipHint(tipEntry) }}</p>
    </div>

    <!-- 右上：视角工具 -->
    <div class="platform-tools" role="toolbar" aria-label="平台视角">
      <n-tooltip><template #trigger><button type="button" class="tool" aria-label="向左旋转" :disabled="!supported" @click="rotate(-1)"><RotateCcw :size="14" /></button></template>向左旋转 90°</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool" aria-label="向右旋转" :disabled="!supported" @click="rotate(1)"><RotateCw :size="14" /></button></template>向右旋转 90°</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool" aria-label="复位视角" :disabled="!supported" @click="resetView"><LocateFixed :size="14" /></button></template>复位到全景（双击平台同样复位）</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool" :class="{'is-on': autoFocus}" :aria-pressed="autoFocus" aria-label="自动聚焦" :disabled="!supported" @click="autoFocus = !autoFocus"><ScanEye :size="14" /></button></template>{{ autoFocus ? '自动聚焦已开启：在占比环上停留时镜头放大并旋转到对应区域' : '自动聚焦已关闭：只有点击时才聚焦' }}</n-tooltip>
    </div>
    <p class="platform-hint">拖动旋转 · 滚轮缩放 · {{ metric === 'production' ? '点击物品区域聚焦' : metric === 'space' ? '点击区域聚焦' : '点击建筑群聚焦' }}，再次点击取消</p>
    <p v-if="!supported" class="platform-message">当前环境不支持 WebGL，无法显示平台；右侧占比环仍可查看占比。</p>
    <p v-else-if="!placed.length" class="platform-message">{{ metric === 'production' ? '当前方案没有产出物品' : metric === 'space' ? '生成蓝图后统计空间利用率' : '当前口径没有可统计的数据' }}</p>
  </div>
</template>

<style scoped lang="scss">
.platform { position: relative; width: 100%; height: 100%; min-height: 220px; overflow: hidden; }
.platform-canvas { position: absolute; inset: 0; width: 100%; height: 100%; cursor: grab; touch-action: none; }
.platform-canvas:active { cursor: grabbing; }
.platform-tip { position: absolute; z-index: 3; width: 224px; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 8px 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 11px; line-height: 1.6; pointer-events: none; box-shadow: 0 6px 18px rgb(0 0 0 / 0.28); }
.platform-tip header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.platform-tip header span { display: flex; min-width: 0; flex-direction: column; line-height: 1.3; }
.platform-tip strong { overflow: hidden; color: var(--ds-text-primary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.platform-tip small { color: var(--ds-text-muted); font-size: 10px; }
.platform-tip dl { display: grid; grid-template-columns: repeat(2, 1fr); gap: 4px; margin: 0 0 6px; }
.platform-tip dl.is-single { grid-template-columns: minmax(0, 1fr); }
.platform-tip dl div { border: 1px solid var(--ds-border-subtle); border-radius: 2px; padding: 2px 4px; text-align: center; }
.platform-tip dl div.is-current { border-color: var(--ds-brand); }
.platform-tip dt { color: var(--ds-text-muted); font-size: 10px; }
.platform-tip dd { margin: 0; color: var(--ds-text-primary); font-size: 13px; font-weight: 600; }
.platform-tip p { margin: 0; }
.platform-tip .tip-hint { margin-top: 4px; color: var(--ds-text-muted); font-size: 10px; }
.platform-tools { position: absolute; top: 6px; right: 8px; z-index: 2; display: flex; gap: 4px; }
.tool { display: inline-flex; width: 26px; height: 26px; align-items: center; justify-content: center; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0; background: color-mix(in srgb, var(--ds-bg-panel-elevated) 90%, transparent); color: var(--ds-text-secondary); cursor: pointer; transition: border-color 150ms, color 150ms; }
.tool:hover:not(:disabled) { border-color: var(--ds-border-strong); color: var(--ds-text-primary); }
.tool.is-on { border-color: var(--ds-brand); color: var(--ds-brand); }
.tool:disabled { cursor: not-allowed; opacity: .45; }
.tool:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: 1px; }
.platform-hint { position: absolute; bottom: 4px; left: 10px; margin: 0; color: var(--ds-text-muted); font-size: 10px; pointer-events: none; }
.platform-message { position: absolute; right: 0; bottom: 24px; left: 0; margin: 0; color: var(--ds-text-muted); font-size: 12px; text-align: center; pointer-events: none; }
</style>
