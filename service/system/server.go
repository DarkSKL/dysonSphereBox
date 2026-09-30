package system

// 文件职责：提供应用服务器连接相关的 Wails 服务。
// 使用：全局设置页通过生成绑定测试服务器地址是否可用。
// 边界：只校验输入并编排连接测试 helper；地址保存仍由 ConfigurationService.SetConfig 负责，请求转发由 helper/system/server 中间件负责。

import (
	"context"
	"fmt"

	serverHelper "DysonSphereBox/helper/system/server"
	"DysonSphereBox/utils"
)

// ServerService 提供前端可调用的服务器连接能力。
// 职责：对用户填写但尚未保存的服务器地址执行只读连接测试。
// 使用：全局设置页“测试连接”按钮通过 Wails 绑定调用。
// 边界：不保存配置、不修改账号会话；测试请求在后端发出，不受 WebView 跨域限制。
type ServerService struct{}

// ProbeServerAddress 测试指定服务器地址是否为可用的账号服务。
// 职责：归一化地址后调用连接测试 helper 并返回测试结论。
// 使用：全局设置页用户点击“测试连接”时调用，传入输入框中的草稿地址。
// 边界：地址格式无效时返回错误；网络失败、超时或响应结构不符以 ProbeResult.OK=false 返回，不视为调用错误。
func (service *ServerService) ProbeServerAddress(address string) (*serverHelper.ProbeResult, error) {
	normalized, err := utils.NormalizeHTTPBaseURL(address)
	if err != nil {
		return nil, fmt.Errorf("服务器地址无效：%w", err)
	}
	return serverHelper.Probe(context.Background(), normalized), nil
}
