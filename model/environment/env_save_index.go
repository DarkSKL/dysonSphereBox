package environment

// 文件职责：定义环境管理存档最近扫描索引模型。
// 使用：存档扫描后缓存最近看到的真实存档文件信息，辅助页面展示和诊断。
// 边界：该表不是存档身份权威；每次存档操作仍以真实 SaveRoot 实时扫描为准。

import "time"

// EnvSaveIndex 保存存档目录最近扫描结果。
// 职责：记录 SaveRoot 下存档文件的相对路径、大小、校验和最近发现时间。
// 使用：存档页刷新后写入缓存，后续可用于诊断或快速展示。
// 边界：外部重命名后旧记录不能继续驱动 UI 展示；真实文件系统始终优先。
type EnvSaveIndex struct {
	// ID 职责：保存基于 SaveRoot 相对路径生成的本地 ID；使用：前端列表和存档操作定位当前扫描项；边界：更换 SaveRoot 后同名文件可能生成不同 ID。
	ID string `json:"id" gorm:"primaryKey;size:80;comment:存档扫描ID;column:id;"`
	// Name 职责：保存存档显示名；使用：前端卡片展示；边界：由文件名派生，不解析游戏内部元数据。
	Name string `json:"name" gorm:"size:255;not null;comment:显示名称;column:name;"`
	// FilePath 职责：保存最近扫描到的真实绝对路径；使用：诊断和操作前重新校验；边界：操作前必须再次确认文件存在且位于 SaveRoot。
	FilePath string `json:"filePath" gorm:"type:text;comment:真实文件路径;column:file_path;"`
	// RelativePath 职责：保存相对 SaveRoot 的路径；使用：生成 ID 和展示文件名；边界：不得包含路径穿越。
	RelativePath string `json:"relativePath" gorm:"type:text;comment:相对路径;column:relative_path;"`
	// SHA256 职责：保存最近索引校验值；使用：后续导入覆盖或诊断扩展；边界：第一阶段可为空以避免大文件扫描开销。
	SHA256 string `json:"sha256" gorm:"size:64;comment:文件SHA256;column:sha256;"`
	// FileSize 职责：保存文件大小；使用：前端展示和排序；边界：不代表文件可读性。
	FileSize int64 `json:"fileSize" gorm:"comment:文件大小;column:file_size;"`
	// LastModifiedAt 职责：保存文件系统最后修改时间；使用：前端显示最近保存时间；边界：由文件系统返回，不解析存档内部时间。
	LastModifiedAt time.Time `json:"lastModifiedAt" gorm:"comment:最后修改时间;column:last_modified_at;"`
	// LastSeenAt 职责：保存最近扫描发现时间；使用：判断缓存陈旧和诊断外部删除；边界：不代表游戏保存时间。
	LastSeenAt time.Time `json:"lastSeenAt" gorm:"comment:最近发现时间;column:last_seen_at;"`
	// Status 职责：保存扫描状态；使用：前端展示 normal、missing、locked、invalid；边界：真实可操作性仍需操作前复核。
	Status string `json:"status" gorm:"index;size:32;not null;comment:扫描状态;column:status;"`
}

// TableName 指定环境存档索引表名。
// 职责：覆盖 GORM 默认表名，保持方案中的 environment_save_index 命名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (EnvSaveIndex) TableName() string {
	return "environment_save_index"
}
