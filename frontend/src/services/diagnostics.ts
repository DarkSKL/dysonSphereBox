import type { App } from 'vue'
import type { LogEntry } from '../types/logging'
import { errorEvidence, frontendEntry } from '../utils/diagnostics'
import { submitLog } from './logging'

interface BridgeWindow extends Window { go?: Record<string, Record<string, Record<string, (...args: unknown[]) => Promise<unknown>>>> }

// =============== 有界前端采集 ===============
// 说明：捕获 Vue、全局异常与 Wails 失败，桥接暂不可用时保存已脱敏事件；队列最多 100 条。
const STORAGE_KEY = 'dsbox.diagnostics.pending.v2'
const queue: LogEntry[] = []
let flushing = false
let dropped = 0
let persistError = ''

const persist = (): void => {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(queue)); persistError = '' } catch (error) { persistError = String(error) }
}

export const frontendLogHealth = (): { pending: number; dropped: number; error: string } => ({ pending: queue.length, dropped, error: persistError })

export const flushFrontendLogs = async (): Promise<void> => {
  if (flushing || !(window as BridgeWindow).go) return
  flushing = true
  try {
    while (queue.length) {
      const entry = queue[0]
      if (!entry) break
      await submitLog(entry)
      queue.shift(); persist()
    }
  } catch (error) { persistError = String(error) } finally { flushing = false }
}

export const recordFrontendLog = (entry: LogEntry): void => {
  if (queue.length >= 100) { queue.shift(); dropped++ }
  queue.push(entry); persist()
  void flushFrontendLogs()
}

// =============== 前端异常边界 ===============
// 说明：只采集 Vue 与浏览器中未处理异常；业务意图和结果由后端领域服务记录，避免桥接层重复生成成功日志。

// installDiagnostics 职责：安装全局异常边界；使用：Vue mount 之前；边界：错误记录自身失败不递归进入异常处理。
export const installDiagnostics = (app: App): (() => void) => {
  try {
    const stored: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    if (Array.isArray(stored)) {
      for (const item of stored.slice(-100) as unknown[]) {
        if (item && typeof item === 'object' && 'source' in item && typeof item.source === 'string' && item.source.startsWith('frontend.')) queue.push(item as LogEntry)
      }
    }
  } catch { try { localStorage.removeItem(STORAGE_KEY) } catch { /* 浏览器存储不可用时不阻止应用挂载。 */ } }
  const capture = (namespace: string, error: unknown, extra: Record<string, unknown> = {}): void => recordFrontendLog(frontendEntry('前端运行异常', namespace, '前端异常边界捕获错误', { route: window.location.hash.replace(/^#/, ''), error: errorEvidence(error), ...extra }))
  const previous = app.config.errorHandler
  app.config.errorHandler = (error, instance, info) => { capture('system.frontend.vueError', error, { info }); previous?.(error, instance, info) }
  const onError = (event: ErrorEvent): void => capture('system.frontend.globalError', event.error ?? event.message, { filename: event.filename, line: event.lineno, column: event.colno })
  const onRejection = (event: PromiseRejectionEvent): void => capture('system.frontend.unhandledRejection', event.reason)
  window.addEventListener('error', onError)
  window.addEventListener('unhandledrejection', onRejection)
  const timer = window.setInterval(() => { void flushFrontendLogs() }, 5000)
  void flushFrontendLogs()
  return () => { clearInterval(timer); app.config.errorHandler = previous; window.removeEventListener('error', onError); window.removeEventListener('unhandledrejection', onRejection) }
}
