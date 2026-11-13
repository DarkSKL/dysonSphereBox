import type {contract} from '../../wailsjs/go/models'

// =============== 市场模组更新状态 ===============
// 说明：旧版本卡片持有新安装任务的 key；下载事件和取消继续使用新 key，不修改旧包安装状态。
export interface InstalledModUpdateTask {
  readonly packageKey: string
  readonly targetKey: string
  running: boolean
  requesting: boolean
  cancelRequested: boolean
  percent: number
  status: string
  downloadSpeed: string
  failureMessage: string
  conflict?: contract.InstallConflictDTO
}
