package installstore

// 文件职责：读取普通 Mod 依赖复用所需的已安装组件快照。
// 使用：artifactresolve 在展开依赖前调用，按本地真实版本建立组件关联。
// 边界：只读取成功安装的 Thunderstore 组件，不查询市场、不修改组件记录。

import (
	installConstant "DysonSphereBox/constant/modDependence/install"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"strings"
)

// InstalledThunderstoreComponents 返回按 Owner-Name 索引的本地组件快照。
// 职责：保留实际安装版本、组件 key 和依赖清单，避免复用时关联到远端最新版本。
// 使用：普通 Mod 依赖解析在下载前调用。
// 边界：只包含稳定主包引用的组件；历史重复版本优先选择最近更新的组件，查询失败返回错误。
func InstalledThunderstoreComponents() (map[string]v2Artifact.Artifact, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	var components []installedModel.ModInstalledComponent
	err = db.Model(&installedModel.ModInstalledComponent{}).Distinct("mod_installed_components.*").
		Joins("JOIN mod_installed_package_components ON mod_installed_package_components.component_key = mod_installed_components.component_key").
		Joins("JOIN mod_installed_packages ON mod_installed_packages.package_key = mod_installed_package_components.package_key").
		Where("lower(mod_installed_components.source) = ?", string(sourceConstant.ModSourceThunderstore)).
		Where("mod_installed_packages.install_state = ? OR mod_installed_packages.install_state = '' OR mod_installed_packages.install_state IS NULL", string(installConstant.StateInstalled)).
		Order("mod_installed_components.updated_at DESC, mod_installed_components.component_key ASC").
		Find(&components).Error
	if err != nil {
		return nil, err
	}
	installed := make(map[string]v2Artifact.Artifact, len(components))
	for _, component := range components {
		artifact := v2Artifact.FromInstalledComponent(component)
		identity := strings.ToLower(artifactlayout.ModName(artifact.Descriptor()))
		if identity == "" {
			continue
		}
		if _, exists := installed[identity]; !exists {
			installed[identity] = artifact
		}
	}
	return installed, nil
}
