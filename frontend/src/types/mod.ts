import { contract } from '../../wailsjs/go/models'

// =============== Mod 安装类型 ===============
// 说明：集中声明 Mod 页与安装器事件共享的前端类型，避免页面和组合式函数重复维护事件结构。
export type InstallConflictAction = 'overwrite' | 'skip' | 'fail'

export interface InstallProgressPayload {
  packageKey: string
  stage: string
  status: string
  percent: number
  current: number
  total: number
  bytesPerSecond: number
  message: string
  state: string
  errorMessage: string
  conflict?: contract.InstallConflictDTO
}
