package modDependence

// 文件职责：定义外部接口契约和前后端传输 DTO。
// 使用：由 service/helper 在请求远端接口或生成 Wails 类型时引用。
// 边界：不发起网络请求，不读写数据库，不承载业务决策。

// PackageListResponse 表示整合包列表接口的完整响应。
// 职责：承载旧官方整合包分页接口的完整响应外壳。
// 使用：服务层解码远端接口结果，并把 Data 中的 DTO 直接返回给前端。
// 边界：只描述远端响应形状，不执行 HTTP 状态码或业务码判断。
type PackageListResponse struct {
	// Code 职责：保存旧官方接口业务状态码；使用：服务层判断接口业务是否成功；边界：HTTP 2xx 但 Code 非 0 仍视为失败。
	Code int `json:"code"`
	// Data 职责：保存旧官方整合包分页数据；使用：接口成功后返回给前端或转换为统一 DTO；边界：失败响应中可能为空值。
	Data PackageListData `json:"data"`
	// Msg 职责：保存旧官方接口业务消息；使用：失败时拼接错误；边界：成功时不参与安装逻辑。
	Msg string `json:"msg"`
}

// PackageListData 表示整合包列表分页数据。
// 职责：承载旧官方整合包列表、总数和分页参数。
// 使用：MarketplaceService 返回旧官方源分页结果时使用。
// 边界：List 使用前端 DTO，避免把 GORM 模型里的 time.Time 暴露给 Wails 生成器。
type PackageListData struct {
	// List 职责：保存当前页旧官方整合包 DTO；使用：旧市场列表和统一 DTO 转换；边界：空列表不代表请求失败。
	List []ModDependencyPackageDTO `json:"list"`
	// Total 职责：保存旧官方接口返回的总数；使用：前端分页器计算页数；边界：按官方接口语义为准，不在本地重新统计。
	Total int `json:"total"`
	// Page 职责：保存当前页码；使用：前端同步分页状态；边界：由服务层归一化后传给远端。
	Page int `json:"page"`
	// PageSize 职责：保存当前分页大小；使用：前端分页器和下一次请求复用；边界：实际值取决于远端接口响应。
	PageSize int `json:"pageSize"`
}

// ModDependencyDTO 表示前端可安全消费的 Mod 依赖信息。
// 职责：描述旧官方源中单个依赖 Mod 的展示和下载元数据。
// 使用：旧整合包 DTO 的 Dependencies 字段和前端依赖明细展示使用。
// 边界：UpdateTime 使用字符串承接远端时间，GameVersions 保留原始 JSON 形状。
type ModDependencyDTO struct {
	// ID 职责：保存旧官方依赖数字 ID；使用：传统安装器和旧 DTO 兼容；边界：统一来源新逻辑不以该字段作为唯一身份。
	ID uint `json:"id"`
	// ModName 职责：保存旧官方依赖名称；使用：前端展示和统一依赖 DTO 转换；边界：指针为空时转换层回退为空字符串。
	ModName *string `json:"modName"`
	// ModVersion 职责：保存旧官方依赖版本；使用：安装明细和统一依赖 key 生成；边界：不在 DTO 层解析语义化版本。
	ModVersion *string `json:"modVersion"`
	// ModDescription 职责：保存旧官方依赖说明；使用：前端展示和搜索扩展；边界：不做 Markdown 或 HTML 清洗。
	ModDescription *string `json:"modDescription"`
	// GameVersions 职责：保存旧官方依赖适配游戏版本原始结构；使用：前端展示或后续筛选；边界：DTO 层不解析具体格式。
	GameVersions interface{} `json:"gameVersions"`
	// UpdateTime 职责：保存旧官方依赖更新时间文本；使用：展示和排序扩展；边界：不解析时间格式。
	UpdateTime *string `json:"updateTime"`
	// Source 职责：保存旧官方依赖来源文本；使用：兼容历史接口返回；边界：统一来源判断优先使用转换后的 Source 字段。
	Source *string `json:"source"`
	// Url 职责：保存旧官方依赖下载或页面地址；使用：传统安装器下载和统一 DTO 转换；边界：可为空，下载前由安装器校验。
	Url *string `json:"url"`
	// Trustworthiness 职责：保存旧官方可信度数值；使用：市场展示和统一 DTO 映射；边界：不作为安全校验依据。
	Trustworthiness *int64 `json:"trustworthiness"`
	// Config 职责：保存旧官方依赖配置文本；使用：兼容历史接口字段；边界：当前 DTO 层不解析配置内容。
	Config *string `json:"config"`
	// Ex 职责：保存旧官方依赖扩展文本；使用：兼容历史接口字段和排障；边界：不参与安装决策。
	Ex *string `json:"ex"`
}

// ModDependencyPackageDTO 表示前端可安全消费的 Mod 整合包信息。
// 职责：描述旧官方源整合包及其依赖列表。
// 使用：旧市场接口返回给前端，也可作为传统安装器入参。
// 边界：不携带 GORM 标签和 time.Time，防止 Wails 模块生成时解析标准库结构体。
type ModDependencyPackageDTO struct {
	// ID 职责：保存旧官方整合包数字 ID；使用：传统安装器、下载计数和旧状态映射；边界：统一来源新逻辑以 Key 为唯一身份。
	ID uint `json:"id"`
	// PackageName 职责：保存旧官方整合包名称；使用：市场展示、搜索和统一 DTO 转换；边界：指针为空时转换层回退为空字符串。
	PackageName *string `json:"packageName"`
	// PackageVersion 职责：保存旧官方整合包版本；使用：安装记录和统一 key 生成；边界：不在 DTO 层解析语义化版本。
	PackageVersion *string `json:"packageVersion"`
	// PackageDescription 职责：保存旧官方整合包说明；使用：市场卡片和详情展示；边界：不做 Markdown 或 HTML 清洗。
	PackageDescription *string `json:"packageDescription"`
	// PackageSize 职责：保存旧官方整合包大小；使用：前端展示和统一 DTO 映射；边界：不作为下载完整性校验。
	PackageSize *int64 `json:"packageSize"`
	// DownloadCount 职责：保存旧官方下载次数；使用：市场热度展示和统一 DTO 映射；边界：不代表本地下载计数。
	DownloadCount *int64 `json:"downloadCount"`
	// DependencyCount 职责：保存旧官方依赖数量；使用：市场摘要展示；边界：实际安装依赖以 Dependencies 列表为准。
	DependencyCount *int64 `json:"dependencyCount"`
	// GameVersions 职责：保存旧官方适配游戏版本原始结构；使用：前端展示或筛选扩展；边界：DTO 层不解析具体格式。
	GameVersions interface{} `json:"gameVersions"`
	// Tags 职责：保存旧官方标签原始结构；使用：前端展示和统一 DTO 转换扩展；边界：不同接口可能返回不同 JSON 形状。
	Tags interface{} `json:"tags"`
	// Source 职责：保存旧官方来源文本；使用：兼容历史接口字段；边界：统一来源转换后会映射为 official。
	Source *string `json:"source"`
	// Url 职责：保存旧官方整合包下载或页面地址；使用：传统安装器下载和统一 DTO 映射；边界：可为空，下载前由安装器校验。
	Url *string `json:"url"`
	// CoverUrl 职责：保存旧官方封面地址；使用：市场卡片展示；边界：图片加载失败由前端兜底。
	CoverUrl *string `json:"coverUrl"`
	// Author 职责：保存旧官方作者文本；使用：市场展示和搜索匹配；边界：为空时不阻塞安装。
	Author *string `json:"author"`
	// Status 职责：保存旧官方包状态码；使用：后续隐藏下架包或诊断；边界：当前 DTO 层不解释具体枚举。
	Status *int `json:"status"`
	// Trustworthiness 职责：保存旧官方可信度数值；使用：市场展示和统一 DTO 映射；边界：不作为本地安全校验依据。
	Trustworthiness *int64 `json:"trustworthiness"`
	// UpdateTime 职责：保存旧官方更新时间文本；使用：前端展示和排序扩展；边界：DTO 层不解析时间格式。
	UpdateTime *string `json:"updateTime"`
	// Config 职责：保存旧官方整合包配置文本；使用：兼容历史接口字段；边界：当前 DTO 层不解析配置内容。
	Config *string `json:"config"`
	// Ex 职责：保存旧官方整合包扩展文本；使用：兼容历史接口字段和排障；边界：不参与安装决策。
	Ex *string `json:"ex"`
	// Dependencies 职责：保存旧官方整合包依赖列表；使用：传统安装器安装依赖和前端展示；边界：空列表代表无显式依赖而不是请求失败。
	Dependencies []ModDependencyDTO `json:"dependencies"`
}
