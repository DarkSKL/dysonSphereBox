package query

// 文件职责：提供 V2 安装记录和持久化进度快照的只读查询能力。
// 使用：service/modDependence 查询已安装包 key，v2 根包恢复运行时进度时读取持久化快照。
// 边界：只读取 installed 模型，不修改安装状态、不触发恢复清理、不访问远端市场。

import (
	"encoding/json"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/shared"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/datatypes"
)

// installConflictManifest 描述查询持久化冲突快照时需要的最小安装 manifest 结构。
// 职责：只解析冲突 DTO，避免 query 包依赖 V2 安装会话的完整内部类型。
// 使用：packageInstallConflictFromManifest 从 installed package 的 JSON manifest 中读取 Conflict。
// 边界：其它安装会话字段不在 query 包解释，恢复和清理由 v2 根包负责。
type installConflictManifest struct {
	// Conflict 职责：保存待前端处理的文件冲突详情；使用：进度快照恢复冲突弹窗；边界：不存在时为 nil。
	Conflict *contract.InstallConflictDTO `json:"conflict,omitempty"`
}

// ListInstalledPackageKeys 返回 V2 已安装包 key 列表。
// 职责：读取本地 installed package 表，返回可视为已安装的统一 packageKey。
// 使用：市场页同步统一来源包的安装状态。
// 边界：只返回已安装状态，不包含失败、运行中或冲突中的包。
func ListInstalledPackageKeys() ([]string, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0)
	if err := db.Model(&modDependenceModel.ModInstalledPackage{}).Where("install_state = ? OR install_state = '' OR install_state IS NULL", string(installConstant.StateInstalled)).Order("package_key ASC").Pluck("package_key", &keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// ListPackageInstallProgressSnapshots 返回本地已持久化的安装中、失败和待处理冲突状态。
// 职责：把 installed package 表中的临时安装状态转换为前端进度快照。
// 使用：v2/progress/snapshot.List 优先读取持久化快照时调用。
// 边界：只返回 running、failed、conflict；冲突详情只从安装 manifest 中解析。
func ListPackageInstallProgressSnapshots() ([]contract.InstallProgressSnapshot, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	records := make([]modDependenceModel.ModInstalledPackage, 0)
	if err := db.Where("install_state IN ?", []string{string(installConstant.StateRunning), string(installConstant.StateFailed), string(installConstant.StateConflict)}).Order("updated_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	snapshots := make([]contract.InstallProgressSnapshot, 0, len(records))
	for _, record := range records {
		snapshots = append(snapshots, contract.InstallProgressSnapshot{PackageKey: record.PackageKey, Stage: record.InstallStage, Status: record.InstallStatus, Percent: record.InstallProgress, BytesPerSecond: record.CurrentSpeed, Message: record.InstallStatus, State: record.InstallState, ErrorMessage: record.ErrorMessage, Conflict: packageInstallConflictFromManifest(record.InstallManifest), UpdatedAt: record.UpdatedAt})
	}
	return snapshots, nil
}

// packageInstallConflictFromManifest 从持久化 manifest 中恢复冲突 DTO。
// 职责：解析安装冲突详情，供前端恢复待处理冲突卡片。
// 使用：ListPackageInstallProgressSnapshots 转换 conflict 状态记录时调用。
// 边界：manifest 为空或解析失败时返回 nil，不阻断其它进度快照展示。
func packageInstallConflictFromManifest(raw datatypes.JSON) *contract.InstallConflictDTO {
	if len(raw) == 0 {
		return nil
	}
	manifest := installConflictManifest{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil
	}
	return manifest.Conflict
}
