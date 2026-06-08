<script setup lang="ts">
import {
  AlertCircle,
  BarChart3,
  Database,
  FileText,
  Network,
  Play,
  RefreshCw,
  Server,
  ShieldCheck,
  Square,
  Zap,
} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {LineChart} from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
} from 'echarts/components'
import {use} from 'echarts/core'
import {CanvasRenderer} from 'echarts/renderers'
import type {EChartsOption} from 'echarts'
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref} from 'vue'
import VChart from 'vue-echarts'
import {useRouter} from 'vue-router'
import {modDependence, system} from '../../wailsjs/go/models'
import {ListLogs} from '../../wailsjs/go/system/LogService'
import {
  getHeadlessEnvironmentStatus,
  startHeadlessEnvironment,
  stopHeadlessEnvironment,
} from '../services/headless'

use([
  CanvasRenderer,
  GridComponent,
  LineChart,
  TooltipComponent,
])

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '开服',
  },
})

interface HeadlessMetricCard {
  label: string
  value: string
  detail: string
  icon: typeof Server
  tone: 'emerald' | 'blue' | 'indigo' | 'violet' | 'zinc' | 'amber'
}

interface HeadlessEventItem {
  time: string
  text: string
  tone: 'emerald' | 'blue' | 'amber' | 'red'
}

const message = useMessage()
const router = useRouter()
const status = ref<modDependence.HeadlessEnvironmentStatus | null>(null)
const recentLogs = ref<system.LogDTO[]>([])
const isLoading = ref(false)
const isStarting = ref(false)
const isStopping = ref(false)
const isRecentLogLoading = ref(false)
const loadError = ref('')
const recentLogError = ref('')
let refreshTimer: number | undefined

const readyDependencies = computed(() => status.value?.dependencies.filter((dependency) => dependency.installed).length ?? 0)
const totalDependencies = computed(() => status.value?.dependencies.length ?? 0)
const networkSamples = computed(() => status.value?.network?.samples || [])
const networkUploadMBps = computed(() => status.value?.network?.uploadMBps || 0)
const networkDownloadMBps = computed(() => status.value?.network?.downloadMBps || 0)
const networkUploadPeakMBps = computed(() => status.value?.network?.uploadPeakMBps || 0)
const networkDownloadPeakMBps = computed(() => status.value?.network?.downloadPeakMBps || 0)
const networkWindowSeconds = computed(() => status.value?.network?.sampleWindowSeconds || 0)
const networkValid = computed(() => Boolean(status.value?.network?.valid))
const stripBars = computed(() => {
  const samples = networkSamples.value.slice(-10)
  if (!samples.length) {
    return [4, 4, 4, 4, 4, 4, 4, 4, 4, 4]
  }

  const maxValue = Math.max(...samples.map((sample) => Math.max(sample.uploadMBps, sample.downloadMBps)), 0.01)
  return samples.map((sample) => Math.max(4, Math.round((Math.max(sample.uploadMBps, sample.downloadMBps) / maxValue) * 44)))
})
const statusLabel = computed(() => {
  if (status.value?.processRunning) {
    return '运行中'
  }
  if (status.value?.ready) {
    return '已就绪'
  }

  return '待准备'
})
const headline = computed(() => {
  if (status.value?.processRunning) {
    return '一键开服实例已启动'
  }
  if (status.value?.ready) {
    return '无头服务器环境已就绪'
  }
  if (status.value) {
    return '无头服务器环境待补齐'
  }

  return '正在检测开服环境'
})
const heroDescription = computed(() => {
  if (!status.value) {
    return '正在读取游戏目录、BepInEx、Nebula 依赖与端口状态。'
  }
  if (status.value.processRunning && status.value.portListening) {
    return `DSPGAME.exe 正在通过 :${status.value.hostPort} 对外监听。`
  }
  if (status.value.ready) {
    return `可通过 :${status.value.hostPort} 启动 Nebula 无头服务器。`
  }

  return status.value.issues?.[0] || '仍有前置项需要处理。'
})
const heroToneClass = computed(() => {
  if (status.value?.processRunning || status.value?.ready) {
    return 'border-emerald-200 bg-emerald-50 text-emerald-600'
  }
  if (status.value) {
    return 'border-amber-200 bg-amber-50 text-amber-600'
  }

  return 'border-blue-200 bg-blue-50 text-blue-600'
})
const statusPillClass = computed(() => {
  if (status.value?.processRunning || status.value?.ready) {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700'
  }
  if (status.value) {
    return 'border-amber-200 bg-amber-50 text-amber-700'
  }

  return 'border-blue-200 bg-blue-50 text-blue-700'
})
const headlessPrerequisiteBlocked = computed(() => {
  const current = status.value
  if (!current) {
    return true
  }
  if (current.processRunning) {
    return false
  }

  return !current.gameDir || !current.bepInExStatus.installed || !current.nebulaReady
})
const headlessGateTitle = computed(() => {
  const current = status.value
  if (!current && isLoading.value) {
    return '正在检测开服前置条件'
  }
  if (!current) {
    return '无法确认开服环境'
  }
  if (!current.gameDir) {
    return '无法开启开服'
  }
  if (!current.bepInExStatus.installed) {
    return '需要优先安装 BepInEx'
  }
  if (!current.nebulaReady) {
    return '需要安装 Nebula 开服依赖'
  }

  return '开服前置条件未完成'
})
const headlessGateDescription = computed(() => {
  const current = status.value
  if (!current && isLoading.value) {
    return '正在确认游戏路径、BepInEx 前置框架与 Nebula 依赖状态。'
  }
  if (!current) {
    return loadError.value || '暂时没有拿到开服环境检测结果，请重新检测。'
  }
  if (!current.gameDir) {
    return '未检测到 DSPGAME.exe，请先在设置页选择《戴森球计划》游戏目录。'
  }
  if (!current.bepInExStatus.installed) {
    return '开服前需要先在当前游戏目录安装 BepInEx 前置框架。'
  }
  if (!current.nebulaReady) {
    return `Nebula 开服依赖未安装完整，当前已就绪 ${readyDependencies.value}/${totalDependencies.value || 7}。`
  }

  return current.issues?.[0] || '仍有开服前置项需要处理。'
})
const headlessGatePrimaryText = computed(() => {
  const current = status.value
  if (!current || !current.gameDir) {
    return current ? '前往设置' : '重新检测'
  }
  if (!current.bepInExStatus.installed || !current.nebulaReady) {
    return '前往 Mod'
  }

  return '重新检测'
})
const metricCards = computed<HeadlessMetricCard[]>(() => {
  const current = status.value
  return [
    {
      label: '服务进程',
      value: current?.processRunning ? '运行中' : '未运行',
      detail: current?.processRunning ? `PID ${current.processId || '-'}` : '等待启动',
      icon: Server,
      tone: current?.processRunning ? 'emerald' : 'zinc',
    },
    {
      label: '监听端口',
      value: current ? `:${current.hostPort || 8469}` : ':8469',
      detail: current?.portListening ? 'TCP / UDP' : '未监听',
      icon: Network,
      tone: current?.portListening ? 'emerald' : 'blue',
    },
    {
      label: '前置框架',
      value: current?.bepInExStatus.installed ? '已就绪' : '缺失',
      detail: current?.bepInExStatus.installedVersion ? `BepInEx ${current.bepInExStatus.installedVersion}` : '需要 BepInEx',
      icon: ShieldCheck,
      tone: current?.bepInExStatus.installed ? 'indigo' : 'amber',
    },
    {
      label: 'Nebula 依赖',
      value: `${readyDependencies.value}/${totalDependencies.value || 7}`,
      detail: current?.nebulaReady ? '依赖完整' : '等待安装',
      icon: Database,
      tone: current?.nebulaReady ? 'emerald' : 'violet',
    },
  ]
})
const chartOption = computed<EChartsOption>(() => {
  const samples = networkSamples.value
  const labels = samples.length ? samples.map((sample) => sample.label) : ['--:--:--']
  const uploadData = samples.length ? samples.map((sample) => Number(sample.uploadMBps.toFixed(3))) : [0]
  const downloadData = samples.length ? samples.map((sample) => Number(sample.downloadMBps.toFixed(3))) : [0]
  const maxValue = Math.max(...uploadData, ...downloadData, 0.1)

  return {
    animation: true,
    animationDuration: 700,
    animationEasing: 'cubicOut',
    grid: {
      left: 16,
      right: 16,
      top: 20,
      bottom: 10,
      containLabel: false,
    },
    tooltip: {
      trigger: 'axis',
      borderWidth: 1,
      borderColor: '#e4e4e7',
      backgroundColor: '#ffffff',
      textStyle: {
        color: '#334155',
        fontSize: 12,
      },
      valueFormatter: (value: unknown) => {
        const normalizedValue = Array.isArray(value) ? value[0] : value
        const numericValue = Number(normalizedValue || 0)

        return `${numericValue.toFixed(1)} MB/s`
      },
    },
    xAxis: {
      type: 'category',
      data: labels,
      boundaryGap: false,
      axisLine: {show: false},
      axisTick: {show: false},
      axisLabel: {show: false},
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: Math.max(0.1, Number((maxValue * 1.2).toFixed(2))),
      splitNumber: 5,
      axisLine: {show: false},
      axisTick: {show: false},
      axisLabel: {show: false},
      splitLine: {
        lineStyle: {
          color: '#eef2f6',
          width: 1,
        },
      },
    },
    series: [
      {
        name: '上行',
        type: 'line',
        data: uploadData,
        smooth: 0.32,
        symbol: 'none',
        showSymbol: false,
        lineStyle: {
          width: 2.7,
          color: '#2563eb',
          cap: 'round',
          join: 'round',
        },
        areaStyle: {
          opacity: 1,
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {offset: 0, color: 'rgba(37, 99, 235, 0.24)'},
              {offset: 0.58, color: 'rgba(37, 99, 235, 0.08)'},
              {offset: 1, color: 'rgba(37, 99, 235, 0)'},
            ],
          },
        },
        emphasis: {
          focus: 'series',
          lineStyle: {
            width: 3.2,
          },
        },
      },
      {
        name: '下行',
        type: 'line',
        data: downloadData,
        smooth: 0.32,
        symbol: 'none',
        showSymbol: false,
        lineStyle: {
          width: 2.7,
          color: '#10b981',
          cap: 'round',
          join: 'round',
        },
        areaStyle: {
          opacity: 1,
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {offset: 0, color: 'rgba(16, 185, 129, 0.22)'},
              {offset: 0.56, color: 'rgba(16, 185, 129, 0.07)'},
              {offset: 1, color: 'rgba(16, 185, 129, 0)'},
            ],
          },
        },
        emphasis: {
          focus: 'series',
          lineStyle: {
            width: 3.2,
          },
        },
      },
    ],
  }
})
const recentEvents = computed<HeadlessEventItem[]>(() => {
  return recentLogs.value.slice(0, 4).map((log) => ({
    time: formatEventTime(log.dateTime),
    text: log.error ? `${log.content || log.event}：${log.error}` : log.content || log.event,
    tone: getLogEventTone(log),
  }))
})

// =============== 环境状态读取 ===============
// 说明：从后端读取开服前置状态，并保持页面所有指标同源刷新。
// 边界：若 Wails 绑定未注入，会显示可读错误，提示重启开发服务或应用。
const refreshStatus = async (showFeedback = false): Promise<void> => {
  if (isLoading.value) {
    return
  }

  isLoading.value = true
  loadError.value = ''
  try {
    status.value = await getHeadlessEnvironmentStatus()
    await loadRecentHeadlessLogs()
    if (showFeedback) {
      message.success('开服环境状态已刷新')
    }
  } catch (error) {
    loadError.value = formatErrorMessage(error)
    if (showFeedback) {
      message.error(`开服环境检测失败：${loadError.value}`)
    }
  } finally {
    isLoading.value = false
  }
}

// =============== 最近事件日志 ===============
// 说明：从系统日志读取 modDependence/headless 审计记录，最近事件只展示真实后端日志。
// 边界：日志读取失败不影响服务状态刷新，页面保留可读错误占位。
const loadRecentHeadlessLogs = async (): Promise<void> => {
  if (isRecentLogLoading.value) {
    return
  }

  isRecentLogLoading.value = true
  recentLogError.value = ''
  try {
    const result = await ListLogs({
      namespace: 'modDependence',
      module: 'headless',
      event: '',
      level: 0,
      type: 0,
      page: 1,
      pageSize: 4,
    } as system.LogQuery)

    recentLogs.value = result.items ?? []
  } catch (error) {
    recentLogError.value = formatErrorMessage(error)
  } finally {
    isRecentLogLoading.value = false
  }
}

// =============== 自动刷新 ===============
// 说明：页面保持激活时定期读取后端真实状态，让网络速率曲线由真实采样自然累积。
// 边界：组件失活或卸载时清理定时器，避免后台重复请求。
const startAutoRefresh = (): void => {
  window.clearInterval(refreshTimer)
  refreshTimer = window.setInterval(() => {
    void refreshStatus()
  }, 5000)
}

const stopAutoRefresh = (): void => {
  window.clearInterval(refreshTimer)
  refreshTimer = undefined
}

// =============== 一键启动动作 ===============
// 说明：由四宫格“启动”入口触发，后端会先补基础环境，再在依赖完整时启动无头进程。
// 边界：Nebula 依赖缺失时只刷新阻塞状态，不会强行启动游戏。
const startEnvironment = async (): Promise<void> => {
  if (isStarting.value) {
    return
  }
  if (headlessPrerequisiteBlocked.value) {
    message.warning(headlessGateDescription.value)
    return
  }

  isStarting.value = true
  loadError.value = ''
  try {
    const result = await startHeadlessEnvironment()
    status.value = result.status
    await loadRecentHeadlessLogs()
    if (result.started) {
      message.success(result.message || '无头服务器启动命令已执行')
      return
    }
    message.warning(result.message || '开服环境未就绪，暂不启动')
  } catch (error) {
    loadError.value = formatErrorMessage(error)
    message.error(`启动失败：${loadError.value}`)
  } finally {
    isStarting.value = false
  }
}

// =============== 一键停止动作 ===============
// 说明：由四宫格“停止”入口触发，后端只会停止确认属于无头服务器的 DSPGAME.exe 进程。
// 边界：未找到可安全停止的进程时只刷新状态并提示，不会误关普通游戏进程。
const stopEnvironment = async (): Promise<void> => {
  if (isStopping.value) {
    return
  }

  isStopping.value = true
  loadError.value = ''
  try {
    const result = await stopHeadlessEnvironment()
    status.value = result.status
    await loadRecentHeadlessLogs()
    if (result.stopped) {
      message.success(result.message || '无头服务器已停止')
      return
    }
    message.warning(result.message || '未找到正在运行的无头服务器')
  } catch (error) {
    loadError.value = formatErrorMessage(error)
    message.error(`停止失败：${loadError.value}`)
  } finally {
    isStopping.value = false
  }
}

const handleHeadlessGatePrimaryAction = (): void => {
  const current = status.value
  if (!current || (!current.gameDir && current)) {
    if (current) {
      router.push('/settings')
      return
    }

    void refreshStatus(true)
    return
  }
  if (!current.bepInExStatus.installed || !current.nebulaReady) {
    router.push('/mod')
    return
  }

  void refreshStatus(true)
}

// =============== 交互与格式化 ===============
// 说明：处理卡片色彩、事件状态和错误消息等展示辅助。
const metricToneClass = (tone: HeadlessMetricCard['tone']): string => {
  const toneMap: Record<HeadlessMetricCard['tone'], string> = {
    emerald: 'border-emerald-100 bg-emerald-50 text-emerald-600',
    amber: 'border-amber-100 bg-amber-50 text-amber-600',
    blue: 'border-blue-100 bg-blue-50 text-blue-600',
    indigo: 'border-indigo-100 bg-indigo-50 text-indigo-600',
    violet: 'border-violet-100 bg-violet-50 text-violet-600',
    zinc: 'border-zinc-200 bg-zinc-50 text-zinc-500',
  }

  return toneMap[tone]
}

const eventDotClass = (tone: HeadlessEventItem['tone']): string => {
  const toneMap: Record<HeadlessEventItem['tone'], string> = {
    emerald: 'bg-emerald-500',
    blue: 'bg-blue-600',
    amber: 'bg-amber-500',
    red: 'bg-red-500',
  }

  return toneMap[tone]
}

const getLogEventTone = (log: system.LogDTO): HeadlessEventItem['tone'] => {
  if (log.level >= 40) {
    return 'red'
  }
  if (log.level >= 30) {
    return 'amber'
  }
  if (log.event === 'start') {
    return 'emerald'
  }
  if (log.event === 'processOnline' || log.event === 'portOpen') {
    return 'emerald'
  }
  if (log.event === 'stop' || log.event === 'processOffline' || log.event === 'portClosed') {
    return 'red'
  }

  return 'blue'
}

const formatEventTime = (dateTime: string): string => {
  if (!dateTime) {
    return '--:--'
  }

  const date = new Date(dateTime)
  if (Number.isNaN(date.getTime())) {
    return '--:--'
  }

  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

const formatMBps = (value: number): string => {
  if (!Number.isFinite(value) || value <= 0) {
    return '0 KB/s'
  }
  if (value < 1) {
    return `${Math.round(value * 1024)} KB/s`
  }

  return `${value.toFixed(1)} MB/s`
}

const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}

onMounted(() => {
  void refreshStatus()
  startAutoRefresh()
})
onActivated(() => {
  void refreshStatus()
  startAutoRefresh()
})
onDeactivated(() => {
  stopAutoRefresh()
})
onBeforeUnmount(() => {
  stopAutoRefresh()
})
</script>

<template>
  <section class="relative mx-auto min-h-full w-full max-w-[1180px] px-8 py-6">
    <!-- ~ 开服状态总览 -->
    <!-- * 说明：按桌面程序宽度展示状态 Hero，高密度呈现实例状态、监听端口和四宫格操作 -->
    <!-- * 交互：启动按钮会先准备基础环境，再在状态就绪时启动无头进程；刷新只读状态，日志进入日志页 -->
    <!-- * 依赖：依赖 HeadlessService 状态与启动接口 -->
    <!-- * 边界：绑定未注入或游戏路径缺失时在错误条展示明确原因 -->
    <div class="h-[164px] rounded border border-zinc-200 bg-white">
      <div class="grid h-full grid-cols-[128px_minmax(0,1fr)_128px] items-center gap-6 px-8">
        <div class="relative grid h-28 w-28 place-items-center">
          <div class="absolute inset-0 rounded-full bg-emerald-50" :class="status?.ready ? 'opacity-100' : 'opacity-60'"></div>
          <div class="relative grid h-[60px] w-[60px] place-items-center rounded-md border" :class="heroToneClass">
            <Server :size="21" :stroke-width="1.8" />
          </div>
        </div>

        <div class="min-w-0">
          <div class="inline-flex h-6 items-center gap-2 rounded border px-3 text-[11px] font-semibold" :class="statusPillClass">
            <span class="h-1.5 w-1.5 rounded-full" :class="status?.ready || status?.processRunning ? 'bg-emerald-500' : 'bg-amber-500'"></span>
            {{ statusLabel }}
          </div>
          <h1 class="mt-2 truncate text-[22px] font-bold leading-[30px] text-zinc-950">{{ headline }}</h1>
          <p class="mt-1 truncate text-xs leading-[18px] text-zinc-500">{{ heroDescription }}</p>
          <p v-if="status?.gameDir" class="mt-1 truncate text-[11px] text-zinc-400">{{ status.gameDir }}</p>
        </div>

        <div class="grid h-32 w-32 grid-cols-2 overflow-hidden rounded border border-blue-200 bg-white shadow-[0_8px_18px_-12px_rgba(15,23,42,0.14)]">
          <button
            class="headless-hero-action"
            :class="status?.processRunning ? 'bg-emerald-600 text-white' : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100'"
            :disabled="isStarting || status?.processRunning || headlessPrerequisiteBlocked"
            @click="startEnvironment"
          >
            <RefreshCw v-if="isStarting" :size="17" :stroke-width="1.8" class="animate-spin" />
            <Play v-else :size="17" :stroke-width="1.8" />
            <span>启动</span>
          </button>
          <button
            class="headless-hero-action"
            :class="status?.processRunning ? 'bg-red-50 text-red-700 hover:bg-red-100' : 'bg-red-600 text-white'"
            :disabled="isStopping || !status?.processRunning"
            @click="stopEnvironment"
          >
            <RefreshCw v-if="isStopping" :size="17" :stroke-width="1.8" class="animate-spin" />
            <Square v-else :size="15" :stroke-width="2" />
            <span>停止</span>
          </button>
          <button class="headless-hero-action text-zinc-700" :disabled="isLoading" @click="() => refreshStatus(true)">
            <RefreshCw :size="17" :stroke-width="1.8" :class="isLoading ? 'animate-spin' : ''" />
            <span>刷新</span>
          </button>
          <button class="headless-hero-action text-zinc-700" @click="router.push('/log')">
            <FileText :size="17" :stroke-width="1.8" />
            <span>日志</span>
          </button>
        </div>
      </div>
    </div>

    <div v-if="loadError" class="mt-4 flex h-10 items-center gap-2 rounded border border-red-200 bg-red-50 px-4 text-sm text-red-700">
      <AlertCircle :size="16" :stroke-width="1.8" />
      <span class="truncate">{{ loadError }}</span>
    </div>

    <!-- ~ 核心指标 -->
    <!-- * 说明：四个指标卡等宽对齐，展示进程、端口、前置框架和 Nebula 依赖 -->
    <!-- * 交互：随刷新和启动动作同步 -->
    <!-- * 依赖：依赖 HeadlessEnvironmentStatus 聚合字段 -->
    <!-- * 边界：未加载时展示默认占位，保持布局稳定 -->
    <div class="mt-4 grid grid-cols-4 gap-4">
      <article v-for="metric in metricCards" :key="metric.label" class="h-[104px] overflow-hidden rounded border border-zinc-200 bg-white p-4">
        <div class="flex gap-3">
          <div class="grid h-8 w-8 shrink-0 place-items-center rounded-md border" :class="metricToneClass(metric.tone)">
            <component :is="metric.icon" :size="17" :stroke-width="1.8" />
          </div>
          <div class="min-w-0">
            <p class="text-[11px] font-medium leading-[14px] text-zinc-400">{{ metric.label }}</p>
            <p class="mt-1 truncate text-lg font-bold leading-6 text-zinc-950">{{ metric.value }}</p>
            <p class="mt-1 truncate text-[11px] leading-4 text-zinc-500">{{ metric.detail }}</p>
          </div>
        </div>
      </article>
    </div>

    <div class="mt-4 grid grid-cols-[minmax(0,1fr)_280px] gap-4">
      <!-- ~ 网络速率图表 -->
      <!-- * 说明：展示后端从系统网卡计数器计算出的真实网络速率 -->
      <!-- * 交互：刷新状态会追加真实采样点，ECharts 使用当前后端样本渲染折线图 -->
      <!-- * 依赖：依赖 HeadlessEnvironmentStatus.network 与 vue-echarts -->
      <!-- * 边界：首次采样没有差值时显示 0，读取失败时显示空态提示，不使用模拟数据 -->
      <div class="h-[390px] rounded border border-zinc-200 bg-white p-4">
        <div class="flex items-center justify-between gap-4">
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-semibold leading-5 text-zinc-950">网络速率</h2>
            <BarChart3 :size="17" :stroke-width="1.8" class="text-blue-600" />
          </div>
          <div class="flex h-7 items-end justify-end gap-0.5    px-2 py-1">
            <span
              v-for="(height, index) in stripBars"
              :key="index"
              class="w-[3px] rounded-sm"
              :class="index % 2 === 0 ? 'bg-blue-200' : 'bg-emerald-200'"
              :style="{height: `${Math.min(height, 18)}px`}"
            ></span>
          </div>
        </div>
        <div class="mt-2 flex flex-wrap items-center gap-x-6 gap-y-1 text-xs font-semibold leading-4">
          <span class="text-blue-700">上行 {{ formatMBps(networkUploadMBps) }}</span>
          <span class="text-emerald-600">下行 {{ formatMBps(networkDownloadMBps) }}</span>
<!--          <span class="text-sky-700">上行峰值 {{ formatMBps(networkUploadPeakMBps) }}</span>-->
<!--          <span class="text-teal-700">下行峰值 {{ formatMBps(networkDownloadPeakMBps) }}</span>-->
<!--          <span class="text-slate-500">采样窗口 {{ networkWindowSeconds }} s</span>-->
        </div>
        <div class="relative mt-2 h-[282px] w-full">
          <VChart class="h-full w-full" :option="chartOption" autoresize />
          <div v-if="networkSamples.length <= 1" class="pointer-events-none absolute inset-0 grid place-items-center text-xs text-zinc-400">
            等待下一次刷新生成真实速率曲线
          </div>
        </div>
      </div>

      <div class="grid gap-4">
        <!-- ~ 服务状态 -->
        <!-- * 说明：右侧服务状态卡片与网络速率面板成列对齐，展示真实进程与端口同步结论 -->
        <!-- * 交互：启动或刷新后状态同步刷新 -->
        <!-- * 依赖：依赖 status.ready、processRunning、portListening -->
        <!-- * 边界：未启动时端口未监听不作为准备失败的唯一条件 -->
        <div class="h-[186px] rounded border border-zinc-200 bg-white p-4">
          <h2 class="text-sm font-bold leading-5 text-zinc-950">服务状态</h2>
          <p class="mt-1 text-[11px] leading-4 text-slate-500">
            {{ status?.processRunning ? '运行中 · 可停止' : status?.ready ? '已就绪 · 待启动' : '待准备 · 缺依赖' }}
          </p>
          <div class="mt-3 flex h-[78px] items-center gap-4 rounded-md border px-5" :class="status?.processRunning ? 'border-emerald-100 bg-emerald-50' : status?.ready ? 'border-blue-100 bg-blue-50' : 'border-amber-100 bg-amber-50'">
            <div class="grid h-[54px] w-[54px] place-items-center rounded-full border-2 border-white/70" :class="status?.processRunning ? 'bg-emerald-100 text-emerald-600' : status?.ready ? 'bg-blue-100 text-blue-600' : 'bg-amber-100 text-amber-600'">
              <Zap :size="22" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm font-semibold" :class="status?.processRunning ? 'text-emerald-600' : status?.ready ? 'text-blue-600' : 'text-amber-600'">
                {{ status?.processRunning ? '心跳正常' : status?.ready ? '等待启动' : '等待就绪' }}
              </p>
              <p class="mt-1 text-[11px] leading-4 text-slate-500">
                {{ status?.portListening ? '服务进程持续响应' : '进程与端口状态同步' }}
              </p>
            </div>
          </div>
          <p class="mt-2 text-[11px] leading-4 text-slate-500">进程与端口状态同步</p>
        </div>

        <!-- ~ 最近事件 -->
        <!-- * 说明：展示系统日志中的开服审计事件，与服务状态卡片保持同列宽度 -->
        <!-- * 交互：启动、停止和刷新后同步读取最新日志 -->
        <!-- * 依赖：依赖 LogService.ListLogs 与 recentEvents -->
        <!-- * 边界：日志读取失败时展示错误，不再用前端模拟事件兜底 -->
        <div class="h-[188px] rounded border border-zinc-200 bg-white p-4">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold leading-5 text-zinc-950">最近事件</h2>
            <RefreshCw v-if="isRecentLogLoading" :size="14" :stroke-width="1.8" class="animate-spin text-zinc-400" />
          </div>
          <div v-if="recentLogError" class="mt-4 rounded border border-red-100 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700">
            {{ recentLogError }}
          </div>
          <div v-else-if="recentEvents.length === 0" class="mt-5 rounded border border-dashed border-zinc-200 py-6 text-center text-xs text-zinc-400">
            暂无开服日志
          </div>
          <div v-else class="mt-3 grid gap-3">
            <div v-for="event in recentEvents" :key="`${event.time}-${event.text}`" class="grid grid-cols-[46px_10px_minmax(0,1fr)] items-center gap-2 text-xs">
              <span class="text-[11px] leading-4 text-zinc-400">{{ event.time }}</span>
              <span class="h-[7px] w-[7px] rounded-full" :class="eventDotClass(event.tone)"></span>
              <span class="truncate text-zinc-600">{{ event.text }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ~ 开服前置条件遮罩 -->
    <!-- * 说明：当前置条件未满足时阻止开服操作，保持与 Mod 页一致的遮挡体验 -->
    <!-- * 交互：根据缺失项跳转设置页、Mod 页或重新检测 -->
    <!-- * 依赖：依赖 HeadlessService 聚合状态中的 gameDir、BepInEx 与 Nebula 依赖字段 -->
    <!-- * 边界：无头进程已经运行时不遮挡，避免异常状态下无法停止服务 -->
    <div v-if="headlessPrerequisiteBlocked" class="absolute inset-0 z-20 grid place-items-center bg-white/65 backdrop-blur-sm">
      <div class="max-w-md px-6 text-center">
        <h1 class="text-xl font-semibold text-zinc-950">{{ headlessGateTitle }}</h1>
        <p class="mt-3 text-sm leading-6 text-zinc-500">{{ headlessGateDescription }}</p>
        <div class="mt-6 flex justify-center gap-3">
          <n-button secondary :loading="isLoading" @click="() => refreshStatus(true)">重新检测</n-button>
          <n-button type="primary" @click="handleHeadlessGatePrimaryAction">{{ headlessGatePrimaryText }}</n-button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.headless-hero-action {
  display: grid;
  min-width: 0;
  place-items: center;
  border: 0;
  border-right: 1px solid #bfdbfe;
  border-bottom: 1px solid #bfdbfe;
  font-size: 11px;
  line-height: 14px;
  transition:
    background-color 0.16s ease,
    color 0.16s ease;
}

.headless-hero-action:nth-child(2n) {
  border-right: 0;
}

.headless-hero-action:nth-child(n + 3) {
  border-bottom: 0;
}

.headless-hero-action:disabled {
  cursor: not-allowed;
}
</style>
