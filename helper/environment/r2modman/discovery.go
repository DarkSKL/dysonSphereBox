package r2modman

// 文件职责：检索本机 r2modman 默认及配置的数据目录中的戴森球计划 Profile。
// 使用：一键同步选择器调用，只返回可解析的配置档。
// 边界：限定候选数据根及游戏目录，不递归扫描磁盘，不自动选择或写入 Profile。

import (
	environmentConstant "DysonSphereBox/constant/environment"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverProfiles 职责：查找默认及额外指定数据根中的 Profile；使用：环境服务检索；边界：无安装时返回空列表，不创建目录，单个无效 Profile 跳过。
func DiscoverProfiles(extraRoots ...string) ([]Profile, error) {
	roots := append([]string{}, extraRoots...)
	if appData := os.Getenv("APPDATA"); appData != "" {
		roots = append(roots, filepath.Join(appData, "r2modmanPlus-local"), filepath.Join(appData, "Thunderstore Mod Manager", "DataFolder"))
	}
	profiles := []Profile{}
	seen := map[string]bool{}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		directory := filepath.Join(root, environmentConstant.R2GameDirectory, "profiles")
		entries, err := os.ReadDir(directory)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			state, err := ReadProfile(filepath.Join(directory, entry.Name()))
			if err != nil {
				continue
			}
			key := strings.ToLower(state.Profile.Path)
			if seen[key] {
				continue
			}
			seen[key] = true
			profiles = append(profiles, *state.Profile)
		}
	}
	sort.Slice(profiles, func(i, j int) bool { return strings.ToLower(profiles[i].Path) < strings.ToLower(profiles[j].Path) })
	return profiles, nil
}
