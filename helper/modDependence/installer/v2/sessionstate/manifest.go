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
	// PlannedTargetDirs 职责：记录计划创建或使用的目标目录；使用：中断清理空目录；边界：不会单独决定删除非空目录。
	PlannedTargetDirs []string `json:"plannedTargetDirs,omitempty"`
	// InstalledComponents 职责：记录已完成保存的组件 key；使用：恢复时判断哪些组件需要数据库回滚；边界：不代表所有文件都仍存在。
	InstalledComponents []string `json:"installedComponents,omitempty"`
	// InstalledFiles 职责：记录本次会话已复制文件及备份信息；使用：失败、取消和中断恢复；边界：只记录安装器管理文件。
	InstalledFiles []File `json:"installedFiles,omitempty"`
	// PreviousPackage 职责：保存覆盖安装前的主包快照；使用：失败清理后恢复历史安装状态；边界：首次安装可为空。
	PreviousPackage *PackageSnapshot `json:"previousPackage,omitempty"`
	// PreviousComponents 职责：保存覆盖安装前的组件快照；使用：失败清理后恢复组件记录；边界：只包含当前会话涉及组件。
	PreviousComponents []ComponentSnapshot `json:"previousComponents,omitempty"`
	// CleanedAt 职责：记录清理完成时间；使用：避免重复清理和诊断；边界：空值表示尚未完成清理。
	CleanedAt string `json:"cleanedAt,omitempty"`
}
