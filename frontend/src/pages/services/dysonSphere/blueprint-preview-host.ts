// 职责：协调全站预览阈值、唯一实例与 UMD 加载；使用：工作区及 viewer-panel；边界：不保存蓝图串、不改应用主题。
// 自动预览和性能提示共用建筑数量阈值；超过阈值仍允许用户手动打开，不作为蓝图导出限制。
export const BLUEPRINT_PREVIEW_BUILDING_LIMIT = 10000

let releaseCurrent: (() => void) | null = null
let loadingScript: Promise<unknown> | null = null

export const claimBlueprintPreview = (dispose: () => void): (() => void) => {
  if (releaseCurrent && releaseCurrent !== dispose) releaseCurrent()
  releaseCurrent = dispose
  return () => { if (releaseCurrent === dispose) releaseCurrent = null }
}

export const shareBlueprintScriptLoad = <T>(load: () => Promise<T>): Promise<T> => {
  if (loadingScript) return loadingScript as Promise<T>
  const pending = load()
  loadingScript = pending
  void pending.finally(() => { if (loadingScript === pending) loadingScript = null }).catch(() => {})
  return pending
}
