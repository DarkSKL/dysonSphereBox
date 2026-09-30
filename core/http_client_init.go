package core

// 文件职责：从应用配置初始化应用服务器客户端。
// 使用：InitDatabase 完成 Configure 初始化后调用 InitServerClient；配置服务保存服务器地址时调用 NewServerClient。
// 边界：Configure 未初始化时返回错误；历史地址格式错误会降级为默认服务器地址，允许用户后续在设置页修复。

import (
	"errors"
	"strings"

	systemConstant "DysonSphereBox/constant/system"
)

// InitServerClient 从 Configure.ServerAddress 读取地址并创建应用服务器客户端。
// 职责：把数据库配置中的服务器地址转换为可复用 HTTPClient。
// 使用：数据库和配置初始化完成后调用；历史配置无效时降级为默认地址，避免阻塞设置页修复。
// 边界：Configure 未初始化或配置读取失败时返回错误。
func InitServerClient() (*HTTPClient, error) {
	if GLOBAL.Configure == nil || GLOBAL.Configure.ServerAddress == nil {
		return nil, errors.New("configure server address is not initialized")
	}

	address, err := GLOBAL.Configure.ServerAddress.Get()
	if err != nil {
		return nil, err
	}

	httpClient, err := NewServerClient(address)
	if err != nil {
		return NewHTTPClient(systemConstant.DefaultServerAddress)
	}

	return httpClient, nil
}

// NewServerClient 按服务器地址配置值创建 HTTP 客户端。
// 职责：统一“空值使用默认服务器地址”的规则，供启动初始化和配置保存复用。
// 使用：InitServerClient 与 ConfigurationService.SetConfig 保存 System/ServerAddress 前调用。
// 边界：只校验地址格式，不访问网络；地址格式无效时返回错误。
func NewServerClient(address string) (*HTTPClient, error) {
	if strings.TrimSpace(address) == "" {
		address = systemConstant.DefaultServerAddress
	}

	return NewHTTPClient(address)
}
