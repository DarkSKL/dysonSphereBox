package account

import "gorm.io/datatypes"

// 文件职责：定义当前账号会话的单行 SQLite 持久化模型。
// 使用：账号会话存储器保存 JWT、有效期、改密标志和用户资料快照时读写。
// 边界：不向前端暴露 JWT，不处理远端接口，也不保存密码或验证码。

// Session 保存当前桌面应用唯一账号会话。
// 职责：把同一登录态所需的安全字段和资料快照原子地存入 SQLite。
// 使用：helper/account/session_store 以固定主键读取、覆盖或清理该记录。
// 边界：仅保存当前账号，不保存账号历史；密码和验证码不得写入本表。
type Session struct {
	// ID 职责：固定为当前会话记录的主键；使用：存储器按该值查询和覆盖；边界：不对应远端用户 ID。
	ID string `gorm:"primaryKey;size:32"`
	// Token 职责：保存私有接口鉴权用 JWT；使用：仅后端账号服务创建请求时读取；边界：绝不通过 Wails 返回前端。
	Token string `gorm:"type:text"`
	// ExpiresAt 职责：保存 JWT 的 Unix 毫秒过期时间；使用：恢复会话时判断是否仍有效；边界：零值表示服务端未提供过期时间。
	ExpiresAt int64
	// NeedChangePassword 职责：保存服务端要求改密的状态；使用：个人页恢复时切换到改密视图；边界：只反映本地上次已知状态。
	NeedChangePassword bool
	// UserJSON 职责：保存当前用户资料 JSON 快照；使用：冷启动时先展示缓存资料，随后后台刷新；边界：不保存密码、令牌或未声明的会话字段。
	UserJSON datatypes.JSON `gorm:"type:json"`
}

// TableName 返回账号会话在 SQLite 中的稳定表名。
// 职责：避免通用 Session 名称与其他领域模型冲突。
// 使用：GORM 迁移和会话存储器查询本模型时自动调用。
// 边界：只声明表名，不处理建表、迁移或数据读写。
func (Session) TableName() string {
	return "account_sessions"
}
