// 职责：协调全站唯一预览实例与 UMD 加载；使用：viewer-panel；边界：不保存蓝图串、不改应用主题。
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
