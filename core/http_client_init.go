package core

// 文件职责：从应用配置初始化全局 HTTP 客户端。
// 使用：InitDatabase 完成 Configure 初始化后调用 InitHTTPClient。
// 边界：Configure 未初始化时返回错误；BaseURL 格式错误会降级为空 BaseURL 客户端，允许用户后续在设置页修复。

import "errors"

// InitHTTPClient 从 Configure.BaseURL 读取地址并创建全局 HTTP 客户端。
// 职责：把数据库配置中的 BaseURL 转换为可复用 HTTPClient。
// 使用：数据库和配置初始化完成后调用；历史配置无效时会降级为空 BaseURL，避免阻塞设置页修复。
// 边界：Configure 未初始化时返回错误；BaseURL 格式错误时创建空 BaseURL 客户端作为可恢复状态。
func InitHTTPClient() (*HTTPClient, error) {
	if GLOBAL.Configure == nil || GLOBAL.Configure.BaseURL == nil {
		return nil, errors.New("configure base url is not initialized")
	}

	baseURL, err := GLOBAL.Configure.BaseURL.Get()
	if err != nil {
		return nil, err
	}

	httpClient, err := NewHTTPClient(baseURL)
	if err != nil {
		return NewHTTPClient("")
	}

	return httpClient, nil
}
