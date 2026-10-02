package environment

// 文件职责：定位并成组处理 .dsv 存档和同目录同名 .server 联机附加数据。
// 使用：环境服务导入、导出、重命名和快照删除时调用。
// 边界：不处理 .moddsv 等其它文件，不访问数据库或决定操作日志；失败时回滚本次已完成的文件。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"DysonSphereBox/utils"
)

// SaveBundlePaths 返回主存档和存在的联机附加文件。
// 职责：校验 .dsv 普通文件并在同目录按大小写不敏感的名称寻找 .server。
// 使用：存档成组复制、移动及导入调用。
// 边界：附加文件不存在时只返回主文件；不跟随符号链接，不把独立 .server 当作存档。
func SaveBundlePaths(sourcePath string) ([]string, error) {
	sourcePath = filepath.Clean(sourcePath)
	if !strings.EqualFold(filepath.Ext(sourcePath), ".dsv") {
		return nil, fmt.Errorf("只支持 .dsv 存档文件: %s", sourcePath)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("存档不是普通文件: %s", sourcePath)
	}
	paths := []string{sourcePath}
	entries, err := os.ReadDir(filepath.Dir(sourcePath))
	if err != nil {
		return nil, err
	}
	companionName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)) + ".server"
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), companionName) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("联机附加数据不是普通文件: %s", entry.Name())
		}
		paths = append(paths, filepath.Join(filepath.Dir(sourcePath), entry.Name()))
	}
	return paths, nil
}

// CopySaveBundleSafely 成组复制主存档和联机附加数据。
// 职责：先校验全部目标冲突，再复用已校验的文件复制；失败时只清理本次新建且 hash 匹配的文件。
// 使用：服务层导入、导出单个存档时调用，目标为用户选择的目录。
// 边界：不覆盖不同内容的已有文件；源数据始终保留，已有相同内容文件不参与回滚。
func CopySaveBundleSafely(sourcePath string, targetRoot string) error {
	_, err := CopySaveBundlesSafely([]string{sourcePath}, targetRoot)
	return err
}

// CopySaveBundlesSafely 批量复制主存档与同名联机数据并返回主存档数量。
// 职责：展开并去重文件组，预检整批目标，失败时按写入证据清理本次新增文件。
// 使用：存档导入调用；同时选择已配对的 .server 时不重复导入或增加存档计数。
// 边界：独立附加文件和其它扩展名返回错误；不覆盖不同内容文件，不清理原有文件。
// 逻辑流：1. 校验并展开主存档 => 2. 检查选择和目标冲突 => 3. 逐个复制 => 4. 失败则逆序回滚。
func CopySaveBundlesSafely(sourcePaths []string, targetRoot string) (int, error) {
	targetRoot = utils.CleanPath(targetRoot)
	if targetRoot == "" {
		return 0, fmt.Errorf("目标目录为空")
	}
	paths := make([]string, 0, len(sourcePaths)*2)
	seen := make(map[string]bool)
	count := 0
	for _, source := range sourcePaths {
		source = utils.CleanPath(source)
		if strings.EqualFold(filepath.Ext(source), ".server") {
			continue
		}
		absolute, err := filepath.Abs(source)
		if err != nil {
			return 0, err
		}
		if seen[strings.ToLower(absolute)] {
			continue
		}
		bundle, err := SaveBundlePaths(source)
		if err != nil {
			return 0, err
		}
		count++
		for _, member := range bundle {
			absolute, err := filepath.Abs(member)
			if err != nil {
				return 0, err
			}
			key := strings.ToLower(absolute)
			if !seen[key] {
				seen[key] = true
				paths = append(paths, member)
			}
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("请选择 .dsv 主存档文件")
	}
	for _, source := range sourcePaths {
		absolute, err := filepath.Abs(utils.CleanPath(source))
		if err != nil {
			return 0, err
		}
		if !seen[strings.ToLower(absolute)] {
			return 0, fmt.Errorf("附加数据未配对，请同时选择同名 .dsv 存档: %s", source)
		}
	}
	created := make([]SyncedFilePlan, 0, len(paths))
	targetHashes := make(map[string]string)
	for _, source := range paths {
		target := filepath.Join(targetRoot, filepath.Base(source))
		sourceHash, _, err := HashFile(source)
		if err != nil {
			return 0, err
		}
		key := strings.ToLower(target)
		if hash, exists := targetHashes[key]; exists && hash != sourceHash {
			return 0, fmt.Errorf("选择的存档包含同名不同内容文件: %s", target)
		}
		targetHashes[key] = sourceHash
		if info, err := os.Lstat(target); err == nil {
			if !info.Mode().IsRegular() {
				return 0, fmt.Errorf("目标不是普通文件: %s", target)
			}
			targetHash, _, err := HashFile(target)
			if err != nil {
				return 0, err
			}
			if sourceHash != targetHash {
				return 0, fmt.Errorf("目标文件已存在且内容不同: %s", target)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
	}
	for _, source := range paths {
		target := filepath.Join(targetRoot, filepath.Base(source))
		_, statErr := os.Lstat(target)
		plan, err := CopySingleFileSafely(source, target, filepath.Base(source))
		if err != nil {
			for index := len(created) - 1; index >= 0; index-- {
				err = errors.Join(err, RemoveFileIfHashMatches(created[index].TargetPath, created[index].SHA256))
			}
			return 0, err
		}
		if errors.Is(statErr, os.ErrNotExist) {
			created = append(created, plan)
		}
	}
	return count, nil
}

// MoveSaveBundleSafely 成组移动或重命名存档及联机数据。
// 职责：预检全部目标后移动文件；跨卷时使用快照复制，后续失败则按原路径恢复已完成文件。
// 使用：服务层传入同目录新存档路径，或快照目录下保留相对结构的存档路径。
// 边界：不覆盖目标；附加文件缺失不阻塞单机存档，回滚错误与原错误一并返回。
func MoveSaveBundleSafely(sourcePath string, targetPath string) error {
	if !strings.EqualFold(filepath.Ext(targetPath), ".dsv") {
		return fmt.Errorf("目标存档必须使用 .dsv 扩展名")
	}
	paths, err := SaveBundlePaths(sourcePath)
	if err != nil {
		return err
	}
	targets := []string{targetPath}
	for _, source := range paths[1:] {
		targets = append(targets, strings.TrimSuffix(targetPath, filepath.Ext(targetPath))+filepath.Ext(source))
	}
	for _, target := range targets {
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("目标文件已存在: %s", target)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for index, source := range paths {
		_, err := MoveFileToSnapshotSafely(source, filepath.Dir(targets[index]), filepath.Base(targets[index]))
		if err == nil {
			continue
		}
		// 跨卷移动可能在复制成功、删除源失败后留下副本，源仍在时只清理内容一致的目标。
		for rollbackIndex := index; rollbackIndex >= 0; rollbackIndex-- {
			original := paths[rollbackIndex]
			moved := targets[rollbackIndex]
			if _, statErr := os.Stat(moved); statErr != nil {
				continue
			}
			if _, statErr := os.Stat(original); statErr == nil {
				hash, _, hashErr := HashFile(original)
				if hashErr != nil {
					err = errors.Join(err, hashErr)
				} else {
					err = errors.Join(err, RemoveFileIfHashMatches(moved, hash))
				}
			} else {
				_, restoreErr := MoveFileToSnapshotSafely(moved, filepath.Dir(original), filepath.Base(original))
				err = errors.Join(err, restoreErr)
			}
		}
		return err
	}
	return nil
}
