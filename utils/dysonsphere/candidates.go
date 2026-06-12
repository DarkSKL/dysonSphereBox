package dysonsphere

// 文件职责：收集戴森球计划自动探测所需的 Steam 根目录和快捷方式目录候选。
// 使用：FindDirFromSteam 构造默认 SearchOptions 时调用。
// 边界：只收集并去重候选路径，不检查目录是否存在，也不读取 Steam manifest。

import (
	"os"
	"path/filepath"
	"strings"

	"DysonSphereBox/utils"
	"golang.org/x/sys/windows/registry"
)

// ShortcutNames 返回常见的戴森球计划快捷方式名称。
// 职责：集中维护中英文常见快捷方式名称。
// 使用：自动检测桌面和开始菜单快捷方式时拼接 .lnk 后缀。
// 边界：只返回名称本体，不检查快捷方式是否存在。
func ShortcutNames() []string {
	return []string{
		"Dyson Sphere Program",
		"戴森球计划",
	}
}

// SteamRootCandidates 返回可能的 Steam 根目录候选列表。
// 职责：从环境变量、Windows 注册表、默认安装目录和常见跨平台目录收集 Steam 根目录。
// 使用：FindDirFromSteam 构造默认搜索选项时调用。
// 边界：只收集并去重候选路径，不检查 Steam 是否真实安装。
// 逻辑流：1. 读取环境变量候选 => 2. 读取 Windows 注册表候选 => 3. 添加 Program Files 默认目录 => 4. 添加用户目录下常见跨平台路径。
func SteamRootCandidates() []string {
	paths := make([]string, 0, 6)
	seen := make(map[string]struct{})

	for _, envName := range []string{"STEAM_PATH", "STEAM_DIR"} {
		utils.AppendUniquePath(&paths, seen, os.Getenv(envName))
	}

	for _, registryPath := range []struct {
		key   registry.Key
		path  string
		value string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamExe"},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `Software\Valve\Steam`, "InstallPath"},
	} {
		utils.AppendUniquePath(&paths, seen, ReadRegistrySteamPath(registryPath.key, registryPath.path, registryPath.value))
	}

	utils.AppendUniquePath(&paths, seen, filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"))
	utils.AppendUniquePath(&paths, seen, filepath.Join(os.Getenv("ProgramFiles"), "Steam"))

	if homeDir, err := os.UserHomeDir(); err == nil {
		utils.AppendUniquePath(&paths, seen, filepath.Join(homeDir, ".steam", "steam"))
		utils.AppendUniquePath(&paths, seen, filepath.Join(homeDir, "Library", "Application Support", "Steam"))
	}

	return paths
}

// ShortcutRootCandidates 返回可能包含游戏快捷方式的目录列表。
// 职责：收集公共桌面、开始菜单、用户桌面和 Steam 开始菜单目录。
// 使用：FindDirFromSteam 构造默认快捷方式搜索目录时调用。
// 边界：只收集候选目录，不检查目录是否存在。
func ShortcutRootCandidates() []string {
	paths := make([]string, 0, 8)
	seen := make(map[string]struct{})

	if publicDir := os.Getenv("PUBLIC"); publicDir != "" {
		utils.AppendUniquePath(&paths, seen, filepath.Join(publicDir, "Desktop"))
	}

	if programDataDir := os.Getenv("PROGRAMDATA"); programDataDir != "" {
		utils.AppendUniquePath(&paths, seen, filepath.Join(programDataDir, "Microsoft", "Windows", "Start Menu", "Programs"))
		utils.AppendUniquePath(&paths, seen, filepath.Join(programDataDir, "Microsoft", "Windows", "Start Menu", "Programs", "Steam"))
	}

	if appDataDir := os.Getenv("APPDATA"); appDataDir != "" {
		utils.AppendUniquePath(&paths, seen, filepath.Join(appDataDir, "Microsoft", "Windows", "Start Menu", "Programs"))
		utils.AppendUniquePath(&paths, seen, filepath.Join(appDataDir, "Microsoft", "Windows", "Start Menu", "Programs", "Steam"))
	}

	if userProfileDir := os.Getenv("USERPROFILE"); userProfileDir != "" {
		utils.AppendUniquePath(&paths, seen, filepath.Join(userProfileDir, "Desktop"))
	}

	return paths
}

// ReadRegistrySteamPath 从 Windows 注册表读取 Steam 安装路径。
// 职责：从指定注册表项读取 Steam 安装目录或 steam.exe 路径并规范为目录。
// 使用：SteamRootCandidates 收集 Windows Steam 安装位置时调用。
// 边界：注册表项不存在或读取失败时返回空字符串；不会返回错误打断自动探测。
func ReadRegistrySteamPath(root registry.Key, path string, value string) string {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	steamPath, _, err := key.GetStringValue(value)
	if err != nil {
		return ""
	}

	if strings.EqualFold(filepath.Base(steamPath), "steam.exe") {
		return filepath.Dir(steamPath)
	}

	return steamPath
}
