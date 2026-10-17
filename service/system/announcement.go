package system

// 文件职责：暴露客户端公告查询与手动已读Wails方法。
// 使用：前端公告composable调用，helper负责HTTP、会话绑定和协议校验。
// 边界：不渲染HTML、不保存公告历史、不把token返回前端。

import (
	systemAPI "DysonSphereBox/apiDefinition/system"
	announcementHelper "DysonSphereBox/helper/system/announcement"
	"context"
)

// AnnouncementService承载公告客户端绑定。
// 职责：把Vue请求转入公告helper。
// 使用：service/system注册后由Wails生成绑定。
// 边界：构造不访问网络或数据库。
type AnnouncementService struct{}

// CaptureSession捕获当前已验证会话并返回不透明句柄。
// 职责：绑定server、subject、代次与Go侧token。
// 使用：前端自动或手动检查前调用。
// 边界：句柄失效时后续请求失败，不泄露token。
func (service *AnnouncementService) CaptureSession() (string, error) {
	return announcementHelper.CaptureSessionID()
}

// GetLatest查询当前账号最新未读公告。
// 职责：返回最多一条严格校验公告。
// 使用：登录恢复后或手动入口调用。
// 边界：失败不阻断主流程，不回溯旧公告。
func (service *AnnouncementService) GetLatest(sessionID string) (*announcementHelper.NoticeLatestDTO, error) {
	return announcementHelper.GetLatest(context.Background(), sessionID)
}

// GetHistory 查询已发布公告的单页历史。
// 职责：校验分页参数并把账号作用域请求委托给helper。
// 使用：通知入口打开后按页加载，与启动时最新未读检查分离。
// 边界：只返回不含token的数据DTO，普通查看不产生已读副作用。
func (service *AnnouncementService) GetHistory(sessionID string, page, pageSize int64) (*announcementHelper.NoticeHistoryDTO, error) {
	return announcementHelper.GetHistory(context.Background(), sessionID, page, pageSize)
}

// GetHistoryItem 选择当前会话最近成功历史页中的一条详情。
// 职责：验证用户打开的版本来自当前分页并为手动确认建立版本绑定。
// 使用：前端用户选择一条历史消息后调用。
// 边界：不读取网络、不确认已读；任意ID或过时页条目均失败。
func (service *AnnouncementService) GetHistoryItem(sessionID, id string, revision int64) (*systemAPI.ClientNoticeHistoryEntry, error) {
	return announcementHelper.GetHistoryItem(sessionID, id, revision)
}

// MarkHistoryRead 提交用户手动确认历史详情。
// 职责：按当前句柄和版本复用服务端首次已读接口。
// 使用：历史正文通过可见性检查且用户点击“已查阅”后调用。
// 边界：过期和已读公告不能重复首次确认，关闭/列表渲染不提交。
func (service *AnnouncementService) MarkHistoryRead(sessionID, id string, revision int64) (*announcementHelper.MarkReadDTO, error) {
	return announcementHelper.MarkHistoryRead(context.Background(), sessionID, id, revision)
}

// MarkRead提交用户手动已查阅。
// 职责：按原句柄和版本发送幂等确认。
// 使用：正文真实可见后按钮点击调用。
// 边界：普通关闭、渲染成功和排队不触发。
func (service *AnnouncementService) MarkRead(sessionID, id string, revision int64) (*announcementHelper.MarkReadDTO, error) {
	return announcementHelper.MarkRead(context.Background(), sessionID, id, revision)
}
