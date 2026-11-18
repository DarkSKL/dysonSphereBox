<script setup lang="ts">
import {computed, ref} from 'vue'
import AnalysisPage from './analysis-page.vue'
import G6Chart from './g6-chart.vue'
import ItemIcon from '../item-icon.vue'
import type {AnalysisSceneBuilder} from '../../../../types/analysis-g6'
import type {FactoryBuildingStatistics, FactoryStatistics} from '../../../../types/factory-statistics'
import {analysisReducedMotion, iconHtml, mixHex, type AnalysisPalette} from '../../../../composables/dysonSphere/useAnalysisCharts'
import {buildPowerProfile, escapeHtml, formatInteger, formatPower, formatValue, powerStageHoldEnd, POWER_STAGES} from '../../../../pages/services/dysonSphere/factory-analysis'
import {chartIcon, chartNode, chartPath, chartText} from '../../../../pages/services/dysonSphere/analysis/g6-rendering'
import {ensureFactoryIcons} from '../../../../pages/services/dysonSphere/factory-icons'

// =============== 建筑用电构成 ===============
// 说明：每层对应一种实际建筑型号，层厚表示该型号在模拟周期中的合计功率；悬停查看原始统计。
const props = defineProps<{stats: FactoryStatistics; palette: AnalysisPalette}>()
ensureFactoryIcons()
const profile = computed(() => buildPowerProfile(props.stats))
const plot = {left: 52, right: 18, top: 38, bottom: 42}
const entering = ref(false)
const hoverId = ref<string | null>(null)
const bands = computed(() => {
  const {powerLayers, shadeTint} = props.palette
  return profile.value.bands.map((band, index) => {
    const base = powerLayers[index % powerLayers.length]
    const depth = Math.floor(index / powerLayers.length)
    return {...band, id: `area-${band.group.itemId}`, color: mixHex(base, shadeTint, 1 - .78 ** depth)}
  })
})
const peakLabel = computed(() => `${profile.value.constant ? '恒定用电' : '满载峰值用电'}  ${formatPower(profile.value.peakMw)}`)
const generation = computed(() => profile.value.generation)
const generationLabel = computed(() => generation.value?.hasCapacity ? '模拟峰值发电' : '峰值发电需求')
const progressLabel = (value: number): string => `${Number(value.toFixed(1))}%`

const details = ({name, count, power}: FactoryBuildingStatistics, phase: number, reading: number, total: number): string => {
  const boost = power.configuredWorkMw - power.baseWorkMw
  const unknown = power.unknownCount + power.missingCount
  const expected = power.expectedMw == null
    ? `负载未知 ${formatInteger(unknown)} 座${power.knownExpectedMw > 0 ? ` · 可估算部分 ${formatPower(power.knownExpectedMw)}` : ''}`
    : `按负载预测 ${formatPower(power.expectedMw)}`
  return [
    `<strong>${iconHtml(name)}${escapeHtml(name)}</strong> · ${formatInteger(count)} 座`,
    `周期进度 ${progressLabel(POWER_STAGES[phase].start)} · ${POWER_STAGES[phase].name}`,
    `全厂用电 ${formatPower(total)}`,
    `本型号 ${formatPower(reading)}（待机 ${formatPower(power.idleMw)} + 工作增量 ${formatPower(reading - power.idleMw)}）`,
    `额定工作 ${formatPower(power.configuredWorkMw)}`,
    ...(boost > 1e-9 ? [`未喷涂 ${formatPower(power.baseWorkMw)} · 增产附加 ${formatPower(boost)}`] : []),
    expected,
  ].join('<br>')
}

// 曲线两端水平；同一阶段的所有边界共用横向控制点，色带厚度在相邻读数之间平滑变化。
const curveTo = (start: number, end: number, from: number, to: number): string => {
  const handle = (end - start) / 3
  return `C${start + handle} ${from} ${end - handle} ${to} ${end} ${to}`
}

const scene = computed<AnalysisSceneBuilder>(() => {
  const {peakMw, peakProgress, constant, idleMw, totals} = profile.value
  const layers = bands.value
  const p = props.palette
  const label = peakLabel.value
  const peakStage = POWER_STAGES.findIndex(stage => stage.start === peakProgress)
  const peakDetails = [
    `<strong>${label}</strong>`,
    ...(constant ? [] : ['设备并发率 100% · 单机负载率 100%', `模拟周期进度 ${progressLabel(peakProgress)}`]),
    ...layers.filter(band => band.readings[peakStage] > 0)
      .map(band => `${iconHtml(band.group.name)}${escapeHtml(band.group.name)} ${formatPower(band.readings[peakStage])}`),
  ].join('<br>')
  return (width, height) => {
    const {left, top} = plot
    const right = width - plot.right
    const bottom = height - plot.bottom
    const x = (progress: number) => left + (right - left) * progress / 100
    const y = (mw: number) => bottom - (bottom - top) * mw / (peakMw || 1)
    const nodes = [chartText('unit', left, 14, peakMw < 1 ? 'kW' : 'MW', p.sub, p.fontDisplay, 'left', 11)]
    for (let tick = 0; tick <= 4; tick++) {
      const mw = peakMw * tick / 4
      nodes.push(chartPath(`grid-${tick}`, left, y(mw), `M0 0 H${right - left}`, p.grid, true))
      nodes.push(chartText(`y-${tick}`, left - 8, y(mw), formatValue(peakMw < 1 ? mw * 1000 : mw), p.muted, p.fontDisplay, 'right', 11))
    }
    // 每段先维持阶段功率，再平滑接入下一读数。保留平台供读数与放置图标，末段一直保持待机。
    POWER_STAGES.forEach((stage, i) => {
      const next = Math.min(i + 1, POWER_STAGES.length - 1)
      const start = x(stage.start)
      const end = x(POWER_STAGES[i + 1]?.start ?? 100)
      const hold = powerStageHoldEnd(start, end)
      for (const band of layers) {
        if (band.readings[i] <= 0 && band.readings[next] <= 0) continue
        const upper = y(band.upper[i])
        const lower = y(band.lower[i])
        const nextUpper = y(band.upper[next])
        const nextLower = y(band.lower[next])
        const id = `${band.id}:${i}`
        const rim = `M${start} ${upper} H${hold} ${curveTo(hold, end, upper, nextUpper)}`
        const path = `${rim} V${nextLower} ${curveTo(end, hold, nextLower, lower)} H${start} Z`
        nodes.push(chartNode(id, 0, 0, {chartShape: 'path', chartPath: path, fill: band.color}, details(band.group, i, band.readings[i], totals[i])))
        const facet = `${rim} V${nextLower} ${curveTo(end, hold, nextLower, upper)} Z`
        nodes.push(chartNode(`${id}-facet`, 0, 0, {chartShape: 'path', chartPath: facet, fill: p.shadeTint, opacity: .12}))
        nodes.push(chartPath(`${id}-rim`, 0, 0, rim, mixHex(band.color, '#fff3df', .38)))
        if (i === peakStage && hold - start >= 24 && lower - upper >= 24) {
          const countLabel = `x ${formatInteger(band.group.count)}`
          const iconSize = 20
          const gap = 3
          const labelWidth = iconSize + gap + countLabel.length * 7
          // 图标与数量一起居中；平台较短时从其左端排起，保留图标与数量的对应关系。
          const labelLeft = Math.max(start + 2, (start + hold - labelWidth) / 2)
          const centerY = (upper + lower) / 2
          const icon = chartIcon(`${id}-icon`, labelLeft + iconSize / 2, centerY, band.group.name, iconSize)
          if (icon) {
            nodes.push(icon, chartNode(`${id}-count`, labelLeft + iconSize + gap, centerY, {
              chartShape: 'text', chartText: countLabel, chartFont: p.fontDisplay, chartFontSize: 11,
              chartFontWeight: 600, fill: '#fff', stroke: p.shadeTint, lineWidth: 2,
            }))
          }
        }
      }
      const contour = `M${start} ${y(totals[i])} H${hold} ${curveTo(hold, end, y(totals[i]), y(totals[next]))}`
      nodes.push(chartNode(`total-${i}`, 0, 0, {chartShape: 'path', chartPath: contour, stroke: p.hero, lineWidth: 1.6}))
    })
    const labelX = constant ? right : x(peakProgress)
    if (!constant) nodes.push(chartPath('peak-guide', labelX, top, `M0 0 V${bottom - top}`, p.hero, true))
    nodes.push(chartNode('peak-label', labelX, top - 16, {chartShape: 'text', chartText: label, chartFont: p.fontDisplay, chartAlign: 'right', chartFontSize: 13, fill: p.text}, peakDetails))
    for (const stage of POWER_STAGES) {
      const color = !constant && stage.start === peakProgress ? p.hero : p.sub
      const showName = right - left >= 620
      const tick = showName ? stage.name : progressLabel(stage.start)
      nodes.push(chartText(`x-${stage.start}`, x(stage.start), bottom + 12, tick, color, p.fontUi, 'center', 10))
    }
    nodes.push(chartText('cycle', (left + right) / 2, height - 10, `模拟周期进度 · 全厂待机 ${formatPower(idleMw)}`, p.sub, p.fontUi, 'center', 11))
    return nodes
  }
})
</script>

<template>
  <!-- ~ 用电建筑的堆叠面积图 -->
  <!-- * 图例沿用从下到上的堆叠顺序；型号较多时随图表区滚动。 -->
  <AnalysisPage scroll>
    <div v-if="generation" class="power-profile" :style="{'--generation-color': palette.shareRamps.production[palette.dark ? 5 : 4]}">
      <div class="generation-reading" :title="generation.hasCapacity ? '按蓝图内机组随负载发电、以额定功率为上限估算。' : '当前统计未计入发电容量，此区间表示满足模拟负载所需的峰值发电。'">
        <span class="generation-key" aria-hidden="true" />
        <strong>{{ generationLabel }} {{ formatPower(generation.mw) }}</strong>
        <span>周期进度 {{ progressLabel(generation.start) }}–{{ progressLabel(generation.end) }}</span>
        <span v-if="generation.hasCapacity">蓝图额定 {{ formatPower(profile.ratedGenerationMw) }}</span>
        <span v-else>未计入发电容量</span>
      </div>
      <div class="power-plot">
        <G6Chart :scene="scene" :label="`各建筑分阶段模拟用电，全厂待机 ${formatPower(profile.idleMw)}，${peakLabel}`" @hover="hoverId = $event?.split(':')[0] ?? null" @ready="entering = !analysisReducedMotion()" />
        <div class="peak-overlay" aria-hidden="true"
          :style="{inset: `${plot.top}px ${plot.right}px ${plot.bottom}px ${plot.left}px`, color: palette.hero}">
          <div class="generation-area" :style="{left: generation.start + '%', width: (generation.end - generation.start) + '%', height: (100 * generation.mw / profile.peakMw) + '%'}" />
          <div v-if="!profile.constant" class="peak-track" :class="{'is-entering': entering}" :style="{width: profile.peakProgress + '%'}">
            <span v-if="entering" class="scan" />
            <i class="star"><i class="orbit" /><i v-if="entering" class="pulse" @animationend="entering = false" /></i>
          </div>
        </div>
      </div>
      <ul class="power-legend" aria-label="用电建筑">
        <li v-for="band in bands" :key="band.id" :class="{'is-active': hoverId === band.id}">
          <i aria-hidden="true" :style="{background: band.color}" />
          <ItemIcon :name="band.group.name" :size="20" />{{ band.group.name }}
        </li>
      </ul>
      <p v-if="stats.power.unknownCount" class="power-note">{{ formatInteger(stats.power.unknownCount) }} 座建筑运行负载未知，常规阶段计待机，满载阶段计额定工作功率。</p>
      <p v-if="stats.power.missingCount" class="power-note">另有 {{ formatInteger(stats.power.missingCount) }} 座建筑缺少原型数据。</p>
    </div>
    <div v-else class="power-empty">
      <div>{{ bands.length ? '已知待机功率为零，尚无可估算的工作负载' : '当前方案暂无可统计的用电建筑' }}
        <p v-if="profile.ratedGenerationMw > 0">蓝图额定发电 {{ formatPower(profile.ratedGenerationMw) }}</p>
      </div>
    </div>
  </AnalysisPage>
</template>

<style scoped lang="scss">
.power-profile { display: flex; flex-direction: column; height: 100%; min-height: 0; }
.generation-reading { display: flex; flex-wrap: wrap; align-items: center; column-gap: 10px; padding: 8px 16px 0; color: var(--ds-text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.generation-reading strong { color: var(--ds-text-primary); }
.generation-key, .generation-area { box-sizing: border-box; border: 1px solid var(--generation-color); background: repeating-linear-gradient(125deg, transparent 0 5px, color-mix(in srgb, var(--generation-color) 46%, transparent) 5px 6px), color-mix(in srgb, var(--generation-color) 12%, transparent); }
.generation-key { width: 16px; height: 10px; }
.generation-area { position: absolute; bottom: 0; border-bottom: 0; }
.power-plot { position: relative; min-height: 164px; flex: 1 0 164px; }
.power-plot > .g6-chart { position: absolute; inset: 0; }
.peak-overlay, .peak-track, .scan, .star, .orbit, .pulse { position: absolute; pointer-events: none; }
.peak-overlay { z-index: 1; }
.peak-track { inset: 0 auto 0 0; }
.star { right: -3px; top: -3px; width: 6px; height: 6px; border-radius: 50%; background: currentColor; box-shadow: 0 0 6px currentColor; }
.orbit { inset: -2px -6px; border: 1px solid currentColor; border-radius: 50%; transform: rotate(-28deg); }
.pulse { inset: -5px; border: 1px solid currentColor; border-radius: 50%; opacity: 0; }
.scan { inset: 0; overflow: hidden; }
@media (prefers-reduced-motion: no-preference) {
  .scan::after { position: absolute; inset: 0; content: ''; background: linear-gradient(90deg, transparent 88%, currentColor); animation: power-sweep 240ms cubic-bezier(.23, 1, .32, 1) both; }
  .is-entering .pulse { animation: power-pulse 200ms 90ms cubic-bezier(.23, 1, .32, 1) both; }
}
@keyframes power-sweep {
  from { opacity: 0; transform: translateX(-100%); }
  45% { opacity: .32; }
  to { opacity: 0; transform: translateX(0); }
}
@keyframes power-pulse {
  from { opacity: .85; transform: scale(.95); }
  to { opacity: 0; transform: scale(1.8); }
}
.power-legend { display: flex; flex: none; flex-wrap: wrap; justify-content: center; gap: 4px 16px; margin: 0; padding: 4px 16px 10px; list-style: none; color: var(--ds-text-secondary); font-size: 11px; }
.power-legend li { display: inline-flex; align-items: center; gap: 5px; }
.power-legend li.is-active { color: var(--ds-text-primary); text-decoration: underline; text-underline-offset: 3px; }
.power-legend i { width: 10px; height: 6px; border-radius: 1px; }
.power-note, .power-empty { color: var(--ds-text-muted); font-size: 12px; text-align: center; }
.power-note { margin: 0; padding: 0 12px 8px; }
.power-empty { display: grid; height: 100%; place-items: center; }
</style>
