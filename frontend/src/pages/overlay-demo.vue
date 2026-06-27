<script setup lang="ts">
import {MonitorUp, Play, RefreshCw, Square, Target} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {
  FindBrowserWindow,
  GetOverlayStatus,
  ListBrowserWindowCandidates,
  StartOverlay,
  StopOverlay,
} from '../../wailsjs/go/overlay/OverlayService'
import {overlay} from '../../wailsjs/go/models'

definePage({
  meta: {
    layout: 'main',
    title: '悬浮窗',
  },
})

const message = useMessage()
const overlayStatus = ref<overlay.OverlayProcessStatusDTO | null>(null)
const browserWindow = ref<overlay.WindowSnapshotDTO | null>(null)
const browserCandidates = ref<overlay.WindowCandidateDTO[]>([])
const isStarting = ref(false)
const isStopping = ref(false)
const isRefreshing = ref(false)
let refreshTimer: number | undefined

const statusText = computed(() => {
  if (overlayStatus.value?.running) {
    return `运行中 · PID ${overlayStatus.value.pid}`
  }
  return '未启动'
})

const browserStatusText = computed(() => {
  if (!browserWindow.value?.found) {
    return '浏览器未处于前台'
  }
  return `${browserWindow.value.processName} · PID ${browserWindow.value.pid}`
})

// =============== 原型状态刷新 ===============
// 说明：同时刷新附进程生命周期和浏览器窗口检测结果。
// 边界：刷新失败时只展示错误消息，不影响用户继续启停附进程。
const refreshState = async (silent = false): Promise<void> => {
  isRefreshing.value = true
  try {
    const [nextStatus, nextWindow, nextCandidates] = await Promise.all([
      GetOverlayStatus(),
      FindBrowserWindow(),
      ListBrowserWindowCandidates(),
    ])
    overlayStatus.value = nextStatus
    browserWindow.value = nextWindow
    browserCandidates.value = nextCandidates
  } catch (error) {
    if (!silent) {
      message.error(`刷新悬浮窗状态失败：${error instanceof Error ? error.message : String(error || '未知错误')}`)
    }
  } finally {
    isRefreshing.value = false
  }
}

// =============== 附进程控制 ===============
// 说明：主进程通过 Wails 服务启动或停止同一个 exe 的 --overlay 模式。
// 边界：重复启动由后端复用现有进程，重复停止返回未运行状态。
const startOverlay = async (): Promise<void> => {
  isStarting.value = true
  try {
    overlayStatus.value = await StartOverlay()
    message.success('已启动悬浮窗附进程')
    await refreshState(true)
  } catch (error) {
    message.error(`启动悬浮窗失败：${error instanceof Error ? error.message : String(error || '未知错误')}`)
  } finally {
    isStarting.value = false
  }
}

const stopOverlay = async (): Promise<void> => {
  isStopping.value = true
  try {
    overlayStatus.value = await StopOverlay()
    message.success('已停止悬浮窗附进程')
    await refreshState(true)
  } catch (error) {
    message.error(`停止悬浮窗失败：${error instanceof Error ? error.message : String(error || '未知错误')}`)
  } finally {
    isStopping.value = false
  }
}

onMounted(() => {
  void refreshState(true)
  refreshTimer = window.setInterval(() => {
    void refreshState(true)
  }, 1200)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>

<template>
  <section class="mx-auto grid max-w-6xl gap-5 px-8 py-8">
    <!-- ~ 悬浮窗原型说明 -->
    <!-- * 说明：展示当前第一版案例的目标和使用方式 -->
    <!-- * 交互：用户打开浏览器后可启动附进程观察透明 Overlay 是否贴附 -->
    <!-- * 依赖：依赖 OverlayService 的进程管理和浏览器窗口查找能力 -->
    <!-- * 边界：第一版只监听浏览器进程，后续再替换为 DSPGAME.exe -->
    <section class="rounded border border-zinc-200 bg-white p-5">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex items-center gap-3">
            <span class="grid h-10 w-10 place-items-center rounded border border-blue-100 bg-blue-50 text-blue-600">
              <MonitorUp :size="20" :stroke-width="2" />
            </span>
            <div>
              <h1 class="text-lg font-semibold text-zinc-950">游戏悬浮窗第一版案例</h1>
              <p class="mt-1 text-sm text-zinc-500">当前先监听 Chrome / Edge / Firefox 等浏览器窗口，后续替换为戴森球计划进程。</p>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <n-button :loading="isRefreshing" @click="refreshState(false)">
            <template #icon>
              <RefreshCw :size="16" :stroke-width="1.9" />
            </template>
            刷新
          </n-button>
          <n-button type="primary" :loading="isStarting" :disabled="overlayStatus?.running" @click="startOverlay">
            <template #icon>
              <Play :size="16" :stroke-width="1.9" />
            </template>
            启动悬浮窗
          </n-button>
          <n-button :loading="isStopping" :disabled="!overlayStatus?.running" @click="stopOverlay">
            <template #icon>
              <Square :size="15" :stroke-width="1.9" />
            </template>
            停止
          </n-button>
        </div>
      </div>
    </section>

    <div class="grid gap-5 lg:grid-cols-2">
      <!-- ~ 附进程状态 -->
      <!-- * 说明：展示主进程管理的 Overlay 附进程 PID 和运行状态 -->
      <!-- * 交互：状态由启动、停止和轮询刷新更新 -->
      <!-- * 依赖：依赖 GetOverlayStatus 返回值 -->
      <!-- * 边界：该状态只覆盖当前主进程启动的附进程 -->
      <section class="rounded border border-zinc-200 bg-white p-5">
        <div class="flex items-center gap-3">
          <span class="grid h-9 w-9 place-items-center rounded border border-emerald-100 bg-emerald-50 text-emerald-600">
            <MonitorUp :size="18" :stroke-width="2" />
          </span>
          <div>
            <h2 class="text-sm font-semibold text-zinc-950">Overlay 附进程</h2>
            <p class="mt-1 text-xs text-zinc-500">{{ statusText }}</p>
          </div>
        </div>
        <p class="mt-4 text-sm leading-6 text-zinc-600">
          启动后会运行同一个 <span class="font-mono text-xs">DysonSphereBox.exe --overlay</span>，加载独立透明窗口和 <span class="font-mono text-xs">/overlay</span> 前端壳。
        </p>
      </section>

      <!-- ~ 浏览器监听状态 -->
      <!-- * 说明：展示当前命中的浏览器窗口，用于验证后续游戏窗口抓取流程 -->
      <!-- * 交互：用户切换前台浏览器后，轮询会自动更新命中窗口 -->
      <!-- * 依赖：依赖 FindBrowserWindow 枚举顶层窗口 -->
      <!-- * 边界：未打开浏览器时展示等待态，不影响附进程启动 -->
      <section class="rounded border border-zinc-200 bg-white p-5">
        <div class="flex items-center gap-3">
          <span class="grid h-9 w-9 place-items-center rounded border border-amber-100 bg-amber-50 text-amber-700">
            <Target :size="18" :stroke-width="2" />
          </span>
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-zinc-950">浏览器窗口监听</h2>
            <p class="mt-1 truncate text-xs text-zinc-500">{{ browserStatusText }}</p>
          </div>
        </div>
        <dl class="mt-4 grid gap-2 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="text-zinc-500">标题</dt>
            <dd class="truncate text-right text-zinc-800">{{ browserWindow?.title || '等待浏览器窗口' }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-zinc-500">坐标</dt>
            <dd class="text-right text-zinc-800">
              <template v-if="browserWindow?.found">
                {{ browserWindow.rect.left }}, {{ browserWindow.rect.top }} · {{ browserWindow.rect.width }} x {{ browserWindow.rect.height }}
              </template>
              <template v-else>--</template>
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-zinc-500">吸附</dt>
            <dd class="text-right text-zinc-800">
              <template v-if="browserWindow?.attachedRect?.width">
                {{ browserWindow.attachedRect.left }}, {{ browserWindow.attachedRect.top }} · {{ browserWindow.attachedRect.width }} x {{ browserWindow.attachedRect.height }}
              </template>
              <template v-else>--</template>
            </dd>
          </div>
        </dl>
      </section>
    </div>

    <!-- ~ 候选窗口诊断 -->
    <!-- * 说明：列出后端枚举到的浏览器顶层窗口，帮助判断吸附偏移是否来自错误 HWND -->
    <!-- * 交互：随刷新按钮和轮询自动更新，得分最高的候选会被 Overlay 优先选择 -->
    <!-- * 依赖：依赖 ListBrowserWindowCandidates 返回的窗口快照和评分 -->
    <!-- * 边界：列表为空时说明当前没有满足过滤条件的浏览器窗口 -->
    <section class="rounded border border-zinc-200 bg-white p-5">
      <div class="flex items-center justify-between gap-4">
        <div>
          <h2 class="text-sm font-semibold text-zinc-950">候选窗口诊断</h2>
          <p class="mt-1 text-xs text-zinc-500">得分最高的窗口会作为当前吸附目标，用来排查 HWND 选错或坐标异常。</p>
        </div>
        <span class="text-xs text-zinc-500">{{ browserCandidates.length }} 个候选</span>
      </div>
      <div class="mt-4 overflow-hidden rounded border border-zinc-100">
        <div v-if="!browserCandidates.length" class="px-4 py-5 text-sm text-zinc-500">
          未找到可贴附的浏览器窗口。
        </div>
        <div
          v-for="(candidate, index) in browserCandidates"
          :key="`${candidate.snapshot.hwnd}-${candidate.score}`"
          class="grid gap-2 border-t border-zinc-100 px-4 py-3 text-sm first:border-t-0 lg:grid-cols-[68px_minmax(0,1fr)_220px]"
        >
          <div class="font-mono text-xs text-zinc-500">#{{ index + 1 }}</div>
          <div class="min-w-0">
            <div class="truncate font-medium text-zinc-900">
              {{ candidate.snapshot.title || '无标题窗口' }}
            </div>
            <div class="mt-1 truncate text-xs text-zinc-500">
              HWND {{ candidate.snapshot.hwnd }} · {{ candidate.snapshot.processName }} · PID {{ candidate.snapshot.pid }} · Score {{ candidate.score }}
              <span v-if="candidate.snapshot.foreground" class="ml-2 text-emerald-600">前台</span>
            </div>
          </div>
          <div class="font-mono text-xs text-zinc-600">
            {{ candidate.snapshot.rect.left }}, {{ candidate.snapshot.rect.top }} · {{ candidate.snapshot.rect.width }} x {{ candidate.snapshot.rect.height }}
          </div>
        </div>
      </div>
    </section>
  </section>
</template>
