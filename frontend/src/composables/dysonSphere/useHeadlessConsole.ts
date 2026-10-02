import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import {
  exportHeadlessServerLog,
  getHeadlessEnvironmentStatus,
  listHeadlessServerArchives,
  prepareHeadlessEnvironment,
  readHeadlessServerLog,
  saveHeadlessSettings,
  startHeadlessEnvironment,
  stopHeadlessEnvironment,
} from '../../services/dysonSphere/headless'
import { contract } from '../../../wailsjs/go/models'
import { formatErrorMessage } from '../../utils/format'

// =============== 控制台状态 ===============
// 说明：维护服务器快照、设置草稿、日志筛选和异步操作状态。
// 使用：开服工作台页面绑定本状态，页面被 KeepAlive 缓存时在重新激活后恢复采样。
export const useHeadlessConsole = () => {
const message = useMessage()
const status = ref<contract.HeadlessEnvironmentStatus | null>(null)
const logSnapshot = ref<contract.HeadlessLogSnapshot | null>(null)
const loading = ref(true)
const busyAction = ref<'prepare' | 'start' | 'stop' | 'save' | ''>('')
const hostPortDraft = ref('8469')
const archiveDraft = ref('')
const archiveOptions = ref<contract.HeadlessArchiveOption[]>([])
const archiveLoading = ref(false)
const archiveError = ref('')
const logSource = ref('bepinex')
const logLevel = ref('all')
const logSearch = ref('')
const autoScroll = ref(true)
const lastUpdated = ref('')
const errorText = ref('')
const logError = ref('')
const settingsDirty = computed(() => status.value !== null && (
  Number(hostPortDraft.value) !== status.value.hostPort || archiveDraft.value.trim() !== status.value.serverArchive
))
// 说明：按输入内容做不区分大小写的包含匹配；输入恰为完整存档名时仍展示全部候选，便于改选。
const archiveSuggestions = computed(() => {
  const needle = archiveDraft.value.trim().toLocaleLowerCase()
  const exact = archiveOptions.value.some((item) => item.name.toLocaleLowerCase() === needle)
  if (!needle || exact) return archiveOptions.value
  return archiveOptions.value.filter((item) => item.name.toLocaleLowerCase().includes(needle))
})
const filteredLogLines = computed(() => {
  const needle = logSearch.value.trim().toLocaleLowerCase()
  return (logSnapshot.value?.lines ?? []).filter((line) => {
    const levelMatches = logLevel.value === 'all' || line.level === logLevel.value
    const textMatches = !needle || line.content.toLocaleLowerCase().includes(needle)
    return levelMatches && textMatches
  })
})

let statusTimer: ReturnType<typeof setInterval> | undefined
let logTimer: ReturnType<typeof setInterval> | undefined
let statusRequestActive = false
let logRequestActive = false

// =============== 数据刷新 ===============
// 说明：分别轮询环境状态和固定日志源，避免慢日志读取阻塞状态刷新。
// 边界：只在页面可见时轮询；后台失败保留最后一次有效快照。
const refreshStatus = async (initial = false): Promise<void> => {
  if (statusRequestActive) return
  statusRequestActive = true
  try {
    const retainDraft = settingsDirty.value
    const next = await getHeadlessEnvironmentStatus()
    status.value = next
    if (!retainDraft || initial) {
      hostPortDraft.value = String(next.hostPort || 8469)
      archiveDraft.value = next.serverArchive ?? ''
    }
    lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    errorText.value = ''
  } catch (error) {
    errorText.value = formatErrorMessage(error)
  } finally {
    statusRequestActive = false
    loading.value = false
  }
}

const refreshLog = async (): Promise<void> => {
  if (logRequestActive) return
  logRequestActive = true
  const requestedSource = logSource.value
  try {
    const next = await readHeadlessServerLog(requestedSource)
    if (requestedSource === logSource.value) {
      logSnapshot.value = next
      logError.value = ''
    }
  } catch (error) {
    if (requestedSource === logSource.value) logError.value = formatErrorMessage(error)
  } finally {
    logRequestActive = false
    if (requestedSource !== logSource.value) void refreshLog()
  }
}

const stopPolling = (): void => {
  if (statusTimer) clearInterval(statusTimer)
  if (logTimer) clearInterval(logTimer)
  statusTimer = undefined
  logTimer = undefined
}

const startPolling = (): void => {
  if (statusTimer) return
  void refreshStatus()
  void refreshLog()
  statusTimer = setInterval(() => void refreshStatus(), 3000)
  logTimer = setInterval(() => void refreshLog(), 5000)
}

watch(logSource, () => {
  logSnapshot.value = null
  logError.value = ''
  void refreshLog()
})

// =============== 用户操作 ===============
// 说明：将准备、启停、保存、存档联想和导出统一映射为可恢复的真实后端操作。
// 边界：取消系统文件选择器不视为错误；运行中的配置禁止保存。
const prepare = async (): Promise<void> => {
  busyAction.value = 'prepare'
  try {
    const result = await prepareHeadlessEnvironment()
    status.value = result.status
    message.success(result.message || '环境检查完成')
  } catch (error) {
    message.error(formatErrorMessage(error))
  } finally {
    busyAction.value = ''
    await refreshStatus()
  }
}

const start = async (): Promise<void> => {
  busyAction.value = 'start'
  try {
    const result = await startHeadlessEnvironment()
    status.value = result.status
    if (result.started) message.success(result.message || '服务器启动命令已执行')
    else message.warning(result.message || '环境尚未就绪')
  } catch (error) {
    message.error(formatErrorMessage(error))
  } finally {
    busyAction.value = ''
    await refreshStatus()
  }
}

const stop = async (): Promise<void> => {
  busyAction.value = 'stop'
  try {
    const result = await stopHeadlessEnvironment()
    status.value = result.status
    message[result.stopped ? 'success' : 'warning'](result.message || '服务器停止状态已更新')
  } catch (error) {
    message.error(formatErrorMessage(error))
  } finally {
    busyAction.value = ''
    await refreshStatus()
  }
}

const saveSettings = async (): Promise<void> => {
  const port = Number(hostPortDraft.value)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    message.error('端口范围应为 1 到 65535')
    return
  }
  busyAction.value = 'save'
  try {
    const result = await saveHeadlessSettings(port, archiveDraft.value.trim())
    status.value = result.status
    hostPortDraft.value = String(result.hostPort)
    archiveDraft.value = result.serverArchive
    message.success('服务器配置已保存')
  } catch (error) {
    message.error(formatErrorMessage(error))
  } finally {
    busyAction.value = ''
    await refreshStatus()
  }
}

// 说明：配置面板展开时刷新存档候选；失败只在面板内提示，不打断手动输入。
const loadArchiveOptions = async (): Promise<void> => {
  if (archiveLoading.value) return
  archiveLoading.value = true
  try {
    archiveOptions.value = await listHeadlessServerArchives()
    archiveError.value = ''
  } catch (error) {
    archiveOptions.value = []
    archiveError.value = formatErrorMessage(error)
  } finally {
    archiveLoading.value = false
  }
}

const exportLog = async (): Promise<void> => {
  try {
    const target = await exportHeadlessServerLog(logSource.value)
    if (target) message.success('日志已导出')
  } catch (error) {
    message.error(formatErrorMessage(error))
  }
}

onMounted(startPolling)
onActivated(startPolling)
onDeactivated(stopPolling)
onUnmounted(stopPolling)

return {
  status, logSnapshot, loading, busyAction, hostPortDraft, archiveDraft, archiveSuggestions, archiveLoading, archiveError,
  logSource, logLevel, logSearch, autoScroll, lastUpdated, errorText, logError, settingsDirty, filteredLogLines,
  refreshStatus, refreshLog, prepare, start, stop, saveSettings, loadArchiveOptions, exportLog,
}
}
