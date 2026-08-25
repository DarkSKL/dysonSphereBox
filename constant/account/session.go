package account

// 文件职责：集中定义账号登录态在本地配置中的稳定键名。
// 使用：AccountService 保存、恢复或清理当前用户的 JWT、资料和强制改密状态时引用。
// 边界：只提供键名，不负责加密、数据库读写或远端鉴权。

const (
	// SessionConfigGroup 职责：标识账号登录态配置分组；使用：读取和写入账号会话时使用；边界：不与系统通用设置混用。
	SessionConfigGroup = "Account"
	// SessionTokenKey 职责：标识 JWT 配置键；使用：私有接口鉴权前读取；边界：不得通过 Wails 返回给前端。
	SessionTokenKey = "Token"
	// SessionExpiresAtKey 职责：标识 JWT 过期时间配置键；使用：恢复本地会话时检查；边界：值为 Unix 毫秒文本。
	SessionExpiresAtKey = "ExpiresAt"
	// SessionNeedChangePasswordKey 职责：标识强制改密状态配置键；使用：个人页决定可用操作时读取；边界：仅表示当前本地会话状态。
	SessionNeedChangePasswordKey = "NeedChangePassword"
	// SessionUserKey 职责：标识用户资料缓存配置键；使用：个人页冷启动展示与请求鉴权用户 ID 读取；边界：缓存不是最新资料的替代品。
	SessionUserKey = "User"
)
