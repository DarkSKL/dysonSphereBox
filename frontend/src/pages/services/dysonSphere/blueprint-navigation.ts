// 职责：在本地页面间请求打开蓝图库标签；使用：生成器入库后查看；边界：不增加 URL query 或历史条目，不保存蓝图串。
let libraryRequested = false
export const requestBlueprintLibrary = (): void => { libraryRequested = true }
export const consumeBlueprintLibraryRequest = (): boolean => {
  const requested = libraryRequested
  libraryRequested = false
  return requested
}
