package updates

// 文件职责：定义已安装市场包更新结果的 Wails DTO。
// 使用：InstallerService.CheckInstalledPackageUpdates 返回给模组市场，生成的 TypeScript 类型归属 updates 命名空间。
// 边界：只描述旧入口与远端目标的对应关系，不查询索引、不执行安装或持久化。

import modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"

// PackageUpdate 描述一个已安装包及其可用更新。
// 职责：关联旧包 key、当前版本和可安装的新版本 DTO。
// 使用：模组市场展示更新按钮，并按 LatestPackage.Key 的进度映射回 PackageKey 对应的旧卡片。
// 边界：仅代表检查时的快照，执行更新时必须重新校验安装状态与目标版本。
type PackageUpdate struct {
	// PackageKey 职责：保存旧安装入口；使用：卡片映射和 UpdateRemotePackage 首个参数；边界：更新成功后由新 key 替代。
	PackageKey string `json:"packageKey"`
	// CurrentVersion 职责：保存本地安装版本；使用：展示更新前版本；边界：保留原始文本，不作为字符串排序依据。
	CurrentVersion string `json:"currentVersion"`
	// LatestVersion 职责：保存最新 active 版本号；使用：展示更新目标；边界：只有语义版本严格高于当前版本时才产生更新。
	LatestVersion string `json:"latestVersion"`
	// LatestPackage 职责：携带统一远端包 DTO；使用：提供更新目标 key 和市场展示数据；边界：执行时不信任前端回传的下载元数据。
	LatestPackage modDependenceResponse.RemoteModPackageDTO `json:"latestPackage"`
}
