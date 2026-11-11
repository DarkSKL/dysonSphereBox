<script setup lang="ts">
import {nextTick, onActivated, onDeactivated, onBeforeUnmount, onMounted, ref, shallowRef, watch} from 'vue'
import {BLUEPRINT_PREVIEW_BUILDING_LIMIT, claimBlueprintPreview} from '../../../pages/services/dysonSphere/blueprint-preview-host'
import {enableBlueprintPreviewInteraction} from '../../../pages/services/dysonSphere/blueprint-preview-interaction'
import {useThemeMode, type ThemeMode} from '../../../composables/useThemeMode'
import type {BlueprintViewer as BlueprintViewerInstance} from '../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js'

type BlueprintEditorModule = typeof import('../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js')

interface BlueprintViewerPanelProps {
  code: string
  interactive?: boolean
  selectedBuildings?: number[]
  focusIndex?: number | null
  focusRevision?: number
  maxBuildings?: number
  revision?: number
}

interface AppThemeSnapshot {
  readonly rootThemeMode: ThemeMode | null
  readonly blueprintEditorStoredTheme: string | null
}

const props = withDefaults(defineProps<BlueprintViewerPanelProps>(), {interactive: false, selectedBuildings: () => [], focusIndex: null, focusRevision: 0, maxBuildings: BLUEPRINT_PREVIEW_BUILDING_LIMIT, revision: 0})
const emit = defineEmits<{
  (event: 'ready', value: {buildingCount: number; blank?: boolean}): void
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

// =============== 空白蓝图 ===============
// 说明：仅包含星球主体、没有任何建筑的合法蓝图；预览器首次加载与增量预览前清场都用它。
// 边界：加载失败时正式场景可能仍是旧蓝图，调用方必须终止后续预览动作。
const BLANK_BLUEPRINT = 'BLUEPRINT:1,10,0,0,0,0,0,0,639273500692970000,0.10.35.29104,%E6%96%B0%E8%93%9D%E5%9B%BE,,,,"H4sIAAAAAAAAA2NiYGDgAGJmBggQAGI2KJuR4T8DwwmoMBtUCAgA9wEkuTQAAAA="7FFF389CC152F6E763D8B8B72DF4D44A'

// =============== 增量预览快照 ===============
// 说明：库的 beginPreview 不会清空正式场景（保留上一次成功结果）。为避免旧蓝图与预览层叠加，
//       这里在 beginPreview 前保存旧蓝图编码与建筑数，用空白蓝图清场；commitPreview 后丢弃快照，
//       discardPreview 时用快照恢复旧蓝图。
// 边界：只在 viewer 实例就绪时操作；快照随 commit/discard 清除，组件卸载时 disposeViewer 兜底。
interface PreviewSnapshot {
  code: string
  buildingCount: number
}
let previewSnapshot: PreviewSnapshot | null = null
// commitPreview 成功后正式场景已替换为新蓝图，code 变化触发的全量重载应跳过，避免闪白与多余加载遮罩
let skipNextCodeReload = false
// =============== 量差动画提交闸门 ===============
// 说明：量差动画播放期间到达的 commitPreview（完整蓝图）先缓存，等预览转发层在动画收尾时调用 finishPreviewReplay 再原子提交；
//       动画期间到达的 code 变化同理跳过，避免整份蓝图提前覆盖动画画面。
// 边界：没有播放过差量时 commitPreview 立即提交，行为与之前一致；beginPreview / discardPreview / 释放实例都会清空缓存。
let replayingDeltas = false
let pendingCommitBlueprint: string | null = null
// 说明：把正式场景切回空白蓝图；返回是否成功，失败已上报 PREVIEW_E_CLEAR。
// 边界：调用方在返回 false 时不得继续 beginPreview 或写选中态。
const loadBlankBlueprint = (): boolean => {
  if (!viewer.value) return false
  const outcome = withAppThemeIsolation(() => viewer.value!.loadBlueprint(BLANK_BLUEPRINT))
  if (!outcome.ok) {
    emit('error', {code: 'PREVIEW_E_CLEAR', message: outcome.error.message})
    return false
  }
  return true
}

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

// 说明：组件挂载即预加载预览器 ESM，不等蓝图编码就绪，缩短首次渲染的等待时间。
// 边界：只负责模块加载；实例创建仍由 loadCurrentBlueprint 在 code 就绪后执行，失败报错由该流程承担。
const preloadBlueprintEditor = async (): Promise<void> => {
  try {
    await ensureBlueprintEditor()
  } catch {
    // 加载失败不在这里上报：code 就绪后 loadCurrentBlueprint 会走 error 事件。
  }
}

// =============== 预览器生命周期 ===============
// 说明：初始化、加载蓝图和同步尺寸都集中在这里，详情切换时复用实例刷新模型。
// 边界：容器不可用或编码为空时不创建实例；组件卸载时释放 WebGL 资源。

// =============== 多建筑选中 ===============
// 说明：预览库提供 selectedBuildings 批量高亮，这里只过滤越界下标并转调；空数组等价取消选中。
// 边界：库在任一建筑找不到时整次调用失败并保持原选中，因此必须先按建筑总数过滤。
const selectBuildings = (indices: number[]): void => {
  const next = [...new Set(indices)].filter(index => Number.isInteger(index) && index >= 0 && index < buildingCount.value)
  withAppThemeIsolation(() => viewer.value?.selectedBuildings(next.length ? next : null))
}
// 重置相机
const resetCamera = (): void => { withAppThemeIsolation(() => viewer.value?.resetCamera()) }
// 捕获当前视图
const captureImage = (): string | null => withAppThemeIsolation(() => viewer.value?.toDataURL?.() ?? null)

// 聚焦建筑
const focusBuilding = (index: number | null): void => {
  withAppThemeIsolation(() => viewer.value?.focusBuilding(index))
}
// 上半部分触发的聚焦：选中态已由 selectBuildings 处理，这里只移动镜头
const focusRequestedBuilding = (): void => {
  const index = props.focusIndex
  if (index === null || !Number.isInteger(index) || index < 0 || index >= buildingCount.value) return
  withAppThemeIsolation(() => viewer.value?.focusBuilding(index, {select: false}))
}
// 设置渲染参数
const setRenderSettings = (showRealModels: boolean, maxInstancesPerModel: number): void => {
  withAppThemeIsolation(() => viewer.value?.setRenderSettings({showRealModels, maxInstancesPerModel}))
}

// =============== 增量预览 API ===============
// 说明：对接后端 preview-delta 事件流；所有调用走主题隔离，避免第三方库写入 html[data-theme]。
// 边界：调用顺序由上层协调层负责（beginPreview → 提交生成 → applyPreviewDelta → commitPreview/discardPreview）。
//       beginPreview 前保存并清空旧正式场景；commitPreview 后丢弃快照；discardPreview 恢复旧场景。
const beginPreview = (): void => {
  if (!viewer.value) return
  // 新一轮量差动画：清空上一轮缓存的待提交蓝图与动画标记
  replayingDeltas = false
  pendingCommitBlueprint = null
  // 保存旧蓝图快照（编码 + 建筑数），供 discardPreview 恢复
  previewSnapshot = {code: props.code, buildingCount: buildingCount.value}
  // 清空正式场景：库的 beginPreview 保留旧场景，这里用空白蓝图清场，避免与预览层叠加
  // 关键：清空失败时正式场景还是旧蓝图，不能继续 beginPreview
  if (!loadBlankBlueprint()) {
    previewSnapshot = null
    return
  }
  buildingCount.value = 0
  withAppThemeIsolation(() => viewer.value?.beginPreview())
}
const applyPreviewDelta = (event: unknown): boolean => {
  // 防御：仍带候选维度（candidateId）或显式 v4 标记的事件说明页面转发层漏过滤。
  // 此时返回 true 表示"已由上层处理"，但不交给底层库，避免多候选对象污染单场景镜像。
  const value = event as {candidateId?: unknown; protocolVersion?: unknown} | null
  if (value && (value.candidateId !== undefined || value.protocolVersion === 4)) return true
  // 后端 FactoryGenerationProgress 与库 PreviewDeltaEvent 结构兼容，协议层已校验 kind/protocolVersion/operations，这里直接透传。
  const applied = withAppThemeIsolation(() => viewer.value?.applyPreviewDelta(event as Parameters<typeof viewer.value.applyPreviewDelta>[0]) ?? false)
  // 差量已被接受：标记量差动画进行中，后续到达的完整蓝图要缓存到动画收尾
  if (applied) replayingDeltas = true
  return applied
}
// 完整蓝图原子替换：成功后正式场景即新蓝图，跳过 code 变化触发的全量重载
const commitBlueprint = (blueprint: string): unknown => {
  const outcome = withAppThemeIsolation(() => viewer.value?.commitPreview(blueprint))
  const ok = outcome && typeof outcome === 'object' && 'ok' in outcome && outcome.ok === true
  if (ok) {
    // commitPreview 内部先建后换，正式场景已替换为新蓝图；跳过 code 变化触发的全量重载
    skipNextCodeReload = true
    buildingCount.value = viewer.value?.getBlueprintData()?.buildings.length ?? 0
    previewSnapshot = null
    if (buildingCount.value > props.maxBuildings) emit('error', {code: 'PREVIEW_E_LIMIT', message: `共 ${buildingCount.value} 个建筑，模型交互可能较慢`})
    emit('ready', {buildingCount: buildingCount.value})
  }
  // commit 失败：不设跳过标记，code 变化会触发全量重载作为兜底
  return outcome
}
const commitPreview = (blueprint: string): unknown => {
  // 量差动画仍在播放：只缓存完整蓝图，等 finishPreviewReplay 在动画播完后提交，避免整份蓝图提前覆盖动画
  if (replayingDeltas) {
    pendingCommitBlueprint = blueprint
    skipNextCodeReload = true
    return {ok: true}
  }
  return commitBlueprint(blueprint)
}
// 量差动画收尾：预览转发层在重放结束（含被打断）时调用，此刻才加载动画期间缓存的完整蓝图
const finishPreviewReplay = (): void => {
  replayingDeltas = false
  const blueprint = pendingCommitBlueprint
  pendingCommitBlueprint = null
  if (blueprint !== null) commitBlueprint(blueprint)
  // 动画期间的 code 变化已在 watch 中跳过，这里清掉跳过标记，保证后续导入等加载照常执行
  skipNextCodeReload = false
}
const discardPreview = (): void => {
  withAppThemeIsolation(() => viewer.value?.discardPreview())
  // 丢弃待提交的完整蓝图与动画标记，避免悬挂状态吞掉下一次 code 变化
  replayingDeltas = false
  pendingCommitBlueprint = null
  // 恢复旧蓝图快照：重新加载旧编码（无编码时回退空白蓝图）、还原建筑数与选中态
  const snapshot = previewSnapshot
  previewSnapshot = null
  if (snapshot && viewer.value) {
    const restoreCode = snapshot.code || BLANK_BLUEPRINT
    withAppThemeIsolation(() => viewer.value!.loadBlueprint(restoreCode))
    buildingCount.value = viewer.value?.getBlueprintData()?.buildings.length ?? snapshot.buildingCount
    selectBuildings(props.selectedBuildings)
    emit('ready', {buildingCount: buildingCount.value})
  }
}
// 内部 viewer 实例是否就绪（ESM 已加载且实例已创建）；未就绪时增量预览调用全部静默失败
const isReady = (): boolean => viewer.value !== null
const getPreviewing = (): boolean => viewer.value?.previewing ?? false
const getPreviewStats = () => withAppThemeIsolation(() => viewer.value?.previewStats() ?? null)
// 暴露 API
defineExpose({selectBuildings, focusBuilding, resetCamera, captureImage, setRenderSettings, beginPreview, applyPreviewDelta, commitPreview, finishPreviewReplay, discardPreview, isReady, getPreviewing, getPreviewStats})

// 释放预览器实例
const releaseViewerInstance = (): void => {
  const canvas = viewer.value?.getCanvas?.() ?? containerRef.value?.querySelector('canvas')
  releaseInteraction?.()
  releaseInteraction = null
  withAppThemeIsolation(() => viewer.value?.dispose())
  viewer.value = null
  const context = canvas?.getContext('webgl2') ?? canvas?.getContext('webgl')
  context?.getExtension('WEBGL_lose_context')?.loseContext()
}
// 释放预览器实例
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
  previewSnapshot = null
  skipNextCodeReload = false
  replayingDeltas = false
  pendingCommitBlueprint = null
  isLoadingViewer.value = false
}

// 蓝图加载器入口：无编码时先加载空白蓝图渲染星球主体，真实编码到达后由 watch 重新加载
const loadCurrentBlueprint = async (): Promise<void> => {
  // 检查状态
  if (!active || !containerRef.value) return
  if (!hasRenderableSize()) return
  const blank = !props.code

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
      // 启用预览交互
      if (canvas) releaseInteraction = enableBlueprintPreviewInteraction(viewer.value, canvas)
    }
    setRenderSettings(false,1024)
    // 加载蓝图模型：无编码时用空白蓝图先渲染星球主体，保证预览器先于流程图出画面
    if (blank) {
      if (!loadBlankBlueprint()) return
    } else {
      withAppThemeIsolation(() => viewer.value?.loadBlueprint(props.code))
    }
    buildingCount.value = viewer.value.getBlueprintData()?.buildings.length ?? 0
    // 强制重置尺寸
    lastSize = {width: 0, height: 0}
    // 同步画布尺寸
    applyViewerSize()
    // 重置相机
    resetCamera()
    // 回填选中态
    selectBuildings(props.selectedBuildings)
    // 设置渲染参数
    setRenderSettings(true,1024)
    // 提示建筑数量
    if (buildingCount.value > props.maxBuildings) emit('error', {code: 'PREVIEW_E_LIMIT', message: `共 ${buildingCount.value} 个建筑，模型交互可能较慢`})
    emit('ready', {buildingCount: buildingCount.value, blank})
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
    resizeObserver = new ResizeObserver(() => { if (!viewer.value && hasRenderableSize()) void loadCurrentBlueprint(); else resizeViewer() })
    if (containerRef.value) resizeObserver.observe(containerRef.value)
  }
  // 先启动预览器模块预加载，再加载蓝图（无编码时加载空白蓝图渲染星球主体）
  void preloadBlueprintEditor()
  await loadCurrentBlueprint()
}
onMounted(() => { void activate() })
onActivated(() => { if (!viewer.value) void activate() })
onDeactivated(() => { active = false; disposeViewer() })
watch(() => props.selectedBuildings, selectBuildings)
watch(() => props.focusRevision, focusRequestedBuilding)

watch(() => [props.code, props.revision], () => {
  if (!props.code) { disposeViewer(); buildingCount.value = 0; return }
  // commitPreview 成功后正式场景已替换为新蓝图，跳过此次全量重载，避免闪白与加载遮罩；
  // 量差动画播放期间同理：完整蓝图已缓存，等 finishPreviewReplay 在动画收尾时替换场景。
  if (skipNextCodeReload || replayingDeltas) { skipNextCodeReload = false; return }
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
