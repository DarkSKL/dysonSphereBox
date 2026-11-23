package planet

// 文件职责：把整数生产模块装入真实纬带；使用：整星制造；边界：直连组合和独立块都只分配实际对外边界。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

type moduleTemplate struct {
	generated build.Result
	document  blueprint.Document
	err       error
	// alternative 按需将同次已通过规则的备用快照编码一次，独立块与跨星快速布局留空。
	alternative func(context.Context) moduleTemplate
}

// moduleSiteCursor 记录同一装配里同一模板已经检查的落点；该次装配的地形固定，已占空间只会增加。
type moduleSiteCursor struct {
	row, x    int
	last      error
	exhausted bool
	placedAt  Position
	certified bool
}

const moduleLongitudeStride = 5

type templateCacheKey struct{}

// powerDeferredRegion 只记录完整搜索因整份合同缺电而推迟的额定模板，供同次规划再次排队时识别。
type powerDeferredRegion struct {
	planetID      int64
	specification string
}

// moduleRegionHintsKey 职责：隔离本次任务的成功选区提示；使用：稳定制造块重核；边界：提示不代表落位、物流或产能已经通过。
type moduleRegionHintsKey struct{}

// moduleRegionHintKey 职责：绑定同星球同制造块的额定模板；使用：安全调整重核顺序；边界：不同块、硬件、设置和版本不共享成功区域。
type moduleRegionHintKey struct {
	planetID int64
	blockID  string
	template string
}

// WithTemplateCache 职责：在一次产业线任务中复用纯制造模板和稳定块成功选区；使用：候选修正共享；边界：每次仍重新核验星球落位、物流和产能，任务结束后释放。
func WithTemplateCache(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, templateCacheKey{}, &sync.Map{})
	ctx = context.WithValue(ctx, directSiteSearchKey{}, &sync.Map{})
	return context.WithValue(ctx, moduleRegionHintsKey{}, &sync.Map{})
}

// moduleMaterialPlan 职责：按已核验的整数设备额定净收支生成独立模板；使用：同硬件需求微调时复用真实供料能力；边界：不改原计划，旧快照缺少 Capacity 时保持原需求，毛流仍保留在 Capacity 供设备端口核验。
func moduleMaterialPlan(ctx context.Context, base quant.Result, p quant.Production, index int) (quant.Result, error) {
	if p.Capacity != nil {
		if err := ctx.Err(); err != nil {
			return quant.Result{}, err
		}
		dataset, err := quant.LoadDataset(base.Dataset)
		if err != nil {
			return quant.Result{}, err
		}
		capacity, err := quant.ValidatedProductionCapacity(dataset, p)
		if err != nil {
			return quant.Result{}, err
		}
		p.OutputCount, p.PowerMW = capacity.OutputPerMinute, capacity.PowerMW
		p.ExactMachineCount, p.RoundingMargin = float64(p.MachineCount), 0
		// 与整数切片相同：乘积再除单机能力可能多出一个舍入位，不让额定模板被误判超负荷。
		if p.OutputCount/capacity.OutputPerMachinePerMinute > p.ExactMachineCount {
			p.OutputCount = math.Nextafter(p.OutputCount, 0)
		}
		p.Inputs = append([]quant.ItemRate(nil), capacity.Inputs...)
		p.Outputs = append([]quant.ItemRate(nil), capacity.Outputs...)
		capacity.Utilization = 1
		p.Capacity = capacity
	}
	return manufacturingMaterial(ctx, base, []quant.Production{p}, index)
}

// manufacturingMaterial 按该组实际生产实例重建守恒流图。同组消耗的中间品仅在仍有净余缺时占用边界。
func manufacturingMaterial(ctx context.Context, base quant.Result, rows []quant.Production, index int) (quant.Result, error) {
	plan := quant.Result{PlanID: fmt.Sprintf("%s-block-%d", base.PlanID, index), Dataset: base.Dataset, DataVersion: base.DataVersion, GameVersion: base.GameVersion, Productions: rows}
	net := map[int]float64{}
	for _, p := range rows {
		plan.MachineCount += p.MachineCount
		plan.PowerMW += p.PowerMW
		for _, v := range p.Inputs {
			net[v.ItemID] -= v.RatePerMinute
		}
		for _, v := range p.Outputs {
			net[v.ItemID] += v.RatePerMinute
		}
	}
	ids := make([]int, 0, len(net))
	for id := range net {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if net[id] < -quant.FlowTolerance(0, 0) {
			plan.RawInputs = append(plan.RawInputs, quant.ItemRate{ItemID: id, RatePerMinute: -net[id]})
			plan.ExternalInputItemIDs = append(plan.ExternalInputItemIDs, id)
		} else if net[id] > quant.FlowTolerance(0, 0) {
			plan.Targets = append(plan.Targets, quant.Target{ItemID: id, RatePerMinute: net[id]})
		}
	}
	graph, err := quant.BuildFlowGraphContext(ctx, plan)
	plan.FlowGraph = &graph
	return plan, err
}

// moduleBoundaryPorts 职责：将额定模板边界限制到当前模块的实际净收支；使用：制造落位后的合同分配；边界：同物品多端口共享余量，零流端口仍保留供远程角色重置，缓存和设备毛流不改动。
func moduleBoundaryPorts(production quant.Production, ports []build.BoundaryPort) []build.BoundaryPort {
	net := map[int]float64{}
	for _, input := range production.Inputs {
		net[input.ItemID] -= input.RatePerMinute
	}
	for _, output := range production.Outputs {
		net[output.ItemID] += output.RatePerMinute
	}
	bounded := append([]build.BoundaryPort(nil), ports...)
	for i := range bounded {
		port := &bounded[i]
		if port.Kind == "external-input" {
			port.RatePerMinute = min(port.RatePerMinute, max(0, -net[port.ItemID]))
			net[port.ItemID] += port.RatePerMinute
		} else {
			port.RatePerMinute = min(port.RatePerMinute, max(0, net[port.ItemID]))
			net[port.ItemID] -= port.RatePerMinute
		}
	}
	return bounded
}

// appendManufacturing 职责：遍历支持的选区装配制造模块，再反馈未落位后缀；使用：每块真实碰撞与液面校验；边界：有限候选失败不证明整星已满，也不发布半成品。
func appendManufacturing(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog) error {
	rows := input.Plan.Plan.Productions
	reported := make([]int, len(rows))
	var subdivided *industryplan.FactoryPlacementError
	if !input.Plan.SinglePlanet {
		parts, parents, err := subdivideWideRecipes(ctx, input.Plan)
		if err != nil {
			return err
		}
		if len(parents) > 0 {
			trial, local := *req, input
			local.Plan = parts
			partCounts := make([]int, len(parents))
			err := appendManufacturingRows(ctx, &trial, local, actual, catalog, newDirectModules(ctx, parts.Plan), partCounts)
			if err == nil {
				*req = trial
				return nil
			}
			var failed *industryplan.FactoryPlacementError
			if !errors.As(err, &failed) || ctx.Err() != nil {
				return err
			}
			prefix := *failed
			prefix.FirstUnplaced = parents[failed.FirstUnplaced]
			subdivided = &prefix
			for i, count := range partCounts {
				reported[parents[i]] += count
			}
			// 试摆仅追加实体，未通过的工区尚未登记站点、机队与端口。恢复原块再决定是否迁星。
		}
	}
	err := appendManufacturingRows(ctx, req, input, actual, catalog, newDirectModules(ctx, input.Plan.Plan), reported)
	var whole *industryplan.FactoryPlacementError
	if subdivided != nil && errors.As(err, &whole) && subdivided.FirstUnplaced > whole.FirstUnplaced {
		return subdivided
	}
	return err
}

// appendManufacturingRows 落位全部制造块；组合占地导致后续工序放不下时，原星再用独立模块复核一次。
func appendManufacturingRows(ctx context.Context, req *Request, input ProductionRequest, actual *industryplan.PlanetActual, catalog *placement.Catalog, direct directModules, reported []int) error {
	if len(input.Plan.Plan.Productions) == 0 {
		return nil
	}
	// 本次试摆只追加实体。组合工区失败后独立试摆会建立自己的占地摘要。
	req.manufacturingOccupancy = &manufacturingOccupancy{}
	defer func() { req.manufacturingOccupancy = nil }()
	parallel := 2
	if input.Options.Profile == "low" {
		parallel = 1
	}
	var boundary []build.BoundaryPort
	// 使用生成器公开的选区，才能同时保留目标纬带及邻带扩展许可；先覆盖各区中心，再探测边缘。
	regions := build.DensityLatitudeOptions()
	sort.SliceStable(regions, func(i, j int) bool {
		return regions[i].Position == "center" && regions[j].Position != "center"
	})
	templates, ok := ctx.Value(templateCacheKey{}).(*sync.Map)
	if !ok {
		templates = &sync.Map{}
	}
	hints, _ := ctx.Value(moduleRegionHintsKey{}).(*sync.Map)
	cursors := map[string]*moduleSiteCursor{}
	occupied := newObstacleIndex(append(extendOccupancy(*req, catalog), req.minerals...))
	relocatable := input.Plan.CanRelocate && len(input.Plan.Plan.Productions) > 1
	input.Options.firstVerifiedLayout = relocatable || input.Plan.CenterRegionsOnly
	// 原星低纬试摆仍给出真实前缀。整份电力合同已不可能供电时，不为它新建高纬模板。
	powerFloor := 0.
	if !input.Plan.SinglePlanet {
		powerFloor = insufficientManufacturingPower(*req, input)
	}
	quickCenterTiers := 2
	if receivers := input.Plan.Photons; receivers != nil && receivers.Count > 0 && powerFloor == 0 {
		// 接收器在制造前占地。实档的前两档都被挡住，试到第四档仍可在原星留下十二块工序。
		quickCenterTiers = 4
	}
	// 完整搜索按序比较所有候选。只并行其中的独立装配，限时时窗、低算力和单星仍用原槽数。
	independentWorkers := parallel
	if !relocatable && !input.Plan.SinglePlanet && parallel > 1 && search.Controlled(ctx) && !buildcore.CandidateBudgetLimited(ctx) {
		independentWorkers = min(8, runtime.GOMAXPROCS(0))
	}
	initialObjects, initialVersion := len(req.Objects), req.GameVersion
	directGroups := 0
	directItems := map[int]bool{}
	misses := directSiteSearch(ctx, input)
	var ahead *directTemplateAhead
	if relocatable && direct != nil && misses != nil && parallel > 1 && runtime.GOMAXPROCS(0) >= 8 && !buildcore.CandidateBudgetLimited(ctx) {
		aheadCtx, cancel := context.WithCancel(ctx)
		ahead = &directTemplateAhead{ctx: aheadCtx}
		defer func() {
			cancel()
			if ahead.done != nil {
				<-ahead.done
			}
		}()
	}
	placedRows := make([]bool, len(input.Plan.Plan.Productions))
	markPlaced := func(index int) {
		placedRows[index] = true
		count := input.Plan.Plan.Productions[index].MachineCount
		if count > reported[index] {
			industryplan.ReportPlacedMachines(ctx, count-reported[index])
			reported[index] = count
		}
	}
	for index, production := range input.Plan.Plan.Productions {
		if err := search.Checkpoint(ctx); err != nil {
			return err
		}
		if placedRows[index] {
			continue
		}
		for _, group := range direct.groups(index, placedRows, input.Plan.Plan.Productions) {
			objects, template, err := placeDirectModules(ctx, *req, input, group, parallel, regions[0], occupied, catalog, templates, cursors, misses, ahead)
			if err != nil {
				return err
			}
			if len(objects) == 0 {
				continue
			}
			summary := template.generated.LogisticsSummary
			attachManufacturing(req, objects, summary.ExternalPorts, template.document.GameVersion, &occupied, catalog, &boundary)
			directGroups++
			for _, lane := range summary.Lanes {
				if lane.InternalConnected {
					directItems[lane.ItemID] = true
				}
			}
			for _, member := range group {
				markPlaced(member)
			}
			if ahead != nil {
				ahead.prepare(input, direct, placedRows, index+1, parallel, regions[0], templates)
			}
			break
		}
		if placedRows[index] {
			continue
		}
		material, err := moduleMaterialPlan(ctx, input.Plan.Plan, production, index)
		if err != nil {
			return err
		}
		placed, deferredRegions, deferredPower := false, false, false
		var last error
		identity, err := independentTemplateIdentity(material, input.Options.Setting, parallel)
		if err != nil {
			return err
		}
		ordered := regions
		var hintKey moduleRegionHintKey
		if hints != nil && index < len(input.Plan.BlockIDs) && input.Plan.BlockIDs[index] != "" {
			hintKey = moduleRegionHintKey{planetID: req.PlanetID, blockID: input.Plan.BlockIDs[index], template: identity}
			if preferred, found := hints.Load(hintKey); found {
				for i, region := range regions {
					if region.Value == preferred && i > 0 {
						ordered = append([]build.DensityLatitudeOption(nil), regions...)
						copy(ordered[1:i+1], ordered[:i])
						ordered[0] = region
						break
					}
				}
			}
		}
		for _, region := range ordered {
			if err := search.Checkpoint(ctx); err != nil {
				return err
			}
			key := fmt.Sprintf("%s/%s", region.Value, identity)
			value, cached := templates.Load(key)
			generateCtx := ctx
			if input.Plan.CenterRegionsOnly && !cached {
				// 初筛已经生成的合规模板可以在本轮重新选址。初筛超时与完整纬区选优各自缓存。
				if quick, found := templates.Load("relocate/" + key); found && quick.(moduleTemplate).err == nil {
					value, cached = quick, true
				}
				key = "center/" + key
				if !cached {
					value, cached = templates.Load(key)
				}
			}
			if relocatable && !cached {
				// 首轮采用独立模板原有的选优时窗；父任务随时可取消，模块之间继续检查暂停。
				// 其成败单独缓存，完整后备搜索依然拥有所有模板、纬区及可恢复的时窗。
				// 完整选区曾因缺电推迟的纬区，当前仍缺电时保留补电后备。其他缺少的形状仍可触发原星完整搜索。
				_, postponed := templates.Load(powerDeferredRegion{req.PlanetID, key})
				if postponed && powerFloor > 0 {
					deferredPower = true
				} else {
					deferredRegions = true
				}
				key = "relocate/" + key
				value, cached = templates.Load(key)
				generateCtx = search.WithControl(ctx, nil)
			}
			// 欠电的完整合同仍先尝试中纬中心。堆叠研究站在第四纬区有能保住同星上下游的真实落点。
			if !cached && input.Plan.CenterRegionsOnly && powerFloor == 0 && (region.Position != "center" || region.Tier >= 5) {
				deferredRegions = true
				continue
			}
			postpone := relocatable && (region.Position != "center" || region.Tier > quickCenterTiers) ||
				!relocatable && powerFloor > 0 && (region.Position != "center" || region.Tier >= 5)
			if postpone && !cached {
				if !relocatable {
					templates.Store(powerDeferredRegion{req.PlanetID, key}, struct{}{})
					deferredPower = true
				}
				continue
			}
			if !cached && relocatable && parallel > 1 && region.Value == regions[0].Value && misses != nil && misses.Load() >= 2 && !buildcore.CandidateBudgetLimited(ctx) {
				// 大厂的直连组合已经回退，优先并行准备接下来的独立赤道模板。真实占地仍逐块、按原顺序核验。
				prepareIndependentBatch(generateCtx, templates, input, parallel, region, index, placedRows, key, material)
				value, cached = templates.Load(key)
			}
			var template moduleTemplate
			if cached {
				template = value.(moduleTemplate)
			}
			if !cached {
				template, err = generateManufacturingTemplate(generateCtx, input.Options, material, independentWorkers, region, false)
				// 多个工作区共享内存预算。执行器拒绝派发时回到原并发核验，不能把资源限额记作纬区无解。
				var execution *blueprint.Error
				if independentWorkers > parallel && ctx.Err() == nil && errors.As(err, &execution) && execution.Code == "GEN_E_WORKFLOW" {
					template, err = generateManufacturingTemplate(generateCtx, input.Options, material, parallel, region, false)
				}
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					// 缓存完整选优的成功与失败，不让同规格后续模块重新执行已经失败的生成。
					templates.Store(key, moduleTemplate{err: err})
					last = err
					continue
				}
				templates.Store(key, template)
			}
			if template.err != nil {
				last = template.err
				continue
			}
			generated, document := template.generated, template.document
			if generated.SurfacePlan == nil || generated.LogisticsSummary == nil {
				return fmt.Errorf("制造模块缺少纬度或物流核验")
			}
			cursor := cursors[key]
			if cursor == nil {
				cursor = &moduleSiteCursor{}
				cursors[key] = cursor
			}
			recordSite := resumeModuleSite(templates, *req, input.Resources, template, index, cursor)
			objects, err := placeManufacturingTemplate(ctx, *req, document, generated.SurfacePlan.AnchorLatitudeGrid, region, fmt.Sprintf("制造模块 %d", index+1), cursor, occupied, catalog)
			if recordSite != nil && ctx.Err() == nil {
				recordSite(len(objects) > 0)
			}
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				last = err
			}
			if len(objects) == 0 {
				continue
			}
			attachManufacturing(req, objects, moduleBoundaryPorts(production, generated.LogisticsSummary.ExternalPorts), document.GameVersion, &occupied, catalog, &boundary)
			if hintKey.blockID != "" {
				hints.Store(hintKey, region.Value)
			}
			placed = true
			markPlaced(index)
			break
		}
		if !placed {
			message := fmt.Sprintf("第 %d 个制造模块在本轮选区与落点候选中未能完成放置", index+1)
			if last != nil {
				message += "：" + last.Error()
			}
			minimumMW := 0.
			if deferredPower && !deferredRegions {
				minimumMW = powerFloor
				message += fmt.Sprintf("。本合同至少需 %.3f MW，已规划 %.3f MW，补电或重新分工后再试其余纬区", powerFloor, input.Plan.Power.PowerMW+input.Plan.Power.ExternalGridMW)
			}
			failure := &industryplan.FactoryPlacementError{FirstUnplaced: index, DeferredRegions: deferredRegions || deferredPower, MinimumPowerMW: minimumMW, Err: &Error{PlanetID: req.PlanetID, Item: production.MainItemID, Code: "PLANET_FACTORY_SITE", Message: message}}
			if directGroups == 0 {
				return failure
			}
			req.Objects, req.GameVersion = req.Objects[:initialObjects], initialVersion
			err := appendManufacturingRows(ctx, req, input, actual, catalog, nil, reported)
			if independent, ok := err.(*industryplan.FactoryPlacementError); ok && independent.FirstUnplaced < index {
				return failure // 回传两个真实试摆中保留工序较多的前缀。
			}
			return err
		}
	}
	buildings := make([]blueprint.Building, len(req.Objects))
	for i := range req.Objects {
		buildings[i] = req.Objects[i].Building
	}
	ports, err := configureProductionFeeds(input.Plan, buildings, boundary, req.addFleetFeed, actual.Ports...)
	if err != nil {
		return err
	}
	req.localPorts = append(req.localPorts, boundary...)
	for i := range req.Objects {
		req.Objects[i].Building = buildings[i]
	}
	for _, port := range ports {
		addActualPort(actual, port)
	}
	if directGroups > 0 {
		actual.Warnings = append(actual.Warnings, fmt.Sprintf("本星有 %d 组制造工序用传送带直接衔接，共输送 %d 种中间品。物流运输机需求按实际塔槽统计。", directGroups, len(directItems)))
	}
	return nil
}

// independentTemplateIdentity 让按需生成与并行准备使用同一个额定合同、版本及设备设置。
func independentTemplateIdentity(material quant.Result, setting build.Setting, parallel int) (string, error) {
	b, err := json.Marshal(struct {
		Production  quant.Production
		Setting     build.Setting
		Dataset     string
		DataVersion string
		GameVersion string
		Concurrency int
	}{material.Productions[0], setting, material.Dataset, material.DataVersion, material.GameVersion, parallel})
	return string(b), err
}

// prepareIndependentBatch 在连续工区中准备至多三个不同的额定模板。当前块保留实际生成结果，后续块只保留成功，不消耗有限候选预算。
func prepareIndependentBatch(ctx context.Context, cache *sync.Map, input ProductionRequest, parallel int, region build.DensityLatitudeOption, index int, placed []bool, key string, material quant.Result) {
	limit := min(3, runtime.GOMAXPROCS(0)/parallel)
	if limit < 2 {
		return
	}
	type job struct {
		key  string
		plan quant.Result
	}
	jobs := []job{{key, material}}
	seen := map[string]bool{key: true}
	rows := input.Plan.Plan.Productions
	for i := index + 1; i < min(len(rows), index+9) && len(jobs) < limit; i++ {
		if placed[i] || ctx.Err() != nil {
			continue
		}
		plan, err := moduleMaterialPlan(ctx, input.Plan.Plan, rows[i], i)
		if err != nil {
			continue
		}
		id, err := independentTemplateIdentity(plan, input.Options.Setting, parallel)
		if err != nil {
			continue
		}
		regular := region.Value + "/" + id
		fast := "relocate/" + regular
		if seen[fast] {
			continue
		}
		seen[fast] = true
		if _, ok := cache.Load(regular); ok {
			continue
		}
		if _, ok := cache.Load(fast); ok {
			continue
		}
		jobs = append(jobs, job{fast, plan})
	}
	if len(jobs) < 2 {
		return
	}
	var workers sync.WaitGroup
	for _, j := range jobs {
		workers.Add(1)
		go func() {
			defer workers.Done()
			template, err := generateManufacturingTemplate(ctx, input.Options, j.plan, parallel, region, false)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				var failure *blueprint.Error
				if j.key != key || !errors.As(err, &failure) || failure.Code == "GEN_E_WORKFLOW" {
					return
				}
				template = moduleTemplate{err: err}
			}
			cache.Store(j.key, template)
		}()
	}
	workers.Wait()
}

// generateManufacturingTemplate 复用原版塔模式的路由与严格编解码，缓存只保存落位所需的数据。
func generateManufacturingTemplate(ctx context.Context, options ProductionOptions, material quant.Result, parallel int, region build.DensityLatitudeOption, keepAlternative bool) (moduleTemplate, error) {
	request := build.Request{Plan: material, Mode: 4, Setting: options.Setting, Name: options.Name, CandidateConcurrency: parallel, FirstVerifiedLayout: options.firstVerifiedLayout,
		SurfaceLayout: &build.SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: region.Value, ExpandAdjacent: true}}
	if !keepAlternative {
		return decodedModuleTemplate(build.Generate(ctx, request))
	}
	generated, verified, err := build.GenerateWithStationAlternative(ctx, request)
	template, err := decodedModuleTemplate(generated, err)
	if err == nil && verified != nil {
		var once sync.Once
		var other moduleTemplate
		template.alternative = func(ctx context.Context) moduleTemplate {
			once.Do(func() {
				var err error
				other, err = decodedModuleTemplate(buildcore.Encode(ctx, *verified))
				verified = nil
				if err != nil || !other.hasDirectRoutes(material.FlowGraph) {
					other = moduleTemplate{}
				}
			})
			return other
		}
	}
	return template, err
}

func decodedModuleTemplate(generated build.Result, err error) (moduleTemplate, error) {
	if err != nil {
		return moduleTemplate{}, err
	}
	document, err := blueprint.Decode(generated.Blueprint, true)
	if err != nil {
		return moduleTemplate{}, err
	}
	return moduleTemplate{generated: build.Result{SurfacePlan: generated.SurfacePlan, LogisticsSummary: generated.LogisticsSummary}, document: document}, nil
}

// attachManufacturing 在整个模块通过落位后提交真实建筑、引用和边界，用同一偏移保留内部传送带链。
func attachManufacturing(req *Request, objects []Object, ports []build.BoundaryPort, version string, occupied *obstacleIndex, catalog *placement.Catalog, boundary *[]build.BoundaryPort) {
	start := nextObjectIndex(req.Objects)
	for i := range objects {
		b := &objects[i].Building
		b.Index += start
		if b.InputObjIdx >= 0 {
			b.InputObjIdx += start
		}
		if b.OutputObjIdx >= 0 {
			b.OutputObjIdx += start
		}
	}
	req.Objects = append(req.Objects, objects...)
	for _, obstacle := range extendOccupancy(Request{Objects: objects}, catalog) {
		occupied.add(obstacle)
	}
	for _, port := range ports {
		port.BuildingIndex += int(start)
		*boundary = append(*boundary, port)
	}
	if req.GameVersion == "" {
		req.GameVersion = version
	}
}

// placeManufacturingTemplate 职责：在模板许可纬行内搜索真实落点；使用：制造模块装配；边界：平移后的球面线路须重新通过完整几何与编码自检，模板本身只读。
func placeManufacturingTemplate(ctx context.Context, req Request, document blueprint.Document, anchor int, region build.DensityLatitudeOption, module string, cursor *moduleSiteCursor, occupied obstacleIndex, catalog *placement.Catalog) ([]Object, error) {
	if cursor.certified {
		cursor.certified = false
		if err := search.Checkpoint(ctx); err != nil {
			return nil, err
		}
		objects, err := ProjectDocument(ctx, document, cursor.placedAt, module)
		if err == nil {
			cursor.x += moduleLongitudeStride
		}
		return objects, err
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	rows := moduleLatitudeRows(document, anchor, region, catalog)
	// 先查实际物流塔及分散在布局中的真实建筑。其任一冲突都能淘汰此位置，通过后仍逐栋复核。
	probe := document
	probe.Buildings = nil
	if len(document.Buildings) > 64 {
		for _, b := range document.Buildings {
			if b.ItemID == 2104 {
				probe.Buildings = append(probe.Buildings, b)
			}
		}
		for i := 0; i < len(document.Buildings); i += (len(document.Buildings) + 23) / 24 {
			if b := document.Buildings[i]; b.ItemID != 2104 {
				probe.Buildings = append(probe.Buildings, b)
			}
		}
		// 抽查只读占地与连接。正式落位的建筑仍单独持有其完整塔槽和设备参数。
		for i := range probe.Buildings {
			probe.Buildings[i].Parameters = nil
		}
	}
	local := req
	local.Foundation = ""
	for ; cursor.row < len(rows); cursor.row, cursor.x = cursor.row+1, 0 {
		row := rows[cursor.row]
		latitude := float64(float32(row) * float32(latitudeStep))
		step, err := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
		if err != nil {
			return nil, err
		}
		width := int(math.Round(2 * math.Pi / float64(step)))
		var sample, previous []Object
		// 只在该格已判定后推进；投影或校验中途超时时，直连重核仍从这个格开始。
		for ; cursor.x < width; cursor.x += moduleLongitudeStride {
			if err := search.Checkpoint(ctx); err != nil {
				return nil, err
			}
			x := cursor.x
			position := Position{Latitude: latitude, Longitude: float64(x) * float64(step)}
			if len(probe.Buildings) > 0 {
				sample, err = projectModuleAtLongitude(ctx, probe, position, step, module, sample)
				if err == nil && (placementConflict(sample, occupied, catalog) || req.terrainConflict(sample, catalog)) {
					continue
				}
			}
			objects, err := projectModuleAtLongitude(ctx, document, position, step, module, previous)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				cursor.last = err
				break
			}
			previous = objects
			if placementConflict(objects, occupied, catalog) || req.terrainConflict(objects, catalog) {
				continue
			}
			if row != anchor {
				if local.placementIndices == nil {
					local.Obstacles = extendOccupancy(req, catalog)
					local.placementIndices, err = indexPlacement(local)
					if err != nil {
						return nil, err
					}
				}
				local.Objects = objects
				// Build 复核球面几何，内部 Encode 已严格解码并逐字节比对，无需重复解码。
				if _, err := Build(ctx, local); err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					cursor.last = err
					continue
				}
			}
			if err := search.Checkpoint(ctx); err != nil {
				return nil, err
			}
			cursor.x += moduleLongitudeStride
			cursor.placedAt = position
			return objects, nil
		}
	}
	cursor.exhausted = true
	return nil, cursor.last
}

// projectModuleAtLongitude 按原版单精度次序在当前纬行重算两端经度，多 Area 按各自父链重新投影。
func projectModuleAtLongitude(ctx context.Context, document blueprint.Document, position Position, step geometry.Radians, module string, row []Object) ([]Object, error) {
	if row == nil || len(document.Areas) != 1 {
		return ProjectDocument(ctx, document, position, module)
	}
	longitude := float32(position.Longitude)
	for i := range document.Buildings {
		b := &document.Buildings[i]
		row[i].First.Longitude = float64(longitude + b.LocalOffset.X*float32(step))
		row[i].Second.Longitude = float64(longitude + b.LocalOffset2.X*float32(step))
	}
	return row, nil
}

// moduleLatitudeRows 职责：补足同原生纬带内的整数平移；使用：原锚点优先，其余按距离搜索；边界：仅单 Area，全部双端点及真实水平占地包络不得越过该区域或经度段数变化边界。
func moduleLatitudeRows(document blueprint.Document, anchor int, region build.DensityLatitudeOption, catalog *placement.Catalog) []int {
	rows := []int{anchor}
	if len(document.Areas) != 1 {
		return rows
	}
	segments := int(document.Areas[0].AreaSegments)
	if actual, err := geometry.LongitudeSegmentsAtGrid(anchor, geometry.Vanilla200); err != nil || actual != segments {
		return rows
	}
	low, high := anchor, anchor
	for low > -249 {
		if n, _ := geometry.LongitudeSegmentsAtGrid(low-1, geometry.Vanilla200); n != segments {
			break
		}
		low--
	}
	for high < 249 {
		if n, _ := geometry.LongitudeSegmentsAtGrid(high+1, geometry.Vanilla200); n != segments {
			break
		}
		high++
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, building := range document.Buildings {
		margin := math.Asin(groundRadius(catalog, building)/200) / float64(latitudeStep)
		for _, point := range []blueprint.Point{building.LocalOffset, building.LocalOffset2} {
			minY = min(minY, float64(point.Y)-margin)
			maxY = max(maxY, float64(point.Y)+margin)
		}
	}
	lo := max(float64(low), region.LatitudeLow*math.Pi/180/float64(latitudeStep))
	hi := min(float64(high), region.LatitudeHigh*math.Pi/180/float64(latitudeStep))
	first, last := int(math.Ceil(lo-minY)), int(math.Floor(hi-maxY))
	for row := first; row <= last; row++ {
		if row == anchor {
			continue
		}
		if n, _ := geometry.LongitudeSegmentsAtGrid(row, geometry.Vanilla200); n == segments {
			rows = append(rows, row)
		}
	}
	extra := rows[1:]
	sort.SliceStable(extra, func(i, j int) bool { return math.Abs(float64(extra[i]-anchor)) < math.Abs(float64(extra[j]-anchor)) })
	return rows
}
