<script setup lang="ts">
import {AlertCircle, AlertTriangle, Bug, ClipboardCopy, Download, Info, ListFilter, RefreshCw, X} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import type {SelectOption} from 'naive-ui'
import {computed, onMounted, ref, watch} from 'vue'
import {system} from '../../wailsjs/go/models'
import {ListLogs} from '../../wailsjs/go/system/LogService'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '系统日志',
  },
})

enum LogLevel {
  Debug = 10,
  Info = 20,
  Warn = 30,
  Error = 40,
}

enum LogType {
  Operation = 1,
  Configuration = 2,
  Installation = 3,
  Security = 4,
}

interface LogLevelConfig {
  readonly label: string
  readonly textClass: string
  readonly badgeClass: string
  readonly dotClass: string
  readonly icon: unknown
}

interface LogTypeConfig {
  readonly label: string
  readonly textClass: string
}

const message = useMessage()
const logs = ref<system.LogDTO[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const isLoading = ref(false)
const isExporting = ref(false)
const loadError = ref('')
const showFilters = ref(false)
const selectedNamespace = ref('')
const selectedModule = ref('')
const selectedEvent = ref('')
const selectedLevel = ref(0)
const selectedType = ref(0)

let latestLoadTicket = 0

const levelConfigs: Record<number, LogLevelConfig> = {
  [LogLevel.Debug]: {
    label: 'Debug',
    textClass: 'text-zinc-500',
    badgeClass: 'bg-zinc-100 text-zinc-600',
    dotClass: 'bg-zinc-300',
    icon: Bug,
  },
  [LogLevel.Info]: {
    label: 'Info',
    textClass: 'text-blue-600',
    badgeClass: 'bg-blue-50 text-blue-700',
    dotClass: 'bg-blue-400',
    icon: Info,
  },
  [LogLevel.Warn]: {
    label: 'Warn',
    textClass: 'text-amber-600',
    badgeClass: 'bg-amber-50 text-amber-700',
    dotClass: 'bg-amber-400',
    icon: AlertTriangle,
  },
  [LogLevel.Error]: {
    label: 'Error',
    textClass: 'text-red-600',
    badgeClass: 'bg-red-50 text-red-700',
    dotClass: 'bg-red-400',
    icon: AlertCircle,
  },
}

const typeConfigs: Record<number, LogTypeConfig> = {
  [LogType.Operation]: {
    label: '操作',
    textClass: 'text-zinc-600',
  },
  [LogType.Configuration]: {
    label: '配置',
    textClass: 'text-violet-600',
  },
  [LogType.Installation]: {
    label: '安装',
    textClass: 'text-emerald-600',
  },
  [LogType.Security]: {
    label: '安全',
    textClass: 'text-orange-600',
  },
}

const commonNamespaces = ['system', 'modDependence', 'server']
const commonModules = ['configuration', 'environment', 'installer', 'update', 'process', 'port']
const pageSizeOptions = [20, 50, 100, 200]
const maxExportPageSize = 200
const levelOptions: SelectOption[] = [
  {label: '全部等级', value: 0},
  {label: 'Debug', value: LogLevel.Debug},
  {label: 'Info', value: LogLevel.Info},
  {label: 'Warn', value: LogLevel.Warn},
  {label: 'Error', value: LogLevel.Error},
]
const typeOptions: SelectOption[] = [
  {label: '全部类型', value: 0},
  {label: '操作', value: LogType.Operation},
  {label: '配置', value: LogType.Configuration},
  {label: '安装', value: LogType.Installation},
  {label: '安全', value: LogType.Security},
]

const namespaceOptions = computed<SelectOption[]>(() => {
  return buildTextOptions('所有命名空间', [
    ...commonNamespaces,
    ...logs.value.map((log) => log.namespace),
  ])
})
const moduleOptions = computed<SelectOption[]>(() => {
  return buildTextOptions('所有模块', [
    ...commonModules,
    ...logs.value.map((log) => log.module),
  ])
})
const activeFilterCount = computed(() => {
  return [
    selectedNamespace.value,
    selectedModule.value,
    selectedEvent.value.trim(),
    selectedLevel.value,
    selectedType.value,
  ].filter(Boolean).length
})
const hasActiveFilters = computed(() => activeFilterCount.value > 0)
const pageRangeText = computed(() => {
  if (total.value === 0) {
    return '0 条'
  }

  const start = (page.value - 1) * pageSize.value + 1
  const end = Math.min(page.value * pageSize.value, total.value)

  return `${start}-${end} / ${total.value} 条`
})
const exportableLogCount = computed(() => Number(total.value || logs.value.length || 0))

// =============== 日志查询 ===============
// 说明：根据命名空间、模块、事件、等级和类型从后端分页读取日志。
// 边界：查询失败时保留当前页面状态，并展示可重试的错误提示。
const loadLogs = async (): Promise<void> => {
  const ticket = latestLoadTicket + 1
  latestLoadTicket = ticket
  isLoading.value = true
  loadError.value = ''

  try {
    const result = await ListLogs({
      namespace: selectedNamespace.value,
      module: selectedModule.value,
      event: selectedEvent.value.trim(),
      level: selectedLevel.value,
      type: selectedType.value,
      page: page.value,
      pageSize: pageSize.value,
    } as system.LogQuery)

    if (ticket !== latestLoadTicket) {
      return
    }

    logs.value = result.items ?? []
    total.value = Number(result.total || 0)
    page.value = result.page || page.value
    pageSize.value = result.pageSize || pageSize.value
  } catch (error) {
    if (ticket !== latestLoadTicket) {
      return
    }

    loadError.value = formatErrorMessage(error)
    message.error(`读取系统日志失败：${loadError.value}`)
  } finally {
    if (ticket === latestLoadTicket) {
      isLoading.value = false
    }
  }
}

// =============== 筛选交互 ===============
// 说明：筛选条件变化后回到第一页，避免旧页码在新条件下落空。
// 边界：页码已经是第一页时直接刷新，减少无意义的分页状态跳转。
const resetPageAndLoad = (): void => {
  if (page.value !== 1) {
    page.value = 1
    return
  }

  void loadLogs()
}

const clearFilters = (): void => {
  selectedNamespace.value = ''
  selectedModule.value = ''
  selectedEvent.value = ''
  selectedLevel.value = 0
  selectedType.value = 0
}

const getLevelConfig = (level: number): LogLevelConfig => {
  return levelConfigs[level] ?? levelConfigs[LogLevel.Info]
}

const getTypeConfig = (type: number): LogTypeConfig => {
  return typeConfigs[type] ?? {
    label: `类型 ${type}`,
    textClass: 'text-zinc-600',
  }
}

const metadataEntries = (metadata?: Record<string, unknown>): Array<[string, unknown]> => {
  return Object.entries(metadata ?? {}).filter(([, value]) => {
    return value !== undefined && value !== null && String(value) !== ''
  })
}

const formatMetadataValue = (value: unknown): string => {
  if (typeof value === 'object' && value !== null) {
    return JSON.stringify(value)
  }

  return String(value)
}

// =============== 日志复制与导出 ===============
// 说明：把单条日志格式化为可读文本，供复制按钮和批量导出共用。
// 边界：metadata 为空时不输出扩展信息，避免复制内容里出现无意义空段落。
const formatLogText = (log: system.LogDTO): string => {
  const lines = [
    log.content || '未填写日志内容',
    '',
    `时间：${formatFullTime(log.dateTime)}`,
    `等级：${getLevelConfig(log.level).label}`,
    `类型：${getTypeConfig(log.type).label}`,
    `命名空间：${log.namespace || '-'}`,
    `模块：${log.module || '-'}`,
    `事件：${log.event || '-'}`,
  ]

  if (log.error) {
    lines.push('', `错误详情：${log.error}`)
  }

  const metadata = metadataEntries(log.metadata)
  if (metadata.length > 0) {
    lines.push('', '扩展信息')
    metadata.forEach(([key, value]) => {
      lines.push(`${key}：${formatMetadataValue(value)}`)
    })
  }

  return lines.join('\n')
}

const copyTextToClipboard = async (text: string): Promise<void> => {
  if (navigator.clipboard !== undefined) {
    await navigator.clipboard.writeText(text)
    return
  }

  const input = document.createElement('textarea')
  input.value = text
  input.setAttribute('readonly', 'true')
  input.style.position = 'fixed'
  input.style.left = '-9999px'
  input.style.top = '0'
  document.body.appendChild(input)
  input.select()

  try {
    const copied = document.execCommand('copy')
    if (!copied) {
      throw new Error('copy command failed')
    }
  } finally {
    document.body.removeChild(input)
  }
}

const copyLog = async (log: system.LogDTO): Promise<void> => {
  try {
    await copyTextToClipboard(formatLogText(log))
    message.success('日志信息已复制')
  } catch (error) {
    message.error(`复制日志失败：${formatErrorMessage(error)}`)
  }
}

const buildLogExportFileName = (): string => {
  const now = new Date()
  const timestamp = [
    now.getFullYear(),
    String(now.getMonth() + 1).padStart(2, '0'),
    String(now.getDate()).padStart(2, '0'),
    String(now.getHours()).padStart(2, '0'),
    String(now.getMinutes()).padStart(2, '0'),
    String(now.getSeconds()).padStart(2, '0'),
  ].join('')

  return `dyson-sphere-box-logs-${timestamp}.txt`
}

const downloadTextFile = (fileName: string, content: string): void => {
  const blob = new Blob([content], {type: 'text/plain;charset=utf-8'})
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

const exportLogs = async (): Promise<void> => {
  if (exportableLogCount.value === 0) {
    message.warning('当前没有可导出的日志')
    return
  }

  isExporting.value = true
  try {
    const exportedLogs: system.LogDTO[] = []
    let exportPage = 1
    let exportTotal = exportableLogCount.value

    while (exportedLogs.length < exportTotal) {
      const result = await ListLogs({
        namespace: selectedNamespace.value,
        module: selectedModule.value,
        event: selectedEvent.value.trim(),
        level: selectedLevel.value,
        type: selectedType.value,
        page: exportPage,
        pageSize: maxExportPageSize,
      } as system.LogQuery)
      const pageLogs = result.items ?? []
      exportTotal = Number(result.total || pageLogs.length || exportTotal)
      exportedLogs.push(...pageLogs)
      if (pageLogs.length < maxExportPageSize) {
        break
      }
      exportPage += 1
    }

    if (exportedLogs.length === 0) {
      message.warning('当前没有可导出的日志')
      return
    }

    const filterSummary = [
      selectedNamespace.value ? `命名空间=${selectedNamespace.value}` : '',
      selectedModule.value ? `模块=${selectedModule.value}` : '',
      selectedEvent.value.trim() ? `事件=${selectedEvent.value.trim()}` : '',
      selectedLevel.value ? `等级=${getLevelConfig(selectedLevel.value).label}` : '',
      selectedType.value ? `类型=${getTypeConfig(selectedType.value).label}` : '',
    ].filter(Boolean)

    const header = [
      'DysonSphereBox 系统日志导出',
      `导出时间：${formatFullTime(new Date().toISOString())}`,
      `导出数量：${exportedLogs.length} / ${exportTotal}`,
      `筛选条件：${filterSummary.length > 0 ? filterSummary.join('，') : '全部日志'}`,
    ].join('\n')
    const content = [
      header,
      ...exportedLogs.map((log, index) => [
        '',
        `========== 日志 ${index + 1} / ${exportedLogs.length} ==========`,
        formatLogText(log),
      ].join('\n')),
    ].join('\n')

    downloadTextFile(buildLogExportFileName(), content)
    message.success(`已导出 ${exportedLogs.length} 条日志`)
  } catch (error) {
    message.error(`导出日志失败：${formatErrorMessage(error)}`)
  } finally {
    isExporting.value = false
  }
}

const formatLogTime = (dateTime: string): string => {
  if (!dateTime) {
    return '未知时间'
  }

  const date = new Date(dateTime)
  const timestamp = date.getTime()
  if (Number.isNaN(timestamp)) {
    return dateTime
  }

  const diff = Date.now() - timestamp
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)

  if (minutes < 1) {
    return '刚刚'
  }
  if (minutes < 60) {
    return `${minutes} 分钟前`
  }
  if (hours < 24) {
    return `${hours} 小时前`
  }
  if (days < 7) {
    return `${days} 天前`
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const formatFullTime = (dateTime: string): string => {
  if (!dateTime) {
    return '未知时间'
  }

  const date = new Date(dateTime)
  if (Number.isNaN(date.getTime())) {
    return dateTime
  }

  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}

function buildTextOptions(defaultLabel: string, values: string[]): SelectOption[] {
  const seen = new Set<string>()
  const items = values
    .map((value) => value.trim())
    .filter((value) => {
      if (!value || seen.has(value)) {
        return false
      }
      seen.add(value)
      return true
    })
    .map((value) => ({
      label: value,
      value,
    }))

  return [
    {label: defaultLabel, value: ''},
    ...items,
  ]
}

watch([
  selectedNamespace,
  selectedModule,
  selectedEvent,
  selectedLevel,
  selectedType,
], resetPageAndLoad)

watch([page, pageSize], () => {
  void loadLogs()
})

onMounted(() => {
  void loadLogs()
})
</script>

<template>
  <section class="mx-auto max-w-5xl px-8 py-8">
    <!-- ~ 日志工具栏 -->
    <!-- * 说明：展示当前日志数量、刷新入口、导出入口和筛选面板开关 -->
    <!-- * 交互：点击刷新会重新读取当前条件下的日志，导出会按当前筛选条件批量下载日志文本 -->
    <!-- * 依赖：依赖 LogService.ListLogs 与当前页面分页状态 -->
    <!-- * 边界：筛选面板悬浮在按钮下方，不参与页面滚动布局，避免展开时挤压日志列表 -->
    <div class="sticky top-0 z-20 -mx-8 -mt-8 border-b border-zinc-200 bg-[#fbfbfc] px-8 py-2">
      <div class="flex h-9 items-center justify-between gap-3">
        <p class="text-xs text-zinc-500">
          共 <span class="font-medium text-zinc-900">{{ total }}</span> 条日志
        </p>

        <div class="flex items-center gap-2">
          <n-button secondary size="small" :loading="isExporting" :disabled="exportableLogCount === 0" @click="exportLogs">
            <template #icon>
              <Download :size="15" :stroke-width="1.8" />
            </template>
            导出
          </n-button>
          <n-button secondary size="small" :loading="isLoading" @click="loadLogs">
            <template #icon>
              <RefreshCw :size="15" :stroke-width="1.8" />
            </template>
            刷新
          </n-button>
          <div class="relative">
            <n-button secondary size="small" @click="showFilters = !showFilters">
              <template #icon>
                <ListFilter :size="15" :stroke-width="1.8" />
              </template>
              筛选
              <span
                v-if="hasActiveFilters"
                class="ml-1 rounded-sm bg-blue-600 px-1.5 py-0.5 text-[11px] leading-none text-white"
              >
                {{ activeFilterCount }}
              </span>
            </n-button>

            <!-- ~ 悬浮筛选 -->
            <!-- * 说明：承载命名空间、模块、事件名、日志等级和日志类型筛选 -->
            <!-- * 交互：输入或选择后自动回到第一页并刷新列表，可一键清除全部条件 -->
            <!-- * 依赖：依赖 selectedNamespace、selectedModule、selectedLevel、selectedType、selectedEvent 等筛选状态 -->
            <!-- * 边界：事件名采用精确匹配，面板绝对定位在筛选按钮下方，不改变列表滚动位置 -->
            <section
              v-if="showFilters"
              class="absolute right-0 top-full z-30 mt-2 w-[min(46rem,calc(100vw-14rem))] rounded border border-zinc-200 bg-white px-4 py-4 shadow-lg"
            >
              <div class="mb-4 flex items-center justify-between">
                <p class="text-sm font-medium text-zinc-950">筛选条件</p>
                <n-button v-if="hasActiveFilters" quaternary size="tiny" type="primary" @click="clearFilters">
                  <template #icon>
                    <X :size="14" :stroke-width="1.9" />
                  </template>
                  清除所有
                </n-button>
              </div>

              <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-[10rem_10rem_10rem_10rem_1fr]">
                <label class="block">
                  <span class="mb-2 block text-xs text-zinc-500">命名空间</span>
                  <n-select v-model:value="selectedNamespace" size="small" :options="namespaceOptions" />
                </label>
                <label class="block">
                  <span class="mb-2 block text-xs text-zinc-500">模块</span>
                  <n-select v-model:value="selectedModule" size="small" :options="moduleOptions" />
                </label>
                <label class="block">
                  <span class="mb-2 block text-xs text-zinc-500">日志等级</span>
                  <n-select v-model:value="selectedLevel" size="small" :options="levelOptions" />
                </label>
                <label class="block">
                  <span class="mb-2 block text-xs text-zinc-500">日志类型</span>
                  <n-select v-model:value="selectedType" size="small" :options="typeOptions" />
                </label>
                <label class="block">
                  <span class="mb-2 block text-xs text-zinc-500">事件名</span>
                  <n-input v-model:value="selectedEvent" size="small" placeholder="例如：set_config" clearable />
                </label>
              </div>
            </section>
          </div>
        </div>
      </div>
    </div>

    <!-- ~ 日志时间线 -->
    <!-- * 说明：按发生时间倒序展示日志内容、维度、错误详情和结构化扩展信息 -->
    <!-- * 交互：分页切换会重新请求后端，错误状态提供重试按钮 -->
    <!-- * 依赖：依赖 logs、total、page、pageSize 与 LogDTO 字段 -->
    <!-- * 边界：metadata 只做通用键值展示，不引入端口、进程等业务专属字段 -->
    <section class="mt-4">
      <n-spin :show="isLoading">
        <div v-if="loadError" class="rounded border border-red-200 bg-red-50 px-4 py-4">
          <div class="flex items-start justify-between gap-4">
            <div>
              <p class="text-sm font-medium text-red-700">日志读取失败</p>
              <p class="mt-1 text-xs text-red-600">{{ loadError }}</p>
            </div>
            <n-button size="small" type="error" secondary @click="loadLogs">重试</n-button>
          </div>
        </div>

        <div v-else class="relative">
          <div class="absolute bottom-0 left-[15px] top-0 w-px bg-zinc-200"></div>

          <div v-if="logs.length === 0" class="rounded border border-dashed border-zinc-200 bg-white py-14">
            <n-empty description="无符合条件的日志" />
          </div>

          <div v-else class="space-y-4">
            <article v-for="log in logs" :key="log.id" class="relative pl-12">
              <div
                class="absolute left-[9px] top-3 h-3 w-3 rounded-full border-2 border-[#fbfbfc]"
                :class="getLevelConfig(log.level).dotClass"
              ></div>

              <div class="rounded border border-zinc-200 bg-white px-4 py-4 transition hover:border-blue-200">
                <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
                  <div class="flex min-w-0 flex-wrap items-center gap-2">
                    <component
                      :is="getLevelConfig(log.level).icon"
                      :size="16"
                      :stroke-width="1.8"
                      :class="getLevelConfig(log.level).textClass"
                    />
                    <span class="rounded-sm px-2 py-0.5 text-xs" :class="getLevelConfig(log.level).badgeClass">
                      {{ getLevelConfig(log.level).label }}
                    </span>
                    <span class="rounded-sm bg-zinc-100 px-2 py-0.5 text-xs" :class="getTypeConfig(log.type).textClass">
                      {{ getTypeConfig(log.type).label }}
                    </span>
                  </div>
                  <div class="flex items-start gap-2">
                    <div class="text-right text-xs text-zinc-500">
                      <p>{{ formatLogTime(log.dateTime) }}</p>
                      <p class="mt-0.5">{{ formatFullTime(log.dateTime) }}</p>
                    </div>
                    <n-tooltip trigger="hover">
                      <template #trigger>
                        <n-button quaternary circle size="tiny" @click="copyLog(log)">
                          <template #icon>
                            <ClipboardCopy :size="14" :stroke-width="1.8" />
                          </template>
                        </n-button>
                      </template>
                      复制日志信息
                    </n-tooltip>
                  </div>
                </div>

                <p class="break-words text-sm leading-6 text-zinc-950">{{ log.content || '未填写日志内容' }}</p>

                <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500">
                  <span>命名空间：<strong class="font-medium text-zinc-700">{{ log.namespace || '-' }}</strong></span>
                  <span>模块：<strong class="font-medium text-zinc-700">{{ log.module || '-' }}</strong></span>
                  <span>事件：<strong class="font-medium text-zinc-700">{{ log.event || '-' }}</strong></span>
                </div>

                <div v-if="log.error" class="mt-3 rounded-sm border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-700">
                  错误详情：{{ log.error }}
                </div>

                <div
                  v-if="metadataEntries(log.metadata).length > 0"
                  class="mt-3 rounded-sm border border-zinc-100 bg-zinc-50 px-3 py-2 text-xs"
                >
                  <p class="mb-1 text-zinc-500">扩展信息</p>
                  <div class="grid gap-1">
                    <div v-for="[key, value] in metadataEntries(log.metadata)" :key="key" class="grid grid-cols-[7rem_1fr] gap-2">
                      <span class="truncate text-zinc-500">{{ key }}</span>
                      <span class="break-all text-zinc-700">{{ formatMetadataValue(value) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </article>
          </div>
        </div>
      </n-spin>
    </section>

    <!-- ~ 分页控制 -->
    <!-- * 说明：展示当前分页区间，并允许切换页码和每页数量 -->
    <!-- * 交互：页码或每页数量变化会触发后端分页查询 -->
    <!-- * 依赖：依赖 total、page、pageSize 与 pageSizeOptions -->
    <!-- * 边界：没有日志时隐藏分页控件，避免无效操作 -->
    <footer v-if="total > 0" class="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-zinc-200 pt-4">
      <span class="text-xs text-zinc-500">{{ pageRangeText }}</span>
      <n-pagination
        v-model:page="page"
        v-model:page-size="pageSize"
        show-size-picker
        :item-count="total"
        :page-sizes="pageSizeOptions"
      />
    </footer>
  </section>
</template>
