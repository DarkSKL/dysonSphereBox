package manifest

// 文件职责：补全 V2 安装会话清理所需的 manifest 证据。
// 使用：清理入口读取 installed package 后调用，用数据库文件、组件关系和 Raw 包数据补齐清理计划。
// 边界：只补齐内存 manifest，不删除文件、不写数据库、不记录清理日志；查询失败时保留已有证据继续清理。
// 逻辑流：1. 读取包相关文件记录 => 2. 补齐已安装文件和目标目录 => 3. 读取组件记录补齐组件计划 => 4. 从 raw 包依赖补齐目录候选。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionutil"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"gorm.io/gorm"
	"path/filepath"
	"slices"
	"strings"
)

// Enrich 补全安装会话清理所需证据。
// 职责：从数据库文件、组件关系和 raw 包数据中补齐 manifest 的文件、组件和目录计划。
// 使用：sessioncleanup/entry.CleanupInterrupted 在执行清理前调用。
// 边界：数据库辅助查询失败时尽量返回已有 manifest，不阻断后续核心清理。
func Enrich(db *gorm.DB, record installedModel.ModInstalledPackage, manifest sessionstate.Manifest) sessionstate.Manifest {
	for _, fileRecord := range loadFileRecords(db, record.PackageKey) {
		if slices.Contains(manifest.ReusedComponents, fileRecord.ComponentKey) || manifest.IsReplacedComponent(fileRecord.ComponentKey) || !sessionsnapshot.BelongsToSession(fileRecord, manifest) {
			continue
		}
		if strings.TrimSpace(fileRecord.ComponentKey) != "" {
			manifest.InstalledComponents = sessionutil.AppendUniqueString(manifest.InstalledComponents, fileRecord.ComponentKey)
		}
		manifest.InstalledFiles = sessionsnapshot.AppendRecordFallbackFiles(manifest.InstalledFiles, []installedModel.ModInstalledFile{fileRecord})
		manifest.PlannedTargetDirs = sessionutil.AppendUniqueTargetDir(manifest.PlannedTargetDirs, filepath.Dir(fileRecord.TargetPath))
	}
	for _, component := range loadComponents(db, record.PackageKey, manifest.InstalledComponents) {
		if manifest.IsReplacedComponent(component.ComponentKey) {
			continue
		}
		manifest.InstalledComponents = sessionutil.AppendUniqueString(manifest.InstalledComponents, component.ComponentKey)
		manifest.PlannedComponents = sessionutil.AppendOrReplacePlannedComponent(manifest.PlannedComponents, sessionstate.ComponentPlan{
			ComponentKey: component.ComponentKey,
			InstallDir: v2Artifact.Artifact{
				ComponentKey: component.ComponentKey,
				Namespace:    component.Namespace,
				DisplayName:  component.DisplayName,
				Name:         component.Name,
			}.InstallDirName(),
		})
	}
	manifest = appendRawPackagePlans(record, manifest)
	return manifest
}

// loadFileRecords 读取清理时可参考的包文件记录。
// 职责：复用卸载文件查询逻辑，兼容历史依赖文件归属异常，并过滤其它包拥有的共享文件。
// 使用：Enrich 补全文件证据时调用。
// 边界：查询失败时返回 nil；借用文件及无法确认共享状态的非本包文件不进入清理证据。
func loadFileRecords(db *gorm.DB, packageKey string) []installedModel.ModInstalledFile {
	records, err := v2Uninstall.FindPackageFiles(db, packageKey)
	if err != nil {
		return nil
	}
	sessionRecords := make([]installedModel.ModInstalledFile, 0, len(records))
	for _, record := range records {
		if record.PackageKey != packageKey {
			shared, err := v2Uninstall.IsFileShared(db, packageKey, record)
			if err != nil || shared {
				continue
			}
		}
		sessionRecords = append(sessionRecords, record)
	}
	return sessionRecords
}

// loadComponents 读取清理时涉及的组件记录。
// 职责：合并 manifest 组件 key 与 package-component 关系后查询组件表。
// 使用：Enrich 补齐组件目录计划时调用。
// 边界：无 key 或查询失败时返回空列表，不影响文件清理。
func loadComponents(db *gorm.DB, packageKey string, componentKeys []string) []installedModel.ModInstalledComponent {
	keys := append([]string(nil), componentKeys...)
	relationKeys := make([]string, 0)
	if err := db.Model(&installedModel.ModInstalledPackageComponent{}).
		Where("package_key = ?", packageKey).
		Pluck("component_key", &relationKeys).Error; err == nil {
		for _, key := range relationKeys {
			keys = sessionutil.AppendUniqueString(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	components := make([]installedModel.ModInstalledComponent, 0)
	_ = db.Where("component_key IN ?", keys).Find(&components).Error
	return components
}

// appendRawPackagePlans 从原始包 DTO 补齐清理目录计划。
// 职责：解析 installed package Raw 字段，把主包和依赖转换为可能的安装目录候选。
// 使用：manifest 记录不完整但 raw 包仍存在时作为目录清理兜底。
// 边界：Raw 为空或解析失败时不修改 manifest；BepInEx 依赖会跳过。
func appendRawPackagePlans(record installedModel.ModInstalledPackage, manifest sessionstate.Manifest) sessionstate.Manifest {
	pack := modDependenceResponse.RemoteModPackageDTO{}
	if len(record.Raw) == 0 || json.Unmarshal(record.Raw, &pack) != nil {
		return manifest
	}
	manifest.PlannedComponents = sessionutil.AppendOrReplacePlannedComponent(manifest.PlannedComponents, sessionstate.ComponentPlan{
		ComponentKey: pack.Key,
		InstallDir: v2Artifact.Artifact{
			ComponentKey: pack.Key,
			Namespace:    pack.Namespace,
			DisplayName:  pack.DisplayName,
			Name:         pack.Name,
		}.InstallDirName(),
	})
	for _, dependency := range pack.Dependencies {
		if v2Artifact.IsBepInExDependencyKey(dependency.Key) {
			continue
		}
		manifest.PlannedComponents = sessionutil.AppendOrReplacePlannedComponent(manifest.PlannedComponents, sessionstate.ComponentPlan{
			ComponentKey: dependency.Key,
			InstallDir: v2Artifact.Artifact{
				ComponentKey: dependency.Key,
				Namespace:    dependency.Namespace,
				DisplayName:  dependency.DisplayName,
				Name:         dependency.Name,
			}.InstallDirName(),
		})
	}
	return manifest
}
