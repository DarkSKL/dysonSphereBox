package environment

// 文件职责：定义环境管理已同步文件清单持久化模型。
// 使用：启用、禁用、状态复核和首次纳管流程记录客户端镜像与游戏目录文件对应关系。
// 边界：只保存文件同步证据，不直接执行复制、删除、hash 计算或冲突决策。

import "time"

// EnvSyncedFile 保存环境管理写入到游戏目录的文件证据。
// 职责：记录资源项、源文件、目标文件、同步根目录、相对路径和校验信息。
// 使用：禁用资源、复核目标目录变化和冲突处理时读取。
// 边界：未登记文件不得通过该表驱动删除；根目录变化时服务层必须重新判断路径边界。
type EnvSyncedFile struct {
	// ID 职责：保存本地数据库自增主键；使用：GORM 维护记录身份；边界：业务定位应优先使用 ItemID 和 RelativePath。
	ID uint `json:"id" gorm:"primaryKey;autoIncrement;comment:主键ID;column:id;"`
	// ItemID 职责：保存归属资源 ID；使用：按资源查询同步文件清单；边界：模型层不校验资源是否存在。
	ItemID string `json:"itemId" gorm:"index;size:64;not null;comment:资源ID;column:item_id;"`
	// Kind 职责：保存同步资源类型；使用：区分 mod 和 blueprint 的同步根；边界：存档不写入同步清单。
	Kind string `json:"kind" gorm:"index;size:32;not null;comment:资源类型;column:kind;"`
	// SourcePath 职责：保存同步时客户端镜像源文件绝对路径；使用：状态复核和诊断展示；边界：不能作为删除目标。
	SourcePath string `json:"sourcePath" gorm:"type:text;comment:源文件路径;column:source_path;"`
	// TargetPath 职责：保存同步时游戏目录目标文件绝对路径；使用：禁用和冲突复核前的候选路径；边界：使用前必须确认仍位于当前根目录。
	TargetPath string `json:"targetPath" gorm:"type:text;comment:目标文件路径;column:target_path;"`
	// TargetRootPath 职责：保存同步时使用的 Mod 或蓝图根目录；使用：识别用户更换目标根目录后的旧记录；边界：不代表当前有效根目录。
	TargetRootPath string `json:"targetRootPath" gorm:"type:text;comment:同步目标根目录;column:target_root_path;"`
	// RelativePath 职责：保存相对同步根目录的路径；使用：重新计算目标路径和冲突展示；边界：写入前必须经过路径清理。
	RelativePath string `json:"relativePath" gorm:"type:text;comment:目标相对路径;column:relative_path;"`
	// SHA256 职责：保存同步时目标文件校验值；使用：禁用前确认文件未被外部修改；边界：为空表示旧记录未计算，不可直接视为匹配。
	SHA256 string `json:"sha256" gorm:"size:64;comment:文件SHA256;column:sha256;"`
	// FileSize 职责：保存同步时文件大小；使用：前端诊断和快速差异提示；边界：不替代 hash 校验。
	FileSize int64 `json:"fileSize" gorm:"comment:文件大小;column:file_size;"`
	// SyncedAt 职责：保存最近同步时间；使用：状态展示和审计补充；边界：不代表源文件最新修改时间。
	SyncedAt time.Time `json:"syncedAt" gorm:"comment:同步时间;column:synced_at;"`
}

// TableName 指定环境已同步文件表名。
// 职责：覆盖 GORM 默认表名，保持方案中的 environment_synced_files 命名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (EnvSyncedFile) TableName() string {
	return "environment_synced_files"
}
