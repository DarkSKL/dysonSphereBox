package planet

// 文件职责：定义整星蓝图的空间契约；使用：产线与矿区装配；边界：不读取或修改存档。

import (
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
)

// Position 职责：标识绝对球面建筑端点；使用：以弧度提供经纬度，以原版建筑层提供高度；边界：半径 200 的固态星球。
type Position struct {
	// Longitude/Latitude 职责：记录球面经纬；使用：游戏坐标约定；边界：单位弧度。
	Longitude, Latitude float64
	// Height 职责：记录高度；使用：编解码建筑端点；边界：单位为层，地表为零。
	Height float64
}

// Cell 职责：标记需保留的地表格；使用：矿点、采集水面及现有占用；边界：采用原版纬带网格。
type Cell struct {
	// Longitude/Latitude 职责：记录格中心；使用：地基屏蔽；边界：单位弧度。
	Longitude, Latitude float64
	// buried 标记地下矿点，铺地时需同时保护决定其显隐的相邻地形顶点。
	buried bool
}

// Obstacle 职责：描述存档中已占用的空间；使用：避让建筑和预建造；边界：半径采用游戏世界距离。
type Obstacle struct {
	// Position 职责：标识占用中心；使用：球面距离；边界：与目标处于同星球。
	Position Position
	// Radius 职责：保守包围存量；使用：碰撞预检；边界：正世界长度。
	Radius float64
	// surfaceOnly 职责：使虚拟基地占地作用于整个地表投影；使用：来自原版地表的已知保护区；边界：不把已有实体的三维占位展成地表柱。
	surfaceOnly bool
	// stationItem 职责：保留物流塔类型；使用：独立复核塔间距；边界：不把间距限制施加到普通设备。
	stationItem int16
}

// Object 职责：关联原版建筑记录和绝对端点；使用：整星装配；边界：连接使用全局建筑 Index，-1 表示无连接。
type Object struct {
	// Building 职责：保存原型和连接；使用：装配重排；边界：Yaw 采用未旋转游戏朝向。
	Building blueprint.Building
	// First/Second 职责：指定双端点；使用：分拣器及有独立第二端点的设备；边界：普通设备两端相同。
	First, Second Position
	// Item 职责：标记计划物品；使用：局部修复；边界：零表示共享设施。
	Item int
	// Module 职责：标记来源模块；使用：局部修复；边界：可空。
	Module string
}

// Request 职责：承载单星蓝图装配输入；使用：Build；边界：建筑须已经匹配真实落点，地基依赖可信地形。
type Request struct {
	// manufacturingOccupancy 只供同一次连续追加制造块时计算占地前缀。
	manufacturingOccupancy *manufacturingOccupancy
	// placementIndices 只供同一制造块在固定前序占地下逐格复核，整星编码自行检查全部占地。
	placementIndices *placementIndices
	// localPorts 与 mineCoverage 保存实装制造、电厂及采集塔的带路速率，用于核验本地配送。requiredGridMW 仅在需要外接电网时填写。
	localPorts                   []build.BoundaryPort
	mineCoverage                 []MiningCoverage
	dronePowerMW, requiredGridMW float64
	// droneChargingMW 按玩家实际装机的塔保存本站配送的持续耗电上界；全星供电仍使用 dronePowerMW。
	droneChargingMW map[int]float64
	// exportFeeds 只记录由真实出矿带或制造边界送入指定塔、可用于出口的件数/分钟。
	exportFeeds map[fleetFeed]float64
	// PlanetID 职责：标识星球；使用：错误与汇总；边界：来自同一存档快照。
	PlanetID int64
	// Name/GameVersion 职责：保存信封信息；使用：编码；边界：不得以游戏版本宣称实贴验证。
	Name, GameVersion string
	// Gas 职责：标记气态巨星；使用：拒绝原版不支持的实贴；边界：不启用 Mod 格式。
	Gas bool
	// Foundation 职责：指定铺设范围；使用：空、none、construction 或 planet；边界：空等同 none。
	Foundation string
	// Objects 职责：提供所有建筑；使用：同一星球的完整装配；边界：失败不交付部分建筑。
	Objects []Object
	// Obstacles 职责：提供存量和预建占用；使用：避让；边界：不会自动拆除。
	Obstacles []Obstacle
	// Protected 职责：提供地基需保留格；使用：屏蔽采集水面和矿点；边界：仅格中心。
	Protected []Cell
	// TerrainKnown 职责：确认保护格数据完整；使用：启用铺设；边界：缺失时拒绝推测。
	TerrainKnown bool
	// wet 职责：采样游戏真实液面；使用：占地检查和地基保护；边界：由生产快照适配，外部格列表仍单独适用。
	wet func(Position) (bool, bool)
	// stationPad 核对星际塔单个支脚的地表碰撞高度，由带液面的真实存档提供。
	stationPad func(Position) bool
	// pumpSite 职责：检查泵脚与取水点命中的地表；使用：岸线选址和整星终检；边界：从本次快照的固态星球网格取高。
	pumpSite func(Position, float64) bool
	// geothermal 职责：复核熔岩类型和三点深度；使用：整星终检；边界：无回调时不得认证地热落位。
	geothermal func(Position) bool
	// externalGrid 职责：允许新建电网以节点为接入点；使用：已有电网供电合同；边界：玩家仍需接到声明余量的已有电网。
	externalGrid bool
	// minerals 职责：保护未开采和已存在矿点；使用：非开采设施落位；边界：不替代地基保护格。
	minerals []Obstacle
	// isReformed 职责：识别已有地基；使用：统计只需新铺的格；边界：原版数组索引。
	isReformed func(longitude, latitude int) bool
	// soilUpper 职责：保存原版沙土可证明上界；使用：建设清单；边界：nil 表示无原始顶点高度依据。
	soilUpper *int64
}

// Result 职责：承载一份可严格解码的原版蓝图；使用：发布至相应星球；边界：游戏实贴独立验收。
type Result struct {
	// Blueprint 职责：保存纯文本蓝图；使用：一星一份导出；边界：不拆分、不写存档。
	Blueprint string
	// Anchor 职责：保存粘贴原点；使用：标明游戏经纬度；边界：经纬弧度。
	Anchor Position
	// Direction 职责：保存固定朝向；使用：粘贴时的旋转度数；边界：默认游戏方向为零。
	Direction float64
	// BuildingCount/AreaCount/FoundationCells 职责：记录结果规模；使用：发布摘要；边界：不包含存量。
	BuildingCount, AreaCount, FoundationCells int
	// ItemCounts 职责：公开实际蓝图建造物品数；使用：施工清单；边界：从最终记录统计。
	ItemCounts map[int16]int
	// StationCount 职责：公开实际已放置星际塔；使用：船队容量核对；边界：不包含蓝图外塔。
	StationCount int
	// StationStorage 职责：汇总最终塔物品槽容量；使用：施工清单；边界：是原版基础槽额定，空余库存独立核对。
	StationStorage map[int]int
	// LocalDroneItems 职责：列出不同实体间已有本地供需配对的物品；使用：实际机队准备条件；边界：不推断所需物流运输机数量。
	LocalDroneItems []int
	// LocalModes 职责：汇总实际实体上的每物品本地供 1、需 2 或兼有 3；使用：定位已有塔可能产生的配对；边界：不包含只配远程的槽。
	LocalModes map[int]uint8
	// SoilEstimateKnown 职责：标记沙土区间存在实际地形依据；使用：施工清单；边界：区间不是精确净耗。
	SoilEstimateKnown bool
	// SoilLowerBound/SoilUpperBound 职责：保存原版沙土消耗区间；使用：施工清单；边界：不计建造以外的清除或已有地基。
	SoilLowerBound, SoilUpperBound int64
}

// Error 职责：标明失败所在星球、物品和模块；使用：规划局部修复；边界：该星球没有可交付蓝图。
type Error struct {
	// mineIndex 保留本次缺矿在施工清单中的顺序，供两种塔距比较已完成的采集合同。
	mineIndex int
	// AvailableRatePerMinute 职责：回传本次实际可接驳的矿区速率；使用：缩减局部合同；边界：下一轮仍须完整生成和复核，零不表示可交付。
	AvailableRatePerMinute float64
	// PlanetID 职责：保存来源星球；使用：定位修复；边界：单星。
	PlanetID int64
	// Item 职责：保存来源物品；使用：重排方案；边界：零表示共享或无法归属。
	Item int
	// Module 职责：保存来源模块；使用：定位冲突；边界：可空。
	Module string
	// Code/Message 职责：保存分类与具体证据；使用：上游诊断；边界：不可用作能力证明。
	Code, Message string
}

// Error 职责：格式化诊断；使用：日志与边界错误；边界：保留结构化字段供调用方判断。
func (e *Error) Error() string {
	return fmt.Sprintf("%s 星球 %d 物品 %d 模块 %q：%s", e.Code, e.PlanetID, e.Item, e.Module, e.Message)
}
