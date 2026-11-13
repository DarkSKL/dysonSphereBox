<script setup lang="ts">
import {computed} from 'vue'
import {X} from '@lucide/vue'

// 职责：游戏建筑窗口的外框：模糊玻璃底、左上标题页签、右上信息/关闭徽章与转角描边；内容由默认插槽按设计坐标排布。
// 使用：研究站与物流站窗口传入设计尺寸（游戏 1:1 截图像素）、随建筑变化的缩放与舞台可用空间；空间不足时进一步缩小。
// 边界：缩放使用 CSS zoom，不使用 transform/opacity/filter 包裹玻璃层，避免打断 backdrop-filter 的背景取样。
const props = withDefaults(defineProps<{
  title: string
  size: {width: number; height: number}
  bounds: {width: number; height: number}
  /** 随建筑投影变化的整体缩放，内部文字与图标保持设计比例。 */
  scale: number
  tone: 'lab' | 'station'
  info: string
  /** 标题页签宽度（设计像素），页签右缘斜切到窗口上沿 */
  tab?: number
}>(), {tab: 210})
const emit = defineEmits<{(event: 'close'): void}>()

const zoom = computed(() => Math.max(.01, Math.min(props.scale, props.bounds.height / props.size.height, props.bounds.width / props.size.width)))
</script>

<template>
  <!-- ~ 游戏建筑窗口外框 -->
  <!-- * 说明：复刻戴森球计划建筑窗口：模糊玻璃底、带切角的标题页签、六边形信息/关闭徽章与转角亮边 -->
  <!-- * 交互：徽章左侧悬停显示建筑信息，右侧关闭窗口；窗口内的键盘焦点在徽章与内容按钮之间切换 -->
  <!-- * 边界：只负责外框与缩放，内容区域按设计像素绝对定位，由具体窗口提供 -->
  <section class="game-window" :class="`is-${tone}`" :style="{width: `${size.width}px`, height: `${size.height}px`, zoom, '--tab': `${tab}px`}" role="dialog" :aria-label="title">
    <div class="game-window-glass" aria-hidden="true" />
    <svg class="game-window-frame" aria-hidden="true">
      <rect class="edge edge-left" />
      <rect class="edge edge-bottom" />
      <rect class="edge edge-right" />
      <rect class="edge edge-top" />
      <path class="line" :d="`M0.5 28L28 .5H${tab}L${tab + 24} 24.5`" />
      <path class="line highlight" d="M.5 40V28L28 .5H72" />
      <svg x="100%" overflow="visible"><path class="line" d="M-9 24.5L-.5 33" /><path class="line accent" d="M-30 24.5H-9L-.5 33V64" /></svg>
      <svg y="100%" overflow="visible"><path class="line accent" d="M1 -18V-1H18" /></svg>
      <svg x="100%" y="100%" overflow="visible"><path class="line accent" d="M-18 -1H-1V-18" /></svg>
    </svg>
    <h2 class="game-window-title">{{ title }}</h2>
    <div class="game-window-badge">
      <svg class="badge-shape" viewBox="0 0 86 38" aria-hidden="true"><path d="M13 1.5H73L84 19L73 36.5H13L2 19Z" /></svg>
      <n-tooltip placement="bottom" :style="{maxWidth: '260px'}">
        <template #trigger>
          <span class="badge-button" tabindex="0" role="img" aria-label="建筑信息">
            <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path d="M4 3h13v7.2l-2-.9V5H6v14h5.4l.8 2H4V3Zm4 4h7v2H8V7Zm0 4h4v2H8v-2Zm0 4h3v2H8v-2Zm6-3.2 8 3.6-3.4 1.2 2.6 3.6-1.7 1.2-2.6-3.6-2.1 2.8L14 11.8Z" fill="currentColor" /></svg>
          </span>
        </template>
        {{ info }}
      </n-tooltip>
      <button type="button" class="badge-button" aria-label="关闭窗口" @click="emit('close')"><X :size="20" :stroke-width="2.6" /></button>
    </div>
    <slot />
  </section>
</template>

<style scoped lang="scss">
.game-window { position: relative; flex: 0 0 auto; color: #eef1f6; font-family: var(--ds-font-ui); letter-spacing: 0; pointer-events: auto; user-select: none; animation: game-window-in 180ms cubic-bezier(.2, .8, .2, 1); }
.game-window-glass { position: absolute; inset: 0; clip-path: polygon(0 28px, 28px 0, var(--tab) 0, calc(var(--tab) + 24px) 24px, calc(100% - 9px) 24px, 100% 33px, 100% 100%, 0 100%); backdrop-filter: blur(18px) saturate(1.35); animation: game-glass-in 180ms ease-out; }
.is-lab .game-window-glass { background: radial-gradient(120% 70% at 18% 92%, rgba(40, 92, 120, .32), transparent 60%), radial-gradient(90% 60% at 82% 100%, rgba(22, 64, 150, .38), transparent 70%), linear-gradient(180deg, rgba(22, 34, 64, .88), rgba(15, 30, 66, .88)); }
.is-station .game-window-glass { background: radial-gradient(80% 40% at 12% 82%, rgba(36, 64, 130, .26), transparent 70%), linear-gradient(180deg, rgba(10, 11, 17, .93) 0%, rgba(9, 10, 16, .93) 55%, rgba(12, 16, 34, .92) 100%); }
.game-window-frame { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.edge { fill: rgba(150, 178, 205, .26); }
.edge-left { x: 0; y: 28px; width: 1px; height: calc(100% - 28px); }
.edge-bottom { x: 0; y: calc(100% - 1px); width: 100%; height: 1px; }
.edge-right { x: calc(100% - 1px); y: 33px; width: 1px; height: calc(100% - 33px); }
.edge-top { x: calc(var(--tab) + 24px); y: 24px; width: calc(100% - var(--tab) - 33px); height: 1px; }
.line { fill: none; stroke: rgba(150, 178, 205, .26); stroke-width: 1; }
.highlight { stroke: rgba(214, 246, 255, .92); stroke-width: 2; }
.accent { stroke: #92dbee; stroke-width: 2; }
.game-window-title { position: absolute; top: 15px; left: 33px; margin: 0; color: #f4f6fa; font-size: 22px; font-weight: 700; line-height: 32px; white-space: nowrap; }
.is-station .game-window-title { top: 13px; font-size: 21px; }
.game-window-badge { position: absolute; top: 5px; right: 16px; display: flex; width: 86px; height: 38px; align-items: center; justify-content: center; gap: 6px; }
.badge-shape { position: absolute; inset: 0; width: 100%; height: 100%; }
.badge-shape path { fill: rgba(22, 40, 62, .96); stroke: #92dbee; stroke-width: 2.2; }
.is-station .badge-shape path { fill: rgba(16, 36, 46, .96); }
.badge-button { position: relative; display: inline-flex; width: 30px; height: 30px; align-items: center; justify-content: center; border: 0; padding: 0; background: transparent; color: #bdeaf6; cursor: pointer; transition: color 120ms, filter 120ms; }
.badge-button:hover, .badge-button:focus-visible { color: #fff; filter: drop-shadow(0 0 4px rgba(146, 219, 238, .9)); outline: none; }
@keyframes game-window-in { from { translate: 0 6px; scale: .985; } }
@keyframes game-glass-in { from { opacity: 0; } }
@media (prefers-reduced-motion: reduce) { .game-window, .game-window-glass { animation: none; } .badge-button { transition: none; } }
</style>
