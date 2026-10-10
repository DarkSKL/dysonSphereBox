// =============== 统一诊断契约 ===============
// 说明：与 Go logging.Entry 使用相同字段；未知触发者不伪造，结构化证据使用 unknown。
export type LogLevel = 'Debug' | 'Info' | 'Warn' | 'Error' | 'Fatal'
export type LogHandler = 'User' | 'System' | 'Unknown'

// 可选筛选项与默认全选共用同一份定义，避免新增选项后默认值遗漏。
export const logLevelOptions = ['Debug', 'Info', 'Warn', 'Error', 'Fatal'].map((value) => ({ label: value, value }))
export const logHandlerOptions = [{ label: '用户', value: 'User' }, { label: '系统', value: 'System' }, { label: '未知', value: 'Unknown' }]
export const logNamespaceOptions = [{ label: '系统', value: 'system' }, { label: '模组 / 开服', value: 'modDependence' }, { label: '资源环境', value: 'environment' }, { label: '联机', value: 'multiplayer' }]

export interface LogEntry {
  readonly id: string
  readonly schemaVersion: number
  readonly action: string
  readonly level: LogLevel
  readonly source: string
  readonly reason: string
  readonly namespace: string
  readonly data: Record<string, unknown>
  readonly time: string
  readonly handler: LogHandler
  readonly sessionId: string
  readonly operationId: string
  readonly appVersion: string
}

export interface LogQuery {
  namespaces: string[]
  levels: string[]
  source: string
  handlers: string[]
  keyword: string
  sessionId: string
  operationId: string
  from: string
  to: string
  page: number
  pageSize: number
}

export interface LogHealth {
  readonly file: { readonly directory: string; readonly sessionId: string; readonly fileError: string; readonly fallback: boolean; readonly memoryCount: number; readonly dropped: number }
  readonly indexError: string
  readonly pending: number
}

export interface LogPage {
  readonly items: LogEntry[]
  readonly total: number
  readonly page: number
  readonly pageSize: number
  readonly warning: string
}

export const emptyLogQuery = (): LogQuery => ({ namespaces: logNamespaceOptions.map((option) => option.value), levels: logLevelOptions.map((option) => option.value), source: '', handlers: logHandlerOptions.map((option) => option.value), keyword: '', sessionId: '', operationId: '', from: '', to: '', page: 1, pageSize: 50 })
