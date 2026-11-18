<script setup lang="ts">
import {computed} from 'vue'
import G6Chart from './g6-chart.vue'
import type {AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import type {AnalysisBarRow, AnalysisSceneBuilder} from '../../../../types/analysis-g6'
import {chartIcon, chartPath, chartRect, chartText} from '../../../../pages/services/dysonSphere/analysis/g6-rendering'
import {clampLabel} from '../../../../composables/dysonSphere/useAnalysisCharts'

// =============== 共用数轴行图 ===============
// 职责：使用 G6 节点绘制同刻度条段、图鉴标签与数轴；使用：电力、负载与物料子页；边界：不改变输入数值、未知状态或排序。
const props = withDefaults(defineProps<{rows: readonly AnalysisBarRow[]; palette: AnalysisPalette; min?: number; max: number; formatTick: (value: number) => string; label: string; unit?: string; rightPadding?: number; reference?: {value: number; label: string}}>(), {min: 0, unit: '', rightPadding: 160})
const scene = computed<AnalysisSceneBuilder>(() => {
  const {rows, palette: p, min, max, formatTick, unit, rightPadding, reference} = props
  return (width, height) => {
    const left = Math.min(138, width * .28)
    const plotWidth = Math.max(40, width - left - Math.min(rightPadding, width * .32))
    const top = reference ? 28 : 14
    const bottom = height - 30
    const xOf = (value: number): number => left + (value - min) / Math.max(1e-9, max - min) * plotWidth
    const nodes = []
    for (let index = 0; index <= 4; index++) {
      const value = min + (max - min) * index / 4
      const x = xOf(value)
      nodes.push(chartPath(`grid-${index}`, x, top, `M0 0 L0 ${Math.max(0, bottom - top)}`, p.grid, true))
      nodes.push(chartText(`tick-${index}`, x, height - 14, formatTick(value), p.muted, p.fontDisplay, 'center', 10))
    }
    if (unit) nodes.push(chartText('unit', left + plotWidth + 12, height - 14, unit, p.muted, p.fontUi, 'left', 10))
    if (min < 0 && max > 0) nodes.push(chartPath('zero', xOf(0), top, `M0 0 L0 ${Math.max(0, bottom - top)}`, p.sub))
    const step = (bottom - top) / Math.max(1, rows.length)
    rows.forEach((row, index) => {
      const y = top + step * (index + .5)
      nodes.push(chartRect(`${row.id}-hit`, 0, y, width, Math.max(24, step - 4), 'rgba(0,0,0,0.001)', row.tooltip))
      const icon = chartIcon(`${row.id}-icon`, 18, y, row.icon)
      if (icon) nodes.push(icon)
      nodes.push(chartText(`${row.id}-name`, 36, y, clampLabel(row.name), p.sub, p.fontUi, 'left', 12))
      row.segments.forEach((segment, part) => nodes.push(chartRect(`${row.id}-bar-${part}`, xOf(Math.min(segment.from, segment.to)), y, Math.abs(xOf(segment.to) - xOf(segment.from)), 14, segment.color)))
      row.marks?.forEach((mark, part) => nodes.push(chartPath(`${row.id}-mark-${part}`, xOf(mark), y, 'M0 -11 L0 11', p.text)))
      const side = row.labelSide === 'left' ? -1 : 1
      const labelX = xOf(row.labelAt) + side * 9
      nodes.push(chartText(`${row.id}-value`, labelX, y, row.label, p.text, p.fontDisplay, side < 0 ? 'right' : 'left', 11))
      if (row.secondaryLabel) {
        const secondary = row.secondaryLabel
        nodes.push(chartText(`${row.id}-secondary`, xOf(secondary.at) + (secondary.side === 'left' ? -9 : 9), y, secondary.text, p.sub, p.fontDisplay, secondary.side === 'left' ? 'right' : 'left', 11))
      }
      if (row.labelIcon) {
        const machine = chartIcon(`${row.id}-machine`, labelX + row.label.length * 6.4 + 14, y, row.labelIcon, 18)
        if (machine) nodes.push(machine)
      }
    })
    if (reference && rows.length) {
      nodes.push(chartPath('reference-line', xOf(reference.value), top, `M0 0 L0 ${Math.max(0, bottom - top)}`, p.text, true))
      nodes.push(chartText('reference-label', xOf(reference.value), 12, reference.label, p.sub, p.fontUi, 'center', 10))
    }
    return nodes
  }
})
</script>

<template>
  <!-- ~ G6 同刻度行图 -->
  <!-- * 边界：无数据时显示说明，不绘制伪造的零值条形 -->
  <G6Chart v-if="rows.length" class="row-chart" :scene="scene" :label="label" />
  <div v-else class="row-empty">当前口径暂无可统计数据</div>
</template>

<style scoped lang="scss">
.row-chart { width: 100%; min-height: 100%; }
.row-empty { display: grid; min-height: 100%; place-items: center; color: var(--ds-text-muted); font-size: 12px; }
</style>
