package notification

// 文件职责：接入收件箱请求并保持页面可识别的失败文案。
// 使用：通知服务读取和标读。
// 边界：接口认证、响应限制和会话验收由请求层执行。

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	api "DysonSphereBox/apiDefinition"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/system/account"
)

// Request 职责：执行已声明的通知 JSON 请求；使用：收件箱 Wails 调用；边界：分页与标读参数由业务校验。
func Request[T any](ctx context.Context, sessionKey string, desc *api.ApiDesc, query url.Values, payload *systemAPI.NotificationReadRequest) (T, error) {
	input := request.Input{SessionKey: sessionKey, Query: query}
	if payload != nil {
		input.Payload = payload
	}
	result, err := request.Do[T](ctx, account.ServerRequests(), desc, input)
	if err != nil {
		return result, notificationFailure(err)
	}
	return result, nil
}

// notificationFailure 映射认证和收件箱响应错误，避免把请求详情直接交给界面。
func notificationFailure(err error) error {
	failure := request.Details(err)
	if failure == nil {
		return errors.New("账号会话已切换或请求已取消")
	}
	switch failure.Kind {
	case request.AuthRequired:
		return errors.New("请先登录并完成账号验证")
	case request.Unauthorized:
		return errors.New("登录已失效，请重新登录")
	case request.PasswordRequired:
		return errors.New("请先修改账号密码")
	case request.Stale:
		return errors.New("账号会话或凭据已更新")
	case request.TransportFailed:
		return errors.New("连接消息服务器失败，请稍后重试")
	case request.ReadFailed:
		return errors.New("消息响应读取失败或超过 4 MiB")
	case request.InvalidEnvelope:
		return errors.New("消息响应格式无效")
	case request.InvalidData:
		return errors.New("消息响应数据格式无效")
	case request.BusinessFailed:
		if failure.Message != "" {
			return errors.New(failure.Message)
		}
		return errors.New("消息接口暂不可用")
	case request.HTTPFailed:
		if failure.Status == http.StatusNotFound {
			return errors.New("当前服务器暂未提供消息收件箱")
		}
	}
	return fmt.Errorf("消息请求失败（HTTP %d）", failure.Status)
}
