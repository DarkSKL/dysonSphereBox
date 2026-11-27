package feedback

// 文件职责：提供反馈星系页面唯一的 Wails API。
// 使用：Vue 页面通过生成的 FeedbackService 调用，每个方法接收一个 DTO。
// 边界：只检查客户端可以确定的输入错误。鉴权、可见性、版本冲突和幂等由服务器裁定。

import (
	api "DysonSphereBox/apiDefinition/feedback"
	"DysonSphereBox/core"
	feedbackHelper "DysonSphereBox/helper/feedback"
	"context"
	"net/url"
	"strings"
)

// FeedbackService 职责：暴露星系读写契约；使用：根服务组注册给 Wails；边界：实例本身不缓存用户、凭据或数据。
type FeedbackService struct{}

// callerContext 使用应用生命周期取消信号，基础设施未就绪时由认证入口拒绝请求。
func (service *FeedbackService) callerContext() context.Context {
	if ctx := core.GLOBAL.RuntimeContext; ctx != nil {
		return ctx
	}
	return context.Background()
}

// Map 职责：获取全景或视区快照；使用：星系页面查询；边界：失效账号不返回数据。
func (service *FeedbackService) Map(input api.MapRequest) (api.MapResult, error) {
	query, err := feedbackHelper.MapQuery(input)
	if err != nil {
		return api.MapResult{}, err
	}
	return feedbackHelper.Do[api.MapResult](service.callerContext(), input.SessionKey, api.MapAPI, query, nil)
}

// Preview 职责：查询星星首行预览；使用：星系页面查询；边界：失效账号不返回数据。
func (service *FeedbackService) Preview(input api.PreviewRequest) (api.PreviewResult, error) {
	if strings.TrimSpace(input.StarID) == "" {
		return api.PreviewResult{}, feedbackHelper.InvalidInput("缺少查询星星首行预览所需的参数")
	}
	query := url.Values{"starId": {input.StarID}}
	return feedbackHelper.Do[api.PreviewResult](service.callerContext(), input.SessionKey, api.PreviewAPI, query, nil)
}

// Previews 职责：读取一批公开星的首行与署名。使用：星图预取时按可见代表星调用。
// 边界：会话键只交给本机会话守卫，仅向服务器发送校验后的 ID；旧接口返回 unsupported。
func (service *FeedbackService) Previews(input api.PreviewsRequest) (api.PreviewsResult, error) {
	ids, err := feedbackHelper.PreviewIDs(input.StarIDs)
	if err != nil {
		return api.PreviewsResult{}, err
	}
	payload := api.PreviewsRequest{StarIDs: ids}
	return feedbackHelper.Do[api.PreviewsResult](service.callerContext(), input.SessionKey, api.PreviewsAPI, nil, payload)
}

// Detail 职责：查询星星详情和发言分页；使用：星系页面查询；边界：失效账号不返回数据。
func (service *FeedbackService) Detail(input api.DetailRequest) (api.StarDetail, error) {
	if strings.TrimSpace(input.StarID) == "" {
		return api.StarDetail{}, feedbackHelper.InvalidInput("缺少查询星星详情和发言分页所需的参数")
	}
	query := feedbackHelper.PageQuery(input.Page, input.PageSize)
	query.Set("starId", input.StarID)
	return feedbackHelper.Do[api.StarDetail](service.callerContext(), input.SessionKey, api.DetailAPI, query, nil)
}

// Mine 职责：查询当前账号的星星；使用：星系页面查询；边界：失效账号不返回数据。
func (service *FeedbackService) Mine(input api.PageRequest) (api.MineResult, error) {
	query := feedbackHelper.PageQuery(input.Page, input.PageSize)
	return feedbackHelper.Do[api.MineResult](service.callerContext(), input.SessionKey, api.MineAPI, query, nil)
}

// Publish 职责：发布星星；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) Publish(input api.PublishRequest) (api.WriteStarResult, error) {
	if strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.Body) == "" {
		return api.WriteStarResult{}, feedbackHelper.InvalidInput("缺少发布星星所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.WriteStarResult](service.callerContext(), sessionKey, api.PublishAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("publish", "发布星星")
	}
	return result, err
}

// Edit 职责：编辑星星原文；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) Edit(input api.EditRequest) (api.WriteStarResult, error) {
	if strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.StarID) == "" || strings.TrimSpace(input.BodyVersion) == "" || strings.TrimSpace(input.Body) == "" {
		return api.WriteStarResult{}, feedbackHelper.InvalidInput("缺少编辑星星原文所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.WriteStarResult](service.callerContext(), sessionKey, api.EditAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("edit", "编辑星星原文")
	}
	return result, err
}

// Reply 职责：提交留言或作者补充；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) Reply(input api.ReplyRequest) (api.ReplyResult, error) {
	if strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.StarID) == "" || strings.TrimSpace(input.BodyVersion) == "" || strings.TrimSpace(input.Kind) == "" || strings.TrimSpace(input.Body) == "" {
		return api.ReplyResult{}, feedbackHelper.InvalidInput("缺少提交留言或作者补充所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.ReplyResult](service.callerContext(), sessionKey, api.ReplyAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("reply", "提交留言或作者补充")
	}
	return result, err
}

// Like 职责：设置点赞状态；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) Like(input api.LikeRequest) (api.LikeResult, error) {
	if strings.TrimSpace(input.StarID) == "" || strings.TrimSpace(input.BodyVersion) == "" {
		return api.LikeResult{}, feedbackHelper.InvalidInput("缺少设置点赞状态所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.LikeResult](service.callerContext(), sessionKey, api.LikeAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("like", "设置点赞状态")
	}
	return result, err
}

// Withdraw 职责：撤回我的星星；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) Withdraw(input api.WithdrawRequest) (api.WithdrawResult, error) {
	if strings.TrimSpace(input.StarID) == "" {
		return api.WithdrawResult{}, feedbackHelper.InvalidInput("缺少撤回我的星星所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.WithdrawResult](service.callerContext(), sessionKey, api.WithdrawAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("withdraw", "撤回我的星星")
	}
	return result, err
}

// Seen 职责：推进留言已读游标；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) Seen(input api.SeenRequest) (api.OKResult, error) {
	if strings.TrimSpace(input.StarID) == "" || strings.TrimSpace(input.ReviewVersion) == "" {
		return api.OKResult{}, feedbackHelper.InvalidInput("缺少推进留言已读游标所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.OKResult](service.callerContext(), sessionKey, api.SeenAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("seen", "推进留言已读游标")
	}
	return result, err
}

// Notices 职责：查询我的作者提醒；使用：星系页面查询；边界：失效账号不返回数据。
func (service *FeedbackService) Notices(input api.PageRequest) (api.NoticesResult, error) {
	query := feedbackHelper.PageQuery(input.Page, input.PageSize)
	return feedbackHelper.Do[api.NoticesResult](service.callerContext(), input.SessionKey, api.NoticesAPI, query, nil)
}

// ReadNotice 职责：确认作者提醒已读；使用：用户主动操作；边界：写入失败时由页面决定是否用原请求标识重试。
func (service *FeedbackService) ReadNotice(input api.ReadNoticeRequest) (api.OKResult, error) {
	if strings.TrimSpace(input.ID) == "" {
		return api.OKResult{}, feedbackHelper.InvalidInput("缺少确认作者提醒已读所需的参数")
	}
	sessionKey := input.SessionKey
	input.SessionKey = ""
	result, err := feedbackHelper.Do[api.OKResult](service.callerContext(), sessionKey, api.ReadNoticeAPI, nil, input)
	if err == nil {
		feedbackHelper.RecordWrite("readNotice", "确认作者提醒已读")
	}
	return result, err
}
