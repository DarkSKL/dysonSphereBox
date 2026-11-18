<script setup lang="ts">
import {computed, ref, shallowRef, watch} from 'vue'
import AnalysisPage from './analysis-page.vue'
import ItemIcon from '../item-icon.vue'
import type {FactoryStatistics} from '../../../../types/factory-statistics'
import {mixHex, rampAt, type AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {ensureFactoryIcons} from '../../../../pages/services/dysonSphere/factory-icons'
import {buildBoundaryRows, formatPower, formatShare, formatValue} from '../../../../pages/services/dysonSphere/factory-analysis'
import {BOUNDARY_FLOWS, boundaryRateOf, boundarySegments, treemapLayout, waffleLayout, type BlockRect, type BoundaryFlow} from '../../../../pages/services/dysonSphere/analysis/boundary-layout'

// =============== 物料边界 ===============
// 说明：工厂每分钟吃进与吐出的物料。外部输入、目标交付、持续剩余三种流向各一种颜色，每个方块里是对应物品的图标。
//       「网格」视图每格约等于同一速率，读构成与量级；「方块」视图按速率分面积，输入在左、输出在右，读精确比例。
// 交互：悬停方块高亮同一物料在该流向上的全部格子并淡化其余；悬停或聚焦图例高亮整个流向并列出其中的物料；右下角切换视图，选择记在本机。
// 边界：纯内部中间品不进入边界；网格取整且每段至少一格，精确速率以浮层为准；持续剩余用琥珀色，表示必须接出口外运，缓存不能抵消。
const props = defineProps<{stats: FactoryStatistics; palette: AnalysisPalette}>()
ensureFactoryIcons()

const segments = computed(() => boundarySegments(buildBoundaryRows(props.stats)))
const totals = computed(() => {
  const result: Record<BoundaryFlow, number> = {input: 0, target: 0, surplus: 0}
  for (const segment of segments.value) result[segment.flow] += segment.rate
  return result
})
const grand = computed(() => totals.value.input + totals.value.target + totals.value.surplus)
const flows = computed(() => BOUNDARY_FLOWS.filter(flow => totals.value[flow.key] > 1e-9))
// 数据变化时重播填充动画并清除悬停：容器按版本号重建；尺寸变化只重排，不重播。
const version = ref(0)
watch(segments, () => { version.value++; focus.value = null })

// =============== 视图 ===============
type BoundaryView = 'grid' | 'blocks'
const VIEW_KEY = 'dsbox.analysis.boundaryView'
const readView = (): BoundaryView => {
  try { return localStorage.getItem(VIEW_KEY) === 'blocks' ? 'blocks' : 'grid' } catch { return 'grid' }
}
const view = ref<BoundaryView>(readView())
watch(view, value => {
  focus.value = null
  try { localStorage.setItem(VIEW_KEY, value) } catch { /* 存储不可用时只保留本次会话 */ }
})

// =============== 画幅 ===============
const root = ref<HTMLElement | null>(null)
const plot = ref<HTMLElement | null>(null)
const size = shallowRef({width: 0, height: 0})
watch(plot, (element, _previous, onCleanup) => {
  if (!element) return
  const observer = new ResizeObserver(([entry]) => {
    if (entry) size.value = {width: Math.floor(entry.contentRect.width), height: Math.floor(entry.contentRect.height)}
  })
  observer.observe(element)
  onCleanup(() => observer.disconnect())
}, {flush: 'post'})
const waffle = computed(() => (view.value === 'grid' ? waffleLayout(segments.value, size.value.width, size.value.height) : null))
const treemap = computed(() => (view.value === 'blocks' ? treemapLayout(segments.value, size.value.width, size.value.height) : null))
const cellIcon = computed(() => (waffle.value ? Math.round(waffle.value.size * 0.68) : 0))

// =============== 配色 ===============
// 说明：输入、交付沿用占比环的建筑蓝与生产青，持续剩余用全局唯一强调色琥珀；格子是色相混入底色的实心块，
//       图标始终压在中等明度上，描边与悬停辉光用流向原色。文字只用文本色，不用流向色。
const flowColor = computed<Record<BoundaryFlow, string>>(() => {
  const p = props.palette
  return {input: rampAt(p.shareRamps.building, p.dark ? 1 : 0.66), target: rampAt(p.shareRamps.production, p.dark ? 1 : 0.66), surplus: p.hero}
})
const flowVars = computed(() => {
  const p = props.palette
  const base = p.dark ? '#0b111d' : '#ffffff'
  const varsOf = (color: string): Record<string, string> => ({'--flow': color, '--flow-fill': mixHex(color, base, p.dark ? 0.64 : 0.72), '--flow-lit': mixHex(color, base, p.dark ? 0.4 : 0.5)})
  return {input: varsOf(flowColor.value.input), target: varsOf(flowColor.value.target), surplus: varsOf(flowColor.value.surplus)}
})
const cellStyle = (segment: number, order: number): Record<string, string | number> => ({...flowVars.value[segments.value[segment].flow], '--i': order})
const blockStyle = (rect: BlockRect, segment: number, order: number): Record<string, string | number> => ({
  ...cellStyle(segment, order), left: `${rect.x}px`, top: `${rect.y}px`, width: `${rect.width}px`, height: `${rect.height}px`,
})
/** 方块里的图标：边长的 42%，最大 56px；小于 14px 时不画，交给浮层 */
const blockIcon = (rect: BlockRect): number => Math.round(Math.min(56, Math.min(rect.width, rect.height) * 0.42))
const blockLabeled = (rect: BlockRect): boolean => rect.width >= 84 && rect.height >= 76

// =============== 悬停与浮层 ===============
// 说明：悬停方块聚焦「物料 × 流向」这一段；悬停图例聚焦整个流向。浮层跟随指针，靠近右侧或下方时翻到另一侧，不挡住指针所在的格子。
type BoundaryFocus = {readonly kind: 'segment'; readonly index: number} | {readonly kind: 'flow'; readonly flow: BoundaryFlow}
const focus = shallowRef<BoundaryFocus | null>(null)
const tip = shallowRef<{x: number; y: number; left: boolean; above: boolean} | null>(null)
const lit = (index: number): boolean => {
  const value = focus.value
  return value !== null && (value.kind === 'segment' ? value.index === index : segments.value[index]?.flow === value.flow)
}
const placeTip = (clientX: number, clientY: number, above?: boolean): void => {
  const rect = root.value?.getBoundingClientRect()
  if (!rect) return
  const x = clientX - rect.left
  const y = clientY - rect.top
  tip.value = {x, y, left: x > rect.width - 280, above: above ?? y > rect.height * 0.55}
}
const segmentAt = (target: EventTarget | null): number | null => {
  const value = target instanceof Element ? target.closest('[data-seg]')?.getAttribute('data-seg') : null
  return value == null ? null : Number(value)
}
const onPlotMove = (event: PointerEvent): void => {
  const index = segmentAt(event.target)
  if (index === null) {
    // 指针落在格子之间的缝隙上时保持当前高亮，避免在相邻格子间移动时浮层闪烁；空仓位与边距才取消。
    const onGap = event.target instanceof Element && (event.target.classList.contains('waffle') || event.target.classList.contains('blocks'))
    if (!onGap && focus.value?.kind === 'segment') focus.value = null
    return
  }
  if (focus.value?.kind !== 'segment' || focus.value.index !== index) focus.value = {kind: 'segment', index}
  placeTip(event.clientX, event.clientY)
}
const onPlotLeave = (): void => { if (focus.value?.kind === 'segment') focus.value = null }
// 图例浮层锚定在图例项上方，键盘聚焦与指针悬停表现一致。
const onLegendEnter = (flow: BoundaryFlow, event: Event): void => {
  focus.value = {kind: 'flow', flow}
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  placeTip(rect.left, rect.top, true)
}
const onLegendLeave = (): void => { if (focus.value?.kind === 'flow') focus.value = null }
const tipStyle = computed(() => {
  const value = tip.value
  if (!value) return {}
  return {left: `${value.x}px`, top: `${value.y}px`, transform: `translate(${value.left ? 'calc(-100% - 14px)' : '14px'}, ${value.above ? 'calc(-100% - 10px)' : '14px'})`}
})

// 浮层内容：标题行为流向色块 + 物品图标 + 名称与速率；首行给出所占比例，其余行补充同一物料的其他流向、本厂产出与消耗。
interface TipRow { readonly key: string; readonly label: string; readonly value: string; readonly icon?: string; readonly tone?: 'strong' | 'muted' }
interface TipContent { readonly color: string; readonly icon?: string; readonly title: string; readonly rate: string; readonly rows: readonly TipRow[] }
const labelOf = (flow: BoundaryFlow): string => BOUNDARY_FLOWS.find(item => item.key === flow)?.label ?? flow
const tipContent = computed<TipContent | null>(() => {
  const value = focus.value
  if (!value) return null
  if (value.kind === 'flow') {
    const members = segments.value.filter(segment => segment.flow === value.flow)
    const rest = members.slice(6)
    return {
      color: flowColor.value[value.flow], title: labelOf(value.flow), rate: `${formatValue(totals.value[value.flow])} /min`,
      rows: [
        {key: 'share', label: '占边界流量', value: formatShare(grand.value > 0 ? totals.value[value.flow] / grand.value : 0), tone: 'strong'},
        ...members.slice(0, 6).map(segment => ({key: `item-${segment.row.itemId}`, icon: segment.row.name, label: segment.row.name, value: formatValue(segment.rate)})),
        ...(rest.length ? [{key: 'rest', label: `其余 ${rest.length} 种`, value: formatValue(rest.reduce((sum, segment) => sum + segment.rate, 0)), tone: 'muted' as const}] : []),
      ],
    }
  }
  const segment = segments.value[value.index]
  if (!segment) return null
  const material = segment.row.material
  const heat = material.heatValueMj != null && material.heatValueMj > 0 ? (segment.flow === 'input' ? material.importPotentialMw : material.exportPotentialMw) : 0
  return {
    color: flowColor.value[segment.flow], icon: segment.row.name, title: `${segment.row.name}${material.jointProduct ? '（联产物）' : ''}`, rate: `${formatValue(segment.rate)} /min`,
    rows: [
      {key: 'share', label: `占${labelOf(segment.flow)}`, value: formatShare(totals.value[segment.flow] > 0 ? segment.rate / totals.value[segment.flow] : 0), tone: 'strong'},
      ...BOUNDARY_FLOWS.filter(flow => flow.key !== segment.flow && boundaryRateOf(segment.row, flow.key) > 1e-9)
        .map(flow => ({key: flow.key, label: `另有${flow.label}`, value: `${formatValue(boundaryRateOf(segment.row, flow.key))} /min`})),
      ...(material.producedPerMinute > 1e-9 ? [{key: 'produced', label: '本厂净产出', value: `${formatValue(material.producedPerMinute)} /min`, tone: 'muted' as const}] : []),
      ...(material.consumedPerMinute > 1e-9 ? [{key: 'consumed', label: '内部净消耗', value: `${formatValue(material.consumedPerMinute)} /min`, tone: 'muted' as const}] : []),
      ...(heat > 0 ? [{key: 'heat', label: '热值通量', value: formatPower(heat), tone: 'muted' as const}] : []),
    ],
  }
})
const ariaLabel = computed(() => flows.value.map(flow => `${flow.label} ${formatValue(totals.value[flow.key])} 件/min`).join('，'))
</script>

<template>
  <!-- ~ 物料边界 -->
  <!-- * 说明：三种流向的物料方块，每块是对应物品的图标；网格视图读构成，方块视图读精确比例 -->
  <!-- * 交互：悬停方块或图例高亮并显示浮层；右下角切换网格 / 方块视图 -->
  <!-- * 依赖：统计报告 materials 的外部输入、目标交付与持续剩余 -->
  <!-- * 边界：只含跨越工厂边界的物料；网格每格为近似单位，精确值见浮层 -->
  <AnalysisPage>
    <div ref="root" class="ledger">
      <div ref="plot" class="ledger-plot" role="img" :aria-label="`物料边界：${ariaLabel}`" @pointermove="onPlotMove" @pointerleave="onPlotLeave">
        <!-- 网格：阅读顺序依次为外部输入、目标交付、持续剩余，其余位置是空仓位 -->
        <div v-if="waffle" :key="`grid-${version}`" class="waffle" :class="{'has-focus': focus}" :style="{gridTemplateColumns: `repeat(${waffle.columns}, ${waffle.size}px)`, gridAutoRows: `${waffle.size}px`, gap: `${waffle.gap}px`}">
          <span v-for="(segment, order) in waffle.cells" :key="order" class="cell" :class="segment === null ? 'is-slot' : {'is-lit': lit(segment)}" :data-seg="segment ?? undefined" :style="segment === null ? undefined : cellStyle(segment, order)">
            <ItemIcon v-if="segment !== null" :name="segments[segment].row.name" :size="cellIcon" />
          </span>
        </div>
        <!-- 方块：输入、交付、剩余三条并列条带，块面积与速率成正比 -->
        <div v-else-if="treemap" :key="`blocks-${version}`" class="blocks" :class="{'has-focus': focus}" :style="{width: `${size.width}px`, height: `${size.height}px`}">
          <div v-for="(block, order) in treemap.blocks" :key="block.segment" class="block" :class="{'is-lit': lit(block.segment)}" :data-seg="block.segment" :style="blockStyle(block.rect, block.segment, order)">
            <ItemIcon v-if="blockIcon(block.rect) >= 14" :name="segments[block.segment].row.name" :size="blockIcon(block.rect)" />
            <template v-if="blockLabeled(block.rect)">
              <span class="block-name">{{ segments[block.segment].row.name }}</span>
              <span class="block-rate type-display">{{ formatValue(segments[block.segment].rate) }} /min</span>
            </template>
          </div>
        </div>
        <p v-else-if="!segments.length" class="ledger-empty">当前方案没有跨越工厂边界的物料</p>
      </div>

      <!-- 图例：流向色块 + 名称 + 每分钟合计；悬停或聚焦查看该流向的物料构成 -->
      <footer class="ledger-legend">
        <button v-for="flow in flows" :key="flow.key" type="button" class="legend-item" :class="{'is-active': focus?.kind === 'flow' && focus.flow === flow.key}" @pointerenter="onLegendEnter(flow.key, $event)" @pointerleave="onLegendLeave" @focus="onLegendEnter(flow.key, $event)" @blur="onLegendLeave">
          <i :style="{background: flowColor[flow.key]}" /><span>{{ flow.label }}</span><span class="legend-value type-display">{{ formatValue(totals[flow.key]) }}/min</span>
        </button>
        <span v-if="waffle" class="legend-note">每格 ≈ {{ formatValue(waffle.unit) }} 件/min，每种物料至少一格</span>
        <span v-else-if="treemap" class="legend-note">面积与速率成正比</span>
        <div class="view-toggle" role="radiogroup" aria-label="物料边界视图">
          <button type="button" role="radio" class="toggle" :aria-checked="view === 'blocks'" title="方块：面积与速率成正比" @click="view = 'blocks'">
            <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="1.5" y="1.5" width="6" height="13" rx="1.2" /><rect x="9" y="1.5" width="5.5" height="6.2" rx="1.2" /><rect x="9" y="9.2" width="5.5" height="5.3" rx="1.2" /></svg>
          </button>
          <button type="button" role="radio" class="toggle" :aria-checked="view === 'grid'" title="网格：每格代表相同速率" @click="view = 'grid'">
            <svg viewBox="0 0 16 16" aria-hidden="true"><template v-for="row in 3" :key="row"><rect v-for="column in 3" :key="column" :x="column * 5 - 3.6" :y="row * 5 - 3.6" width="3.2" height="3.2" rx=".8" /></template></svg>
          </button>
        </div>
      </footer>

      <!-- 浮层：流向色块 + 图标 + 名称与速率，下方逐行列出占比与补充信息 -->
      <div v-if="tipContent && tip" class="ledger-tip" :style="tipStyle" role="tooltip">
        <header class="tip-head">
          <i :style="{background: tipContent.color}" />
          <ItemIcon v-if="tipContent.icon" :name="tipContent.icon" :size="18" />
          <strong>{{ tipContent.title }}</strong>
          <span class="type-display">{{ tipContent.rate }}</span>
        </header>
        <div v-for="row in tipContent.rows" :key="row.key" class="tip-row" :class="row.tone ? `is-${row.tone}` : ''">
          <span class="tip-label"><ItemIcon v-if="row.icon" :name="row.icon" :size="16" />{{ row.label }}</span>
          <b class="type-display">{{ row.value }}</b>
        </div>
      </div>
    </div>
  </AnalysisPage>
</template>

<style scoped lang="scss">
.ledger { position: absolute; inset: 0; display: flex; flex-direction: column; }
// 画幅：内容居中；格子超出时纵向滚动，预留滚动条位置，避免出现滚动条后宽度变化导致反复重排。
.ledger-plot { position: relative; display: flex; min-height: 0; flex: 1; overflow-x: hidden; overflow-y: auto; padding: 14px 16px 6px; scrollbar-gutter: stable; scrollbar-width: thin; }
.waffle { display: grid; margin: auto; }
.blocks { position: relative; flex: 0 0 auto; margin: auto; }
.ledger-empty { margin: auto; color: var(--ds-text-muted); font-size: 12px; }

// 方块：流向色混入底色的实心圆角块，内描边为流向原色；悬停块提亮并带同色辉光，其余整体淡化。
.cell, .block { border-radius: 5px; background: var(--flow-fill); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--flow) 50%, transparent), inset 0 1px 0 rgb(255 255 255 / 0.1); transition: opacity 160ms, background-color 160ms, box-shadow 160ms; animation: ledger-tile-in 380ms cubic-bezier(0.2, 0.9, 0.3, 1.15) backwards; }
.cell { display: grid; place-items: center; animation-delay: calc(var(--i, 0) * 3ms); }
.cell.is-slot { background: color-mix(in srgb, var(--ds-text-primary) 4%, transparent); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--ds-text-primary) 5%, transparent); animation: none; }
.block { position: absolute; display: flex; box-sizing: border-box; flex-direction: column; align-items: center; justify-content: center; gap: 4px; overflow: hidden; padding: 6px; color: var(--ds-text-primary); animation-delay: calc(var(--i, 0) * 18ms); }
.cell.is-lit, .block.is-lit { background: var(--flow-lit); box-shadow: inset 0 0 0 1px var(--flow), 0 0 12px color-mix(in srgb, var(--flow) 40%, transparent); }
.has-focus .cell:not(.is-lit), .has-focus .block:not(.is-lit) { opacity: .26; }
.cell :deep(.factory-item-icon), .block :deep(.factory-item-icon) { filter: drop-shadow(0 1px 1px rgb(0 0 0 / 0.28)); }
.block-name { max-width: 100%; overflow: hidden; font-size: 12px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.block-rate { color: var(--ds-text-secondary); font-size: 11px; }

// 图例：流向色块 + 名称 + 合计，右侧为视图切换；与参考设计一致放在图表下方。
.ledger-legend { display: flex; flex: 0 0 auto; flex-wrap: wrap; align-items: center; gap: 4px 18px; padding: 6px 16px 12px; }
.legend-item { display: inline-flex; align-items: center; gap: 7px; border: 0; border-radius: 2px; padding: 3px 4px; background: transparent; color: var(--ds-text-secondary); font: inherit; font-size: 12.5px; cursor: default; transition: background-color 120ms, color 120ms; }
.legend-item:hover, .legend-item.is-active { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.legend-item:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: 1px; }
.legend-item i { width: 11px; height: 11px; flex: 0 0 auto; border-radius: 3px; }
.legend-value { color: var(--ds-text-muted); font-size: 12px; }
.legend-note { color: var(--ds-text-muted); font-size: 11px; }
.view-toggle { display: inline-flex; gap: 2px; margin-left: auto; border-radius: 6px; padding: 2px; background: color-mix(in srgb, var(--ds-text-primary) 6%, transparent); }
.toggle { display: grid; width: 30px; height: 24px; place-items: center; border: 0; border-radius: 4px; padding: 0; background: transparent; color: var(--ds-text-muted); cursor: pointer; transition: background-color 150ms, color 150ms; }
.toggle:hover { color: var(--ds-text-primary); }
.toggle[aria-checked='true'] { background: var(--ds-bg-panel-elevated); color: var(--ds-text-primary); box-shadow: 0 1px 3px rgb(0 0 0 / 0.22); }
.toggle:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: 1px; }
.toggle svg { width: 15px; height: 15px; fill: currentColor; }

// 浮层：圆角深色卡片，标题行右侧为速率，下方左标签右数值。
.ledger-tip { position: absolute; z-index: 5; min-width: 220px; max-width: 300px; border: 1px solid var(--ds-border-default); border-radius: 8px; padding: 10px 12px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; box-shadow: 0 12px 30px rgb(0 0 0 / 0.3); pointer-events: none; }
.tip-head { display: flex; min-width: 0; align-items: center; gap: 7px; margin-bottom: 4px; }
.tip-head i { width: 10px; height: 10px; flex: 0 0 auto; border-radius: 3px; }
.tip-head strong { overflow: hidden; color: var(--ds-text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.tip-head span { flex: 0 0 auto; margin-left: auto; padding-left: 12px; color: var(--ds-text-muted); font-size: 12px; }
.tip-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 22px; }
.tip-label { display: inline-flex; min-width: 0; align-items: center; gap: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tip-row b { flex: 0 0 auto; color: var(--ds-text-primary); font-weight: 600; }
.tip-row.is-strong { color: var(--ds-text-primary); }
.tip-row.is-muted, .tip-row.is-muted b { color: var(--ds-text-muted); font-weight: 400; }

@keyframes ledger-tile-in { from { opacity: 0; transform: scale(.6); } }
@media (prefers-reduced-motion: reduce) {
  .cell, .block { animation: none; transition: none; }
}
</style>
