package saveanalysis

// 文件职责：按受支持存档布局顺序消费区段，仅保留查询字段。
// 使用：只读解码器内部调用；边界：版本不符立即失败，不推测区段位置。

// layoutGameStatData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameStatData struct {
	statistics *layoutProductionStatistics
}

// readGameStatData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameStatData(props ...int64) *layoutGameStatData {
	r.section = "GameStatData"
	v := &layoutGameStatData{}
	r.expect(r.number(4, true), 3, "GameStatData.version")
	var numTechHashedHistory int64
	numTechHashedHistory = r.number(4, true)
	ntechHashedHistory := r.length(numTechHashedHistory)
	r.skip(ntechHashedHistory * 4)
	var statistics *layoutProductionStatistics
	statistics = r.readProductionStatistics()
	v.statistics = statistics
	r.production = v.statistics
	r.readKillStatistics()
	r.readTrafficStatistics()
	r.readCustomCharts()
	return v
}

// layoutProductionStatistics 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutProductionStatistics struct {
	factoryStatPool []*layoutFactoryProductionStat
}

// readProductionStatistics 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readProductionStatistics(props ...int64) *layoutProductionStatistics {
	r.section = "ProductionStatistics"
	v := &layoutProductionStatistics{}
	r.expect(r.number(4, true), 0, "ProductionStatistics.version")
	var factoryCount int64
	factoryCount = r.number(4, true)
	var factoryStatPool []*layoutFactoryProductionStat
	nfactoryStatPool := r.length(factoryCount)
	factoryStatPool = make([]*layoutFactoryProductionStat, nfactoryStatPool)
	for i := int64(0); i < nfactoryStatPool; i++ {
		factoryStatPool[i] = r.readFactoryProductionStat()
	}
	v.factoryStatPool = factoryStatPool
	var numFirstCreateIds int64
	numFirstCreateIds = r.number(4, true)
	nfirstCreateIds := r.length(numFirstCreateIds)
	r.skip(nfirstCreateIds * 4)
	var numFavoriteIds int64
	numFavoriteIds = r.number(4, true)
	nfavoriteIds := r.length(numFavoriteIds)
	r.skip(nfavoriteIds * 4)
	return v
}

// layoutFactoryProductionStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutFactoryProductionStat struct {
	productPool []*layoutProductStat
}

// readFactoryProductionStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readFactoryProductionStat(props ...int64) *layoutFactoryProductionStat {
	r.section = "FactoryProductionStat"
	v := &layoutFactoryProductionStat{}
	r.expect(r.number(4, true), 1, "FactoryProductionStat.version")
	r.skip(4)
	var productCursor int64
	productCursor = r.number(4, true)
	var productPool []*layoutProductStat
	nproductPool := r.length(productCursor - 1)
	productPool = make([]*layoutProductStat, nproductPool)
	for i := int64(0); i < nproductPool; i++ {
		productPool[i] = r.readProductStat()
	}
	v.productPool = productPool
	var numPowerPool int64
	numPowerPool = r.number(4, true)
	npowerPool := r.length(numPowerPool)
	for i := int64(0); i < npowerPool; i++ {
		r.readPowerStat()
	}
	var numProductIndices int64
	numProductIndices = r.number(4, true)
	nproductIndices := r.length(numProductIndices)
	r.skip(nproductIndices * 4)
	r.skip(8)
	return v
}

// layoutProductStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutProductStat struct {
	numCount  int64
	numCursor int64
	total     []int64
	itemId    int64
}

// readProductStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readProductStat(props ...int64) *layoutProductStat {
	r.section = "ProductStat"
	v := &layoutProductStat{}
	r.expect(r.number(4, true), 1, "ProductStat.version")
	var numCount int64
	numCount = r.number(4, true)
	v.numCount = numCount
	ncount := r.length(numCount)
	r.skip(ncount * 4)
	var numCursor int64
	numCursor = r.number(4, true)
	v.numCursor = numCursor
	ncursor := r.length(numCursor)
	r.skip(ncursor * 4)
	var numTotal int64
	numTotal = r.number(4, true)
	var total []int64
	ntotal := r.length(numTotal)
	total = make([]int64, ntotal)
	for i := int64(0); i < ntotal; i++ {
		total[i] = r.number(8, true)
		// 游戏加载时将历史件数取绝对值；最小负整数无法表示为合法正计数。
		if total[i] < 0 {
			total[i] = -total[i]
			if total[i] < 0 {
				r.fail("历史件数超出有效范围")
			}
		}
	}
	v.total = total
	var itemId int64
	itemId = r.number(4, true)
	v.itemId = itemId
	return v
}

// layoutPowerStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerStat struct {
}

// readPowerStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerStat(props ...int64) *layoutPowerStat {
	r.section = "PowerStat"
	v := &layoutPowerStat{}
	r.expect(r.number(4, true), 0, "PowerStat.version")
	var numEnergy int64
	numEnergy = r.number(4, true)
	nenergy := r.length(numEnergy)
	r.skip(nenergy * 8)
	var numCursor int64
	numCursor = r.number(4, true)
	ncursor := r.length(numCursor)
	r.skip(ncursor * 4)
	var numTotal int64
	numTotal = r.number(4, true)
	ntotal := r.length(numTotal)
	r.skip(ntotal * 8)
	return v
}

// layoutKillStatistics 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKillStatistics struct {
}

// readKillStatistics 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKillStatistics(props ...int64) *layoutKillStatistics {
	r.section = "KillStatistics"
	v := &layoutKillStatistics{}
	r.expect(r.number(4, true), 0, "KillStatistics.version")
	var numStarKillStatPool int64
	numStarKillStatPool = r.number(4, true)
	nstarKillStatPool := r.length(numStarKillStatPool)
	for i := int64(0); i < nstarKillStatPool; i++ {
		r.readExportStruct_uint8__AstroKillStat_()
	}
	var numFactoryKillStatPool int64
	numFactoryKillStatPool = r.number(4, true)
	nfactoryKillStatPool := r.length(numFactoryKillStatPool)
	for i := int64(0); i < nfactoryKillStatPool; i++ {
		r.readExportStruct_uint8__AstroKillStat_()
	}
	r.readExportStruct_uint8__AstroKillStat_()
	var numFavoriteIds int64
	numFavoriteIds = r.number(4, true)
	nfavoriteIds := r.length(numFavoriteIds)
	r.skip(nfavoriteIds * 4)
	return v
}

// layoutExportStruct_uint8__AstroKillStat_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_uint8__AstroKillStat_ struct {
}

// readExportStruct_uint8__AstroKillStat_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_uint8__AstroKillStat_(props ...int64) *layoutExportStruct_uint8__AstroKillStat_ {
	r.section = "ExportStruct<uint8, AstroKillStat>"
	v := &layoutExportStruct_uint8__AstroKillStat_{}
	var index int64
	index = r.number(1, false)
	if index != 0 {
		r.readAstroKillStat()
	}
	return v
}

// layoutAstroKillStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroKillStat struct {
}

// readAstroKillStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroKillStat(props ...int64) *layoutAstroKillStat {
	r.section = "AstroKillStat"
	v := &layoutAstroKillStat{}
	r.expect(r.number(4, true), 0, "AstroKillStat.version")
	nkillStatPool := r.length(2048)
	for i := int64(0); i < nkillStatPool; i++ {
		r.readExportStruct_uint8__KillStat_()
	}
	return v
}

// layoutExportStruct_uint8__KillStat_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_uint8__KillStat_ struct {
}

// readExportStruct_uint8__KillStat_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_uint8__KillStat_(props ...int64) *layoutExportStruct_uint8__KillStat_ {
	r.section = "ExportStruct<uint8, KillStat>"
	v := &layoutExportStruct_uint8__KillStat_{}
	var index int64
	index = r.number(1, false)
	if index != 0 {
		r.readKillStat()
	}
	return v
}

// layoutKillStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKillStat struct {
}

// readKillStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKillStat(props ...int64) *layoutKillStat {
	r.section = "KillStat"
	v := &layoutKillStat{}
	r.expect(r.number(4, true), 0, "KillStat.version")
	var numCount int64
	numCount = r.number(4, true)
	ncount := r.length(numCount)
	r.skip(ncount * 4)
	var numCursor int64
	numCursor = r.number(4, true)
	ncursor := r.length(numCursor)
	r.skip(ncursor * 4)
	var numTotal int64
	numTotal = r.number(4, true)
	ntotal := r.length(numTotal)
	r.skip(ntotal * 4)
	r.skip(4)
	return v
}

// layoutTrafficStatistics 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTrafficStatistics struct {
}

// readTrafficStatistics 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTrafficStatistics(props ...int64) *layoutTrafficStatistics {
	r.section = "TrafficStatistics"
	v := &layoutTrafficStatistics{}
	r.expect(r.number(4, true), 0, "TrafficStatistics.version")
	var numStarTrafficPool int64
	numStarTrafficPool = r.number(4, true)
	nstarTrafficPool := r.length(numStarTrafficPool)
	for i := int64(0); i < nstarTrafficPool; i++ {
		r.readExportStruct_uint8__AstroTrafficStat_()
	}
	var numFactoryTrafficPool int64
	numFactoryTrafficPool = r.number(4, true)
	nfactoryTrafficPool := r.length(numFactoryTrafficPool)
	for i := int64(0); i < nfactoryTrafficPool; i++ {
		r.readExportStruct_uint8__AstroTrafficStat_()
	}
	return v
}

// layoutExportStruct_uint8__AstroTrafficStat_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_uint8__AstroTrafficStat_ struct {
}

// readExportStruct_uint8__AstroTrafficStat_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_uint8__AstroTrafficStat_(props ...int64) *layoutExportStruct_uint8__AstroTrafficStat_ {
	r.section = "ExportStruct<uint8, AstroTrafficStat>"
	v := &layoutExportStruct_uint8__AstroTrafficStat_{}
	var index int64
	index = r.number(1, false)
	if index != 0 {
		r.readAstroTrafficStat()
	}
	return v
}

// layoutAstroTrafficStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroTrafficStat struct {
}

// readAstroTrafficStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroTrafficStat(props ...int64) *layoutAstroTrafficStat {
	r.section = "AstroTrafficStat"
	v := &layoutAstroTrafficStat{}
	r.expect(r.number(4, true), 0, "AstroTrafficStat.version")
	r.skip(4)
	var trafficCursor int64
	trafficCursor = r.number(4, true)
	ntrafficPool := r.length(trafficCursor - 1)
	for i := int64(0); i < ntrafficPool; i++ {
		r.readTrafficStat()
	}
	var numItemIndices int64
	numItemIndices = r.number(4, true)
	nitemIndices := r.length(numItemIndices)
	r.skip(nitemIndices * 4)
	return v
}

// layoutTrafficStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTrafficStat struct {
}

// readTrafficStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTrafficStat(props ...int64) *layoutTrafficStat {
	r.section = "TrafficStat"
	v := &layoutTrafficStat{}
	r.expect(r.number(4, true), 1, "TrafficStat.version")
	var numCount int64
	numCount = r.number(4, true)
	ncount := r.length(numCount)
	r.skip(ncount * 4)
	var numCursor int64
	numCursor = r.number(4, true)
	ncursor := r.length(numCursor)
	r.skip(ncursor * 4)
	var numTotal int64
	numTotal = r.number(4, true)
	ntotal := r.length(numTotal)
	r.skip(ntotal * 8)
	r.skip(4)
	return v
}

// layoutCustomCharts 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCustomCharts struct {
}

// readCustomCharts 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCustomCharts(props ...int64) *layoutCustomCharts {
	r.section = "CustomCharts"
	v := &layoutCustomCharts{}
	r.expect(r.number(4, true), 0, "CustomCharts.version")
	r.readDashboardViewState()
	r.readStatPlanPool()
	r.readDashboardLayout()
	r.readWatchLayout()
	return v
}

// layoutDashboardViewState 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDashboardViewState struct {
}

// readDashboardViewState 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDashboardViewState(props ...int64) *layoutDashboardViewState {
	r.section = "DashboardViewState"
	v := &layoutDashboardViewState{}
	r.expect(r.number(4, true), 0, "DashboardViewState.version")
	r.skip(4)
	return v
}

// layoutStatPlanPool 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStatPlanPool struct {
}

// readStatPlanPool 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStatPlanPool(props ...int64) *layoutStatPlanPool {
	r.section = "StatPlanPool"
	v := &layoutStatPlanPool{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readStatPlanExportStruct()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	norder := r.length(cursor - recycleCursor - 1)
	r.skip(norder * 4)
	return v
}

// layoutStatPlanExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStatPlanExportStruct struct {
}

// readStatPlanExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStatPlanExportStruct(props ...int64) *layoutStatPlanExportStruct {
	r.section = "StatPlanExportStruct"
	v := &layoutStatPlanExportStruct{}
	var v_type int64
	v_type = r.number(4, true)
	if v_type == 1 {
		r.readAstroItemProductionStatPlan()
	}
	if v_type == 2 {
		r.readAstroPowerStatPlan()
	}
	if v_type == 8 {
		r.readAstroResourceStatPlan()
	}
	if v_type == 6 {
		r.readAstroStorageStatPlan()
	}
	if v_type == 5 {
		r.readAstroTransportStatPlan()
	}
	if v_type == 3 {
		r.readDysonSphereStatPlan()
	}
	if v_type == 12 {
		r.readPlanetATFieldStatPlan()
	}
	if v_type == 56 {
		r.readSingleMonitorStatPlan()
	}
	if v_type == 51 {
		r.readSingleProducerStatPlan()
	}
	if v_type == 54 {
		r.readSingleStorageStatPlan()
	}
	if v_type == 11 {
		r.readTechStatPlan()
	}
	if v_type == 21 {
		r.readVeinGroupStatPlan()
	}
	if v_type == 1000 {
		r.readWarningStatPlan()
	}
	return v
}

// layoutAstroItemProductionStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroItemProductionStatPlan struct {
}

// readAstroItemProductionStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroItemProductionStatPlan(props ...int64) *layoutAstroItemProductionStatPlan {
	r.section = "AstroItemProductionStatPlan"
	v := &layoutAstroItemProductionStatPlan{}
	r.expect(r.number(4, true), 0, "AstroItemProductionStatPlan.version")
	r.skip(12)
	r.readStatPlan()
	return v
}

// layoutStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStatPlan struct {
}

// readStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStatPlan(props ...int64) *layoutStatPlan {
	r.section = "StatPlan"
	v := &layoutStatPlan{}
	r.expect(r.number(4, true), 1, "StatPlan.version")
	r.skip(8)
	var nameLength int64
	nameLength = r.number(4, true)
	if nameLength > 0 {
		r.string()
	}
	r.skip(4)
	return v
}

// layoutAstroPowerStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroPowerStatPlan struct {
}

// readAstroPowerStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroPowerStatPlan(props ...int64) *layoutAstroPowerStatPlan {
	r.section = "AstroPowerStatPlan"
	v := &layoutAstroPowerStatPlan{}
	r.expect(r.number(4, true), 0, "AstroPowerStatPlan.version")
	r.skip(12)
	r.readStatPlan()
	return v
}

// layoutAstroResourceStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroResourceStatPlan struct {
}

// readAstroResourceStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroResourceStatPlan(props ...int64) *layoutAstroResourceStatPlan {
	r.section = "AstroResourceStatPlan"
	v := &layoutAstroResourceStatPlan{}
	r.expect(r.number(4, true), 0, "AstroResourceStatPlan.version")
	r.skip(4)
	r.readStatPlan()
	return v
}

// layoutAstroStorageStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroStorageStatPlan struct {
}

// readAstroStorageStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroStorageStatPlan(props ...int64) *layoutAstroStorageStatPlan {
	r.section = "AstroStorageStatPlan"
	v := &layoutAstroStorageStatPlan{}
	r.expect(r.number(4, true), 0, "AstroStorageStatPlan.version")
	r.skip(8)
	r.readStatPlan()
	return v
}

// layoutAstroTransportStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroTransportStatPlan struct {
}

// readAstroTransportStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroTransportStatPlan(props ...int64) *layoutAstroTransportStatPlan {
	r.section = "AstroTransportStatPlan"
	v := &layoutAstroTransportStatPlan{}
	r.expect(r.number(4, true), 0, "AstroTransportStatPlan.version")
	r.skip(9)
	r.readStatPlan()
	return v
}

// layoutDysonSphereStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSphereStatPlan struct {
}

// readDysonSphereStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSphereStatPlan(props ...int64) *layoutDysonSphereStatPlan {
	r.section = "DysonSphereStatPlan"
	v := &layoutDysonSphereStatPlan{}
	r.expect(r.number(4, true), 0, "DysonSphereStatPlan.version")
	r.skip(8)
	r.readStatPlan()
	return v
}

// layoutPlanetATFieldStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlanetATFieldStatPlan struct {
}

// readPlanetATFieldStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlanetATFieldStatPlan(props ...int64) *layoutPlanetATFieldStatPlan {
	r.section = "PlanetATFieldStatPlan"
	v := &layoutPlanetATFieldStatPlan{}
	r.expect(r.number(4, true), 0, "PlanetATFieldStatPlan.version")
	r.skip(4)
	r.readStatPlan()
	return v
}

// layoutSingleMonitorStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSingleMonitorStatPlan struct {
}

// readSingleMonitorStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSingleMonitorStatPlan(props ...int64) *layoutSingleMonitorStatPlan {
	r.section = "SingleMonitorStatPlan"
	v := &layoutSingleMonitorStatPlan{}
	r.expect(r.number(4, true), 0, "SingleMonitorStatPlan.version")
	r.skip(8)
	r.readStatPlan()
	return v
}

// layoutSingleProducerStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSingleProducerStatPlan struct {
}

// readSingleProducerStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSingleProducerStatPlan(props ...int64) *layoutSingleProducerStatPlan {
	r.section = "SingleProducerStatPlan"
	v := &layoutSingleProducerStatPlan{}
	r.expect(r.number(4, true), 0, "SingleProducerStatPlan.version")
	r.skip(12)
	r.readStatPlan()
	return v
}

// layoutSingleStorageStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSingleStorageStatPlan struct {
}

// readSingleStorageStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSingleStorageStatPlan(props ...int64) *layoutSingleStorageStatPlan {
	r.section = "SingleStorageStatPlan"
	v := &layoutSingleStorageStatPlan{}
	r.expect(r.number(4, true), 0, "SingleStorageStatPlan.version")
	r.skip(8)
	r.readStatPlan()
	return v
}

// layoutTechStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTechStatPlan struct {
}

// readTechStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTechStatPlan(props ...int64) *layoutTechStatPlan {
	r.section = "TechStatPlan"
	v := &layoutTechStatPlan{}
	r.expect(r.number(4, true), 0, "TechStatPlan.version")
	r.skip(4)
	r.readStatPlan()
	return v
}

// layoutVeinGroupStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVeinGroupStatPlan struct {
}

// readVeinGroupStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVeinGroupStatPlan(props ...int64) *layoutVeinGroupStatPlan {
	r.section = "VeinGroupStatPlan"
	v := &layoutVeinGroupStatPlan{}
	r.expect(r.number(4, true), 1, "VeinGroupStatPlan.version")
	r.skip(12)
	var numExhaustedVeins int64
	numExhaustedVeins = r.number(4, true)
	nexhaustedVeins := r.length(numExhaustedVeins)
	for i := int64(0); i < nexhaustedVeins; i++ {
		r.readExhaustedVeins()
	}
	r.readStatPlan()
	return v
}

// layoutExhaustedVeins 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExhaustedVeins struct {
}

// readExhaustedVeins 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExhaustedVeins(props ...int64) *layoutExhaustedVeins {
	r.section = "ExhaustedVeins"
	v := &layoutExhaustedVeins{}
	r.skip(4)
	r.readVector3()
	return v
}

// layoutWarningStatPlan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutWarningStatPlan struct {
}

// readWarningStatPlan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readWarningStatPlan(props ...int64) *layoutWarningStatPlan {
	r.section = "WarningStatPlan"
	v := &layoutWarningStatPlan{}
	r.expect(r.number(4, true), 0, "WarningStatPlan.version")
	r.readStatPlan()
	return v
}

// layoutDashboardLayout 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDashboardLayout struct {
}

// readDashboardLayout 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDashboardLayout(props ...int64) *layoutDashboardLayout {
	r.section = "DashboardLayout"
	v := &layoutDashboardLayout{}
	r.expect(r.number(4, true), 0, "DashboardLayout.version")
	var numPages int64
	numPages = r.number(4, true)
	npage := r.length(numPages)
	for i := int64(0); i < npage; i++ {
		r.readExportStruct_int32__DashboardPage_()
	}
	return v
}

// layoutExportStruct_int32__DashboardPage_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DashboardPage_ struct {
}

// readExportStruct_int32__DashboardPage_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DashboardPage_(props ...int64) *layoutExportStruct_int32__DashboardPage_ {
	r.section = "ExportStruct<int32, DashboardPage>"
	v := &layoutExportStruct_int32__DashboardPage_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readDashboardPage()
	}
	return v
}

// layoutDashboardPage 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDashboardPage struct {
}

// readDashboardPage 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDashboardPage(props ...int64) *layoutDashboardPage {
	r.section = "DashboardPage"
	v := &layoutDashboardPage{}
	r.expect(r.number(4, true), 0, "DashboardPage.version")
	var nameLength int64
	nameLength = r.number(4, true)
	if nameLength > 0 {
		r.string()
	}
	var numChartDatas int64
	numChartDatas = r.number(4, true)
	nchartDatas := r.length(numChartDatas)
	for i := int64(0); i < nchartDatas; i++ {
		r.readChartData()
	}
	return v
}

// layoutChartData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutChartData struct {
}

// readChartData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readChartData(props ...int64) *layoutChartData {
	r.section = "ChartData"
	v := &layoutChartData{}
	r.expect(r.number(4, true), 1, "ChartData.version")
	r.readVector2Int()
	r.readVector2Int()
	r.skip(25)
	var numDisplayTypeParams int64
	numDisplayTypeParams = r.number(4, true)
	ndisplayTypeParams := r.length(numDisplayTypeParams)
	r.skip(ndisplayTypeParams * 4)
	return v
}

// layoutVector2Int 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVector2Int struct {
}

// readVector2Int 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVector2Int(props ...int64) *layoutVector2Int {
	r.section = "Vector2Int"
	v := &layoutVector2Int{}
	r.skip(8)
	return v
}

// layoutWatchLayout 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutWatchLayout struct {
}

// readWatchLayout 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readWatchLayout(props ...int64) *layoutWatchLayout {
	r.section = "WatchLayout"
	v := &layoutWatchLayout{}
	r.expect(r.number(4, true), 0, "WatchLayout.version")
	var numChartDatas int64
	numChartDatas = r.number(4, true)
	nchartDatas := r.length(numChartDatas)
	for i := int64(0); i < nchartDatas; i++ {
		r.readChartData()
	}
	return v
}
