<script setup lang="ts">
import {BaseEdge, EdgeLabelRenderer, getBezierPath, Position} from '@vue-flow/core'
import {computed} from 'vue'

// 连线渲染为片段（路径 + 标签），Vue Flow 透传的属性不落到 DOM 上。
defineOptions({inheritAttrs: false})
const props = defineProps<{id: string; sourceX: number; sourceY: number; targetX: number; targetY: number; sourcePosition: Position; targetPosition: Position; data: {name: string; rate: number; cycle: boolean; label: string; width: number; highlighted: boolean; dimmed: boolean}}>()
const path = computed(() => getBezierPath({sourceX: props.sourceX, sourceY: props.sourceY, targetX: props.targetX, targetY: props.targetY, sourcePosition: props.sourcePosition, targetPosition: props.targetPosition}))
</script>

<template>
  <!-- ~ 物料流连线 -->
  <!-- * 说明：线宽按速率对数缩放，标签显示物品与每分钟速率；循环回流使用虚线 -->
  <!-- * 边界：只展示，不允许手动改连；选中节点时相关连线高亮，其余变淡 -->
  <BaseEdge :id="id" :path="path[0]" class="flow-edge" :class="{'is-cycle': data.cycle, 'is-highlighted': data.highlighted, 'is-dimmed': data.dimmed}" :style="{strokeWidth: data.width}" />
  <EdgeLabelRenderer>
    <div class="flow-label nodrag nopan" :class="{'is-highlighted': data.highlighted, 'is-dimmed': data.dimmed}" :style="{transform: `translate(-50%, -50%) translate(${path[1]}px, ${path[2]}px)`}">{{ data.label }}</div>
  </EdgeLabelRenderer>
</template>

<style scoped>
.flow-edge { stroke: var(--ds-border-strong); fill: none; transition: stroke 150ms, opacity 150ms; }
.flow-edge.is-cycle { stroke-dasharray: 6 4; }
.flow-edge.is-highlighted { stroke: var(--ds-brand); }
.flow-edge.is-dimmed { opacity: .25; }
.flow-label { position: absolute; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 5px; background: var(--ds-bg-panel); color: var(--ds-text-secondary); font-family: var(--ds-font-display); font-size: 10px; line-height: 16px; white-space: nowrap; pointer-events: none; transition: opacity 150ms; }
.flow-label.is-highlighted { border-color: var(--ds-brand); color: var(--ds-text-primary); }
.flow-label.is-dimmed { opacity: .2; }
</style>
