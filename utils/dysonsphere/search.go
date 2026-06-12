package dysonsphere

// 文件职责：编排 Steam 库、默认目录和快捷方式来源来自动定位戴森球计划游戏目录。
// 使用：系统环境服务自动获取游戏路径时调用 FindDirFromSteam 或 FindDir。
// 边界：只返回本地可验证目录；找不到时返回空字符串，不写入配置。

import (
	"errors"
	"os"
	"path/filepath"

	"DysonSphereBox/utils"
)

// SearchOptions 描述自动搜索戴森球计划目录所需的候选路径和快捷方式解析器。
// 职责：承载自动搜索所需的 Steam 根目录、快捷方式目录和快捷方式解析器。
// 使用：FindDir 接收该结构以支持默认搜索和单元测试注入。
// 边界：调用方可注入候选目录，便于测试或支持非默认环境；结构体不自行校验路径。
type SearchOptions struct {
	// SteamRoots 职责：保存待探测的 Steam 根目录候选；使用：FindDir 遍历并扩展为 Steam 库目录；边界：空列表表示跳过 Steam 根目录扫描。
	SteamRoots []string
	// ShortcutRoots 职责：保存待探测的快捷方式目录候选；使用：FindDirFromShortcuts 遍历候选目录时读取；边界：空列表表示跳过快捷方式扫描。
	ShortcutRoots []string
	// ShortcutResolver 职责：保存快捷方式目标路径解析函数；使用：FindDirFromShortcuts 对每个候选快捷方式调用；边界：为空时跳过快捷方式扫描。
	ShortcutResolver func(string) (string, error)
}

// FindDirFromSteam 使用默认 Steam、注册表和快捷方式候选路径定位游戏目录。
// 职责：使用默认候选源自动查找戴森球计划安装目录。
// 使用：环境服务自动检测游戏路径时调用。
// 边界：只返回本地可验证目录；找不到时返回空字符串。
func FindDirFromSteam() (string, error) {
	return FindDir(SearchOptions{
		SteamRoots:       SteamRootCandidates(),
		ShortcutRoots:    ShortcutRootCandidates(),
		ShortcutResolver: utils.ResolveWindowsShortcutTarget,
	})
}

// FindDir 按 Steam 库、默认 common 目录和快捷方式顺序查找游戏目录。
// 职责：编排 Steam 库、默认目录和快捷方式三类来源查找游戏根目录。
// 使用：环境服务自动获取游戏路径时调用。
// 边界：找不到时返回空字符串；读取异常会透传给调用方；不会写入配置。
// 逻辑流：1. 收集 Steam 根目录和库目录 => 2. 通过 appmanifest 查找安装目录 => 3. 尝试默认 common 目录 => 4. 解析快捷方式目标 => 5. 找不到则返回空结果。
func FindDir(options SearchOptions) (string, error) {
	libraryRoots := make([]string, 0, len(options.SteamRoots))
	seen := make(map[string]struct{})

	for _, steamRoot := range options.SteamRoots {
		utils.AppendUniquePath(&libraryRoots, seen, steamRoot)

		paths, err := utils.ReadSteamLibraryFolders(steamRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		for _, path := range paths {
			utils.AppendUniquePath(&libraryRoots, seen, path)
		}
	}

	for _, libraryRoot := range libraryRoots {
		manifestDirPath, err := utils.ReadSteamAppInstallDir(libraryRoot, SteamAppID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if dirPath := NormalizeDir(manifestDirPath); dirPath != "" {
			return dirPath, nil
		}

		dirPath := filepath.Join(libraryRoot, "steamapps", "common", DirName)
		if NormalizeDir(dirPath) != "" {
			return dirPath, nil
		}
	}

	if dirPath, err := FindDirFromShortcuts(options.ShortcutRoots, options.ShortcutResolver); err != nil || dirPath != "" {
		return dirPath, err
	}

	return "", nil
}

// FindDirFromShortcuts 从候选快捷方式目录解析游戏安装目录。
// 职责：遍历常见快捷方式名称并解析其目标路径为游戏目录。
// 使用：FindDir 在 Steam 探测无结果后调用。
// 边界：解析器为空时跳过；单个快捷方式不存在不会中断搜索；其它解析错误会返回给调用方。
// 逻辑流：1. 遍历候选快捷方式目录 => 2. 拼接常见快捷方式名称 => 3. 调用解析器读取目标路径 => 4. 归一化并校验游戏目录。
func FindDirFromShortcuts(shortcutRoots []string, shortcutResolver func(string) (string, error)) (string, error) {
	if shortcutResolver == nil {
		return "", nil
	}

	for _, shortcutRoot := range shortcutRoots {
		shortcutRoot = utils.CleanPath(shortcutRoot)
		if shortcutRoot == "" {
			continue
		}

		for _, shortcutName := range ShortcutNames() {
			targetPath, err := shortcutResolver(filepath.Join(shortcutRoot, shortcutName+".lnk"))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return "", err
			}
			if dirPath := NormalizeDir(targetPath); dirPath != "" {
				return dirPath, nil
			}
		}
	}

	return "", nil
}
