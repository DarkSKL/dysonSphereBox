package installstore

// 文件职责：保存 V2 安装器的组件及文件清单。
// 使用：远端安装流程在单个 artifact 文件复制完成后调用，用于建立组件、包组件关系和文件归属。
// 边界：只负责组件相关数据库写入，不下载、不解压、不复制文件，也不记录服务层审计日志。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// SaveArtifact 保存单个组件及其安装文件清单。
// 职责：upsert 组件记录、维护 package-component 关系，并写入文件归属记录。
// 使用：每个 artifact 文件复制成功后调用。
// 边界：必须在文件已经写入后调用；数据库失败时返回错误交给上层清理安装会话。
// 逻辑流：1. 构造组件模型 => 2. upsert 组件 => 3. 确保包组件关系 => 4. upsert 文件清单 => 5. 提交事务。
func SaveArtifact(pack modDependenceResponse.RemoteModPackageDTO, artifact v2Artifact.Artifact, files []installplan.File) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}

	componentRaw, _ := json.Marshal(artifact)
	component := modDependenceModel.ModInstalledComponent{
		ComponentKey:       artifact.ComponentKey,
		Source:             string(artifact.Source),
		SourceDependencyID: artifact.ComponentKey,
		Name:               artifact.Name,
		DisplayName:        artifact.DisplayName,
		Version:            artifact.Version,
		DownloadURL:        artifact.DownloadURL,
		ArtifactKind:       string(artifact.ArtifactKind),
		LocalPath:          artifact.LocalPath,
		Raw:                datatypes.JSON(componentRaw),
		UpdatedAt:          time.Now(),
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "component_key"}}, UpdateAll: true}).Create(&component).Error; err != nil {
			return err
		}

		relation := modDependenceModel.ModInstalledPackageComponent{
			PackageKey:   pack.Key,
			ComponentKey: artifact.ComponentKey,
			CreatedAt:    time.Now(),
		}
		if err := tx.Where("package_key = ? AND component_key = ?", relation.PackageKey, relation.ComponentKey).
			FirstOrCreate(&relation).Error; err != nil {
			return err
		}

		for _, file := range files {
			record := modDependenceModel.ModInstalledFile{
				PackageKey:   file.PackageKey,
				ComponentKey: file.ComponentKey,
				SourcePath:   file.SourcePath,
				TargetPath:   file.TargetPath,
				RelativePath: file.RelativePath,
				Sha256:       file.Sha256,
				FileSize:     file.FileSize,
				CreatedAt:    time.Now(),
			}
			if err := tx.Where("package_key = ? AND lower(target_path) IN ?", file.PackageKey, installpath.ValuesForQuery(file.TargetPath)).
				Assign(record).
				FirstOrCreate(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
