package build

// 文件职责：验证并编排布局和自检；使用：factory Generate；边界：不信任前端修改的 plan，服务需另行复核来源指纹。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"
)

// generationTimeout 职责：限制单次完整蓝图生成占用时间；使用：所有布局、接驳与预检共享同一截止时间；边界：允许复杂多目标方案更长搜索，但取消仍立即生效。
const generationTimeout = 4 * time.Minute

// normalizeSetting 职责：补齐参考默认值并限制生成规模；使用：布局前；边界：不截断请求机器总数。
func normalizeSetting(s Setting) (Setting, error) {
	defaults := Setting{MaxMachineInALine: 15, BeltLv: 1, InserterLv: 1, MaxLays: 3, SmelterMaxCount: 15, WorkbenchMaxCount: 15, RefineryMaxCount: 15, ChemicalMaxCount: 15, ColliderMaxCount: 15, MatrixMaxCount: 999, MergeType: 2, BlockWidthLimit: 50, LocalStationHitRadius: 7}
	v, d := reflect.ValueOf(&s).Elem(), reflect.ValueOf(defaults)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Int:
			if field.Int() < 0 || field.Int() > 10000 {
				return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "布局整数参数超限"}
			}
			if field.Int() == 0 {
				field.Set(d.Field(i))
			}
		case reflect.Float64:
			n := field.Float()
			if n < 0 || n > 1000000 || math.IsNaN(n) || math.IsInf(n, 0) {
				return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "布局数值无效"}
			}
			if n == 0 {
				field.Set(d.Field(i))
			}
		}
	}
	if s.NeedPower == nil {
		value := true
		s.NeedPower = &value
	}
	if s.Mix2 == nil {
		value := true
		s.Mix2 = &value
	}
	if s.BeltLv > 3 || s.InserterLv > 4 || s.MaxLays > 15 || s.MergeType > 2 || s.BlockWidthLimit < 10 {
		return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "带/分拣器等级、堆叠或布局方式无效"}
	}
	// 旧工作区的 BeltSpeed 只作读取兼容，不能覆盖所选等级的单带能力。
	capacity, err := transport.BeltCapacityPerMinute(s.BeltLv)
	if err != nil {
		return s, err
	}
	s.BeltSpeed = capacity
	return s, nil
}

// lineCounts 职责：按参考运力和列上限均匀拆列；使用：每条量化产线；边界：运力不足时减列，不丢机器。
func lineCounts(p quant.Production, r quant.Recipe, s Setting, speed float64) ([]int, error) {
	capacity, err := transport.BeltCapacityPerMinute(s.BeltLv)
	if err != nil {
		return nil, err
	}
	if p.MachineCount <= 0 || p.MachineCount > 10000 || r.TimeSpend <= 0 || !finitePositive(speed) {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "机器数、配方周期或设备速度无效"}
	}
	maxUse := 0.
	for _, n := range r.ItemCounts {
		maxUse = math.Max(maxUse, n)
	}
	output := 0.
	for _, n := range r.ResultCounts {
		output += n
	}
	maxUse = math.Max(maxUse, output)
	acc := 1.
	if p.ProductionMode == 1 && p.IncreaseLv > 0 {
		rates := []float64{0, .25, .5, 1}
		acc += rates[p.IncreaseLv]
	}
	perMachine := 60 / (r.TimeSpend / 60) * speed * maxUse * acc
	if !finitePositive(perMachine) {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 没有有效的单机物料流量", r.ID)}
	}
	theory := int(math.Floor((capacity + 1e-6) / perMachine))
	limit := s.maxCount(p.MachineType)
	if s.MaxMachineInALine > 0 && p.MachineType != 6 {
		limit = min(limit, s.MaxMachineInALine)
	}
	limit = min(limit, theory)
	// 配方额定值之外还核对可信计划的实际净流量；共享输出带必须承载全部产物。
	rowRate := 0.
	for _, rate := range p.Inputs {
		rowRate = math.Max(rowRate, rate.RatePerMinute)
	}
	outputRate := 0.
	for _, rate := range p.Outputs {
		outputRate += rate.RatePerMinute
	}
	rowRate = math.Max(rowRate, outputRate)
	if rowRate > 0 {
		limit = min(limit, int(math.Floor((capacity+1e-6)*float64(p.MachineCount)/rowRate)))
	}
	if limit <= 0 {
		return nil, &blueprint.Error{Code: "GEN_E_OVERFLOW", Message: "带速不足以支持一台机器，请升级传送带"}
	}
	lines := int(math.Ceil(float64(p.MachineCount) / float64(limit)))
	balanced := int(math.Ceil(float64(p.MachineCount) / float64(lines)))
	result := []int{}
	for left := p.MachineCount; left > 0; {
		n := min(left, balanced)
		result = append(result, n)
		left -= n
	}
	return result, nil
}

// coatSupply 职责：连接同组喷涂机供剂带；使用：有喷涂的布局；边界：默认暴露供给口，不伪造已存在的增产剂。
func (b *builder) coatSupply(lines []*line) error {
	created := false
	for _, l := range lines {
		if err := b.checkGeneration("喷涂供剂干线"); err != nil {
			return err
		}
		if l == nil {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂供剂缺少已装配机器行"}
		}
		if len(l.coaters) == 0 {
			continue
		}
		if l.coaterInput != nil {
			continue
		}
		if len(l.machines) == 0 {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的喷涂机器行没有任何真实机器，拒绝输出装饰喷涂网络", l.recipe.ID)}
		}
		// 原料切片保持配方逻辑顺序，底侧物理槽位已反转；供剂干线
		// 必须按喷涂机真实Y排序，不能把逻辑首尾当作递增几何区间。
		coaters := append([]*placed(nil), l.coaters...)
		for _, coater := range coaters {
			if coater == nil {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂机器行存在空喷涂端点"}
			}
		}
		sort.SliceStable(coaters, func(i, j int) bool { return coaters[i].a.y < coaters[j].a.y })
		first, last := coaters[0], coaters[len(coaters)-1]
		for _, coater := range coaters {
			if math.Abs(coater.a.x-first.a.x) > 1e-6 {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 的喷涂口横坐标不一致，当前局部供剂干线无法连接", l.recipe.ID)}
			}
		}
		// 入口降到地面，再用一个水平格加一层的真实斜坡进入供剂轨；
		// 各模式可以统一连接地面端点，不要求箱体臂跨层斜插。
		points := []vector{{first.a.x - 3, first.a.y - 1, 0}, {first.a.x - 2, first.a.y - 1, 0}}
		for y := first.a.y - 1; y <= last.a.y; y++ {
			points = append(points, vector{first.a.x - 1, y, 1})
		}
		owner := b.owner
		b.owner = l.machines[0].owner
		belts := b.byPath(points)
		b.owner = owner
		// 塔接驳从l.all读取占位；供剂入口必须同步登记到本行独立切片，
		// 避免在共享底层数组上append覆盖随后产线或让斜坡成为隐形障碍。
		l.all = append(append([]*placed(nil), l.all...), belts...)
		id := productionSprayItem(l.info)
		if id == 0 {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的喷涂设施没有合法增产剂等级", l.recipe.ID)}
		}
		belts[0].record.Parameters = []int32{int32(id), 0}
		l.coaterInput = belts[0]
		b.coaterLinks[id] = append(b.coaterLinks[id], belts[0])
		created = true
	}
	if created {
		b.warnings = append(b.warnings, "喷涂机供剂已按物流诊断中的物料来源接入；标记为外部供给的端口需持续提供增产剂，供剂不足会降低产能")
	}
	return nil
}

// Generate 职责：生成并严格自检蓝图；使用：服务对已验证方案调用；边界：超时/布局失败不返回部分串，预览不参与判定。
func Generate(parent context.Context, req Request) (Result, error) {
	result := Result{Warnings: []string{}, Bindings: []ProductionBinding{}, SharedBuildingIndices: []int{}}
	reportProgress(parent, "validate", "正在校验生成参数", 0, 0)
	if req.Mode < 1 || req.Mode > 6 {
		return result, &blueprint.Error{Code: "GEN_E_MODE", Message: "策略必须为 1 到 6"}
	}
	surface, err := normalizeSurfaceLayout(req.SurfaceLayout)
	if err != nil {
		return result, err
	}
	req.SurfaceLayout = surface
	ctx, cancel := context.WithTimeout(parent, generationTimeout)
	defer cancel()
	setting, err := normalizeSetting(req.Setting)
	if err != nil {
		return result, err
	}
	req.Setting = setting
	dataset, err := quant.LoadDataset(req.Plan.Dataset)
	if err != nil {
		return result, err
	}
	if req.Plan.DataVersion != dataset.Info.DataVersion || req.Plan.PlanID == "" {
		return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "数据集已变化，请重新量化"}
	}
	if len(req.Plan.Productions) == 0 {
		return result, &blueprint.Error{Code: "BP_E_EMPTY", Message: "当前方案没有可生成的制造产线"}
	}
	b, err := newBuilder(setting)
	if err != nil {
		return result, err
	}
	b.generationCtx = ctx
	// 所有模式共享同一份守恒物料图和容量诊断；不能让普通生成路径
	// 继续在没有 flowGraph 的情况下按匿名产线返回“成功”蓝图。
	graph, err := quant.BuildFlowGraphContext(ctx, req.Plan)
	if err != nil {
		return result, err
	}
	b.flowGraph = &graph
	b.logistics = newLogisticsSummary(graph, setting)
	for _, production := range req.Plan.Productions {
		if err := ctx.Err(); err != nil {
			return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "设备端口能力核验已取消或超时"}
		}
		if err := validateProductionCapacity(production, dataset, setting); err != nil {
			return result, err
		}
		recipe := dataset.Recipes[dataset.RecipeByID[production.RecipeID]]
		if err := validateMachineInserterCapacity(production, recipe, setting); err != nil {
			return result, err
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "machine-inserter-capacity", Status: "passed", Message: "全部生产节点按所选分拣器等级、真实跨度及研究站堆叠核算机器端口额定能力；不依赖未知科技补齐超载"})
	if req.Mode == 1 {
		return b.generateConnected(ctx, req, dataset)
	}
	if req.Mode == 6 {
		return b.generateBus(ctx, req, dataset)
	}
	// 设施模式保留目标分组，但组内顺序必须来自同一规范依赖图。
	reportProgress(ctx, "zones", "正在规划产线分区", 0, 0)
	zones := planZones(req.Plan)
	order := make(map[int]int, len(graph.ProductionOrder))
	for rank, index := range graph.ProductionOrder {
		order[index] = rank
	}
	for i := range zones {
		sort.SliceStable(zones[i].ProductionIndices, func(left, right int) bool {
			return order[zones[i].ProductionIndices[left]] < order[zones[i].ProductionIndices[right]]
		})
	}
	// bounds 已包含本地带、分拣器和电力设施；不再为所有块预留物流塔半径。
	// 塔型的真实安全距离在放置时单独补齐，并由最终规则再次核验。
	const blockGap = 2.
	const zoneGap = blockGap
	zoneX := 0.
	productionNumber := 0
	for zi := range zones {
		zone := &zones[zi]
		zoneBegin := len(b.all)
		nextX, nextY, columnWidth := zoneX, 0., 0.
		for _, index := range zone.ProductionIndices {
			p := req.Plan.Productions[index]
			productionNumber++
			productionMessage := "正在铺设" + dataset.ByID[p.MainItemID].Name + "产线"
			reportProgress(ctx, "production", productionMessage, productionNumber, len(req.Plan.Productions))
			if ctx.Err() != nil {
				return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过 4 分钟"}
			}
			if p.MachineCount <= 0 || p.MachineCount > 10000 || p.IncreaseLv < 0 || p.IncreaseLv > 3 {
				return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "方案机器数量或等级无效"}
			}
			ri, ok := dataset.RecipeByID[p.RecipeID]
			if !ok {
				return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "配方已变化"}
			}
			recipe := dataset.Recipes[ri]
			valid := false
			for _, id := range recipe.Factories {
				if p.MachineID == id {
					valid = true
				}
			}
			if !valid {
				return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "设施与配方不匹配"}
			}
			b.owner = index
			counts, err := lineCounts(p, recipe, setting, dataset.ByID[p.MachineID].Speed)
			if err != nil {
				return result, err
			}
			perGroup := len(counts)
			if req.SurfaceLayout != nil && req.Mode != 3 && req.Mode != 4 {
				// 球面多 Area 只能移动连接闭合的局部产线块，非塔式模式按单列保留边界。
				perGroup = 1
			}
			if req.Mode == 3 || req.Mode == 4 {
				inputItems, resultItems := map[int]bool{}, map[int]bool{}
				for _, item := range recipe.Items {
					inputItems[item] = true
				}
				for _, item := range recipe.Results {
					resultItems[item] = true
				}
				if spray := productionSprayItem(p); spray > 0 {
					inputItems[spray] = true
				}
				routeCount := len(inputItems) + len(resultItems)
				perGroup = max(1, 12/max(1, routeCount))
				slots := 4
				if req.Mode == 4 {
					slots = 5
				}
				if routeCount > slots*2 {
					perGroup = 1
				}
				if len(resultItems) > 1 {
					// 多产物需要独立过滤支路；逐行独立塔组避免短行出口被相邻行输入封闭。
					perGroup = 1
				}
			}
			for groupStart := 0; groupStart < len(counts); groupStart += perGroup {
				reportProgress(ctx, "production", productionMessage, productionNumber, len(req.Plan.Productions))
				groupEnd := min(groupStart+perGroup, len(counts))
				begin := len(b.all)
				lines := []*line{}
				lineY := 0.
				for i := groupStart; i < groupEnd; i++ {
					// 独立塔组会整体移到不同位置，补电奇偶和末行必须以组内行序计算。
					lineIndex, last := i-groupStart, i == groupEnd-1
					if req.SurfaceLayout != nil && req.Mode != 3 && req.Mode != 4 {
						lineIndex, last = 0, true
					}
					l, err := b.buildLine(p, recipe, counts[i], req.Mode, lineIndex, last)
					if err != nil {
						return result, err
					}
					minPoint, maxPoint := bounds(l.all)
					shift(l.all, -minPoint.x, lineY-minPoint.y, 0)
					lineY += math.Ceil(maxPoint.y-minPoint.y) + 1
					lines = append(lines, l)
				}
				if err := b.coatSupply(lines); err != nil {
					return result, err
				}
				if req.Mode == 3 || req.Mode == 4 {
					reportProgress(ctx, "stations", "正在连接物流站与传送带", productionNumber, len(req.Plan.Productions))
					if err := b.attachStations(ctx, lines, req.Mode); err != nil {
						return result, err
					}
				}
				block := b.all[begin:]
				minimum, maximum := bounds(block)
				width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
				if setting.MergeType == 2 && nextY > 0 && nextY+height > setting.BlockWidthLimit {
					nextX += columnWidth + blockGap
					nextY = 0
					columnWidth = 0
				}
				if req.Mode == 3 || req.Mode == 4 {
					nextY += stationBlockClearance(b.all[:begin], block, nextX-minimum.x, nextY-minimum.y)
				}
				shift(block, nextX-minimum.x, nextY-minimum.y, 0)
				nextY += height + blockGap
				columnWidth = math.Max(columnWidth, width)
				if req.SurfaceLayout != nil {
					b.surfaceBlocks = append(b.surfaceBlocks, surfaceBlockRange{begin: begin, end: len(b.all)})
				}
				if len(b.all) > blueprint.MaxBuildingCount {
					return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑超过 20 万，请拆分生成"}
				}
			}
			if len(recipe.Results) > 1 {
				b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 的多个产物共用输出带，请保证所有副产物有消耗出口", p.RecipeID))
			}
		}
		if len(b.all) > zoneBegin {
			_, maximum := bounds(b.all[zoneBegin:])
			zoneX = math.Ceil(maximum.x) + 1 + zoneGap
		}
		b.zoneRanges = append(b.zoneRanges, zoneRange{zone: zi, begin: zoneBegin, end: len(b.all)})
	}
	if len(zones) > 1 {
		b.warnings = append(b.warnings, fmt.Sprintf("多目标已按 %d 个分区排开：共用中间产物只生成一份，并按规范供需边配置各区物流；原料、目标及剩余副产物的外部接口见物流诊断", len(zones)))
	}
	b.zones = zones
	return b.finish(ctx, req, dataset)
}

// planZones 职责：按产线服务的目标划分分区；使用：普通模式布局；边界：只服务一个目标的产线归该目标，多目标或无目标归「共用」。
func planZones(plan quant.Result) []Zone {
	zones := []Zone{}
	byTarget := map[int]int{}
	for _, target := range plan.Targets {
		if _, ok := byTarget[target.ItemID]; ok {
			continue
		}
		byTarget[target.ItemID] = len(zones)
		zones = append(zones, Zone{Key: fmt.Sprintf("target-%d", target.ItemID), TargetItemID: target.ItemID, ProductionIndices: []int{}})
	}
	shared := Zone{Key: "shared", ProductionIndices: []int{}}
	for i, p := range plan.Productions {
		if len(p.TargetItemIDs) == 1 {
			if zi, ok := byTarget[p.TargetItemIDs[0]]; ok {
				zones[zi].ProductionIndices = append(zones[zi].ProductionIndices, i)
				continue
			}
		}
		shared.ProductionIndices = append(shared.ProductionIndices, i)
	}
	result := []Zone{}
	if len(shared.ProductionIndices) > 0 && len(zones) > 1 {
		result = append(result, shared)
	} else if len(shared.ProductionIndices) > 0 && len(zones) == 1 {
		zones[0].ProductionIndices = append(zones[0].ProductionIndices, shared.ProductionIndices...)
	}
	for _, zone := range zones {
		if len(zone.ProductionIndices) > 0 {
			result = append(result, zone)
		}
	}
	if len(result) == 0 {
		result = append(result, Zone{Key: "shared", ProductionIndices: shared.ProductionIndices})
	}
	return result
}

// finish 职责：构建文档、归属和严格编码；使用：所有模式共同出口；边界：只有 Go 自检通过才返回串。
func (b *builder) finish(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	result := Result{}
	// 增产剂现在和其它物料一样走规范流边；旧可选网络不能再次接入同一端点造成重复带和双重供给。
	if req.Mode == 6 && req.SurfaceLayout != nil {
		reportProgress(ctx, "surface", "正在折叠球面总线布局", 0, 0)
		if err := b.foldSurfaceBus(ctx, req.SurfaceLayout); err != nil {
			return result, err
		}
	}
	if len(b.all) > blueprint.MaxBuildingCount {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑超过 20 万，请拆分生成"}
	}
	if len(b.all) > 100000 {
		b.warnings = append(b.warnings, "建筑超过 10 万，生成和游戏内粘贴可能较慢；建议按产物拆分，并关闭实时预览")
	}
	var surface *surfaceEncoding
	if req.SurfaceLayout != nil {
		var err error
		reportProgress(ctx, "surface", "正在适配行星网格与纬带", 0, 0)
		surface, err = b.prepareSurfaceEncodingContext(ctx, req.SurfaceLayout)
		if err != nil {
			return result, err
		}
	} else {
		minimumBefore, _ := bounds(b.all)
		shift(b.all, -minimumBefore.x, -minimumBefore.y, 0)
	}
	if ctx.Err() != nil {
		return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过 4 分钟"}
	}
	reportProgress(ctx, "records", "正在整理建筑与产线归属", 0, 0)
	records, bindings, shared, err := b.records()
	if err != nil {
		return result, err
	}
	if surface != nil {
		if err := surface.applyRecords(b.all, records); err != nil {
			return result, err
		}
		reportProgress(ctx, "collision", "正在检查球面锚点与建筑碰撞", 0, 0)
		err = surface.validateAnchorCandidates(ctx, records)
		if err != nil && req.Mode == 1 && surfaceAnchorRetryable(err) && !surface.packed && len(b.transportRoutes) > 0 && (req.SurfaceLayout.Mode == "autoTropic" || req.SurfaceLayout.ExpandAdjacent) {
			reportProgress(ctx, "surface-retry", "正在保留物料连接并扩展相邻纬带", 0, 0)
			// 直连图必须逐原始带边补齐接驳，不能像独立产线块一样平移后留下断线。
			surface, err = b.prepareConnectedSurfaceEncoding(ctx, req.SurfaceLayout)
			if err == nil {
				records, bindings, shared, err = b.records()
			}
			if err == nil {
				err = surface.applyRecords(b.all, records)
			}
			if err == nil {
				err = surface.validateAnchorCandidates(ctx, records)
			}
		}
		if err != nil && surface != nil && surfaceAnchorRetryable(err) && !surface.packed && len(b.surfaceBlocks) > 0 {
			reportProgress(ctx, "surface-retry", "正在重排产线并复查球面布局", 0, 0)
			// 矩形容量通过但所有锚点几何失败时，仍可按完整块重排，而不是把容量可行误当成唯一方案。
			surface, err = b.preparePackedSurfaceEncoding(ctx, req.SurfaceLayout)
			if err == nil {
				records, bindings, shared, err = b.records()
			}
			if err == nil {
				err = surface.applyRecords(b.all, records)
			}
			if err == nil {
				err = surface.validateAnchorCandidates(ctx, records)
			}
		}
		if err != nil {
			return result, err
		}
	}
	zones := []Zone{}
	for _, r := range b.zoneRanges {
		if r.end <= r.begin {
			continue
		}
		zone := b.zones[r.zone]
		minimum, maximum := bounds(b.all[r.begin:r.end])
		zone.MinX, zone.MinY, zone.MaxX, zone.MaxY = minimum.x, minimum.y, maximum.x, maximum.y
		zones = append(zones, zone)
	}
	minimum, maximum := bounds(b.all)
	width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
	if width > 32767 || height > 32767 {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "蓝图尺寸超过区域整数上限"}
	}
	var surfacePlan *SurfacePlan
	areas := []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: int16(width), Height: int16(height)}}
	primaryArea := int32(0)
	if surface != nil {
		surfacePlan, areas = surface.plan, surface.areas
		primaryArea = int32(surfacePlan.PrimaryAreaIndex)
		width, height = float64(surfacePlan.LayoutWidth), float64(surfacePlan.LayoutHeight)
	}
	reportProgress(ctx, "rules", "正在核验最终建筑、路线、槽位和供电规则", 0, 0)
	ruleAudit, err := validateFinalPlanetRules(ctx, req, surface, records)
	if err != nil {
		return result, err
	}
	// 所有模式在编码前统一把规范流边绑定到实际建筑链，并生成最终
	// 物流诊断；否则 flowGraph 只停留在计划层，结果会丢失 lane/入口证据。
	if req.Mode == 3 || req.Mode == 4 {
		if err := b.validateStationTransport(ctx, req.Mode, records); err != nil {
			return result, err
		}
	} else if req.Mode == 2 || req.Mode == 5 {
		if err := b.recordFacilityTransport(req.Mode); err != nil {
			return result, err
		}
	}
	logistics, err := b.finalizeLogistics(req.Mode, records, areas)
	if err != nil {
		return result, err
	}
	name := req.Name
	if name == "" {
		name = "量化产线"
	}
	dragWidth, dragHeight := int32(width), int32(height)
	if surface != nil {
		dragWidth, dragHeight = int32(areas[primaryArea].Width), 0
		for _, area := range areas {
			dragHeight += int32(area.Height)
		}
	}
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Patch: 1, Name: name, GameVersion: dataset.Info.GameVersion, Description: "戴森球盒子生成；需确认所有原料及副产物出口", DragBoxSize: blueprint.PointInt{X: dragWidth, Y: dragHeight}, PrimaryAreaIndex: primaryArea, CursorTargetArea: primaryArea, Areas: areas, Buildings: records}
	reportProgress(ctx, "encode", "正在编码并校验蓝图", 0, 0)
	encoded, err := blueprint.Encode(document, 1)
	if err != nil {
		return result, err
	}
	if len(records) > 6000 {
		b.warnings = append(b.warnings, "建筑超过 6000，预览可能较慢，可折叠预览后复制导出")
	}
	return Result{Blueprint: encoded.Blueprint, BuildingCount: len(records), Areas: len(areas), Warnings: b.warnings, SourcePlanID: req.Plan.PlanID, Bindings: bindings, SharedBuildingIndices: shared, Zones: zones, SurfacePlan: surfacePlan, LogisticsSummary: logistics, RuleAudit: ruleAudit}, nil
}

// surfaceAnchorRetryable 职责：判断球面锚点预检失败是否可以换到邻圈重试；使用：Generate 候选循环；边界：未知原型、超时和未验证错误不被软化。
func surfaceAnchorRetryable(err error) bool {
	failure, ok := err.(*blueprint.Error)
	if !ok {
		return false
	}
	return failure.Code == "GEN_E_COLLISION" || failure.Code == "GEN_E_SURFACE_ROUTING"
}
