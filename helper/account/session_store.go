package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	accountDefinition "DysonSphereBox/apiDefinition/account"
	accountConstant "DysonSphereBox/constant/account"
	"DysonSphereBox/core"
	accountModel "DysonSphereBox/model/account"

	"gorm.io/gorm"
)

// 文件职责：封装当前账号会话的 SQLite 单行读写。
// 使用：AccountService 通过本模块恢复、保存、更新资料或清理当前会话。
// 边界：不执行 HTTP 请求、不决定登录流程，也不把 JWT 返回给前端。

// currentSessionID 是 SQLite 中当前客户端唯一会话记录的固定主键。
// 职责：保证账号状态只占用一条记录。
// 使用：所有会话读取和写入都以该主键定位。
// 边界：不代表远端用户 ID 或登录令牌。
const currentSessionID = "current"

// LoadSession 从 SQLite 恢复当前账号会话和仅供后端使用的 JWT。
// 职责：将单行持久化记录转换为 Wails 安全会话 DTO。
// 使用：AccountService 在页面初始化和私有请求鉴权前调用。
// 边界：记录不存在时返回零值会话和空令牌；JSON 损坏时返回错误，不猜测用户资料。
func LoadSession() (accountDefinition.AccountSession, string, error) {
	if core.GLOBAL.DB == nil {
		return accountDefinition.AccountSession{}, "", errors.New("sqlite database is not initialized")
	}
	var record accountModel.Session
	err := core.GLOBAL.DB.First(&record, "id = ?", currentSessionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return migrateLegacySession()
	}
	if err != nil {
		return accountDefinition.AccountSession{}, "", fmt.Errorf("读取本地账号会话失败: %w", err)
	}
	session := accountDefinition.AccountSession{NeedChangePassword: record.NeedChangePassword, ExpiresAt: record.ExpiresAt}
	if len(record.UserJSON) > 0 && string(record.UserJSON) != "null" {
		if err := json.Unmarshal(record.UserJSON, &session.User); err != nil {
			return accountDefinition.AccountSession{}, "", fmt.Errorf("本地用户资料缓存无效: %w", err)
		}
	}
	return session, record.Token, nil
}

// migrateLegacySession 将旧配置表中的账号数据迁移为唯一会话记录。
// 职责：让升级后的应用保留已有登录资料，同时结束旧的分散存储方式。
// 使用：LoadSession 找不到 account_sessions 记录时自动调用一次。
// 边界：旧资料缺失或损坏时返回未登录或错误，不尝试伪造令牌；迁移成功后清空旧配置键。
func migrateLegacySession() (accountDefinition.AccountSession, string, error) {
	token, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionTokenKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	if token == "" {
		return accountDefinition.AccountSession{}, "", nil
	}
	expiresText, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionExpiresAtKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	needChangeText, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionNeedChangePasswordKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	userText, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionUserKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}

	session := accountDefinition.AccountSession{Authenticated: true, NeedChangePassword: needChangeText == "true"}
	if expiresText != "" {
		session.ExpiresAt, err = strconv.ParseInt(expiresText, 10, 64)
		if err != nil {
			return accountDefinition.AccountSession{}, "", fmt.Errorf("旧登录过期时间无效: %w", err)
		}
	}
	if userText != "" {
		if err := json.Unmarshal([]byte(userText), &session.User); err != nil {
			return accountDefinition.AccountSession{}, "", fmt.Errorf("旧用户资料缓存无效: %w", err)
		}
	}
	if err := SaveSession(session, token); err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	for _, key := range []string{accountConstant.SessionTokenKey, accountConstant.SessionExpiresAtKey, accountConstant.SessionNeedChangePasswordKey, accountConstant.SessionUserKey} {
		if err := core.InitConfigUnit(accountConstant.SessionConfigGroup, key).Set(""); err != nil {
			return accountDefinition.AccountSession{}, "", fmt.Errorf("清理旧账号会话失败: %w", err)
		}
	}
	return session, token, nil
}

// SaveSession 将完整登录会话覆盖写入 SQLite。
// 职责：一次写入令牌、有效期、改密状态和用户资料，避免分散配置产生中间态。
// 使用：AccountService 在登录成功后调用。
// 边界：不保存密码；序列化或数据库失败时返回错误，不返回部分写入成功的状态。
func SaveSession(session accountDefinition.AccountSession, token string) error {
	if core.GLOBAL.DB == nil {
		return errors.New("sqlite database is not initialized")
	}
	userJSON, err := json.Marshal(session.User)
	if err != nil {
		return fmt.Errorf("序列化本地用户资料失败: %w", err)
	}
	record := accountModel.Session{ID: currentSessionID, Token: token, ExpiresAt: session.ExpiresAt, NeedChangePassword: session.NeedChangePassword, UserJSON: userJSON}
	if err := core.GLOBAL.DB.Save(&record).Error; err != nil {
		return fmt.Errorf("保存本地账号会话失败: %w", err)
	}
	return nil
}

// UpdateCachedUser 更新 SQLite 中当前会话的用户资料快照。
// 职责：保持 Pinia 冷启动使用的资料与服务器最近成功响应一致。
// 使用：AccountService 获取最新用户资料成功后调用。
// 边界：会话不存在时返回错误，不创建无令牌会话；不修改令牌和过期时间。
func UpdateCachedUser(user accountDefinition.UserInfo) error {
	if core.GLOBAL.DB == nil {
		return errors.New("sqlite database is not initialized")
	}
	userJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("序列化本地用户资料失败: %w", err)
	}
	result := core.GLOBAL.DB.Model(&accountModel.Session{}).Where("id = ?", currentSessionID).Update("user_json", userJSON)
	if result.Error != nil {
		return fmt.Errorf("更新本地用户资料失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("本地账号会话不存在")
	}
	return nil
}

// UpdateNeedChangePassword 更新 SQLite 中当前会话的改密标志。
// 职责：保留服务器拒绝普通私有操作后的改密状态。
// 使用：AccountService 收到改密相关错误时调用。
// 边界：会话不存在时不创建新记录，不修改令牌和用户资料。
func UpdateNeedChangePassword(value bool) error {
	if core.GLOBAL.DB == nil {
		return errors.New("sqlite database is not initialized")
	}
	result := core.GLOBAL.DB.Model(&accountModel.Session{}).Where("id = ?", currentSessionID).Update("need_change_password", value)
	if result.Error != nil {
		return fmt.Errorf("更新本地改密状态失败: %w", result.Error)
	}
	return nil
}

// ClearSession 删除 SQLite 中唯一的当前账号会话。
// 职责：清理令牌、资料和改密标志，确保退出后不残留用户数据。
// 使用：注销、令牌过期、401 和改密成功后调用。
// 边界：只删除固定会话记录，不删除其他领域的 SQLite 数据。
func ClearSession() error {
	if core.GLOBAL.DB == nil {
		return errors.New("sqlite database is not initialized")
	}
	if err := core.GLOBAL.DB.Delete(&accountModel.Session{}, "id = ?", currentSessionID).Error; err != nil {
		return fmt.Errorf("清理本地账号会话失败: %w", err)
	}
	return nil
}
