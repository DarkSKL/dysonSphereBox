<script setup lang="ts">
import {computed} from 'vue'
import {Workflow} from '@lucide/vue'
import GameWindow from './game-window.vue'
import BoltGlyph from './bolt-glyph.vue'
import ItemIcon from '../item-icon.vue'
import type {BuildingPanelItem, LabBuildingPanel} from '../../../../types/building-panel'
import {formatGamePower} from '../../../../utils/format'

// 职责：复刻戴森球计划「矩阵研究站」窗口（生产模式）：原料五边形、返回按钮、速率箭头、产物进度环与增产效果。
// 使用：算法可视化选中研究站时显示；坐标为游戏 1:1 截图的设计像素，由外框整体缩放。
// 边界：只读；原料与产物件数为配方每次制作件数，进度环按单次制作时长循环，不代表游戏实时库存或进度。
const props = defineProps<{panel: LabBuildingPanel; bounds: {width: number; height: number}; scale: number}>()
const emit = defineEmits<{(event: 'close'): void}>()

const SIZE = {width: 809, height: 526}
// 原料槽：0 上、1 右上、2 右下、3 左下、4 左上、5 中心；连线为外圈五边与中心辐条，只画在两圆之间的空隙
const SLOTS = [{x: 237, y: 170}, {x: 363, y: 262}, {x: 316, y: 412}, {x: 158, y: 412}, {x: 110, y: 262}, {x: 237, y: 304}]
const RADIUS = 61
const LINKS: readonly [number, number][] = [[0, 1], [1, 2], [2, 3], [3, 4], [4, 0], [5, 0], [5, 1], [5, 2], [5, 3], [5, 4]]
// 原料数量对应的槽位：对称分布，两种原料时占右上与左下（与游戏电磁矩阵一致）
const LAYOUTS: Readonly<Record<number, readonly number[]>> = {1: [5], 2: [1, 3], 3: [0, 2, 3], 4: [4, 1, 2, 3], 5: [0, 1, 2, 3, 4], 6: [0, 1, 2, 3, 4, 5]}

const links = LINKS.map(([from, to]) => {
  const a = SLOTS[from], b = SLOTS[to]
  const length = Math.hypot(b.x - a.x, b.y - a.y)
  const ux = (b.x - a.x) / length, uy = (b.y - a.y) / length
  return {x1: a.x + ux * RADIUS, y1: a.y + uy * RADIUS, x2: b.x - ux * RADIUS, y2: b.y - uy * RADIUS}
})
const slots = computed<(BuildingPanelItem | null)[]>(() => {
  const result: (BuildingPanelItem | null)[] = SLOTS.map(() => null)
  const inputs = props.panel.inputs.slice(0, 6)
  const layout = LAYOUTS[inputs.length] ?? []
  inputs.forEach((item, index) => { result[layout[index]] = item })
  return result
})

const research = computed(() => !props.panel.output)
const info = computed(() => `${props.panel.owner} · ${props.panel.record} · 坐标 ${props.panel.coords}`)
const rate = computed(() => (props.panel.rate ?? 0).toFixed(1))
const power = computed(() => props.panel.power ? formatGamePower(props.panel.power, '') : '')
const utilization = computed(() => props.panel.utilization === null ? '' : `${Math.round(props.panel.utilization * 100)}%`)
const percent = (ratio: number): string => `+${(ratio * 100).toFixed(1)} %`
// 进度环周长：r = 80
const RING = 2 * Math.PI * 80
const ringStyle = computed(() => props.panel.cycle ? {animationDuration: `${Math.max(.2, props.panel.cycle)}s`} : {strokeDashoffset: '0', animation: 'none'})
</script>

<template>
  <!-- ~ 矩阵研究站窗口 -->
  <!-- * 说明：复刻戴森球计划矩阵研究站生产模式窗口：左侧原料五边形，右侧产物进度环、单机速率与增产效果 -->
  <!-- * 交互：徽章悬停查看归属、记录与坐标；返回按钮与关闭按钮均关闭窗口 -->
  <!-- * 依赖：LabBuildingPanel（量化数据集配方、设备功率与当前方案的负荷和增产设置） -->
  <!-- * 边界：没有配方时显示为科研模式空槽；件数为配方每次制作件数，不代表游戏实时库存 -->
  <GameWindow :title="panel.name" :size="SIZE" :bounds="bounds" :scale="scale" tone="lab" :info="info" @close="emit('close')">
    <!-- 右上状态：产线归属、功率与负荷、记录状态 -->
    <div class="lab-status">
      <p class="lab-owner" :title="`产线归属：${panel.owner}`"><Workflow :size="18" :stroke-width="1.8" />{{ panel.owner }}</p>
      <p v-if="power" class="lab-power" :title="`满载工作功率 ${power}${utilization ? `，产线负荷 ${utilization}` : ''}`"><BoltGlyph :size="26" /><span>{{ power }}</span><span v-if="utilization">{{ utilization }}</span></p>
      <p class="lab-state" :class="{'is-temporary': panel.temporary}">{{ research ? '科研模式' : panel.record }}</p>
    </div>

    <!-- 原料五边形 -->
    <svg class="lab-links" :viewBox="`0 0 ${SIZE.width} ${SIZE.height}`" aria-hidden="true">
      <line v-for="(link, index) in links" :key="index" :x1="link.x1" :y1="link.y1" :x2="link.x2" :y2="link.y2" />
    </svg>
    <ul class="lab-slots" aria-label="原料">
      <li v-for="(item, index) in slots" :key="index" class="lab-slot" :style="{left: `${SLOTS[index].x - RADIUS}px`, top: `${SLOTS[index].y - RADIUS}px`}" :title="item ? `原料 · ${item.name} × ${item.count ?? 0} / 次` : undefined">
        <template v-if="item">
          <ItemIcon :name="item.name" :size="60" />
          <span class="lab-count">{{ item.count }}</span>
          <span class="sr-only">{{ item.name }} × {{ item.count }}</span>
        </template>
      </li>
    </ul>

    <button type="button" class="lab-back" aria-label="返回" @click="emit('close')">
      <svg viewBox="0 0 24 24" width="40" height="40" aria-hidden="true"><path d="M9.6 4.2 2.4 11l7.2 6.8v-4.3c5.2-.3 8.9 1.4 11.6 5.8-.6-6.2-4.4-10.6-11.6-11.4V4.2Z" fill="currentColor" /></svg>
    </button>

    <!-- 速率与产物 -->
    <template v-if="panel.output">
      <p class="lab-rate" :title="`单机满载产量 ${rate} / 分钟`">{{ rate }}/ 分钟</p>
      <span class="lab-arrows" :class="{'is-running': panel.cycle}" aria-hidden="true"><i /><i /><i /></span>
    </template>
    <div class="lab-output" :title="panel.output ? `产物 · ${panel.output.name} × ${panel.output.count ?? 0} / 次${panel.cycle ? `，每次 ${panel.cycle.toFixed(2)} 秒` : ''}` : undefined">
      <svg class="lab-output-svg" viewBox="0 0 172 172" aria-hidden="true">
        <circle class="lab-output-fill" cx="86" cy="86" r="80" />
        <circle v-if="panel.output" class="lab-output-ring" cx="86" cy="86" r="80" :style="{strokeDasharray: `${RING}`, strokeDashoffset: `${RING}`, ...ringStyle}" />
      </svg>
      <template v-if="panel.output">
        <ItemIcon class="lab-output-icon" :name="panel.output.name" :size="98" />
        <span class="lab-count is-output">{{ panel.output.count }}</span>
        <span class="sr-only">产物 {{ panel.output.name }} × {{ panel.output.count }}</span>
      </template>
    </div>

    <!-- 增产效果 -->
    <section v-if="panel.output" class="lab-spray" aria-label="增产效果">
      <h3>增产效果</h3>
      <p class="is-bonus"><span class="value">{{ percent(panel.proliferator.output) }}</span><span>{{ panel.proliferator.mode === 'extra' ? '额外产出' : '生产加速' }}</span><span class="lab-switch" :class="{'is-speed': panel.proliferator.mode === 'speed'}" role="img" :aria-label="panel.proliferator.mode === 'extra' ? '增产模式：额外产出' : '增产模式：生产加速'"><i /></span></p>
      <p><span class="value">{{ percent(panel.proliferator.power) }}</span><span>电力消耗</span></p>
    </section>
  </GameWindow>
</template>

<style scoped lang="scss">
.lab-status { position: absolute; top: 60px; right: 28px; display: flex; flex-direction: column; align-items: flex-end; gap: 9px; }
.lab-status p { display: flex; height: 30px; align-items: center; gap: 8px; margin: 0; white-space: nowrap; }
.lab-owner { color: #f1f3f7; font-size: 17px; }
.lab-owner svg { color: #c9ced8; }
.lab-power { gap: 14px !important; color: #7fe3ff; font-family: var(--ds-font-display); font-size: 16px; font-variant-numeric: tabular-nums; font-weight: 600; text-shadow: 0 0 6px rgba(70, 190, 255, .45); }
.lab-power .bolt-glyph { margin-right: 14px; color: #c9f6ff; filter: drop-shadow(0 0 3px rgba(120, 220, 255, .7)); }
.lab-state { color: #7fe8c6; font-size: 17px; }
.lab-state.is-temporary { color: #ffae73; }
.lab-links { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
.lab-links line { stroke: rgba(170, 214, 232, .55); stroke-width: 1.6; }
.lab-slots { margin: 0; padding: 0; list-style: none; }
.lab-slot { position: absolute; display: flex; width: 122px; height: 122px; align-items: center; justify-content: center; border-radius: 50%; background: radial-gradient(circle at 50% 38%, rgba(30, 40, 48, .62), rgba(16, 24, 32, .74)); box-shadow: inset 0 0 0 1.5px rgba(108, 142, 158, .38), inset 0 0 22px rgba(0, 0, 0, .28); }
.lab-count { position: absolute; right: 22px; bottom: 18px; color: #88ffff; font-family: var(--ds-font-display); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 600; line-height: 1; text-shadow: 0 0 6px rgba(60, 190, 255, .6), 0 1px 2px rgba(0, 0, 0, .8); }
.lab-count.is-output { right: 34px; bottom: 30px; font-size: 16px; }
.lab-back { position: absolute; top: 131px; left: 529px; display: flex; width: 82px; height: 82px; align-items: center; justify-content: center; border: 0; border-radius: 50%; padding: 0; background: radial-gradient(circle at 50% 40%, rgba(16, 52, 76, .78), rgba(8, 34, 54, .86)); box-shadow: inset 0 0 0 1.5px rgba(46, 92, 116, .55); color: #f4f6fa; cursor: pointer; transition: box-shadow 140ms, color 140ms; }
.lab-back:hover, .lab-back:focus-visible { box-shadow: inset 0 0 0 1.5px rgba(146, 219, 238, .85), 0 0 12px rgba(80, 190, 240, .35); outline: none; }
.lab-rate { position: absolute; top: 280px; left: 440px; width: 130px; margin: 0; color: #f7b47e; font-family: var(--ds-font-display); font-size: 16px; font-variant-numeric: tabular-nums; font-weight: 600; text-align: center; white-space: nowrap; }
.lab-arrows { position: absolute; top: 310px; left: 482px; display: flex; gap: 6px; }
.lab-arrows i { width: 0; height: 0; border-top: 8px solid transparent; border-bottom: 8px solid transparent; border-left: 12px solid rgba(160, 96, 104, .72); }
.lab-arrows.is-running i { animation: lab-arrow 1.2s linear infinite; }
.lab-arrows.is-running i:nth-child(2) { animation-delay: .2s; }
.lab-arrows.is-running i:nth-child(3) { animation-delay: .4s; }
.lab-output { position: absolute; top: 219px; left: 578px; display: flex; width: 172px; height: 172px; align-items: center; justify-content: center; }
.lab-output-svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; transform: rotate(-90deg); }
.lab-output-icon { position: relative; }
.lab-output-fill { fill: rgba(8, 22, 46, .72); stroke: rgba(255, 255, 255, .14); stroke-width: 5; }
.lab-output-ring { fill: none; stroke: #fbfdff; stroke-width: 5.5; animation: lab-cycle linear infinite; }
.lab-spray { position: absolute; top: 392px; left: 466px; }
.lab-spray h3 { margin: 0 0 6px; color: #f4f6fa; font-size: 17px; font-weight: 700; line-height: 26px; }
.lab-spray p { display: flex; height: 29px; align-items: center; margin: 0; color: #d6dae2; font-size: 17px; white-space: nowrap; }
.lab-spray .value { width: 62px; margin-right: 30px; font-family: var(--ds-font-display); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 600; text-align: right; }
.lab-spray .is-bonus { color: #5fd6ff; }
.lab-switch { position: relative; display: inline-block; width: 42px; height: 19px; margin-left: 12px; border-radius: 10px; background: #45c8f6; }
.lab-switch i { position: absolute; top: 0; left: 0; width: 19px; height: 19px; border-radius: 50%; background: #fff; box-shadow: 0 0 0 1px rgba(0, 0, 0, .08); transition: left 160ms; }
.lab-switch.is-speed i { left: 23px; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
@keyframes lab-cycle { to { stroke-dashoffset: 0; } }
@keyframes lab-arrow { 0%, 100% { border-left-color: rgba(160, 96, 104, .72); } 30% { border-left-color: #ffb27a; } }
@media (prefers-reduced-motion: reduce) {
  .lab-output-ring { animation: none; stroke-dashoffset: 0 !important; }
  .lab-arrows.is-running i { animation: none; border-left-color: #f7b47e; }
  .lab-back, .lab-switch i { transition: none; }
}
</style>
