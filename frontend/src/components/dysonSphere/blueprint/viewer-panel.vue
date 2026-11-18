<script setup lang="ts">
// ~ 蓝图模型预览面板
// * 职责：提供挂载容器与模板状态，蓝图库注册调用、主题隔离、增量预览状态机由 useBlueprintViewer / blueprint-editor-bridge 承担。
// * 使用：资源详情抽屉（environment.vue）与量化蓝图页（blueprint-generator.vue）通过 defineExpose 调用预览 API。
// * 边界：这里只做 props / emit 映射与布局；不直接调用 dsp-blueprint-editor，也不触碰 html[data-theme]。
import {ref} from 'vue'
import {BLUEPRINT_PREVIEW_BUILDING_LIMIT} from '../../../pages/services/dysonSphere/blueprint-preview-host'
import {useBlueprintViewer, type UseBlueprintViewerEmit} from '../../../composables/useBlueprintViewer'

interface BlueprintViewerPanelProps {
  code: string
  interactive?: boolean
  selectedBuildings?: number[]
  focusIndex?: number | null
  focusRevision?: number
  maxBuildings?: number
  revision?: number
}

const props = withDefaults(defineProps<BlueprintViewerPanelProps>(), {interactive: false, selectedBuildings: () => [], focusIndex: null, focusRevision: 0, maxBuildings: BLUEPRINT_PREVIEW_BUILDING_LIMIT, revision: 0})
const emit = defineEmits<{
  (event: 'ready', value: {buildingCount: number; blank?: boolean}): void
  (event: 'select', value: {index: number | null}): void
  (event: 'error', value: {code: string; message: string}): void
}>()

// =============== 预览挂载点 ===============
// 说明：蓝图库把 WebGL canvas 注入该元素；组件只负责提供容器与尺寸。
const containerRef = ref<HTMLElement | null>(null)

const {isLoadingViewer, viewerError, buildingCount, api} = useBlueprintViewer({
  container: containerRef,
  code: () => props.code,
  revision: () => props.revision,
  interactive: () => props.interactive,
  focusIndex: () => props.focusIndex,
  focusRevision: () => props.focusRevision,
  selectedBuildings: () => props.selectedBuildings,
  maxBuildings: () => props.maxBuildings,
}, emit as UseBlueprintViewerEmit)

// 暴露 API：签名与拆分前一致，调用方无需改动
defineExpose(api)
</script>

<template>
  <!-- ~ 蓝图模型预览 -->
  <!-- * 说明：在资源详情抽屉内直接挂载 dsp-blueprint-editor 的 BlueprintViewer 模型画布 -->
  <!-- * 交互：保留模型拖拽、缩放和视角重置能力；详情切换时刷新当前蓝图编码 -->
  <!-- * 依赖：依赖后端详情返回的原始 BLUEPRINT 编码和本地编辑器库暴露函数 -->
  <!-- * 边界：这里只读渲染模型，不展示标题、版本、校验、描述、建筑组成等旧信息列表 -->
  <section class="blueprint-viewer-panel">
    <div class="blueprint-viewer-canvas">
      <div ref="containerRef" class="blueprint-viewer-mount" />
      <span v-if="!isLoadingViewer && !viewerError && buildingCount" class="blueprint-viewer-hint">左键拖动平移 · 右键拖动旋转 · 滚轮缩放</span>

      <div v-if="isLoadingViewer" class="blueprint-viewer-overlay">
        <n-spin size="small" />
        <span>正在加载模型</span>
      </div>

      <div v-if="viewerError" class="blueprint-viewer-overlay blueprint-viewer-overlay-error">
        <strong>预览暂不可用，不影响已自检蓝图的导出</strong>
        <span>{{ viewerError }}</span>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.blueprint-viewer-panel {
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--ds-bg-panel);
}

.blueprint-viewer-canvas {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--ds-bg-panel);
}

.blueprint-viewer-mount {
  width: 100%;
  height: 100%;

  :deep(canvas) {
    display: block;
    width: 100% !important;
    height: 100% !important;
    cursor: grab;
    touch-action: none;

    &:active { cursor: grabbing; }
  }
}

.blueprint-viewer-hint {
  position: absolute;
  right: 8px;
  bottom: 8px;
  padding: 4px 6px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-secondary);
  font-size: 11px;
  pointer-events: none;
}

.blueprint-viewer-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: color-mix(in srgb, var(--ds-bg-panel) 85%, transparent);
  color: var(--ds-text-primary);
  font-size: 0.8125rem;
  pointer-events: none;
}

.blueprint-viewer-overlay-error {
  flex-direction: column;
  padding: 24px;
  text-align: center;

  strong {
    color: var(--ds-error);
    font-weight: 650;
  }

  span {
    max-width: 36rem;
    color: var(--ds-text-secondary);
    line-height: 1.6;
  }
}
</style>
