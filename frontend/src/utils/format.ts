// =============== 通用展示格式化 ===============
// 说明：提供前端页面通用的错误与速度文案格式化，保持组合式逻辑只关注状态流转。
export const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}

// 说明：把字节数转换为紧凑的 KB/MB 文案，供资源卡片和详情展示。
// 边界：非法或非正数统一显示 0 KB。
export const formatFileSize = (sizeBytes: number): string => {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) {
    return '0 KB'
  }
  if (sizeBytes >= 1024 * 1024) {
    return `${(sizeBytes / 1024 / 1024).toFixed(1)} MB`
  }

  return `${Math.max(1, Math.round(sizeBytes / 1024))} KB`
}

// 说明：把 Wails 传来的时间值格式化为本地日期。
// 边界：Wails time.Time 可能是字符串、对象或零值，无法解析时返回占位文案。
export const formatDate = (value: unknown): string => {
  if (!value) {
    return '未知时间'
  }
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) {
    return '未知时间'
  }

  return date.toLocaleDateString('zh-CN', {year: 'numeric', month: '2-digit', day: '2-digit'})
}

export const formatDownloadSpeed = (bytesPerSecond: number): string => {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return ''
  }

  const kbPerSecond = bytesPerSecond / 1024
  if (kbPerSecond < 1024) {
    return `${kbPerSecond.toFixed(kbPerSecond >= 100 ? 0 : 1)} KB/s`
  }

  return `${(kbPerSecond / 1024).toFixed(1)} MB/s`
}
