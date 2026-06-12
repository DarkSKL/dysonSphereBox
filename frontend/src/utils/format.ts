// =============== 通用展示格式化 ===============
// 说明：提供前端页面通用的错误与速度文案格式化，保持组合式逻辑只关注状态流转。
export const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
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
