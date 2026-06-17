package thunderstore

// 文件职责：定义 Thunderstore 索引模式相关配置键和值。
// 使用：设置页通过配置服务写入优先模式，thunderstoreindex 适配器读取后决定 provider 顺序。
// 边界：只保存稳定字符串常量，不读取数据库、不发起网络请求、不参与安装决策。

const (
	// IndexModeConfigGroup 是 Thunderstore 索引模式配置分组。
	// 职责：为 system_configs 表中的 Thunderstore 设置提供稳定 group。
	// 使用：前端设置页和后端索引适配器读写 ThunderstoreIndexMode 时使用。
	// 边界：不代表数据来源，具体 provider 顺序由 thunderstoreindex 解释。
	IndexModeConfigGroup = "ModDependence"
	// IndexModeConfigKey 是 Thunderstore 索引模式配置键名。
	// 职责：为用户选择的索引优先模式提供稳定 key。
	// 使用：ConfigurationService.GetConfig/SetConfig 和 thunderstoreindex 适配器读取时使用。
	// 边界：值为空或非法时由适配器按新版优先处理。
	IndexModeConfigKey = "ThunderstoreIndexMode"
	// IndexModeChunked 表示优先使用新版 package-listing-index 分片链路。
	// 职责：作为 Thunderstore 新版索引 provider 的配置值。
	// 使用：设置页选中“新版优先”或默认配置时写入/读取。
	// 边界：新版失败仍会回退旧版和缓存，不保证只使用新版。
	IndexModeChunked = "chunked"
	// IndexModeLegacy 表示优先使用旧版 api/v1/package 完整链路。
	// 职责：作为 Thunderstore 旧版索引 provider 的配置值。
	// 使用：用户需要验证旧方案或新版链路异常时可从设置页切换。
	// 边界：旧版失败仍会回退新版和缓存，不保证只使用旧版。
	IndexModeLegacy = "legacy"
)
