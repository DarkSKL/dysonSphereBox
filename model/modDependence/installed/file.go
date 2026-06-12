package installed

// 文件职责：定义 V2 安装器写入文件的持久化模型。
// 使用：卸载、冲突检测、失败清理和恢复安装会话时按 ModInstalledFile 精准定位文件。
// 边界：只记录安装器管理的文件，不代表用户手动文件可被删除、不执行文件系统操作。

import "time"

// ModInstalledFile 记录 V2 安装器写入的单个文件。
// 职责：保存安装器写入的目标文件、来源文件、校验值和归属关系。
// 使用：卸载、冲突检测、失败清理和恢复安装会话时按文件清单精准处理。
// 边界：只记录由安装器管理的文件，不代表用户手动文件可被删除。
type ModInstalledFile struct {
	// ID 职责：保存本地数据库自增主键；使用：GORM 维护文件记录；边界：文件身份以 TargetPath 和包/组件归属为准。
	ID uint `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	// PackageKey 职责：保存文件所属安装包；使用：卸载和失败清理按包定位文件；边界：必须对应当前安装会话主包。
	PackageKey string `json:"packageKey" gorm:"index;size:255;not null;comment:所属安装包;column:package_key;"`
	// ComponentKey 职责：保存文件所属组件；使用：冲突提示、卸载明细和共享判断；边界：主包自身文件也应写入组件 key。
	ComponentKey string `json:"componentKey" gorm:"index;size:255;comment:所属组件;column:component_key;"`
	// SourcePath 职责：保存缓存或解压源文件路径；使用：诊断文件来源和清理缓存；边界：安装完成后源文件可能已被清理。
	SourcePath string `json:"sourcePath" gorm:"type:text;comment:缓存或解压源文件;column:source_path;"`
	// TargetPath 职责：保存目标绝对路径；使用：卸载、冲突检测和失败回滚；边界：删除前仍需校验路径位于受管理目录内。
	TargetPath string `json:"targetPath" gorm:"index;type:text;not null;comment:目标绝对路径;column:target_path;"`
	// RelativePath 职责：保存相对 BepInEx 或游戏目录的路径；使用：前端展示和清理目录规划；边界：不作为绝对删除路径。
	RelativePath string `json:"relativePath" gorm:"type:text;not null;comment:相对 BepInEx 或游戏目录路径;column:relative_path;"`
	// Sha256 职责：保存目标文件校验值；使用：冲突检测和诊断文件变化；边界：为空表示未计算或非普通文件场景。
	Sha256 string `json:"sha256" gorm:"size:64;comment:目标文件校验值;column:sha256;"`
	// FileSize 职责：保存目标文件大小；使用：卸载明细和诊断；边界：不作为完整性唯一依据。
	FileSize int64 `json:"fileSize" gorm:"comment:文件大小;column:file_size;"`
	// CreatedAt 职责：保存文件记录创建时间；使用：诊断安装顺序；边界：不代表文件系统 mtime。
	CreatedAt time.Time `json:"createdAt" gorm:"comment:记录时间;column:created_at;"`
}

// TableName 指定 V2 安装文件表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModInstalledFile) TableName() string {
	return "mod_installed_files"
}
