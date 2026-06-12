package plan

// 文件职责：构建历史 BepInEx 裸露文件迁移计划。
// 使用：migration/legacy 入口读取候选记录后调用本包生成可执行计划。
// 边界：只生成计划，不移动文件、不更新数据库、不计算哈希；排序和路径算法由同包独立文件维护。

import (
	"path/filepath"

	"DysonSphereBox/helper/modDependence/fileops"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
)

// ScopedFileRepairPlan 表示单个历史裸露文件的迁移计划。
// 职责：记录数据库文件记录、当前路径、迁移目标路径和新相对路径。
// 使用：legacy 入口构建计划后交给 repair 包逐个执行。
// 边界：只在当前启动迁移流程内使用，不写入前端 DTO。
type ScopedFileRepairPlan struct {
	// Record 职责：保存待修复的 installed 文件记录；使用：校验哈希和更新数据库；边界：必须来自 installed 表。
	Record modDependenceModel.ModInstalledFile
	// CurrentPath 职责：保存当前文件实际路径；使用：哈希校验和 os.Rename；边界：文件不存在时跳过迁移。
	CurrentPath string
	// NextPath 职责：保存修复后的目标绝对路径；使用：创建目录并移动文件；边界：目标已存在时跳过。
	NextPath string
	// NextRelativePath 职责：保存修复后的游戏目录相对路径；使用：数据库更新；边界：必须与 NextPath 指向同一文件。
	NextRelativePath string
}

// BuildScopedFileRepairPlans 根据历史记录生成可执行迁移计划。
// 职责：复用 artifactlayout 计算新相对路径，并确保目标路径位于游戏目录内。
// 使用：legacy 入口读取候选记录后调用。
// 边界：无法解析 BepInEx 根目录、目标越界或无需迁移的记录会被跳过。
func BuildScopedFileRepairPlans(records []modDependenceModel.ModInstalledFile) []ScopedFileRepairPlan {
	plans := make([]ScopedFileRepairPlan, 0)
	for _, record := range records {
		nextRelativePath, ok := repairScopedRelativePath(record)
		if !ok || sameFilesystemPath(nextRelativePath, record.RelativePath) {
			continue
		}

		bepInExRoot := resolveBepInExRootFromTarget(record.TargetPath)
		if bepInExRoot == "" {
			continue
		}
		nextPath := filepath.Join(filepath.Dir(bepInExRoot), nextRelativePath)
		if !fileops.IsSubPath(filepath.Dir(bepInExRoot), nextPath) {
			continue
		}
		plans = append(plans, ScopedFileRepairPlan{
			Record:           record,
			CurrentPath:      filepath.Clean(record.TargetPath),
			NextPath:         filepath.Clean(nextPath),
			NextRelativePath: nextRelativePath,
		})
	}
	return plans
}
