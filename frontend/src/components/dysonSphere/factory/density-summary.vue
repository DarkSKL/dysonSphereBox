<script setup lang="ts">
import {computed} from 'vue'
import type {build} from '../../../../wailsjs/go/models'

// =============== 实际布局覆盖 ===============
// 说明：只汇总后端报告中的实体数；边界：没有普通对照时不推算节省率，旧报告仍可查看但明确标记。
const props = defineProps<{report: build.DensityReport; surfacePlan?: build.SurfacePlan; metrics?: build.LayoutMetrics; stale: boolean; itemName: (id: number) => string}>()
const totals = computed(() => props.report.productions.reduce((sum, row) => ({
  paired: sum.paired + row.pairedMachines,
  rows: sum.rows + row.rowMachines,
  single: sum.single + row.singleMachines,
  reference: sum.reference + (row.referenceMachines ?? 0),
}), {paired: 0, rows: 0, single: 0, reference: 0}))
const placementLatitude = computed(() => {
  const latitude = props.surfacePlan?.latitudeDegrees
  if (typeof latitude !== 'number' || !Number.isFinite(latitude)) return ''
  if (Math.abs(latitude) < .005) return '赤道（0°）'
  return `${latitude > 0 ? '北纬' : '南纬'} ${Math.abs(latitude).toLocaleString('zh-CN', {maximumFractionDigits: 2})}°`
})
// 说明：展示后端逐起点核验后的连续范围；边界：缺少结果时保留单点建议，不把地区名称当作整个纬带均可放置。
const placementRange = computed(() => {
  const low = props.surfacePlan?.safeLatitudeMinDegrees
  const high = props.surfacePlan?.safeLatitudeMaxDegrees
  if (typeof low !== 'number' || typeof high !== 'number' || !Number.isFinite(low) || !Number.isFinite(high) || low > high) return ''
  const direction = high < 0 ? '南纬' : '北纬'
  const first = high < 0 ? -high : low
  const last = high < 0 ? -low : high
  const format = (value: number): string => value.toLocaleString('zh-CN', {maximumFractionDigits: 2})
  return first === last ? `${direction} ${format(first)}°` : `${direction} ${format(first)}°–${format(last)}°`
})
const hasBeltMetrics = computed(() => typeof props.metrics?.beltCount === 'number' && Number.isFinite(props.metrics.totalBeltLengthMeters))
const formatMetric = (value: number | undefined): string => (value ?? 0).toLocaleString('zh-CN', {maximumFractionDigits: 1})
</script>

<template>
  <!-- ~ 获选候选的密度说明 -->
  <!-- * 说明：实际单排/双排覆盖及预检范围；交互：展开查看逐产线；依赖：DensityReport；边界：数值检查不等同游戏验收。 -->
  <section class="density-summary" aria-label="生成密度结果">
    <div class="density-heading"><strong>{{ report.requested === 'dense' ? '超密度 · 试验' : '普通密度' }}</strong><span v-if="stale" role="status">上次成功结果</span></div>
    <template v-if="report.requested === 'dense'">
      <p v-if="surfacePlan?.latitudeZoneLabel" class="placement-location">建造位置：<strong>{{ surfacePlan.latitudeZoneLabel }}</strong><template v-if="surfacePlan.placementHint"><br />{{ surfacePlan.placementHint }}</template><br /><span v-if="placementLatitude">建议放置起点：{{ placementLatitude }}</span><template v-if="placementRange"><br />默认方向起点范围：{{ placementRange }}</template><br />请以默认方向整张粘贴到建议位置。</p>
      <p v-else class="placement-location">此结果未记录建造地区，请重新生成后按建议位置放置。</p>
      <p v-if="!report.applied">本次为独立单机或单列堆叠，没有可压缩的相邻水平间距。</p>
      <dl><div><dt>相向双排</dt><dd>{{ totals.paired }} 台</dd></div><div><dt>密集单排</dt><dd>{{ totals.rows }} 台</dd></div><div><dt>单机 / 单列堆叠</dt><dd>{{ totals.single }} 台</dd></div></dl>
      <p v-if="totals.reference">其中 {{ totals.reference }} 台采用参考蓝图的低层轨道变体；多物料变体会额外预留分拣空间。</p>
      <details><summary>逐产线应用范围</summary><ul><li v-for="row in report.productions" :key="row.productionIndex">产线 {{ row.productionIndex + 1 }} · {{ itemName(row.machineId) }}<br />双排 {{ row.pairedMachines }} / 单排 {{ row.rowMachines }} / 单机或单列 {{ row.singleMachines }}</li></ul></details>
    </template>
    <p>{{ report.verification }}</p>
    <p>布局包络 {{ report.layoutEnvelopeArea }} 格²。该值用于布局对照，不是实际地表面积。</p>
    <dl v-if="hasBeltMetrics"><div><dt>传送带数量</dt><dd>{{ formatMetric(metrics?.beltCount) }} 节</dd></div><div><dt>总带长</dt><dd>{{ formatMetric(metrics?.totalBeltLengthMeters) }} 米</dd></div></dl>
    <small>规则 <code>{{ report.profileVersion }}</code></small>
  </section>
</template>

<style scoped lang="scss">
.density-summary { margin-top: 8px; border-top: 1px solid var(--ds-border-subtle); padding-top: 10px; color: var(--ds-text-secondary); font-size: 11px; line-height: 1.6; }
.density-heading { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px; }
.density-heading strong { color: var(--ds-text-primary); font-weight: 600; }
.density-heading span { color: var(--ds-warning); }
.placement-location strong { color: var(--ds-text-primary); font-weight: 600; }
p { margin: 6px 0; }
dl { margin: 6px 0; }
dl > div { display: flex; justify-content: space-between; gap: 8px; }
dd { margin: 0; color: var(--ds-text-primary); font-family: var(--ds-font-display); font-variant-numeric: tabular-nums; }
summary { cursor: pointer; color: var(--ds-text-primary); }
summary:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: 3px; }
ul { max-height: 180px; overflow-y: auto; padding-left: 16px; }
li + li { margin-top: 6px; }
small { font-size: 10px; }
code { overflow-wrap: anywhere; }
</style>
