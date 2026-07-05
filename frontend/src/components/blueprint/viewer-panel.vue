<script setup lang="ts">
import * as Vue from 'vue'
import {nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useThemeMode, type ThemeMode} from '../../composables/useThemeMode'
import blueprintEditorScriptUrl from '../../packages/dsp-blueprint-editor/dsp-blueprint-editor.js?url'

type BlueprintViewerTheme = ThemeMode | 'auto'

interface BlueprintViewerPanelProps {
  code: string
}

interface BlueprintViewerConstructorOptions {
  container: HTMLElement
  theme?: BlueprintViewerTheme
  width?: number
  height?: number
}

interface BlueprintViewerInstance {
  loadBlueprint: (code: string) => void
  resetCamera: () => void
  setSize: (width: number, height: number) => void
  setTheme?: (theme: BlueprintViewerTheme) => void
  dispose: () => void
}

interface DspBlueprintEditorGlobal {
  BlueprintViewer: new (options: BlueprintViewerConstructorOptions) => BlueprintViewerInstance
}

interface MinimalVueI18nGlobal {
  t: (key: string) => string
}

interface MinimalVueI18n {
  createI18n: () => {
    global: MinimalVueI18nGlobal
  }
  useI18n: () => {
    t: MinimalVueI18nGlobal['t']
  }
}

interface BlueprintViewerWindow extends Window {
  DSPBlueprintEditor?: DspBlueprintEditorGlobal
  Vue?: typeof Vue
  VueI18n?: MinimalVueI18n
  exports?: {
    DSPBlueprintEditor?: DspBlueprintEditorGlobal
  }
  module?: {
    exports?: DspBlueprintEditorGlobal | {
      DSPBlueprintEditor?: DspBlueprintEditorGlobal
    }
  }
  define?: unknown
}

declare global {
  interface Window {
    DSPBlueprintEditor?: DspBlueprintEditorGlobal
  }
}

interface AppThemeSnapshot {
  readonly rootThemeMode: ThemeMode | null
  readonly blueprintEditorStoredTheme: string | null
}

const props = defineProps<BlueprintViewerPanelProps>()
const {themeMode, isThemeReady} = useThemeMode()

// =============== 蓝图模型预览状态 ===============
// 说明：在当前资源详情抽屉内挂载 BlueprintViewer，保持预览区域和页面其它 UI 同源协作。
// 边界：只调用库暴露的纯预览 API，不嵌入独立页面，也不启用完整编辑器的编辑和保存能力。
const containerRef = ref<HTMLElement | null>(null)
const viewer = ref<BlueprintViewerInstance | null>(null)
const isLoadingViewer = ref(false)
const viewerError = ref('')
let resizeObserver: ResizeObserver | null = null
let pendingLoadFrame = 0
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

const withAsyncAppThemeIsolation = async <T>(operation: () => Promise<T>): Promise<T> => {
  const snapshot = captureAppThemeSnapshot()
  try {
    return await operation()
  } finally {
    restoreAppThemeSnapshot(snapshot)
  }
}

// =============== 预览器脚本加载 ===============
// 说明：按需加载 dsp-blueprint-editor 的 UMD 资源，复用库对外暴露的 BlueprintViewer 构造函数。
// 边界：脚本只加载一次；兼容 Wails/WebView 注入 exports 或 module 导致 UMD 未挂到 window 的情况。
const ensureBlueprintEditor = async (): Promise<DspBlueprintEditorGlobal> => {
  const loadedEditor = resolveBlueprintEditorGlobal()
  if (loadedEditor) {
    cleanupBlueprintEditorGlobalStyles()
    document.documentElement.dataset.theme = themeMode.value
    document.documentElement.style.colorScheme = themeMode.value
    return loadedEditor
  }

  await withAsyncAppThemeIsolation(() => new Promise<void>((resolve, reject) => {
    const existingScript = document.querySelector<HTMLScriptElement>(
      `script[data-dsp-blueprint-editor="true"]`,
    )
    if (existingScript) {
      existingScript.remove()
    }

    const script = document.createElement('script')
    script.src = blueprintEditorScriptUrl
    script.async = true
    script.dataset.dspBlueprintEditor = 'true'
    const existingStyleNodes = new Set<HTMLStyleElement>(
      Array.from(document.head.querySelectorAll<HTMLStyleElement>('style')),
    )
    const maskedGlobals = prepareBlueprintEditorGlobals()
    script.addEventListener('load', () => {
      restoreBlueprintEditorGlobals(maskedGlobals)
      cleanupBlueprintEditorInjectedStyles(existingStyleNodes)
      document.documentElement.dataset.theme = themeMode.value
      document.documentElement.style.colorScheme = themeMode.value
      resolve()
    }, {once: true})
    script.addEventListener('error', () => {
      restoreBlueprintEditorGlobals(maskedGlobals)
      cleanupBlueprintEditorInjectedStyles(existingStyleNodes)
      document.documentElement.dataset.theme = themeMode.value
      document.documentElement.style.colorScheme = themeMode.value
      reject(new Error('蓝图预览脚本加载失败。'))
    }, {once: true})
    document.head.appendChild(script)
  }))

  const editor = resolveBlueprintEditorGlobal()
  if (!editor) {
    throw new Error('蓝图预览器未暴露 BlueprintViewer。')
  }

  window.DSPBlueprintEditor = editor
  return editor
}

const blueprintI18nFallback: MinimalVueI18n = {
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({
    t: (key: string) => key,
  }),
}

const resolveBlueprintEditorGlobal = (): DspBlueprintEditorGlobal | null => {
  const globalWindow = window as BlueprintViewerWindow
  const candidates = [
    globalWindow.DSPBlueprintEditor,
    globalWindow.exports?.DSPBlueprintEditor,
    globalWindow.module?.exports,
    (globalWindow.module?.exports as {DSPBlueprintEditor?: DspBlueprintEditorGlobal} | undefined)
      ?.DSPBlueprintEditor,
  ]

  return candidates.find((candidate): candidate is DspBlueprintEditorGlobal => (
    Boolean(candidate && 'BlueprintViewer' in candidate && candidate.BlueprintViewer)
  )) ?? null
}

const prepareBlueprintEditorGlobals = (): Partial<BlueprintViewerWindow> => {
  const globalWindow = window as BlueprintViewerWindow
  const snapshot: Partial<BlueprintViewerWindow> = {
    Vue: globalWindow.Vue,
    VueI18n: globalWindow.VueI18n,
    exports: globalWindow.exports,
    module: globalWindow.module,
    define: globalWindow.define,
  }

  globalWindow.Vue = Vue
  globalWindow.VueI18n = globalWindow.VueI18n ?? blueprintI18nFallback
  globalWindow.exports = undefined
  globalWindow.module = undefined
  globalWindow.define = undefined
  return snapshot
}

const restoreBlueprintEditorGlobals = (snapshot: Partial<BlueprintViewerWindow>): void => {
  const globalWindow = window as BlueprintViewerWindow
  globalWindow.Vue = snapshot.Vue
  globalWindow.VueI18n = snapshot.VueI18n
  globalWindow.exports = snapshot.exports
  globalWindow.module = snapshot.module
  globalWindow.define = snapshot.define
}

const cleanupBlueprintEditorInjectedStyles = (existingStyleNodes: Set<HTMLStyleElement>): void => {
  document.head.querySelectorAll<HTMLStyleElement>('style').forEach((styleNode) => {
    if (existingStyleNodes.has(styleNode)) {
      return
    }
    if (isBlueprintEditorGlobalStyle(styleNode.textContent ?? '')) {
      styleNode.remove()
    }
  })
}

const cleanupBlueprintEditorGlobalStyles = (): void => {
  cleanupBlueprintEditorInjectedStyles(new Set<HTMLStyleElement>())
}

const isBlueprintEditorGlobalStyle = (styleText: string): boolean => {
  const editorStyleSignatures = [
    'body{margin:0;background:var(--bg-body)',
    '.sidebar-wrap',
    '.editor-root{width:100%;height:100%}',
    '/* ── Theme Variables ── */',
    '.icon-select',
    '.building-params',
    '.fleet-grid',
    '.station-storage',
    '.bp-icon',
  ]

  return editorStyleSignatures.some((signature) => styleText.includes(signature))
}

// =============== 预览器生命周期 ===============
// 说明：初始化、加载蓝图和同步尺寸都集中在这里，详情切换时复用实例刷新模型。
// 边界：容器不可用或编码为空时不创建实例；组件卸载时释放 WebGL 资源。
const loadCurrentBlueprint = async (): Promise<void> => {
  const container = containerRef.value
  if (!container || !props.code) {
    return
  }
  if (!hasRenderableSize()) {
    scheduleBlueprintLoad()
    return
  }

  isLoadingViewer.value = true
  viewerError.value = ''

  try {
    const editor = await ensureBlueprintEditor()
    if (!viewer.value) {
      viewer.value = withAppThemeIsolation(() => new editor.BlueprintViewer({
        container,
        theme: themeMode.value,
      }))
    }

    withAppThemeIsolation(() => viewer.value?.loadBlueprint(props.code))
    resizeViewer()
    withAppThemeIsolation(() => viewer.value?.resetCamera())
  } catch (error) {
    viewerError.value = error instanceof Error ? error.message : String(error || '蓝图模型渲染失败。')
  } finally {
    isLoadingViewer.value = false
  }
}

const resizeViewer = (): void => {
  if (!containerRef.value || !viewer.value) {
    return
  }

  const {width, height} = containerRef.value.getBoundingClientRect()
  if (width > 0 && height > 0) {
    viewer.value.setSize(Math.floor(width), Math.floor(height))
  }
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

onMounted(async () => {
  await nextTick()
  resizeObserver = new ResizeObserver(() => resizeViewer())
  if (containerRef.value) {
    resizeObserver.observe(containerRef.value)
  }
  await loadCurrentBlueprint()
})

watch(() => props.code, () => {
  void loadCurrentBlueprint()
})

watch(themeMode, (mode) => {
  if (!viewer.value?.setTheme) {
    return
  }

  withAppThemeIsolation(() => viewer.value?.setTheme?.(mode))
})

onBeforeUnmount(() => {
  if (pendingLoadFrame) {
    window.cancelAnimationFrame(pendingLoadFrame)
    pendingLoadFrame = 0
  }
  resizeObserver?.disconnect()
  resizeObserver = null
  viewer.value?.dispose()
  viewer.value = null
})
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

      <div v-if="isLoadingViewer" class="blueprint-viewer-overlay">
        <n-spin size="small" />
        <span>正在加载模型</span>
      </div>

      <div v-if="viewerError" class="blueprint-viewer-overlay blueprint-viewer-overlay-error">
        <strong>模型渲染失败</strong>
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
  background: #0d1117;
}

.blueprint-viewer-canvas {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: #0d1117;
}

.blueprint-viewer-mount {
  width: 100%;
  height: 100%;
}

.blueprint-viewer-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: rgb(13 17 23 / 72%);
  color: #d6dde7;
  font-size: 0.8125rem;
  pointer-events: none;
}

.blueprint-viewer-overlay-error {
  flex-direction: column;
  padding: 24px;
  text-align: center;

  strong {
    color: #fca5a5;
    font-weight: 650;
  }

  span {
    max-width: 36rem;
    color: #cbd5e1;
    line-height: 1.6;
  }
}
</style>
