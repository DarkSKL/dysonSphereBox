package modDependence

// 文件职责：维护模组文件下载代理的外部 API 路径。
// 使用：artifact 下载 helper 通过当前服务器 HTTPClient 解析该相对路径。
// 边界：只定义固定路径，不负责查询参数、鉴权、HTTP 执行或错误处理。

// APIURLDownloadPackage 是当前盒子服务器精确版本模组代理下载路径。
// 职责：集中描述协议规定的代理路由。
// 使用：客户端构造代理 GET 请求时作为相对路径解析。
// 边界：坐标只通过 namespace/name/version 查询参数传递，不允许调用方覆盖来源或附加用户标识。
const APIURLDownloadPackage = "/modDownload/downloadPackage"
