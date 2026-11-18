package saveanalysis

// 文件职责：定义存档解析的查询契约。
// 使用：Wails 服务与后续页面读取；边界：数量为保存时快照，不表示理论产能、全仓库库存或可调拨量。

// Window 职责：说明历史计数时间窗；使用：每次查询随结果返回；边界：游戏时间以每秒 60 tick 计，非现实时间。
type Window struct {
	// Seconds 职责：标识窗口秒数；使用：与游戏统计筛选对应；边界：只接受支持的固定窗口。
	Seconds int64 `json:"seconds"`
	// EndTick 职责：标识该档的游戏时刻；使用：匹配同一存档；边界：不是查询时的实时游戏状态。
	EndTick int64 `json:"endTick"`
	// BucketTicks 职责：说明历史记录粒度；使用：解释窗口的离散采样；边界：不把计数转换为速率。
	BucketTicks int64 `json:"bucketTicks"`
}

// Counts 职责：保存历史生产和消耗件数；使用：汇总及星球详情；边界：缺记录用空指针表示，不冒充零产耗。
type Counts struct {
	// Produced 职责：记录窗口生产件数；使用：历史统计；边界：不是每分钟理论产能。
	Produced int64 `json:"produced"`
	// Consumed 职责：记录窗口消耗件数；使用：历史统计；边界：不含未来需求。
	Consumed int64 `json:"consumed"`
}

// ItemSummary 职责：汇总一种物品的已知历史和塔内数量；使用：全档物品列表；边界：未知物品保留原 ID。
type ItemSummary struct {
	// ItemID 职责：标识物品；使用：详情查询；边界：不要求百科已有该 ID。
	ItemID int64 `json:"itemId"`
	// History 职责：汇总有记录的厂区产耗；使用：历史展示；边界：全部缺失时为 null。
	History *Counts `json:"history"`
	// HistoryPlanets 职责：说明历史覆盖厂区数；使用：识别缺失；边界：不是全星系星球总数。
	HistoryPlanets int `json:"historyPlanets"`
	// Inventory 职责：合计塔内已存物品；使用：找料；边界：不包括运输途中、普通仓库或玩家背包。
	Inventory int64 `json:"inventory"`
	// Capacity 职责：合计该物品货槽容量；使用：与库存对照；边界：不代表需求量或空余可用运力。
	Capacity int64 `json:"capacity"`
}

// Slot 职责：描述单个物流塔货槽；使用：按星球寻找物资；边界：空货槽不进入物品查询。
type Slot struct {
	// StationID 职责：保存星球内塔 ID；使用：与 PlanetID 组合定位；边界：非全局唯一。
	StationID int64 `json:"stationId"`
	// GlobalID 职责：保存星际物流全局 ID；使用：辅助定位；边界：本地塔可为零。
	GlobalID int64 `json:"globalId"`
	// EntityID 职责：保存建筑实体 ID；使用：游戏内定位；边界：仅在所属星球有效。
	EntityID int64 `json:"entityId"`
	// Index 职责：保存原货槽序号；使用：与游戏货槽对应；边界：从零开始，跳过空槽不重排。
	Index int `json:"index"`
	// Inventory 职责：保存货槽库存；使用：快照展示；边界：不减去运输预订单。
	Inventory int64 `json:"inventory"`
	// Capacity 职责：保存解码后的货槽容量；使用：快照展示；边界：已经去除容量字段中的模式编码。
	Capacity int64 `json:"capacity"`
	// LocalLogic 职责：保存本地物流配置；使用：0 仓储、1 供应、2 需求；边界：不是实时缺口。
	LocalLogic int64 `json:"localLogic"`
	// RemoteLogic 职责：保存星际物流配置；使用：0 仓储、1 供应、2 需求；边界：本地塔不具备星际运输能力。
	RemoteLogic int64 `json:"remoteLogic"`
	// Stellar 职责：标识星际塔；使用：解释星际配置；边界：不表示有运输船。
	Stellar bool `json:"stellar"`
	// Collector 职责：标识轨道采集器；使用：区分货源类型；边界：采集速度不在本版计算。
	Collector bool `json:"collector"`
}

// PlanetItem 职责：返回某物品在单颗已建厂星球的情况；使用：物品详情；边界：不编造星球名称和坐标。
type PlanetItem struct {
	// PlanetID 职责：保存实档星球 ID；使用：关联生产与物流；边界：不是统计数组下标。
	PlanetID int64 `json:"planetId"`
	// History 职责：保存该厂区产耗；使用：详情显示；边界：缺记录为 null。
	History *Counts `json:"history"`
	// Slots 职责：列出该物品的非空货槽；使用：查找供需和库存；边界：无塔时返回空数组。
	Slots []Slot `json:"slots"`
}

// Summary 职责：返回全档物品目录；使用：解析后和更换时间窗时查询；边界：所有条目属于同一快照。
type Summary struct {
	// SHA256 职责：标识解析的内容快照；使用：验收及查询一致性核对；边界：不是 saveId。
	SHA256 string `json:"sha256"`
	// GameVersion 职责：返回实档版本；使用：说明兼容范围；边界：不代表本机安装版本。
	GameVersion string `json:"gameVersion"`
	// Window 职责：标识历史窗口；使用：与物品历史配套；边界：库存始终为保存瞬间。
	Window Window `json:"window"`
	// Items 职责：返回按 ID 排序的物品；使用：物品列表；边界：不依赖前端百科过滤。
	Items []ItemSummary `json:"items"`
}

// ItemDetail 职责：返回单物品的星球展开；使用：选中物品后查询；边界：不重新读取存档。
type ItemDetail struct {
	// SHA256 职责：标识当前解析内容；使用：关联全档汇总；边界：更换存档后变化。
	SHA256 string `json:"sha256"`
	// ItemID 职责：标识被查询的物品；使用：详情标题；边界：不存在时返回错误。
	ItemID int64 `json:"itemId"`
	// Window 职责：说明历史统计口径；使用：与游戏筛选对应；边界：不影响库存。
	Window Window `json:"window"`
	// Planets 职责：列出有历史或货槽的星球；使用：详情展开；边界：不生成未出现的星球记录。
	Planets []PlanetItem `json:"planets"`
}

// Analysis 职责：返回选档解析结果与附档线索；使用：分析入口；边界：附档存在不证明存档来源。
type Analysis struct {
	// SaveID 职责：沿用环境列表身份；使用：后续快照查询；边界：不是内容哈希。
	SaveID string `json:"saveId"`
	// HasServerCompanion 职责：提示同名联机附档存在；使用：来源提示；边界：不读取或解释附档内容。
	HasServerCompanion bool `json:"hasServerCompanion"`
	// Summary 职责：返回初次汇总；使用：选档后直接显示物品；边界：解析失败不返回部分结果。
	Summary Summary `json:"summary"`
}
