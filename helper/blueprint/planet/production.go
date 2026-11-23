package planet

// 文件职责：将制造、采集及货运计划装配至存档星球；使用：全局规划的逐星终检；边界：只有证据支持的实际速率才可返回。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// ProductionOptions 职责：携带生成策略；使用：Generate；边界：仅影响当前星球蓝图。
type ProductionOptions struct {
	firstVerifiedLayout bool // 多星初筛先采用已通过规则的模板，完整纬区后备继续比较布局。
	nearMineTowers      bool // 岸边普通矿区按原带长缺矿时，重试离矿机较近的塔位。
	// DroneCargo/DroneSpeed 使用当前存档的单机载量与速度，供本地矿料配送的飞行用电核验。
	DroneCargo int
	DroneSpeed float64
	// Name/GameVersion/Foundation 职责：指定蓝图信封与铺设；使用：Build；边界：选项不会写入游戏。
	Name, GameVersion, Foundation string
	// Setting 职责：重用工厂生成器选项；使用：生产地块；边界：按原布局规则解释。
	Setting build.Setting
	// Fuel 职责：请求实际发电燃料配套；使用：全局规划与逐星建设；边界：翘曲器补给另由 Setting 控制。
	Fuel bool
	// OrbitalCollectors 职责：标记轨道采集条件；使用：边界诊断；边界：原版不能在气态巨星粘贴。
	OrbitalCollectors bool
	// Profile 职责：选择低算力或标准搜索；使用：工厂候选并发与落点探测；边界：全局候选预算由父任务持有。
	Profile string
	// RemoteGroup 职责：携带已核查的原版星际专用分组掩码；使用：实际远程供需塔；边界：零表示由玩家配置配对。
	RemoteGroup uint64
}

// WithCandidateBudget 职责：创建跨星共用的真实求解预算；使用：总任务传回上下文给每次 Generate；边界：进度可读 Used，已有预算不会重置。
func WithCandidateBudget(ctx context.Context, limit int) (context.Context, *buildcore.CandidateBudget) {
	return buildcore.WithCandidateBudget(ctx, limit)
}

// ProductionRequest 职责：绑定逐星计划和存档空间快照；使用：Generate；边界：计划和资源须对应同一星球。
type ProductionRequest struct {
	// Plan 职责：保存制造和边界流；使用：产线及矿区装配；边界：量化方案只读。
	Plan industryplan.PlanetPlan
	// Resources 职责：保存真实矿位及存量；使用：落位和避让；边界：不管存档文件读写。
	Resources saveanalysis.PlanetResourceDetail
	// Options 职责：保存当次蓝图配置；使用：局部与整体生成；边界：不更改设置持久化。
	Options ProductionOptions
}

// ProductionResult 职责：交付整星蓝图和已验证边界能力；使用：全局规划复核；边界：缺少任何计划条件时顶层返回错误。
type ProductionResult struct {
	// RequiredGridMW 是实际设备没有新建发电覆盖的持续用电，单独列入本星外接电网。
	RequiredGridMW float64
	// Result 职责：附带整星原版蓝图和锚点；使用：最终导出；边界：不拆成模块发布。
	Result
	// Actual 职责：返回可追溯的边界能力；使用：逐星流量守恒；边界：不复制未连接的计划端口。
	Actual industryplan.PlanetActual
	// Mining 职责：记录真实已放置矿机的覆盖矿点和额定流量；使用：施工清单；边界：包含接驳带实际瓶颈。
	Mining []MiningCoverage
}

// Generate 职责：搜索存档上的落位并装配完整蓝图；使用：逐星规划回调；边界：资料、几何或物流缺口返回带物品的诊断。
func Generate(ctx context.Context, input ProductionRequest) (ProductionResult, error) {
	return generate(ctx, input, false)
}

// CheckSourceSites 复用施工入口检查采集、取水和光子接收器落位，供多星分工先发现资源缺口。仅完整 Generate 能出图。
func CheckSourceSites(ctx context.Context, input ProductionRequest) error {
	_, err := generate(ctx, input, true)
	return err
}

// generate 按合同顺序装配同一批实体。分工预检在采集段完成时退出，制造、物流和供电由完整施工核验。
func generate(ctx context.Context, input ProductionRequest, sourcesOnly bool) (ProductionResult, error) {
	stage := industryplan.NewLayoutStageTimer(ctx)
	stage("地形与已有建筑")
	defer stage("")
	plan, detail, options := input.Plan, input.Resources, input.Options
	fail := func(item int, code, message string) (ProductionResult, error) {
		return ProductionResult{}, &Error{PlanetID: plan.PlanetID, Item: item, Code: code, Message: message}
	}
	if plan.PlanetID <= 0 || plan.PlanetID != detail.Planet.PlanetID {
		return fail(0, "PLANET_SNAPSHOT", "规划与资源详情不属于同一已验证星球")
	}
	if detail.Planet.Gas {
		return fail(0, "PLANET_GAS_NATIVE_UNSUPPORTED", "原版 25955035 在气态巨星禁用蓝图粘贴")
	}
	for _, resource := range detail.Planet.Resources {
		if resource.Kind == "gas" {
			return fail(0, "PLANET_GAS_NATIVE_UNSUPPORTED", "原版 25955035 在气态巨星禁用蓝图粘贴")
		}
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return ProductionResult{}, err
	}
	request := Request{PlanetID: plan.PlanetID, Name: options.Name, GameVersion: options.GameVersion, Foundation: options.Foundation}
	request.geothermal = func(p Position) bool { return geothermalSite(input, p) }
	for _, zone := range detail.RelayZones {
		position, err := savePosition(zone.Position)
		if err != nil || zone.PlanetID != plan.PlanetID || !(zone.Radius > 0 && zone.Radius <= 100) {
			return fail(0, "PLANET_RELAY_ZONE", "存档黑雾基地的中心或活动范围无效")
		}
		position.Height = 0
		request.Obstacles = append(request.Obstacles, Obstacle{Position: position, Radius: zone.Radius, surfaceOnly: true})
	}
	if terrain := detail.Terrain; terrain.WaterItemID != nil && terrain.WaterHeight != nil {
		dry := *terrain.WaterItemID == 0
		_, sampled := terrain.HeightAt(galaxy.Vector{X: 1})
		request.TerrainKnown = dry || sampled
		if !dry {
			request.stationPad = func(p Position) bool {
				h, known := surfaceRadiusAt(terrain, p)
				// 原版 50 号塔的 landOffset 为零，支脚至少需达到基球减 0.30 米。
				return known && h >= 199.70 && h >= 200+*terrain.WaterHeight-.27
			}
			request.pumpSite = func(p Position, yaw float64) bool {
				return pumpSurfaceFits(p, yaw, terrain)
			}
			request.wet = func(p Position) (bool, bool) {
				point, err := world(p)
				if err != nil {
					return false, false
				}
				height, ok := terrain.HeightAt(galaxy.Vector{X: float64(point.X), Y: float64(point.Y), Z: float64(point.Z)})
				delta := height - *terrain.WaterHeight
				// 原版地形黄金样本曾记录算法 3 最多 3 厘米差异，取 5 厘米保守留量。
				if !ok || math.Abs(delta) <= .05 {
					return false, false
				}
				return delta < 0, true
			}
		}
	}
	if !request.TerrainKnown {
		return fail(0, "PLANET_TERRAIN_UNKNOWN", "存档未提供足以核对建设落点的地表和液面")
	}
	if request.Foundation != "" && request.Foundation != "none" && detail.VeinSource == "unknown" {
		return fail(0, "PLANET_VEINS_UNKNOWN", "矿点位置未知，无法保证铺设地基时保留矿点")
	}
	if request.Foundation != "" && request.Foundation != "none" {
		if err := configureTerrainReform(&request, detail.Terrain); err != nil {
			return fail(0, "PLANET_FOUNDATION_SNAPSHOT", err.Error())
		}
	}
	occupancyRadii := map[int16]float64{}
	for _, hole := range detail.GeothermalHoles {
		p, err := savePosition(hole)
		if err != nil {
			return fail(0, "PLANET_SNAPSHOT", err.Error())
		}
		// 模型 406 的建造盒水平半宽 6、半长 5.75，中心向后偏 0.8 米。
		p.Height = 0
		request.Obstacles = append(request.Obstacles, Obstacle{Position: p, Radius: math.Hypot(6, 5.75+.8)})
	}
	for _, entry := range detail.Buildings {
		point, err := savePosition(entry.Position)
		if err != nil {
			return fail(int(entry.ItemID), "PLANET_SNAPSHOT", err.Error())
		}
		item := int16(entry.ItemID)
		radius := occupancyRadii[item]
		if radius == 0 {
			radius = occupancyRadius(catalog, item)
			occupancyRadii[item] = radius
		}
		request.Obstacles = append(request.Obstacles, Obstacle{Position: point, Radius: radius, stationItem: item})
		if end, err := savePosition(entry.EndPosition); err == nil && end != point {
			request.Obstacles = append(request.Obstacles, Obstacle{Position: end, Radius: radius})
		}
	}
	for _, vein := range detail.Veins {
		p, err := savePosition(vein.Position)
		if err == nil {
			request.Protected = append(request.Protected, Cell{Longitude: p.Longitude, Latitude: p.Latitude, buried: vein.IsDeepBuried()})
			_, radius := veinCollider(vein.VeinType)
			if !vein.IsSurfaceMiningBlocked() {
				// 设备占地在粘贴地表量取。矿石原点的地形高差不能为地面电杆或传送带腾出净空。
				p.Height = 0
			}
			request.minerals = append(request.minerals, Obstacle{Position: p, Radius: radius})
		}
	}
	stage("矿区与液体采集")
	actual, mining, err := placeMining(ctx, &request, input, catalog)
	if err != nil {
		return ProductionResult{}, err
	}
	request.mineCoverage = mining
	stage("光子接收")
	if err := appendPhotons(ctx, &request, input, &actual, catalog); err != nil {
		return ProductionResult{}, err
	}
	if sourcesOnly {
		return ProductionResult{}, ctx.Err()
	}
	stage("制造模板与传送带落位")
	if err := appendManufacturing(ctx, &request, input, &actual, catalog); err != nil {
		return ProductionResult{}, err
	}
	stage("收发站与配船")
	if err := appendReceiving(ctx, &request, input, &actual, catalog); err != nil {
		return ProductionResult{}, err
	}
	stage("供电、燃料与电网")
	if err := appendEnergy(ctx, &request, input, &actual, catalog); err != nil {
		return ProductionResult{}, err
	}
	stage("实物复核与蓝图编码")
	if err := configureFleetCharging(&request, plan, &actual); err != nil {
		return ProductionResult{}, err
	}
	for _, desired := range plan.Ports {
		found := false
		for _, port := range actual.Ports {
			if port.Kind == desired.Kind && port.ItemID == desired.ItemID && port.PeerPlanetID == desired.PeerPlanetID && port.RatePerMinute+1e-6 >= desired.RatePerMinute && port.StorageItems >= desired.StorageItems {
				found = true
				break
			}
		}
		if !found {
			return fail(desired.ItemID, "PLANET_PORT", fmt.Sprintf("本星 %s 端口缺少可验证带路或库存，目标 %.3f 件/分钟、%d 件", desired.Kind, desired.RatePerMinute, desired.StorageItems))
		}
	}
	for i := range request.Objects {
		building := &request.Objects[i].Building
		configureRemoteGroup(building, options.RemoteGroup)
		DisableVesselAutoReplenishment(building)
		if building.ItemID == 2104 && len(building.Parameters) > 324 && plan.WarpDistance > 0 {
			building.Parameters[324] = int32(math.Round(plan.WarpDistance))
		}
		if building.ItemID == 2104 && len(building.Parameters) == 2048 && len(plan.OutboundFleet) > 0 {
			for slot := 0; slot < 5; slot++ {
				if building.Parameters[slot*6] > 0 && building.Parameters[slot*6+2] == 1 {
					// 飞船往返持续耗电按满舱折算，仅远供发船塔采用此起运设置。
					building.Parameters[327] = 100
					break
				}
			}
		}
	}
	output, err := Build(ctx, request)
	if err != nil {
		return ProductionResult{}, err
	}
	return ProductionResult{Result: output, Actual: actual, Mining: mining, RequiredGridMW: request.requiredGridMW}, nil
}

// productionMaterialPlan 职责：将边界间直通流从制造黑盒切出；使用：独立物流、采集和供电设施另行承担这些流量；边界：保留生产配方消耗、制造和联产的全部物料，不修改整星计划。
func productionMaterialPlan(ctx context.Context, site industryplan.PlanetPlan) (quant.Result, error) {
	plan := site.Plan
	graph := plan.FlowGraph
	if graph == nil {
		rebuilt, err := quant.BuildFlowGraphContext(ctx, plan)
		if err != nil {
			return quant.Result{}, err
		}
		graph = &rebuilt
	}
	kinds := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		kinds[node.ID] = node.Kind
	}
	inputs, targets, leftovers := map[int]float64{}, map[int]float64{}, map[int]float64{}
	for _, edge := range graph.Edges {
		source := kinds[edge.SourceNodeID]
		if source != "raw-source" && source != "external-source" {
			continue
		}
		switch kinds[edge.TargetNodeID] {
		case "target-sink":
			targets[edge.ItemID] += edge.RatePerMinute
		case "leftover-sink":
			leftovers[edge.ItemID] += edge.RatePerMinute
		default:
			continue
		}
		inputs[edge.ItemID] += edge.RatePerMinute
	}
	if len(inputs) == 0 {
		return plan, nil
	}
	// 按已配对直通边同步扣减两端，重复物品条目逐项消费；制造支路和原始切片均保留。
	trimRates := func(rates []quant.ItemRate, remaining map[int]float64) []quant.ItemRate {
		var kept []quant.ItemRate
		for _, rate := range rates {
			delta := min(rate.RatePerMinute, remaining[rate.ItemID])
			rate.RatePerMinute -= delta
			remaining[rate.ItemID] -= delta
			if rate.RatePerMinute > 0 {
				kept = append(kept, rate)
			}
		}
		return kept
	}
	plan.RawInputs = trimRates(site.Plan.RawInputs, inputs)
	plan.Leftovers = trimRates(site.Plan.Leftovers, leftovers)
	remainingInputs := map[int]bool{}
	for _, raw := range plan.RawInputs {
		remainingInputs[raw.ItemID] = true
	}
	plan.ExternalInputItemIDs = nil
	for _, item := range site.Plan.ExternalInputItemIDs {
		if remainingInputs[item] {
			plan.ExternalInputItemIDs = append(plan.ExternalInputItemIDs, item)
		}
	}
	plan.Targets = nil
	for _, target := range site.Plan.Targets {
		delta := min(target.RatePerMinute, targets[target.ItemID])
		target.RatePerMinute -= delta
		targets[target.ItemID] -= delta
		if target.RatePerMinute > 0 {
			plan.Targets = append(plan.Targets, target)
		}
	}
	projected, err := quant.BuildFlowGraphContext(ctx, plan)
	if err != nil {
		return quant.Result{}, err
	}
	plan.FlowGraph = &projected
	plan.PlanID += "-factory-" + projected.Digest
	return plan, nil
}

// savePosition 职责：从存档本地向量换算球面位置；使用：建筑和矿位；边界：原版半径。
func savePosition(v galaxy.Vector) (Position, error) {
	point, err := geometry.WorldToSurface(geometry.WorldPoint{Frame: geometry.PlanetWorldFrame(), X: geometry.WorldLength(v.X), Y: geometry.WorldLength(v.Y), Z: geometry.WorldLength(v.Z)})
	return Position{Longitude: float64(point.Longitude), Latitude: float64(point.Latitude), Height: float64(point.Height)}, err
}

// occupancyRadius 职责：提供已知存量安全包围；使用：资源详情缺少模型号时；边界：从同一物品的全部已知模型取最大值。
func occupancyRadius(catalog *placement.Catalog, item int16) float64 {
	radius := 1.5
	for _, model := range catalog.ModelsForItem(item) {
		radius = max(radius, recordRadius(catalog, blueprint.Building{ItemID: item, ModelIndex: model}))
	}
	if item == 2103 || item == 2104 {
		radius = max(radius, 25)
	}
	return radius
}

// placementConflict 职责：快速筛查模块与现有范围；使用：候选落位；边界：最终装配仍复核实际规则。
func placementConflict(objects []Object, index obstacleIndex, catalog *placement.Catalog) bool {
	for _, obj := range objects {
		radius := groundRadius(catalog, obj.Building)
		for _, p := range []Position{obj.First, obj.Second} {
			point, err := world(p)
			if err != nil || index.conflicts(point, radius) || index.stationConflict(point, obj.Building.ItemID) {
				return true
			}
			if p == obj.Second {
				break
			}
		}
	}
	return index.surfaceRouteConflict(objects, catalog) >= 0
}
