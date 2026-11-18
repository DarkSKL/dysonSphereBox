<script lang="ts">
// SVG 渐变与滤镜 ID 需要在页面内唯一；四张卡片同时存在，按实例递增编号。
let serial = 0
</script>

<script setup lang="ts">
import {computed, onBeforeUnmount, ref, shallowRef, watch} from 'vue'
import ItemIcon from '../item-icon.vue'
import {analysisReducedMotion, mixHex, rampAt, shareEntryTone, type AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {ensureFactoryIcons} from '../../../../pages/services/dysonSphere/factory-icons'
import {formatInteger, formatPercent, formatShare, formatValue, shareReadingOf, shareValueOf, splitPower, type ShareEntry, type ShareMetricInfo, type ShareSpaceSummary} from '../../../../pages/services/dysonSphere/factory-analysis'
import {RING_RISE, RING_SCAN, RING_SQUASH, RING_START, fitRing, ringArcLength, ringPieces, ringPoint, ringSpans, ringTicks, type RingBlock, type RingPiece} from '../../../../pages/services/dysonSphere/analysis/iso-ring'

// =============== 整体占比环形卡片 ===============
// 说明：一个口径一张卡片。环由纪念碑谷式等距块组成：顶面按占比取明度，侧壁按受光混入冷色阴影；底座是科幻刻度、扫描环与中心光源。
//       环心平时显示合计，悬停或固定时换成该项占比；图标立在放得下的块顶，右侧图例列出全部条目与精确占比。
// 交互：悬停块或图例行时该块升起、其余淡化，并联动台地与同列表的卡片；点击块或图例行固定该项并把台地切到本口径；点击卡片其他位置只切换台地口径。
// 边界：角度严格按数值比例，块高只表示悬停，不编码数据；空间统计缺失或口径为空时画全息空环并说明原因，不伪造零值。
const props = defineProps<{info: ShareMetricInfo; entries: readonly ShareEntry[]; total: number; space: ShareSpaceSummary | null; palette: AnalysisPalette; hovered: number | null; pinned: number | null; linked: boolean; selectable: boolean}>()
const emit = defineEmits<{(event: 'hover', index: number | null): void; (event: 'pick', index: number): void; (event: 'focus'): void}>()
const uid = `share-ring-${++serial}`
const reducedMotion = analysisReducedMotion()
ensureFactoryIcons()
const active = computed(() => props.hovered ?? props.pinned)

// =============== 条目 ===============
// 说明：先记录原始下标再筛选排序，保证悬停、固定与台地指向同一设施或物品；空间按「建筑 → 线路 → 空地」固定顺序，从 12 点顺时针读出利用率。
interface RingRow { readonly entry: ShareEntry; readonly index: number; readonly value: number; readonly share: number; readonly ratio: number }
const rows = computed<RingRow[]>(() => {
  const metric = props.info.key
  const list = props.entries.map((entry, index) => ({entry, index, value: shareValueOf(entry, metric)})).filter(row => row.value > 1e-9)
  if (metric !== 'space') list.sort((a, b) => b.value - a.value || a.index - b.index)
  const whole = list.reduce((sum, row) => sum + row.value, 0)
  const peak = list.reduce((max, row) => Math.max(max, row.value), 0)
  return list.map(row => ({...row, share: whole > 0 ? row.value / whole : 0, ratio: peak > 0 ? row.value / peak : 0}))
})
// 配色与数据分开计算：切换日夜只换颜色，不重播升起动画。null 表示空地，画成全息虚框。
const tones = computed(() => new Map(rows.value.map(row => [row.index, shareEntryTone(props.palette, props.info.key, row.entry, row.ratio)])))
const rampOf = computed(() => props.palette.shareRamps[props.info.key])
const hudLine = computed(() => rampAt(rampOf.value, props.palette.dark ? 0.85 : 0.72))
/** 等距小方块的三面：顶面原色，左右侧面按受光混入阴影；卡片标题与图例共用 */
const cubeOf = (top: string): {top: string; left: string; right: string} => ({top, left: mixHex(top, props.palette.shadeTint, 0.3), right: mixHex(top, props.palette.shadeTint, 0.52)})
const accent = computed(() => cubeOf(rampAt(rampOf.value, props.palette.dark ? 1 : 0.62)))
// 图例：与环上的块同一套三面明暗；空地没有实体颜色，画成虚线框。
const legend = computed(() => rows.value.map(row => {
  const tone = tones.value.get(row.index)
  return {...row, cube: tone ? cubeOf(tone) : null}
}))

// =============== 画幅 ===============
const plot = ref<HTMLElement | null>(null)
const size = shallowRef({width: 0, height: 0})
watch(plot, (element, _previous, onCleanup) => {
  if (!element) return
  const observer = new ResizeObserver(([entry]) => {
    if (entry) size.value = {width: Math.round(entry.contentRect.width), height: Math.round(entry.contentRect.height)}
  })
  observer.observe(element)
  onCleanup(() => observer.disconnect())
}, {flush: 'post'})
const frame = computed(() => fitRing(size.value.width, size.value.height))

// =============== 动画 ===============
// 说明：数据变化时块从地面依次升起（从 12 点顺时针），呼应台地立柱；悬停抬升与淡化按指数平滑过渡，只在变化期间逐帧刷新。
interface Motion { lift: number; dim: number; grow: number; delay: number }
const motion = new Map<number, Motion>()
const tick = ref(0)
let frameId = 0
let lastTime = 0
let buildStart = 0
let disposed = false
const easeOut = (t: number): number => 1 - (1 - Math.min(1, Math.max(0, t))) ** 3
const kick = (): void => { if (!frameId && !disposed) frameId = requestAnimationFrame(step) }
const step = (now: number): void => {
  frameId = 0
  const dt = lastTime ? Math.min(0.05, (now - lastTime) / 1000) : 1 / 60
  lastTime = now
  const smoothing = reducedMotion ? 1 : 1 - Math.exp(-dt * 14)
  let busy = false
  for (const [index, state] of motion) {
    const lift = active.value === index ? 1 : 0
    const dim = active.value !== null && active.value !== index ? 1 : 0
    state.lift += (lift - state.lift) * smoothing
    state.dim += (dim - state.dim) * smoothing
    if (Math.abs(lift - state.lift) > 0.002 || Math.abs(dim - state.dim) > 0.002) busy = true
    else { state.lift = lift; state.dim = dim }
    if (state.grow < 1) {
      state.grow = reducedMotion ? 1 : easeOut((now - buildStart - state.delay) / 480)
      busy = true
    }
  }
  tick.value++
  if (busy) kick()
  else lastTime = 0
}
watch(rows, list => {
  const keep = new Set(list.map(row => row.index))
  for (const key of [...motion.keys()]) if (!keep.has(key)) motion.delete(key)
  const stagger = Math.min(55, 520 / Math.max(1, list.length))
  list.forEach((row, rank) => {
    const state = motion.get(row.index) ?? {lift: 0, dim: 0, grow: 0, delay: 0}
    state.grow = reducedMotion ? 1 : 0
    state.delay = rank * stagger
    motion.set(row.index, state)
  })
  buildStart = performance.now()
  kick()
}, {immediate: true})
watch(active, kick)
onBeforeUnmount(() => {
  disposed = true
  if (frameId) cancelAnimationFrame(frameId)
})

// =============== 场景 ===============
// 说明：块的顶面取占比色；外壁、内壁与端面按受光混入冷色阴影（左前受光、右后背光），淡化时整体向地面色靠拢。
//       空地与空环画成低矮的全息块：半透明填充加虚线描边。
interface Face { readonly d: string; readonly fill: string }
interface PaintedPiece {
  readonly key: string
  readonly entry: number
  readonly ghost: boolean
  readonly focus: boolean
  readonly opacity: number
  readonly top: Face
  readonly wall: Face
  readonly caps: readonly Face[]
  readonly rim: {readonly d: string; readonly stroke: string} | null
}
interface RingIcon { readonly key: number; readonly name: string; readonly size: number; readonly x: number; readonly y: number; readonly shadowY: number; readonly pop: number; readonly dim: number }
const scene = computed(() => {
  void tick.value
  const f = frame.value
  if (!f) return null
  const p = props.palette
  const list = rows.value
  const blocks: RingBlock[] = []
  const icons: RingIcon[] = []
  const spans = ringSpans(f, list.map(row => row.value))
  const radius = (f.outer + f.inner) / 2
  const iconSize = Math.round(Math.min(28, Math.max(14, (f.outer - f.inner) * RING_SQUASH * 0.95)))
  list.forEach((row, rank) => {
    const span = spans[rank]
    const state = motion.get(row.index)
    const grow = state?.grow ?? 1
    const ghost = tones.value.get(row.index) == null
    const z = (ghost ? f.height * 0.36 : f.height) * grow + f.lift * (state?.lift ?? 0)
    blocks.push({entry: row.index, from: span.from, to: span.to, z, whole: list.length === 1})
    // 图标立在块顶中线：只放进弧长足够的块，小块交给图例与悬停读数。
    if (ghost || ringArcLength(f, radius, span.from, span.to) < iconSize * 1.3) return
    const [x, y] = ringPoint(f, span.mid, radius, z)
    icons.push({key: row.index, name: row.entry.icon, size: iconSize, x, y: y - iconSize * 0.22, shadowY: y + iconSize * 0.16, pop: easeOut((grow - 0.55) / 0.45), dim: state?.dim ?? 0})
  })
  // 无数据：一圈全息空环，保留底座，让四张卡片的构图一致。
  if (!list.length) blocks.push({entry: -1, from: RING_START, to: RING_START + Math.PI * 2, z: f.height * 0.36, whole: true})
  const pieces = ringPieces(f, blocks).map(piece => paint(piece, p))
  return {pieces, icons}
})
const paint = (piece: RingPiece, p: AnalysisPalette): PaintedPiece => {
  const state = motion.get(piece.entry)
  const dim = state?.dim ?? 0
  const grow = state?.grow ?? 1
  const tone = piece.entry < 0 ? null : tones.value.get(piece.entry) ?? null
  const base = {key: piece.key, entry: piece.entry, focus: (state?.lift ?? 0) > 0.5, opacity: (0.25 + 0.75 * Math.min(1, grow * 2)) * (1 - dim * (tone ? 0 : 0.5))}
  if (!tone) {
    // 全息：顶面与侧壁共用扫描线色，透明度区分前后。
    const glass = hudLine.value
    return {...base, ghost: true, top: {d: piece.top, fill: glass}, wall: {d: piece.wall.d, fill: glass}, caps: piece.caps.map(cap => ({d: cap.d, fill: glass})), rim: null}
  }
  const fade = (color: string): string => mixHex(color, p.tileB, dim * 0.62)
  const shade = (amount: number): string => fade(mixHex(tone, p.shadeTint, amount))
  const top = fade(tone)
  return {
    ...base, ghost: false,
    top: {d: piece.top, fill: top},
    wall: {d: piece.wall.d, fill: shade(piece.wall.inner ? 0.7 - 0.24 * piece.wall.light : 0.6 - 0.3 * piece.wall.light)},
    caps: piece.caps.map(cap => ({d: cap.d, fill: shade(0.5 - 0.3 * cap.light)})),
    rim: {d: piece.rim, stroke: mixHex(top, '#ffffff', p.dark ? 0.42 : 0.6)},
  }
}
const hud = computed(() => {
  const f = frame.value
  if (!f) return null
  return {
    ticks: ringTicks(f),
    glow: {cx: f.cx, cy: f.cy, rx: f.outer * 1.3, ry: f.outer * 1.3 * RING_SQUASH},
    shadow: {cx: f.cx, cy: f.cy + f.outer * 0.03, rx: f.outer * 1.03, ry: f.outer * 1.03 * RING_SQUASH, blur: Math.max(2, f.outer * 0.045)},
    plinth: {cx: f.cx, cy: f.cy, rx: f.outer * 1.08, ry: f.outer * 1.08 * RING_SQUASH},
    scan: {cx: f.cx, cy: f.cy, rx: f.outer * RING_SCAN, ry: f.outer * RING_SCAN * RING_SQUASH},
    core: {cx: f.cx, cy: f.cy, rx: f.inner * 0.62, ry: f.inner * 0.62 * RING_SQUASH},
  }
})

// =============== 读数 ===============
// 说明：环心的全息读数位于环孔正中；默认是本口径合计，悬停或固定时换成该项的占比、名称与数值。
const focusRow = computed(() => (active.value === null ? null : rows.value.find(row => row.index === active.value) ?? null))
const headline = computed<{value: string; unit: string; caption: string}>(() => {
  const count = rows.value.length
  const key = props.info.key
  if (key === 'space') return props.space ? {value: formatPercent(props.space.utilization), unit: '', caption: `包络 ${formatInteger(props.space.envelopeGrid2)} 格²`} : {value: '—', unit: '', caption: '生成蓝图后统计'}
  if (!count) return {value: '—', unit: '', caption: key === 'production' ? '当前方案没有产出物品' : '暂无可统计数据'}
  if (key === 'power') {
    const power = splitPower(props.total)
    return {value: power.value, unit: power.unit, caption: `额定 · ${count} 种设施`}
  }
  return key === 'building' ? {value: formatInteger(props.total), unit: '座', caption: `${count} 种设施`} : {value: formatValue(props.total), unit: '件/min', caption: `${count} 种物品`}
})
const readoutStyle = computed(() => {
  const f = frame.value
  if (!f) return {}
  const clamp = (value: number, min: number, max: number): number => Math.min(max, Math.max(min, value))
  return {left: `${f.cx}px`, top: `${f.cy - f.height * RING_RISE * 0.75}px`, width: `${f.inner * 1.7}px`, '--readout-size': `${clamp(f.outer * 0.19, 14, 30)}px`, '--caption-size': `${clamp(f.outer * 0.085, 9.5, 12)}px`}
})
const groundShadow = computed(() => (props.palette.dark ? 'rgba(0, 0, 0, 0.55)' : 'rgba(27, 35, 51, 0.2)'))
// 卡片没有边框，连接台地的那张靠更亮的底座光晕区分。
const glowStrength = computed(() => (props.palette.dark ? 0.24 : 0.16) * (props.linked ? 1.5 : 1))
const ariaLabel = computed(() => {
  const list = rows.value
  if (!list.length) return `${props.info.label}：${headline.value.caption}`
  const parts = list.slice(0, 12).map(row => `${row.entry.name} ${formatShare(row.share)}`)
  return `${props.info.label}：${parts.join('，')}${list.length > 12 ? `，另有 ${list.length - 12} 项` : ''}`
})

// =============== 指针与键盘 ===============
// 说明：环上的面片与图例行都带 data-entry；按目标元素解析条目，同一块被切成前后两片时悬停也不会闪断。
let lastHover: number | null = null
watch(() => props.hovered, value => { lastHover = value })
const entryAt = (target: EventTarget | null): number | null => {
  const value = target instanceof Element ? target.closest('[data-entry]')?.getAttribute('data-entry') : null
  return value == null ? null : Number(value)
}
const setHover = (index: number | null): void => {
  if (index === lastHover) return
  lastHover = index
  emit('hover', index)
}
const onPointerMove = (event: PointerEvent): void => setHover(entryAt(event.target))
const onPointerLeave = (): void => setHover(null)
const onFocusIn = (event: FocusEvent): void => { const index = entryAt(event.target); if (index !== null) setHover(index) }
const onFocusOut = (event: FocusEvent): void => { if (entryAt(event.target) !== null) setHover(null) }
const onClick = (event: MouseEvent): void => {
  const index = entryAt(event.target)
  if (index !== null) emit('pick', index)
  else emit('focus')
}
</script>

<template>
  <!-- ~ 整体占比环形卡片 -->
  <!-- * 说明：等距环形块 + 科幻底座，环心为合计或悬停项的读数，右侧图例给出精确占比 -->
  <!-- * 交互：悬停块或图例行联动台地；点击块或图例行固定该项并切换台地口径；点击卡片其余位置只切换台地口径 -->
  <!-- * 依赖：整体占比条目、口径合计与空间摘要；颜色来自分析调色板的口径明度梯 -->
  <!-- * 边界：无数据时画全息空环并说明原因；块高只表示悬停，不编码数值 -->
  <section class="ring-card" :class="{'is-active': linked, 'is-locked': !selectable, 'has-focus': active !== null}" :style="{'--card-accent': accent.top, '--hud-line': hudLine}" :aria-label="info.label" @click="onClick" @pointermove="onPointerMove" @pointerleave="onPointerLeave" @focusin="onFocusIn" @focusout="onFocusOut">
    <header class="card-head">
      <button type="button" class="card-title" :aria-pressed="linked" :disabled="!selectable" :title="selectable ? (linked ? '左侧台地正在显示此口径' : '在左侧台地显示此口径') : headline.caption">
        <svg class="card-cube" viewBox="0 0 14 14" aria-hidden="true"><path d="M7 1.2 12.6 4.4 7 7.6 1.4 4.4Z" :fill="accent.top" /><path d="M1.4 4.4 7 7.6v5.4L1.4 9.8Z" :fill="accent.left" /><path d="M12.6 4.4 7 7.6v5.4l5.6-3.2Z" :fill="accent.right" /></svg>
        <span class="card-code type-display">{{ info.code }}</span>
        <span class="card-label">{{ info.label }}</span>
      </button>
      <span class="card-link" :class="{'is-on': linked}" aria-hidden="true">{{ linked ? '台地' : selectable ? '在台地显示' : '' }}</span>
    </header>

    <div class="card-body">
      <div ref="plot" class="ring-plot">
        <svg v-if="scene && hud" class="ring-svg" :viewBox="`0 0 ${size.width} ${size.height}`" role="img" :aria-label="ariaLabel">
          <defs>
            <radialGradient :id="`${uid}-glow`"><stop offset="0" :stop-color="hudLine" :stop-opacity="glowStrength" /><stop offset=".6" :stop-color="hudLine" :stop-opacity="glowStrength * 0.3" /><stop offset="1" :stop-color="hudLine" stop-opacity="0" /></radialGradient>
            <radialGradient :id="`${uid}-core`"><stop offset="0" :stop-color="hudLine" :stop-opacity="palette.dark ? 0.42 : 0.3" /><stop offset="1" :stop-color="hudLine" stop-opacity="0" /></radialGradient>
            <filter :id="`${uid}-blur`" x="-20%" y="-40%" width="140%" height="180%"><feGaussianBlur :stdDeviation="hud.shadow.blur" /></filter>
          </defs>
          <!-- 地面：光晕、接地阴影、底座边线、百分比刻度、扫描环与环心光源 -->
          <g class="ring-ground" aria-hidden="true">
            <ellipse :cx="hud.glow.cx" :cy="hud.glow.cy" :rx="hud.glow.rx" :ry="hud.glow.ry" :fill="`url(#${uid}-glow)`" />
            <ellipse :cx="hud.shadow.cx" :cy="hud.shadow.cy" :rx="hud.shadow.rx" :ry="hud.shadow.ry" :fill="groundShadow" :filter="`url(#${uid}-blur)`" />
            <ellipse class="ground-plinth" :cx="hud.plinth.cx" :cy="hud.plinth.cy" :rx="hud.plinth.rx" :ry="hud.plinth.ry" />
            <path class="ground-ticks" :d="hud.ticks.minor" />
            <path class="ground-ticks is-major" :d="hud.ticks.major" />
            <ellipse class="ground-scan" :cx="hud.scan.cx" :cy="hud.scan.cy" :rx="hud.scan.rx" :ry="hud.scan.ry" />
            <ellipse class="ground-scan-head" pathLength="100" :cx="hud.scan.cx" :cy="hud.scan.cy" :rx="hud.scan.rx" :ry="hud.scan.ry" />
            <ellipse :cx="hud.core.cx" :cy="hud.core.cy" :rx="hud.core.rx" :ry="hud.core.ry" :fill="`url(#${uid}-core)`" />
          </g>
          <!-- 环形块：按画家算法排好序，后半圈先画；同一块的前后两片共用 data-entry -->
          <g v-for="piece in scene.pieces" :key="piece.key" class="ring-piece" :class="{'is-ghost': piece.ghost, 'is-focus': piece.focus}" :data-entry="piece.entry >= 0 ? piece.entry : undefined" :opacity="piece.opacity">
            <path class="piece-wall" :d="piece.wall.d" :fill="piece.wall.fill" />
            <path v-for="(cap, part) in piece.caps" :key="part" class="piece-wall" :d="cap.d" :fill="cap.fill" />
            <path class="piece-top" :d="piece.top.d" :fill="piece.top.fill" />
            <path v-if="piece.rim" class="piece-rim" :d="piece.rim.d" :stroke="piece.rim.stroke" />
          </g>
          <!-- 图标落脚的接触阴影：随块一起抬升 -->
          <ellipse v-for="icon in scene.icons" :key="`shadow-${icon.key}`" class="icon-shadow" :cx="icon.x" :cy="icon.shadowY" :rx="icon.size * 0.32 * icon.pop" :ry="icon.size * 0.13 * icon.pop" :opacity="(palette.dark ? 0.5 : 0.24) * (1 - icon.dim * 0.7)" />
        </svg>
        <!-- 图标与读数是同一像素坐标系里的 HTML 覆盖层，不拦截指针 -->
        <template v-if="scene">
          <span v-for="icon in scene.icons" :key="icon.key" class="ring-icon" :style="{left: `${icon.x}px`, top: `${icon.y}px`, opacity: 1 - icon.dim * 0.65, transform: `translate(-50%, -50%) scale(${icon.pop})`}"><ItemIcon :name="icon.name" :size="icon.size" /></span>
          <div class="ring-readout" :style="readoutStyle" aria-hidden="true">
            <template v-if="focusRow">
              <strong class="type-display">{{ formatShare(focusRow.share) }}</strong>
              <span class="readout-name">{{ focusRow.entry.name }}</span>
              <small>{{ shareReadingOf(focusRow.entry, info.key) }}</small>
            </template>
            <template v-else>
              <strong class="type-display">{{ headline.value }}<em v-if="headline.unit">{{ headline.unit }}</em></strong>
              <small>{{ headline.caption }}</small>
            </template>
          </div>
        </template>
      </div>

      <!-- 图例：全部条目按环上顺序列出，小方块与块同色，数值为精确占比 -->
      <ul v-if="legend.length" class="ring-legend" :aria-label="`${info.label}图例`">
        <li v-for="row in legend" :key="row.index">
          <button type="button" class="legend-row" :class="{'is-active': active === row.index, 'is-pinned': pinned === row.index, 'is-muted': active !== null && active !== row.index}" :data-entry="row.index" :aria-pressed="pinned === row.index" :aria-label="`${row.entry.name} · ${shareReadingOf(row.entry, info.key)} · ${formatShare(row.share)}`">
            <svg class="legend-cube" :class="{'is-ghost': !row.cube}" viewBox="0 0 10 11" aria-hidden="true">
              <template v-if="row.cube"><path d="M5 .6 9.4 3 5 5.4.6 3Z" :fill="row.cube.top" /><path d="M.6 3 5 5.4v5L.6 8Z" :fill="row.cube.left" /><path d="M9.4 3 5 5.4v5L9.4 8Z" :fill="row.cube.right" /></template>
              <path v-else d="M5 .6 9.4 3v5L5 10.4.6 8V3Z" />
            </svg>
            <ItemIcon :name="row.entry.icon" :size="16" />
            <span class="legend-name">{{ row.entry.name }}</span>
            <span class="legend-share type-display">{{ formatShare(row.share) }}</span>
          </button>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped lang="scss">
// 卡片：无边框、无底板、无分界，四个占比环直接浮在分析天空上；连接台地的那张以标题旁的菱形点、更亮的底座光晕和更快的扫描环示意。
.ring-card { position: relative; display: flex; min-width: 0; min-height: 0; flex-direction: column; container: ring-card / size; cursor: pointer; }
.ring-card.is-locked { cursor: default; }

.card-head { position: relative; z-index: 1; display: flex; min-width: 0; align-items: center; gap: 8px; padding: 7px 10px 0; }
.card-title { display: inline-flex; min-width: 0; align-items: center; gap: 7px; border: 0; border-radius: 2px; padding: 2px 0; background: transparent; color: var(--ds-text-primary); font: inherit; font-size: 12.5px; font-weight: 600; cursor: inherit; }
.card-title:disabled { color: var(--ds-text-secondary); }
.card-title:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: 2px; }
.card-cube { width: 14px; height: 14px; flex: 0 0 auto; }
.card-code { color: var(--ds-text-muted); font-size: 10px; font-weight: 500; letter-spacing: .12em; }
.card-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
// 台地连接状态：平时隐藏，悬停提示可切换；连接后常亮，菱形点用口径色。
.card-link { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 5px; margin-left: auto; color: var(--ds-text-muted); font-size: 10px; letter-spacing: .08em; white-space: nowrap; opacity: 0; transition: opacity 160ms; }
.card-link.is-on { color: var(--ds-text-secondary); opacity: 1; }
.card-link.is-on::before { width: 6px; height: 6px; background: var(--card-accent); content: ''; transform: rotate(45deg); box-shadow: 0 0 6px var(--card-accent); }
.ring-card:not(.is-locked):hover .card-link { opacity: 1; }

.card-body { position: relative; display: grid; min-height: 0; flex: 1; grid-template-columns: minmax(0, 1fr) minmax(132px, .7fr); gap: 2px; padding: 0 6px 6px 2px; }
.ring-plot { position: relative; min-width: 0; min-height: 0; }
.ring-svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }

// 地面层：线宽固定 1px 左右，透明度区分层级；扫描头沿底座环行，连接台地的卡片转得更快。
.ground-plinth { fill: none; stroke: var(--hud-line); stroke-width: 1; opacity: .24; }
.ground-ticks { fill: none; stroke: var(--hud-line); stroke-width: 1; opacity: .4; }
.ground-ticks.is-major { stroke-width: 1.5; opacity: .72; }
.ground-scan { fill: none; stroke: var(--hud-line); stroke-width: 1; stroke-dasharray: 1 3; opacity: .28; }
.ground-scan-head { fill: none; stroke: var(--hud-line); stroke-width: 1.6; stroke-dasharray: 7 93; stroke-linecap: round; opacity: .7; animation: ring-scan 14s linear infinite; }
.ring-card.is-active .ground-scan-head { opacity: 1; animation-duration: 6s; }

// 环形块：侧壁与顶面平涂（透明度与颜色由逐帧动画给出，不另加过渡）；顶面外沿一道亮边，聚焦块带口径色辉光。
.piece-rim { fill: none; stroke-width: 1; stroke-linecap: round; opacity: .85; pointer-events: none; }
.ring-piece.is-ghost .piece-top { fill-opacity: .1; stroke: var(--hud-line); stroke-width: 1; stroke-dasharray: 3 3; stroke-opacity: .75; }
.ring-piece.is-ghost .piece-wall { fill-opacity: .05; }
.ring-piece.is-focus { filter: drop-shadow(0 0 5px color-mix(in srgb, var(--card-accent) 55%, transparent)); }
.icon-shadow { fill: #000; pointer-events: none; }

.ring-icon { position: absolute; z-index: 2; display: flex; pointer-events: none; transition: opacity 120ms; filter: drop-shadow(0 1px 1px rgb(0 0 0 / 0.22)); }

// 环心读数：Oxanium 数字，夜间带一圈面板色光晕，压在底座光源与扫描线上仍清晰。
.ring-readout { position: absolute; z-index: 3; display: flex; flex-direction: column; align-items: center; gap: 1px; text-align: center; transform: translate(-50%, -50%); pointer-events: none; }
.ring-readout strong { color: var(--ds-text-primary); font-size: var(--readout-size); font-weight: 600; line-height: 1.08; white-space: nowrap; text-shadow: 0 0 10px var(--ds-bg-panel), 0 0 2px var(--ds-bg-panel); }
.ring-readout em { margin-left: 3px; color: var(--ds-text-secondary); font-size: .46em; font-style: normal; font-weight: 500; }
.ring-readout small { max-width: 100%; overflow: hidden; color: var(--ds-text-muted); font-size: var(--caption-size); line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
.readout-name { max-width: 100%; overflow: hidden; color: var(--ds-text-secondary); font-size: var(--caption-size); line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }

// 图例：行高 24px，与块同色的等距小方块 + 图标 + 名称 + 占比；超出时在卡片内滚动，底部渐隐。
.ring-legend { align-self: center; max-height: 100%; min-height: 0; margin: 0; padding: 4px 0 12px; overflow-x: hidden; overflow-y: auto; list-style: none; scrollbar-width: thin; mask-image: linear-gradient(to bottom, #000 calc(100% - 12px), transparent); }
.legend-row { display: grid; width: 100%; min-height: 24px; grid-template-columns: 10px 16px minmax(0, 1fr) auto; align-items: center; gap: 6px; border: 0; border-radius: 2px; padding: 2px 6px; background: transparent; color: var(--ds-text-secondary); font: inherit; font-size: 11.5px; text-align: left; cursor: pointer; transition: background-color 120ms, color 120ms, opacity 160ms; }
.legend-row:hover, .legend-row.is-active { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.legend-row.is-pinned { background: var(--ds-active-bg); color: var(--ds-text-primary); }
.legend-row.is-muted:not(:hover) { opacity: .5; }
.legend-row:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: -1px; }
.legend-cube { width: 10px; height: 11px; }
.legend-cube.is-ghost { fill: none; stroke: var(--hud-line); stroke-dasharray: 2 1.5; stroke-width: .9; }
.legend-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.legend-share { color: var(--ds-text-primary); font-size: 11.5px; font-weight: 600; }

// 窄卡片：收起图例，环占满卡片；读数与悬停仍给出每项占比。
@container ring-card (max-width: 340px) {
  .card-body { grid-template-columns: minmax(0, 1fr); }
  .ring-legend { display: none; }
}
// 偏高的卡片：图例移到环下方并排成两列，环可以放得更大（宽高比 1.4 处两种排布的环半径相同）。
@container ring-card (min-width: 341px) and (aspect-ratio < 1.4) {
  .card-body { grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(0, 1fr) auto; padding-left: 6px; }
  .ring-legend { display: grid; max-height: 34cqh; align-self: stretch; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 6px; padding-top: 0; }
}
@container ring-card (max-height: 150px) {
  .card-head { padding-top: 5px; }
}
@keyframes ring-scan { to { stroke-dashoffset: -100; } }
@media (prefers-reduced-motion: reduce) {
  .ground-scan-head { animation: none; stroke-dasharray: none; opacity: .3; }
  .ring-icon, .legend-row, .card-link { transition: none; }
}
</style>
