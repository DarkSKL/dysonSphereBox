package system

// 职责：暴露通知中心的 Wails 查询与写入；使用：应用消息状态与面板；边界：连接由桌面生命周期独立维护。

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"sync"

	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/helper/system/account"
	"DysonSphereBox/helper/system/notification"
)

// NotificationService 职责：编排收件箱和应用级推送；使用：系统服务注册及应用生命周期；边界：注册时不发请求。
type NotificationService struct {
	// mu 保护应用上下文与工作协程句柄；Wails 查询从同一生命周期取得取消信号。
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// 职责：读取应用级请求上下文；使用：每次 Wails 调用前；边界：尚未启动或已关闭时拒绝访问。
func (s *NotificationService) requestContext(sessionKey string) (context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil || s.ctx.Err() != nil {
		return nil, errors.New("消息服务尚未就绪")
	}
	if sessionKey == "" {
		return nil, errors.New("缺少消息会话")
	}
	return s.ctx, nil
}

// GetUnread 职责：查询当前账号未读汇总；使用：全局角标；边界：不读取公告或修改回执。
func (s *NotificationService) GetUnread(sessionKey string) (systemAPI.NotificationUnread, error) {
	ctx, err := s.requestContext(sessionKey)
	if err != nil {
		return systemAPI.NotificationUnread{}, err
	}
	return notification.Request[systemAPI.NotificationUnread](ctx, sessionKey, systemAPI.NotificationUnreadAPI, nil, nil)
}

// GetInbox 职责：按页查询站内信；使用：全局消息面板；边界：页码和容量在发送前校验，查询不标读。
func (s *NotificationService) GetInbox(sessionKey string, page, pageSize int) (notification.InboxPage, error) {
	ctx, err := s.requestContext(sessionKey)
	if err != nil {
		return notification.InboxPage{}, err
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return notification.InboxPage{}, errors.New("消息分页参数无效")
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "pageSize": {strconv.Itoa(pageSize)}}
	result, err := notification.Request[systemAPI.NotificationPage](ctx, sessionKey, systemAPI.NotificationInboxAPI, query, nil)
	if err != nil {
		return notification.InboxPage{}, err
	}
	if result.Page < 1 || result.PageSize < 1 || result.Total < 0 {
		return notification.InboxPage{}, errors.New("消息分页响应无效")
	}
	return notification.ToInboxPage(result), nil
}

// MarkRead 职责：确认当前账号的一条投递；使用：消息列表项操作；边界：ID 按整数传给服务端，重复确认允许影响零行。
func (s *NotificationService) MarkRead(sessionKey, id string) (systemAPI.NotificationReadResult, error) {
	ctx, err := s.requestContext(sessionKey)
	if err != nil {
		return systemAPI.NotificationReadResult{}, err
	}
	numericID, err := strconv.ParseUint(id, 10, 64)
	if err != nil || numericID == 0 {
		return systemAPI.NotificationReadResult{}, errors.New("消息 ID 无效")
	}
	result, err := notification.Request[systemAPI.NotificationReadResult](ctx, sessionKey, systemAPI.NotificationReadAPI, nil, &systemAPI.NotificationReadRequest{IDs: []uint64{numericID}})
	if err == nil && result.Affected > 0 {
		account.RecordStateChange("notificationRead", "已标记站内信为已读")
	}
	return result, err
}
