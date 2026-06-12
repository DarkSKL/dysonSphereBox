package sessionstate

// 文件职责：定义安装会话中文件清单和旧文件记录快照结构。
// 使用：复制、冲突处理、回滚、失败清理和 manifest 持久化共享这些结构。
// 边界：只描述当前会话文件状态，不直接删除文件、不恢复数据库、不等同于最终安装记录。

import "time"

// File 表示安装会话中单个已计划或已复制文件。
// 职责：保存源路径、目标路径、哈希、备份路径和目标旧记录。
// 使用：复制、回滚、失败清理和 manifest 持久化共享。
// 边界：只描述当前会话文件，不等同于数据库最终文件记录。
type File struct {
	// PackageKey 职责：记录文件归属包 key；使用：清理和恢复查询；边界：依赖文件可能归属主包。
	PackageKey string `json:"packageKey"`
	// ComponentKey 职责：记录文件归属组件 key；使用：组件共享判断；边界：不能为空。
	ComponentKey string `json:"componentKey"`
	// SourcePath 职责：记录复制源路径；使用：诊断和部分恢复；边界：清理时不依赖源文件仍存在。
	SourcePath string `json:"sourcePath,omitempty"`
	// TargetPath 职责：记录目标绝对路径；使用：删除、恢复和数据库回滚；边界：必须是规划阶段验证过的游戏目录内路径。
	TargetPath string `json:"targetPath"`
	// RelativePath 职责：记录游戏目录相对路径；使用：数据库恢复和日志；边界：需与 TargetPath 保持一致。
	RelativePath string `json:"relativePath"`
	// Sha256 职责：记录复制源文件哈希；使用：冲突判断和恢复核验；边界：不单独证明文件仍属于当前会话。
	Sha256 string `json:"sha256"`
	// FileSize 职责：记录文件大小；使用：数据库恢复和展示；边界：不作为唯一一致性判断。
	FileSize int64 `json:"fileSize"`
	// TargetExisted 职责：记录复制前目标是否存在；使用：回滚时决定恢复还是删除；边界：只代表复制前瞬时状态。
	TargetExisted bool `json:"targetExisted,omitempty"`
	// BackupPath 职责：记录覆盖前备份路径；使用：失败清理恢复原文件；边界：目标原本不存在时为空。
	BackupPath string `json:"backupPath,omitempty"`
	// PreviousRecords 职责：记录目标文件此前数据库归属；使用：覆盖冲突后失败恢复；边界：只保存当前目标路径相关记录。
	PreviousRecords []FileRecordSnapshot `json:"previousRecords,omitempty"`
}

// FileRecordSnapshot 表示目标文件安装前已有的数据库记录快照。
// 职责：保存可恢复的 installed file 字段。
// 使用：覆盖冲突、失败回滚和恢复历史归属时调用。
// 边界：只保存必要字段，不包含数据库自增 ID。
type FileRecordSnapshot struct {
	// PackageKey 职责：记录旧文件归属包；使用：恢复 installed file 记录；边界：不能为空。
	PackageKey string `json:"packageKey"`
	// ComponentKey 职责：记录旧文件归属组件；使用：恢复组件级文件归属；边界：可能为空。
	ComponentKey string `json:"componentKey"`
	// SourcePath 职责：记录旧源路径；使用：恢复数据库字段；边界：不要求源文件仍存在。
	SourcePath string `json:"sourcePath"`
	// TargetPath 职责：记录旧目标路径；使用：恢复数据库字段和路径匹配；边界：不做越界校验。
	TargetPath string `json:"targetPath"`
	// RelativePath 职责：记录旧相对路径；使用：恢复 installed file 记录；边界：需与 TargetPath 同源。
	RelativePath string `json:"relativePath"`
	// Sha256 职责：记录旧文件哈希；使用：恢复数据库字段；边界：不重新计算。
	Sha256 string `json:"sha256"`
	// FileSize 职责：记录旧文件大小；使用：恢复数据库字段；边界：不重新读取文件。
	FileSize int64 `json:"fileSize"`
	// CreatedAt 职责：记录旧记录创建时间；使用：恢复 installed file 创建时间；边界：不代表恢复时间。
	CreatedAt time.Time `json:"createdAt"`
}
