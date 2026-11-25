<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, shallowRef, watch} from 'vue'

import {
  createStarGlows, detailLevel, drawFeedbackStar, drawSeedGlow, drawSkyDust, drawStarFocus, haloScale, starCaptions, starLabel,
} from '../../composables/feedback/galaxy-art'
import {boundedCamera, resolveClick, type Camera, type ProjectedStar, type ViewSize} from '../../composables/feedback/galaxy-camera'
import {cloudExposure, cloudPlan, cloudPresence, easeCloudLevel, edgeTransmission, FLOW_PERIOD_MS} from '../../composables/feedback/galaxy-nebula'
import {useGalaxyTheme} from '../../composables/feedback/useGalaxyTheme'
import {useNebulaSamples} from '../../composables/feedback/useNebulaSamples'
import {createNebulaLayer} from '../../composables/feedback/nebula-renderer'
import type {Coordinates, DisplayStar} from '../../types/feedback'

const props = withDefaults(defineProps<{
  stars: ProjectedStar[]
  sources: DisplayStar[]
  camera: Camera
  arrivals: ReadonlySet<string>
  hovered: string
  selected: string
  active: boolean
  focused?: boolean
  tooltipId?: string
  menuOpen?: boolean
}>(), {focused: false})

const emit = defineEmits<{
  change: [camera: Camera, size: ViewSize]
  zoom: [factor: number]
  home: []
  hover: [point: DisplayStar | null, screen?: Coordinates]
  approach: [point: DisplayStar | null]
  open: [starId: string]
  /** 点击落在拥挤处，最近的几颗星难以分辨：交给页面用「附近的星」列表让用户选。 */
  crowd: [stars: ProjectedStar[]]
  browse: []
  menu: [point: Coordinates]
  dismissMenu: []
}>()

// =============== 绘制生命周期 ===============
// 说明：旋转更新世界投影。空视区绘制静态远景，待相机、尺寸或数据变化时再重绘。
const stage = ref<HTMLElement>()
const canvas = ref<HTMLCanvasElement>()
const dragging = ref(false)
const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
const size = shallowRef<ViewSize>({width: 0, height: 0})
const samples = useNebulaSamples(() => props.sources)
// 最后一颗公开星消失时，采样会先变空；淡出期间继续用上一批非空采样绘制云雾，淡出结束再丢弃。
let fadingSamples: Float32Array | undefined
const theme = useGalaxyTheme()
// 仅绘制按远近排序，键盘列表遵循快照顺序，转动镜头不会打乱同一批反馈。
const drawOrder = computed(() => props.stars.slice().sort((a, b) => a.depth - b.depth))
const detail = computed(() => detailLevel(props.camera.zoom))
const captions = computed(() => starCaptions(props.stars, size.value, detail.value))
const visibleMass = computed(() => {
  const {width, height} = size.value
  return props.stars.reduce((total, {x, y, point}) => {
    if (point.privateLabel) return total
    const edge = edgeTransmission(Math.min(x, y, width - x, height - y), width, height)
    return total + edge * Math.max(1, point.count ?? 1)
  }, 0)
})
let nebula: ReturnType<typeof createNebulaLayer> | undefined
let frame = 0
let flowTimer: number | undefined
let lastFlowFrame: number | undefined
let flowElapsed = 0
let observer: ResizeObserver | undefined
let glows: ReturnType<typeof createStarGlows> | undefined
let glowsMode = ''
// 云雾亮度的缓动状态：数量变化时在约 0.45 秒内趋近目标，避免 0→1→少量→密集之间亮度跳变。减少动态效果时直接到位。
let cloudLevel = -1
let cloudAt = 0
let drag: {id: number; start: Coordinates; camera: Camera; button: number; sawContext: boolean} | undefined
let skipNativeMenu = false
let arrivalsAt = 0

function scheduleDraw(): void {
  if (props.active && !document.hidden && !frame) frame = requestAnimationFrame(draw)
}

// 只累计前台星云实际流动的时间，恢复页面或退出聚焦时从停下的位置继续。
function pauseFlow(): void {
  window.clearTimeout(flowTimer)
  flowTimer = undefined
  lastFlowFrame = undefined
}

function syncMotion(): void {
  pauseFlow()
  if (document.hidden || !props.active) {
    cancelAnimationFrame(frame)
    frame = 0
  } else scheduleDraw()
}

// 视区质量换算为云雾目标亮度：presence 让第一颗星就带出柔和的云，exposure 压低密集时的叠加亮度。
function cloudTarget(): number {
  const mass = visibleMass.value
  return mass > 0 ? cloudPresence(mass) * cloudExposure(mass) : 0
}

function easeCloud(now: number, target: number): boolean {
  const step = easeCloudLevel(cloudLevel, target, now - cloudAt, reduced.matches)
  cloudAt = now
  cloudLevel = step.level
  return step.settling
}

function draw(now: number): void {
  const started = performance.now()
  frame = 0
  window.clearTimeout(flowTimer)
  flowTimer = undefined
  const surface = canvas.value
  const context = surface?.getContext('2d')
  const {width, height} = size.value
  if (!surface || !context || !width || !height || !props.active || document.hidden) { pauseFlow(); return }
  context.resetTransform()
  context.clearRect(0, 0, surface.width, surface.height)
  context.setTransform(surface.width / width, 0, 0, surface.height / height, 0, 0)
  const palette = theme.value
  if (glowsMode !== palette.mode) { glows = createStarGlows(palette); glowsMode = palette.mode }
  // 星云层与远景尘点对任意数量都走同一套绘制，仅亮度由 cloudLevel 连续变化，0 条也保留同一张星图。
  const settling = easeCloud(now, props.stars.length ? cloudTarget() : 0)
  // 云雾有亮度就继续绘制：目标为 0 时用上一批采样淡出；亮度降到阈值以下才停绘并释放云雾层。
  const plan = cloudPlan(cloudLevel, samples.value, fadingSamples)
  fadingSamples = plan.keep
  const cloudSamples = plan.use
  const hasCloud = plan.draw
  if (plan.release) { nebula?.dispose(); nebula = undefined }
  drawSkyDust(context, props.camera, size.value, palette, cloudLevel)
  drawSeedGlow(context, size.value, palette, 1 - Math.min(1, cloudLevel * 2.2))
  if (!props.stars.length && !settling && !hasCloud) { pauseFlow(); return }
  const flowing = hasCloud && !props.focused && !reduced.matches
  if (flowing) {
    if (lastFlowFrame !== undefined) {
      const dt = Math.min(250, Math.max(0, now - lastFlowFrame))
      flowElapsed = (flowElapsed + dt / Math.pow(props.camera.zoom, .65)) % FLOW_PERIOD_MS
    }
    lastFlowFrame = now
  } else lastFlowFrame = undefined
  if (hasCloud) {
    nebula ??= createNebulaLayer(scheduleDraw)
    const volume = nebula.render(cloudSamples!, props.camera, size.value, surface.width / width, cloudLevel, palette, props.focused, flowElapsed)
    context.drawImage(volume, 0, 0, width, height)
  }
  const pulse = reduced.matches || !arrivalsAt ? 0 : Math.max(0, 1 - (now - arrivalsAt) / 260)
  let pulsing = false
  const scale = haloScale(props.stars.length)
  for (const star of drawOrder.value) {
    const amount = props.arrivals.has(star.point.starId) ? pulse : 0
    context.globalCompositeOperation = star.point.privateLabel ? 'source-over' : palette.composite
    drawFeedbackStar(context, glows!, star, amount, detail.value, palette, scale)
    if (amount > 0) pulsing = true
  }
  context.globalCompositeOperation = 'source-over'
  for (const star of props.stars) {
    if (star.point.starId === props.hovered || star.point.starId === props.selected) drawStarFocus(context, star, palette)
  }
  if (pulsing || settling) scheduleDraw()
  else if (flowing && nebula) {
    // 手动操作始终可以立即排帧。缓慢的尘流单独限频，绘制较慢的设备留出更多输入时间。
    const cost = performance.now() - started
    const delay = Math.max(0, nebula.flowInterval - cost - 7, cost * .75)
    flowTimer = window.setTimeout(scheduleDraw, delay)
  }
}

function resize(): void {
  if (!stage.value || !canvas.value || !stage.value.clientWidth || !stage.value.clientHeight) return
  size.value = {width: stage.value.clientWidth, height: stage.value.clientHeight}
  const pixelRatio = Math.min(window.devicePixelRatio || 1, 2)
  canvas.value.width = Math.round(size.value.width * pixelRatio)
  canvas.value.height = Math.round(size.value.height * pixelRatio)
  emit('change', props.camera, size.value)
  scheduleDraw()
}

// =============== 镜头与命中 ===============
// 说明：拖动和方向键均绕银心转动，滚轮和按钮以银心变焦。重叠时优先命中前面的星。
function screenPoint(event: MouseEvent): Coordinates {
  const rect = stage.value?.getBoundingClientRect()
  return {x: event.clientX - (rect?.left ?? 0), y: event.clientY - (rect?.top ?? 0)}
}

function hit(screen: Coordinates, radius = 12): ProjectedStar | null {
  let nearest: ProjectedStar | null = null
  let distance = radius ** 2
  for (const star of props.stars) {
    const squared = (star.x - screen.x) ** 2 + (star.y - screen.y) ** 2
    if (squared > distance) continue
    if (squared === distance && nearest && (star.depth < nearest.depth ||
      (star.depth === nearest.depth && star.point.starId > nearest.point.starId))) continue
    nearest = star
    distance = squared
  }
  return nearest
}

// 点击命中：判定规则在 resolveClick（可单测）。拥挤时交出半径内全部候选，列表自身分页。
function clickTarget(screen: Coordinates): {open?: ProjectedStar; crowd?: ProjectedStar[]} {
  return resolveClick(props.stars, screen, 12)
}

function move(next: Camera): void {
  emit('hover', null)
  emit('approach', null)
  emit('change', boundedCamera(next), size.value)
  scheduleDraw()
}

function onWheel(event: WheelEvent): void {
  if (props.menuOpen) emit('dismissMenu')
  const unit = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? size.value.height : 1
  const delta = Math.max(-240, Math.min(240, event.deltaY * unit))
  emit('zoom', Math.exp(-delta * .00185))
}

function onDown(event: PointerEvent): void {
  if ((event.button !== 0 && event.button !== 2) || drag) return
  skipNativeMenu = false
  stage.value?.focus({preventScroll: true})
  if (props.menuOpen) {
    emit('dismissMenu')
    if (event.button === 0) return
  }
  stage.value?.setPointerCapture(event.pointerId)
  drag = {id: event.pointerId, start: screenPoint(event), camera: props.camera, button: event.button, sawContext: false}
}

function onMove(event: PointerEvent): void {
  const screen = screenPoint(event)
  if (drag) {
    if (drag.id !== event.pointerId) return
    const dx = screen.x - drag.start.x
    const dy = screen.y - drag.start.y
    if (Math.hypot(dx, dy) < 4 && !dragging.value) return
    dragging.value = true
    move({...props.camera, yaw: drag.camera.yaw + dx * .006, inclination: drag.camera.inclination + dy * .006})
    return
  }
  if (props.menuOpen) return
  // 一次最近邻查找同时覆盖命中和接近意图，预取区本身不能点开反馈。
  const near = hit(screen, 64)
  const star = near && (near.x - screen.x) ** 2 + (near.y - screen.y) ** 2 <= 12 ** 2 ? near : null
  emit('hover', star?.point ?? null, star ?? undefined)
  emit('approach', near?.point ?? null)
}

function leaveMap(): void {
  emit('approach', null)
  emit('hover', null)
}

function stopDrag(): void {
  drag = undefined
  dragging.value = false
}

function onUp(event: PointerEvent): void {
  if (drag?.id !== event.pointerId) return
  const right = drag.button === 2
  const menuGesture = right || drag.sawContext
  const clicked = !dragging.value
  const target = clicked && !menuGesture ? clickTarget(screenPoint(event)) : {}
  // WebView 的 contextmenu 可能先于或晚于松键。同一次右击只打开一次，右键拖动仍用于旋转。
  skipNativeMenu = right && !drag.sawContext
  stopDrag()
  if (menuGesture && clicked) emit('menu', {x: event.clientX, y: event.clientY})
  else if (target.open) emit('open', target.open.point.starId)
  else if (target.crowd) emit('crowd', target.crowd)
}

function keyboardMenu(target: EventTarget | null): void {
  const anchor = target instanceof HTMLElement ? target.closest('button') ?? stage.value : stage.value
  const rect = anchor?.getBoundingClientRect()
  if (rect) emit('menu', {x: rect.left + rect.width / 2, y: rect.top + rect.height / 2})
}

function onContextMenu(event: MouseEvent): void {
  if (drag) { drag.sawContext = true; return }
  if (skipNativeMenu) { skipNativeMenu = false; return }
  if (!event.clientX && !event.clientY) keyboardMenu(event.target)
  else emit('menu', {x: event.clientX, y: event.clientY})
}

function captionPointer(event: PointerEvent): void {
  if (event.button !== 2) event.stopPropagation()
}

function onKey(event: KeyboardEvent): void {
  if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')) {
    event.preventDefault()
    keyboardMenu(event.target)
    return
  }
  if (props.menuOpen) return
  const currentCamera = props.camera
  const step = Math.PI / 14
  switch (event.key) {
    case 'ArrowLeft': move({...currentCamera, yaw: currentCamera.yaw - step}); break
    case 'ArrowRight': move({...currentCamera, yaw: currentCamera.yaw + step}); break
    case 'ArrowUp': move({...currentCamera, inclination: currentCamera.inclination - step}); break
    case 'ArrowDown': move({...currentCamera, inclination: currentCamera.inclination + step}); break
    case '+':
    case '=': event.preventDefault(); emit('zoom', 1.4); return
    case '-': event.preventDefault(); emit('zoom', 1 / 1.4); return
    case 'Home': event.preventDefault(); emit('home'); return
    case 'Enter': event.preventDefault(); emit('browse'); return
    default: return
  }
  event.preventDefault()
}

watch(samples, scheduleDraw)
watch(theme, scheduleDraw)
watch(() => props.camera, scheduleDraw)
watch([() => props.stars, () => props.hovered, () => props.selected, () => props.focused], scheduleDraw)
watch(() => props.arrivals, (arrivals) => {
  if (arrivals.size) arrivalsAt = performance.now()
  else arrivalsAt = 0
  scheduleDraw()
})
watch(() => props.active, (active) => {
  syncMotion()
  if (active) resize()
  else stopDrag()
})
onMounted(() => {
  document.addEventListener('visibilitychange', syncMotion)
  reduced.addEventListener('change', syncMotion)
  observer = new ResizeObserver(resize)
  if (stage.value) observer.observe(stage.value)
  resize()
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', syncMotion)
  reduced.removeEventListener('change', syncMotion)
  pauseFlow()
  observer?.disconnect()
  cancelAnimationFrame(frame)
  nebula?.dispose()
})

defineExpose({focus: () => stage.value?.focus({preventScroll: true})})
</script>

<template>
  <!-- ~ 反馈星群 -->
  <!-- * 说明：来自反馈的三维光尘在星盘上相叠，星核打开对应反馈。 -->
  <!-- * 交互：拖动或方向键环绕，滚轮缩放，回车浏览星点，右击打开操作菜单。 -->
  <!-- * 依赖：当前相机、服务器返回星点和账号范围内的私有星。 -->
  <!-- * 边界：0 条与多条共用同一张星图，只有云雾亮度随数量连续变化；减少动态效果时亮度直接到位。 -->
  <div ref="stage" class="galaxy-stage" :class="{dragging, 'has-star': hovered}" tabindex="0"
       role="region" aria-haspopup="menu" aria-keyshortcuts="Shift+F10" :aria-controls="menuOpen ? 'feedback-map-actions' : undefined"
       aria-label="立体反馈星图，方向键绕银心旋转，加减键缩放，回车浏览星点，右击或 Shift 加 F10 打开反馈菜单"
       @contextmenu.prevent="onContextMenu" @wheel.prevent="onWheel" @pointerdown="onDown" @pointermove="onMove" @pointerup="onUp"
       :aria-describedby="tooltipId"
       @pointercancel="stopDrag" @lostpointercapture="stopDrag" @pointerleave="leaveMap" @keydown="onKey">
    <canvas ref="canvas" aria-hidden="true" />
    <!-- ~ 近景星号：按可用空间排布，指向与星核相同的反馈。 -->
    <button v-for="caption in captions" :key="caption.star.point.starId" type="button" class="star-caption"
      :class="{'on-left': caption.x < caption.star.x}"
      :style="{left: caption.x + 'px', top: caption.y + 'px', width: caption.width + 'px', opacity: detail}"
      :aria-label="starLabel(caption.star.point) + '，星 ' + caption.star.point.starId"
      :aria-describedby="hovered === caption.star.point.starId ? tooltipId : undefined"
      @pointerdown="captionPointer" @pointerup="captionPointer" @pointermove.stop="emit('hover', caption.star.point, caption.star)"
      @pointerleave="emit('hover', null)" @focus="emit('hover', caption.star.point, caption.star)" @blur="emit('hover', null)"
      @keydown.enter.stop @click.stop="emit('open', caption.star.point.starId)">#{{ caption.star.point.starId }}</button>
  </div>
</template>

<style scoped lang="scss">
.galaxy-stage {
  position: absolute;
  inset: 0;
  overflow: hidden;
  // 舞台底色与首页戴森球场景一致：日间 #ebebeb 的白灰底，夜间深空黑；星云与星核由画布按主题绘制。
  background: var(--ds-bg-shell);
  cursor: grab;
  touch-action: none;

  &.has-star { cursor: pointer; }
  &.dragging { cursor: grabbing; }
  &:focus-visible { outline: 2px solid var(--ds-brand); outline-offset: -3px; }

  canvas { width: 100%; height: 100%; display: block; }
  .star-caption {
    position: absolute; height: 18px; padding: 0 4px; border: 0; border-radius: 2px;
    background: transparent; color: var(--ds-text-primary); text-align: center; white-space: nowrap;
    text-shadow: 0 1px 3px var(--ds-bg-shell), 0 0 4px var(--ds-bg-shell);
    font: 500 12px/18px var(--ds-font-display, sans-serif); font-variant-numeric: tabular-nums; cursor: pointer;
    &::before { content: ''; position: absolute; left: -4px; top: 50%; width: 5px; height: 1px; background: currentColor; opacity: .45; }
    &.on-left::before { left: auto; right: -4px; }
    &:hover, &:focus-visible { background: var(--ds-bg-panel-elevated); outline: 1px solid var(--ds-brand); }
  }
}
</style>
