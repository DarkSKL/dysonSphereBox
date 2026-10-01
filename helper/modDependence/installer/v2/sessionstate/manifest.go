package sessionstate

// 文件职责：定义 V2 安装会话 manifest 的顶层状态结构。
// 使用：V2 安装、冲突恢复、中断恢复和失败清理在 InstallManifest JSON 中读写 Manifest。
// 边界：只描述 manifest 聚合关系，不执行数据库、文件系统、进度事件或审计日志操作。

import "DysonSphereBox/helper/modDependence/installer/contract"

// Manifest 表示可恢复安装会话的完整 manifest。
// 职责：保存冲突决策、已规划文件、已安装文件、历史包快照和清理标记。
// 使用：安装开始、文件复制、冲突暂停、失败清理和继续安装时读写。
// 边界：只存入 installed package 的 InstallManifest JSON，不直接暴露给前端。
type Manifest struct {
	// Conflict 职责：记录当前等待用户处理的冲突；使用：前端恢复冲突弹窗；边界：非 conflict 状态可为空。
	Conflict *contract.InstallConflictDTO `json:"conflict,omitempty"`
	// Decisions 职责：记录用户已选择的冲突决策；使用：继续安装时复用；边界：仅对当前安装会话有效。
	Decisions []contract.InstallConflictDecisionDTO `json:"decisions,omitempty"`
	// StartedAt 职责：记录会话开始时间；使用：审计和恢复判断；边界：只作诊断。
	StartedAt string `json:"startedAt,omitempty"`
	// PlannedComponents 职责：记录计划处理的组件和安装目录；使用：失败清理补充目录清理目标；边界：可能早于实际文件写入。
	PlannedComponents []ComponentPlan `json:"plannedComponents,omitempty"`
	// ResolvedDependencies 职责：记录实际选中版本的组件依赖边；使用：升级时向旧使用方传递完整引用；边界：不使用远端声明版本代替本地复用版本。
	ResolvedDependencies map[string][]string `json:"resolvedDependencies,omitempty"`
	// PlannedTargetDirs 职责：记录计划创建或使用的目标目录；使用：中断清理空目录；边界：不会单独决定删除非空目录。
	PlannedTargetDirs []string `json:"plannedTargetDirs,omitempty"`
	// InstalledComponents 职责：记录已完成保存的组件 key；使用：恢复时判断哪些组件需要数据库回滚；边界：不代表所有文件都仍存在。
	InstalledComponents []string `json:"installedComponents,omitempty"`
	// ReusedComponents 职责：记录本次只建立引用而未写文件的组件；使用：失败清理排除其原有文件清单；边界：依然需要移除本次新增的组件关联。
	ReusedComponents []string `json:"reusedComponents,omitempty"`
	// InstalledFiles 职责：按写入顺序记录受管文件的恢复证据；使用：失败、取消和中断逆序恢复；边界：复制前持久化，不表示写入已经成功。
	InstalledFiles []File `json:"installedFiles,omitempty"`
	// UntrackedFiles 职责：保存配置写入前的恢复证据；使用：失败时按写入逆序恢复；边界：不进入卸载清单或共享归属。
	UntrackedFiles []File `json:"untrackedFiles,omitempty"`
	// RemovedVersionFiles 职责：保存已移除旧版本文件的备份；使用：失败恢复旧文件和成功清理备份；边界：旧文件数据库记录保留到最终提交。
	RemovedVersionFiles []File `json:"removedVersionFiles,omitempty"`
	// VersionReplacements 职责：记录待提交的旧组件到新组件映射；使用：成功安装后切换所有共享引用；边界：提交前不改变其它包的关联。
	VersionReplacements []VersionReplacement `json:"versionReplacements,omitempty"`
	// PreviousPackage 职责：保存覆盖安装前的主包快照；使用：失败清理后恢复历史安装状态；边界：首次安装可为空。
	PreviousPackage *PackageSnapshot `json:"previousPackage,omitempty"`
	// PreviousComponents 职责：保存覆盖安装前的组件快照；使用：失败清理后恢复组件记录；边界：只包含当前会话涉及组件。
	PreviousComponents []ComponentSnapshot `json:"previousComponents,omitempty"`
	// PreviousPackageComponents 职责：保存会话开始前的包组件关联；使用：重新安装失败时恢复共享依赖引用；边界：历史 manifest 可缺失，不表示本次新增关联。
	PreviousPackageComponents []string `json:"previousPackageComponents,omitempty"`
	// CleanedAt 职责：记录清理完成时间；使用：避免重复清理和诊断；边界：空值表示尚未完成清理。
	CleanedAt string `json:"cleanedAt,omitempty"`
}
