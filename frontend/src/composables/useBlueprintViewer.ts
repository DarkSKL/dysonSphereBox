// =============== 蓝图预览组合式封装 ===============
// 职责：把 blueprint-editor-bridge 的纯 TS 桥接接到 Vue 生命周期上，同步模板状态并返回 defineExpose 可用的 API。
// 使用：components/dysonSphere/blueprint/viewer-panel.vue 在 setup 中调用一次。
// 边界：只做 Vue 胶水（watch / KeepAlive 钩子 / ref 同步 / emit 映射），不包含预览库调用与主题隔离逻辑。

import {nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch, type Ref} from 'vue'
import {
  BlueprintEditorBridge,
  type BlueprintEditorThemePort,
  type BlueprintViewerBridgeApi,
  type BlueprintViewerBridgeState,
} from '../pages/services/dysonSphere/blueprint-editor-bridge'
import {useThemeMode} from './useThemeMode'

// =============== 组件输入 ===============
// 说明：与 viewer-panel 的 props 一一对应；用读取函数交给桥接，保证桥接读到的是最新响应式值。
export interface UseBlueprintViewerOptions {
  container: Ref<HTMLElement | null>
  code: () => string
  revision: () => number
  interactive: () => boolean
  focusIndex: () => number | null
  focusRevision: () => number
  selectedBuildings: () => number[]
  maxBuildings: () => number
}

// =============== 组件输出事件 ===============
// 说明：与组件 defineEmits 的三个事件载荷完全一致，由组件原样转交 emit。
export interface UseBlueprintViewerEmit {
  (event: 'ready', value: {buildingCount: number; blank?: boolean}): void
  (event: 'select', value: {index: number | null}): void
  (event: 'error', value: {code: string; message: string}): void
}

// =============== 组合式封装 ===============
export const useBlueprintViewer = (options: UseBlueprintViewerOptions, emit: UseBlueprintViewerEmit) => {
  const {themeMode, isThemeReady} = useThemeMode()

  const isLoadingViewer = ref(false)
  const viewerError = ref('')
  const buildingCount = ref(0)

  // 主题端口：桥接只读取当前应用主题与就绪状态，写入仍由 useThemeMode 独占
  const themePort: BlueprintEditorThemePort = {
    read: () => themeMode.value,
    isReady: () => isThemeReady.value,
  }

  const bridge = new BlueprintEditorBridge({
    inputs: {
      container: () => options.container.value,
      blueprint: () => ({code: options.code(), revision: options.revision()}),
      interactive: () => options.interactive(),
      focusIndex: () => options.focusIndex(),
      selectedBuildings: () => options.selectedBuildings(),
      maxBuildings: () => options.maxBuildings(),
    },
    theme: themePort,
    events: {
      onReady: value => emit('ready', value),
      onSelect: value => emit('select', value),
      onError: value => emit('error', value),
      onStateChange: (state: BlueprintViewerBridgeState) => {
        isLoadingViewer.value = state.loading
        viewerError.value = state.error
        buildingCount.value = state.buildingCount
      },
    },
  })

  // =============== 生命周期 ===============
  // 说明：浏览器首帧后再激活，确保容器已完成布局、能测量到可渲染尺寸；
  //       等待期间可能被 onDeactivated 打断，因此用 activated 在 nextTick 后复检，避免失活后重新建实例。
  let activated = true
  const activate = async (): Promise<void> => {
    activated = true
    await nextTick()
    if (!activated) return
    await bridge.activate()
  }

  onMounted(() => { void activate() })
  onActivated(() => { if (!bridge.isReady()) void activate() })
  onDeactivated(() => { activated = false; bridge.deactivate() })
  onBeforeUnmount(() => { activated = false; bridge.dispose() })

  // =============== 输入同步 ===============
  watch(() => options.selectedBuildings(), indices => bridge.selectBuildings(indices))
  watch(() => options.focusRevision(), () => bridge.focusRequested())
  watch(() => [options.code(), options.revision()], () => bridge.load())
  watch(themeMode, mode => bridge.setTheme(mode))

  // =============== 公开面 ===============
  // 说明：把桥接方法逐项转发，保证组件 defineExpose 的签名与拆分前完全一致。
  const api: BlueprintViewerBridgeApi = {
    selectBuildings: indices => bridge.selectBuildings(indices),
    focusBuilding: index => bridge.focusBuilding(index),
    resetCamera: () => bridge.resetCamera(),
    captureImage: () => bridge.captureImage(),
    setRenderSettings: (showRealModels, maxInstancesPerModel) => bridge.setRenderSettings(showRealModels, maxInstancesPerModel),
    beginPreview: () => bridge.beginPreview(),
    applyPreviewDelta: event => bridge.applyPreviewDelta(event),
    commitPreview: blueprint => bridge.commitPreview(blueprint),
    beginPreviewReplay: () => bridge.beginPreviewReplay(),
    finishPreviewReplay: () => bridge.finishPreviewReplay(),
    discardPreview: () => bridge.discardPreview(),
    isReady: () => bridge.isReady(),
    getPreviewing: () => bridge.getPreviewing(),
    getPreviewStats: () => bridge.getPreviewStats(),
  }

  return {
    isLoadingViewer,
    viewerError,
    buildingCount,
    api,
  }
}