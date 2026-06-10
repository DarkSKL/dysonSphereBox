<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '蓝图生成器',
  },
})

// =============== 蓝图生成器来源 ===============
// 说明：集中维护内嵌页面地址，方便后续切换到镜像站或本地托管版本。
const blueprintGeneratorUrl = 'https://sgwswordemperor.github.io/'

// =============== 内嵌视口尺寸 ===============
// 说明：外部蓝图工具是信息密度较高的桌面工具，过窄时保留最小工作宽度并交给外层滚动容器处理。
const embeddedViewportStyle = {
  minWidth: '1080px',
  minHeight: '680px',
}

type EmbeddedLoadStage = 'preloading' | 'embedding' | 'ready' | 'error'

// =============== 静态资源预加载 ===============
// 说明：父页面先解析目标站点声明的脚本与样式资源，并用 fetch 统计 pending 数量。
// 边界：该统计只覆盖首页声明的静态资源，不能代表外部页面内部运行时的异步任务完成状态。
const iframeUrl = ref('')
const loadStage = ref<EmbeddedLoadStage>('preloading')
const totalResourceCount = ref(0)
const finishedResourceCount = ref(0)
const pendingResourceCount = ref(0)
const failedResourceCount = ref(0)
const loadErrorMessage = ref('')
const iframeRenderKey = ref(0)

let preloadController: AbortController | null = null

const loadProgressPercentage = computed(() => {
  if (totalResourceCount.value === 0) {
    return loadStage.value === 'embedding' || loadStage.value === 'ready' ? 100 : 0
  }

  return Math.round((finishedResourceCount.value / totalResourceCount.value) * 100)
})

// =============== Naive UI 加载状态 ===============
// 说明：将资源预加载、内嵌页启动与失败状态映射为 Naive UI 进度组件的视觉语义。
// 边界：进度只覆盖父页面可统计的静态资源，不读取跨域 iframe 内部执行状态。
const loadProgressStatus = computed(() => {
  if (loadStage.value === 'error') {
    return 'error'
  }
  if (loadStage.value === 'embedding') {
    return 'success'
  }

  return 'info'
})

const loadStageTitle = computed(() => {
  if (loadStage.value === 'preloading') {
    return '正在预加载蓝图生成器资源'
  }
  if (loadStage.value === 'embedding') {
    return '正在启动蓝图生成器'
  }
  if (loadStage.value === 'error') {
    return '蓝图生成器加载失败'
  }

  return ''
})

const loadStageDescription = computed(() => {
  if (loadStage.value === 'preloading') {
    return `剩余 ${pendingResourceCount.value} / ${totalResourceCount.value} 个资源，已完成 ${finishedResourceCount.value} 个。`
  }
  if (loadStage.value === 'embedding') {
    if (failedResourceCount.value > 0) {
      return `有 ${failedResourceCount.value} 个资源预加载失败，正在交给内嵌页面继续尝试加载。`
    }

    return '静态资源已就绪，正在等待内嵌页面完成首屏加载。'
  }
  if (loadStage.value === 'error') {
    return loadErrorMessage.value || '请检查网络连接后重试。'
  }

  return ''
})

const resolveBlueprintResourceUrl = (resourcePath: string): string => {
  return new URL(resourcePath, blueprintGeneratorUrl).toString()
}

const collectBlueprintResources = (html: string): string[] => {
  const documentNode = new DOMParser().parseFromString(html, 'text/html')
  const resourcePaths = [
    ...Array.from(documentNode.querySelectorAll<HTMLScriptElement>('script[src]')).map((node) => node.src),
    ...Array.from(documentNode.querySelectorAll<HTMLLinkElement>('link[rel="modulepreload"][href]')).map((node) => node.href),
    ...Array.from(documentNode.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"][href]')).map((node) => node.href),
  ]

  return Array.from(new Set(
    resourcePaths
      .filter((path) => path.trim().length > 0)
      .map(resolveBlueprintResourceUrl),
  ))
}

const preloadBlueprintResource = async (resourceUrl: string, signal: AbortSignal): Promise<void> => {
  try {
    const response = await fetch(resourceUrl, {
      cache: 'force-cache',
      signal,
    })
    if (!response.ok) {
      failedResourceCount.value += 1
    }
  } catch (error) {
    if ((error as Error).name !== 'AbortError') {
      failedResourceCount.value += 1
    }
  } finally {
    if (!signal.aborted) {
      finishedResourceCount.value += 1
      pendingResourceCount.value = Math.max(0, pendingResourceCount.value - 1)
    }
  }
}

const startEmbeddedPage = (): void => {
  loadStage.value = 'embedding'
  iframeRenderKey.value += 1
  iframeUrl.value = blueprintGeneratorUrl
}

const preloadBlueprintGenerator = async (): Promise<void> => {
  preloadController?.abort()
  preloadController = new AbortController()

  iframeUrl.value = ''
  loadStage.value = 'preloading'
  loadErrorMessage.value = ''
  totalResourceCount.value = 0
  finishedResourceCount.value = 0
  pendingResourceCount.value = 0
  failedResourceCount.value = 0

  try {
    const response = await fetch(blueprintGeneratorUrl, {
      cache: 'no-cache',
      signal: preloadController.signal,
    })
    if (!response.ok) {
      throw new Error(`入口页面响应异常：${response.status}`)
    }

    const resources = collectBlueprintResources(await response.text())
    totalResourceCount.value = resources.length
    pendingResourceCount.value = resources.length

    if (resources.length > 0) {
      await Promise.all(resources.map((resourceUrl) => (
        preloadBlueprintResource(resourceUrl, preloadController!.signal)
      )))
    }

    if (!preloadController.signal.aborted) {
      startEmbeddedPage()
    }
  } catch (error) {
    if ((error as Error).name === 'AbortError') {
      return
    }

    loadStage.value = 'error'
    loadErrorMessage.value = error instanceof Error ? error.message : '未知加载错误'
  }
}

const markEmbeddedPageLoaded = (): void => {
  loadStage.value = 'ready'
}

onMounted(() => {
  void preloadBlueprintGenerator()
})

onBeforeUnmount(() => {
  preloadController?.abort()
})
</script>

<template>
  <section class="flex h-full min-h-[calc(100vh-4rem)] flex-col bg-zinc-100">
    <!-- ~ 蓝图生成器内嵌页 -->
    <!-- * 说明：直接承载外部蓝图生成器页面，保留原站点完整交互能力 -->
    <!-- * 交互：用户在嵌入页面内完成蓝图编辑与生成操作；窗口较窄时外层提供横向滚动 -->
    <!-- * 依赖：依赖 sgwswordemperor.github.io 可访问，并由 WebView 承载 iframe -->
    <!-- * 边界：外部站点不可访问或主动拒绝嵌入时，浏览器会在 iframe 内展示加载失败状态；小窗口下不压缩外部工具布局 -->
    <div class="relative min-h-0 flex-1 overflow-auto">
      <iframe
        v-if="iframeUrl"
        :key="iframeRenderKey"
        class="block h-full w-full border-0 bg-white"
        :style="embeddedViewportStyle"
        :src="iframeUrl"
        title="蓝图生成器"
        allow="clipboard-read; clipboard-write"
        loading="eager"
        referrerpolicy="no-referrer-when-downgrade"
        sandbox="allow-downloads allow-forms allow-popups allow-same-origin allow-scripts"
        @load="markEmbeddedPageLoaded"
      ></iframe>

      <!-- ~ 加载状态遮罩 -->
      <!-- * 说明：展示父页面可观测到的静态资源预加载与 iframe 首屏加载状态 -->
      <!-- * 交互：加载失败时允许用户重新触发预加载和内嵌页面启动 -->
      <!-- * 依赖：依赖 fetch、DOMParser 与 iframe load 事件 -->
      <!-- * 边界：跨域页面内部脚本执行进度不可读取，因此只呈现父页面可统计的资源等待状态 -->
      <div
        v-if="loadStage !== 'ready'"
        class="absolute inset-0 z-10 grid place-items-center bg-white/90 px-8 backdrop-blur-sm"
      >
        <div class="grid w-full max-w-xl grid-cols-[156px_minmax(0,1fr)] items-center gap-6 p-6 max-sm:grid-cols-1 max-sm:justify-items-center max-sm:text-center">
          <div class="grid h-36 w-36 shrink-0 place-items-center">
            <n-progress
              type="circle"
              :percentage="loadProgressPercentage"
              :status="loadProgressStatus"
              :stroke-width="10"
            />
          </div>

          <div class="min-w-0">
            <div class="flex items-center gap-3 max-sm:justify-center">
              <n-spin v-if="loadStage !== 'error'" size="small" />
              <p class="text-base font-semibold text-zinc-950">{{ loadStageTitle }}</p>
            </div>
            <p class="mt-2 text-sm leading-6 text-zinc-500">{{ loadStageDescription }}</p>

            <div v-if="loadStage === 'preloading'" class="mt-5 grid grid-cols-3 gap-2 text-xs text-zinc-500">
              <span class="border border-zinc-200 bg-zinc-50 px-3 py-2">资源 {{ totalResourceCount }}</span>
              <span class="border border-zinc-200 bg-zinc-50 px-3 py-2">完成 {{ finishedResourceCount }}</span>
              <span class="border border-zinc-200 bg-zinc-50 px-3 py-2">等待 {{ pendingResourceCount }}</span>
            </div>

            <div v-if="loadStage === 'error'" class="mt-5">
              <n-button type="primary" @click="preloadBlueprintGenerator">重新加载</n-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
