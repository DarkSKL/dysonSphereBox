<script setup lang="ts">
import {computed, reactive, ref, shallowRef, toRaw, watch} from 'vue'
import type {QuantWorkspace} from '../../../../composables/dysonSphere/useQuantWorkspace'
import {useAnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {factoryStatisticsOf, type FactoryStatistics} from '../../../../types/factory-statistics'
import ShareOverview from './share-overview.vue'
import PowerRange from './power-range.vue'
import LineLoad from './line-load.vue'
import MaterialLedger from './material-ledger.vue'

// =============== 量化分析 ===============
// 说明：预览区「量化分析」标签：左侧子标签切换图表页；视觉沿用平面预览的戴森球深空与纪念碑谷几何。
// 交互：子标签可点击或用 ↑ ↓ 切换，选择记在本机；刷新期间保留上一份图表并降低不透明度。
// 依赖：useQuantWorkspace 的量化方案与生成结果中的 statistics；二维图表由 G6 绘制，等距平台由 three.js 绘制。
// 边界：所有分析都以整个工厂为分母，不做局部区域统计；只读展示后端统计，不触发量化或生成；生成结果必须属于当前方案才使用。
const props = defineProps<{w: QuantWorkspace; active: boolean}>()
const palette = useAnalysisPalette()

// =============== 子标签 ===============
type AnalysisPageKey = 'share' | 'power' | 'lines' | 'materials'
const PAGES: readonly {key: AnalysisPageKey; label: string}[] = [
  {key: 'share', label: '整体占比'},
  {key: 'power', label: '电力构成'},
  {key: 'lines', label: '产线负载'},
  {key: 'materials', label: '物料边界'},
]
const PAGE_KEY = 'dsbox.analysis.page'
const readPage = (): AnalysisPageKey => {
  try {
    const stored = localStorage.getItem(PAGE_KEY)
    return PAGES.find(page => page.key === stored)?.key ?? 'share'
  } catch { return 'share' }
}
const page = ref<AnalysisPageKey>(readPage())
// 首次进入才挂载对应图表，之后只隐藏，保留各页的筛选、旋转等状态。
const visited = reactive(new Set<AnalysisPageKey>([page.value]))
watch(page, value => {
  visited.add(value)
  try { localStorage.setItem(PAGE_KEY, value) } catch { /* 存储不可用时只保留本次会话 */ }
})
// 方向键在子标签间循环，焦点随选中项移动（按 id 定位，不依赖 v-for 模板引用的顺序）。
const moveTab = (step: number): void => {
  const index = (PAGES.findIndex(entry => entry.key === page.value) + step + PAGES.length) % PAGES.length
  page.value = PAGES[index].key
  document.getElementById(`analysis-tab-${PAGES[index].key}`)?.focus()
}

// =============== 数据范围 ===============
// 说明：生成结果属于当前量化方案时用实体蓝图统计（含物流、供电与空间），否则用量化预估统计（仅制造设备）。
const generatedStats = computed<FactoryStatistics | null>(() => {
  const generated = props.w.generated
  const plan = props.w.plan
  if (props.w.replay || !generated || !plan || generated.sourcePlanId !== plan.planId) return null
  return factoryStatisticsOf(generated)
})
// 显示名：统计目录沿用游戏原型名（如「低速传送带」「流速器」），与工作区其它位置的量化数据集译名不一致，
// 这里统一换成数据集名称，图标与文字才能和物品库、格式面板对上；数据集没有的原型保留统计名。
const withDisplayNames = (value: FactoryStatistics): FactoryStatistics => ({
  ...value,
  buildings: value.buildings.map(group => ({...group, name: props.w.itemMap.get(group.itemId)?.name ?? group.name})),
})
const liveStats = computed<FactoryStatistics | null>(() => {
  const value = generatedStats.value ?? (props.w.replay ? null : factoryStatisticsOf(props.w.plan))
  return value ? withDisplayNames(toRaw(value)) : null
})
// 渲染闸门：标签隐藏期间保留上一份统计，避免后台反复量化/生成时重绘不可见的图表；重新显示时同步到最新结果。
const stats = shallowRef<FactoryStatistics | null>(null)
watch([liveStats, () => props.active], ([value, active]) => { if (active) stats.value = value }, {immediate: true})
const plan = computed(() => (props.w.plan ? toRaw(props.w.plan) : null))
const refreshing = computed(() => props.w.solving || props.w.quantStale)

// 空状态：导入回放、未设置目标、旧快照缺少统计分别说明，不伪造零值图表。
const emptyText = computed(() => {
  if (props.w.replay) return '导入回放只识别建筑，不含量化统计。点击检查面板中的「转为量化目标」后即可分析。'
  if (props.w.loadingDataset) return '正在读取数据集…'
  if (!props.w.plan) return props.w.solving ? '正在计算量化方案…' : '设置期望目标后，量化结果会在这里展开分析。'
  return '当前结果缺少统计数据，请重新量化或生成。'
})
</script>

<template>
  <!-- ~ 量化分析 -->
  <!-- * 说明：子标签切换四个图表页（整体占比 / 电力构成 / 产线负载 / 物料边界） -->
  <!-- * 交互：点击或 ↑ ↓ 切换子标签，选择记在本机；刷新期间保留上一份图表并降低不透明度 -->
  <!-- * 依赖：工作区量化方案与生成结果的 statistics；G6 绘图 -->
  <!-- * 边界：只读；导入回放、无方案或旧快照缺统计时显示说明，不绘制零值图表 -->
  <div class="analysis-stage" :class="{'is-dark': palette.dark}">
    <div class="analysis-sky" aria-hidden="true" />

    <!-- 左侧：图表页子标签 -->
    <nav class="analysis-rail" role="tablist" aria-label="量化分析图表" aria-orientation="vertical" @keydown.down.prevent="moveTab(1)" @keydown.up.prevent="moveTab(-1)">
      <button v-for="entry in PAGES" :id="`analysis-tab-${entry.key}`" :key="entry.key" type="button" role="tab" class="rail-tab ds-bracket-hover" :class="{'is-active': page === entry.key}" :aria-selected="page === entry.key" :aria-controls="`analysis-panel-${entry.key}`" :tabindex="page === entry.key ? 0 : -1" @click="page = entry.key">
        {{ entry.label }}
      </button>
    </nav>

    <template v-if="stats">
      <!-- 图表页：首次切换时挂载，之后隐藏保留状态 -->
      <div class="analysis-pages" :class="{'is-refreshing': refreshing}">
        <div v-for="entry in PAGES" v-show="page === entry.key" :id="`analysis-panel-${entry.key}`" :key="entry.key" class="analysis-pane" role="tabpanel" :aria-labelledby="`analysis-tab-${entry.key}`">
          <template v-if="visited.has(entry.key)">
            <ShareOverview v-if="entry.key === 'share'" :stats="stats" :palette="palette" />
            <PowerRange v-else-if="entry.key === 'power'" :stats="stats" :palette="palette" />
            <LineLoad v-else-if="entry.key === 'lines'" :stats="stats" :palette="palette" :plan="plan" :item-name="w.itemName" />
            <MaterialLedger v-else :stats="stats" :palette="palette" />
          </template>
        </div>
      </div>
    </template>

    <!-- 空状态：全息平台 + 说明 -->
    <div v-else class="analysis-empty">
      <svg class="empty-slab" viewBox="0 0 120 70" aria-hidden="true"><path d="M60 6 112 32 60 58 8 32Z" /><path d="M8 32v8l52 26v-8M112 32v8L60 66" /></svg>
      <p>{{ emptyText }}</p>
    </div>
  </div>
</template>

<style scoped lang="scss">
.analysis-stage { position: absolute; inset: 0; display: grid; overflow: hidden; isolation: isolate; container-type: inline-size; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); color: var(--ds-text-primary); }
// 背景：日间为行星白昼天空，夜间为深空星云与星点；叠一层极淡的 ±30° 等距线，呼应纪念碑谷的几何构图。
.analysis-sky { position: absolute; inset: 0; z-index: -1; background:
  repeating-linear-gradient(30deg, transparent 0 27px, rgb(63 151 220 / 0.05) 27px 28px),
  repeating-linear-gradient(150deg, transparent 0 27px, rgb(63 151 220 / 0.05) 27px 28px),
  radial-gradient(ellipse 60% 70% at 85% 0%, rgb(255 255 255 / 0.55), transparent 70%),
  linear-gradient(180deg, #e4ebf4, #cdd7e4); }
.analysis-stage.is-dark .analysis-sky { background:
  var(--ds-cosmos-stars),
  repeating-linear-gradient(30deg, transparent 0 27px, rgb(76 194 255 / 0.035) 27px 28px),
  repeating-linear-gradient(150deg, transparent 0 27px, rgb(76 194 255 / 0.035) 27px 28px),
  radial-gradient(ellipse 55% 65% at 82% 0%, rgb(51 37 111 / 0.38), transparent 70%),
  radial-gradient(ellipse 45% 60% at 8% 100%, rgb(13 86 121 / 0.32), transparent 70%),
  linear-gradient(180deg, #01030a, #0a1430); }

// 子标签按中文名称的宽度排列，窄窗口也能读到完整名称。
.analysis-rail { display: flex; box-sizing: border-box; min-height: 0; flex-direction: column; gap: 2px; overflow-y: auto; border-right: 1px solid var(--ds-border-subtle); padding: 10px 8px; background: color-mix(in srgb, var(--ds-bg-panel) 62%, transparent); scrollbar-width: none; }
.rail-tab { --bracket-size: 8px; --bracket-color: var(--ds-brand); min-height: 36px; border: 0; border-radius: 2px; padding: 0 8px; background: transparent; color: var(--ds-text-secondary); font-size: 12px; text-align: left; white-space: nowrap; cursor: pointer; transition: background 150ms, color 150ms; }
.rail-tab:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.rail-tab.is-active { background: var(--ds-active-bg); color: var(--ds-text-primary); font-weight: 600; box-shadow: var(--ds-active-glow); }
.rail-tab:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: -1px; }

.analysis-pages { position: relative; min-height: 0; grid-column: 2; transition: opacity 200ms; }
.analysis-pages.is-refreshing { opacity: .55; }
.analysis-pane { position: absolute; inset: 0; }

.analysis-empty { display: flex; grid-column: 2; flex-direction: column; align-items: center; justify-content: center; gap: 10px; padding: 16px; color: var(--ds-text-muted); font-size: 13px; text-align: center; }
.analysis-empty p { max-width: 420px; margin: 0; line-height: 1.6; }
// 全息空平台：虚线描边的等距方块，沿用平面预览临时建筑的全息投影语义。
.empty-slab { width: 120px; height: 70px; fill: none; stroke: var(--ds-brand); stroke-dasharray: 4 4; stroke-width: 1; opacity: .7; }
.empty-slab path:first-child { fill: color-mix(in srgb, var(--ds-brand) 8%, transparent); }

@media (prefers-reduced-motion: reduce) { .analysis-pages { transition: none; } }
@container (max-width: 720px) {
  .analysis-rail { padding-inline: 4px; }
  .rail-tab { padding-inline: 4px; }
}
</style>
