<script setup lang="ts">
import {X} from '@lucide/vue'
import ItemIcon from '../item-icon.vue'
import BoltGlyph from './bolt-glyph.vue'
import type {BriefBuildingPanel, BuildingPanelItem} from '../../../../types/building-panel'
import {formatGamePower} from '../../../../utils/format'

// 职责：复刻戴森球计划光标悬停片刻后出现的建筑浮窗。
// 使用：尺寸按游戏 1:1 截图像素设计，由 scale 与建筑窗口共用同一界面缩放，保证各面板文字与图标比例一致。
// 边界：只读；缩放使用 CSS zoom，不在玻璃层的祖先上使用 transform/opacity/filter，避免打断 backdrop-filter。
defineProps<{panel: BriefBuildingPanel; scale: number}>()
const emit = defineEmits<{(event: 'close'): void}>()

const roleLabels: Record<BuildingPanelItem['role'], string> = {product: '产物', input: '原料', marker: '传送带标记'}
const itemTitle = (item: BuildingPanelItem): string => `${roleLabels[item.role]} · ${item.name}${item.count ? ` × ${item.count} / 次` : ''}`
</script>

<template>
  <!-- ~ 建筑悬停浮窗 -->
  <!-- * 说明：复刻戴森球计划建筑悬停浮窗：深色标题栏、模糊玻璃内容区、物品格与电力环 -->
  <!-- * 交互：跟随点击位置出现；悬停或聚焦时右上角显示关闭按钮，Esc 或点击空白处也可关闭 -->
  <!-- * 依赖：BriefBuildingPanel；物品件数为配方每次制作的件数，功率为满载工作功率与待机功率 -->
  <!-- * 边界：没有物品与耗电的建筑只显示标题栏 -->
  <div class="brief" :style="{zoom: scale}" role="dialog" :aria-label="`${panel.name}详情`">
    <header class="brief-head" :title="`${panel.record} · 坐标 ${panel.coords}`">
      <ItemIcon :name="panel.name" :size="28" />
      <strong>{{ panel.name }}</strong>
      <span class="brief-owner" :class="{'is-temporary': panel.temporary}">{{ panel.temporary ? '生成中' : panel.owner }}</span>
      <button type="button" class="brief-close" aria-label="关闭详情" @click="emit('close')"><X :size="16" :stroke-width="2.4" /></button>
    </header>
    <div v-if="panel.items.length || panel.power" class="brief-body">
      <ul v-if="panel.items.length" class="brief-items" aria-label="配方物品">
        <li v-for="(item, index) in panel.items" :key="`${item.role}-${item.itemId}-${index}`" :title="itemTitle(item)">
          <ItemIcon :name="item.name" :size="48" />
          <span v-if="item.count" class="brief-count">{{ item.count }}</span>
          <span class="sr-only">{{ itemTitle(item) }}</span>
        </li>
      </ul>
      <div v-if="panel.power" class="brief-power" :aria-label="`工作功率 ${formatGamePower(panel.power.work)}，待机功率 ${formatGamePower(panel.power.idle)}`">
        <span class="brief-ring" aria-hidden="true">
          <svg class="brief-ring-arc" viewBox="0 0 42 42"><circle class="track" cx="21" cy="21" r="19.5" /><circle class="arc" cx="21" cy="21" r="19.5" /></svg>
          <BoltGlyph class="brief-bolt" :size="22" />
        </span>
        <span class="brief-values" aria-hidden="true">
          <span title="满载工作功率">{{ formatGamePower(panel.power.work) }}</span>
          <span title="待机功率">{{ formatGamePower(panel.power.idle) }}</span>
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.brief { position: relative; display: flex; min-width: 210px; flex-direction: column; color: #fff; font-family: var(--ds-font-ui); letter-spacing: 0; user-select: none; animation: brief-in 140ms ease-out; }
.brief-head { position: relative; display: flex; height: 38px; align-items: center; gap: 10px; padding: 0 34px 0 8px; background: rgba(4, 7, 21, .9); backdrop-filter: blur(10px) saturate(1.2); }
.brief-head strong { overflow: hidden; color: #f5f7fb; font-size: 17px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.brief-owner { margin-left: auto; padding-left: 16px; overflow: hidden; color: #8a95ac; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.brief-owner.is-temporary { color: #ffae73; }
.brief-close { position: absolute; top: 50%; right: 7px; display: inline-flex; width: 22px; height: 22px; align-items: center; justify-content: center; border: 0; padding: 0; background: transparent; color: #8fdcf0; cursor: pointer; opacity: 0; transform: translateY(-50%); transition: opacity 120ms, color 120ms; }
.brief:hover .brief-close, .brief-close:focus-visible { opacity: 1; }
.brief-close:hover, .brief-close:focus-visible { color: #fff; outline: none; }
.brief-body { display: flex; min-height: 70px; align-items: stretch; }
.brief-items { display: flex; flex: 1 1 auto; align-items: center; gap: 13px; margin: 0; padding: 0 18px 0 15px; list-style: none; background: linear-gradient(90deg, rgba(20, 28, 50, .84), rgba(17, 22, 43, .84)); backdrop-filter: blur(10px) saturate(1.2); }
.brief-items li { position: relative; display: flex; width: 48px; height: 48px; align-items: center; justify-content: center; }
.brief-count { position: absolute; right: -6px; bottom: -7px; color: #88ffff; font-family: var(--ds-font-display); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 600; line-height: 1; text-shadow: 0 0 5px rgba(60, 190, 255, .55), 0 1px 2px rgba(0, 0, 0, .9); }
.brief-power { display: flex; flex: 0 0 auto; align-items: center; gap: 18px; padding: 0 9px 0 12px; background: rgba(12, 17, 34, .88); backdrop-filter: blur(10px) saturate(1.2); }
.brief-items + .brief-power { box-shadow: inset 1px 0 0 rgba(0, 0, 0, .25); }
.brief-ring { position: relative; display: inline-flex; width: 42px; height: 42px; flex: 0 0 auto; align-items: center; justify-content: center; }
.brief-ring-arc { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.brief-ring-arc circle { fill: none; stroke-width: 2.6; }
.brief-ring-arc .track { stroke: rgba(136, 255, 255, .16); }
.brief-ring-arc .arc { stroke: #a4ffff; filter: drop-shadow(0 0 3px rgba(97, 216, 255, .85)); }
.brief-bolt { position: relative; color: #eef0f4; }
.brief-values { display: flex; flex-direction: column; align-items: flex-end; color: #88ffff; font-family: var(--ds-font-display); font-size: 17px; font-variant-numeric: tabular-nums; font-weight: 600; line-height: 1.12; text-shadow: 0 0 6px rgba(70, 190, 255, .5); white-space: nowrap; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
@keyframes brief-in { from { translate: 0 4px; } }
@media (prefers-reduced-motion: reduce) { .brief { animation: none; } .brief-close { transition: none; } }
</style>
