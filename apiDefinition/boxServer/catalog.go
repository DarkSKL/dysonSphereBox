package boxServer

// 文件职责：汇总盒子客户端需要装配的 HTTP 接口描述。
// 使用：应用启动和契约验证调用 Endpoints。
// 边界：只引用各业务描述，第三方请求和独立联机协议各自管理。

import (
	api "DysonSphereBox/apiDefinition"
	"DysonSphereBox/apiDefinition/feedback"
	"DysonSphereBox/apiDefinition/forum"
	"DysonSphereBox/apiDefinition/modTranslation"
	"DysonSphereBox/apiDefinition/system"
)

// Endpoints 职责：返回待加载的盒子服务器 HTTP 目录；使用：启动验证；边界：描述由各业务包定义。
func Endpoints() []*api.ApiDesc {
	endpoints := system.Endpoints()
	endpoints = append(endpoints, forum.Endpoints()...)
	endpoints = append(endpoints, feedback.Endpoints()...)
	return append(endpoints, modTranslation.TranslateBatchAPI)
}
