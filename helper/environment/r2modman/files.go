package r2modman

// 文件职责：按 R2 的 SUBDIR 安装约定定位包文件并计算依赖操作集合。
// 使用：扫描、启停和移除的计划阶段调用。
// 边界：只支持 plugins、patchers、monomod 的逐包目录；框架、core 与 STATE 布局交回 R2 管理。

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 职责：定位明确归属于清单 name 的全部包文件；使用：修改计划与只读状态；边界：不扫描配置等共享目录，拒绝链接及不一致后缀。
func packageFiles(root string, mod Mod) ([]string, error) {
	if !validPackageName(mod.Name) {
		return nil, fmt.Errorf("模组标识无效")
	}
	parts := strings.SplitN(mod.Name, "-", 2)
	if len(parts) != 2 || strings.Contains(strings.ToLower(parts[1]), "bepinex") {
		return nil, fmt.Errorf("加载器或框架包请在 r2modman 中管理")
	}
	for _, relative := range []string{"BepInEx/core/" + mod.Name, "_state/" + mod.Name + "-state.yml"} {
		path, err := safeProfilePath(root, relative)
		if err != nil {
			return nil, err
		}
		if _, err := os.Lstat(path); err == nil {
			return nil, fmt.Errorf("此包使用 core 或逐文件安装状态，请在 r2modman 中管理")
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	files := []string{}
	for _, kind := range []string{"plugins", "patchers", "monomod"} {
		relative := filepath.Join("BepInEx", kind, mod.Name)
		path, err := safeProfilePath(root, relative)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("模组包路径不是目录: %s", relative)
		}
		err = filepath.WalkDir(path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if _, err := safeProfilePath(root, rel); err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("模组含非普通文件: %s", rel)
			}
			old := strings.HasSuffix(strings.ToLower(entry.Name()), ".old")
			if old == mod.Enabled {
				return fmt.Errorf("文件后缀与 R2 清单状态不一致，请先在 r2modman 中修复: %s", rel)
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("未找到支持直接管理的独立包文件")
	}
	return files, nil
}

// 职责：从带版本依赖中分离包身份；使用：依赖闭包与反向依赖检查；边界：版本格式无效时不猜测匹配。
func dependencyName(value string) string {
	index := strings.LastIndex(value, "-")
	if index < 1 {
		return ""
	}
	return value[:index]
}

// 职责：计算启用依赖、禁用依赖者闭包或校验移除反向依赖；使用：服务授权后的修改计划；边界：缺失依赖不下载，循环依赖只处理一次，禁用闭包必须经用户确认。
func affectedMods(mods []Mod, name string, enabled, remove bool) ([]int, error) {
	byName := map[string]int{}
	selected := -1
	for index, mod := range mods {
		byName[mod.Name] = index
		if mod.Name == name {
			selected = index
		}
	}
	if selected < 0 {
		return nil, fmt.Errorf("Profile 中未找到模组: %s", name)
	}
	if remove {
		for _, mod := range mods {
			if mod.Name == name {
				continue
			}
			for _, dependency := range mod.Dependencies {
				if dependencyName(dependency) == name {
					return nil, fmt.Errorf("%s 依赖此模组，请先处理依赖者", mod.DisplayName)
				}
			}
		}
		return []int{selected}, nil
	}
	if !enabled {
		// 逐层查找启用中的依赖者，包含循环依赖，避免只禁用一个包后留下失效依赖。
		affected := map[string]bool{name: true}
		for changed := true; changed; {
			changed = false
			for _, mod := range mods {
				if !mod.Enabled || affected[mod.Name] {
					continue
				}
				for _, dependency := range mod.Dependencies {
					if affected[dependencyName(dependency)] {
						affected[mod.Name], changed = true, true
						break
					}
				}
			}
		}
		indices := []int{}
		for index, mod := range mods {
			if affected[mod.Name] && mod.Enabled {
				indices = append(indices, index)
			}
		}
		return indices, nil
	}
	visited := map[string]bool{}
	indices := []int{}
	var visit func(int) error
	visit = func(index int) error {
		mod := mods[index]
		if visited[mod.Name] {
			return nil
		}
		visited[mod.Name] = true
		for _, dependency := range mod.Dependencies {
			depIndex, exists := byName[dependencyName(dependency)]
			if !exists {
				return fmt.Errorf("缺少依赖 %s，请先在 r2modman 中安装", dependency)
			}
			if !dependencyVersionSatisfied(mods[depIndex].Version, dependency) {
				return fmt.Errorf("依赖 %s 的已安装版本 %s 不满足要求，请先在 r2modman 中更新", dependency, mods[depIndex].Version)
			}
			if !mods[depIndex].Enabled {
				if err := visit(depIndex); err != nil {
					return err
				}
			}
		}
		if !mod.Enabled {
			indices = append(indices, index)
		}
		return nil
	}
	if err := visit(selected); err != nil {
		return nil, err
	}
	return indices, nil
}

// 职责：从已同步的文件可管理性派生启停权限与联动范围；使用：实盘扫描后及缓存读取时；边界：纯内存计算，不访问外部目录，不解除框架、未知文件布局或离线快照的限制。
func updateModPermissions(mods []Mod) {
	for index := range mods {
		mod := &mods[index]
		mod.CanDisable, mod.CanRemove = mod.CanManage, mod.CanManage
		mod.DisableReason, mod.RemoveReason = "", ""
		mod.DisableWith = []string{}
		if !mod.CanManage {
			mod.DisableReason, mod.RemoveReason = mod.Warning, mod.Warning
			continue
		}
		indices, err := affectedMods(mods, mod.Name, false, false)
		if err != nil {
			mod.CanDisable, mod.DisableReason = false, err.Error()
		}
		for _, affected := range indices {
			if !mods[affected].CanManage {
				mod.CanDisable = false
				mod.DisableReason = fmt.Sprintf("联动范围包含不能直接管理的模组 %s", mods[affected].DisplayName)
			}
			if mods[affected].Name != mod.Name {
				mod.DisableWith = append(mod.DisableWith, mods[affected].Name)
			}
		}
		if _, err := affectedMods(mods, mod.Name, false, true); err != nil {
			mod.CanRemove, mod.RemoveReason = false, err.Error()
		}
	}
}

// 职责：核对 Thunderstore 依赖的最低三段版本；使用：启用依赖闭包前；边界：无效版本拒绝猜测，不执行下载或降级。
func dependencyVersionSatisfied(installed, dependency string) bool {
	index := strings.LastIndex(dependency, "-")
	if index < 0 {
		return false
	}
	actual, required := strings.Split(installed, "."), strings.Split(dependency[index+1:], ".")
	if len(actual) != 3 || len(required) != 3 {
		return false
	}
	comparison := 0
	for index := 0; index < 3; index++ {
		left, leftErr := strconv.Atoi(actual[index])
		right, rightErr := strconv.Atoi(required[index])
		if leftErr != nil || rightErr != nil || left < 0 || right < 0 {
			return false
		}
		if comparison == 0 {
			if left > right {
				comparison = 1
			} else if left < right {
				comparison = -1
			}
		}
	}
	return comparison >= 0
}
