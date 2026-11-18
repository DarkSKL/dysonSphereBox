<script setup lang="ts">
import {onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {Graph, type IElementEvent} from '@antv/g6'
import DOMPurify from 'dompurify'
import type {AnalysisSceneBuilder} from '../../../../types/analysis-g6'
import {factoryIconImageReady, factoryIconsReady} from '../../../../pages/services/dysonSphere/factory-icons'
import '../../../../pages/services/dysonSphere/analysis/g6-rendering'

// =============== G6 画布生命周期 ===============
// 职责：管理分析图表的 G6 实例、尺寸、更新队列和浮层；使用：所有二维分析图；边界：隐藏时不绘制，卸载时销毁实例和观察器。
const props = defineProps<{scene: AnalysisSceneBuilder; label: string}>()
const emit = defineEmits<{(event: 'hover', id: string | null): void; (event: 'pick', id: string): void; (event: 'ready'): void}>()
const host = ref<HTMLElement | null>(null)
const surface = ref<HTMLElement | null>(null)
const failed = ref(false)
const tooltip = ref('')
const tipX = ref(8)
const tipY = ref(8)
const tipAbove = ref(false)
let graph: Graph | null = null
let observer: ResizeObserver | null = null
let frame = 0
let drawing = false
let pending = false
let disposed = false
let rendered = false
let content = new Map<string, string>()
let hoveredId: string | null = null

// 更新串行化：连续量化、主题切换和尺寸变化只消费最新场景，避免异步 draw 与 destroy 交错。
const schedule = (): void => {
  pending = true
  if (!frame && !disposed && !drawing) frame = requestAnimationFrame(() => {frame = 0; void draw()})
}
const leave = (): void => {hoveredId = null; tooltip.value = ''; emit('hover', null)}
const draw = async (): Promise<void> => {
  if (drawing || disposed) return
  drawing = true
  try {
    while (pending && !disposed) {
      pending = false
      const element = host.value
      const container = surface.value
      if (!element || !container) continue
      // G6 会给内部宿主写入像素宽高；观察外层容器，避免这些内联尺寸锁住后续缩放。
      const width = Math.floor(container.clientWidth)
      const height = Math.floor(container.clientHeight)
      if (width < 2 || height < 2 || !element.getClientRects().length) continue
      if (!graph) {
        graph = new Graph({container: element, width, height, autoResize: false, animation: false, padding: 0, behaviors: [], node: {type: 'dsbox-analysis-shape', style: {label: false, icon: false, halo: false, badge: false, port: false}}})
        graph.on('node:pointerenter', (event: IElementEvent) => {
          const id = String(event.target.id)
          hoveredId = id
          tooltip.value = DOMPurify.sanitize(content.get(id) ?? '')
          emit('hover', id)
        })
        graph.on('node:pointerleave', leave)
        graph.on('node:click', (event: IElementEvent) => emit('pick', String(event.target.id)))
      }
      const instance = graph
      const [previousWidth, previousHeight] = instance.getSize()
      const resized = previousWidth !== width || previousHeight !== height
      // G6 缩放画布时保留相机位置，而场景使用容器像素坐标；尺寸变化后恢复原点，避免与 SVG 光环、DOM 图标错位。
      const nodes = props.scene(width, height)
      if (resized) {
        instance.setSize(width, height)
        await instance.translateTo([0, 0], false)
        if (disposed) return
      }
      content = new Map(nodes.filter(node => node.data?.tooltip).map(node => [node.id, node.data!.tooltip!]))
      tooltip.value = DOMPurify.sanitize(content.get(hoveredId ?? '') ?? '')
      instance.setData({nodes})
      if (!rendered) {
        await instance.render()
        rendered = true
        if (!disposed) emit('ready')
      } else await instance.draw()
      // 改变像素尺寸会清空画布；在本帧提交新图层，避免拖动时展示空白帧。
      if (resized && !disposed) instance.getCanvas().getLayer().render()
      failed.value = false
    }
  } catch (error) {
    if (!disposed) {failed.value = true; console.error('G6 分析图表绘制失败', error)}
  } finally {drawing = false; if (pending && !disposed) schedule()}
}
// 浮层局限在当前图表内，不捕获指针，也不阻挡环上图标和页面滚动。
const move = (event: PointerEvent): void => {
  const element = surface.value
  if (!element) return
  const rect = element.getBoundingClientRect()
  tipX.value = Math.max(8, Math.min(event.clientX - rect.left + 12, element.clientWidth - 288))
  tipY.value = event.clientY - rect.top + 12
  tipAbove.value = event.clientY - rect.top > element.clientHeight / 2
}
watch([() => props.scene, factoryIconImageReady, factoryIconsReady], schedule, {flush: 'post'})
onMounted(() => {observer = new ResizeObserver(schedule); if (surface.value) observer.observe(surface.value); schedule()})
onBeforeUnmount(() => {
  disposed = true
  cancelAnimationFrame(frame)
  observer?.disconnect()
  graph?.destroy()
  graph = null
  content.clear()
})
</script>

<template>
  <!-- ~ G6 分析画布 -->
  <!-- * 说明：G6 负责实际图形绘制，Vue 仅管理主题浮层与错误状态 -->
  <!-- * 边界：浮层 HTML 经清理；尺寸为零时等待容器重新显示 -->
  <div ref="surface" class="g6-chart" data-renderer="g6" role="img" :aria-label="label" @pointermove="move" @pointerleave="leave">
    <div ref="host" class="g6-canvas" />
    <div v-if="tooltip" class="g6-tooltip" :class="{'is-above': tipAbove}" :style="{left: `${tipX}px`, top: `${tipY}px`}" v-html="tooltip" />
    <p v-if="failed" class="g6-error">图表暂时无法显示</p>
  </div>
</template>

<style scoped lang="scss">
.g6-chart { position: relative; width: 100%; height: 100%; min-height: 0; }
.g6-canvas { position: absolute; inset: 0; }
.g6-tooltip { position: absolute; z-index: 10; max-width: min(280px, calc(100% - 16px)); border: 1px solid var(--ds-border-strong); border-radius: 2px; padding: 8px 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-primary); font-size: 12px; line-height: 1.65; pointer-events: none; }
.g6-tooltip.is-above { transform: translateY(calc(-100% - 24px)); }
.g6-error { position: absolute; inset: 0; display: grid; place-items: center; margin: 0; color: var(--ds-text-muted); font-size: 12px; }
</style>
