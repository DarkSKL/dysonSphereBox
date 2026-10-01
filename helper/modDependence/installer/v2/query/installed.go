package query

// 文件职责：提供已安装 Mod 包摘要和安装器托管插件目录的只读查询。
// 使用：InstallerService.ListInstalledPackages 返回资源页市场模组列表；环境管理服务识别安装器托管目录，避免重复纳管或误删。
// 边界：只读取 installed 模型，不修改安装记录、不扫描磁盘、不访问远端市场。

import (
	"path"
	"sort"
	"strings"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/shared"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
)

// pluginRelativePrefix 职责：保存安装记录中插件目录的相对路径前缀；使用：提取 BepInEx/plugins 下的一级目录；边界：比较前统一转小写和正斜杠。
const pluginRelativePrefix = "bepinex/plugins/"

// ListInstalledPackageSummaries 返回本地已安装 Mod 包摘要。
// 职责：聚合安装包、组件关联和文件记录，生成资源页展示所需的包信息。
// 使用：InstallerService.ListInstalledPackages 调用。
// 边界：只返回已安装状态的包；文件统计以安装记录为准，不实时扫描磁盘。
// 逻辑流：1. 查询已安装包 => 2. 查询组件关联与组件元数据 => 3. 按包和组件聚合文件记录 => 4. 组装并排序摘要。
func ListInstalledPackageSummaries() ([]contract.InstalledPackageSummary, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}

	packages := make([]modDependenceModel.ModInstalledPackage, 0)
	if err := db.Where("install_state = ? OR install_state = '' OR install_state IS NULL", string(installConstant.StateInstalled)).Find(&packages).Error; err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return []contract.InstalledPackageSummary{}, nil
	}
	packageKeys := make([]string, 0, len(packages))
	for _, record := range packages {
		packageKeys = append(packageKeys, record.PackageKey)
	}

	relations := make([]modDependenceModel.ModInstalledPackageComponent, 0)
	if err := db.Where("package_key IN ?", packageKeys).Order("id ASC").Find(&relations).Error; err != nil {
		return nil, err
	}
	componentKeys := make([]string, 0, len(relations))
	for _, relation := range relations {
		componentKeys = append(componentKeys, relation.ComponentKey)
	}
	componentsByKey := map[string]modDependenceModel.ModInstalledComponent{}
	if len(componentKeys) > 0 {
		components := make([]modDependenceModel.ModInstalledComponent, 0)
		if err := db.Where("component_key IN ?", componentKeys).Find(&components).Error; err != nil {
			return nil, err
		}
		for _, component := range components {
			componentsByKey[component.ComponentKey] = component
		}
	}

	files := make([]modDependenceModel.ModInstalledFile, 0)
	if err := db.Select("package_key", "component_key", "relative_path", "file_size").Where("package_key IN ?", packageKeys).Find(&files).Error; err != nil {
		return nil, err
	}
	filesByPackage := map[string][]modDependenceModel.ModInstalledFile{}
	for _, file := range files {
		filesByPackage[file.PackageKey] = append(filesByPackage[file.PackageKey], file)
	}
	relationsByPackage := map[string][]string{}
	for _, relation := range relations {
		relationsByPackage[relation.PackageKey] = append(relationsByPackage[relation.PackageKey], relation.ComponentKey)
	}

	summaries := make([]contract.InstalledPackageSummary, 0, len(packages))
	for _, record := range packages {
		summaries = append(summaries, buildInstalledPackageSummary(record, relationsByPackage[record.PackageKey], componentsByKey, filesByPackage[record.PackageKey]))
	}
	sort.Slice(summaries, func(i int, j int) bool {
		return strings.ToLower(summaryTitle(summaries[i])) < strings.ToLower(summaryTitle(summaries[j]))
	})
	return summaries, nil
}

// ListManagedPluginDirNames 返回安装器托管的 BepInEx/plugins 一级目录名集合。
// 职责：从安装文件记录提取插件目录名，供环境管理识别由模组市场接管的目录。
// 使用：环境管理在扫描、首次纳管、启用和禁用 Mod 前调用。
// 边界：目录名统一转小写；包含安装中或冲突中包已写入的文件，宁可多保护也不误删。
func ListManagedPluginDirNames() (map[string]struct{}, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	relativePaths := make([]string, 0)
	if err := db.Model(&modDependenceModel.ModInstalledFile{}).Distinct("relative_path").Pluck("relative_path", &relativePaths).Error; err != nil {
		return nil, err
	}
	dirNames := make(map[string]struct{})
	for _, relativePath := range relativePaths {
		if dirName := pluginDirName(relativePath); dirName != "" {
			dirNames[strings.ToLower(dirName)] = struct{}{}
		}
	}
	return dirNames, nil
}

// buildInstalledPackageSummary 组装单个安装包摘要。
// 职责：统计包文件数量、大小、插件目录和组件明细。
// 使用：ListInstalledPackageSummaries 遍历安装包时调用。
// 边界：组件元数据缺失时用组件 key 兜底，不返回错误。
func buildInstalledPackageSummary(record modDependenceModel.ModInstalledPackage, componentKeys []string, componentsByKey map[string]modDependenceModel.ModInstalledComponent, files []modDependenceModel.ModInstalledFile) contract.InstalledPackageSummary {
	summary := contract.InstalledPackageSummary{
		PackageKey:  record.PackageKey,
		Source:      record.Source,
		Namespace:   record.Namespace,
		Name:        record.Name,
		DisplayName: record.DisplayName,
		Version:     record.Version,
		Author:      record.Author,
		Description: record.Description,
		PackageURL:  record.PackageURL,
		InstalledAt: record.InstalledAt,
		FileCount:   len(files),
		PluginDirs:  []string{},
		Components:  []contract.InstalledComponentSummary{},
	}

	fileCountByComponent := map[string]int{}
	seenDirs := map[string]struct{}{}
	for _, file := range files {
		summary.SizeBytes += file.FileSize
		fileCountByComponent[file.ComponentKey]++
		dirName := pluginDirName(file.RelativePath)
		if dirName == "" {
			continue
		}
		if _, ok := seenDirs[strings.ToLower(dirName)]; ok {
			continue
		}
		seenDirs[strings.ToLower(dirName)] = struct{}{}
		summary.PluginDirs = append(summary.PluginDirs, dirName)
	}
	sort.Strings(summary.PluginDirs)

	seenComponents := map[string]struct{}{}
	for _, componentKey := range componentKeys {
		if _, ok := seenComponents[componentKey]; ok {
			continue
		}
		seenComponents[componentKey] = struct{}{}
		component, ok := componentsByKey[componentKey]
		item := contract.InstalledComponentSummary{ComponentKey: componentKey, FileCount: fileCountByComponent[componentKey]}
		if ok {
			item.Namespace = component.Namespace
			item.Name = component.Name
			item.DisplayName = component.DisplayName
			item.Version = component.Version
		}
		summary.Components = append(summary.Components, item)
	}
	sort.SliceStable(summary.Components, func(i int, j int) bool {
		return isMainComponent(summary, summary.Components[i]) && !isMainComponent(summary, summary.Components[j])
	})
	return summary
}

// pluginDirName 从安装记录相对路径提取 BepInEx/plugins 下的一级目录名。
// 职责：兼容 Windows 反斜杠和正斜杠两种记录格式。
// 使用：插件目录集合和包摘要统计调用。
// 边界：不位于 plugins 子目录的文件返回空字符串，保留原始大小写。
func pluginDirName(relativePath string) string {
	normalized := path.Clean(strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/"))
	if !strings.HasPrefix(strings.ToLower(normalized), pluginRelativePrefix) {
		return ""
	}
	rest := normalized[len(pluginRelativePrefix):]
	segments := strings.SplitN(rest, "/", 2)
	if len(segments) < 2 || segments[0] == "" || segments[0] == "." || segments[0] == ".." {
		return ""
	}
	return segments[0]
}

// isMainComponent 判断组件是否为安装包主组件。
// 职责：让主包组件在详情中排在依赖之前。
// 使用：buildInstalledPackageSummary 排序组件时调用。
// 边界：只按命名空间和名称比较，不比较版本。
func isMainComponent(summary contract.InstalledPackageSummary, component contract.InstalledComponentSummary) bool {
	return strings.EqualFold(component.Namespace, summary.Namespace) && strings.EqualFold(component.Name, summary.Name)
}

// summaryTitle 返回安装包排序用名称。
// 职责：优先使用显示名，缺失时回退名称和包 key。
// 使用：ListInstalledPackageSummaries 排序时调用。
// 边界：只用于排序，不改写摘要字段。
func summaryTitle(summary contract.InstalledPackageSummary) string {
	for _, candidate := range []string{summary.DisplayName, summary.Name, summary.PackageKey} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}
