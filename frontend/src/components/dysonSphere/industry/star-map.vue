<script setup lang="ts">
import * as THREE from 'three'
import {OrbitControls} from 'three/examples/jsm/controls/OrbitControls.js'
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch} from 'vue'
import {ArrowLeft, LocateFixed, Orbit} from '@lucide/vue'
import type {industryworkspace} from '../../../../wailsjs/go/models'

// =============== 存档三维星图 ===============
// 说明：银河与恒星内两级浏览共用一个相机；坐标来自同一存档快照，球体大小仅用于拾取。
// 边界：只展示规划路线和选择星球，不在前端计算产量或修改存档；缓存页停用时停止渲染。
const props = defineProps<{selection: industryworkspace.Selection; report: industryworkspace.Report | null; selected: number | null}>()
const emit = defineEmits<{(e: 'select', id: number): void}>()
const host = ref<HTMLDivElement | null>(null)
const starId = ref<number | null>(null)
const hovered = ref(''), unavailable = ref(false)
const star = computed(() => props.selection.starSystems.find(s => s.starId === starId.value))
const systemSites = computed(() => props.selection.sites.filter(s => s.starId === starId.value))
const used = computed(() => new Set(props.report?.planets.map(p => p.id) ?? []))
let renderer: THREE.WebGLRenderer | null = null, camera: THREE.PerspectiveCamera | null = null, controls: OrbitControls | null = null
let observer: ResizeObserver | null = null, theme: MutationObserver | null = null, frame = 0, active = true
const scene = new THREE.Scene(), bodies = new THREE.Group(), lines = new THREE.Group()
const raycaster = new THREE.Raycaster(), pointer = new THREE.Vector2()
const focusCenter = new THREE.Vector3()
let picks: THREE.Mesh[] = [], extent = 70, dragStart = {x: 0, y: 0}
let labels: {text: string; point: THREE.Vector3; selected: boolean; id: number}[] = []
const visibleLabels = ref<{text: string; x: number; y: number; selected: boolean; id: number}[]>([])

interface MapNode {
  id: number
  name: string
  position: {x: number; y: number; z: number}
  participating: boolean
  gas: boolean
  stellar?: {kind: number; spectrum: number; starId: number}
}

// 图示按存档恒星类型和光谱着色；规划参与情况由外圈表示。
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

// 从当前页面主题读取实际色值，避免画布与日夜样式分离。
function color(token: string): THREE.Color {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(token).trim()
  const helper = document.createElement('span')
  helper.style.color = raw; host.value?.appendChild(helper)
  const value = getComputedStyle(helper).color
  helper.remove()
  return new THREE.Color(value)
}
function disposeGroup(group: THREE.Group): void {
  for (const object of [...group.children]) {
    if (object instanceof THREE.Mesh || object instanceof THREE.Line) {
      object.geometry.dispose()
      const materials = Array.isArray(object.material) ? object.material : [object.material]
      materials.forEach(m => m.dispose())
    }
    group.remove(object)
  }
}
function resetCamera(): void {
  if (!camera || !controls) return
  controls.target.copy(focusCenter)
  const direction = new THREE.Vector3(.8, 1, 1.15).normalize()
  const right = camera.up.clone().cross(direction).normalize()
  const up = direction.clone().cross(right)
  const halfHeight = Math.tan(THREE.MathUtils.degToRad(camera.fov / 2))
  let distance = 4
  for (const body of picks) {
    const point = body.position.clone().sub(focusCenter)
    const fit = Math.max(Math.abs(point.dot(right)) / camera.aspect, Math.abs(point.dot(up))) / halfHeight
    distance = Math.max(distance, point.dot(direction) + fit * 1.22)
  }
  camera.position.copy(focusCenter).addScaledVector(direction, distance)
  camera.near = .05; camera.far = Math.max(2000, extent * 20); camera.updateProjectionMatrix()
  controls.minDistance = 3; controls.maxDistance = Math.max(extent * 6, distance * 2); controls.update()
  invalidate()
}
function rebuild(reset = false): void {
  if (!renderer || !camera) return
  disposeGroup(bodies); disposeGroup(lines); picks = []; labels = []
  const ordinary = color('--ds-text-muted'), accent = color('--ds-brand'), selected = color('--ds-success')
  const origin = star.value?.position ?? {x: 0, y: 0, z: 0}
  const scale = star.value ? 1 / 1800 : 1 / 2400000
  const positions = new Map<number, THREE.Vector3>()
  const usedStars = new Set(props.selection.sites.filter(s => used.value.has(s.id)).map(s => s.starId))
  const dark = document.documentElement.dataset.theme === 'dark'
  const nodes: MapNode[] = star.value
    ? [{id: -star.value.starId, name: star.value.name, position: star.value.position, participating: usedStars.has(star.value.starId), gas: false, stellar: star.value}, ...systemSites.value.map(s => ({...s, participating: used.value.has(s.id)}))]
    : props.selection.starSystems.map(s => ({id: s.starId, name: s.name, position: s.position, participating: usedStars.has(s.starId), gas: false, stellar: s}))
  for (const node of nodes) {
    const point = new THREE.Vector3(node.position.x - origin.x, node.position.y - origin.y, node.position.z - origin.z).multiplyScalar(scale)
    positions.set(node.id, point)
  }
  const bounds = new THREE.Box3().setFromPoints([...positions.values()])
  bounds.getCenter(focusCenter)
  extent = Math.max(5, bounds.getSize(new THREE.Vector3()).length() / 2)
  for (const node of nodes) {
    const point = positions.get(node.id)!
    const isSelected = !!star.value && node.id === props.selected
    const appearance = node.stellar && starAppearance(node.stellar, dark)
    const bodyColor = isSelected ? selected : appearance?.color ?? (node.participating ? accent : ordinary)
    const radius = star.value ? extent * (node.id < 0 ? .055 : node.gas ? .03 : .022) : Math.max(.19, extent * .0085) * (appearance?.size ?? 1)
    const mesh = new THREE.Mesh(new THREE.SphereGeometry(radius, 16, 12), new THREE.MeshBasicMaterial({color: bodyColor}))
    mesh.position.copy(point); mesh.userData = {id: node.id, name: appearance ? `${node.name} · ${appearance.name}` : node.name}; bodies.add(mesh); picks.push(mesh)
    if (appearance) {
      const halo = new THREE.Mesh(new THREE.SphereGeometry(radius*2.3,16,12), new THREE.MeshBasicMaterial({color:bodyColor,transparent:true,opacity:.12,depthWrite:false}))
      halo.position.copy(point); halo.userData = mesh.userData; bodies.add(halo); picks.push(halo)
    }
    if (star.value || node.participating || node.stellar?.starId === 1 || (node.stellar?.kind ?? 0) > 0 || nodes.length <= 12) {
      const text = star.value && node.id > 0 ? node.name.replace(star.value.name + ' - ', '') : node.name
      labels.push({text, point: point.clone(), selected: isSelected, id: node.id})
    }
    if (isSelected || node.participating) {
      const ring = new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(Array.from({length: 48}, (_, i) => {
        const a = i * Math.PI / 24
        return new THREE.Vector3(Math.cos(a) * radius * 1.75, 0, Math.sin(a) * radius * 1.75).add(point)
      })), new THREE.LineBasicMaterial({color: isSelected ? selected : accent, transparent: true, opacity: .55}))
      bodies.add(ring)
    }
  }
  if (star.value) {
    for (const site of systemSites.value) {
      const parent = positions.get(site.parentPlanetId || -star.value.starId)
      const child = positions.get(site.id)
      if (!parent || !child) continue
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints([parent, child]),
        new THREE.LineDashedMaterial({color: ordinary, transparent: true, opacity: .3, dashSize: extent * .025, gapSize: extent * .016}))
      line.computeLineDistances(); lines.add(line)
    }
  }
  const bySite = new Map(props.selection.sites.map(s => [s.id, s]))
  const seen = new Set<string>()
  for (const route of props.report?.routes ?? []) {
    const from = bySite.get(route.fromPlanetId), to = bySite.get(route.toPlanetId)
    if (!from || !to) continue
    const a = star.value ? from.id : from.starId, b = star.value ? to.id : to.starId
    const key = [a, b].sort((x, y) => x - y).join(':')
    if (a === b || seen.has(key) || !positions.has(a) || !positions.has(b)) continue
    seen.add(key)
    const focused = from.id === props.selected || to.id === props.selected
    const geometry = new THREE.BufferGeometry().setFromPoints([positions.get(a)!, positions.get(b)!])
    lines.add(new THREE.Line(geometry, new THREE.LineBasicMaterial({color: focused ? selected : accent, transparent: true, opacity: focused ? .9 : .25})))
  }
  if (reset) resetCamera()
  invalidate()
}
function enterSystem(id: number | null): void { starId.value = id; hovered.value = ''; rebuild(true) }
function pick(event: PointerEvent): THREE.Mesh | undefined {
  if (!camera || !renderer) return
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1)
  raycaster.setFromCamera(pointer, camera)
  return raycaster.intersectObjects(picks, false)[0]?.object as THREE.Mesh | undefined
}
function pointerUp(event: PointerEvent): void {
  if (event.button !== 0 || Math.hypot(event.clientX - dragStart.x, event.clientY - dragStart.y) > 5) return
  const hit = pick(event)
  if (!hit) return
  const id = Number(hit.userData.id)
  if (starId.value == null) enterSystem(id)
  else if (id > 0) emit('select', id)
}
// 相机和内容发生变化时才绘制。阻尼旋转会继续触发 change，静止时为后台规划留出算力。
function invalidate(): void {
  if (active && !frame) frame = requestAnimationFrame(render)
}
function render(): void {
  frame = 0
  if (!active || !renderer || !camera) return
  controls?.update(); renderer.render(scene, camera)
  const width = host.value?.clientWidth ?? 1, height = host.value?.clientHeight ?? 1
  const placed: {x:number;y:number;width:number}[] = []
  visibleLabels.value = [...labels].sort((a,b)=>Number(b.selected)-Number(a.selected)).flatMap(label => {
    const p = label.point.clone().project(camera!)
    if (p.z > 1 || p.z < -1 || Math.abs(p.x) > .95 || Math.abs(p.y) > .93) return []
    const labelWidth = [...label.text].reduce((size, char)=>size+(char.charCodeAt(0)>255?11:6),0)
    const x = Math.max(10,Math.min(width-labelWidth-10,(p.x+1)*width/2+8))
    const y = Math.max(44,Math.min(height-44,(1-p.y)*height/2+8))
    if (placed.some(old=>Math.abs(y-old.y)<18 && x<old.x+old.width+8 && x+labelWidth+8>old.x)) return []
    placed.push({x,y,width:labelWidth})
    return [{...label,x,y}]
  })
}
function resize(): void {
  if (!host.value || !renderer || !camera) return
  const {clientWidth: width, clientHeight: height} = host.value
  renderer.setSize(width, height); camera.aspect = Math.max(1, width) / Math.max(1, height); camera.updateProjectionMatrix(); invalidate()
}
onMounted(() => {
  if (!host.value) return
  try { renderer = new THREE.WebGLRenderer({alpha: true, antialias: true}) } catch { unavailable.value = true; return }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2)); host.value.prepend(renderer.domElement)
  camera = new THREE.PerspectiveCamera(45, 1, .05, 2000)
  controls = new OrbitControls(camera, renderer.domElement); controls.enableDamping = true; controls.dampingFactor = .12
  controls.addEventListener('change', invalidate)
  scene.add(lines, bodies); resize(); rebuild(true)
  observer = new ResizeObserver(resize); observer.observe(host.value)
  theme = new MutationObserver(() => rebuild()); theme.observe(document.documentElement, {attributes: true, attributeFilter: ['data-theme']})
  invalidate()
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
  active = false; cancelAnimationFrame(frame); observer?.disconnect(); theme?.disconnect(); controls?.removeEventListener('change', invalidate); controls?.dispose()
  disposeGroup(bodies); disposeGroup(lines); renderer?.dispose(); renderer?.domElement.remove()
})
</script>

<template>
  <!-- ~ 星图画布：点击恒星进入，拖动旋转；文字与可访问的星球选择保持在 DOM 中 -->
  <div ref="host" class="star-map" @pointerdown="dragStart = {x:$event.clientX,y:$event.clientY}" @pointerup="pointerUp" @pointermove="hovered = String(pick($event)?.userData.name ?? '')" @pointerleave="hovered = ''">
    <div class="map-heading"><n-button v-if="star" size="tiny" quaternary @click.stop="enterSystem(null)"><template #icon><ArrowLeft :size="14" /></template>银河</n-button><Orbit v-else :size="16" /><strong>{{ star?.name ?? '银河星图' }}</strong><span>{{ star ? systemSites.length + ' 颗星球' : selection.stars + ' 个恒星系' }}</span></div>
    <div v-for="label in visibleLabels" :key="label.id" class="map-label" :class="{'is-selected':label.selected}" :style="{left:label.x+'px',top:label.y+'px'}">{{ label.text }}</div>
    <div class="map-bottom"><span>{{ hovered || '拖动旋转 · 滚轮缩放 · 点击选择' }}</span><n-button size="tiny" quaternary aria-label="重置星图视角" @click.stop="resetCamera"><template #icon><LocateFixed :size="15" /></template></n-button></div>
    <div v-if="unavailable" class="unavailable">三维视图暂不可用，可通过右侧星球列表查看规划。</div>
  </div>
</template>

<style scoped lang="scss">
.star-map {
  --grid: color-mix(in srgb, var(--ds-text-primary) 4%, transparent);
  --grid-major: color-mix(in srgb, var(--ds-text-primary) 7%, transparent);
  position:relative; min-width:0; min-height:260px; height:100%; overflow:hidden; background-color:var(--ds-bg-panel);
  background-image:linear-gradient(to right,var(--grid-major) 1px,transparent 1px),linear-gradient(to bottom,var(--grid-major) 1px,transparent 1px),
    linear-gradient(to right,var(--grid) 1px,transparent 1px),linear-gradient(to bottom,var(--grid) 1px,transparent 1px);
  background-size:100px 100px,100px 100px,20px 20px,20px 20px;
}
.star-map :deep(canvas) { display:block; width:100%; height:100%; outline:none; }
.map-heading,.map-bottom { position:absolute; left:14px; right:14px; display:flex; align-items:center; gap:9px; pointer-events:none; }
.map-heading { top:12px; font-size:13px; } .map-heading span { font-size:11px; color:var(--ds-text-muted); }
.map-bottom { bottom:10px; justify-content:space-between; font-size:11px; color:var(--ds-text-muted); }
.map-heading :deep(button),.map-bottom :deep(button) { pointer-events:auto; }
.map-label { position:absolute; pointer-events:none; font-size:11px; color:var(--ds-text-secondary); text-shadow:0 0 4px var(--ds-bg-panel); white-space:nowrap; }
.map-label.is-selected { color:var(--ds-success); font-weight:700; }
.unavailable { position:absolute; inset:45% 20px auto; text-align:center; color:var(--ds-text-secondary); font-size:13px; }
</style>
