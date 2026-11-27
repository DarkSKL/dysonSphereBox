package forum

// 文件职责：将论坛接口接入盒子服务器请求适配器并保留前端错误契约。
// 使用：论坛 JSON 读取、写入与文件传输。
// 边界：网络层产生的底层诊断在 Go 内映射为固定文案。

import (
	"context"
	"errors"
	"net/http"

	api "DysonSphereBox/apiDefinition"
	forumAPI "DysonSphereBox/apiDefinition/forum"
	"DysonSphereBox/core/request"
	"DysonSphereBox/core/requestcache"
	"DysonSphereBox/helper/system/account"
)

// DoJSON 职责：运行论坛接口描述；使用：查询、写入和上传回执；边界：保留论坛业务错误种类。
func DoJSON[T any](ctx context.Context, desc *api.ApiDesc, input request.Input) (T, error) {
	result, err := request.Do[T](ctx, account.ServerRequests(), desc, input)
	if err != nil {
		return result, forumRequestFailure(desc, err)
	}
	return result, nil
}

// forumRequestFailure 将请求阶段失败映射到论坛前端支持的稳定错误。
func forumRequestFailure(desc *api.ApiDesc, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return failure("timeout", "论坛请求超时，请稍后重试")
	case errors.Is(err, context.Canceled):
		return failure("canceled", "论坛请求已取消")
	case errors.Is(err, requestcache.ErrInvalidated):
		return failure("requestFailed", "论坛数据已更新，请重新读取")
	}
	if detail := request.Details(err); detail != nil {
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
			return InvalidInput("论坛请求数据无效或超过大小限制")
		case request.BuildFailed:
			return failure("requestFailed", "无法构造论坛请求")
		case request.TransportFailed:
			if desc == forumAPI.DownloadAPI {
				return failure("requestFailed", "下载资源失败，请检查网络后重试")
			}
			return failure("requestFailed", "无法连接论坛服务，请稍后重试")
		case request.ReadFailed:
			return failure("requestFailed", "论坛响应读取失败或超过大小限制")
		case request.InvalidEnvelope:
			return failure("requestFailed", "论坛响应格式无效")
		case request.InvalidData:
			return failure("requestFailed", "论坛响应数据解析失败")
		case request.BusinessFailed:
			return businessFailure(detail.Code, detail.Data)
		case request.HTTPFailed:
			if detail.Status == http.StatusNotFound {
				if desc == forumAPI.DownloadAPI {
					return failure("notFound", "资源不存在或已下架")
				}
				return failure("unsupported", "当前服务器暂未提供社区论坛功能")
			}
		}
	}
	var domain *requestError
	if errors.As(err, &domain) {
		return domain
	}
	return failure("requestFailed", "论坛服务请求失败，请稍后重试")
}
