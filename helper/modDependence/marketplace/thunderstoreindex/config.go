package thunderstoreindex

// 文件职责：读取 Thunderstore 索引适配器的用户优先模式配置。
// 使用：LoadPackages 在选择 provider 顺序前调用，保证设置页切换能影响市场和安装链路。
// 边界：只读取配置并归一化模式值，不写入配置、不验证远端接口可用性。

import (
	modThunderstoreConstant "DysonSphereBox/constant/modDependence/thunderstore"
	"DysonSphereBox/core"
)

// preferredIndexMode 返回当前 Thunderstore 索引优先模式。
// 职责：从全局配置读取用户选择，并把空值或非法值归一化为新版分片优先。
// 使用：orderedProviders 构造 provider 顺序时调用。
// 边界：配置系统未初始化或读取失败时使用默认新版优先，不把配置错误传播给市场加载链路。
func preferredIndexMode() string {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.ThunderstoreIndexMode == nil {
		return modThunderstoreConstant.IndexModeChunked
	}

	mode, err := core.GLOBAL.Configure.ThunderstoreIndexMode.Get()
	if err != nil {
		return modThunderstoreConstant.IndexModeChunked
	}

	switch mode {
	case modThunderstoreConstant.IndexModeLegacy:
		return modThunderstoreConstant.IndexModeLegacy
	case modThunderstoreConstant.IndexModeChunked:
		return modThunderstoreConstant.IndexModeChunked
	default:
		return modThunderstoreConstant.IndexModeChunked
	}
}

// orderedProviders 按用户配置返回 Thunderstore 索引 provider 调用顺序。
// 职责：让新旧索引链路互为兜底，并支持设置页选择优先使用哪一种。
// 使用：LoadPackages 在远端刷新时调用。
// 边界：只调整顺序，不跳过兜底 provider；服务层始终只看到统一 LoadPackages 契约。
func orderedProviders() []indexProvider {
	if preferredIndexMode() == modThunderstoreConstant.IndexModeLegacy {
		return []indexProvider{
			legacyIndexProvider{},
			chunkedIndexProvider{},
		}
	}

	return []indexProvider{
		chunkedIndexProvider{},
		legacyIndexProvider{},
	}
}
