package environment

// 文件职责：协调环境管理 Mod 写入与模组市场安装器之间的目录归属和串行保护。
// 使用：EnvironmentManagerService 的 Mod 列表、首次纳管、启用、禁用和删除流程调用。
// 边界：只读取安装记录并拒绝冲突操作，不修改安装器记录、不调用安装或卸载流程。

import (
	"fmt"

	"DysonSphereBox/core"
	environmentHelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/helper/modDependence/installer/v2/operationguard"
	v2Query "DysonSphereBox/helper/modDependence/installer/v2/query"
)

// installerManagedModDirs 读取模组市场安装器托管的插件目录集合。
// 职责：把安装记录中的 BepInEx/plugins 一级目录转换为小写目录名集合。
// 使用：ListMods、首次纳管、启用、禁用和删除前调用。
// 边界：数据库未初始化时返回空集合，保持只读列表可用。
func (service *EnvironmentManagerService) installerManagedModDirs() (map[string]struct{}, error) {
	if core.GLOBAL.DB == nil {
		return map[string]struct{}{}, nil
	}
	return v2Query.ListManagedPluginDirNames()
}

// rejectInstallerManagedMod 拒绝对安装器托管目录执行环境同步。
// 职责：防止环境管理复制或删除模组市场安装器已登记的游戏侧文件。
// 使用：EnableMod、DisableMod 在真实文件操作前调用。
// 边界：只按资源库目录名判断；读取安装记录失败时返回错误，不放行写入。
func (service *EnvironmentManagerService) rejectInstallerManagedMod(libraryPath string) error {
	managedDirs, err := service.installerManagedModDirs()
	if err != nil {
		return fmt.Errorf("读取模组安装记录失败: %w", err)
	}
	dirName := environmentHelper.ModLibraryDirName(libraryPath)
	if environmentHelper.IsInstallerManagedDir(managedDirs, dirName) {
		return fmt.Errorf("游戏侧目录 %s 已由模组市场安装管理，请在资源页“已安装”分区或模组页卸载", dirName)
	}
	return nil
}

// modLibraryPathByID 按资源 ID 查找资源库 Mod 相对路径。
// 职责：为禁用前的托管目录判断提供资源库路径。
// 使用：disableModWithoutOperation 调用。
// 边界：资源库中已不存在时返回错误，不继续删除游戏侧文件。
func (service *EnvironmentManagerService) modLibraryPathByID(itemId string) (string, error) {
	libraryRoot, err := environmentHelper.Resolver{}.LibraryRoot("plugins")
	if err != nil {
		return "", err
	}
	mods, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		return "", err
	}
	for _, mod := range mods {
		if mod.ID == itemId {
			return mod.LibraryPath, nil
		}
	}
	return "", fmt.Errorf("未找到 Mod 资源")
}

// withModTargetGuard 在安装器串行保护内执行写入游戏侧 Mod 目录的操作。
// 职责：让环境启用、禁用、删除与模组安装、卸载、BepInEx 操作互斥。
// 使用：EnableMod、DisableMod、DeleteMod 的业务闭包外层调用。
// 边界：保护忙碌时直接返回错误，不排队；不支持在闭包内再次获取。
func (service *EnvironmentManagerService) withModTargetGuard(action func() (EnvironmentOperationResult, error)) (EnvironmentOperationResult, error) {
	release, err := operationguard.Acquire()
	if err != nil {
		return EnvironmentOperationResult{}, err
	}
	defer release()
	return action()
}
