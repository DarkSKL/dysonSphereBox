package utils

// 文件职责：提供 Steam 库配置、应用 manifest 和 VDF 文本的轻量解析工具。
// 使用：戴森球计划自动路径探测通过 Steam 根目录、libraryfolders.vdf 和 appmanifest 定位安装目录时调用。
// 边界：只读取和解析本地 Steam 配置文件，不启动 Steam、不访问网络、不写入业务数据库。

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReadSteamLibraryFolders 读取 Steam libraryfolders.vdf 中的库目录。
// 职责：从 Steam 根目录读取所有库目录配置。
// 使用：自动定位戴森球计划安装目录时，从默认 Steam 根目录扩展出所有库路径。
// 边界：Steam 根目录为空时返回空结果；新旧配置路径都会尝试读取；读取失败返回原始错误。
func ReadSteamLibraryFolders(steamRoot string) ([]string, error) {
	if steamRoot == "" {
		return nil, nil
	}

	content, err := os.ReadFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"))
	if os.IsNotExist(err) {
		content, err = os.ReadFile(filepath.Join(steamRoot, "config", "libraryfolders.vdf"))
	}
	if err != nil {
		return nil, err
	}

	return ParseSteamLibraryFolders(string(content)), nil
}

// ParseSteamLibraryFolders 从 Steam libraryfolders.vdf 内容解析库目录。
// 职责：兼容 Steam 新旧 VDF 格式并提取库目录路径。
// 使用：ReadSteamLibraryFolders 读取文件内容后调用，也可在测试中直接传入 VDF 文本。
// 边界：只解析路径字段，不检查目录是否存在；重复路径会按大小写不敏感规则去重。
// 逻辑流：1. 解析新格式 path 字段 => 2. 解析旧格式数字键路径 => 3. 清理并去重路径 => 4. 返回候选库目录。
func ParseSteamLibraryFolders(content string) []string {
	paths := make([]string, 0)
	seen := make(map[string]struct{})

	for _, path := range ParseSteamVdfValues(content, "path") {
		AppendUniquePath(&paths, seen, path)
	}

	legacyLibraryExpression := regexp.MustCompile(`"\d+"\s+"([^"]+)"`)
	for _, match := range legacyLibraryExpression.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}

		AppendUniquePath(&paths, seen, DecodeSteamVdfPath(match[1]))
	}

	return paths
}

// ReadSteamAppInstallDir 读取指定 Steam 应用在库目录中的安装路径。
// 职责：根据 Steam appmanifest 解析指定应用的安装目录。
// 使用：dysonsphere.FindDir 遍历 Steam 库目录时调用，用于定位 DSPGAME.exe 所在 common 子目录。
// 边界：库目录或应用 ID 为空时返回空路径；manifest 不存在时透传读取错误给调用方判断是否跳过。
func ReadSteamAppInstallDir(libraryRoot string, appID string) (string, error) {
	if libraryRoot == "" || appID == "" {
		return "", nil
	}

	content, err := os.ReadFile(filepath.Join(libraryRoot, "steamapps", "appmanifest_"+appID+".acf"))
	if err != nil {
		return "", err
	}

	values := ParseSteamVdfValues(string(content), "installdir")
	if len(values) == 0 {
		return "", nil
	}

	return filepath.Join(libraryRoot, "steamapps", "common", values[0]), nil
}

// ParseSteamVdfValues 从 VDF 文本中提取指定 key 的字符串值。
// 职责：以轻量正则从 Steam VDF 文本中提取指定键的字符串值。
// 使用：ParseSteamLibraryFolders 和 ReadSteamAppInstallDir 解析 path、installdir 等字段时调用。
// 边界：key 为空时返回空切片；返回值会解码 Steam 路径转义；不实现完整 VDF 语法解析。
func ParseSteamVdfValues(content string, key string) []string {
	values := make([]string, 0)
	if key == "" {
		return values
	}

	expression := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s+"([^"]+)"`)
	for _, match := range expression.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}

		values = append(values, DecodeSteamVdfPath(match[1]))
	}

	return values
}

// DecodeSteamVdfPath 解码 Steam VDF 路径中的反斜杠转义。
// 职责：把 Steam VDF 中的双反斜杠路径还原为普通文件系统路径。
// 使用：解析 libraryfolders 和 appmanifest 路径字段后统一调用。
// 边界：只处理反斜杠转义，不清理空白、不展开环境变量。
func DecodeSteamVdfPath(path string) string {
	return strings.ReplaceAll(path, `\\`, `\`)
}
