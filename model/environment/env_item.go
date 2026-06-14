package environment

// 文件职责：定义环境管理资源项目持久化模型。
// 使用：由环境管理服务在纳管 Mod、蓝图和蓝图文件夹时读写，core 自动迁移表结构。
// 边界：只保存客户端资源库项目身份和状态，不直接描述真实文件同步清单或执行文件操作。

import (
	"time"

	"gorm.io/datatypes"
)

// EnvItem 保存环境管理中的本地资源项目。
// 职责：记录 Mod、蓝图和蓝图文件夹的客户端资源库身份、目录关系和展示状态。
// 使用：首次同步、资源导入、目录树浏览和后续启用/禁用流程通过该模型定位资源。
// 边界：存档不写入该表；文件复制、删除和冲突处理由服务与 helper 完成。
type EnvItem struct {
	// ID 职责：保存资源本地稳定 ID；使用：前端操作和同步记录关联；边界：由服务层生成，不依赖数据库自增。
	ID string `json:"id" gorm:"primaryKey;size:64;comment:本地资源ID;column:id;"`
	// Kind 职责：保存资源类型；使用：区分 mod、blueprint、blueprint_folder；边界：模型层不校验枚举合法性。
	Kind string `json:"kind" gorm:"index;size:32;not null;comment:资源类型;column:kind;"`
	// Name 职责：保存用户可见名称；使用：环境页卡片和目录树展示；边界：Mod 第一版不允许用户重命名。
	Name string `json:"name" gorm:"size:255;not null;comment:显示名称;column:name;"`
	// ParentID 职责：保存蓝图目录树父节点 ID；使用：蓝图文件夹导航和移动；边界：Mod 始终为空。
	ParentID string `json:"parentId" gorm:"index;size:64;comment:父节点ID;column:parent_id;"`
	// State 职责：保存资源持久化启用状态；使用：列表展示 enabled、disabled、default、conflict、missing；边界：syncing 和 changed 不写入该字段。
	State string `json:"state" gorm:"index;size:32;not null;comment:资源状态;column:state;"`
	// SourceType 职责：保存资源来源类型；使用：区分导入、市场、首次发现和快速导入；边界：只用于诊断和展示，不决定同步策略。
	SourceType string `json:"sourceType" gorm:"size:64;comment:来源类型;column:source_type;"`
	// LibraryPath 职责：保存资源库相对路径；使用：服务层解析客户端镜像位置；边界：不保存真实游戏目录目标路径。
	LibraryPath string `json:"libraryPath" gorm:"type:text;comment:资源库相对路径;column:library_path;"`
	// OriginPath 职责：保存首次导入或纳管来源路径；使用：诊断和用户追溯；边界：后续删除不能只依据该路径。
	OriginPath string `json:"originPath" gorm:"type:text;comment:来源路径;column:origin_path;"`
	// BlueprintRelativeDir 职责：保存蓝图真实相对目录；使用：同步到游戏蓝图根目录时还原目录结构；边界：非蓝图资源为空。
	BlueprintRelativeDir string `json:"blueprintRelativeDir" gorm:"type:text;comment:蓝图相对目录;column:blueprint_relative_dir;"`
	// SizeBytes 职责：保存资源最近扫描大小；使用：前端展示和排序；边界：不是同步完整性依据。
	SizeBytes int64 `json:"sizeBytes" gorm:"comment:资源大小;column:size_bytes;"`
	// MetadataJSON 职责：保存 Mod 清单、蓝图扩展信息等结构化元数据；使用：详情页和后续筛选读取；边界：模型层不解析业务含义。
	MetadataJSON datatypes.JSON `json:"metadataJson" gorm:"comment:扩展元数据;column:metadata_json;"`
	// CreatedAt 职责：保存资源创建时间；使用：GORM 创建记录和前端排序；边界：不代表源文件创建时间。
	CreatedAt time.Time `json:"createdAt" gorm:"comment:创建时间;column:created_at;"`
	// UpdatedAt 职责：保存资源更新时间；使用：GORM 更新和前端排序；边界：不代表游戏侧文件修改时间。
	UpdatedAt time.Time `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
}

// TableName 指定环境资源项目表名。
// 职责：覆盖 GORM 默认表名，保持方案中的 environment_items 命名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不访问数据库。
func (EnvItem) TableName() string {
	return "environment_items"
}
