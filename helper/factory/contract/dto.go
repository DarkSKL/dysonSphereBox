package factorycontract

// 文件职责：定义 factory 桥接 DTO；使用：Wails 生成绑定；边界：不作为数据库模型，不做业务决策。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/quant"
)

// Status 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type Status struct {
	// Code 职责：保存OK 或稳定错误码；使用：类型化传递；边界：不隐式访问文件。
	Code string `json:"code"`
	// Message 职责：保存中文诊断；使用：类型化传递；边界：不隐式访问文件。
	Message string `json:"message"`
}

// DatasetDetail 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type DatasetDetail struct {
	// Info 职责：保存数据集摘要；使用：类型化传递；边界：不隐式访问文件。
	Info quant.DatasetInfo `json:"info"`
	// Items 职责：保存物品列表；使用：类型化传递；边界：不隐式访问文件。
	Items []quant.Item `json:"items"`
	// Recipes 职责：保存原始配方序列；使用：类型化传递；边界：不隐式访问文件。
	Recipes []quant.Recipe `json:"recipes"`
}

// DecodeRequest 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type DecodeRequest struct {
	// Blueprint 职责：保存蓝图文本；使用：类型化传递；边界：不隐式访问文件。
	Blueprint string `json:"blueprint"`
	// Strict 职责：保存是否校验 MD5F，缺省 true；使用：类型化传递；边界：不隐式访问文件。
	Strict *bool `json:"strict,omitempty"`
}

// EncodeRequest 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type EncodeRequest struct {
	// Document 职责：保存完整文档；使用：类型化传递；边界：不隐式访问文件。
	Document blueprint.Document `json:"document"`
	// Envelope 职责：保存信封版本，缺省文档版本；使用：类型化传递；边界：不隐式访问文件。
	Envelope *int `json:"envelope,omitempty"`
	// Recompute 职责：保存兼容字段，出口始终重算保证安全；使用：类型化传递；边界：不隐式访问文件。
	Recompute bool `json:"recompute"`
}

// InspectRequest 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type InspectRequest struct {
	// Blueprint 职责：保存待反算文本；使用：类型化传递；边界：不隐式访问文件。
	Blueprint string `json:"blueprint"`
	// Dataset 职责：保存数据集键；使用：类型化传递；边界：不隐式访问文件。
	Dataset string `json:"dataset"`
}

// InspectedMachine 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type InspectedMachine struct {
	// BuildingIndex 职责：保存建筑数组下标；使用：类型化传递；边界：不隐式访问文件。
	BuildingIndex int `json:"buildingIndex"`
	// RecipeID 职责：保存配方编号；使用：类型化传递；边界：不隐式访问文件。
	RecipeID int `json:"recipeId"`
	// MachineID 职责：保存设施原型；使用：类型化传递；边界：不隐式访问文件。
	MachineID int `json:"machineId"`
	// Outputs 职责：保存无喷涂理论产出每分钟；使用：类型化传递；边界：不隐式访问文件。
	Outputs []quant.ItemRate `json:"outputs"`
}

// Inspection 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type Inspection struct {
	// Summary 职责：保存建筑聚合；使用：类型化传递；边界：不隐式访问文件。
	Summary blueprint.Summary `json:"summary"`
	// Machines 职责：保存每台制造建筑的理论值；使用：类型化传递；边界：不隐式访问文件。
	Machines []InspectedMachine `json:"machines"`
	// Outputs 职责：保存聚合理论产出；使用：类型化传递；边界：不隐式访问文件。
	Outputs []quant.ItemRate `json:"outputs"`
	// NetRates 职责：保存理论净产出，原料消耗为负；使用：类型化传递；边界：不隐式访问文件。
	NetRates []quant.ItemRate `json:"netRates"`
	// UnknownIDs 职责：保存数据集未识别原型；使用：类型化传递；边界：不隐式访问文件。
	UnknownIDs []int `json:"unknownIds"`
	// Warnings 职责：保存喷涂/配方识别限制；使用：类型化传递；边界：不隐式访问文件。
	Warnings []string `json:"warnings"`
}

// ChunkRequest 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type ChunkRequest struct {
	// Token 职责：保存生成内容令牌；使用：类型化传递；边界：不隐式访问文件。
	Token string `json:"token"`
	// Offset 职责：保存字节偏移；使用：类型化传递；边界：不隐式访问文件。
	Offset int `json:"offset"`
}

// ChunkResult 职责：承载桥接边界；使用：生成器服务；边界：具体验证由服务执行。
type ChunkResult struct {
	// Text 职责：保存不超过 256 KiB 的片段；使用：类型化传递；边界：不隐式访问文件。
	Text string `json:"text"`
	// NextOffset 职责：保存后续偏移；使用：类型化传递；边界：不隐式访问文件。
	NextOffset int `json:"nextOffset"`
	// Done 职责：保存是否已读完；使用：类型化传递；边界：不隐式访问文件。
	Done bool `json:"done"`
}

// WorkspaceRequest 职责：保存可复核的计算方案快照；使用：导出方案 JSON；边界：不包含账号、文件路径或实际资源库内容。
type WorkspaceRequest struct {
	// Name 职责：保存用户标题；使用：默认文件名；边界：服务清理路径分隔符。
	Name string `json:"name"`
	// Request 职责：保存目标、人工配方和量化设置；使用：复核与离线记录；边界：导出前重新求解确认一致性。
	Request quant.Request `json:"request"`
	// Plan 职责：保存对应的量化结果；使用：后续审阅；边界：必须与当前数据集重新求解结果完全一致。
	Plan quant.Result `json:"plan"`
	// Setting 职责：保存布局参数；使用：复现生成输入；边界：记录不等于实际游戏验收。
	Setting build.Setting `json:"setting"`
	// SurfaceLayout 职责：保存行星网格布局偏好；使用：复现纬圈预检输入；边界：不包含游戏存档位置或碰撞验收结果。
	SurfaceLayout *build.SurfaceLayout `json:"surfaceLayout,omitempty"`
	// Mode 职责：保存生成策略；使用：复现生成输入；边界：仅接受当前支持的 1..4。
	Mode int `json:"mode"`
}

// DatasetResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type DatasetResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data []quant.DatasetInfo `json:"data"`
}

// DatasetDetailResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type DatasetDetailResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *DatasetDetail `json:"data"`
}

// QuantResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type QuantResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *quant.Result `json:"data"`
}

// DecodeResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type DecodeResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *blueprint.Document `json:"data"`
}

// EncodeResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type EncodeResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *blueprint.EncodeResult `json:"data"`
}

// GenerateResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type GenerateResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *build.Result `json:"data"`
}

// InspectResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type InspectResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *Inspection `json:"data"`
}

// ChunkResponse 职责：返回具体类型；使用：Wails 调用；边界：失败时 data 为空，不依赖泛型。
type ChunkResponse struct {
	// Status 职责：响应状态；使用：前端错误分支；边界：不携带大对象。
	Status
	// Data 职责：具体载荷；使用：成功读取；边界：失败为空。
	Data *ChunkResult `json:"data"`
}
