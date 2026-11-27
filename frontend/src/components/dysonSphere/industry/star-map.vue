<script setup lang="ts">
import * as THREE from 'three'
import {OrbitControls} from 'three/examples/jsm/controls/OrbitControls.js'
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch} from 'vue'
import {ArrowLeft, LocateFixed, Minus, Orbit, Plus} from '@lucide/vue'
import type {industryworkspace} from '../../../../wailsjs/go/models'

// =============== 存档三维星图 ===============
// 说明：银河与恒星内两级浏览共用一个相机；坐标来自同一存档快照，球体大小仅为示意。
// 边界：只展示规划路线和选择星球，不在前端计算产量或修改存档；缓存页停用时停止渲染。
// 状态表达：星体颜色区分类别（光谱 / 固态 / 气态），规划、中心、交付与选中由 DOM 标记和标签文字表达，
// 不只依赖颜色；标签按优先级避让，缩放后自动显示更多名称。
const props = defineProps<{
  selection: industryworkspace.Selection
  report: industryworkspace.Report | null
  selected: number | null
  center?: number | null
  deliveries?: number[]
  loading?: boolean
}>()
const emit = defineEmits<{(e: 'select', id: number): void}>()

interface MapNode {
  id: number
  name: string
  label: string
  position: {x: number; y: number; z: number}
  parent?: number
  gas: boolean
  moon: boolean
  planned: number
  center: boolean
  delivery: boolean
  stellar?: {kind: number; spectrum: number; starId: number}
}
interface MapTag {text: string; kind: 'center' | 'delivery' | 'planned'}
interface MapBody {
  id: number
  point: THREE.Vector3
  radius: number
  hover: string
  label: string
  tags: MapTag[]
  priority: number
  width: number
  selected: boolean
  anchor: boolean
  planned: boolean
  star: boolean
}
interface ScreenBody {body: MapBody; x: number; y: number; r: number}
interface MapLabel {id: number; text: string; tags: MapTag[]; x: number; y: number; selected: boolean; hovered: boolean; star: boolean}
interface MapMarker {id: number; x: number; y: number; size: number; selected: boolean; anchor: boolean; planned: boolean; hovered: boolean}
interface Rect {x: number; y: number; w: number; h: number}

const host = ref<HTMLDivElement | null>(null)
const starId = ref<number | null>(null)
const hoveredId = ref<number | null>(null)
const tooltip = ref<{title: string; action: string; x: number; y: number} | null>(null)
const unavailable = ref(false)
const visibleLabels = ref<MapLabel[]>([])
const markers = ref<MapMarker[]>([])
const star = computed(() => props.selection.starSystems.find(s => s.starId === starId.value))
const systemSites = computed(() => props.selection.sites.filter(s => s.starId === starId.value))
const used = computed(() => new Set(props.report?.planets.map(p => p.id) ?? []))
const plannedHere = computed(() => star.value
  ? systemSites.value.filter(s => used.value.has(s.id)).length
  : new Set(props.selection.sites.filter(s => used.value.has(s.id)).map(s => s.starId)).size)
const pointing = computed(() => hoveredId.value != null && (!star.value || hoveredId.value > 0))

let renderer: THREE.WebGLRenderer | null = null, camera: THREE.PerspectiveCamera | null = null, controls: OrbitControls | null = null
let observer: ResizeObserver | null = null, theme: MutationObserver | null = null, frame = 0, active = true
let glowTexture: THREE.CanvasTexture | null = null
const scene = new THREE.Scene(), bodies = new THREE.Group(), lines = new THREE.Group()
const focusCenter = new THREE.Vector3()
let mapBodies: MapBody[] = [], extent = 70, dragStart = {x: 0, y: 0}
const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
const measure = document.createElement('canvas').getContext('2d')
const LABEL_HEIGHT = 20, SAFE_TOP = 52, SAFE_BOTTOM = 48, ORDINARY_LABELS = 16
// 标签方位记忆与交互状态：拖动、阻尼惯性期间冻结普通标签集合，停止后补齐。
const labelSlots = new Map<number, number>()
let interacting = false, settleTimer: ReturnType<typeof setTimeout> | undefined
// 用户未手动旋转或缩放前，窗口尺寸变化时重新取景，避免窄窗口下边缘星体（如选中的卫星）移出画面。
let framed = true
function startInteract(): void { interacting = true; framed = false; clearTimeout(settleTimer) }
function endInteract(): void {
  clearTimeout(settleTimer)
  settleTimer = setTimeout(() => { interacting = false; invalidate() }, reduceMotion ? 0 : 320)
}

// 图示按存档恒星类型和光谱着色；规划参与情况由外圈和标签表示。
const spectra = ['M', 'K', 'G', 'F', 'A', 'B', 'O']
const daylight = ['#af5245', '#bc733c', '#ae842e', '#89845e', '#678397', '#497ca9', '#41649d']
const night = ['#ed947c', '#f4b879', '#eacb84', '#dddab7', '#c1d9ed', '#91bee6', '#82a0ec']
function starAppearance(stellar: NonNullable<MapNode['stellar']>, dark: boolean) {
  const compact = ['白矮星', '中子星', '黑洞'][stellar.kind - 2]
  if (compact) {
    const colors = dark ? ['#d4dce2', '#8ddade', '#bd9edc'] : ['#737f8d', '#327d85', '#6a527e']
    return {name: compact, color: colors[stellar.kind - 2], size: 1.15}
  }
  const giant = stellar.kind === 1
  return {name: `${spectra[stellar.spectrum] ?? ''} 型${giant ? '巨星' : '主序星'}`, color: (dark ? night : daylight)[stellar.spectrum], size: giant ? 1.45 : 1}
}

// 从星图容器读取实际色值（含组件内的星图语义变量），避免画布与日夜样式分离；透明度由材质单独控制。
function color(token: string): THREE.Color {
  const target = host.value ?? document.documentElement
  const raw = getComputedStyle(target).getPropertyValue(token).trim()
  const probe = document.createElement('span')
  probe.style.color = raw; target.appendChild(probe)
  const value = getComputedStyle(probe).color
  probe.remove()
  const [r = 128, g = 128, b = 128] = value.match(/[\d.]+/g)?.map(Number) ?? []
  return new THREE.Color().setRGB(r / 255, g / 255, b / 255, THREE.SRGBColorSpace)
}
function glow(): THREE.CanvasTexture {
  if (glowTexture) return glowTexture
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = 64
  const context = canvas.getContext('2d')!
  const gradient = context.createRadialGradient(32, 32, 0, 32, 32, 32)
  gradient.addColorStop(0, 'rgba(255,255,255,1)'); gradient.addColorStop(.22, 'rgba(255,255,255,.42)'); gradient.addColorStop(1, 'rgba(255,255,255,0)')
  context.fillStyle = gradient; context.fillRect(0, 0, 64, 64)
  glowTexture = new THREE.CanvasTexture(canvas); glowTexture.colorSpace = THREE.SRGBColorSpace
  return glowTexture
}
function disposeGroup(group: THREE.Group): void {
  for (const object of [...group.children]) {
    if (object instanceof THREE.Mesh || object instanceof THREE.Line) {
      object.geometry.dispose()
      const materials = Array.isArray(object.material) ? object.material : [object.material]
      materials.forEach(m => m.dispose())
    } else if (object instanceof THREE.Sprite) object.material.dispose()
    group.remove(object)
  }
}
// 以父天体为圆心、经过子天体的近水平圆，示意轨道；倾角取自存档坐标，不代表真实轨道根数。
function orbit(center: THREE.Vector3, point: THREE.Vector3): THREE.Vector3[] {
  const radial = point.clone().sub(center), radius = radial.length()
  if (radius < 1e-6) return []
  const u = radial.normalize(), up = new THREE.Vector3(0, 1, 0)
  let normal = up.clone().addScaledVector(u, -up.dot(u))
  if (normal.lengthSq() < 1e-6) normal = new THREE.Vector3(1, 0, 0).addScaledVector(u, -u.x)
  const v = normal.normalize().cross(u)
  return Array.from({length: 128}, (_, i) => {
    const a = i * Math.PI / 64
    return center.clone().addScaledVector(u, Math.cos(a) * radius).addScaledVector(v, Math.sin(a) * radius)
  })
}
function textWidth(text: string, font: string): number {
  if (!measure) return [...text].reduce((size, char) => size + (char.charCodeAt(0) > 255 ? 12 : 7), 0)
  measure.font = font
  return measure.measureText(text).width
}
function labelWidth(text: string, tags: MapTag[]): number {
  const base = textWidth(text, '600 12px "Source Han Sans SC", "Microsoft YaHei", sans-serif') + 16
  return Math.ceil(tags.reduce((size, tag) => size + 4 + 8 + textWidth(tag.text, '500 11px "Source Han Sans SC", "Microsoft YaHei", sans-serif'), base))
}

function resetCamera(): void {
  if (!camera || !controls) return
  controls.target.copy(focusCenter)
  const direction = new THREE.Vector3(.8, 1, 1.15).normalize()
  const right = camera.up.clone().cross(direction).normalize()
  const up = direction.clone().cross(right)
  const halfHeight = Math.tan(THREE.MathUtils.degToRad(camera.fov / 2))
  let distance = 4
  for (const body of mapBodies) {
    const point = body.point.clone().sub(focusCenter)
    const fit = Math.max(Math.abs(point.dot(right)) / camera.aspect, Math.abs(point.dot(up))) / halfHeight
    distance = Math.max(distance, point.dot(direction) + fit * 1.22)
  }
  camera.position.copy(focusCenter).addScaledVector(direction, distance)
  camera.near = .05; camera.far = Math.max(2000, extent * 20); camera.updateProjectionMatrix()
  controls.minDistance = 3; controls.maxDistance = Math.max(extent * 6, distance * 2); controls.update()
  framed = true
  invalidate()
}
function zoom(factor: number): void {
  if (!camera || !controls) return
  framed = false
  const offset = camera.position.clone().sub(controls.target)
  const distance = THREE.MathUtils.clamp(offset.length() * factor, controls.minDistance, controls.maxDistance)
  camera.position.copy(controls.target).addScaledVector(offset.normalize(), distance)
  controls.update(); invalidate()
}

function rebuild(reset = false): void {
  if (!renderer || !camera) return
  disposeGroup(bodies); disposeGroup(lines); mapBodies = []
  const dark = document.documentElement.dataset.theme === 'dark'
  const rocky = color('--map-rocky'), gas = color('--map-gas'), planned = color('--ds-brand'), chosen = color('--ds-success'), orbitColor = color('--map-orbit')
  const system = star.value
  const origin = system?.position ?? {x: 0, y: 0, z: 0}
  const scale = system ? 1 / 1800 : 1 / 2400000
  const siteById = new Map(props.selection.sites.map(s => [s.id, s]))
  const deliveries = new Set(props.deliveries ?? [])
  const centerStar = props.center ? siteById.get(props.center)?.starId : undefined
  const deliveryStars = new Set([...deliveries].map(id => siteById.get(id)?.starId))
  const plannedByStar = new Map<number, number>()
  for (const site of props.selection.sites) if (used.value.has(site.id)) plannedByStar.set(site.starId, (plannedByStar.get(site.starId) ?? 0) + 1)
  const nodes: MapNode[] = system
    ? [
        {id: -system.starId, name: system.name, label: system.name, position: system.position, gas: false, moon: false, planned: 0, center: false, delivery: false, stellar: system},
        ...systemSites.value.map(s => ({
          id: s.id, name: s.name, label: s.name.replace(system.name + ' - ', ''), position: s.position, parent: s.parentPlanetId || -system.starId,
          gas: s.gas, moon: s.parentPlanetId > 0, planned: used.value.has(s.id) ? 1 : 0, center: s.id === props.center, delivery: deliveries.has(s.id),
        })),
      ]
    : props.selection.starSystems.map(s => ({
        id: s.starId, name: s.name, label: s.name, position: s.position, gas: false, moon: false,
        planned: plannedByStar.get(s.starId) ?? 0, center: centerStar === s.starId, delivery: deliveryStars.has(s.starId), stellar: s,
      }))
  const positions = new Map<number, THREE.Vector3>()
  for (const node of nodes) positions.set(node.id, new THREE.Vector3(node.position.x - origin.x, node.position.y - origin.y, node.position.z - origin.z).multiplyScalar(scale))
  const bounds = new THREE.Box3().setFromPoints([...positions.values()])
  bounds.getCenter(focusCenter)
  extent = Math.max(5, bounds.getSize(new THREE.Vector3()).length() / 2)
  // 卫星离母星的真实距离远小于示意半径，按比例绘制会被母星盖住（如天钩二 2 号星藏在 1 号星内），无法辨认也点不到。
  // 恒星系视图中把过近的卫星沿原方向推到母星半径之外；轨道本就是示意，不影响星球之间的远近关系。
  if (system) {
    const byId = new Map(nodes.map(node => [node.id, node]))
    for (const node of nodes) {
      const parent = node.moon && node.parent != null ? byId.get(node.parent) : undefined
      const from = parent && positions.get(parent.id), to = positions.get(node.id)
      if (!parent || !from || !to) continue
      const minimum = extent * ((parent.gas ? .032 : .021) + .015 + .06)
      const offset = to.clone().sub(from), distance = offset.length()
      if (distance >= minimum) continue
      if (distance < 1e-6) offset.set(1, 0, 0)
      to.copy(from).addScaledVector(offset.normalize(), minimum)
    }
  }

  for (const node of nodes) {
    const point = positions.get(node.id)!
    const appearance = node.stellar && starAppearance(node.stellar, dark)
    const selected = !!system && node.id === props.selected
    const radius = system
      ? extent * (appearance ? .05 : node.gas ? .032 : node.moon ? .015 : .021)
      : Math.max(.19, extent * .0085) * (appearance?.size ?? 1)
    const fill = appearance ? new THREE.Color(appearance.color) : node.gas ? gas : rocky
    const mesh = new THREE.Mesh(new THREE.SphereGeometry(radius, 24, 16), new THREE.MeshBasicMaterial({color: fill}))
    mesh.position.copy(point); bodies.add(mesh)
    if (appearance) {
      // 柔和光晕替代半透明球壳：夜间叠加发光，日间常规混合，避免浅色背景上出现灰色圆盘。
      const halo = new THREE.Sprite(new THREE.SpriteMaterial({map: glow(), color: fill, transparent: true, depthWrite: false,
        opacity: dark ? .6 : .34, blending: dark ? THREE.AdditiveBlending : THREE.NormalBlending}))
      const size = radius * (system ? 3.6 : 6.4)
      halo.position.copy(point); halo.scale.set(size, size, 1); bodies.add(halo)
    }
    const tags: MapTag[] = []
    if (node.center) tags.push({text: '中心', kind: 'center'})
    if (node.delivery) tags.push({text: '交付', kind: 'delivery'})
    if (node.planned) tags.push({text: system ? '已规划' : `规划 ${node.planned}`, kind: 'planned'})
    const priority = selected ? 100 : node.center ? 80 : node.delivery ? 70 : node.planned ? 60
      : appearance && (system || node.stellar?.starId === 1) ? 50 : (node.stellar?.kind ?? 0) > 0 ? 40 : system ? 30 : 10
    mapBodies.push({
      id: node.id, point, radius, label: node.label, tags, priority, width: labelWidth(node.label, tags),
      hover: appearance ? `${node.name} · ${appearance.name}` : `${node.name}${node.gas ? ' · 气态巨星' : node.moon ? ' · 卫星' : ' · 固态星球'}`,
      selected, anchor: node.center || node.delivery, planned: node.planned > 0, star: !!appearance && !!system,
    })
  }
  if (system) {
    for (const node of nodes) {
      const parent = node.parent == null ? undefined : positions.get(node.parent), child = positions.get(node.id)
      if (!parent || !child) continue
      const points = orbit(parent, child)
      if (!points.length) continue
      const focused = node.id === props.selected
      lines.add(new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(points),
        new THREE.LineBasicMaterial({color: focused ? chosen : orbitColor, transparent: true, opacity: focused ? .8 : dark ? .38 : .5})))
    }
  }
  const seen = new Set<string>()
  for (const route of props.report?.routes ?? []) {
    const from = siteById.get(route.fromPlanetId), to = siteById.get(route.toPlanetId)
    if (!from || !to) continue
    const a = system ? from.id : from.starId, b = system ? to.id : to.starId
    const key = [a, b].sort((x, y) => x - y).join(':')
    if (a === b || seen.has(key) || !positions.has(a) || !positions.has(b)) continue
    seen.add(key)
    const focused = from.id === props.selected || to.id === props.selected
    const geometry = new THREE.BufferGeometry().setFromPoints([positions.get(a)!, positions.get(b)!])
    lines.add(new THREE.Line(geometry, new THREE.LineBasicMaterial({color: focused ? chosen : planned, transparent: true, opacity: focused ? .9 : .45})))
  }
  if (reset) resetCamera()
  invalidate()
}
function enterSystem(id: number | null): void { starId.value = id; hoveredId.value = null; tooltip.value = null; rebuild(true) }

function toScreen(body: MapBody, width: number, height: number): ScreenBody | null {
  if (!camera) return null
  const p = body.point.clone().project(camera)
  if (p.z > 1 || p.z < -1) return null
  const x = (p.x + 1) * width / 2, y = (1 - p.y) * height / 2
  if (x < -40 || x > width + 40 || y < -40 || y > height + 40) return null
  const distance = Math.max(.001, camera.position.distanceTo(body.point))
  const r = body.radius / (distance * Math.tan(THREE.MathUtils.degToRad(camera.fov / 2))) * height / 2
  return {body, x, y, r}
}
// 屏幕空间拾取：按最近中心点命中，小星体也有足够的点击范围。
function pick(event: PointerEvent): MapBody | undefined {
  if (!camera || !host.value) return
  const rect = host.value.getBoundingClientRect()
  const x = event.clientX - rect.left, y = event.clientY - rect.top
  let best: MapBody | undefined, bestDistance = Infinity
  for (const body of mapBodies) {
    const screen = toScreen(body, rect.width, rect.height)
    if (!screen) continue
    const distance = Math.hypot(screen.x - x, screen.y - y)
    if (distance <= Math.max(12, screen.r + 6) && distance < bestDistance) { best = body; bestDistance = distance }
  }
  return best
}
function pointerMove(event: PointerEvent): void {
  if (event.buttons) { tooltip.value = null; return }
  const body = pick(event)
  const id = body?.id ?? null
  if (id !== hoveredId.value) { hoveredId.value = id; invalidate() }
  if (!body || !host.value) { tooltip.value = null; return }
  const rect = host.value.getBoundingClientRect()
  const x = event.clientX - rect.left, y = event.clientY - rect.top
  tooltip.value = {
    title: body.hover,
    action: !star.value ? '点击进入恒星系' : body.id > 0 ? (body.id === props.selected ? '当前选择' : '点击查看星球详情') : '恒星',
    x: Math.min(x + 14, rect.width - 220), y: Math.min(y + 16, rect.height - 56),
  }
}
function pointerLeave(): void {
  tooltip.value = null
  if (hoveredId.value != null) { hoveredId.value = null; invalidate() }
}
function pointerUp(event: PointerEvent): void {
  if (event.button !== 0 || Math.hypot(event.clientX - dragStart.x, event.clientY - dragStart.y) > 5) return
  const hit = pick(event)
  if (!hit) return
  if (starId.value == null) enterSystem(hit.id)
  else if (hit.id > 0) emit('select', hit.id)
}

// 相机和内容发生变化时才绘制。阻尼旋转会继续触发 change，静止时为后台规划留出算力。
function invalidate(): void {
  if (active && !frame) frame = requestAnimationFrame(render)
}
function render(): void {
  frame = 0
  if (!active || !renderer || !camera || !host.value) return
  controls?.update(); renderer.render(scene, camera)
  layoutOverlay(host.value.clientWidth || 1, host.value.clientHeight || 1)
}
function overlaps(a: Rect, b: Rect, pad: number): boolean {
  return a.x < b.x + b.w + pad && a.x + a.w + pad > b.x && a.y < b.y + b.h + pad && a.y + a.h + pad > b.y
}
function hitsBody(body: ScreenBody, rect: Rect): boolean {
  const nx = Math.max(rect.x, Math.min(body.x, rect.x + rect.w)), ny = Math.max(rect.y, Math.min(body.y, rect.y + rect.h))
  return Math.hypot(body.x - nx, body.y - ny) < Math.max(2, body.r) + 2
}
// 标签按优先级放置：右、左、上、下四个候选位；重要标签找不到空位时允许压住远处星点，但不压住其他标签。
function layoutOverlay(width: number, height: number): void {
  const screen = mapBodies.flatMap(body => toScreen(body, width, height) ?? [])
  const hovered = hoveredId.value
  markers.value = screen
    .filter(s => s.body.selected || s.body.anchor || s.body.planned || s.body.id === hovered)
    .map(s => ({id: s.body.id, x: s.x, y: s.y, size: Math.round(Math.max(14, s.r * 2 + 10)), selected: s.body.selected, anchor: s.body.anchor, planned: s.body.planned, hovered: s.body.id === hovered}))
  // 稳定性：旋转时屏幕坐标逐帧变化，若每帧重新贪心选位，标签会在四个方位间跳动或时隐时现。
  // 已显示的标签优先保留并沿用上次方位，只在越界或撞上更高优先级标签时才换位；拖动期间不新增普通标签，静止后再补齐。
  const shown = new Set(visibleLabels.value.map(label => label.id))
  const base = (s: ScreenBody) => s.body.id === hovered ? 95 : s.body.priority
  const rank = (s: ScreenBody) => base(s) + (shown.has(s.body.id) ? 5 : 0)
  const placed: Rect[] = [], labels: MapLabel[] = []
  let ordinary = 0
  for (const s of [...screen].sort((a, b) => rank(b) - rank(a))) {
    const id = s.body.id, priority = base(s), was = shown.has(id)
    if (priority <= 10 && ordinary >= ORDINARY_LABELS) continue
    if (interacting && !was && priority < 50) continue
    const w = s.body.width, h = LABEL_HEIGHT, gap = Math.max(3, s.r) + 6
    const slots: Rect[] = [
      {x: s.x + gap, y: s.y - h / 2, w, h}, {x: s.x - gap - w, y: s.y - h / 2, w, h},
      {x: s.x - w / 2, y: s.y - gap - h, w, h}, {x: s.x - w / 2, y: s.y + gap, w, h},
    ]
    const fits = (r: Rect) => r.x >= 8 && r.x + w <= width - 8 && r.y >= SAFE_TOP && r.y + h <= height - SAFE_BOTTOM && !placed.some(old => overlaps(old, r, 4))
    const previous = labelSlots.get(id)
    let index = was && previous != null && fits(slots[previous]!) ? previous : slots.findIndex(r => fits(r) && !screen.some(o => o !== s && hitsBody(o, r)))
    if (index < 0 && (priority >= 50 || was)) index = slots.findIndex(fits)
    if (index < 0) { labelSlots.delete(id); continue }
    const spot = slots[index]!
    labelSlots.set(id, index)
    if (priority <= 10) ordinary++
    placed.push(spot)
    labels.push({id, text: s.body.label, tags: s.body.tags, x: Math.round(spot.x), y: Math.round(spot.y), selected: s.body.selected, hovered: id === hovered, star: s.body.star})
  }
  const kept = new Set(labels.map(label => label.id))
  for (const id of [...labelSlots.keys()]) if (!kept.has(id)) labelSlots.delete(id)
  visibleLabels.value = labels
}
function resize(): void {
  if (!host.value || !renderer || !camera) return
  const {clientWidth: width, clientHeight: height} = host.value
  renderer.setSize(width, height); camera.aspect = Math.max(1, width) / Math.max(1, height); camera.updateProjectionMatrix()
  if (framed && mapBodies.length) resetCamera(); else invalidate()
}
onMounted(() => {
  if (!host.value) return
  try { renderer = new THREE.WebGLRenderer({alpha: true, antialias: true}) } catch { unavailable.value = true; return }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2)); host.value.prepend(renderer.domElement)
  camera = new THREE.PerspectiveCamera(45, 1, .05, 2000)
  controls = new OrbitControls(camera, renderer.domElement); controls.enableDamping = !reduceMotion; controls.dampingFactor = .12
  controls.addEventListener('change', invalidate)
  controls.addEventListener('start', startInteract); controls.addEventListener('end', endInteract)
  scene.add(lines, bodies); resize(); rebuild(true)
  observer = new ResizeObserver(resize); observer.observe(host.value)
  theme = new MutationObserver(() => rebuild()); theme.observe(document.documentElement, {attributes: true, attributeFilter: ['data-theme']})
  invalidate()
})
// 交付列表由页面每次渲染重新生成，按内容比较，避免无关刷新时重建场景。
watch(() => [props.report, props.selected, props.center ?? null, (props.deliveries ?? []).join(',')], () => rebuild())
watch(() => props.selection.snapshotId, () => enterSystem(null))
watch(() => props.selected, id => {
  if (id == null) return
  const selected = props.selection.sites.find(s => s.id === id)
  if (selected && starId.value !== selected.starId) enterSystem(selected.starId)
})
onActivated(() => { active = true; invalidate() })
onDeactivated(() => { active = false; cancelAnimationFrame(frame); frame = 0 })
onBeforeUnmount(() => {
  active = false; cancelAnimationFrame(frame); clearTimeout(settleTimer); observer?.disconnect(); theme?.disconnect()
  controls?.removeEventListener('change', invalidate); controls?.removeEventListener('start', startInteract); controls?.removeEventListener('end', endInteract); controls?.dispose()
  disposeGroup(bodies); disposeGroup(lines); glowTexture?.dispose(); renderer?.dispose(); renderer?.domElement.remove()
})
</script>

<template>
  <!-- ~ 星图画布：点击恒星进入，拖动旋转；标签、状态标记与控件保持在 DOM 中，可读且跟随主题 -->
  <div ref="host" class="star-map" :class="{'is-pointing': pointing}" @pointerdown="dragStart = {x: $event.clientX, y: $event.clientY}" @pointerup="pointerUp" @pointermove="pointerMove" @pointerleave="pointerLeave">
    <div class="map-overlay" aria-hidden="true">
      <span v-for="m in markers" :key="'m' + m.id" class="map-marker"
        :class="{'is-selected': m.selected, 'is-anchor': m.anchor && !m.selected, 'is-planned': m.planned && !m.anchor && !m.selected, 'is-hovered': m.hovered && !m.selected && !m.anchor && !m.planned}"
        :style="{left: m.x + 'px', top: m.y + 'px', width: m.size + 'px', height: m.size + 'px'}" />
      <div v-for="label in visibleLabels" :key="label.id" class="map-label" :class="{'is-selected': label.selected, 'is-hovered': label.hovered, 'is-star': label.star}"
        :style="{transform: `translate(${label.x}px, ${label.y}px)`}">
        <span class="label-text">{{ label.text }}</span>
        <span v-for="tag in label.tags" :key="tag.kind" class="label-tag" :class="'is-' + tag.kind">{{ tag.text }}</span>
      </div>
    </div>

    <header class="map-heading" :class="{'has-back': star}" @pointerdown.stop @pointerup.stop>
      <n-button v-if="star" size="tiny" quaternary aria-label="返回银河星图" @click="enterSystem(null)"><template #icon><ArrowLeft :size="14" /></template>银河</n-button>
      <Orbit v-else :size="15" class="heading-icon" />
      <strong :title="star?.name ?? '银河星图'">{{ star?.name ?? '银河星图' }}</strong>
      <span class="heading-meta"><b>{{ star ? systemSites.length : selection.stars }}</b>{{ star ? " 颗星球" : " 个恒星系" }}<template v-if="plannedHere"> · 规划 <b>{{ plannedHere }}</b></template></span>
    </header>

    <div class="map-tools" role="toolbar" aria-label="星图视角" @pointerdown.stop @pointerup.stop>
      <n-tooltip placement="left"><template #trigger><n-button size="small" quaternary aria-label="放大" @click="zoom(.72)"><template #icon><Plus :size="15" /></template></n-button></template>放大</n-tooltip>
      <n-tooltip placement="left"><template #trigger><n-button size="small" quaternary aria-label="缩小" @click="zoom(1.38)"><template #icon><Minus :size="15" /></template></n-button></template>缩小</n-tooltip>
      <n-tooltip placement="left"><template #trigger><n-button size="small" quaternary aria-label="重置星图视角" @click="resetCamera"><template #icon><LocateFixed :size="15" /></template></n-button></template>重置视角</n-tooltip>
    </div>

    <footer class="map-legend" aria-label="图例">
      <template v-if="star">
        <span class="legend-item"><i class="swatch is-rocky" />固态</span>
        <span class="legend-item"><i class="swatch is-gas" />气态巨星</span>
      </template>
      <span v-else class="legend-item"><i class="swatch is-spectrum" />光谱 M → O</span>
      <span class="legend-item"><i class="ring is-planned" />已规划</span>
      <span class="legend-item"><i class="ring is-anchor" />中心 / 交付</span>
      <span v-if="star" class="legend-item"><i class="ring is-selected" />当前选择</span>
      <span class="map-hint">拖动旋转 · 滚轮缩放</span>
    </footer>

    <div v-if="tooltip" class="map-tooltip" :style="{left: tooltip.x + 'px', top: tooltip.y + 'px'}" role="tooltip">
      <strong>{{ tooltip.title }}</strong><span>{{ tooltip.action }}</span>
    </div>
    <div v-if="loading" class="map-state is-loading" role="status"><n-spin size="small" /><span>正在读取星区…</span></div>
    <div v-if="unavailable" class="map-state" role="status">三维视图暂不可用，可在右侧搜索星球查看规划。</div>
  </div>
</template>

<style scoped lang="scss">
.star-map {
  // 星图语义色：固态 / 气态 / 轨道只在本组件使用，规划与选中沿用全局品牌色与成功色。
  --map-rocky: var(--ds-text-muted);
  --map-gas: #a87b45;
  --map-orbit: #9c9ca2;
  --map-core: rgb(255 255 255 / 0.7);
  --map-chip: color-mix(in srgb, var(--ds-bg-panel) 88%, transparent);
  --map-halo: var(--ds-bg-panel);
  --map-spectrum: linear-gradient(90deg, #af5245, #bc733c, #ae842e, #89845e, #678397, #497ca9, #41649d);
  position: relative;
  min-width: 0;
  min-height: 260px;
  height: 100%;
  overflow: hidden;
  container-type: inline-size;
  user-select: none;
  background: radial-gradient(ellipse 72% 64% at 50% 46%, var(--map-core), transparent 76%), var(--ds-cosmos-stars), var(--ds-bg-panel);
}
:global(html[data-theme='dark']) .star-map {
  --map-rocky: #a3abc4;
  --map-gas: #e3b583;
  --map-orbit: #56658a;
  --map-core: rgb(22 48 94 / 0.32);
  --map-spectrum: linear-gradient(90deg, #ed947c, #f4b879, #eacb84, #dddab7, #c1d9ed, #91bee6, #82a0ec);
}
.star-map :deep(canvas) { display: block; width: 100%; height: 100%; outline: none; cursor: grab; }
.star-map :deep(canvas:active) { cursor: grabbing; }
.star-map.is-pointing :deep(canvas) { cursor: pointer; }

// ---------- 状态标记与标签 ----------
// 风格对齐「量化与蓝图」：2px 圆角、括角取代整圈描边、数字用 Oxanium；标签为无底色文字加光晕，减少遮挡与视觉噪声。
.map-overlay { position: absolute; inset: 0; pointer-events: none; }
.map-marker {
  --reticle: var(--ds-brand);
  position: absolute;
  translate: -50% -50%;
  box-sizing: border-box;
  border-radius: 50%;
  &.is-planned { border: 1.5px solid var(--ds-success); }
  &.is-anchor { border: 1.5px solid var(--ds-brand); border-radius: 2px; rotate: 45deg; scale: .86; }
  &.is-hovered { border: 1px dashed var(--ds-text-secondary); }
  // 选中：四角准星括线，与量化页括角卡片同一语言；内圈细环标示星体边界。
  &.is-selected {
    border-radius: 0;
    background:
      linear-gradient(var(--reticle), var(--reticle)) left top / 7px 1.5px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) left top / 1.5px 7px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) right top / 7px 1.5px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) right top / 1.5px 7px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) left bottom / 7px 1.5px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) left bottom / 1.5px 7px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) right bottom / 7px 1.5px no-repeat,
      linear-gradient(var(--reticle), var(--reticle)) right bottom / 1.5px 7px no-repeat;
    padding: 6px;
    box-sizing: content-box;
    &::after { content: ''; position: absolute; inset: 4px; border: 1px solid color-mix(in srgb, var(--reticle) 55%, transparent); border-radius: 50%; }
  }
}
.map-label {
  position: absolute;
  left: 0;
  top: 0;
  height: 20px;
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0 2px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
  // 文字光晕取背景色，任意星点上方都可读，不再需要底板。
  text-shadow: 0 0 2px var(--map-halo), 0 0 4px var(--map-halo), 0 0 6px var(--map-halo);
  will-change: transform;
  &.is-star { color: var(--ds-text-primary); font-weight: 600; }
  &.is-hovered { color: var(--ds-text-primary); }
  &.is-selected { color: var(--ds-text-primary); font-weight: 700; }
}
.label-tag {
  height: 15px;
  box-sizing: border-box;
  padding: 0 4px;
  border-radius: 2px;
  font-size: 10px;
  font-weight: 600;
  line-height: 13px;
  text-shadow: none;
  border: 1px solid var(--ds-brand);
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-brand);
  &.is-planned { border-color: var(--ds-success); color: var(--ds-success); font-family: var(--ds-font-display); }
}

// ---------- 浮层控件 ----------
// 浮层使用切角外框（与量化页面板一致）：外层画边框色，内层 1px 内缩画底色。
.map-heading,.map-legend,.map-tools,.map-tooltip {
  --chamfer: 7px;
  position: absolute;
  isolation: isolate;
  &::before, &::after { content: ''; position: absolute; clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); pointer-events: none; }
  &::before { inset: 0; z-index: -2; background: var(--ds-border-default); }
  &::after { inset: 1px; z-index: -1; background: var(--map-chip); }
}
.map-heading {
  top: 12px;
  left: 12px;
  max-width: calc(100% - 24px);
  height: 32px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px 0 10px;
  font-size: 13px;
  &.has-back { padding-left: 3px; }
  strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
}
.heading-icon { flex: none; color: var(--ds-text-secondary); }
.heading-meta { flex: none; font-size: 12px; color: var(--ds-text-muted); white-space: nowrap; b { margin-right: 2px; color: var(--ds-text-primary); font-family: var(--ds-font-display); font-weight: 600; font-variant-numeric: tabular-nums; } }
.map-tools {
  right: 12px;
  bottom: 12px;
  display: flex;
  flex-direction: column;
  padding: 3px;
  gap: 1px;
  :deep(.n-button) { width: 28px; height: 28px; padding: 0; --n-border-radius: 2px !important; }
}
.map-legend {
  left: 12px;
  bottom: 12px;
  max-width: calc(100% - 80px);
  box-sizing: border-box;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 14px;
  padding: 6px 12px;
  font-size: 12px;
  color: var(--ds-text-secondary);
  pointer-events: none;
}
.legend-item { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
.map-hint { color: var(--ds-text-muted); white-space: nowrap; }
.swatch { width: 8px; height: 8px; border-radius: 50%; &.is-rocky { background: var(--map-rocky); } &.is-gas { width: 11px; height: 11px; background: var(--map-gas); } &.is-spectrum { width: 28px; height: 6px; border-radius: 1px; background: var(--map-spectrum); } }
.ring {
  width: 11px;
  height: 11px;
  box-sizing: border-box;
  border-radius: 50%;
  &.is-planned { border: 1.5px solid var(--ds-success); }
  &.is-anchor { width: 9px; height: 9px; border: 1.5px solid var(--ds-brand); border-radius: 1px; rotate: 45deg; }
  &.is-selected {
    border-radius: 0;
    background:
      linear-gradient(var(--ds-brand), var(--ds-brand)) left top / 4px 1.5px no-repeat, linear-gradient(var(--ds-brand), var(--ds-brand)) left top / 1.5px 4px no-repeat,
      linear-gradient(var(--ds-brand), var(--ds-brand)) right bottom / 4px 1.5px no-repeat, linear-gradient(var(--ds-brand), var(--ds-brand)) right bottom / 1.5px 4px no-repeat,
      linear-gradient(var(--ds-brand), var(--ds-brand)) right top / 4px 1.5px no-repeat, linear-gradient(var(--ds-brand), var(--ds-brand)) right top / 1.5px 4px no-repeat,
      linear-gradient(var(--ds-brand), var(--ds-brand)) left bottom / 4px 1.5px no-repeat, linear-gradient(var(--ds-brand), var(--ds-brand)) left bottom / 1.5px 4px no-repeat;
  }
}
.map-tooltip {
  --chamfer: 6px;
  display: grid;
  gap: 2px;
  max-width: 220px;
  padding: 6px 10px;
  font-size: 12px;
  pointer-events: none;
  &::after { background: var(--ds-bg-panel-elevated); }
  strong { font-weight: 600; }
  span { font-size: 11px; color: var(--ds-text-muted); }
}
.map-state {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 20px;
  text-align: center;
  font-size: 13px;
  color: var(--ds-text-secondary);
  &.is-loading { background: color-mix(in srgb, var(--ds-bg-panel) 62%, transparent); }
}
@container (max-width: 620px) { .map-hint { display: none; } }
@container (max-width: 440px) { .map-legend { gap: 4px 10px; } .heading-meta { display: none; } }
</style>
