package source

// 文件职责：声明 Gitee 更新源的默认连接参数和初始化配置结构。
// 使用：NewSource 读取 Config 与默认值创建 Source。
// 边界：只描述更新源配置，不保存仓库 owner/repo、不执行 HTTP 请求、不处理版本选择。

import (
	"net/http"
	"time"
)

// Source 默认配置常量组描述 Release API 的连接默认值。
// 职责：提供公开 Gitee API 地址和默认 HTTP 超时时间。
// 使用：NewSource 在调用方未显式配置时读取。
// 边界：仅用于应用自更新源，不覆盖 core 全局 HTTP 客户端配置。
const (
	// defaultGiteeAPIBaseURL 是 Gitee Release API 的默认基础地址。
	// 职责：在 Config.APIBaseURL 为空时作为兜底。
	// 使用：NewSource 初始化 apiBaseURL 时读取。
	// 边界：只包含 API 根路径，不包含仓库 owner/repo。
	defaultGiteeAPIBaseURL = "https://gitee.com/api/v5"
	// defaultHTTPTimeout 是 Gitee 发布元数据请求默认超时时间。
	// 职责：避免更新检查无限阻塞，不限制附件下载耗时。
	// 使用：NewSource 在未注入 HTTPClient 时创建默认客户端。
	// 边界：调用方提供 HTTPClient 时该值不生效。
	defaultHTTPTimeout = 30 * time.Second
	// defaultDownloadTimeout 是更新附件下载的完整请求超时。
	// 职责：允许大体积更新包在慢速网络下完成读取；使用：NewSource 创建附件客户端时读取。
	// 边界：涵盖重定向和响应体读取，仍受调用方 context 的更短期限或取消约束。
	defaultDownloadTimeout = 30 * time.Minute
)

// Config 描述 Gitee Release 更新源配置。
// 职责：承载更新源初始化所需的 API 地址、访问令牌和 HTTP 客户端。
// 使用：NewSource 接收该配置创建 Source。
// 边界：公开仓库 Token 可为空；该配置不保存 owner/repo，仓库由 repository 单独提供。
type Config struct {
	// APIBaseURL 是 Gitee API 基础地址。
	// 职责：支持默认 Gitee 或兼容镜像地址。
	// 使用：NewSource 校验并规范化该字段。
	// 边界：为空时使用 defaultGiteeAPIBaseURL。
	APIBaseURL string
	// Token 是可选的 Gitee access_token。
	// 职责：支持私有仓库或更高访问额度的 API 请求。
	// 使用：newRequest 在非空时追加到请求 query。
	// 边界：当前不做持久化和脱敏日志输出。
	Token string
	// HTTPClient 是可选的自定义 HTTP 客户端。
	// 职责：允许调用方注入代理、测试传输或元数据请求超时。
	// 使用：NewSource 使用该客户端读取元数据，并复制其配置用于附件下载。
	// 边界：为空时使用默认客户端；附件副本的 Timeout 独立使用 defaultDownloadTimeout，不修改传入对象。
	HTTPClient *http.Client
}
