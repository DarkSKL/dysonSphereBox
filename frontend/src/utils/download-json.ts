// 职责：把可序列化对象以 JSON 文件下载到本地；使用：调试样例与事件导出；边界：只触发下载，不弹出保存对话框。
export const downloadJson = (data: unknown, filename: string): void => {
  const blob = new Blob([JSON.stringify(data, null, 2)], {type: 'application/json'})
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  // 立即回收会让部分 WebView 来不及开始下载，延后一帧再释放。
  setTimeout(() => URL.revokeObjectURL(url), 0)
}
