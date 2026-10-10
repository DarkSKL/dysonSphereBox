import { CreateLog, DeleteLog, ExportDiagnostics, GetLogHealth, ListLogs, OpenLogDirectory } from '../../wailsjs/go/system/LogService'
import { logging, system } from '../../wailsjs/go/models'
import { logHandlerOptions, logLevelOptions, logNamespaceOptions, type LogEntry, type LogHealth, type LogPage, type LogQuery } from '../types/logging'

// =============== 日志桥接 ===============
// 说明：统一使用生成绑定，采集异常时不通过自身的桥接监控，避免递归。
// 全选传 null 表示不限（含历史/新增模块）；显式空数组保留“无结果”语义，列表与导出共用转换。
const toLogQuery = (query: LogQuery): system.LogQuery => new system.LogQuery({
  ...query,
  levels: logLevelOptions.every((option) => query.levels.includes(option.value)) ? null : [...query.levels],
  namespaces: logNamespaceOptions.every((option) => query.namespaces.includes(option.value)) ? null : [...query.namespaces],
  handlers: logHandlerOptions.every((option) => query.handlers.includes(option.value)) ? null : [...query.handlers],
})
export const listLogs = async (query: LogQuery): Promise<LogPage> => ListLogs(toLogQuery(query)) as Promise<LogPage>
export const getLogHealth = async (): Promise<LogHealth> => GetLogHealth()
export const exportDiagnostics = async (query: LogQuery): Promise<string> => ExportDiagnostics(toLogQuery(query))
export const openLogDirectory = async (): Promise<void> => OpenLogDirectory()
// 删除单条记录；仅在后端持久化成功后由调用方刷新列表。
export const deleteLog = async (id: string): Promise<void> => DeleteLog(id)
export const submitLog = async (entry: LogEntry): Promise<void> => CreateLog(new logging.Entry(entry))
