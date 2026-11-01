<script setup lang="ts">
import {nextTick, onActivated, onDeactivated, onBeforeUnmount, onMounted, ref, shallowRef, watch} from 'vue'
import {BLUEPRINT_PREVIEW_BUILDING_LIMIT, claimBlueprintPreview} from '../../../services/dysonSphere/blueprint-preview-host'
import {enableBlueprintPreviewRotation} from '../../../services/dysonSphere/blueprint-preview-interaction'
import {useThemeMode, type ThemeMode} from '../../../composables/useThemeMode'
import type {BlueprintViewer as BlueprintViewerInstance} from '../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js'

type BlueprintEditorModule = typeof import('../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js')

interface BlueprintViewerPanelProps {
  code: string
  interactive?: boolean
  selectedIndex?: number | null
  maxBuildings?: number
  revision?: number
}

interface AppThemeSnapshot {
  readonly rootThemeMode: ThemeMode | null
  readonly blueprintEditorStoredTheme: string | null
}

const props = withDefaults(defineProps<BlueprintViewerPanelProps>(), {interactive: false, selectedIndex: null, maxBuildings: BLUEPRINT_PREVIEW_BUILDING_LIMIT, revision: 0})
const emit = defineEmits<{
  (event: 'ready', value: {buildingCount: number}): void
  (event: 'select', value: {index: number | null}): void
  (event: 'error', value: {code: string; message: string}): void
}>()
const {themeMode, isThemeReady} = useThemeMode()

// =============== 蓝图模型预览状态 ===============
// 说明：在当前资源详情抽屉内挂载 BlueprintViewer，保持预览区域和页面其它 UI 同源协作。
// 边界：使用纯预览 API，控制器映射兼容集中在独立输入适配模块；不启用完整编辑器的编辑和保存能力。
const containerRef = ref<HTMLElement | null>(null)
const viewer = shallowRef<BlueprintViewerInstance | null>(null)
const isLoadingViewer = ref(false)
const viewerError = ref('')
let resizeObserver: ResizeObserver | null = null
let pendingLoadFrame = 0
let pendingResizeFrame = 0
let resizeSettleTimer = 0
let lastSize = {width: 0, height: 0}
let active = true
let loadVersion = 0
let releaseHost: (() => void) | null = null
let releaseInteraction: (() => void) | null = null
const buildingCount = ref(0)
const blueprintEditorThemeStorageKey = 'dsp-editor-theme'

// =============== 应用主题隔离 ===============
// 说明：第三方蓝图库加载时会写入 html[data-theme]，这里在每次外部调用后恢复盒子当前主题。
// 边界：只恢复应用根主题和蓝图库自己的主题缓存，不接管用户在设置页选择主题的业务状态。
const readRootThemeMode = (): ThemeMode | null => {
  const rootTheme = document.documentElement.dataset.theme
  if (rootTheme === 'dark' || rootTheme === 'light') {
    return rootTheme
  }

  return null
}

const captureAppThemeSnapshot = (): AppThemeSnapshot => ({
  rootThemeMode: readRootThemeMode(),
  blueprintEditorStoredTheme: localStorage.getItem(blueprintEditorThemeStorageKey),
})

const restoreAppThemeSnapshot = (snapshot: AppThemeSnapshot): void => {
  if (snapshot.blueprintEditorStoredTheme === null) {
    localStorage.removeItem(blueprintEditorThemeStorageKey)
  } else {
    localStorage.setItem(blueprintEditorThemeStorageKey, snapshot.blueprintEditorStoredTheme)
  }

  const restoreThemeMode = isThemeReady.value
    ? themeMode.value
    : snapshot.rootThemeMode ?? themeMode.value
  document.documentElement.dataset.theme = restoreThemeMode
  document.documentElement.style.colorScheme = restoreThemeMode
}

const withAppThemeIsolation = <T>(operation: () => T): T => {
  const snapshot = captureAppThemeSnapshot()
  try {
    return operation()
  } finally {
    restoreAppThemeSnapshot(snapshot)
  }
}

// =============== 预览器模块加载 ===============
// 说明：按需导入本地 ESM 产物；库以 import.meta.url 定位同目录的 models/、img/，开发期无需额外配置。
// 边界：模块只导入一次且不触碰 window 全局；不再通过 ?url 注入脚本或兼容 exports/module。
const ensureBlueprintEditor = (): Promise<BlueprintEditorModule> =>
  import('../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js')

// =============== 预览器生命周期 ===============
// 说明：初始化、加载蓝图和同步尺寸都集中在这里，详情切换时复用实例刷新模型。
// 边界：容器不可用或编码为空时不创建实例；组件卸载时释放 WebGL 资源。
const selectBuilding = (index: number | null): void => {
  if (index !== null && (!Number.isInteger(index) || index < 0 || index >= buildingCount.value)) return
  withAppThemeIsolation(() => viewer.value?.selectBuilding(index))
}
const resetCamera = (): void => { withAppThemeIsolation(() => viewer.value?.resetCamera()) }
const captureImage = (): string | null => withAppThemeIsolation(() => viewer.value?.toDataURL?.() ?? null)
defineExpose({selectBuilding, resetCamera, captureImage})

// 说明：当前库反复 loadBlueprint 会保留已上传的纹理；通过公开 dispose/getCanvas 释放整个旧实例及 WebGL 上下文，再建立新实例。
// 边界：不修改第三方包；停止其渲染循环后才丢失上下文，避免隐藏页继续持有 GPU 资源。
const releaseViewerInstance = (): void => {
  const canvas = viewer.value?.getCanvas?.() ?? containerRef.value?.querySelector('canvas')
  releaseInteraction?.()
  releaseInteraction = null
  withAppThemeIsolation(() => viewer.value?.dispose())
  viewer.value = null
  const context = canvas?.getContext('webgl2') ?? canvas?.getContext('webgl')
  context?.getExtension('WEBGL_lose_context')?.loseContext()
}

const disposeViewer = (): void => {
  loadVersion++
  if (pendingLoadFrame) window.cancelAnimationFrame(pendingLoadFrame)
  pendingLoadFrame = 0
  if (pendingResizeFrame) window.cancelAnimationFrame(pendingResizeFrame)
  pendingResizeFrame = 0
  window.clearTimeout(resizeSettleTimer)
  lastSize = {width: 0, height: 0}
  resizeObserver?.disconnect()
  resizeObserver = null
  releaseViewerInstance()
  releaseHost?.()
  releaseHost = null
  isLoadingViewer.value = false
}

// 蓝图加载器入口
const loadCurrentBlueprint = async (): Promise<void> => {
  // 检查状态
  if (!active || !containerRef.value || !props.code) return
  if (!hasRenderableSize()) return

  const version = ++loadVersion
  isLoadingViewer.value = true
  viewerError.value = ''
  let phase: 'load' | 'render' = 'load'

  try {
    // 加载蓝图编辑器
    const {BlueprintViewer} = await ensureBlueprintEditor()
    // 检查状态
    if (!active || version !== loadVersion || !containerRef.value) return
    phase = 'render'
    // 清理旧实例
    releaseViewerInstance()
    // 创建新实例
    if (!viewer.value) {
      // 登记释放函数
      releaseHost = claimBlueprintPreview(disposeViewer)
      // 
      viewer.value = withAppThemeIsolation(() => new BlueprintViewer({container: containerRef.value!, theme: themeMode.value}, {
        // 注册建筑选择事件
        onBuildingSelect: (pick) => withAppThemeIsolation(() => {
          if (active && props.interactive) emit('select', {index: pick?.index ?? null})
        }),
      }))
      // 获取画布
      const canvas = viewer.value.getCanvas?.()
      // 启用旋转交互
      if (canvas) releaseInteraction = enableBlueprintPreviewRotation(viewer.value, canvas)
    }
    withAppThemeIsolation(() => viewer.value?.loadBlueprint(props.code))
    buildingCount.value = viewer.value.getBlueprintData()?.buildings.length ?? 0
    // 强制重置尺寸
    lastSize = {width: 0, height: 0}
    // 同步画布尺寸
    applyViewerSize()
    // 重置相机
    resetCamera()
    // 回填选中态
    selectBuilding(props.selectedIndex)
    // 提示建筑数量
    if (buildingCount.value > props.maxBuildings) emit('error', {code: 'PREVIEW_E_LIMIT', message: `共 ${buildingCount.value} 个建筑，模型交互可能较慢`})
    emit('ready', {buildingCount: buildingCount.value})
  } catch (error) {
    if (!active || version !== loadVersion) return
    viewerError.value = error instanceof Error ? error.message : String(error || '蓝图模型渲染失败。')
    emit('error', {code: phase === 'load' ? 'PREVIEW_E_LOAD' : 'PREVIEW_E_RENDER', message: viewerError.value})
  } finally {
    if (version === loadVersion) isLoadingViewer.value = false
  }
}

// =============== 尺寸同步 ===============
// 说明：WebGL setSize 会清空画布，连续拖动时逐帧重设会闪白。拖动期间只用 CSS 拉伸上一帧，
// 停止约 120ms 后再按最终尺寸重设并立即重绘；同一帧内的多次通知合并为一次。
const applyViewerSize = (): void => {
  pendingResizeFrame = 0
  if (!containerRef.value || !viewer.value) return
  const {width, height} = containerRef.value.getBoundingClientRect()
  const next = {width: Math.floor(width), height: Math.floor(height)}
  if (next.width <= 0 || next.height <= 0 || (next.width === lastSize.width && next.height === lastSize.height)) return
  lastSize = next
  withAppThemeIsolation(() => {
    viewer.value?.setSize(next.width, next.height)
    viewer.value?.toDataURL?.()
  })
}
const resizeViewer = (): void => {
  if (!viewer.value) return
  window.clearTimeout(resizeSettleTimer)
  resizeSettleTimer = window.setTimeout(() => {
    if (pendingResizeFrame) return
    pendingResizeFrame = window.requestAnimationFrame(applyViewerSize)
  }, 120)
}

const hasRenderableSize = (): boolean => {
  if (!containerRef.value) {
    return false
  }
  const {width, height} = containerRef.value.getBoundingClientRect()
  return width > 0 && height > 0
}

const scheduleBlueprintLoad = (): void => {
  if (pendingLoadFrame) {
    return
  }
  pendingLoadFrame = window.requestAnimationFrame(() => {
    pendingLoadFrame = 0
    void loadCurrentBlueprint()
  })
}

const activate = async (): Promise<void> => {
  active = true
  await nextTick()
  if (!active) return
  if (!resizeObserver) {
    resizeObserver = new ResizeObserver(() => { if (!viewer.value && props.code && hasRenderableSize()) void loadCurrentBlueprint(); else resizeViewer() })
    if (containerRef.value) resizeObserver.observe(containerRef.value)
  }
  await loadCurrentBlueprint()
}
onMounted(() => { void activate() })
onActivated(() => { if (!viewer.value) void activate() })
onDeactivated(() => { active = false; disposeViewer() })
watch(() => props.selectedIndex, selectBuilding)

watch(() => [props.code, props.revision], () => {
  if (!props.code) { disposeViewer(); buildingCount.value = 0; return }
  void loadCurrentBlueprint()
})

watch(themeMode, (mode) => {
  if (!viewer.value) {
    return
  }

  withAppThemeIsolation(() => viewer.value?.setTheme?.(mode))
})

onBeforeUnmount(() => { active = false; disposeViewer() })

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
      <span v-if="!isLoadingViewer && !viewerError && buildingCount" class="blueprint-viewer-hint">左键拖动旋转 · 滚轮缩放</span>

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
