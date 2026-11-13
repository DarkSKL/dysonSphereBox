import * as binding from '../../wailsjs/go/environment/R2ModmanService'
import type {r2modman} from '../../wailsjs/go/models'

// =============== r2modman 桌面桥接 ===============
// 说明：统一调用生成的 R2ModmanService；DTO 直接使用 r2modman 命名空间。
// 边界：不扫描目录或推断文件归属；开发宿主缺少新绑定时返回可见错误。
const assertBridgeReady = (method: keyof typeof binding): void => {
  const host = window as Window & {
    go?: {environment?: {R2ModmanService?: Record<string, unknown>}}
  }
  if (typeof host.go?.environment?.R2ModmanService?.[method] !== 'function') {
    throw new Error(`R2ModmanService.${method} 尚未绑定，请重新启动 Wails 开发服务或应用后重试。`)
  }
}

// 说明：只读取盒子数据库中的 Profile 列表，不检索实盘。
export const listR2ModmanProfiles = async (): Promise<r2modman.Profile[]> => {
  assertBridgeReady('ListProfiles')
  return binding.ListProfiles()
}

// 说明：严格读取盒子持久快照；无快照时返回空状态，等待用户显式同步。
export const getR2ModmanProfile = async (): Promise<r2modman.ProfileState> => {
  assertBridgeReady('GetProfile')
  return binding.GetProfile()
}

// 说明：仅由显式同步调用实盘检索，后端持久化后返回当前 Profile 快照。
export const syncR2ModmanProfiles = async (): Promise<r2modman.ProfileState> => {
  assertBridgeReady('SyncProfiles')
  return binding.SyncProfiles()
}

// 说明：下拉选择直接切换缓存 Profile；手动定位的新路径由后端读取并持久化。
export const manageR2ModmanProfile = async (path: string): Promise<r2modman.ProfileState> => {
  assertBridgeReady('ManageProfile')
  return binding.ManageProfile(path)
}

// 说明：返回用户手动定位的目录；取消为空，调用方在非空时直接提交 ManageProfile。
export const selectR2ModmanProfileDirectory = async (): Promise<string> => {
  assertBridgeReady('SelectProfileDirectory')
  return binding.SelectProfileDirectory()
}

// 说明：显式传入用户操作时的 Profile 路径，由后端验证并更新启用状态。
export const setR2ModmanModEnabled = async (
  profilePath: string,
  name: string,
  enabled: boolean,
  confirmedNames: string[] = [],
): Promise<r2modman.ProfileState> => {
  assertBridgeReady('SetModEnabled')
  return binding.SetModEnabled(profilePath, name, enabled, confirmedNames)
}

// 说明：从指定 Profile 移除模组；备份及已安装依赖者检查由后端负责。
export const removeR2ModmanMod = async (profilePath: string, name: string): Promise<r2modman.ProfileState> => {
  assertBridgeReady('RemoveMod')
  return binding.RemoveMod(profilePath, name)
}

