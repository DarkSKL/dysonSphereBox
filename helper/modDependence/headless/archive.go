package headless

// 文件职责：规范和校验无头服务器的启动存档名称。
// 使用：保存配置与启动前检测共用，避免把任意路径交给游戏启动参数。
// 边界：只读取当前解析的游戏存档目录，不创建目录、不导入外部文件。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	environmentHelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/helper/modDependence/headless/contract"
)

// ListHeadlessArchives 职责：列出可作为启动存档的 SaveRoot 直属 .dsv；使用：开服配置的存档输入联想；边界：不递归子目录，目录缺失时返回错误，按修改时间倒序。
func ListHeadlessArchives() ([]contract.HeadlessArchiveOption, error) {
	root := (environmentHelper.Resolver{}).ResolveSaveRoot()
	if !root.Exists {
		return nil, fmt.Errorf("未找到游戏存档目录，请先在资源页确认存档目录")
	}
	entries, err := os.ReadDir(root.Path)
	if err != nil {
		return nil, fmt.Errorf("读取游戏存档目录失败: %w", err)
	}
	options := []contract.HeadlessArchiveOption{}
	modifiedAt := map[string]time.Time{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".dsv") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		modifiedAt[name] = info.ModTime()
		options = append(options, contract.HeadlessArchiveOption{Name: name, SizeBytes: info.Size(), LastModifiedAt: info.ModTime().Format(time.RFC3339)})
	}
	sort.Slice(options, func(i int, j int) bool { return modifiedAt[options[i].Name].After(modifiedAt[options[j].Name]) })
	return options, nil
}

// normalizeHeadlessArchive 职责：检查直属存档文件并返回不含扩展名的名称；使用：设置保存和环境检测；边界：空名保持默认加载模式，目录和外部路径均拒绝。
func normalizeHeadlessArchive(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", nil
	}
	if strings.ContainsAny(name, `\/:*?"<>|`) || name == "." || name == ".." {
		return "", fmt.Errorf("请填写存档名称；外部存档请先在资源页导入")
	}
	if strings.EqualFold(filepath.Ext(name), ".dsv") {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	root := (environmentHelper.Resolver{}).ResolveSaveRoot()
	if !root.Exists {
		return "", fmt.Errorf("未找到游戏存档目录，请在资源页确认存档目录")
	}
	info, err := os.Stat(filepath.Join(root.Path, name+".dsv"))
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("存档 %q 不存在或不可读取，请重新选择", name)
	}
	return name, nil
}
