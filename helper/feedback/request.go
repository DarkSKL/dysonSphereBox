package feedback

// 文件职责：从反馈业务接入盒子服务器请求适配器。
// 使用：星系的 Wails 查询和写入。
// 边界：错误仅使用反馈业务码和客户端界面文案。

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	api "DysonSphereBox/apiDefinition"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/system/account"
)

// Do 职责：运行反馈接口描述并映射失败；使用：星系查询和写入；边界：保留现有的 Wails 业务错误种类。
func Do[T any](ctx context.Context, sessionKey string, desc *api.ApiDesc, query url.Values, payload any) (T, error) {
	result, err := request.Do[T](ctx, account.ServerRequests(), desc, request.Input{
		SessionKey: sessionKey, Query: query, Payload: payload,
	})
	if err == nil {
		return result, nil
	}
	return result, requestFailure(err)
}

// requestFailure 将认证、网络和业务协议失败转换为反馈界面识别的原因。
func requestFailure(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return failure("timeout", "反馈请求超时，请稍后重试")
	case errors.Is(err, context.Canceled):
		return failure("canceled", "反馈请求已取消")
	}
	detail := request.Details(err)
	if detail == nil {
		return failure("requestFailed", "反馈服务请求失败，请稍后重试")
	}
	switch detail.Kind {
	case request.AuthRequired:
		return failure("unauthorized", "请先登录并完成账号验证")
	case request.Unauthorized:
		return failure("unauthorized", "登录已失效，请重新登录")
	case request.PasswordRequired:
		return failure("passwordRequired", "请先修改账号密码")
	case request.Stale:
		return failure("stale", "账号会话或凭据已更新")
	case request.InvalidInput:
		return InvalidInput("反馈请求数据无效或超过大小限制")
	case request.TransportFailed:
		return failure("requestFailed", "无法连接反馈服务，请稍后重试")
	case request.BuildFailed:
		return failure("requestFailed", "无法构造反馈请求")
	case request.ReadFailed:
		return failure("requestFailed", "反馈响应读取失败或超过 4 MiB")
	case request.InvalidEnvelope:
		return failure("requestFailed", "反馈响应格式无效")
	case request.InvalidData:
		return failure("requestFailed", "反馈响应数据无效")
	case request.BusinessFailed:
		return businessFailure(detail.Code, detail.Data)
	case request.HTTPFailed:
		if detail.Status == http.StatusNotFound {
			return failure("unsupported", "当前服务器暂未提供反馈星系")
		}
	}
	return failure("requestFailed", "反馈服务请求失败，请稍后重试")
}
