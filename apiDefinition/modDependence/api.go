package modDependence

// 文件职责：定义外部接口契约和前后端传输 DTO。
// 使用：由 service/helper 在请求远端接口或生成 Wails 类型时引用。
// 边界：不发起网络请求，不读写数据库，不承载业务决策。

const (
	// APIURLPackageList 是官方 Mod 整合包分页接口路径。
	// 职责：集中保存旧官方整合包分页接口相对路径。
	// 使用：MarketplaceService 构造官方源分页请求时引用。
	// 边界：只描述路径，不发起请求，不拼接查询参数。
	APIURLPackageList = "/moddeppkg/getModDependencyPackageList"
	// APIURLDownloadRecord 是官方源下载计数记录接口路径。
	// 职责：集中保存官方源下载计数记录接口相对路径。
	// 使用：安装器在真实下载官方资源前记录下载事件时引用。
	// 边界：只描述路径，不决定记录时机或错误处理策略。
	APIURLDownloadRecord = "/download/record"
)
