package installplan

// 文件职责：定义 V2 安装器在文件规划、复制和回滚阶段共享的安装计划结构。
// 使用：由 artifact 规划、文件复制、冲突处理、会话记录和 installed 持久化流程传递安装文件上下文。
// 边界：只承载内存态计划数据，不访问数据库、不执行文件复制、不代表最终安装记录已经写入。

import "DysonSphereBox/helper/modDependence/installer/v2/sessionstate"

// File 表示准备复制到游戏目录的单个文件计划。
// 职责：保存源路径、目标路径、哈希、备份和既有记录。
// 使用：冲突检测、复制、回滚和会话恢复流程共享。
// 边界：只在安装执行过程中使用，最终持久化会转换为 installed model。
type File struct {
	// PackageKey 职责：记录文件归属包 key；使用：文件清单和进度归属；边界：依赖文件可能归属父包。
	PackageKey string `json:"packageKey"`
	// ComponentKey 职责：记录文件归属组件 key；使用：冲突决策复用和组件清理；边界：不能为空。
	ComponentKey string `json:"componentKey"`
	// SourcePath 职责：记录待复制源文件路径；使用：复制和哈希计算；边界：必须位于下载/解压或本地导入路径内。
	SourcePath string `json:"sourcePath"`
	// TargetPath 职责：记录目标游戏目录内路径；使用：冲突检测、复制、回滚；边界：规划阶段必须校验不越界。
	TargetPath string `json:"targetPath"`
	// RelativePath 职责：记录相对游戏目录路径；使用：数据库文件清单和卸载；边界：必须与 TargetPath 保持一致。
	RelativePath string `json:"relativePath"`
	// Sha256 职责：记录源文件哈希；使用：冲突判断和安装记录；边界：哈希失败时不生成计划。
	Sha256 string `json:"sha256"`
	// FileSize 职责：记录源文件大小；使用：安装记录展示或后续校验；边界：不作为唯一一致性判断。
	FileSize int64 `json:"fileSize"`
	// Tracked 职责：标记文件是否纳入安装器归属清单；使用：R2 config 等 untracked 文件复制时跳过冲突、卸载记录和共享判断；边界：物理复制仍会发生，失败回滚仍由 CopyRollback 负责。
	Tracked bool `json:"tracked"`
	// TargetExisted 职责：标记复制前目标是否存在；使用：回滚决定恢复或删除；边界：只代表复制前瞬时状态。
	TargetExisted bool
	// BackupPath 职责：记录覆盖前备份文件路径；使用：失败回滚；边界：仅覆盖已有文件时可能存在。
	BackupPath string
	// PreviousRecords 职责：保存目标文件此前的安装记录；使用：冲突覆盖后恢复归属；边界：只用于当前会话。
	PreviousRecords []sessionstate.FileRecordSnapshot
}

// CopyRollback 表示一次复制操作的回滚信息。
// 职责：保存目标路径、备份路径和原文件存在性。
// 使用：安装失败或取消时恢复已复制文件。
// 边界：只在当前复制会话内有效。
type CopyRollback struct {
	// TargetPath 职责：记录需要回滚的目标路径；使用：删除新文件或恢复备份；边界：必须位于游戏目录内。
	TargetPath string
	// BackupPath 职责：记录覆盖前备份路径；使用：恢复原文件；边界：目标原本不存在时可能为空。
	BackupPath string
	// Existed 职责：记录复制前目标是否存在；使用：决定恢复还是删除；边界：只代表复制前瞬时状态。
	Existed bool
}
