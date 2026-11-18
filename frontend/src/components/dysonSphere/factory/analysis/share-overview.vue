<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import AnalysisPage from './analysis-page.vue'
import SharePlatform from './share-platform.vue'
import ShareRing from './share-ring.vue'
import type {FactoryStatistics} from '../../../../types/factory-statistics'
import type {AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {SHARE_METRICS, buildShareComposition, shareListOf, shareValueOf, type ShareEntry, type ShareMetric} from '../../../../pages/services/dysonSphere/factory-analysis'

// =============== 整体占比 ===============
// 说明：左侧 three.js 纪念碑谷式台地看构成的形状，右侧 2×2 四张等距占比环同时给出建筑、电力、生产与空间利用率。
//       建筑、电力按设施统计；生产按物品统计，包含半成品与最终产物；空间把最终蓝图的包络拆成建筑占地、运输线路与空地。
// 交互：点击卡片把台地切到该口径；点击环上的块或图例行固定该项，台地随之切换并聚焦；建筑与电力共用设施列表，悬停在两张卡片与台地间联动。
// 边界：分拣器仅排除建筑计数，耗电仍计入电力；生产保留内部消耗的半成品；空间只在生成蓝图后可用，缺失时台地不切到空间口径。
const props = defineProps<{stats: FactoryStatistics; palette: AnalysisPalette}>()

const composition = computed(() => buildShareComposition(props.stats))
const entriesOf = (metric: ShareMetric): readonly ShareEntry[] => {
  const value = composition.value
  return metric === 'production' ? value.productionEntries : metric === 'space' ? value.spaceEntries : value.entries
}
/** 口径至少有一项非零数据时才能显示在台地上 */
const selectable = (metric: ShareMetric): boolean => entriesOf(metric).some(entry => shareValueOf(entry, metric) > 1e-9)

// =============== 台地口径与联动 ===============
// 说明：悬停与固定都记录来源口径；只要口径共用同一条目列表（建筑与电力），下标就指向同一设施，可以互相高亮。
interface ShareMark { readonly metric: ShareMetric; readonly index: number }
const metric = ref<ShareMetric>('building')
const hover = ref<ShareMark | null>(null)
const pin = ref<ShareMark | null>(null)
const sameList = (a: ShareMetric, b: ShareMetric): boolean => shareListOf(a) === shareListOf(b)
const markOf = (mark: ShareMark | null, target: ShareMetric): number | null => (mark && sameList(mark.metric, target) ? mark.index : null)

const focus = (target: ShareMetric): void => {
  if (target === metric.value || !selectable(target)) return
  metric.value = target
  if (pin.value && !sameList(pin.value.metric, target)) pin.value = null
}
const onHover = (target: ShareMetric, index: number | null): void => {
  if (index !== null) hover.value = {metric: target, index}
  else if (hover.value && sameList(hover.value.metric, target)) hover.value = null
}
// 固定：再次点击同一项或点击台地空白处取消；固定其他口径的条目时台地先切到该口径。
const onPick = (target: ShareMetric, index: number | null): void => {
  focus(target)
  const current = markOf(pin.value, target)
  pin.value = index === null || index === current ? null : {metric: target, index}
}
// 统计更新后下标可能指向别的设施，清除悬停与固定；台地口径失去数据时（如回到量化估算，没有空间统计）退回建筑。
watch(composition, () => {
  hover.value = null
  pin.value = null
  if (!selectable(metric.value)) metric.value = 'building'
})
</script>

<template>
  <!-- ~ 整体占比 -->
  <!-- * 说明：左侧等距台地显示当前口径的构成，右侧 2×2 占比环同时展示建筑、电力、生产与空间利用率 -->
  <!-- * 交互：点击卡片切换台地口径；块、图例行与台地区域双向联动，点击固定后台地镜头聚焦 -->
  <!-- * 依赖：统计报告 buildings、materials 与 space -->
  <!-- * 边界：只表达整体构成，不表示蓝图中的实际位置；空间利用率需生成蓝图后才有数据 -->
  <AnalysisPage>
    <div class="overview-wrap">
      <div class="overview-body">
        <div class="overview-platform">
          <SharePlatform :entries="entriesOf(metric)" :metric="metric" :palette="palette" :hovered="markOf(hover, metric)" :pinned="markOf(pin, metric)" @hover="onHover(metric, $event)" @pick="onPick(metric, $event)" />
        </div>
        <div class="overview-rings" role="group" aria-label="整体占比">
          <ShareRing v-for="info in SHARE_METRICS" :key="info.key" :info="info" :entries="entriesOf(info.key)" :total="composition.totals[info.key]" :space="composition.space" :palette="palette" :hovered="markOf(hover, info.key)" :pinned="markOf(pin, info.key)" :linked="metric === info.key" :selectable="selectable(info.key)" @hover="onHover(info.key, $event)" @pick="onPick(info.key, $event)" @focus="focus(info.key)" />
        </div>
      </div>
    </div>
  </AnalysisPage>
</template>

<style scoped lang="scss">
// 外层作为容器查询的参照：宽屏左右排布，窄屏台地在上、四张卡片在下，超出时外层纵向滚动（容器查询不能作用于容器自身，滚动写在常规样式里）。
.overview-wrap { position: absolute; inset: 0; overflow-x: hidden; overflow-y: auto; container-type: inline-size; scrollbar-width: thin; }
.overview-body { display: grid; height: 100%; grid-template-columns: minmax(0, .9fr) minmax(0, 1.1fr); }
.overview-platform { position: relative; min-width: 0; min-height: 0; }
.overview-rings { display: grid; min-width: 0; min-height: 0; grid-template-columns: repeat(2, minmax(0, 1fr)); grid-template-rows: repeat(2, minmax(0, 1fr)); gap: 8px; padding: 10px 12px 10px 4px; }
@container (max-width: 1080px) {
  .overview-body { grid-template-columns: minmax(0, .72fr) minmax(0, 1.28fr); }
}
@container (max-width: 720px) {
  .overview-body { height: auto; min-height: 100%; grid-template-columns: minmax(0, 1fr); grid-template-rows: 280px auto; }
  .overview-rings { grid-template-rows: repeat(2, 220px); padding-left: 12px; }
}
</style>
