package copyconflict

// 文件职责：检测 V2 安装文件写入前的目标路径冲突。
// 使用：copyfiles.CopyPlannedFiles 在写入目标文件前调用 InspectFile。
// 边界：只做检测和 DTO 构造，不修改文件、不释放旧记录、不保存安装会话 manifest。

import (
	"fmt"
	"os"
	"path/filepath"

	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/filemeta"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// InspectFile 检查单个计划文件是否会与现有文件或目录冲突。
// 职责：识别目标目录冲突、非托管文件冲突和已由其它包管理的文件冲突。
// 使用：copyfiles.CopyPlannedFiles 在写入目标文件前调用，并根据返回 DTO 决定覆盖、跳过或暂停安装。
// 边界：只做检测和 DTO 构造，不修改文件、不释放旧记录；目标不存在时返回 nil。
// 逻辑流：1. 检查目标存在性 => 2. 目录冲突直接返回 DTO => 3. 哈希一致则无冲突 => 4. 查询其它包管理记录 => 5. 返回托管或非托管冲突 DTO。
func InspectFile(db *gorm.DB, file installplan.File, artifact v2Artifact.Artifact) (*contract.InstallConflictDTO, error) {
	info, err := os.Stat(file.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return buildDirectoryDTO(file, artifact), nil
	}

	existingHash, _, err := filemeta.SHA256(file.TargetPath)
	if err != nil {
		return nil, err
	}
	if existingHash == file.Sha256 {
		return nil, nil
	}

	managedRecord := modDependenceModel.ModInstalledFile{}
	result := db.
		Where("lower(target_path) IN ? AND package_key <> ?", installpath.ValuesForQuery(file.TargetPath), file.PackageKey).
		Take(&managedRecord)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return nil, result.Error
	}
	if result.Error == nil {
		return buildManagedDTO(db, file, artifact, existingHash, managedRecord), nil
	}

	var samePackageCount int64
	if err := db.Model(&modDependenceModel.ModInstalledFile{}).
		Where("lower(target_path) IN ? AND package_key = ?", installpath.ValuesForQuery(file.TargetPath), file.PackageKey).
		Count(&samePackageCount).Error; err != nil {
		return nil, err
	}
	if samePackageCount > 0 {
		return nil, nil
	}

	return &contract.InstallConflictDTO{
		PackageKey:       file.PackageKey,
		ComponentKey:     file.ComponentKey,
		ComponentName:    artifact.DisplayName,
		ComponentVersion: artifact.Version,
		TargetPath:       file.TargetPath,
		RelativePath:     file.RelativePath,
		IncomingSha256:   file.Sha256,
		ExistingSha256:   existingHash,
		Message:          fmt.Sprintf("%s 已存在且不是当前整合包记录的文件，需要选择覆盖、跳过或失败", filepath.Base(file.TargetPath)),
	}, nil
}
