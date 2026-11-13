import {CheckInstalledPackageUpdates, UpdateRemotePackage} from '../../wailsjs/go/modDependence/InstallerService'
import type {contract, updates} from '../../wailsjs/go/models'

// =============== 模组更新桥接 ===============
// 说明：后台检查可复用 Thunderstore 索引缓存；手动检查强制请求新索引，更新只传已安装 key 和明确的新版本 key。
export const checkInstalledModUpdates = async (forceRefresh = false): Promise<updates.PackageUpdate[]> => (
  CheckInstalledPackageUpdates(forceRefresh)
)

// 说明：后端重新核对来源、模组身份和版本递增后，复用现有安装器、审计与回滚。
export const updateInstalledMod = async (packageKey: string, targetPackageKey: string): Promise<contract.PackageInstallResult> => (
  UpdateRemotePackage(packageKey, targetPackageKey)
)
