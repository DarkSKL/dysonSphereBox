package config

// 文件职责：读取和修复 BepInEx Doorstop 配置及本地版本信息。
// 使用：bepinex/status 检测状态时读取版本与配置，bepinex/install 安装或跳过时修复配置。
// 边界：只处理 doorstop_config.ini 与版本文本，不下载 BepInEx、不复制文件、不清理安装记录。

import (
	"DysonSphereBox/helper/modDependence/bepinex/winver"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReadBepInExVersion 读取本地 BepInEx 版本文本。
// 职责：优先读取 DLL 文件版本，失败时回退到 .doorstop_version。
// 使用：status.CheckBepInExStatus 构造状态快照时调用。
// 边界：读取失败返回空字符串，不阻断状态检测。
func ReadBepInExVersion(gameDir string) string {
	if version := winver.ReadWindowsFileVersion(filepath.Join(gameDir, "BepInEx", "core", "BepInEx.dll")); version != "" {
		return version
	}

	content, err := os.ReadFile(filepath.Join(gameDir, ".doorstop_version"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

// IsDoorstopConfigValid 判断 Doorstop 配置是否启用并指向 BepInEx Preloader。
// 职责：检测已有 doorstop_config.ini 是否可用于加载 BepInEx。
// 使用：status.CheckBepInExStatus 和安装跳过判断时调用。
// 边界：读取或解析失败时返回 false，由安装流程决定是否修复。
func IsDoorstopConfigValid(gameDir string) bool {
	config, err := readDoorstopConfig(gameDir)
	if err != nil {
		return false
	}

	return strings.EqualFold(config["enabled"], "true") &&
		strings.EqualFold(filepath.Clean(config["targetassembly"]), filepath.Clean(filepath.Join("BepInEx", "core", "BepInEx.Preloader.dll")))
}

// readDoorstopConfig 解析 doorstop_config.ini 的简单键值配置。
// 职责：提取 enabled、targetAssembly 等 Doorstop 设置。
// 使用：isDoorstopConfigValid 校验配置有效性时调用。
// 边界：忽略空行、注释和 section，不保留原始顺序或重复键。
func readDoorstopConfig(gameDir string) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Join(gameDir, "doorstop_config.ini"))
	if err != nil {
		return nil, err
	}

	config := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		config[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.Trim(strings.TrimSpace(parts[1]), `"`)
	}

	return config, nil
}

// EnsureDoorstopConfig 修复 Doorstop 启用状态和目标程序集路径。
// 职责：确保 doorstop_config.ini 启用并指向 BepInEx Preloader。
// 使用：install.InstallBepInEx 在已安装或复制完文件后调用。
// 边界：只更新 enabled 与 targetAssembly，不改写其他配置项；文件不存在时返回错误。
// 逻辑流：1. 读取配置文件 => 2. 写入 enabled=true => 3. 写入 targetAssembly => 4. 内容变化时覆盖保存。
func EnsureDoorstopConfig(gameDir string) error {
	cfgPath := filepath.Join(gameDir, "doorstop_config.ini")
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	nextContent := upsertDoorstopSetting(string(content), "enabled", "true")
	nextContent = upsertDoorstopSetting(nextContent, "targetAssembly", `BepInEx\core\BepInEx.Preloader.dll`)
	if nextContent == string(content) {
		return nil
	}

	return os.WriteFile(cfgPath, []byte(nextContent), 0o644)
}

// upsertDoorstopSetting 更新或追加单个 Doorstop 配置项。
// 职责：在保留原文件主体的前提下修改指定键。
// 使用：ensureDoorstopConfig 修复 enabled 和 targetAssembly 时调用。
// 边界：按大小写不敏感匹配键；不存在时追加到文件末尾。
func upsertDoorstopSetting(content string, key string, value string) string {
	pattern := regexp.MustCompile(`(?mi)^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*).*$`)
	replacement := "${1}" + value
	if pattern.MatchString(content) {
		return pattern.ReplaceAllString(content, replacement)
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\r\n"
	}

	return content + key + "=" + value + "\r\n"
}
