// Package industryworkspace 提供跨星球桌面工作区的结果和蓝图包契约。
package industryworkspace

import (
	"time"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// Options 是实际写进蓝图的设施选项，各项默认关闭，传送带和分拣器等级由玩家选择。
type Options struct {
	TechnologyMode string `json:"technologyMode,omitempty"` // save 采用存档有效科技，custom 使用模拟输入。
	// Continuous 让计时窗到期后接着算；未传此项的旧请求保留分段续算。
	Continuous    bool   `json:"continuous,omitempty"`
	Power         bool   `json:"power"`
	Spray         bool   `json:"spray"`
	Fuel          bool   `json:"fuel"`
	Buffer        bool   `json:"buffer"`
	Foundation    string `json:"foundation"`
	BeltLevel     int    `json:"beltLevel"`
	InserterLevel int    `json:"inserterLevel"`
}

// Request 把规划参数与玩家已打开的存档快照绑定，界面不能提交自己生成的物料解。
type Request struct {
	SaveID     string               `json:"saveId"`
	SnapshotID string               `json:"snapshotId"`
	Planning   industryplan.Request `json:"planning"`
	Options    Options              `json:"options"`
}

// Site 保存选星用的轻量资源摘要，具体矿点只在选中候选时展开。
type Site struct {
	ID     int64  `json:"id"`
	StarID int64  `json:"starId"`
	Name   string `json:"name"`
	Star   string `json:"star"`
	// Position 是存档时刻的游戏米坐标；使用：三维星图；边界：天体显示尺寸不代表距离。
	Position galaxy.Vector `json:"position"`
	// ParentPlanetID 标识卫星的母星；使用：恒星内浏览；边界：零表示直接绕恒星运行。
	ParentPlanetID int64                         `json:"parentPlanetId"`
	Gas            bool                          `json:"gas"`
	Source         string                        `json:"source"`
	Resources      []saveanalysis.PlanetResource `json:"resources"`
}

// Selection 标识一次只读解析。即使文件名相同，新内容也须重新选定。
type Selection struct {
	SaveName    string                           `json:"saveName"`
	Technology  *saveanalysis.PlanningTechnology `json:"technology,omitempty"`
	SaveID      string                           `json:"saveId"`
	SnapshotID  string                           `json:"snapshotId"`
	Tick        int64                            `json:"tick"`
	GameVersion string                           `json:"gameVersion"`
	Stars       int                              `json:"stars"`
	// StarSystems 提供恒星名称与真实位置；使用：星图；边界：不传递戴森球供能假设。
	StarSystems []saveanalysis.StarLink `json:"starSystems"`
	Sites       []Site                  `json:"sites"`
}

// Task 展示最近任务的状态与诊断；不会在轮询时跨桥传输蓝图。
type Task struct {
	ID             int64                 `json:"id"`
	State          string                `json:"state"`
	Progress       industryplan.Progress `json:"progress"`
	Error          string                `json:"error,omitempty"`
	ElapsedSeconds int64                 `json:"elapsedSeconds"`
	Estimate       *TimeEstimate         `json:"estimate,omitempty"`
	Expanding      bool                  `json:"expanding"`

	activeSince                   time.Time
	activeTime, roundStart        time.Duration
	workDone, workTotal, furthest int
	simpleLayout                  bool
}

// PlanetSheet 是单星施工清单。经纬与朝向显示为度数，蓝图文件另行存放。
type PlanetSheet struct {
	OutboundVesselMW float64                    `json:"outboundVesselMw,omitempty"` // 本星发货的持续耗电，已计入供电核验。
	Fleet             []industryplan.FleetStation `json:"fleet,omitempty"`
	Sheets            []BlueprintSheet            `json:"sheets"`
	Modules           int                         `json:"modules"`
	Photons           *industryplan.PhotonPlan    `json:"photons,omitempty"`
	ID                int64                       `json:"id"`
	Name              string                      `json:"name"`
	Star              string                      `json:"star"`
	File              string                      `json:"file"`
	Longitude         float64                     `json:"longitude"`
	Latitude          float64                     `json:"latitude"`
	Direction         float64                     `json:"direction"`
	Buildings         int                         `json:"buildings"`
	Areas             int                         `json:"areas"`
	FoundationCells   int                         `json:"foundationCells"`
	SoilEstimateKnown bool                        `json:"soilEstimateKnown"`
	SoilLowerBound    int64                       `json:"soilLowerBound"`
	SoilUpperBound    int64                       `json:"soilUpperBound"`
	Stations          int                         `json:"stations"`
	Vessels           int                         `json:"vessels"`
	PowerMW           float64                     `json:"powerMw"`
	Ports             []industryplan.Port         `json:"ports"`
	Mines             []industryplan.Mine         `json:"mines"`
	MinePlacements    []MinePlacement             `json:"minePlacements"`
	Materials         []ItemCount                 `json:"materials"`
	StationStorage    []ItemCount                 `json:"stationStorage"`
	Power             *industryplan.PowerPlan     `json:"power,omitempty"`
	ExistingSlots     []ExistingSlot              `json:"existingSlots,omitempty"`
	Warnings          []string                    `json:"warnings"`
}

// BlueprintSheet 是可以单独在游戏中粘贴的一张图，ID 用于预览，顺序按施工先后排列。
type BlueprintSheet struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	File            string `json:"file"`
	Kind            string `json:"kind"`
	Buildings       int    `json:"buildings"`
	FoundationCells int    `json:"foundationCells"`
}

// ExistingSlot 标出可能与新建物流站自行配对的已有本地货槽，便于玩家在原版中定位。
type ExistingSlot struct {
	ItemID    int   `json:"itemId"`
	PlanetID  int64 `json:"planetId"`
	StationID int64 `json:"stationId"`
	Slot      int   `json:"slot"`
	Logic     int64 `json:"logic"`
	Remote    bool  `json:"remote"`
}

// MinePlacement 保存最终矿机与所覆盖矿点之间的关系，位置以度数显示。
type MinePlacement struct {
	ItemID        int     `json:"itemId"`
	FacilityID    int     `json:"facilityId"`
	BuildingIndex int     `json:"buildingIndex"`
	Longitude     float64 `json:"longitude"`
	Latitude      float64 `json:"latitude"`
	VeinIDs       []int64 `json:"veinIds"`
	CoveredItems  *int64  `json:"coveredItems,omitempty"`
	RatePerMinute float64 `json:"ratePerMinute"`
}

// ItemCount 是建筑材料或单物品物流库存容量的计数。
type ItemCount struct {
	ItemID int `json:"itemId"`
	Count  int `json:"count"`
}

// Report 记录可导出方案和交付成立的外供条件，省略大体积建筑数组。
type Report struct {
	SaveName           string                           `json:"saveName"`
	Technology         *saveanalysis.PlanningTechnology `json:"technology,omitempty"`
	TechnologyChanges  []string                         `json:"technologyChanges"`
	ExportName         string                           `json:"exportName"`
	BlueprintCount     int                              `json:"blueprintCount"`
	LayoutMetrics      industryplan.LayoutMetrics       `json:"layoutMetrics"`
	Issues             []Issue                          `json:"issues"`
	AchievedTargets    []quant.Target                   `json:"achievedTargets"`
	ExtraTargets       []quant.Target                   `json:"extraTargets"`
	ExpansionNote      string                           `json:"expansionNote"`
	ResourcesInspected int                              `json:"resourcesInspected"`
	LimitStage         string                           `json:"limitStage,omitempty"`   // 本轮停止的搜索阶段，供页面提供相应操作。
	ResourceGaps       []industryplan.ResourceGap       `json:"resourceGaps,omitempty"` // 已确认的采集速率与目标原料需求之差。
	LayoutCandidates   int                              `json:"layoutCandidates"`
	RemoteGroup        int                              `json:"remoteGroup,omitempty"`
	ScoreBasis         string                           `json:"scoreBasis"`
	StoragePlanetID    int64                            `json:"storagePlanetId"`
	Exportable         bool                             `json:"exportable"`
	SnapshotID         string                           `json:"snapshotId"`
	Tick               int64                            `json:"tick"`
	GameVersion        string                           `json:"gameVersion"`
	Status             string                           `json:"status"`
	CenterPlanetID     int64                            `json:"centerPlanetId"`
	Targets            []quant.Target                   `json:"targets"`
	Profile            string                           `json:"profile"`
	Preference         string                           `json:"preference"`
	Candidates         int                              `json:"candidates"`
	Repairs            int                              `json:"repairs"`
	Planets            []PlanetSheet                    `json:"planets"`
	Routes             []industryplan.Route             `json:"routes"`
	External           []industryplan.External          `json:"external"`
	Warnings           []string                         `json:"warnings"`
}

// Bundle 由一次获选的全局方案构成，发布后作为不可变快照供预览、导出使用。
type Bundle struct {
	Report  Report
	Request Request
	Plan    industryplan.Result
	Codes   map[int64]string
}

// Chunk 是单颗星球预览的连续文本片段；界面用 NextOffset 拉取后一段。
type Chunk struct {
	Text       string `json:"text"`
	NextOffset int    `json:"nextOffset"`
	Done       bool   `json:"done"`
}
