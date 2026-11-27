package account

// 文件职责：将账号请求策略的失败转换为现有提示和诊断记录。
// 使用：账号、验证码和资料调用共享的 HTTP 适配器。
// 边界：诊断只记录脱敏阶段与状态，不记录账号凭据或响应正文。

import (
	"context"
	"errors"
	"fmt"

	api "DysonSphereBox/apiDefinition"
	"DysonSphereBox/core/request"
)

// accountRequest 委托适配器执行请求，再用原有账号提示表达失败。
func accountRequest[T any](ctx context.Context, desc *api.ApiDesc, input request.Input) (T, error) {
	result, err := request.Do[T](ctx, ServerRequests(), desc, input)
	if err == nil || errors.Is(err, errLogoutWithoutToken) {
		return result, err
	}
	status, code := 0, 0
	if failure := request.Details(err); failure != nil {
		status, code = failure.Status, failure.Code
		err = accountFailure(failure)
	}
	recordRequestFailure(desc.Method, desc.URL, "accountResponse", status, code, err)
	return result, err
}

// accountFailure 保留账号界面的可读错误，日志仍由现有脱敏诊断生成。
func accountFailure(failure *request.Error) error {
	switch failure.Kind {
	case request.Unavailable:
		return errors.New("服务器连接尚未初始化")
	case request.Unauthorized, request.AuthRequired:
		return errors.New("登录已失效，请重新登录。")
	case request.Stale:
		return errors.New("账号会话或服务器已切换，请重试。")
	case request.TransportFailed:
		return errors.New("无法连接账号服务器，请稍后重试。")
	case request.ReadFailed:
		return errors.New("账号响应读取失败或超过4MiB")
	case request.InvalidEnvelope:
		return fmt.Errorf("账号响应格式无效 (HTTP %d)", failure.Status)
	case request.InvalidData:
		return errors.New("账号响应字段格式无效")
	case request.PasswordRequired:
		if failure.Message == "" {
			return errors.New("请先修改账号密码。")
		}
	}
	if failure.Message != "" {
		return errors.New(failure.Message)
	}
	return fmt.Errorf("账号请求失败 (HTTP %d)", failure.Status)
}
