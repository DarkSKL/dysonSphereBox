package forum

// 职责：提供发布审核详情桥接；使用：桌面站内信通知；边界：访问权限由论坛服务器校验。

import (
	"strconv"

	api "DysonSphereBox/apiDefinition/forum"
	forumHelper "DysonSphereBox/helper/forum"
)

// PublicationReview 职责：读取当前投稿人的审核记录；使用：通知中心传入会话与十进制审核 ID；边界：ID 保持原始精度并校验结果对应原请求。
func (service *ForumService) PublicationReview(sessionKey, reviewID string) (api.PublicationReview, error) {
	id, err := strconv.ParseUint(reviewID, 10, strconv.IntSize)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != reviewID {
		return api.PublicationReview{}, forumHelper.InvalidInput("缺少有效的审核记录 ID")
	}
	call, err := forumHelper.IDInput(sessionKey, uint(id))
	if err != nil {
		return api.PublicationReview{}, err
	}
	result, err := forumHelper.DoJSON[api.PublicationReview](service.callerContext(), api.ReviewAPI, call)
	if err == nil && (uint64(result.ID) != id || result.Status == "") {
		return api.PublicationReview{}, forumHelper.InvalidInput("审核详情响应无效")
	}
	return result, err
}
