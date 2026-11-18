<script setup lang="ts">
import {onMounted, ref, shallowRef, type Component} from 'vue'

defineOptions({inheritAttrs: false})
const props = defineProps<{label: string; load: () => Promise<{default: Component}>}>()
const panel = shallowRef<Component>()
const failed = ref(false)

// 首次挂载才请求模块；失败时保留当前标签，允许在原位重试。
const loadPanel = async (): Promise<void> => {
  failed.value = false
  try {
    panel.value = (await props.load()).default
  } catch (error) {
    console.error(`[${props.label}] 面板加载失败`, error)
    failed.value = true
  }
}
onMounted(loadPanel)
</script>

<template>
  <component :is="panel" v-if="panel" v-bind="$attrs" />
  <!-- 面板资源未就绪时在同一显示区域提供反馈，避免露出空白背景。 -->
  <div v-else class="panel-status" role="status">
    <template v-if="failed">
      <span>{{ label }}加载失败</span>
      <n-button size="tiny" @click="loadPanel">重新加载</n-button>
    </template>
    <template v-else>
      <!-- 六条等长路径自下向上绘入，再按同一顺序擦除；状态文字负责读屏提示。 -->
      <svg class="panel-bars" viewBox="0 0 49 26" aria-hidden="true" focusable="false">
        <path v-for="i in 6" :key="i" :d="`M${4 + (i - 1) * 8} 25v-24`" pathLength="1" :style="{animationDelay: `${(i - 1) * 60}ms`}" />
      </svg>
      <span>正在加载{{ label }}…</span>
    </template>
  </div>
</template>

<style scoped>
.panel-status {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--ds-text-secondary);
  background: var(--ds-bg-panel);
  font-size: 13px;
}
.panel-bars { width: 49px; height: 26px; color: var(--ds-brand); }
.panel-bars path {
  fill: none;
  stroke: currentColor;
  stroke-width: 5;
  stroke-dasharray: 1 1;
  stroke-dashoffset: 1;
  animation: panel-bar-trace 1200ms linear infinite;
}
@keyframes panel-bar-trace {
  0% { stroke-dashoffset: 1; }
  15%, 50% { stroke-dashoffset: 0; }
  65%, 100% { stroke-dashoffset: -1; }
}
@media (prefers-reduced-motion: reduce) {
  .panel-bars path { animation: none; stroke-dashoffset: 0; }
}
</style>
