package forum

// 职责：定义论坛发布审核详情契约；使用：桌面站内信跳转；边界：审核结果以服务端当前记录为准。

// PublicationReview 职责：承载发布审核结果及目标；使用：通过时定位帖子或回复，未通过时展示审核理由；边界：理由可能缺省。
type PublicationReview struct {
	// ID 职责：审核记录 ID；使用：校验响应对应点击的通知；边界：与帖子及回复 ID 独立。
	ID uint `json:"id"`
	// Status 职责：审核状态；使用：决定跳转或显示审核信息；边界：仅 approved 可跳转。
	Status string `json:"status"`
	// PostID 职责：所属帖子 ID；使用：打开论坛详情；边界：隐藏帖子由论坛接口拒绝。
	PostID uint `json:"postId"`
	// TargetType 职责：审核目标类型；使用：区分帖子和回复；边界：已知值为 post、reply。
	TargetType string `json:"targetType"`
	// TargetID 职责：被审核内容的 ID；使用：通过的回复页定位；边界：不用于提交标读。
	TargetID uint `json:"targetId"`
	// Reason 职责：作者可见的审核理由；使用：未通过提示；边界：为空时不展示理由区域。
	Reason string `json:"reason"`
}
