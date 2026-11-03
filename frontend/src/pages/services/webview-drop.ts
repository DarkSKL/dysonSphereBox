// =============== WebView 拖入导航保护 ===============
// 职责：阻止外部文件或链接替换应用页面；使用：由前端入口安装并在热更新时释放。
// 边界：只取消浏览器默认动作，不阻止事件传播，保留 Wails 文件路径回调及页面拖入处理。
export const installWebviewDropGuard = (): (() => void) => {
  const preventNavigation = (event: DragEvent): void => {
    const types = event.dataTransfer?.types
    if (types?.includes('Files') || types?.includes('text/uri-list')) event.preventDefault()
  }

  window.addEventListener('dragover', preventNavigation)
  window.addEventListener('drop', preventNavigation)
  return () => {
    window.removeEventListener('dragover', preventNavigation)
    window.removeEventListener('drop', preventNavigation)
  }
}
