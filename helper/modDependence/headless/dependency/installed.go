package dependency

// 文件职责：对齐 V2 安装组件与磁盘文件，兼容 plugins/patchers 分目录布局。
// 使用：Inspect 按 Owner-Name 获取已完成安装组件的版本与文件证据。
// 边界：只读当前游戏目录；不把安装中或失败记录当成就绪，不执行安装和修复。

import (
	"os"
	"path/filepath"
	"strings"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"DysonSphereBox/utils"
)

// installedComponent 职责：保存依赖版本与有效文件；使用：Inspect 合并磁盘检测；边界：仅包含当前目录的文件。
type installedComponent struct {
	// version 职责：保存已安装版本；使用：前端展示；边界：不推断远端版本。
	version string
	// files 职责：保存受管文件路径；使用：完整性检查；边界：检查时仍验证文件存在。
	files []string
}

// readInstalledComponents 职责：查询已完成的 V2 组件文件；使用：Inspect 优先识别新版安装结果；边界：无数据库时只允许磁盘检测，查询失败透传。
func readInstalledComponents(pluginRoot string) (map[string]installedComponent, error) {
	result := map[string]installedComponent{}
	if pluginRoot == "" || core.GLOBAL.DB == nil {
		return result, nil
	}
	bepInExRoot := filepath.Dir(pluginRoot)
	components := []installedModel.ModInstalledComponent{}
	err := core.GLOBAL.DB.Model(&installedModel.ModInstalledComponent{}).
		Joins("JOIN mod_installed_package_components r ON r.component_key = mod_installed_components.component_key").
		Joins("JOIN mod_installed_packages p ON p.package_key = r.package_key").
		Where("p.install_state = ? OR p.install_state = '' OR p.install_state IS NULL", string(installConstant.StateInstalled)).
		Distinct("mod_installed_components.*").Find(&components).Error
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(components))
	for _, component := range components {
		keys = append(keys, component.ComponentKey)
	}
	if len(keys) == 0 {
		return result, nil
	}
	files := []installedModel.ModInstalledFile{}
		if err := core.GLOBAL.DB.Model(&installedModel.ModInstalledFile{}).
		Joins("JOIN mod_installed_packages p ON p.package_key = mod_installed_files.package_key").
		Where("component_key IN ?", keys).
		Where("p.install_state = ? OR p.install_state = '' OR p.install_state IS NULL", string(installConstant.StateInstalled)).
		Find(&files).Error; err != nil {
		return nil, err
	}
	for _, component := range components {
		item := installedComponent{version: component.Version, files: []string{}}
		for _, file := range files {
			if file.ComponentKey != component.ComponentKey {
				continue
			}
			relative, err := filepath.Rel(bepInExRoot, file.TargetPath)
			scope := strings.SplitN(relative, string(filepath.Separator), 2)[0]
			if err == nil && !filepath.IsAbs(relative) && (strings.EqualFold(scope, "plugins") || strings.EqualFold(scope, "patchers")) {
				item.files = append(item.files, file.TargetPath)
			}
		}
		if len(item.files) > 0 {
			folder := artifactlayout.InstallDirName(artifactlayout.ArtifactDescriptor{Namespace: component.Namespace, Name: component.Name, DisplayName: component.DisplayName, ComponentKey: component.ComponentKey})
			result[strings.ToLower(folder)] = item
		}
	}
	return result, nil
}

// localComponentFiles 职责：扫描已知依赖作用域目录；使用：兼容手动安装与旧版记录；边界：只扫描同名 plugins/patchers 目录，不跟随目录符号链接。
func localComponentFiles(pluginRoot, folder string) []string {
	files := []string{}
	for _, root := range []string{pluginRoot, filepath.Join(filepath.Dir(pluginRoot), "patchers")} {
		_ = filepath.WalkDir(filepath.Join(root, folder), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if !entry.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}

// containsExistingFile 职责：按文件名验证必需 DLL；使用：Inspect 检查分目录组件；边界：目录和丢失文件不能满足条件。
func containsExistingFile(files []string, name string) bool {
	for _, path := range files {
		if strings.EqualFold(filepath.Base(path), name) && utils.FileExists(path) {
			return true
		}
	}
	return false
}

// containsExistingDLL 职责：验证辅助依赖具有实际 DLL；使用：Inspect 检查无必需文件列表的规格；边界：空目录与仅 manifest 不算安装。
func containsExistingDLL(files []string) bool {
	for _, path := range files {
		if strings.EqualFold(filepath.Ext(path), ".dll") && utils.FileExists(path) {
			return true
		}
	}
	return false
}
