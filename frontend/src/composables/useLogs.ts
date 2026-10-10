import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { deleteLog, exportDiagnostics, getLogHealth, listLogs, openLogDirectory } from '../services/logging'
import { flushFrontendLogs, frontendLogHealth } from '../services/diagnostics'
import { emptyLogQuery, type LogEntry, type LogHealth } from '../types/logging'
import { formatErrorMessage } from '../utils/format'
import { EventsOn } from '../../wailsjs/runtime/runtime'

// =============== 日志查询状态 ===============
// 说明：页面只渲染状态；请求序号阻止过期响应覆盖筛选结果，卸载停止刷新。
export const useLogs = () => {
  const message = useMessage()
  const query = reactive(emptyLogQuery())
  const items = ref<LogEntry[]>([])
  const total = ref(0)
  const loading = ref(false)
  const exporting = ref(false)
  const deleting = ref('')
  const error = ref('')
  const warning = ref('')
  const health = ref<LogHealth | null>(null)
  const selected = ref<LogEntry | null>(null)
  const timeline = ref<LogEntry[]>([])
  const timelineWarning = ref('')
  const frontendHealth = ref(frontendLogHealth())
  let version = 0
  let detailVersion = 0
  let disposed = false
  let timer: number | undefined
  let refreshTimer: number | undefined
  let unsubscribe: (() => void) | undefined
  let refreshPending = false
  const healthWarning = computed(() => [health.value?.file.fileError, health.value?.indexError, health.value?.file.memoryCount ? `${health.value.file.memoryCount} 条后端日志只有内存副本，异常退出时可能丢失` : '', health.value?.file.dropped ? `文件写入失败后缓存已丢弃 ${health.value.file.dropped} 条记录` : '', frontendHealth.value.error, frontendHealth.value.dropped ? `前端缓存已丢弃 ${frontendHealth.value.dropped} 条记录` : ''].filter(Boolean).join('；'))

  const load = async (reset = false): Promise<void> => {
    if (reset) query.page = 1
    const current = ++version
    loading.value = true; error.value = ''
    try {
      await flushFrontendLogs()
      const [page, state] = await Promise.all([listLogs({ ...query }), getLogHealth()])
      if (disposed || current !== version) return
      items.value = page.items; total.value = page.total; warning.value = page.warning; health.value = state
      frontendHealth.value = frontendLogHealth()
    } catch (caught) { if (!disposed && current === version) error.value = formatErrorMessage(caught) }
    finally {
      if (current === version && !disposed) {
        loading.value = false
        if (refreshPending) { refreshPending = false; scheduleRefresh() }
      }
    }
  }

  // 后端只通知“数据已变化”；列表始终重新查询，不在前端拼接日志。
  const scheduleRefresh = (): void => {
    if (disposed) return
    if (loading.value) { refreshPending = true; return }
    if (refreshTimer !== undefined) return
    refreshTimer = window.setTimeout(() => { refreshTimer = undefined; void load() }, 250)
  }

  const select = async (entry: LogEntry): Promise<void> => {
    selected.value = entry; timeline.value = []; timelineWarning.value = ''
    const current = ++detailVersion
    if (!entry.operationId) return
    try {
      const page = await listLogs({ ...emptyLogQuery(), operationId: entry.operationId, pageSize: 200 })
      if (current === detailVersion && !disposed) { timeline.value = [...page.items].reverse(); timelineWarning.value = page.warning || (page.total > 200 ? '仅显示最近 200 条关联事件' : '') }
    } catch (caught) { if (current === detailVersion) timelineWarning.value = formatErrorMessage(caught) }
  }

  const exportBundle = async (): Promise<void> => {
    exporting.value = true
    try { const target = await exportDiagnostics({ ...query }); if (target) message.success(`诊断包已导出：${target}`) }
    catch (caught) { message.error(formatErrorMessage(caught)) }
    finally { exporting.value = false }
  }
  // 删除成功后刷新后端分页；末页删空时回到有效页，失败保留当前内容。
  const remove = async (entry: LogEntry): Promise<void> => {
    if (deleting.value) return
    deleting.value = entry.id
    try {
      await deleteLog(entry.id)
      if (disposed) return
      if (selected.value?.id === entry.id) { selected.value = null; detailVersion++ }
      else if (selected.value?.operationId && selected.value.operationId === entry.operationId) await select(selected.value)
      await load()
      if (disposed) return
      const lastPage = Math.max(1, Math.ceil(total.value / query.pageSize))
      if (query.page > lastPage) { query.page = lastPage; await load() }
      message.success('日志已删除')
    } catch (caught) { if (!disposed) message.error(`删除日志失败：${formatErrorMessage(caught)}`) }
    finally { deleting.value = '' }
  }
  const openDirectory = async (): Promise<void> => { try { await openLogDirectory() } catch (caught) { message.error(formatErrorMessage(caught)) } }
  const copy = async (): Promise<void> => { if (!selected.value) return; try { await navigator.clipboard.writeText(JSON.stringify(selected.value, null, 2)); message.success('日志已复制') } catch { message.error('复制失败，请使用诊断包导出') } }
  const reset = (): void => { Object.assign(query, emptyLogQuery()); void load() }
  onMounted(() => {
    try { unsubscribe = EventsOn('system:logsChanged', scheduleRefresh) } catch { /* 普通浏览器无 Wails 运行时，接口错误由 load 显示。 */ }
    void load()
    timer = window.setInterval(scheduleRefresh, 5000)
  })
  onBeforeUnmount(() => { disposed = true; version++; detailVersion++; clearInterval(timer); clearTimeout(refreshTimer); unsubscribe?.() })
  return { query, items, total, loading, exporting, deleting, error, warning, health, healthWarning, selected, timeline, timelineWarning, load, select, remove, exportBundle, openDirectory, copy, reset }
}
