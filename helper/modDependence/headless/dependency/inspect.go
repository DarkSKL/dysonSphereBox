package dependency

// 文件职责：检测 Nebula 无头服务器所需的本地插件依赖状态。
// 使用：headless 环境编排在生成开服状态时调用 Inspect。
// 边界：只读取插件目录和 manifest.json，不下载、不安装、不修改文件，也不负责 BepInEx 或端口状态。

import (
	"path/filepath"
	"strings"

	"DysonSphereBox/helper/modDependence/headless/contract"
)

// Inspect 检查插件根目录中的开服依赖状态。
// 职责：按内置规格检查目录、manifest 和必需文件。
// 使用：headless.InspectHeadlessEnvironment 构造 contract.HeadlessEnvironmentStatus 时调用。
// 边界：pluginRoot 为空时只返回全部依赖缺失，不访问磁盘深层路径。
// 逻辑流：1. 遍历内置依赖规格 => 2. 定位插件目录和 manifest => 3. 检查目录存在 => 4. 读取 manifest 覆盖展示信息 => 5. 检查必需文件 => 6. 汇总安装状态。
func Inspect(pluginRoot string) ([]contract.HeadlessDependencyStatus, error) {
	results := make([]contract.HeadlessDependencyStatus, 0, len(specs))
	pluginRoot = strings.TrimSpace(pluginRoot)
	records, err := readInstalledComponents(pluginRoot)
	if err != nil {
		return nil, err
	}
	for _, item := range specs {
		pluginDir := ""
		manifestPath := ""
		if pluginRoot != "" {
			pluginDir = filepath.Join(pluginRoot, item.Folder)
			manifestPath = filepath.Join(pluginDir, "manifest.json")
		}
		result := contract.HeadlessDependencyStatus{
			ID:           item.ID,
			Name:         item.Name,
			Folder:       item.Folder,
			Function:     item.Function,
			Description:  item.Description,
			Source:       item.Source,
			PluginDir:    pluginDir,
			ManifestPath: manifestPath,
			MissingFiles: make([]string, 0),
		}

		if pluginRoot == "" {
			result.MissingFiles = append(result.MissingFiles, item.Folder)
			results = append(results, result)
			continue
		}

		if manifest, err := readManifest(result.ManifestPath); err == nil {
			if strings.TrimSpace(manifest.Name) != "" {
				result.Name = manifest.Name
			}
			result.InstalledVersion = strings.TrimSpace(manifest.VersionNumber)
			if strings.TrimSpace(manifest.Description) != "" {
				result.Description = manifest.Description
			}
			if strings.TrimSpace(manifest.WebsiteURL) != "" {
				result.Source = manifest.WebsiteURL
			}
		}

		files := localComponentFiles(pluginRoot, item.Folder)
		if record, ok := records[strings.ToLower(item.Folder)]; ok {
			result.InstalledVersion = record.version
			files = record.files
		}
		for _, requiredFile := range item.RequiredFiles {
			if !containsExistingFile(files, requiredFile) {
				result.MissingFiles = append(result.MissingFiles, requiredFile)
			}
		}
		if len(item.RequiredFiles) == 0 && !containsExistingDLL(files) {
			result.MissingFiles = append(result.MissingFiles, item.Folder+"/*.dll")
		}
		result.Installed = len(result.MissingFiles) == 0
		results = append(results, result)
	}

	return results, nil
}
