<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'

import {DISK_RADIUS} from '../../composables/feedback/galaxy-camera'
import {useGalaxyTheme} from '../../composables/feedback/useGalaxyTheme'
import {progressLabels, type DisplayStar, type GalaxyPosition, type StarProgress} from '../../types/feedback'

const props = defineProps<{
  stars: readonly DisplayStar[]
  selected: GalaxyPosition | null
}>()

// =============== 缩略图数据 ===============
// 说明：以盘面俯视（x/y）把当前星图快照中的全部星点缩到一个圆盘里；聚合星按 count 计入总数并略微放大。
// 边界：仅统计已公开的星（无 privateLabel）；作者自己的待审私有星只以淡色点呈现，不计入「反馈星点」总数。
const SIZE = 60
const TAU = Math.PI * 2
const order: StarProgress[] = ['feedback', 'accepted', 'implemented']
const theme = useGalaxyTheme()
const canvas = ref<HTMLCanvasElement>()

const totals = computed(() => {
  const byProgress: Record<StarProgress, number> = {feedback: 0, accepted: 0, implemented: 0}
  let total = 0
  for (const star of props.stars) {
    if (star.privateLabel) continue
    const count = star.count ?? 1
    byProgress[star.progress] += count
    total += count
  }
  return {total, byProgress}
})

function draw(): void {
  const element = canvas.value
  const context = element?.getContext('2d')
  if (!element || !context) return
  const ratio = window.devicePixelRatio || 1
  if (element.width !== Math.round(SIZE * ratio)) {
    element.width = Math.round(SIZE * ratio)
    element.height = Math.round(SIZE * ratio)
  }
  context.setTransform(ratio, 0, 0, ratio, 0, 0)
  context.clearRect(0, 0, SIZE, SIZE)

  const current = theme.value
  const dark = current.mode === 'dark'
  const half = SIZE / 2
  const edge = half - 2
  const scale = (edge - 1) / DISK_RADIUS
  const place = (point: GalaxyPosition): {x: number; y: number} => {
    let dx = point.x * scale
    let dy = point.y * scale
    const distance = Math.hypot(dx, dy)
    if (distance > edge - 1) {
      dx *= (edge - 1) / distance
      dy *= (edge - 1) / distance
    }
    return {x: half + dx, y: half + dy}
  }

  context.beginPath()
  context.arc(half, half, edge, 0, TAU)
  context.fillStyle = dark ? 'rgba(138, 216, 255, 0.06)' : 'rgba(26, 30, 36, 0.045)'
  context.fill()
  context.lineWidth = 1
  context.strokeStyle = dark ? 'rgba(138, 216, 255, 0.30)' : 'rgba(26, 30, 36, 0.26)'
  context.stroke()

  for (const star of props.stars) {
    const {x, y} = place(star)
    context.beginPath()
    context.arc(x, y, Math.min(2.2, 0.8 + Math.log10(star.count ?? 1) * 0.7), 0, TAU)
    context.globalAlpha = star.privateLabel ? 0.5 : 0.92
    context.fillStyle = star.privateLabel ? current.privateColor : (current.starColors[star.progress] ?? current.starColors.feedback)
    context.fill()
  }
  context.globalAlpha = 1

  if (props.selected) {
    const {x, y} = place(props.selected)
    context.beginPath()
    context.arc(x, y, 4.2, 0, TAU)
    context.lineWidth = 1.2
    context.strokeStyle = dark ? '#f7fbff' : '#1a1e24'
    context.stroke()
  }
}

watch([() => props.stars, () => props.selected, theme], draw, {flush: 'post'})
onMounted(draw)
</script>

<template>
  <!-- ~ 星图微型缩略图 -->
  <!-- * 说明：详情卡片右上角的只读小盘面，一眼看出当前快照里有多少颗反馈星、分布在哪、读的是哪一颗。 -->
  <!-- * 交互：纯展示，不拦截指针，点击仍落到遮罩上关闭卡片。 -->
  <!-- * 依赖：星图快照星点、当前星位置与星图主题色板。 -->
  <!-- * 边界：遮罩宽度不足以与居中卡片并排时隐藏，避免压住卡片内容。 -->
  <div class="galaxy-minimap" role="img" :aria-label="'星图缩略图，当前共 ' + totals.total + ' 颗反馈星点'">
    <canvas ref="canvas" class="minimap-canvas" aria-hidden="true" />
    <div class="minimap-stats">
      <span class="minimap-label">反馈星点</span>
      <strong class="minimap-total type-display">{{ totals.total }}</strong>
      <span class="minimap-legend">
        <span v-for="key in order" :key="key" class="legend-item" :title="progressLabels[key]">
          <i :style="{background: theme.starColors[key]}" aria-hidden="true" />{{ totals.byProgress[key] }}
        </span>
      </span>
    </div>
  </div>
</template>

<style scoped lang="scss">
.galaxy-minimap {
  position: absolute;
  z-index: 1;
  top: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px 8px 8px;
  border: 1px solid var(--ds-border-default);
  border-radius: 6px;
  background: color-mix(in srgb, var(--ds-bg-panel) 88%, transparent);
  backdrop-filter: blur(6px);
  pointer-events: none;
  user-select: none;
}

.minimap-canvas {
  flex: none;
  display: block;
  width: 60px;
  height: 60px;
}

.minimap-stats {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 56px;
}

.minimap-label {
  color: var(--ds-text-muted);
  font-size: 10.5px;
  line-height: 1.4;
}

.minimap-total {
  color: var(--ds-text-primary);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

.minimap-legend {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  color: var(--ds-text-secondary);
  font-size: 10.5px;
  line-height: 1.4;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 3px;

  i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
}

// 遮罩宽度分级：>=920px 完整面板；680-919px 收成只有圆盘与总数的紧凑版（贴在居中卡片外的右侧余白里）；更窄时隐藏，避免压住卡片。
@container star-backdrop (max-width: 919px) {
  .galaxy-minimap {
    top: 10px;
    right: 10px;
    flex-direction: column;
    gap: 2px;
    padding: 0;
    border: 0;
    background: transparent;
    backdrop-filter: none;
  }

  .minimap-canvas {
    width: 46px;
    height: 46px;
  }

  .minimap-label,
  .minimap-legend {
    display: none;
  }

  .minimap-stats {
    min-width: 0;
    align-items: center;
  }

  .minimap-total {
    font-size: 14px;
  }
}

@container star-backdrop (max-width: 679px) {
  .galaxy-minimap {
    display: none;
  }
}
</style>
