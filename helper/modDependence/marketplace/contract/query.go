package contract

// 文件职责：定义 Mod 统一市场查询的跨层契约。
// 使用：service/modDependence 构造查询条件，marketplace helper 根据该契约分派 Thunderstore 或手动来源。
// 边界：只描述查询参数，不保存查询结果，不发起 HTTP 请求，不读写缓存或安装记录。

import sourceConstant "DysonSphereBox/constant/modDependence/source"

// RemotePackageQuery 表示统一市场查询条件。
// 职责：承载统一市场查询所需来源、分页、搜索和缓存策略。
// 使用：服务层传入来源、分页与搜索词，Provider 负责把来源响应转换为统一模型。
// 边界：只描述查询参数，不保存查询结果，不执行 HTTP 请求。
type RemotePackageQuery struct {
	// Source 是统一市场来源。
	// 职责：决定请求 Thunderstore 或手动来源。
	// 使用：ListRemotePackages 根据该字段分派 Provider。
	// 边界：为空时 normalizeRemotePackageQuery 回退到 Thunderstore。
	Source sourceConstant.ModSource
	// Page 是分页页码。
	// 职责：控制返回列表的页数。
	// 使用：Thunderstore 本地分页读取。
	// 边界：小于 1 时归一化为 1。
	Page int
	// PageSize 是分页大小。
	// 职责：控制每页返回条数。
	// 使用：Thunderstore 本地分页读取。
	// 边界：小于 1 时归一化为 10，大于 100 时截断为 100。
	PageSize int
	// Keyword 是市场搜索关键字。
	// 职责：过滤包名称、描述、作者和命名空间。
	// 使用：Thunderstore 本地列表过滤时读取。
	// 边界：空字符串表示不过滤。
	Keyword string
	// IncludeDeprecated 表示是否包含 Thunderstore 废弃包。
	// 职责：控制 Thunderstore 来源是否展示 deprecated 包。
	// 使用：listThunderstoreRemotePackages 过滤时读取。
	// 边界：对手动来源无实际影响。
	IncludeDeprecated bool
	// UseCache 表示是否优先复用本地市场缓存。
	// 职责：控制 Thunderstore 全量索引读取是否先尝试缓存。
	// 使用：thunderstoreindex.LoadPackages 读取缓存策略时使用。
	// 边界：即使为 false，请求失败时仍可能回退旧缓存以保持可用性。
	UseCache bool
}
