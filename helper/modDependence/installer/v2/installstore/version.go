package installstore

// 文件职责：提交同一 Mod 的版本替换和共享引用迁移。
// 使用：主包完成事务在清空 manifest 前调用。
// 边界：只操作调用方事务中的记录；物理文件由准备和回滚阶段处理。
// 逻辑流：1. 验证映射与新组件 => 2. 迁移独立安装入口 => 3. 合并共享引用 => 4. 删除旧清单与组件。

import (
	"encoding/json"
	"fmt"
	"strings"

	modDTO "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// commitVersionReplacements 合并旧版本的包组件关联并删除旧清单。
// 职责：使共享包使用真实新组件，避免最后卸载时漏删或误删文件。
// 使用：SaveRemotePackage 的完成事务调用。
// 边界：新组件缺失、身份不匹配或映射歧义返回错误，事务整体回滚。
func commitVersionReplacements(tx *gorm.DB, manifest sessionstate.Manifest) error {
	mappings := make(map[string]string, len(manifest.VersionReplacements))
	unique := make([]sessionstate.VersionReplacement, 0, len(manifest.VersionReplacements))
	for _, replacement := range manifest.VersionReplacements {
		oldKey, newKey := replacement.OldComponentKey, replacement.NewComponentKey
		if oldKey == "" || newKey == "" || oldKey == newKey {
			return fmt.Errorf("无效组件版本映射：%s => %s", oldKey, newKey)
		}
		if previous, exists := mappings[oldKey]; exists && previous != newKey {
			return fmt.Errorf("同一组件不能替换为多个版本：%s", oldKey)
		}
		if _, exists := mappings[oldKey]; !exists {
			unique = append(unique, replacement)
		}
		mappings[oldKey] = newKey
	}
	for _, replacement := range unique {
		if _, chained := mappings[replacement.NewComponentKey]; chained {
			return fmt.Errorf("安装计划包含同一 Mod 的多个替换版本：%s", replacement.NewComponentKey)
		}
		var oldComponent, newComponent installedModel.ModInstalledComponent
		if err := tx.Where("component_key = ?", replacement.OldComponentKey).Take(&oldComponent).Error; err != nil {
			return err
		}
		if err := tx.Where("component_key = ?", replacement.NewComponentKey).Take(&newComponent).Error; err != nil {
			return err
		}
		oldIdentity := artifactlayout.ModName(v2Artifact.FromInstalledComponent(oldComponent).Descriptor())
		newIdentity := artifactlayout.ModName(v2Artifact.FromInstalledComponent(newComponent).Descriptor())
		if oldIdentity == "" || !strings.EqualFold(oldIdentity, newIdentity) || !strings.EqualFold(oldComponent.Source, newComponent.Source) {
			return fmt.Errorf("组件版本映射身份不匹配：%s => %s", oldComponent.ComponentKey, newComponent.ComponentKey)
		}
		if err := migrateRootEntry(tx, oldComponent.ComponentKey, newComponent); err != nil {
			return err
		}
		closure := resolvedDependencyClosure(newComponent.ComponentKey, manifest.ResolvedDependencies)
		for _, key := range closure {
			if err := tx.Where("component_key = ?", key).Take(&installedModel.ModInstalledComponent{}).Error; err != nil {
				return fmt.Errorf("版本替换所需组件未安装：%s：%w", key, err)
			}
		}
		var relations []installedModel.ModInstalledPackageComponent
		if err := tx.Where("component_key = ?", oldComponent.ComponentKey).Find(&relations).Error; err != nil {
			return err
		}
		for _, relation := range relations {
			for _, key := range closure {
				updated := installedModel.ModInstalledPackageComponent{PackageKey: relation.PackageKey, ComponentKey: key, CreatedAt: relation.CreatedAt}
				if err := tx.Where("package_key = ? AND component_key = ?", updated.PackageKey, updated.ComponentKey).FirstOrCreate(&updated).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Where("component_key = ?", oldComponent.ComponentKey).Delete(&installedModel.ModInstalledPackageComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("component_key = ?", oldComponent.ComponentKey).Delete(&installedModel.ModInstalledFile{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&oldComponent).Error; err != nil {
			return err
		}
	}
	return nil
}

// resolvedDependencyClosure 展开实际安装版本的传递依赖。
// 职责：避免共享升级后遗漏新依赖的生命周期引用；使用：版本提交迁移旧使用方；边界：循环去重，历史 manifest 无依赖边时仅返回组件自身。
func resolvedDependencyClosure(root string, edges map[string][]string) []string {
	keys := []string{}
	seen := make(map[string]bool)
	var visit func(string)
	visit = func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		keys = append(keys, key)
		for _, child := range edges[key] {
			visit(child)
		}
	}
	visit(root)
	return keys
}

// migrateRootEntry 迁移旧组件对应的独立安装入口。
// 职责：避免依赖替换后仍把旧版本主包 key 展示为已安装。
// 使用：版本提交在迁移组件引用前调用。
// 边界：保留原有包的其它组件引用；已有新入口时合并关系，不覆盖当前安装主包状态。
func migrateRootEntry(tx *gorm.DB, oldKey string, component installedModel.ModInstalledComponent) error {
	var oldPackage installedModel.ModInstalledPackage
	if err := tx.Where("package_key = ?", oldKey).Take(&oldPackage).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	var target installedModel.ModInstalledPackage
	result := tx.Where("package_key = ?", component.ComponentKey).Take(&target)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	if result.Error == gorm.ErrRecordNotFound {
		target = oldPackage
		target.ID = 0
		target.PackageKey = component.ComponentKey
		target.SourcePackageID = component.Namespace + "-" + component.Name
		target.Namespace, target.Name, target.DisplayName = component.Namespace, component.Name, component.DisplayName
		target.Version, target.DownloadURL = component.Version, component.DownloadURL
		target.InstallManifest = datatypes.JSON([]byte("{}"))
		pack := modDTO.RemoteModPackageDTO{}
		if len(oldPackage.Raw) > 0 {
			if err := json.Unmarshal(oldPackage.Raw, &pack); err != nil {
				return err
			}
		}
		pack.Key, pack.Namespace, pack.Name, pack.DisplayName = target.PackageKey, target.Namespace, target.Name, target.DisplayName
		pack.Source, pack.SourcePackageID = sourceConstant.ModSource(component.Source), target.SourcePackageID
		pack.Version, pack.DownloadURL = target.Version, target.DownloadURL
		pack.Dependencies = v2Artifact.FromInstalledComponent(component).Dependencies
		raw, err := json.Marshal(pack)
		if err != nil {
			return err
		}
		target.Raw = datatypes.JSON(raw)
		if err := tx.Create(&target).Error; err != nil {
			return err
		}
	}
	var relations []installedModel.ModInstalledPackageComponent
	if err := tx.Where("package_key = ?", oldKey).Find(&relations).Error; err != nil {
		return err
	}
	for _, relation := range relations {
		updated := installedModel.ModInstalledPackageComponent{PackageKey: component.ComponentKey, ComponentKey: relation.ComponentKey, CreatedAt: relation.CreatedAt}
		if err := tx.Where("package_key = ? AND component_key = ?", updated.PackageKey, updated.ComponentKey).FirstOrCreate(&updated).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&installedModel.ModInstalledFile{}).Where("package_key = ?", oldKey).Update("package_key", component.ComponentKey).Error; err != nil {
		return err
	}
	if err := tx.Where("package_key = ?", oldKey).Delete(&installedModel.ModInstalledPackageComponent{}).Error; err != nil {
		return err
	}
	return tx.Delete(&oldPackage).Error
}
