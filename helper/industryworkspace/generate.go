package industryworkspace

// 文件职责：把固定存档与量化求解、逐星几何、最终产能复核连接起来，不在前端重算。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"runtime"
	"sort"
	"sync"

	"DysonSphereBox/helper/blueprint/build"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/planet"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// verifiedConstruction 将可粘贴的施工次序与对应星球的核验结果一并保留。
type verifiedConstruction struct {
	layout planet.ProductionResult
	parts  []ConstructionPart
}

// sourceSiteKey 标识采集预检的实体输入；同一任务内可复用选址结论，发船量、库存和工厂带口仍由整星实装核验。
func sourceSiteKey(p industryplan.PlanetPlan) (string, error) {
	type miningSite struct {
		Item, Facility       int
		Rate, Speed, Setting float64
		Export               bool
	}
	type sourceSites struct {
		Planet                              int64
		Mines                               []miningSite
		Receivers                           int
		HasReceivers                        bool
		Lenses, ImportLenses, ExportPhotons bool
	}
	contract := sourceSites{Planet: p.PlanetID}
	exports := map[int]bool{}
	for _, port := range p.Ports {
		if port.Kind == "export" {
			exports[port.ItemID] = true
		}
	}
	for _, mine := range p.Mines {
		contract.Mines = append(contract.Mines, miningSite{mine.ItemID, mine.FacilityID, mine.RatePerMinute, mine.SpeedMultiplier, mine.FacilityMultiplier, exports[mine.ItemID]})
	}
	if receivers := p.Photons; receivers != nil {
		contract.HasReceivers = true
		contract.Receivers, contract.Lenses = receivers.Count, receivers.LensRatePerMinute > 0
		contract.ExportPhotons = exports[1208]
		for _, port := range p.Ports {
			if contract.Lenses && port.Kind == "import" && port.ItemID == 1209 {
				contract.ImportLenses = true
			}
		}
	}
	key, err := json.Marshal(contract)
	return string(key), err
}

// Generate 编排一次完整计算。布局回传的实物证据和蓝图由同一候选携带，只发布最终获选版本。
func Generate(ctx context.Context, req Request, selection Selection, world industryplan.World, snapshot *saveanalysis.Snapshot, progress func(industryplan.Progress)) (*Bundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tech := snapshot.PlanningTechnology()
	changes, err := applyTechnology(&req, tech)
	if err != nil {
		return nil, err
	}
	req.Planning.ConstructionInserterLevel = req.Options.InserterLevel
	req.Planning.ConstructionBeltLevel = req.Options.BeltLevel
	if tech != nil {
		req.Planning.ConstructionDroneCargo = tech.DroneCargo
	}
	if req.Options.TechnologyMode == "save" {
		req.Planning.Quant = req.Planning.Quant.WithUnlockedRecipes(snapshot.RecipeUnlocked)
	}
	if tech != nil && tech.BlueprintLimit <= 0 {
		return nil, fmt.Errorf("这个存档尚未解锁建筑蓝图，请解锁蓝图科技后重新读取存档")
	}
	ctx, metrics := industryplan.WithLayoutMetrics(ctx)
	dataset, err := quant.LoadVanillaDataset(req.Planning.Quant.Dataset)
	if err != nil {
		return nil, err
	}
	// 实际候选总数只做诊断计数；任务时间窗在可恢复检查点暂停，不能由前一颗星耗尽后续星的机会。
	limit := 0
	ctx, geometryBudget := planet.WithCandidateBudget(ctx, limit)
	ctx = planet.WithTemplateCache(ctx)
	if req.Planning.Profile != "low" && !buildcore.CandidateBudgetLimited(ctx) {
		world.LayoutWorkers = min(4, runtime.GOMAXPROCS(0)/4)
	}
	remoteGroup := snapshot.FreeRemoteGroup()
	sites := make(map[int64]Site, len(selection.Sites))
	for _, s := range selection.Sites {
		sites[s.ID] = s
	}
	planetInput := func(ctx context.Context, p industryplan.PlanetPlan) (planet.ProductionRequest, error) {
		if err := ctx.Err(); err != nil {
			return planet.ProductionRequest{}, err
		}
		detail, err := snapshot.PlanetResourceDetailContext(ctx, p.PlanetID)
		if err != nil {
			return planet.ProductionRequest{}, err
		}
		options := planet.ProductionOptions{
			RemoteGroup: remoteGroup,
			Name:        sites[p.PlanetID].Name, GameVersion: selection.GameVersion,
			Foundation: req.Options.Foundation, Fuel: req.Options.Fuel, Profile: req.Planning.Profile,
			OrbitalCollectors: req.Planning.EnableOrbitalCollectors != nil && *req.Planning.EnableOrbitalCollectors,
			Setting: build.Setting{
				NeedPower: &req.Options.Power, EnableIntermediateBuffer: req.Options.Buffer,
				BeltLv: req.Options.BeltLevel, InserterLv: req.Options.InserterLevel,
			},
		}
		if p.ManualRoutes {
			options.RemoteGroup = 0
		}
		if tech != nil {
			options.DroneCargo, options.DroneSpeed = tech.DroneCargo, tech.DroneSpeed
		}
		return planet.ProductionRequest{Plan: p, Resources: detail, Options: options}, nil
	}
	sourceError := func(p industryplan.PlanetPlan, err error) error {
		var failure *planet.Error
		if errors.As(err, &failure) && failure.Code == "PLANET_MINING_SITE" {
			return &industryplan.SourcePlacementError{
				ItemID: failure.Item, Err: err, AvailableRatePerMinute: failure.AvailableRatePerMinute,
				Description: fmt.Sprintf("在%s采集%s时，%s", sites[p.PlanetID].Name, dataset.ByID[failure.Item].Name, failure.Message),
			}
		}
		return err
	}
	// 矿机与接收器按实体输入记住成功及确定的缺口。调整外运目的地后仍重新计算真实货流、仓储和供电。
	type sourceReceipt struct {
		load planet.SourceLoadMW
		err  error
	}
	var checkedSources sync.Map
	world.CheckSourceSites = func(ctx context.Context, p industryplan.PlanetPlan) error {
		if len(p.Mines) == 0 && p.Photons == nil {
			return planet.SourceLoadMW(0).CheckManufacturingPower(p, req.Options.InserterLevel)
		}
		key, err := sourceSiteKey(p)
		if err != nil {
			return err
		}
		if previous, ok := checkedSources.Load(key); ok {
			receipt := previous.(sourceReceipt)
			if receipt.err != nil {
				return receipt.err
			}
			return receipt.load.CheckManufacturingPower(p, req.Options.InserterLevel)
		}
		input, err := planetInput(ctx, p)
		if err != nil {
			return err
		}
		load, err := planet.SurveySourceLoad(ctx, input)
		err = sourceError(p, err)
		var placement *industryplan.SourcePlacementError
		if ctx.Err() == nil && (err == nil || errors.As(err, &placement)) {
			checkedSources.Store(key, sourceReceipt{load, err})
		}
		if err != nil {
			return err
		}
		return load.CheckManufacturingPower(p, req.Options.InserterLevel)
	}
	validator := func(ctx context.Context, p industryplan.PlanetPlan) (industryplan.PlanetActual, error) {
		input, err := planetInput(ctx, p)
		if err != nil {
			return industryplan.PlanetActual{}, err
		}
		built, err := planet.Generate(ctx, input)
		if err != nil {
			return industryplan.PlanetActual{}, sourceError(p, err)
		}
		if built.StationCount < p.StationCount {
			return industryplan.PlanetActual{}, fmt.Errorf("星球 %d 需要 %d 座星际物流站，实际布置 %d 座", p.PlanetID, p.StationCount, built.StationCount)
		}
		if len(p.OutboundFleet) > 0 && len(built.Actual.Fleet) == 0 {
			return industryplan.PlanetActual{}, fmt.Errorf("星球 %d 的实际蓝图缺少出口货物的逐塔配船记录", p.PlanetID)
		}
		actual := built.Actual
		if n := len(input.Resources.RelayZones); n > 0 {
			actual.Warnings = append(actual.Warnings, fmt.Sprintf("已绕开存档中的 %d 处黑雾基地或其落地区。若施工前基地位置发生变化，请读取新存档重算。", n))
		}
		parts := []ConstructionPart{{Code: built.Blueprint, Buildings: built.BuildingCount, FoundationCells: built.FoundationCells, Kind: "whole"}}
		if tech != nil && (built.BuildingCount > tech.BlueprintLimit || built.FoundationCells > tech.FoundationLimit) {
			parts, err = splitConstructionBlueprintAt(ctx, built.Blueprint, tech.BlueprintLimit, tech.FoundationLimit, &built.Anchor.Latitude)
			if err != nil {
				return industryplan.PlanetActual{}, fmt.Errorf("%s的施工蓝图超过存档科技限制：%w", sites[p.PlanetID].Name, err)
			}
		}
		actual.Artifact = &verifiedConstruction{layout: built, parts: parts}
		return actual, nil
	}
	plan, err := industryplan.Plan(ctx, req.Planning, world, validator, progress)
	if err != nil {
		return nil, err
	}
	report := Report{
		SaveName: selection.SaveName, Technology: tech, TechnologyChanges: changes, LayoutMetrics: *metrics,
		AchievedTargets: plan.AchievedTargets, ExtraTargets: plan.ExtraTargets, ExpansionNote: plan.ExpansionNote,
		SnapshotID: selection.SnapshotID, Tick: selection.Tick, GameVersion: selection.GameVersion,
		Status: plan.Status, CenterPlanetID: plan.CenterPlanetID, StoragePlanetID: plan.StoragePlanetID,
		LimitStage: plan.LimitStage, ResourceGaps: plan.ResourceGaps,
		Targets: req.Planning.Quant.Targets, Profile: req.Planning.Profile, Preference: plan.Preference, ScoreBasis: plan.ScoreBasis,
		Candidates: plan.Candidates, Repairs: plan.Repairs, LayoutCandidates: geometryBudget.Used(), ResourcesInspected: plan.ResourcesInspected,
		Routes: plan.Routes, External: plan.External, Warnings: plan.Warnings, Planets: []PlanetSheet{},
	}
	report.ExportName = productionName(dataset, report.Targets, report.AchievedTargets)
	if len(changes) > 0 && report.Status == "ready" {
		report.Status = "conditional"
	}
	if report.Profile == "" {
		report.Profile = "standard"
	}
	b := &Bundle{Request: req, Plan: plan, Report: report, Codes: map[int64]string{}}
	manualRoutes := len(plan.Planets) > 0 && plan.Planets[0].ManualRoutes
	for _, route := range b.Report.Routes {
		if route.VesselLoad <= 0 {
			continue
		}
		if remoteGroup != 0 && !manualRoutes {
			b.Report.RemoteGroup = bits.TrailingZeros64(remoteGroup) + 1
		} else {
			reason := "没有可确认的空闲物流站组。"
			if manualRoutes {
				reason = "同一物品有多个发送和接收地点，共组会允许计划外配对。"
			}
			b.Report.Warnings = append(b.Report.Warnings, reason+"新塔已设为指定航线，请在粘贴后按运输清单建立站对站航线。")
			if b.Report.Status == "ready" {
				b.Report.Status = "conditional"
			}
		}
		break
	}
	for index, p := range plan.Planets {
		site := sites[p.PlanetID]
		sheet := PlanetSheet{
			OutboundVesselMW: p.OutboundVesselMW,
			Modules:          len(p.Plan.Productions), Photons: p.Photons,
			ID: p.PlanetID, Name: site.Name, Star: site.Star, Stations: p.StationCount, Vessels: p.VesselCount,
			Mines: append([]industryplan.Mine(nil), p.Mines...), PowerMW: p.Plan.PowerMW, Ports: p.Ports,
		}
		if actual, ok := plan.Actuals[p.PlanetID]; ok {
			verified, ok := actual.Artifact.(*verifiedConstruction)
			if !ok || verified == nil || len(verified.parts) == 0 {
				return nil, fmt.Errorf("星球 %d 的已核验布局缺少蓝图", p.PlanetID)
			}
			result := &verified.layout
			sheet.Longitude, sheet.Latitude, sheet.Direction = result.Anchor.Longitude*180/math.Pi, result.Anchor.Latitude*180/math.Pi, result.Direction
			sheet.Buildings, sheet.Areas, sheet.FoundationCells = result.BuildingCount, result.AreaCount, result.FoundationCells
			sheet.SoilEstimateKnown, sheet.SoilLowerBound, sheet.SoilUpperBound = result.SoilEstimateKnown, result.SoilLowerBound, result.SoilUpperBound
			sheet.Warnings, sheet.Ports = actual.Warnings, actual.Ports
			sheet.Stations, sheet.Power = result.StationCount, actual.Power
			if result.RequiredGridMW > 0 {
				b.recordRequiredGrid(p.PlanetID, result.RequiredGridMW)
			}
			sheet.Fleet = actual.Fleet
			sheet.Materials = sortedCounts(result.ItemCounts)
			sheet.ExistingSlots = existingSlots(snapshot, p.PlanetID, result.LocalModes, p.Ports)
			if len(sheet.ExistingSlots) > 0 && b.Report.Status == "ready" {
				b.Report.Status = "conditional"
			}
			sheet.StationStorage = sortedCounts(result.StationStorage)
			for _, item := range result.LocalDroneItems {
				b.Report.External = append(b.Report.External, industryplan.External{PlanetID: p.PlanetID, ItemID: item, Kind: "local-drones"})
				if b.Report.Status == "ready" {
					b.Report.Status = "conditional"
				}
			}
			installed := map[int]int{}
			coveredItems := map[int]int64{}
			for _, mine := range result.Mining {
				sheet.MinePlacements = append(sheet.MinePlacements, MinePlacement{
					ItemID: mine.ItemID, FacilityID: mine.FacilityID, BuildingIndex: mine.BuildingIndex, VeinIDs: mine.VeinIDs, CoveredItems: mine.CoveredItems,
					Longitude: mine.Position.Longitude * 180 / math.Pi, Latitude: mine.Position.Latitude * 180 / math.Pi, RatePerMinute: mine.RatePerMinute,
				})
				installed[mine.ItemID]++
				if mine.CoveredItems != nil {
					coveredItems[mine.ItemID] += mine.NewCoveredItems
				}
			}
			for i, mine := range sheet.Mines {
				if n, ok := installed[mine.ItemID]; ok {
					sheet.Mines[i].Count = n
				}
				if count, ok := coveredItems[mine.ItemID]; ok && mine.RatePerMinute > 0 {
					minutes := float64(count) / mine.RatePerMinute
					sheet.Mines[i].CoveredItems, sheet.Mines[i].AvailableMinutes = &count, &minutes
				}
			}
			b.Plan.Planets[index].Mines = sheet.Mines
			if err := b.addSheets(ctx, &sheet, verified.parts, len(plan.Planets) > 1); err != nil {
				return nil, err
			}
		}
		b.Report.Planets = append(b.Report.Planets, sheet)
	}
	b.Plan.External, b.Plan.Status = b.Report.External, b.Report.Status
	if len(b.Report.Warnings) > 0 {
		RecordOperation("diagnostics", "已保存规划诊断", map[string]interface{}{"warnings": b.Report.Warnings})
	}
	b.Report.Warnings, b.Report.Issues = playerMessages(b.Report.Warnings, sites, dataset)
	for i := range b.Report.Planets {
		clean, issues := playerMessages(b.Report.Planets[i].Warnings, sites, dataset)
		b.Report.Planets[i].Warnings = clean
		b.Report.Issues = append(b.Report.Issues, issues...)
	}
	b.Report.Exportable = plan.Verified && (b.Report.Status == "ready" || b.Report.Status == "conditional") && len(b.Codes) > 0
	for _, site := range b.Report.Planets {
		b.Report.Exportable = b.Report.Exportable && len(site.Sheets) > 0
	}
	b.Plan.Actuals = nil
	return b, nil
}

// sortedCounts 保留可重现的施工材料顺序，供清单与 ZIP 共用。
func sortedCounts[K ~int | ~int16](items map[K]int) []ItemCount {
	result := make([]ItemCount, 0, len(items))
	for id, n := range items {
		result = append(result, ItemCount{ItemID: int(id), Count: n})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ItemID < result[j].ItemID })
	return result
}
