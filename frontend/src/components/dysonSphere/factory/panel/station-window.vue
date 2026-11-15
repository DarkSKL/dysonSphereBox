<script setup lang="ts">
import {computed} from 'vue'
import {Aperture, ArrowDownLeft, Check, Lock} from '@lucide/vue'
import GameWindow from './game-window.vue'
import BoltGlyph from './bolt-glyph.vue'
import ItemIcon from '../item-icon.vue'
import type {StationBuildingPanel, StationLogic, StationSlot} from '../../../../types/building-panel'
import {formatGameEnergy, formatGamePower} from '../../../../utils/format'

// 职责：复刻戴森球计划「物流运输站」窗口：物品槽位行、运输机与储能、物流站设置。
// 使用：算法可视化选中行星内 / 星际物流运输站时显示；槽位与设置读取自蓝图参数，坐标为游戏 1:1 截图的设计像素。
// 边界：只读；蓝图建成时的站点没有库存与在途货物，「当前」「在途」恒为 0；储能条表示储能上限，滑块不可拖动。
const props = defineProps<{panel: StationBuildingPanel; bounds: {width: number; height: number}; scale: number}>()
const emit = defineEmits<{(event: 'close'): void}>()

const interstellar = computed(() => props.panel.facts.interstellar)
const settingRows = computed(() => interstellar.value ? 7 : 4)
const size = computed(() => ({width: 763, height: Math.round(821 + (props.panel.facts.slots - 4) * 101 + (settingRows.value - 4) * 26.5)}))
const info = computed(() => `${props.panel.owner} · ${props.panel.record} · 坐标 ${props.panel.coords}`)

const logicLabels: Record<StationLogic, string> = {supply: '供应', demand: '需求', none: '仓储'}
const markerStyle = (slot: StationSlot): Record<string, string> => ({left: `${Math.min(1, Math.max(0, slot.max / props.panel.facts.capacity)) * 100}%`})
const rowTitle = (slot: StationSlot): string => `${slot.name} · 本地${logicLabels[slot.local]}${interstellar.value ? ` · 星际${logicLabels[slot.remote]}` : ''} · 上限 ${slot.max}`

// =============== 物流站设置 ===============
// 说明：滑块比例按游戏界面的取值范围换算：充能功率为上限的 10% 到 100%，运输机路程 20° 到 180°，起送量 1% 到 100%，
//       运输船路程 1 到 60 光年（超出为无限），曲速启用路程 0.5 到 60 AU。
interface SettingRow {
  readonly label: string
  readonly value: string
  readonly ratio: number
  readonly disabled?: boolean
  readonly check?: {readonly label: string; readonly checked: boolean}
}
const ratio = (value: number, min: number, max: number): number => Math.min(1, Math.max(0, (value - min) / (max - min)))
const settings = computed<SettingRow[]>(() => {
  const value = props.panel.settings
  if (!value) return []
  const maxCharge = props.panel.facts.maxCharge
  const charge: SettingRow = {label: '最大充能功率', value: formatGamePower(value.chargePower), ratio: ratio(value.chargePower, maxCharge / 10, maxCharge)}
  const drone: SettingRow = {label: '运输机最远路程', value: `${Math.round(value.droneRange)} °`, ratio: ratio(value.droneRange, 20, 180)}
  const droneDelivery: SettingRow = {label: '运输机起送量', value: `${value.droneDelivery} %`, ratio: ratio(value.droneDelivery, 1, 100)}
  const piler: SettingRow = {label: '输出货物集装数量', value: value.pilerCount > 0 ? String(value.pilerCount) : '—', ratio: value.pilerCount > 0 ? ratio(value.pilerCount, 1, 4) : 1, disabled: value.pilerCount <= 0, check: {label: '使用科技上限', checked: value.pilerCount <= 0}}
  if (!interstellar.value) return [charge, drone, droneDelivery, piler]
  return [
    charge,
    drone,
    {label: '运输船最远路程', value: value.shipRange === null ? '∞' : `${Math.round(value.shipRange)} ly`, ratio: value.shipRange === null ? 1 : ratio(value.shipRange, 1, 60)},
    {label: '曲速启用路程', value: `${value.warpDistance.toFixed(1)} AU`, ratio: ratio(value.warpDistance, .5, 60), check: {label: '翘曲器必备', checked: value.warperNecessary}},
    droneDelivery,
    {label: '运输船起送量', value: `${value.shipDelivery} %`, ratio: ratio(value.shipDelivery, 1, 100), check: {label: '轨道采集器', checked: value.includeOrbitCollector}},
    piler,
  ]
})
const chargePower = computed(() => props.panel.settings ? formatGamePower(props.panel.settings.chargePower, '') : '—')
</script>

<template>
  <!-- ~ 物流运输站窗口 -->
  <!-- * 说明：复刻戴森球计划物流运输站窗口：槽位行（物品、当前/在途/上限、库存条、供需按钮）、运输机与储能、物流站设置 -->
  <!-- * 交互：徽章悬停查看归属、记录与坐标；关闭按钮关闭窗口；槽位行悬停显示物流角色摘要 -->
  <!-- * 依赖：StationBuildingPanel（STATION_FACTS 原型能力与蓝图参数解析结果） -->
  <!-- * 边界：参数读取中或不可用时槽位显示为空并在副标题说明原因；总控面板仅为外观，不可切换 -->
  <GameWindow :title="panel.name" :size="size" :bounds="bounds" :scale="scale" tone="station" :info="info" :tab="interstellar ? 222 : 265" @close="emit('close')">
    <div class="st">
      <p class="st-sub"><span>本地站点 · {{ panel.owner }} · {{ panel.record }}</span><span v-if="panel.notice" class="st-notice" role="status">{{ panel.notice }}</span></p>

      <!-- 物品槽位 -->
      <ol class="st-slots" aria-label="物品槽位">
        <template v-for="(slot, index) in panel.slots" :key="index">
          <li v-if="slot" class="st-slot" :class="`is-${slot.local}`" :title="rowTitle(slot)">
            <span class="st-icon"><ItemIcon :name="slot.name" :size="70" /></span>
            <Lock class="st-lock" :size="14" :stroke-width="2.6" aria-hidden="true" />
            <span class="st-labels" aria-hidden="true"><span>当前</span><span>在途</span><span>上限</span></span>
            <span class="st-values" aria-hidden="true"><span>0</span><span>0</span><span>{{ slot.max }}</span></span>
            <span class="st-bar" role="meter" :aria-label="`${slot.name} 库存`" aria-valuemin="0" :aria-valuemax="panel.facts.capacity" aria-valuenow="0" :aria-valuetext="`当前 0，在途 0，上限 ${slot.max}`">
              <i class="st-mark is-top" :style="markerStyle(slot)" />
              <i class="st-mark is-max" :style="markerStyle(slot)"><b>MAX</b></i>
            </span>
            <span class="st-roles" :class="{'is-dual': interstellar}">
              <span class="st-role" :class="`is-${slot.local}`">{{ interstellar ? `本地${logicLabels[slot.local]}` : logicLabels[slot.local] }}</span>
              <span v-if="interstellar" class="st-role" :class="`is-${slot.remote}`">星际{{ logicLabels[slot.remote] }}</span>
            </span>
          </li>
          <li v-else class="st-slot is-empty"><i class="st-empty-dot" aria-hidden="true" /><span>空栏位</span></li>
        </template>
      </ol>

      <!-- 运输机与储能 -->
      <div class="st-power">
        <span v-for="carrier in interstellar ? ['drone', 'vessel'] : ['drone']" :key="carrier" class="st-carrier" :title="carrier === 'drone' ? `物流运输机 0 / ${panel.facts.drones}` : `星际物流运输船 0 / ${panel.facts.vessels}`">
          <svg class="st-carrier-ring" viewBox="0 0 80 80" aria-hidden="true"><circle cx="40" cy="40" r="39" /></svg>
          <ItemIcon class="st-carrier-icon" :name="carrier === 'drone' ? '物流运输机' : '星际物流运输船'" :size="46" />
          <ArrowDownLeft class="st-carrier-arrow" :size="22" :stroke-width="3.2" aria-hidden="true" />
          <b>0/{{ carrier === 'drone' ? panel.facts.drones : panel.facts.vessels }}</b>
        </span>
        <BoltGlyph class="st-bolt" :size="36" />
        <span class="st-energy" :title="`储能上限 ${formatGameEnergy(panel.facts.energy)}`"><b>{{ formatGameEnergy(panel.facts.energy) }}</b></span>
        <span class="st-charge" :title="`最大充能功率 ${chargePower}`"><b>{{ chargePower }}</b><span>最大充能</span></span>
      </div>

      <hr class="st-divider">

      <div class="st-tabs" role="tablist" aria-label="物流站面板">
        <span class="st-tab is-active" role="tab" aria-selected="true">物流站设置</span>
        <span class="st-tab is-disabled" role="tab" aria-selected="false" aria-disabled="true" title="总控面板仅在游戏内可用"><Aperture :size="17" :stroke-width="2.2" aria-hidden="true" />总控面板 (I)</span>
      </div>

      <dl class="st-settings" role="tabpanel" aria-label="物流站设置">
        <template v-for="row in settings" :key="row.label">
          <dt>{{ row.label }}</dt>
          <dd class="st-slider" :class="{'is-disabled': row.disabled}" role="meter" :aria-label="row.label" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(row.ratio * 100)" :aria-valuetext="row.value"><i :style="{width: `${row.ratio * 100}%`}" /></dd>
          <dd class="st-value">{{ row.value }}</dd>
          <dd class="st-check"><template v-if="row.check"><span class="st-box" :class="{'is-checked': row.check.checked}" role="img" :aria-label="`${row.check.label}：${row.check.checked ? '是' : '否'}`"><Check v-if="row.check.checked" :size="15" :stroke-width="3" aria-hidden="true" /></span>{{ row.check.label }}</template></dd>
        </template>
      </dl>
    </div>
  </GameWindow>
</template>

<style scoped lang="scss">
.st { position: absolute; top: 62px; right: 37px; bottom: 0; left: 33px; }
.st-sub { display: flex; height: 34px; align-items: center; gap: 16px; margin: 0; padding-left: 15px; overflow: hidden; color: #a3a6b0; font-size: 16px; white-space: nowrap; }
.st-notice { overflow: hidden; color: #ffae73; text-overflow: ellipsis; }
.st-slots { display: flex; flex-direction: column; gap: 9px; margin: 9px 0 0; padding: 0; list-style: none; }
.st-slot { position: relative; height: 92px; background: rgba(0, 0, 0, .58); box-shadow: inset 4px 0 0 var(--st-logic, #b2b2b2); }
.st-slot.is-demand { --st-logic: #e18c5f; }
.st-slot.is-supply { --st-logic: #3d8ba7; }
.st-slot.is-none { --st-logic: #9a9ca4; }
.st-slot.is-empty { display: flex; align-items: center; justify-content: center; background: rgba(36, 36, 48, .55); }
.st-slot.is-empty span { position: absolute; right: 0; left: 90px; color: #34343d; font-size: 37px; font-weight: 700; text-align: center; }
.st-empty-dot { position: absolute; top: 10px; left: 29px; width: 72px; height: 72px; border-radius: 50%; background: #efefef; box-shadow: 0 0 0 3px #26272f, 0 0 0 4.5px rgba(110, 112, 122, .55); }
.st-icon { position: absolute; top: 11px; left: 30px; display: flex; width: 70px; height: 70px; align-items: center; justify-content: center; }
.st-lock { position: absolute; top: 15px; left: 112px; color: #8b8d95; }
.st-labels, .st-values { position: absolute; top: 12px; display: flex; flex-direction: column; color: #f2f3f6; line-height: 24px; text-align: right; }
.st-labels { right: calc(100% - 163px); font-size: 17px; }
.st-values { right: calc(100% - 259px); font-family: var(--ds-font-display); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 600; }
.st-bar { position: absolute; top: 23px; left: 267px; width: 265px; height: 46px; background: #040405; box-shadow: inset 0 0 0 1px #2a2a2c; }
.st-mark { position: absolute; width: 0; height: 0; border-right: 5px solid transparent; border-left: 5px solid transparent; transform: translateX(-50%); }
.st-mark.is-top { top: -9px; border-top: 6px solid #9a9ca2; }
.st-mark.is-max { bottom: -9px; border-bottom: 6px solid #9a9ca2; }
.st-mark b { position: absolute; top: 6px; left: 50%; color: #a6a8ae; font-family: var(--ds-font-display); font-size: 10px; font-weight: 600; line-height: 1; transform: translateX(-50%); }
.st-roles { position: absolute; top: 15px; left: 571px; display: flex; width: 105px; height: 64px; flex-direction: column; gap: 4px; }
.st-role { display: flex; flex: 1 1 0; align-items: center; justify-content: center; clip-path: polygon(0 0, calc(100% - 7px) 0, 100% 7px, 100% 100%, 7px 100%, 0 calc(100% - 7px)); background: #6b6e78; color: #fff; font-size: 17px; }
.st-roles.is-dual .st-role { font-size: 15px; }
.st-role.is-demand { background: #e18c5f; }
.st-role.is-supply { background: #3d8ba7; }
.st-power { display: flex; height: 84px; align-items: center; margin-top: 27px; padding: 0 4px 0 2px; }
.st-carrier { position: relative; display: flex; width: 80px; height: 80px; flex: 0 0 auto; align-items: center; justify-content: center; }
.st-carrier + .st-carrier { margin-left: 12px; }
.st-carrier-ring { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.st-carrier-ring circle { fill: rgba(0, 0, 0, .25); stroke: rgba(222, 228, 238, .72); stroke-dasharray: 195.3 38.1 11.6 0; stroke-width: 1.6; }
.st-carrier-icon { position: relative; }
.st-carrier-arrow { position: absolute; top: -4px; right: -4px; color: #fff; }
.st-carrier b { position: absolute; bottom: -5px; left: 50%; color: #6fe3ff; font-family: var(--ds-font-display); font-size: 14px; font-variant-numeric: tabular-nums; font-weight: 600; transform: translateX(-30%); text-shadow: 0 0 5px rgba(60, 190, 255, .5); white-space: nowrap; }
.st-bolt { margin-left: 45px; color: #f4f6fa; }
.st-energy { position: relative; display: flex; height: 32px; flex: 1 1 auto; align-items: center; justify-content: flex-end; margin-left: 23px; padding-right: 12px; background: #c0ffff; box-shadow: 0 0 10px rgba(120, 232, 255, .75), 0 0 2px rgba(200, 255, 255, .9); }
.st-energy b { color: #29384a; font-family: var(--ds-font-display); font-size: 19px; font-variant-numeric: tabular-nums; font-weight: 600; }
.st-charge { display: flex; min-width: 72px; flex-direction: column; align-items: flex-end; gap: 2px; margin-left: 30px; }
.st-charge b { color: #6fe3ff; font-family: var(--ds-font-display); font-size: 16px; font-variant-numeric: tabular-nums; font-weight: 600; text-shadow: 0 0 6px rgba(70, 190, 255, .5); white-space: nowrap; }
.st-charge span { color: #f2f3f6; font-size: 17px; white-space: nowrap; }
.st-divider { height: 1px; margin: 18px 0 0; border: 0; background: rgba(62, 66, 92, .85); }
.st-tabs { display: flex; height: 34px; align-items: stretch; margin-top: 14px; }
.st-tab { display: inline-flex; align-items: center; justify-content: center; gap: 7px; font-size: 18px; }
.st-tab.is-active { width: 127px; border-bottom: 2px solid #e1e1e1; color: #f6f7fa; font-weight: 700; }
.st-tab.is-disabled { margin-left: 70px; color: #8d9099; }
.st-settings { display: grid; grid-template-columns: 197px 215px 76px 1fr; grid-auto-rows: 26.5px; align-items: center; margin: 9px 0 0; padding-left: 7px; }
.st-settings dt { color: #e8eaef; font-size: 16.5px; white-space: nowrap; }
.st-settings dd { margin: 0; }
.st-slider { position: relative; height: 10px; background: rgba(22, 24, 32, .95); }
.st-slider i { position: absolute; top: 2px; bottom: 2px; left: 2px; max-width: calc(100% - 4px); background: #f1f1f1; }
.st-slider.is-disabled { background: rgba(72, 74, 80, .95); box-shadow: inset 0 0 0 1px rgba(110, 112, 118, .6); }
.st-slider.is-disabled i { display: none; }
.st-value { color: #f2f3f6; font-family: var(--ds-font-display); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 600; text-align: right; white-space: nowrap; }
.st-check { display: flex; align-items: center; gap: 14px; padding-left: 40px; color: #e8eaef; font-size: 17px; white-space: nowrap; }
.st-box { display: inline-flex; width: 17px; height: 17px; align-items: center; justify-content: center; box-shadow: inset 0 0 0 1.6px #d8dbe2; color: #fff; }
</style>
