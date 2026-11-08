<script setup lang="ts">
import * as THREE from 'three'
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'

// =============== 经纬位置球体选择器 ===============
// 职责：以可转动的三维星球呈现超密度建造位置，悬停纬圈子区高亮，单击选中；使用：参数面板超密度「经纬位置」。
// 交互：左键单击选择；鼠标中键拖动转动球体（无平移、无缩放，滚轮仍滚动面板）；键盘上下键沿纬度切换选项，左右键转动。
// 边界：纬带几何完全来自后端选项（latitudeLow/High + position），缺失几何的旧后端选项不绘制；只负责展示与选择，不推测纬圈规则。
export interface LatitudeGlobeOption {value: string; label: string; tier?: number; position?: string; latitudeLow?: number; latitudeHigh?: number}
const props = withDefaults(defineProps<{options: LatitudeGlobeOption[]; disabled?: boolean; placeholder?: string}>(), {disabled: false, placeholder: ''})
const value = defineModel<string | null>('value')

const host = ref<HTMLDivElement | null>(null)
const hovered = ref<string | null>(null)
const rotating = ref(false)

// 选项按纬带内位置切成三等分子区：南边缘 / 中心 / 北边缘；按纬度由北到南排序供键盘切换。
interface Zone {option: LatitudeGlobeOption; south: number; north: number}
const zones = computed<Zone[]>(() => props.options.flatMap(option => {
  const {latitudeLow: low, latitudeHigh: high, position} = option
  if (typeof low !== 'number' || typeof high !== 'number' || high <= low) return []
  const third = (high - low) / 3
  const south = position === 'south' ? low : position === 'north' ? high - third : low + third
  return [{option, south, north: south + third}]
}).sort((a, b) => b.north - a.north))
const labelOf = (key: string | null | undefined) => props.options.find(option => option.value === key)?.label ?? ''
const readout = computed(() => labelOf(hovered.value) || labelOf(value.value) || props.placeholder)
const previewing = computed(() => !!hovered.value && hovered.value !== value.value)

// ~ 场景常量：半径 1 的星球，透视相机固定距离；只有球体自身旋转，相机不动（即「可转不可拖」）。
const RADIUS = 1
const ZONE_RADIUS = 1.006
const DEG = Math.PI / 180
const TILT_LIMIT = 80 * DEG
const reduceMotion = typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

let renderer: THREE.WebGLRenderer | null = null
let scene: THREE.Scene | null = null
let camera: THREE.PerspectiveCamera | null = null
let globe: THREE.Group | null = null
let baseMaterial: THREE.ShaderMaterial | null = null
let gridMaterial: THREE.LineBasicMaterial | null = null
let bandMaterial: THREE.LineBasicMaterial | null = null
let bandGroup: THREE.Group | null = null
let resizeObserver: ResizeObserver | null = null
let themeObserver: MutationObserver | null = null
let frame = 0
let tilt = 22 * DEG
let spin = -20 * DEG
let dragOrigin: {x: number; y: number; tilt: number; spin: number} | null = null
const raycaster = new THREE.Raycaster()
const pointer = new THREE.Vector2()
const accent = new THREE.Color('#3ee0b0')

// 每个子区一组网格 + 上下边线，透明度按悬停 / 选中状态缓动。
interface ZoneMesh {value: string; fill: THREE.Mesh<THREE.SphereGeometry, THREE.MeshBasicMaterial>; edge: THREE.LineSegments<THREE.BufferGeometry, THREE.LineBasicMaterial>; fillTarget: number; edgeTarget: number}
let zoneMeshes: ZoneMesh[] = []

// 纬度圈顶点：在给定纬度生成一圈闭合线段对。
const ringPoints = (latitude: number, radius: number, segments = 128): THREE.Vector3[] => {
  const points: THREE.Vector3[] = []
  const y = Math.sin(latitude * DEG) * radius
  const r = Math.cos(latitude * DEG) * radius
  for (let i = 0; i < segments; i++) {
    const a = (i / segments) * Math.PI * 2
    const b = ((i + 1) / segments) * Math.PI * 2
    points.push(new THREE.Vector3(Math.cos(a) * r, y, Math.sin(a) * r), new THREE.Vector3(Math.cos(b) * r, y, Math.sin(b) * r))
  }
  return points
}

// 读取主题色：借助 2D 画布把任意 CSS 颜色（含 var 解析后的 rgb 空格语法）归一为 rgba。
const cssColor = (name: string, fallback: string): {color: THREE.Color; alpha: number} => {
  const raw = getComputedStyle(host.value ?? document.documentElement).getPropertyValue(name).trim() || fallback
  const ctx = document.createElement('canvas').getContext('2d')
  if (!ctx) return {color: new THREE.Color(fallback), alpha: 1}
  ctx.fillStyle = fallback
  ctx.fillStyle = raw
  const normalized = String(ctx.fillStyle)
  const match = normalized.match(/rgba?\(([^)]+)\)/)
  if (!match) return {color: new THREE.Color(normalized), alpha: 1}
  const [r, g, b, a] = match[1].split(',').map(part => Number(part.trim()))
  return {color: new THREE.Color(r / 255, g / 255, b / 255), alpha: Number.isFinite(a) ? a : 1}
}

// 主题切换时同步球体、网格与高亮色；高亮使用资源「开启」色（暗色主题为青绿，亮色主题为深青）。
const applyTheme = () => {
  if (!baseMaterial || !gridMaterial || !bandMaterial) return
  const dark = document.documentElement.getAttribute('data-theme') === 'dark'
  const text = cssColor('--ds-text-primary', '#18181a').color
  const muted = cssColor('--ds-text-muted', '#707072').color
  accent.copy(cssColor('--ds-res-on', '#0e9f8e').color)
  baseMaterial.uniforms.uBase.value.copy(dark ? new THREE.Color('#0d1420') : new THREE.Color('#d9d9dc'))
  baseMaterial.uniforms.uRim.value.copy(dark ? accent.clone().lerp(new THREE.Color('#4cc2ff'), .5) : text)
  baseMaterial.uniforms.uBaseAlpha.value = dark ? .55 : .5
  baseMaterial.uniforms.uRimAlpha.value = dark ? .7 : .45
  gridMaterial.color.copy(muted)
  gridMaterial.opacity = dark ? .22 : .2
  bandMaterial.color.copy(dark ? muted : text)
  bandMaterial.opacity = dark ? .5 : .35
  for (const zone of zoneMeshes) {
    zone.fill.material.color.copy(accent)
    zone.edge.material.color.copy(accent)
  }
  render()
}

const render = () => {
  if (!renderer || !scene || !camera || !globe) return
  globe.rotation.set(tilt, spin, 0)
  renderer.render(scene, camera)
}

// 状态 → 目标透明度：可选子区常驻极淡底色，悬停加亮，选中最亮并显示实线边。
const updateTargets = () => {
  for (const zone of zoneMeshes) {
    const selected = zone.value === value.value
    const hover = zone.value === hovered.value
    zone.fillTarget = selected ? .46 : hover ? .26 : .045
    zone.edgeTarget = selected ? 1 : hover ? .7 : 0
  }
  animate()
}

// 透明度缓动：仅在有差值时逐帧推进，收敛后停止循环；减少动态时直接到位。
const animate = () => {
  if (frame) return
  const step = () => {
    frame = 0
    let pending = false
    for (const zone of zoneMeshes) {
      for (const [material, target] of [[zone.fill.material, zone.fillTarget], [zone.edge.material, zone.edgeTarget]] as const) {
        const delta = target - material.opacity
        if (reduceMotion || Math.abs(delta) < .005) material.opacity = target
        else { material.opacity += delta * .28; pending = true }
      }
    }
    render()
    if (pending) frame = requestAnimationFrame(step)
  }
  frame = requestAnimationFrame(step)
}

// 依据后端选项重建子区网格与档位分界圈。
const buildZones = () => {
  if (!globe || !bandGroup) return
  for (const zone of zoneMeshes) {
    globe.remove(zone.fill, zone.edge)
    zone.fill.geometry.dispose(); zone.fill.material.dispose()
    zone.edge.geometry.dispose(); zone.edge.material.dispose()
  }
  zoneMeshes = []
  for (const child of [...bandGroup.children]) { bandGroup.remove(child); (child as THREE.LineSegments).geometry.dispose() }
  const edges = new Set<number>()
  for (const {option, south, north} of zones.value) {
    // SphereGeometry 的 theta 从北极起算：theta = 90° - 纬度。
    const geometry = new THREE.SphereGeometry(ZONE_RADIUS, 96, 6, 0, Math.PI * 2, (90 - north) * DEG, (north - south) * DEG)
    const fill = new THREE.Mesh(geometry, new THREE.MeshBasicMaterial({color: accent, transparent: true, opacity: 0, depthWrite: false, side: THREE.FrontSide}))
    fill.renderOrder = 2
    const edgeGeometry = new THREE.BufferGeometry().setFromPoints([...ringPoints(north, ZONE_RADIUS + .002), ...ringPoints(south, ZONE_RADIUS + .002)])
    const edge = new THREE.LineSegments(edgeGeometry, new THREE.LineBasicMaterial({color: accent, transparent: true, opacity: 0, depthWrite: false}))
    edge.renderOrder = 3
    globe.add(fill, edge)
    zoneMeshes.push({value: option.value, fill, edge, fillTarget: 0, edgeTarget: 0})
    if (typeof option.latitudeLow === 'number') edges.add(Math.round(option.latitudeLow * 100) / 100)
    if (typeof option.latitudeHigh === 'number') edges.add(Math.round(option.latitudeHigh * 100) / 100)
  }
  if (bandMaterial && edges.size) {
    const band = new THREE.LineSegments(new THREE.BufferGeometry().setFromPoints([...edges].flatMap(latitude => ringPoints(latitude, RADIUS + .003))), bandMaterial)
    band.renderOrder = 1
    bandGroup.add(band)
  }
  if (hovered.value && !zoneMeshes.some(zone => zone.value === hovered.value)) hovered.value = null
  applyTheme()
  updateTargets()
}

const init = () => {
  const el = host.value
  if (!el) return
  renderer = new THREE.WebGLRenderer({antialias: true, alpha: true})
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  renderer.setClearColor(0x000000, 0)
  el.prepend(renderer.domElement)
  scene = new THREE.Scene()
  camera = new THREE.PerspectiveCamera(30, 1, .1, 20)
  camera.position.set(0, 0, 4.4)
  globe = new THREE.Group()
  scene.add(globe)

  // 球体本体：菲涅耳边缘光，写入深度以遮挡背面经纬线与子区。
  baseMaterial = new THREE.ShaderMaterial({
    transparent: true,
    uniforms: {uBase: {value: new THREE.Color()}, uRim: {value: new THREE.Color()}, uBaseAlpha: {value: .5}, uRimAlpha: {value: .6}},
    vertexShader: 'varying vec3 vNormal; varying vec3 vView; void main(){ vec4 mv = modelViewMatrix * vec4(position, 1.0); vNormal = normalize(normalMatrix * normal); vView = normalize(-mv.xyz); gl_Position = projectionMatrix * mv; }',
    fragmentShader: 'uniform vec3 uBase; uniform vec3 uRim; uniform float uBaseAlpha; uniform float uRimAlpha; varying vec3 vNormal; varying vec3 vView; void main(){ float f = pow(1.0 - max(dot(normalize(vNormal), normalize(vView)), 0.0), 2.6); gl_FragColor = vec4(mix(uBase, uRim, f), uBaseAlpha + f * uRimAlpha); }',
  })
  const base = new THREE.Mesh(new THREE.SphereGeometry(RADIUS, 64, 48), baseMaterial)
  base.renderOrder = 0
  globe.add(base)

  // 经纬网格：纬线每 10°、经线每 15°。
  gridMaterial = new THREE.LineBasicMaterial({transparent: true, depthWrite: false})
  const gridPoints: THREE.Vector3[] = []
  for (let latitude = -80; latitude <= 80; latitude += 10) gridPoints.push(...ringPoints(latitude, RADIUS + .001, 96))
  for (let longitude = 0; longitude < 360; longitude += 15) {
    for (let latitude = -90; latitude < 90; latitude += 3) {
      const point = (lat: number) => new THREE.Vector3(Math.cos(lat * DEG) * Math.cos(longitude * DEG), Math.sin(lat * DEG), Math.cos(lat * DEG) * Math.sin(longitude * DEG)).multiplyScalar(RADIUS + .001)
      gridPoints.push(point(latitude), point(latitude + 3))
    }
  }
  const grid = new THREE.LineSegments(new THREE.BufferGeometry().setFromPoints(gridPoints), gridMaterial)
  grid.renderOrder = 1
  globe.add(grid)

  bandMaterial = new THREE.LineBasicMaterial({transparent: true, depthWrite: false})
  bandGroup = new THREE.Group()
  globe.add(bandGroup)

  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(el)
  themeObserver = new MutationObserver(applyTheme)
  themeObserver.observe(document.documentElement, {attributes: true, attributeFilter: ['data-theme']})
  resize()
  buildZones()
}

const resize = () => {
  const el = host.value
  if (!el || !renderer || !camera) return
  const width = Math.max(1, el.clientWidth)
  const height = Math.max(1, el.clientHeight)
  renderer.setSize(width, height, false)
  camera.aspect = width / height
  camera.updateProjectionMatrix()
  render()
}

// 拾取：只命中朝向相机的正面子区，背面被球体遮挡的纬圈不可选。
const pick = (event: PointerEvent): string | null => {
  if (!renderer || !camera || !zoneMeshes.length) return null
  const rect = renderer.domElement.getBoundingClientRect()
  pointer.set(((event.clientX - rect.left) / rect.width) * 2 - 1, -((event.clientY - rect.top) / rect.height) * 2 + 1)
  raycaster.setFromCamera(pointer, camera)
  const hit = raycaster.intersectObjects(zoneMeshes.map(zone => zone.fill), false)[0]
  return hit ? zoneMeshes.find(zone => zone.fill === hit.object)?.value ?? null : null
}

const onPointerDown = (event: PointerEvent) => {
  if (props.disabled) return
  if (event.button === 1) {
    // 中键：进入转动；阻止浏览器自动滚动。
    event.preventDefault()
    ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
    dragOrigin = {x: event.clientX, y: event.clientY, tilt, spin}
    rotating.value = true
    hovered.value = null
    updateTargets()
  }
}
const onPointerMove = (event: PointerEvent) => {
  if (props.disabled) return
  if (dragOrigin) {
    spin = dragOrigin.spin + (event.clientX - dragOrigin.x) * .012
    tilt = Math.max(-TILT_LIMIT, Math.min(TILT_LIMIT, dragOrigin.tilt + (event.clientY - dragOrigin.y) * .012))
    render()
    return
  }
  const next = pick(event)
  if (next !== hovered.value) { hovered.value = next; updateTargets() }
}
const onPointerUp = (event: PointerEvent) => {
  if (event.button === 1 && dragOrigin) {
    dragOrigin = null
    rotating.value = false
    ;(event.currentTarget as HTMLElement).releasePointerCapture?.(event.pointerId)
    return
  }
  if (event.button === 0 && !props.disabled) {
    const next = pick(event)
    if (next) value.value = next
  }
}
const onPointerLeave = () => {
  if (dragOrigin) return
  if (hovered.value) { hovered.value = null; updateTargets() }
}
// 中键在 Windows 上会触发自动滚动，mousedown 阶段也需拦截。
const onMouseDown = (event: MouseEvent) => { if (event.button === 1) event.preventDefault() }

// 键盘：上下键按纬度在子区间切换，左右键转动球体。
const onKeyDown = (event: KeyboardEvent) => {
  if (props.disabled) return
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault()
    spin += (event.key === 'ArrowRight' ? 1 : -1) * 15 * DEG
    render()
    return
  }
  if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
  event.preventDefault()
  const list = zones.value
  if (!list.length) return
  const index = list.findIndex(zone => zone.option.value === value.value)
  const next = index < 0 ? Math.floor(list.length / 2) : Math.max(0, Math.min(list.length - 1, index + (event.key === 'ArrowUp' ? -1 : 1)))
  value.value = list[next].option.value
}

watch(zones, buildZones)
watch(value, updateTargets)

onMounted(init)
onBeforeUnmount(() => {
  cancelAnimationFrame(frame)
  resizeObserver?.disconnect()
  themeObserver?.disconnect()
  scene?.traverse(object => {
    const mesh = object as THREE.Mesh
    mesh.geometry?.dispose()
    const material = mesh.material as THREE.Material | THREE.Material[] | undefined
    if (Array.isArray(material)) material.forEach(item => item.dispose())
    else material?.dispose()
  })
  renderer?.dispose()
  renderer?.domElement.remove()
  renderer = null
  scene = null
})
</script>

<template>
  <!-- ~ 经纬位置球体 -->
  <!-- * 说明：内嵌可转动星球，纬圈子区悬停与选中均高亮；下方读数显示悬停预览或当前选中位置 -->
  <!-- * 交互：左键单击选择，中键拖动转动（不可平移/缩放），键盘上下切换、左右转动 -->
  <!-- * 依赖：后端选项提供纬带几何；边界：无选项时球体置灰并显示占位提示 -->
  <div class="latitude-globe ds-bracket" :class="{'is-disabled': disabled, 'is-rotating': rotating, 'is-hovering': !!hovered}">
    <div
      ref="host"
      class="globe-stage"
      tabindex="0"
      role="slider"
      aria-label="超密度建造位置"
      :aria-valuetext="labelOf(value) || placeholder"
      :aria-disabled="disabled"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointerleave="onPointerLeave"
      @mousedown="onMouseDown"
      @auxclick.prevent
      @contextmenu.prevent
      @keydown="onKeyDown"
    >
      <span class="globe-hint" aria-hidden="true">中键转动</span>
    </div>
    <div class="globe-readout" :class="{'is-preview': previewing}">
      <i aria-hidden="true" />
      <span>{{ readout }}</span>
    </div>
  </div>
</template>

<style scoped lang="scss">
.latitude-globe { --bracket-color: var(--ds-border-strong); --bracket-size: 8px; position: relative; display: flex; flex-direction: column; background: var(--ds-hover-bg); border-radius: 2px; transition: opacity 150ms; }
.latitude-globe:hover, .latitude-globe:focus-within { --bracket-color: var(--ds-brand); }
.globe-stage { position: relative; height: 196px; outline: none; cursor: default; touch-action: none; user-select: none; }
.globe-stage :deep(canvas) { display: block; width: 100%; height: 100%; }
.is-hovering .globe-stage { cursor: pointer; }
.is-rotating .globe-stage { cursor: grabbing; }
.globe-stage:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: -1px; }
.globe-hint { position: absolute; top: 6px; right: 8px; color: var(--ds-text-muted); font-size: 9px; letter-spacing: .08em; opacity: .7; pointer-events: none; }
.globe-readout { display: flex; align-items: center; gap: 6px; min-height: 26px; padding: 0 10px; border-top: 1px solid var(--ds-border-subtle); color: var(--ds-text-primary); font-size: 11px; }
.globe-readout i { width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--ds-res-on); box-shadow: var(--ds-res-on-dot-glow); }
.globe-readout.is-preview { color: var(--ds-text-secondary); }
.globe-readout.is-preview i { background: transparent; box-shadow: inset 0 0 0 1px var(--ds-res-on); }
.is-disabled { opacity: .45; pointer-events: none; }
</style>
