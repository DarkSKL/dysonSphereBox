import type { LogEntry, LogHandler, LogLevel } from '../types/logging'

// =============== 前端证据清洗 ===============
// 说明：在桥接不可用时也先脱敏，再进入本机有界缓存；后端仍会再次清洗。
export const cleanLogText = (value: string): string => value
  .replace(/https?:\/\/[^\s"'<>\uFF1A\uFF0C\uFF08\uFF09\u3002]+/g, (raw) => {
    try {
      const url = new URL(raw)
      url.username = ''; url.password = ''; url.hash = ''
      url.searchParams.forEach((_, key) => url.searchParams.set(key, '[redacted]'))
      return url.toString()
    } catch { return '[invalid-url]' }
  })
  .replace(/(bearer\s+)[^\s"']+/gi, '$1[redacted]')
  .replace(/((?:password|passwd|token|secret|authorization|cookie|api[_-]?key)\s*[=:]\s*)[^\s&,;"']+/gi, '$1[redacted]')
  .slice(0, 16384)

export const cleanLogValue = (value: unknown, depth = 0): unknown => {
  if (depth > 8) return '[depth-limit]'
  if (typeof value === 'string') return cleanLogText(value)
  if (Array.isArray(value)) return value.slice(0, 100).map((item: unknown) => cleanLogValue(item, depth + 1))
  if (value && typeof value === 'object') {
    const output: Record<string, unknown> = {}
    for (const [key, item] of Object.entries(value)) {
      output[key] = /password|passwd|token|secret|authorization|cookie|apikey|requestbody|responsebody/i.test(key.replace(/[_-]/g, '')) ? '[redacted]' : cleanLogValue(item, depth + 1)
    }
    return output
  }
  return value
}

export const errorEvidence = (error: unknown): Record<string, unknown> => {
  if (error instanceof Error) return { type: error.name, message: cleanLogText(error.message), stack: cleanLogText(error.stack ?? '') }
  return { type: typeof error, message: cleanLogText(typeof error === 'string' ? error : String(error)) }
}

export const newOperationId = (): string => globalThis.crypto?.randomUUID?.() ?? `frontend-${Date.now()}-${Math.random().toString(16).slice(2)}`

export const frontendEntry = (action: string, namespace: string, reason: string, data: Record<string, unknown>, level: LogLevel = 'Error', handler: LogHandler = 'Unknown', operationId = ''): LogEntry => ({
  id: '', schemaVersion: 2, action: cleanLogText(action), level, source: 'frontend.runtime', reason, namespace,
  data: cleanLogValue(data) as Record<string, unknown>, time: new Date().toISOString(), handler,
  sessionId: '', operationId, appVersion: '',
})
