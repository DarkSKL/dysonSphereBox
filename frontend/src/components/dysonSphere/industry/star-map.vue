<script setup lang="ts">
import * as THREE from 'three'
import {OrbitControls} from 'three/examples/jsm/controls/OrbitControls.js'
import {celestialMaterial} from './celestial-material'
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch} from 'vue'
import {ArrowLeft, ChevronRight, LocateFixed, Minus, Orbit, Plus} from '@lucide/vue'
import type {industryworkspace} from '../../../../wailsjs/go/models'

// =============== 存档三维星图 ===============
// 说明：银河与恒星内两级浏览共用一个相机；坐标来自同一存档快照，球体大小仅为示意。
// 边界：只展示规划路线和选择星球，不在前端计算产量或修改存档；缓存页停用时停止渲染。
// 星图固定使用深空配色；星体形态和纹理表达类别，选中态只用于当前检查的星球。
const props = defineProps<{
  selection: industryworkspace.Selection
  report: industryworkspace.Report | null
  selected: number | null
  loading?: boolean
  inspector?: HTMLElement | null
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
  stellar?: {kind: number; spectrum: number; starId: number}
}
interface MapBody {
  id: number
  point: THREE.Vector3
  radius: number
  hover: string
  label: string
  priority: number
  width: number
  selected: boolean
  star: boolean
}
interface ScreenBody {body: MapBody; x: number; y: number; r: number}
interface MapLabel {id: number; text: string; x: number; y: number; selected: boolean; hovered: boolean; star: boolean}
interface MapMarker {id: number; x: number; y: number; size: number; selected: boolean; hovered: boolean}
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
const pointing = computed(() => hoveredId.value != null && (!star.value || hoveredId.value > 0))

let renderer: THREE.WebGLRenderer | null = null, camera: THREE.PerspectiveCamera | null = null, controls: OrbitControls | null = null
let observer: ResizeObserver | null = null, frame = 0, active = true
let viewportWidth = 0, viewportHeight = 0
let glowTexture: THREE.CanvasTexture | null = null
const scene = new THREE.Scene(), bodies = new THREE.Group(), lines = new THREE.Group()
const focusCenter = new THREE.Vector3()
let mapBodies: MapBody[] = [], extent = 70, dragStart = {x: 0, y: 0}
let rightPress: {id: number; x: number; y: number; moved: boolean} | null = null
const CLICK_SLOP = 5
const motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)')
const measure = document.createElement('canvas').getContext('2d')
let labelFont = 'sans-serif'
const LABEL_HEIGHT = 20, SAFE_TOP = 52, SAFE_BOTTOM = 48, ORDINARY_LABELS = 16
const OVERLAY_EDGE = 8, INSPECTOR_GAP = 5
// 标签方位记忆与交互状态：拖动、阻尼惯性期间冻结普通标签集合，停止后补齐。
const labelSlots = new Map<number, number>()
let interacting = false, settleTimer: ReturnType<typeof setTimeout> | undefined
// 用户未手动旋转或缩放前，窗口尺寸变化时重新取景，避免窄窗口下边缘星体（如选中的卫星）移出画面。
let framed = true
function startInteract(): void { interacting = true; framed = false; clearTimeout(settleTimer) }
function endInteract(): void {
  clearTimeout(settleTimer)
  settleTimer = setTimeout(() => { interacting = false; invalidate() }, motionPreference.matches ? 0 : 320)
}
function updateMotionPreference(): void {
  if (!controls) return
  controls.enableDamping = !motionPreference.matches
  invalidate()
}

// 恒星依存档类型和温度着色，行星依据编号取配色，仅作为浏览星区的示意。
const starColors = ['#f49b73', '#ffbf84', '#ffe0a4', '#fff2ce', '#d2e8ff', '#a1cfff', '#8eaeff']
const rockyColors = ['#8dafbd', '#92ab9a', '#bf8d71', '#afb6c8', '#ae9dc0']
const gasColors = ['#e0a77e', '#8bb7d9', '#a9c4aa']
const orbitColor = new THREE.Color(), selectedColor = new THREE.Color(), routeColor = new THREE.Color()
function starAppearance(stellar: NonNullable<MapNode['stellar']>) {
  switch (stellar.kind) {
    case 2: return {name: '白矮星', color: '#e7edf8', size: .85}
    case 3: return {name: '中子星', color: '#a5f4ff', size: .8}
    case 4: return {name: '黑洞', color: '#b495fb', size: 1}
    default: return {name: stellar.kind === 1 ? '巨星' : '主序星', color: starColors[stellar.spectrum] ?? '#d5e9ff', size: stellar.kind === 1 ? 1.45 : 1}
  }
}

function glow(): THREE.CanvasTexture {
  if (glowTexture) return glowTexture
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = 128
  const context = canvas.getContext('2d')!
  const gradient = context.createRadialGradient(64, 64, 0, 64, 64, 64)
  gradient.addColorStop(0, 'rgb(255 255 255 / 1)')
  gradient.addColorStop(.19, 'rgb(255 255 255 / .74)')
  gradient.addColorStop(.38, 'rgb(255 255 255 / .23)')
  gradient.addColorStop(.65, 'rgb(255 255 255 / .055)')
  gradient.addColorStop(1, 'rgb(255 255 255 / 0)')
  context.fillStyle = gradient; context.fillRect(0, 0, 128, 128)
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
function measureLabels(): void {
  if (measure) measure.font = '700 12px ' + labelFont
  for (const body of mapBodies) {
    const width = measure?.measureText(body.label).width ?? body.label.length * 12
    body.width = Math.ceil(width) + 6
  }
}

function resetCamera(): void {
  if (!camera || !controls) return
  // 切换星区或复位前消耗旧视角的阻尼余量，以免残余位移带到新视角。
  const damping = controls.enableDamping
  controls.enableDamping = false
  controls.update()
  controls.enableDamping = damping
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
  camera.near = .05; camera.far = Math.max(2000, extent * 20); camera.clearViewOffset()
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
  const system = star.value
  const origin = system?.position ?? {x: 0, y: 0, z: 0}
  const scale = system ? 1 / 1800 : 1 / 2400000
  const siteById = new Map(props.selection.sites.map(s => [s.id, s]))
  const nodes: MapNode[] = system
    ? [
        {id: -system.starId, name: system.name, label: system.name, position: system.position, gas: false, moon: false, stellar: system},
        ...systemSites.value.map(s => ({
          id: s.id, name: s.name, label: s.name.replace(system.name + ' - ', ''), position: s.position, parent: s.parentPlanetId || -system.starId,
          gas: s.gas, moon: s.parentPlanetId > 0,
        })),
      ]
    : props.selection.starSystems.map(s => ({
        id: s.starId, name: s.name, label: s.name, position: s.position, gas: false, moon: false,
        stellar: s,
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
    const appearance = node.stellar && starAppearance(node.stellar)
    const selected = !!system && node.id === props.selected
    const radius = system
      ? extent * (appearance ? .06 : node.gas ? .036 : node.moon ? .016 : .024)
      : Math.max(.19, extent * .0085) * (appearance?.size ?? 1)
    const palette = node.gas ? gasColors : rockyColors
    const tint = new THREE.Color(appearance?.color ?? palette[node.id % palette.length])
    const surface = appearance ? (node.stellar?.kind === 4 ? 'blackHole' : 'star') : node.gas ? 'gas' : 'rocky'
    const light = point.clone().negate().normalize().add(new THREE.Vector3(-.45, .6, .7)).normalize()
    const mesh = new THREE.Mesh(new THREE.SphereGeometry(radius, 28, 20), celestialMaterial(surface, tint, node.id, light))
    mesh.position.copy(point); bodies.add(mesh)
    if (appearance) {
      const halo = new THREE.Sprite(new THREE.SpriteMaterial({map: glow(), color: tint, transparent: true, depthWrite: false,
        opacity: surface === 'blackHole' ? .9 : .72, blending: THREE.AdditiveBlending}))
      const size = radius * (system ? 7 : 8.5)
      halo.position.copy(point); halo.scale.set(size, size, 1); bodies.add(halo)
    }
    const priority = selected ? 100 : appearance && (system || node.stellar?.starId === 1) ? 50 : (node.stellar?.kind ?? 0) > 0 ? 40 : system ? 30 : 10
    mapBodies.push({
      id: node.id, point, radius, label: node.label, priority, width: 0,
      hover: appearance ? `${node.name} · ${appearance.name}` : `${node.name}${node.gas ? ' · 气态巨星' : node.moon ? ' · 卫星' : ' · 固态星球'}`,
      selected, star: !!appearance,
    })
  }
  measureLabels()
  if (system) {
    for (const node of nodes) {
      const parent = node.parent == null ? undefined : positions.get(node.parent), child = positions.get(node.id)
      if (!parent || !child) continue
      const points = orbit(parent, child)
      if (!points.length) continue
      const focused = node.id === props.selected
      const opacity = focused ? .75 : node.moon ? .29 : .43
      lines.add(new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(points),
        new THREE.LineBasicMaterial({color: focused ? selectedColor : orbitColor, transparent: true, opacity, depthWrite: false})))
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
    lines.add(new THREE.Line(geometry, new THREE.LineBasicMaterial({color: focused ? selectedColor : routeColor, transparent: true, opacity: focused ? .9 : .45})))
  }
  if (reset) resetCamera()
  invalidate()
}
function enterSystem(id: number | null): void {
  starId.value = id; hoveredId.value = null; tooltip.value = null
  rightPress = null
  labelSlots.clear(); visibleLabels.value = []
  rebuild(true)
}

function toScreen(body: MapBody, width: number, height: number, margin = 40): ScreenBody | null {
  if (!camera) return null
  const p = body.point.clone().project(camera)
  if (p.z > 1 || p.z < -1) return null
  const x = (p.x + 1) * width / 2, y = (1 - p.y) * height / 2
  if (x < -margin || x > width + margin || y < -margin || y > height + margin) return null
  const distance = Math.max(.001, -body.point.clone().applyMatrix4(camera.matrixWorldInverse).z)
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
// =============== 星图鼠标交互 ===============
// 说明：只从画布接收右键返回手势。记录全过程的位移，拖离起点再移回来也仍然算拖动。
function pointerDown(event: PointerEvent): void {
  if (event.target !== renderer?.domElement) return
  if (event.button === 0) dragStart = {x: event.clientX, y: event.clientY}
  if (event.button !== 2) return
  if (starId.value == null) {
    event.stopPropagation()
    return
  }
  rightPress = {id: event.pointerId, x: event.clientX, y: event.clientY, moved: false}
}
function pointerMove(event: PointerEvent): void {
  if (rightPress?.id === event.pointerId && Math.hypot(event.clientX - rightPress.x, event.clientY - rightPress.y) > CLICK_SLOP) {
    rightPress.moved = true
  }
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
    x: Math.max(8, Math.min(x + 14, rect.width - 228)), y: Math.max(8, Math.min(y + 16, rect.height - 60)),
  }
}
function pointerLeave(): void {
  tooltip.value = null
  if (hoveredId.value != null) { hoveredId.value = null; invalidate() }
}
function pointerUp(event: PointerEvent): void {
  if (event.button === 2) {
    const press = rightPress
    rightPress = null
    if (press?.id === event.pointerId && !press.moved && Math.hypot(event.clientX - press.x, event.clientY - press.y) <= CLICK_SLOP) {
      enterSystem(null)
    }
    return
  }
  if (event.button !== 0 || Math.hypot(event.clientX - dragStart.x, event.clientY - dragStart.y) > CLICK_SLOP) return
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
function inspectorRect(): Rect | null {
  const panel = props.inspector?.getBoundingClientRect()
  const viewport = panel && host.value?.getBoundingClientRect()
  return panel && viewport ? {x: panel.left - viewport.left, y: panel.top - viewport.top, w: panel.width, h: panel.height} : null
}
function labelPositions(s: ScreenBody): Rect[] {
  const w = s.body.width, h = LABEL_HEIGHT, gap = Math.max(3, s.r) + 8
  return [
    {x: s.x + gap, y: s.y - h / 2, w, h}, {x: s.x - gap - w, y: s.y - h / 2, w, h},
    {x: s.x - w / 2, y: s.y - gap - h, w, h}, {x: s.x - w / 2, y: s.y + gap, w, h},
    {x: s.x + gap, y: s.y - gap - h, w, h}, {x: s.x - gap - w, y: s.y - gap - h, w, h},
    {x: s.x + gap, y: s.y + gap, w, h}, {x: s.x - gap - w, y: s.y + gap, w, h},
  ]
}
// 标签先贴近星体放置，遇到相邻星球时使用对角位置，且每帧检查原位置是否仍无遮挡。
function layoutOverlay(width: number, height: number): void {
  const occupied = inspectorRect()
  const screen = mapBodies.flatMap(body => toScreen(body, width, height) ?? [])
  const hovered = hoveredId.value
  markers.value = screen
    .filter(s => s.body.selected || s.body.id === hovered)
    .map(s => ({id: s.body.id, x: s.x, y: s.y, size: Math.round(Math.max(16, s.r * 2 + 10)), selected: s.body.selected, hovered: s.body.id === hovered}))
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
    const slots = labelPositions(s)
    const fits = (r: Rect) => r.x >= OVERLAY_EDGE && r.x + r.w <= width - OVERLAY_EDGE && r.y >= SAFE_TOP && r.y + r.h <= height - SAFE_BOTTOM
      && (!occupied || !overlaps(occupied, r, INSPECTOR_GAP)) && !placed.some(old => overlaps(old, r, 4))
    const previous = labelSlots.get(id)
    const clear = (r: Rect) => fits(r) && !screen.some(o => o !== s && hitsBody(o, r))
    let index = was && previous != null && clear(slots[previous]!) ? previous : slots.findIndex(clear)
    if (index < 0 && s.body.selected) index = slots.findIndex(fits)
    if (index < 0) { labelSlots.delete(id); continue }
    const spot = slots[index]!
    labelSlots.set(id, index)
    if (priority <= 10) ordinary++
    placed.push(spot)
    labels.push({id, text: s.body.label, x: Math.round(spot.x), y: Math.round(spot.y), selected: s.body.selected, hovered: id === hovered, star: s.body.star})
  }
  const kept = new Set(labels.map(label => label.id))
  for (const id of [...labelSlots.keys()]) if (!kept.has(id)) labelSlots.delete(id)
  visibleLabels.value = labels
}
// =============== 画布或浮窗尺寸变化时保留选星 ===============
// 投影平移量只取能容纳括角和任一标签方位的最小值，保留相机的旋转、焦点和缩放。
function keepSelectionInView(selected: MapBody, width: number, height: number): void {
  const screen = toScreen(selected, width, height, Infinity)
  if (!camera || !screen) return
  const {x, y, r} = screen
  const half = Math.round(Math.max(16, r * 2 + 10)) / 2 + 4
  const marker: Rect = {x: x - half, y: y - half, w: half * 2, h: half * 2}
  const occupied = inspectorRect()
  let best: {dx: number; dy: number; distance: number} | undefined

  for (const label of labelPositions(screen)) {
    // 预留一像素，吸收投影和标签坐标取整的误差。
    const minX = OVERLAY_EDGE + 1 - Math.min(marker.x, label.x)
    const maxX = width - OVERLAY_EDGE - 1 - Math.max(marker.x + marker.w, label.x + label.w)
    const minY = SAFE_TOP + 1 - Math.min(marker.y, label.y)
    const maxY = height - SAFE_BOTTOM - 1 - Math.max(marker.y + marker.h, label.y + label.h)
    if (minX > maxX || minY > maxY) continue
    const xs = [THREE.MathUtils.clamp(0, minX, maxX)]
    const ys = [THREE.MathUtils.clamp(0, minY, maxY)]
    if (occupied) {
      // 最近的可行位置只可能在画布边界内的最近点，或贴着浮窗的某条边。
      const gap = INSPECTOR_GAP + 1
      for (const rect of [marker, label]) {
        xs.push(occupied.x - gap - rect.x - rect.w, occupied.x + occupied.w + gap - rect.x)
        ys.push(occupied.y - gap - rect.y - rect.h, occupied.y + occupied.h + gap - rect.y)
      }
    }
    for (const dx of xs) {
      if (dx < minX || dx > maxX) continue
      for (const dy of ys) {
        if (dy < minY || dy > maxY) continue
        if (occupied && [marker, label].some(rect => overlaps(occupied, {...rect, x: rect.x + dx, y: rect.y + dy}, INSPECTOR_GAP))) continue
        const distance = dx * dx + dy * dy
        if (!distance) return
        if (!best || distance < best.distance) best = {dx, dy, distance}
      }
    }
  }
  if (best) camera.setViewOffset(width, height, -best.dx, -best.dy, width, height)
}
function resize(inspectorChanged = false): void {
  if (!host.value || !renderer || !camera) return
  const {clientWidth: width, clientHeight: height} = host.value
  const viewportChanged = width !== viewportWidth || height !== viewportHeight
  if (!width || !height || (!viewportChanged && !inspectorChanged)) { invalidate(); return }

  camera.updateMatrixWorld()
  const selected = mapBodies.find(body => body.selected)
  const before = selected && viewportWidth && viewportHeight ? toScreen(selected, viewportWidth, viewportHeight, 0) : null
  if (viewportChanged) {
    viewportWidth = width; viewportHeight = height
    renderer.setSize(width, height, false)
    camera.aspect = width / height
  }
  // 从无偏移投影重新求最小位移，收起时可撤回展开产生的偏移。浮窗变动沿用当前相机姿态。
  if (viewportChanged || before) camera.clearViewOffset()
  if (viewportChanged && framed && mapBodies.length) resetCamera()
  if (before) {
    camera.updateMatrixWorld()
    keepSelectionInView(before.body, width, height)
  }
  invalidate()
}
onMounted(() => {
  if (!host.value) return
  try { renderer = new THREE.WebGLRenderer({alpha: true, antialias: true}) } catch { unavailable.value = true; return }
  const style = getComputedStyle(host.value)
  labelFont = style.fontFamily
  orbitColor.setStyle(style.getPropertyValue('--ds-starmap-orbit').trim())
  selectedColor.setStyle(style.getPropertyValue('--ds-starmap-focus').trim())
  routeColor.setStyle(style.getPropertyValue('--ds-starmap-route').trim())
  void document.fonts.ready.then(() => { measureLabels(); invalidate() })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2)); host.value.prepend(renderer.domElement)
  camera = new THREE.PerspectiveCamera(45, 1, .05, 2000)
  controls = new OrbitControls(camera, renderer.domElement); controls.dampingFactor = .12
  updateMotionPreference()
  motionPreference.addEventListener('change', updateMotionPreference)
  controls.addEventListener('change', invalidate)
  controls.addEventListener('start', startInteract); controls.addEventListener('end', endInteract)
  scene.add(lines, bodies); resize(); rebuild(true)
  // 按浮窗实际外边界避让，过渡中的尺寸变化和无过渡的开合都由同一监听处理。
  observer = new ResizeObserver(entries => resize(entries.some(entry => entry.target === props.inspector)))
  observer.observe(host.value)
  if (props.inspector) observer.observe(props.inspector, {box: 'border-box'})
  invalidate()
})
watch(() => props.inspector, (element, previous) => {
  if (previous) observer?.unobserve(previous)
  if (element) observer?.observe(element, {box: 'border-box'})
  else resize(true)
})
watch(() => [props.report, props.selected], () => rebuild())
watch(() => props.selection.snapshotId, () => enterSystem(null))
watch(() => props.selected, id => {
  if (id == null) return
  const selected = props.selection.sites.find(s => s.id === id)
  if (selected && starId.value !== selected.starId) enterSystem(selected.starId)
})
onActivated(() => { active = true; invalidate() })
onDeactivated(() => { active = false; cancelAnimationFrame(frame); frame = 0 })
onBeforeUnmount(() => {
  active = false; cancelAnimationFrame(frame); clearTimeout(settleTimer); observer?.disconnect()
  motionPreference.removeEventListener('change', updateMotionPreference)
  controls?.removeEventListener('change', invalidate); controls?.removeEventListener('start', startInteract); controls?.removeEventListener('end', endInteract); controls?.dispose()
  disposeGroup(bodies); disposeGroup(lines); glowTexture?.dispose(); renderer?.dispose(); renderer?.domElement.remove()
})
</script>

<template>
  <!-- ~ 星图画布：左键进入恒星系或选择星球，恒星系内单击右键返回银河。 -->
  <div ref="host" class="star-map" :class="{'is-pointing': pointing}" @contextmenu.prevent @pointerdown.capture="pointerDown" @pointerup="pointerUp" @pointermove="pointerMove" @pointerleave="pointerLeave" @pointercancel="rightPress = null">
    <div class="map-overlay" aria-hidden="true">
      <span v-for="m in markers" :key="'m' + m.id" class="map-marker"
        :class="{'is-selected': m.selected, 'ds-bracket': m.selected, 'is-hovered': m.hovered && !m.selected}"
        :style="{left: m.x + 'px', top: m.y + 'px', width: m.size + 'px', height: m.size + 'px'}" />
      <div v-for="label in visibleLabels" :key="label.id" class="map-label" :class="{'is-selected': label.selected, 'is-hovered': label.hovered, 'is-star': label.star}"
        :style="{transform: `translate(${label.x}px, ${label.y}px)`}">
        {{ label.text }}
      </div>
    </div>

    <nav class="map-heading map-hud ds-bracket" :class="{'has-back': star}" aria-label="星图位置" @pointerdown.stop @pointerup.stop>
      <n-button v-if="star" size="tiny" quaternary aria-label="返回银河星图" @click="enterSystem(null)"><template #icon><ArrowLeft :size="14" /></template>银河</n-button>
      <Orbit v-else :size="15" class="heading-icon" aria-hidden="true" />
      <ChevronRight v-if="star" :size="12" class="heading-divider" aria-hidden="true" />
      <strong aria-current="location" :title="star?.name ?? '银河星图'">{{ star?.name ?? '银河星图' }}</strong>
      <span class="heading-meta"><b>{{ star ? systemSites.length : selection.stars }}</b>{{ star ? " 颗星球" : " 个恒星系" }}</span>
    </nav>

    <div class="map-tools map-hud ds-bracket" role="toolbar" aria-label="星图视角" @pointerdown.stop @pointerup.stop>
      <n-tooltip placement="left"><template #trigger><n-button size="small" quaternary aria-label="放大" @click="zoom(.72)"><template #icon><Plus :size="15" /></template></n-button></template>放大</n-tooltip>
      <n-tooltip placement="left"><template #trigger><n-button size="small" quaternary aria-label="缩小" @click="zoom(1.38)"><template #icon><Minus :size="15" /></template></n-button></template>缩小</n-tooltip>
      <n-tooltip placement="left"><template #trigger><n-button size="small" quaternary aria-label="重置星图视角" @click="resetCamera"><template #icon><LocateFixed :size="15" /></template></n-button></template>重置视角</n-tooltip>
    </div>

    <p class="map-hint"><span class="map-hint-camera">拖动旋转 · 滚轮缩放</span><span v-if="star" class="map-hint-back">右键返回银河</span></p>

    <div v-if="tooltip" class="map-tooltip map-hud" :style="{left: tooltip.x + 'px', top: tooltip.y + 'px'}" role="tooltip">
      <strong>{{ tooltip.title }}</strong><span>{{ tooltip.action }}</span>
    </div>
    <div v-if="loading" class="map-state is-loading" role="status"><n-spin size="small" /><span>正在读取星区…</span></div>
    <div v-if="unavailable" class="map-state" role="status">三维视图暂不可用，可在检查面板中搜索星球。</div>
  </div>
</template>

<style scoped lang="scss">
.star-map {
  // 深空底色独立于日夜主题；贴在画布上的文字用场景色，控件沿用应用主题。
  --ds-starmap-space: #03060d;
  --ds-starmap-nebula: #213b5c;
  --ds-starmap-text: #eaf5ff;
  --ds-starmap-muted: #b3c4d6;
  --ds-starmap-focus: #a9e6ff;
  --ds-starmap-orbit: #6484a7;
  --ds-starmap-route: #6798c6;
  --ds-starmap-warm: #f7d2ad;
  position: relative;
  min-width: 0;
  min-height: 260px;
  height: 100%;
  overflow: hidden;
  container-type: inline-size;
  user-select: none;
  color: var(--ds-starmap-text);
  background:
    radial-gradient(circle at 19px 37px, color-mix(in srgb, var(--ds-starmap-muted) 28%, transparent) .45px, transparent 1px) 0 0 / 83px 107px,
    radial-gradient(circle at 41px 91px, color-mix(in srgb, var(--ds-starmap-orbit) 36%, transparent) .5px, transparent 1px) 0 0 / 107px 123px,
    radial-gradient(circle at 79px 17px, color-mix(in srgb, var(--ds-starmap-focus) 42%, transparent) .55px, transparent 1.2px) 0 0 / 151px 139px,
    radial-gradient(circle at 51px 121px, color-mix(in srgb, var(--ds-starmap-text) 57%, transparent) .7px, transparent 1.8px) 0 0 / 227px 163px,
    radial-gradient(circle at 142px 49px, color-mix(in srgb, var(--ds-starmap-warm) 45%, transparent) .65px, transparent 1.5px) 0 0 / 251px 211px,
    radial-gradient(ellipse at 33% 57%, color-mix(in srgb, var(--ds-starmap-nebula) 56%, transparent), transparent 63%),
    radial-gradient(ellipse at 77% 21%, color-mix(in srgb, var(--ds-starmap-nebula) 36%, transparent), transparent 52%),
    var(--ds-starmap-space);
}
.star-map :deep(canvas) { position: absolute; inset: 0; display: block; width: 100%; height: 100%; outline: none; cursor: grab; }
.star-map :deep(canvas:active) { cursor: grabbing; }
.star-map.is-pointing :deep(canvas) { cursor: pointer; }

// ---------- 星体名称与选中态 ----------
.map-overlay { position: absolute; inset: 0; pointer-events: none; }
.map-marker {
  position: absolute;
  translate: -50% -50%;
  box-sizing: border-box;
  border-radius: 50%;
  &.is-hovered { border: 1px solid color-mix(in srgb, var(--ds-starmap-focus) 58%, transparent); }
  &.is-selected {
    --bracket-color: var(--ds-starmap-focus);
    --bracket-size: 8px;
    border: 1px solid color-mix(in srgb, var(--ds-starmap-focus) 55%, transparent);
    box-shadow: 0 0 10px color-mix(in srgb, var(--ds-starmap-focus) 32%, transparent);
    &::after { inset: -4px; border-radius: 0; animation: none; }
  }
}
.map-label {
  position: absolute;
  left: 0;
  top: 0;
  height: 20px;
  padding: 0 2px;
  color: var(--ds-starmap-muted);
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
  text-shadow: 0 1px 2px var(--ds-starmap-space), 0 0 5px var(--ds-starmap-space), 0 0 9px var(--ds-starmap-space);
  will-change: transform;
  &.is-star,&.is-hovered { color: var(--ds-starmap-text); }
  &.is-selected { color: var(--ds-starmap-focus); font-weight: 700; }
}

// ---------- 视角控件 ----------
.map-heading,.map-tools,.map-tooltip { position: absolute; }
.map-heading {
  top: 12px;
  left: 12px;
  max-width: calc(100% - var(--map-inspector-width, 0px) - 36px);
  height: 36px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  font-size: 13px;
  &.has-back { padding-left: 4px; }
  strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
}
.heading-icon { flex: none; color: var(--ds-brand); }
.heading-divider { flex: none; color: var(--ds-text-muted); }
.heading-meta { flex: none; padding-left: 8px; border-left: 1px solid var(--ds-border-default); font-size: 12px; color: var(--ds-text-secondary); white-space: nowrap; b { margin-right: 2px; color: var(--ds-text-primary); font-family: var(--ds-font-display); font-weight: 600; font-variant-numeric: tabular-nums; } }
.map-tools {
  right: 12px;
  bottom: 12px;
  display: flex;
  padding: 1px;
  gap: 1px;
  :deep(.n-button) { width: 32px; height: 30px; padding: 0; }
  :deep(.n-button + .n-button) { border-left: 1px solid var(--ds-border-subtle); }
}
.map-hint {
  position: absolute;
  left: 12px;
  bottom: 12px;
  display: flex;
  align-items: center;
  min-height: 34px;
  max-width: calc(100% - 130px);
  margin: 0;
  font-size: 12px;
  color: var(--ds-starmap-muted);
  text-shadow: 0 0 6px var(--ds-starmap-space);
  pointer-events: none;
}
.map-hint-camera + .map-hint-back { margin-left: 8px; padding-left: 8px; border-left: 1px solid var(--ds-starmap-orbit); }
.map-hint-back { color: var(--ds-starmap-focus); }
.map-tooltip {
  display: grid;
  gap: 2px;
  max-width: min(220px, calc(100% - 16px));
  padding: 6px 10px;
  font-size: 12px;
  pointer-events: none;
  strong { font-weight: 600; }
  span { font-size: 11px; color: var(--ds-text-secondary); }
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
  color: var(--ds-starmap-text);
  background: color-mix(in srgb, var(--ds-starmap-space) 78%, transparent);
}
@container (max-width: 520px) { .map-heading { max-width: calc(100% - 24px); } }
@container (max-width: 370px) {
  .map-hint-camera { display: none; }
  .map-hint-camera + .map-hint-back { margin-left: 0; padding-left: 0; border: 0; }
}
@container (max-width: 220px) { .heading-meta { display: none; } }
</style>
