<script setup lang="ts">
import {computed, onBeforeUnmount, ref, shallowRef, watch} from 'vue'
import {Area, Axis, Canvas, Chart, Line, Point, createRef, jsx} from '@antv/f2'
import AnalysisPage from './analysis-page.vue'
import ItemIcon from '../item-icon.vue'
import type {quant} from '../../../../../wailsjs/go/models'
import type {FactoryStatistics} from '../../../../types/factory-statistics'
import {rampAt, type AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {ensureFactoryIcons} from '../../../../pages/services/dysonSphere/factory-icons'
import {buildLineRows, formatPercent, formatPower, formatValue, sprayModeLabel} from '../../../../pages/services/dysonSphere/factory-analysis'

// =============== 产线负载 ===============
// 说明：AntV F2 雷达图。每条产线一根辐条，半径 = 期望产量 ÷ 整数设备满载产能（0–100%）；产线按负载从高到低自 12 点顺时针排列，
//       图形向内收的那一段就是取整浪费最多的产线。琥珀虚线环为按设备台数加权的平均负载，雷达扫描线只作装饰。
// 交互：指针靠近哪根辐条就高亮哪条产线，浮层给出设备、需求台数、喷涂与功率；辐条末端是产物图标。
// 依赖：F2 绘制网格、面积、折线与数据点；图标、平均环、悬停高亮与浮层是同一像素坐标下的 HTML / SVG 覆盖层，坐标取自 F2 渲染后的极坐标系。
// 边界：负载是静态预测，逐线满载不代表全厂可同时满产；少于 3 条产线时雷达图围不出面积，改用列表。
const props = defineProps<{stats: FactoryStatistics; palette: AnalysisPalette; plan: quant.Result | null; itemName: (id: number) => string}>()
ensureFactoryIcons()

const rows = computed(() => buildLineRows(props.stats, props.plan, props.itemName).sort((a, b) => b.utilization - a.utilization || a.index - b.index))
const machines = computed(() => rows.value.reduce((total, row) => total + row.count, 0))
const average = computed(() => (machines.value > 0 ? rows.value.reduce((total, row) => total + row.utilization * row.count, 0) / machines.value : 0))
const asRadar = computed(() => rows.value.length >= 3)
// F2 的分类值必须唯一：同一产物可能有多条产线，用排序后的序号作分类，显示名放在覆盖层。
const records = computed(() => rows.value.map((row, rank) => ({line: `L${rank}`, load: Math.min(1, Math.max(0, row.utilization))})))

// =============== 画幅 ===============
const plot = ref<HTMLElement | null>(null)
const canvasEl = ref<HTMLCanvasElement | null>(null)
const size = shallowRef({width: 0, height: 0})
watch(plot, (element, _previous, onCleanup) => {
  if (!element) return
  const observer = new ResizeObserver(([entry]) => {
    if (!entry) return
    const width = Math.floor(entry.contentRect.width)
    const height = Math.floor(entry.contentRect.height)
    if (size.value.width !== width || size.value.height !== height) size.value = {width, height}
  })
  observer.observe(element)
  onCleanup(() => observer.disconnect())
}, {flush: 'post'})
/** 辐条末端图标：按外圈弧长均分，12–26px；图表四周留出图标的位置 */
const iconSize = computed(() => {
  const radius = Math.max(40, Math.min(size.value.width, size.value.height) / 2 - 40)
  return Math.round(Math.min(26, Math.max(12, 2 * Math.PI * (radius + 14) / Math.max(1, rows.value.length) * 0.62)))
})

// =============== 配色 ===============
// 说明：负载沿用建筑蓝明度梯：面积半透明、折线取亮色，数据点按负载取明度；平均环用全局强调色琥珀。网格与刻度只用低透明度的同色相。
const colors = computed(() => {
  const p = props.palette
  const ramp = p.shareRamps.building
  return {
    line: rampAt(ramp, p.dark ? 0.95 : 0.82),
    grid: p.dark ? 'rgba(134, 211, 255, 0.14)' : 'rgba(47, 98, 159, 0.16)',
    rim: p.dark ? 'rgba(134, 211, 255, 0.34)' : 'rgba(47, 98, 159, 0.34)',
    halo: p.dark ? '#0b111d' : '#f4f6fa',
    average: p.hero,
    point: (load: number): string => rampAt(ramp, 0.35 + 0.65 * load),
  }
})

// =============== F2 雷达图 ===============
// 说明：分类轴只画辐条与外圈多边形，不画文字（图标由覆盖层放在辐条末端）；数值轴画 25% 一档的多边形网格，刻度文字沿 12 点辐条。
// 边界：F2 文档用 JSX 书写，这里在 .vue 中直接调用同一个 jsx 函数；其经典签名要求组件构造参数与通用 IProps 兼容，
//       F2 的泛型组件不满足，故经由宽松签名调用，运行时行为不变。
const h = jsx as unknown as (type: unknown, config: Record<string, unknown> | null, ...children: unknown[]) => ReturnType<typeof jsx>
const chartRef = createRef<Chart>()
const buildTree = (context: CanvasRenderingContext2D, width: number, height: number) => {
  const p = props.palette
  const c = colors.value
  const pad = iconSize.value + 18
  return h(Canvas, {context, pixelRatio: Math.min(2, window.devicePixelRatio || 1), width, height, animate: false, theme: {fontFamily: p.fontDisplay, fontSize: 10}},
    h(Chart, {
      ref: chartRef, data: records.value, coord: {type: 'polar'}, style: {padding: [pad, pad, pad, pad]},
      scale: {line: {type: 'cat', values: records.value.map(record => record.line)}, load: {min: 0, max: 1, nice: false, tickCount: 5}},
    },
      h(Axis, {field: 'line', grid: 'line', style: {label: null, tickLine: null, grid: {stroke: c.grid, lineWidth: 1, lineDash: [2, 4]}, line: {stroke: c.rim, lineWidth: 1}}}),
      h(Axis, {
        field: 'load', grid: 'line', formatter: (value: number) => `${Math.round(value * 100)}%`,
        style: {line: null, tickLine: null, labelOffset: 5, grid: {stroke: c.grid, lineWidth: 1}, label: (_text: string, index: number) => ({fill: index === 0 ? 'transparent' : p.muted, fontSize: 9})},
      }),
      h(Area, {x: 'line', y: 'load', color: c.line, style: {fillOpacity: p.dark ? 0.2 : 0.14}}),
      h(Line, {x: 'line', y: 'load', color: c.line, size: 1.6, style: {lineJoin: 'round', shadowColor: p.dark ? c.line : 'transparent', shadowBlur: p.dark ? 8 : 0}}),
      h(Point, {x: 'line', y: 'load', size: 3.2, color: {field: 'load', callback: c.point}, style: {stroke: c.halo, lineWidth: 1}}),
    ),
  )
}

// 渲染串行化：F2 的 render / update / resize 都是异步的，连续的数据、主题与尺寸变化只消费最新状态；卸载后不再触碰画布。
interface Spoke { readonly angle: number; readonly ux: number; readonly uy: number }
interface RadarGeometry { readonly cx: number; readonly cy: number; readonly radius: number; readonly spokes: readonly Spoke[] }
const geometry = shallowRef<RadarGeometry | null>(null)
const failed = ref(false)
let canvas: Canvas | null = null
let boundTo: HTMLCanvasElement | null = null
let drawn = {width: 0, height: 0}
let running = false
let again = false
let disposed = false
const release = (): void => {
  canvas?.destroy()
  canvas = null
  boundTo = null
  drawn = {width: 0, height: 0}
  geometry.value = null
}
// 覆盖层坐标直接读 F2 的极坐标系：圆心、半径与每个分类的辐条方向，不自行推算角度，图标与网格不会错位。
const readGeometry = (): void => {
  const chart = chartRef.current
  if (!chart) return
  const coord = chart.getCoord() as unknown as {center?: {x: number; y: number}; radius?: number; convertPoint: (point: {x: number; y: number}) => {x: number; y: number}}
  const scale = chart.getScale('line')
  const center = coord.center
  const radius = coord.radius ?? 0
  if (!center || !(radius > 0)) return
  geometry.value = {cx: center.x, cy: center.y, radius, spokes: records.value.map(record => {
    const end = coord.convertPoint({x: scale.scale(record.line), y: 1})
    const dx = end.x - center.x
    const dy = end.y - center.y
    const length = Math.hypot(dx, dy) || 1
    return {angle: Math.atan2(dy, dx), ux: dx / length, uy: dy / length}
  })}
}
const draw = async (): Promise<void> => {
  if (running) { again = true; return }
  running = true
  try {
    do {
      again = false
      const element = canvasEl.value
      const {width, height} = size.value
      if (!asRadar.value || !element) { release(); continue }
      if (width < 120 || height < 120) continue
      if (canvas && boundTo !== element) release()
      const context = element.getContext('2d')
      if (!context) { failed.value = true; continue }
      const tree = buildTree(context, width, height)
      if (!canvas) {
        canvas = new Canvas(tree.props)
        boundTo = element
        await canvas.render()
      } else {
        // F2 的 resize 会先按旧图形绘制一次；在更新树之前统一调整底图和布局，只提交最终画面。
        if (drawn.width !== width || drawn.height !== height) {
          canvas.context.canvas.resize(width, height)
          canvas.updateLayout(tree.props)
        }
        await canvas.update(tree.props)
      }
      if (disposed) return
      canvas.context.canvas.render()
      drawn = {width, height}
      readGeometry()
      failed.value = false
    } while (again && !disposed)
  } catch (error) {
    if (!disposed) { failed.value = true; console.error('F2 产线负载雷达图绘制失败', error) }
  } finally {
    running = false
  }
}
watch([records, () => props.palette, size, iconSize, canvasEl], () => { void draw() }, {flush: 'post'})
onBeforeUnmount(() => {
  disposed = true
  release()
})

// =============== 覆盖层 ===============
// 说明：辐条末端立产物图标；平均负载画成与网格同形的琥珀虚线多边形，标签贴在 12 点辐条右侧；悬停时补一根亮辐条和数据点光圈。
const overlay = computed(() => {
  const g = geometry.value
  if (!g || g.spokes.length !== rows.value.length) return null
  const reach = g.radius + iconSize.value / 2 + 8
  const at = (spoke: Spoke, ratio: number): [number, number] => [g.cx + spoke.ux * g.radius * ratio, g.cy + spoke.uy * g.radius * ratio]
  return {
    icons: g.spokes.map((spoke, rank) => ({rank, name: rows.value[rank].name, x: g.cx + spoke.ux * reach, y: g.cy + spoke.uy * reach})),
    average: g.spokes.map(spoke => at(spoke, average.value).map(value => value.toFixed(1)).join(',')).join(' '),
    averageLabel: {x: g.cx + 6, y: g.cy - g.radius * average.value - 4},
    at,
  }
})
const hovered = ref<number | null>(null)
const tip = shallowRef<{x: number; y: number; left: boolean; above: boolean} | null>(null)
const focus = computed(() => {
  const g = geometry.value
  const layer = overlay.value
  const rank = hovered.value
  if (!g || !layer || rank === null || !rows.value[rank]) return null
  const spoke = g.spokes[rank]
  const [px, py] = layer.at(spoke, rows.value[rank].utilization)
  const [ex, ey] = layer.at(spoke, 1)
  return {row: rows.value[rank], px, py, ex, ey}
})
// 指针按角度吸附到最近的辐条：只在雷达外圈与图标范围内响应，靠近圆心时不判定。
const onMove = (event: PointerEvent): void => {
  const g = geometry.value
  const element = plot.value
  if (!g || !element || !g.spokes.length) return
  const rect = element.getBoundingClientRect()
  const x = event.clientX - rect.left
  const y = event.clientY - rect.top
  const distance = Math.hypot(x - g.cx, y - g.cy)
  if (distance < 8 || distance > g.radius + iconSize.value + 16) { hovered.value = null; return }
  const angle = Math.atan2(y - g.cy, x - g.cx)
  let best = 0
  let closest = Infinity
  g.spokes.forEach((spoke, rank) => {
    const delta = Math.abs(Math.atan2(Math.sin(angle - spoke.angle), Math.cos(angle - spoke.angle)))
    if (delta < closest) { closest = delta; best = rank }
  })
  hovered.value = best
  tip.value = {x, y, left: x > rect.width - 300, above: y > rect.height * 0.55}
}
const onLeave = (): void => { hovered.value = null }
const tipStyle = computed(() => {
  const value = tip.value
  if (!value) return {}
  return {left: `${value.x}px`, top: `${value.y}px`, transform: `translate(${value.left ? 'calc(-100% - 16px)' : '16px'}, ${value.above ? 'calc(-100% - 12px)' : '16px'})`}
})
const ariaLabel = computed(() => `产线负载雷达图：平均 ${formatPercent(average.value)}，${rows.value.map(row => `${row.name} ${formatPercent(row.utilization)}`).join('，')}`)
</script>

<template>
  <!-- ~ 产线负载 -->
  <!-- * 说明：F2 雷达图，每条产线一根辐条，半径为负载；琥珀虚线为台数加权平均，辐条末端为产物图标 -->
  <!-- * 交互：指针靠近辐条高亮该产线并显示设备、需求台数、喷涂与功率 -->
  <!-- * 依赖：统计报告 lines；主产物名称取自同一量化方案 -->
  <!-- * 边界：静态预测，不代表游戏实测；少于 3 条产线时改用列表 -->
  <AnalysisPage>
    <div class="radar">
      <div ref="plot" class="radar-plot" :class="{'has-focus': focus}" @pointermove="onMove" @pointerleave="onLeave">
        <template v-if="asRadar">
          <canvas ref="canvasEl" class="radar-canvas" role="img" :aria-label="ariaLabel" />
          <!-- 雷达扫描：沿外接圆缓慢旋转的渐隐扇面，只作装饰 -->
          <div v-if="geometry" class="radar-sweep" :style="{left: `${geometry.cx}px`, top: `${geometry.cy}px`, width: `${geometry.radius * 2}px`, height: `${geometry.radius * 2}px`, '--sweep': colors.line}" aria-hidden="true" />
          <svg v-if="overlay" class="radar-overlay" aria-hidden="true">
            <polygon class="average-ring" :points="overlay.average" :stroke="colors.average" />
            <text class="average-label type-display" :x="overlay.averageLabel.x" :y="overlay.averageLabel.y" :fill="colors.average">平均 {{ formatPercent(average) }}</text>
            <template v-if="focus">
              <line class="focus-spoke" :x1="geometry!.cx" :y1="geometry!.cy" :x2="focus.ex" :y2="focus.ey" :stroke="colors.line" />
              <circle class="focus-ring" :cx="focus.px" :cy="focus.py" r="7" :stroke="colors.line" />
            </template>
          </svg>
          <template v-if="overlay">
            <span v-for="icon in overlay.icons" :key="icon.rank" class="radar-icon" :class="{'is-active': hovered === icon.rank}" :style="{left: `${icon.x}px`, top: `${icon.y}px`}"><ItemIcon :name="icon.name" :size="iconSize" /></span>
          </template>
          <!-- 左上角摘要：台数加权平均负载与规模 -->
          <div class="radar-summary">
            <span>平均负载</span>
            <strong class="type-display">{{ formatPercent(average) }}</strong>
            <small>{{ rows.length }} 条产线 · {{ machines }} 台设备</small>
          </div>
        </template>
        <!-- 少于 3 条产线：雷达图围不出面积，逐条列出负载 -->
        <ul v-else-if="rows.length" class="radar-list">
          <li v-for="row in rows" :key="row.index">
            <ItemIcon :name="row.name" :size="22" />
            <span class="list-name">{{ row.name }}</span>
            <span class="list-bar"><i :style="{width: `${row.utilization * 100}%`, background: colors.line}" /></span>
            <b class="type-display">{{ formatPercent(row.utilization) }}</b>
            <small>{{ row.machineName }} × {{ row.count }}</small>
          </li>
        </ul>
        <p v-else class="radar-empty">当前方案没有产线</p>
        <p v-if="failed" class="radar-empty">图表暂时无法显示</p>

        <!-- 浮层：产物 + 负载，下方为设备、需求台数、喷涂、功率与净输出 -->
        <div v-if="focus && tip" class="radar-tip" :style="tipStyle" role="tooltip">
          <header><ItemIcon :name="focus.row.name" :size="20" /><strong>{{ focus.row.name }}</strong><span class="type-display">{{ formatPercent(focus.row.utilization, 1) }}</span></header>
          <div class="tip-row"><span>设备</span><b><ItemIcon :name="focus.row.machineName" :size="16" />{{ focus.row.machineName }} × {{ focus.row.count }}</b></div>
          <div class="tip-row"><span>需求台数</span><b class="type-display">{{ formatValue(focus.row.exact) }}（多出 {{ formatValue(focus.row.idle) }}）</b></div>
          <div class="tip-row"><span>喷涂</span><b>{{ focus.row.mode > 0 ? `${sprayModeLabel(focus.row.mode)} · ${focus.row.points} 点` : '不喷涂' }}</b></div>
          <div class="tip-row"><span>{{ focus.row.expectedMw != null ? '预测 / 额定功率' : '额定功率' }}</span><b class="type-display">{{ focus.row.expectedMw != null ? `${formatPower(focus.row.expectedMw)} / ` : '' }}{{ formatPower(focus.row.configuredMw) }}</b></div>
          <div v-if="focus.row.line.netOutputs.length" class="tip-row is-muted"><span>净输出</span><b>{{ focus.row.line.netOutputs.slice(0, 3).map(rate => `${rate.name} ${formatValue(rate.ratePerMinute)}/min`).join(' · ') }}</b></div>
        </div>
      </div>

      <!-- 图例：负载面积、平均虚线与读法说明 -->
      <footer v-if="rows.length" class="radar-legend">
        <span class="legend-key"><i class="key-area" :style="{background: colors.line}" />产线负载</span>
        <span class="legend-key"><i class="key-dash" :style="{borderColor: colors.average}" />台数加权平均 {{ formatPercent(average) }}</span>
        <span class="legend-note">负载 = 期望产量 ÷ 整数设备满载产能，自 12 点起按负载从高到低顺时针排列</span>
      </footer>
    </div>
  </AnalysisPage>
</template>

<style scoped lang="scss">
.radar { position: absolute; inset: 0; display: flex; flex-direction: column; container-type: inline-size; }
.radar-plot { position: relative; min-height: 0; flex: 1; overflow: hidden; }
.radar-canvas { position: absolute; inset: 0; width: 100%; height: 100%; }

// 扫描扇面：亮边在前、尾迹渐隐，顺时针一圈 9 秒；透明度很低，不改变数据面积的读数。
.radar-sweep { position: absolute; z-index: 1; border-radius: 50%; background: conic-gradient(from 0deg, transparent 0deg 300deg, color-mix(in srgb, var(--sweep) 14%, transparent) 356deg, color-mix(in srgb, var(--sweep) 30%, transparent) 359.5deg, transparent 360deg); transform: translate(-50%, -50%); animation: radar-sweep 9s linear infinite; pointer-events: none; }
.radar-overlay { position: absolute; inset: 0; z-index: 2; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.average-ring { fill: none; stroke-width: 1.2; stroke-dasharray: 5 4; opacity: .9; }
.average-label { font-size: 10px; font-weight: 600; }
.focus-spoke { stroke-width: 1.2; opacity: .85; }
.focus-ring { fill: none; stroke-width: 1.5; }

// 辐条末端图标：悬停时放大并发光，其余淡化。
.radar-icon { position: absolute; z-index: 3; display: flex; transform: translate(-50%, -50%); pointer-events: none; transition: opacity 150ms, transform 150ms; filter: drop-shadow(0 1px 1px rgb(0 0 0 / 0.25)); }
.has-focus .radar-icon:not(.is-active) { opacity: .4; }
.radar-icon.is-active { transform: translate(-50%, -50%) scale(1.18); filter: drop-shadow(0 0 5px var(--ds-brand)); }

.radar-summary { position: absolute; top: 12px; left: 16px; z-index: 3; display: flex; flex-direction: column; gap: 1px; pointer-events: none; }
.radar-summary span { color: var(--ds-text-muted); font-size: 11px; }
.radar-summary strong { color: var(--ds-text-primary); font-size: 24px; font-weight: 600; line-height: 1.1; }
.radar-summary small { color: var(--ds-text-muted); font-size: 11px; }

.radar-list { display: flex; height: 100%; box-sizing: border-box; flex-direction: column; justify-content: center; gap: 10px; margin: 0; padding: 16px 24px; list-style: none; }
.radar-list li { display: grid; grid-template-columns: 22px minmax(80px, 160px) minmax(0, 1fr) 44px auto; align-items: center; gap: 10px; color: var(--ds-text-secondary); font-size: 12px; }
.list-name { overflow: hidden; color: var(--ds-text-primary); text-overflow: ellipsis; white-space: nowrap; }
.list-bar { height: 8px; overflow: hidden; border-radius: 4px; background: var(--ds-hover-bg); }
.list-bar i { display: block; height: 100%; border-radius: 4px; }
.radar-list b { color: var(--ds-text-primary); text-align: right; }
.radar-list small { color: var(--ds-text-muted); }
.radar-empty { position: absolute; inset: 0; display: grid; margin: 0; place-items: center; color: var(--ds-text-muted); font-size: 12px; pointer-events: none; }

// 浮层：与物料统计页同一套圆角卡片样式。
.radar-tip { position: absolute; z-index: 5; min-width: 230px; max-width: 320px; border: 1px solid var(--ds-border-default); border-radius: 8px; padding: 10px 12px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; box-shadow: 0 12px 30px rgb(0 0 0 / 0.3); pointer-events: none; }
.radar-tip header { display: flex; min-width: 0; align-items: center; gap: 7px; margin-bottom: 4px; }
.radar-tip header strong { overflow: hidden; color: var(--ds-text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.radar-tip header span { flex: 0 0 auto; margin-left: auto; padding-left: 12px; color: var(--ds-text-primary); font-size: 14px; font-weight: 600; }
.tip-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 22px; }
.tip-row > span { flex: 0 0 auto; }
.tip-row b { display: inline-flex; min-width: 0; align-items: center; gap: 5px; overflow: hidden; color: var(--ds-text-primary); font-weight: 600; text-align: right; text-overflow: ellipsis; white-space: nowrap; }
.tip-row.is-muted b { color: var(--ds-text-muted); font-weight: 400; }

.radar-legend { display: flex; flex: 0 0 auto; flex-wrap: wrap; align-items: center; gap: 4px 18px; padding: 6px 16px 12px; color: var(--ds-text-secondary); font-size: 12.5px; }
.legend-key { display: inline-flex; align-items: center; gap: 7px; }
.key-area { width: 11px; height: 11px; border-radius: 3px; opacity: .7; }
.key-dash { width: 16px; height: 0; border-top: 2px dashed; }
.legend-note { color: var(--ds-text-muted); font-size: 11px; }

@keyframes radar-sweep { to { transform: translate(-50%, -50%) rotate(360deg); } }
// 窄画幅：雷达贴近左缘，收起左上角摘要，平均值仍在图例与平均环标签上。
@container (max-width: 560px) {
  .radar-summary { display: none; }
}
@media (prefers-reduced-motion: reduce) {
  .radar-sweep { display: none; }
  .radar-icon { transition: none; }
}
</style>
