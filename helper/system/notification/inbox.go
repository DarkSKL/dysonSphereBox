package notification

// 职责：转换站内信分页用于桌面桥接；使用：通知服务；边界：投递 ID 在 JavaScript 中始终保留十进制字符串。

import (
	"strconv"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// InboxEntry 职责：提供无精度损失的投递数据；使用：消息列表和标读；边界：业务 ID 保持服务端字符串格式。
type InboxEntry struct {
	// ID 职责：投递记录 ID；使用：标读；边界：不要转换为 JS 数值。
	ID string `json:"ID"`
	// Title 职责：标题；使用：列表；边界：纯文本。
	Title string `json:"title"`
	// Body 职责：正文；使用：列表；边界：纯文本。
	Body string `json:"body"`
	// BizType 职责：业务类别；使用：跳转分发；边界：未知类型可阅读。
	BizType string `json:"bizType"`
	// BizID 职责：业务对象 ID；使用：跳转参数；边界：不能用于标读。
	BizID string `json:"bizID"`
	// ReadAt 职责：已读时间；使用：标记未读；边界：未读为空。
	ReadAt string `json:"readAt"`
	// RecalledAt 职责：撤回时间；使用：屏蔽跳转及标读；边界：未撤回为空。
	RecalledAt string `json:"recalledAt"`
	// CreatedAt 职责：投递时间；使用：列表显示；边界：保留服务端格式。
	CreatedAt string `json:"createdAt"`
}

// InboxPage 职责：提供桌面收件箱分页；使用：Wails 结果；边界：总数以服务端为准。
type InboxPage struct {
	// Page 职责：页码；使用：分页；边界：从 1 起。
	Page int `json:"page"`
	// PageSize 职责：每页条数；使用：计算页数；边界：使用远端实际值。
	PageSize int `json:"pageSize"`
	// Total 职责：总条数；使用：分页；边界：可能随消息撤回变化。
	Total int64 `json:"total"`
	// List 职责：当页条目；使用：列表渲染；边界：空页为空数组。
	List []InboxEntry `json:"list"`
}

// ToInboxPage 职责：转换契约数据；使用：服务出参；边界：在 Go 侧将整数 ID 转为字符串。
func ToInboxPage(page systemAPI.NotificationPage) InboxPage {
	entries := make([]InboxEntry, 0, len(page.List))
	for _, entry := range page.List {
		entries = append(entries, InboxEntry{
			ID: strconv.FormatUint(entry.ID, 10), Title: entry.Title, Body: entry.Body,
			BizType: entry.BizType, BizID: entry.BizID, ReadAt: entry.ReadAt, RecalledAt: entry.RecalledAt, CreatedAt: entry.CreatedAt,
		})
	}
	return InboxPage{Page: page.Page, PageSize: page.PageSize, Total: page.Total, List: entries}
}
