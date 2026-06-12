package copyconflict

// 文件职责：构造 V2 安装文件冲突 DTO。
// 使用：InspectFile 在发现目录冲突或其它包管理冲突时调用。
// 边界：只补齐前端展示所需信息，不修改数据库、不删除文件、不决定用户动作。

import (
	"fmt"
	"path/filepath"

	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// buildDirectoryDTO 构造目标路径为目录时的冲突 DTO。
// 职责：把目录冲突表达为前端可选择的安装冲突信息。
// 使用：InspectFile 发现目标路径存在且为目录时调用。
// 边界：不尝试删除或移动目录；目录冲突通常需要用户终止或调整包结构。
func buildDirectoryDTO(file installplan.File, artifact v2Artifact.Artifact) *contract.InstallConflictDTO {
	return &contract.InstallConflictDTO{
		PackageKey:       file.PackageKey,
		ComponentKey:     file.ComponentKey,
		ComponentName:    artifact.DisplayName,
		ComponentVersion: artifact.Version,
		TargetPath:       file.TargetPath,
		RelativePath:     file.RelativePath,
		IncomingSha256:   file.Sha256,
		Message:          fmt.Sprintf("%s 已存在且是目录，不能直接作为文件覆盖；通常是旧版本路径大小写冲突或包内目录结构异常", filepath.Base(file.TargetPath)),
	}
}

// buildManagedDTO 构造目标文件已被其它包管理时的冲突 DTO。
// 职责：补齐既有包和组件名称，帮助前端展示覆盖风险。
// 使用：InspectFile 发现目标路径归属其它安装包时调用。
// 边界：名称读取失败时保留 key 信息，不阻断冲突返回。
func buildManagedDTO(db *gorm.DB, file installplan.File, artifact v2Artifact.Artifact, existingHash string, managedRecord modDependenceModel.ModInstalledFile) *contract.InstallConflictDTO {
	conflict := &contract.InstallConflictDTO{
		PackageKey:           file.PackageKey,
		ComponentKey:         file.ComponentKey,
		ComponentName:        artifact.DisplayName,
		ComponentVersion:     artifact.Version,
		TargetPath:           file.TargetPath,
		RelativePath:         file.RelativePath,
		IncomingSha256:       file.Sha256,
		ExistingSha256:       existingHash,
		ExistingPackageKey:   managedRecord.PackageKey,
		ExistingComponentKey: managedRecord.ComponentKey,
		Message:              fmt.Sprintf("%s 已由其它整合包管理，当前版本不同，需要选择覆盖、跳过或失败", filepath.Base(file.TargetPath)),
	}

	installedPackage := modDependenceModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", managedRecord.PackageKey).Take(&installedPackage).Error; err == nil {
		conflict.ExistingPackageName = installedPackage.DisplayName
	}
	installedComponent := modDependenceModel.ModInstalledComponent{}
	if err := db.Where("component_key = ?", managedRecord.ComponentKey).Take(&installedComponent).Error; err == nil {
		conflict.ExistingComponentName = installedComponent.DisplayName
		conflict.ExistingComponentVersion = installedComponent.Version
	}
	return conflict
}
