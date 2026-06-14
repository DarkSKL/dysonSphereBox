package environment

// 文件职责：提供环境管理页使用的系统文件和目录选择器。
// 使用：前端导入 Mod、蓝图、存档或导出资源时调用这些 Wails 绑定方法获取路径。
// 边界：只负责打开系统对话框并返回路径，不复制文件、不写数据库、不决定资源状态。

import (
	"fmt"
	"os"

	"DysonSphereBox/core"
	environmentHelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SelectModImportPaths 打开 Mod 导入路径选择器。
// 职责：优先选择 zip 包文件，供前端继续调用 ImportMod；目录型 Mod 可通过 SelectModImportDirectory 选择。
// 使用：环境页点击“导入 Mod”时调用。
// 边界：用户取消时返回 Canceled=true；不验证 zip 内容是否为有效 Mod。
func (service *EnvironmentManagerService) SelectModImportPaths() (EnvironmentPathSelection, error) {
	paths, err := openMultipleEnvironmentFilesDialog("选择 Mod 压缩包", []runtime.FileFilter{
		{DisplayName: "Mod 压缩包 (*.zip)", Pattern: "*.zip"},
		{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
	})
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return EnvironmentPathSelection{Paths: paths, Canceled: len(paths) == 0}, nil
}

// SelectModImportDirectory 打开目录型 Mod 选择器。
// 职责：选择一个目录型 Mod 根目录，供前端继续调用 ImportMod。
// 使用：环境页点击“导入 Mod 目录”时调用。
// 边界：用户取消时返回空路径；不判断目录内是否包含 manifest 或插件文件。
func (service *EnvironmentManagerService) SelectModImportDirectory() (EnvironmentPathSelection, error) {
	path, err := openEnvironmentDirectoryDialog("选择 Mod 目录")
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return singlePathSelection(path), nil
}

// SelectBlueprintImportFiles 打开蓝图文件导入选择器。
// 职责：选择 .txt、.bp 或 .json 蓝图文件，供前端继续调用 ImportBlueprintFiles。
// 使用：环境页蓝图标签点击导入时调用。
// 边界：用户取消时返回 Canceled=true；文件内容校验仍由导入流程负责。
func (service *EnvironmentManagerService) SelectBlueprintImportFiles() (EnvironmentPathSelection, error) {
	paths, err := openMultipleEnvironmentFilesDialog("选择蓝图文件", []runtime.FileFilter{
		{DisplayName: "蓝图文件 (*.txt;*.bp;*.json)", Pattern: "*.txt;*.bp;*.json"},
		{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
	})
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return EnvironmentPathSelection{Paths: paths, Canceled: len(paths) == 0}, nil
}

// SelectBlueprintRootDirectory 打开真实蓝图根目录选择器。
// 职责：让用户在自动探测失败或需要覆盖默认路径时选择蓝图根目录。
// 使用：环境页蓝图目录未确认或用户主动更换目录时调用。
// 边界：只返回目录路径，不写配置；保存必须继续调用 SetEnvironmentRootDirectory。
func (service *EnvironmentManagerService) SelectBlueprintRootDirectory() (EnvironmentPathSelection, error) {
	path, err := openEnvironmentDirectoryDialog("选择蓝图根目录")
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return singlePathSelection(path), nil
}

// SelectSaveImportFiles 打开存档文件导入选择器。
// 职责：选择一个或多个存档文件，供前端继续调用 ImportSaves。
// 使用：环境页存档标签点击导入时调用。
// 边界：用户取消时返回 Canceled=true；不解析存档内部结构。
func (service *EnvironmentManagerService) SelectSaveImportFiles() (EnvironmentPathSelection, error) {
	paths, err := openMultipleEnvironmentFilesDialog("选择存档文件", []runtime.FileFilter{
		{DisplayName: "戴森球计划存档 (*.dsv)", Pattern: "*.dsv"},
		{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
	})
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return EnvironmentPathSelection{Paths: paths, Canceled: len(paths) == 0}, nil
}

// SelectSaveRootDirectory 打开真实存档根目录选择器。
// 职责：让用户在自动探测失败或需要覆盖默认路径时选择存档根目录。
// 使用：环境页存档目录未找到或用户主动更换目录时调用。
// 边界：只返回目录路径，不写配置；保存必须继续调用 SetEnvironmentRootDirectory。
func (service *EnvironmentManagerService) SelectSaveRootDirectory() (EnvironmentPathSelection, error) {
	path, err := openEnvironmentDirectoryDialog("选择存档根目录")
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return singlePathSelection(path), nil
}

// SelectEnvironmentExportDirectory 打开资源导出目录选择器。
// 职责：选择蓝图或存档导出的目标目录。
// 使用：ExportBlueprint 和 ExportSave 前由前端调用。
// 边界：只返回目录路径，不创建导出文件，不覆盖目标文件。
func (service *EnvironmentManagerService) SelectEnvironmentExportDirectory() (EnvironmentPathSelection, error) {
	path, err := openEnvironmentDirectoryDialog("选择导出目标目录")
	if err != nil {
		return EnvironmentPathSelection{}, err
	}
	return singlePathSelection(path), nil
}

// SetEnvironmentRootDirectory 保存蓝图或存档真实根目录。
// 职责：校验用户选择的真实目录并写入 Game 分组配置，然后返回最新解析状态。
// 使用：环境页选择蓝图或存档根目录后调用，作为自动探测失败的手动兜底。
// 边界：不创建目录、不迁移资源、不修改游戏启动路径；kind 只允许 blueprint/save。
func (service *EnvironmentManagerService) SetEnvironmentRootDirectory(kind string, path string) (EnvironmentRootDirectoryResult, error) {
	normalizedKind, configKey, err := normalizeEnvironmentRootKind(kind)
	if err != nil {
		return EnvironmentRootDirectoryResult{}, err
	}
	cleanPath := utils.CleanPath(path)
	if cleanPath == "" {
		return EnvironmentRootDirectoryResult{}, fmt.Errorf("目录不能为空")
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return EnvironmentRootDirectoryResult{}, fmt.Errorf("目录不可访问: %w", err)
	}
	if !info.IsDir() {
		return EnvironmentRootDirectoryResult{}, fmt.Errorf("请选择文件夹目录")
	}
	if err := core.InitConfigUnit(environmentHelper.ConfigGroupGame, configKey).Set(cleanPath); err != nil {
		return EnvironmentRootDirectoryResult{}, fmt.Errorf("保存环境目录失败: %w", err)
	}
	resolver := environmentHelper.Resolver{}
	root := resolver.ResolveBlueprintRoot()
	if normalizedKind == "save" {
		root = resolver.ResolveSaveRoot()
	}
	overview, err := service.getOverviewWithoutRecovery()
	if err != nil {
		return EnvironmentRootDirectoryResult{}, err
	}
	return EnvironmentRootDirectoryResult{
		Kind:     normalizedKind,
		Root:     root,
		Overview: overview,
	}, nil
}

// openMultipleEnvironmentFilesDialog 打开多文件选择器。
// 职责：封装 RuntimeContext 校验和 Wails 多文件对话框调用。
// 使用：环境导入文件选择方法复用。
// 边界：RuntimeContext 缺失时返回错误；用户取消不视为错误。
func openMultipleEnvironmentFilesDialog(title string, filters []runtime.FileFilter) ([]string, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return nil, fmt.Errorf("app context is not initialized")
	}
	paths, err := runtime.OpenMultipleFilesDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// openEnvironmentDirectoryDialog 打开目录选择器。
// 职责：封装 RuntimeContext 校验和 Wails 目录对话框调用。
// 使用：Mod 目录导入和导出目录选择方法复用。
// 边界：RuntimeContext 缺失时返回错误；用户取消时返回空字符串。
func openEnvironmentDirectoryDialog(title string) (string, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("app context is not initialized")
	}
	return runtime.OpenDirectoryDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{
		Title:                title,
		CanCreateDirectories: true,
	})
}

// singlePathSelection 将单路径转换为选择器 DTO。
// 职责：统一目录选择取消和成功返回结构。
// 使用：目录型选择器方法调用。
// 边界：空路径只表示取消，不做错误处理。
func singlePathSelection(path string) EnvironmentPathSelection {
	if path == "" {
		return EnvironmentPathSelection{Paths: []string{}, Canceled: true}
	}
	return EnvironmentPathSelection{Paths: []string{path}, Canceled: false}
}

// normalizeEnvironmentRootKind 归一化可手动配置的环境根目录类型。
// 职责：把前端传入的目录类型转换为稳定 kind 和配置键。
// 使用：SetEnvironmentRootDirectory 保存配置前调用。
// 边界：拒绝 mod/all 等非手动目录类型，游戏目录仍由设置页维护。
func normalizeEnvironmentRootKind(kind string) (string, string, error) {
	switch kind {
	case "blueprint", "blueprints":
		return "blueprint", environmentHelper.ConfigKeyBlueprintDirPath, nil
	case "save", "saves":
		return "save", environmentHelper.ConfigKeySaveDirPath, nil
	default:
		return "", "", fmt.Errorf("不支持的环境目录类型: %s", kind)
	}
}
