import {AuthorizeRead, Cancel, ChooseDirectory, Continue, ExportPack, ExportToLibrary, GetDirectory, GetReport, GetTask, InspectResources, ListSaves, ReadBlueprint, RevokeRead, SelectSave, Start} from '../../wailsjs/go/environment/IndustryPlannerService'
import {industryworkspace} from '../../wailsjs/go/models'

// =============== 本地规划桥接 ===============
// 说明：存档、求解、蓝图字符串与 ZIP 均由桌面服务持有，只在预览时拉取选中星球。
export const industryService = {authorize: AuthorizeRead, cancel: Cancel, choose: ChooseDirectory, resume: Continue, export: ExportPack, library: ExportToLibrary, directory: GetDirectory, report: GetReport, task: GetTask, inspect: InspectResources, saves: ListSaves, revoke: RevokeRead, select: SelectSave, start: Start}

/** 顺序读取原版编码。切换预览、任务或选档时立即停止后续桥接。 */
export async function readIndustryBlueprint(task: number, planet: number, signal: AbortSignal): Promise<string> {
  let offset = 0
  const parts: string[] = []
  while (!signal.aborted) {
    const chunk: industryworkspace.Chunk = await ReadBlueprint(task, planet, offset)
    if (signal.aborted) return ''
    if (!chunk.text || chunk.nextOffset !== offset + chunk.text.length) throw new Error('星球蓝图片段不连续，请重新打开预览')
    parts.push(chunk.text)
    if (chunk.done) return parts.join('')
    offset = chunk.nextOffset
  }
  return ''
}
