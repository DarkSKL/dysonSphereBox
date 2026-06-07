package response

// PackageListResponse 表示整合包列表接口的完整响应。
// 用途：服务层解码远端接口结果，并把 Data 中的 DTO 直接返回给前端。
type PackageListResponse struct {
	Code int             `json:"code"`
	Data PackageListData `json:"data"`
	Msg  string          `json:"msg"`
}

// PackageListData 表示整合包列表分页数据。
// 边界：List 使用前端 DTO，避免把 GORM 模型里的 time.Time 暴露给 Wails 生成器。
type PackageListData struct {
	List     []ModDependencyPackageDTO `json:"list"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"pageSize"`
}

// ModDependencyDTO 表示前端可安全消费的 Mod 依赖信息。
// 边界：UpdateTime 使用字符串承接远端时间，GameVersions 保留原始 JSON 形状。
type ModDependencyDTO struct {
	ID              uint        `json:"id"`
	ModName         *string     `json:"modName"`
	ModVersion      *string     `json:"modVersion"`
	ModDescription  *string     `json:"modDescription"`
	GameVersions    interface{} `json:"gameVersions"`
	UpdateTime      *string     `json:"updateTime"`
	Source          *string     `json:"source"`
	Url             *string     `json:"url"`
	Trustworthiness *int64      `json:"trustworthiness"`
	Config          *string     `json:"config"`
	Ex              *string     `json:"ex"`
}

// ModDependencyPackageDTO 表示前端可安全消费的 Mod 整合包信息。
// 边界：不携带 GORM 标签和 time.Time，防止 Wails 模块生成时解析标准库结构体。
type ModDependencyPackageDTO struct {
	ID                 uint               `json:"id"`
	PackageName        *string            `json:"packageName"`
	PackageVersion     *string            `json:"packageVersion"`
	PackageDescription *string            `json:"packageDescription"`
	PackageSize        *int64             `json:"packageSize"`
	DownloadCount      *int64             `json:"downloadCount"`
	DependencyCount    *int64             `json:"dependencyCount"`
	GameVersions       interface{}        `json:"gameVersions"`
	Tags               interface{}        `json:"tags"`
	Source             *string            `json:"source"`
	Url                *string            `json:"url"`
	CoverUrl           *string            `json:"coverUrl"`
	Author             *string            `json:"author"`
	Status             *int               `json:"status"`
	Trustworthiness    *int64             `json:"trustworthiness"`
	UpdateTime         *string            `json:"updateTime"`
	Config             *string            `json:"config"`
	Ex                 *string            `json:"ex"`
	Dependencies       []ModDependencyDTO `json:"dependencies"`
}
