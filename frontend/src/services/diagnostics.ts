import type { App } from 'vue'
import type { Router } from 'vue-router'
import type { LogEntry, LogHandler } from '../types/logging'
import { errorEvidence, frontendEntry, newOperationId } from '../utils/diagnostics'
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

// =============== Wails 调用边界 ===============
// 说明：集中观察生成绑定最终调用的 window.go；不记录参数和返回正文，不改变 promise 结果。
const instrumentBridge = (router: Router): (() => void) => {
  const go = (window as BridgeWindow).go
  if (!go) return () => undefined
  const restores: (() => void)[] = []
  for (const [domain, services] of Object.entries(go)) {
    for (const [service, methods] of Object.entries(services)) {
      if (service === 'LogService') continue
      for (const [name, original] of Object.entries(methods)) {
        if (typeof original !== 'function') continue
        const tracked = /^(Install|Uninstall|Continue|Cancel|Set|Save|Update|Import|Export|Enable|Disable|Delete|Remove|Create|Join|Leave|Start|Stop|Prepare|Apply|Migrate|Sync|Recover|Select)/.test(name)
        const handler: LogHandler = /^(Recover)/.test(name) ? 'System' : 'Unknown'
        const wrapped = (...args: unknown[]): Promise<unknown> => {
          const operationId = newOperationId()
          const start = performance.now()
          const data = { service, method: name, route: router.currentRoute.value.path }
          const record = (result: string, error?: unknown): void => {
            const special = typeof error === 'string' && /cancel|conflict|取消|冲突/i.test(error)
            const entry = frontendEntry(`${service}.${name}`, `${domain}.${service}.${name}`, error === undefined ? '前端发起的 Wails 调用返回结果' : 'Wails 调用返回错误，未推断底层根因', { ...data, result: special ? 'needs_attention' : result, durationMs: Math.round(performance.now() - start), ...(error === undefined ? {} : { error: errorEvidence(error) }) }, error === undefined ? 'Info' : special ? 'Warn' : 'Error', handler, operationId)
            recordFrontendLog({ ...entry, source: 'frontend.bridge' })
          }
          if (tracked) record('started')
          let promise: Promise<unknown>
          try { promise = original.apply(methods, args) } catch (error) { record('failed', error); throw error }
          return promise.then((result: unknown) => { if (tracked) record('returned'); return result }, (error: unknown) => { record('failed', error); throw error })
        }
        methods[name] = wrapped
        restores.push(() => { if (methods[name] === wrapped) methods[name] = original })
      }
    }
  }
  return () => restores.forEach((restore) => restore())
}

// installDiagnostics 职责：安装全局异常边界；使用：Vue mount 之前；边界：错误记录自身失败不递归进入异常处理。
export const installDiagnostics = (app: App, router: Router): (() => void) => {
  try {
    const stored: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    if (Array.isArray(stored)) {
      for (const item of stored.slice(-100) as unknown[]) {
        if (item && typeof item === 'object' && 'source' in item && typeof item.source === 'string' && item.source.startsWith('frontend.')) queue.push(item as LogEntry)
      }
    }
  } catch { try { localStorage.removeItem(STORAGE_KEY) } catch { /* 浏览器存储不可用时不阻止应用挂载。 */ } }
  const capture = (namespace: string, error: unknown, extra: Record<string, unknown> = {}): void => recordFrontendLog(frontendEntry('前端运行异常', namespace, '前端异常边界捕获错误', { route: router.currentRoute.value.path, error: errorEvidence(error), ...extra }))
  const previous = app.config.errorHandler
  app.config.errorHandler = (error, instance, info) => { capture('system.frontend.vueError', error, { info }); previous?.(error, instance, info) }
  const onError = (event: ErrorEvent): void => capture('system.frontend.globalError', event.error ?? event.message, { filename: event.filename, line: event.lineno, column: event.colno })
  const onRejection = (event: PromiseRejectionEvent): void => capture('system.frontend.unhandledRejection', event.reason)
  window.addEventListener('error', onError)
  window.addEventListener('unhandledrejection', onRejection)
  const restore = instrumentBridge(router)
  const timer = window.setInterval(() => { void flushFrontendLogs() }, 5000)
  void flushFrontendLogs()
  return () => { clearInterval(timer); restore(); app.config.errorHandler = previous; window.removeEventListener('error', onError); window.removeEventListener('unhandledrejection', onRejection) }
}
