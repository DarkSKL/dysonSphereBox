package environment

// 文件职责：识别本地 Mod 压缩包解压后的有效内容根目录。
// 使用：环境服务导入 zip 时调用，返回复制到资源库的目录及名称。
// 边界：只读目录结构，不解压、不复制、不写数据库，不处理 patchers/config 的安装落位。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverModRootFromExtractedZip 识别 zip 解压后的目录型 Mod 根。
// 职责：兼容 plugins 子目录、单个包根目录或根目录即 Mod 文件集的结构。
// 使用：环境服务准备导入源时调用，目录名回退到原始 zip 文件名。
// 边界：多个 plugins 子目录拒绝自动合并；无法识别时返回错误，不创建资源库项目。
// 逻辑流：1. 优先解析显式 plugins 布局 => 2. 按 manifest 或 DLL 识别根文件集 => 3. 尝试单个外层目录 => 4. 普通根文件兜底，否则报错。
func DiscoverModRootFromExtractedZip(extractRoot string, sourceZipPath string) (string, string, error) {
	pluginsRoot := filepath.Join(extractRoot, "plugins")
	if dirExists(pluginsRoot) {
		entries, err := os.ReadDir(pluginsRoot)
		if err != nil {
			return "", "", err
		}
		if directoryHasModFiles(entries) {
			return pluginsRoot, modImportArchiveName(extractRoot, sourceZipPath), nil
		}
		dirs := []os.DirEntry{}
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry)
			}
		}
		if len(dirs) == 1 {
			return filepath.Join(pluginsRoot, dirs[0].Name()), dirs[0].Name(), nil
		}
		if len(dirs) > 1 {
			return "", "", fmt.Errorf("zip 包含多个 Mod 目录，请分别导入目录")
		}
		if directoryHasRegularFile(entries) {
			return pluginsRoot, modImportArchiveName(extractRoot, sourceZipPath), nil
		}
	}
	entries, err := os.ReadDir(extractRoot)
	if err != nil {
		return "", "", err
	}
	// 根层已有包清单或 DLL 时，子目录属于同一个包，不能将唯一附属目录误当作包根。
	if directoryHasModFiles(entries) {
		return extractRoot, modImportArchiveName(extractRoot, sourceZipPath), nil
	}
	dirs := []os.DirEntry{}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "patchers" && entry.Name() != "config" {
			dirs = append(dirs, entry)
		}
	}
	if len(dirs) == 1 {
		return filepath.Join(extractRoot, dirs[0].Name()), dirs[0].Name(), nil
	}
	if directoryHasRegularFile(entries) {
		return extractRoot, modImportArchiveName(extractRoot, sourceZipPath), nil
	}
	return "", "", fmt.Errorf("未在 zip 包中识别到目录型 Mod")
}

// 职责：从普通文件中识别包清单或 DLL；使用：下降到唯一子目录之前确认当前层是否为包根；边界：不解析清单内容，不验证 DLL 格式。
func directoryHasModFiles(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.Type().IsRegular() && (strings.EqualFold(entry.Name(), "manifest.json") || strings.EqualFold(filepath.Ext(entry.Name()), ".dll")) {
			return true
		}
	}
	return false
}

// 职责：判断当前层是否包含普通文件；使用：根目录文件集识别兜底；边界：不递归，不把符号链接视为普通文件。
func directoryHasRegularFile(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			return true
		}
	}
	return false
}

// 职责：为无外层目录的包生成资源库目录名；使用：根目录文件集导入；边界：保留下载副本后缀，空名称回退解压目录名。
func modImportArchiveName(extractRoot string, sourceZipPath string) string {
	name := strings.TrimSuffix(filepath.Base(sourceZipPath), filepath.Ext(sourceZipPath))
	if name == "" {
		return filepath.Base(extractRoot)
	}
	return name
}
