package industryworkspace

// 文件职责：把固定存档与量化求解、逐星几何、最终产能复核连接起来，不在前端重算。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sort"

	"DysonSphereBox/helper/blueprint/build"
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
	remoteGroup := snapshot.FreeRemoteGroup()
	sites := make(map[int64]Site, len(selection.Sites))
	for _, s := range selection.Sites {
		sites[s.ID] = s
	}
	validator := func(ctx context.Context, p industryplan.PlanetPlan) (industryplan.PlanetActual, error) {
		if err := ctx.Err(); err != nil {
			return industryplan.PlanetActual{}, err
		}
		detail, err := snapshot.PlanetResourceDetailContext(ctx, p.PlanetID)
		if err != nil {
			return industryplan.PlanetActual{}, err
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
		built, err := planet.Generate(ctx, planet.ProductionRequest{Plan: p, Resources: detail, Options: options})
		if err != nil {
			var failure *planet.Error
			if errors.As(err, &failure) && failure.Code == "PLANET_MINING_SITE" {
				return industryplan.PlanetActual{}, &industryplan.SourcePlacementError{
					ItemID: failure.Item, Err: err, AvailableRatePerMinute: failure.AvailableRatePerMinute,
					Description: fmt.Sprintf("在%s采集%s时，%s", sites[p.PlanetID].Name, dataset.ByID[failure.Item].Name, failure.Message),
				}
			}
			return industryplan.PlanetActual{}, err
		}
		if built.StationCount < p.StationCount {
			return industryplan.PlanetActual{}, fmt.Errorf("星球 %d 需要 %d 座星际物流站，实际布置 %d 座", p.PlanetID, p.StationCount, built.StationCount)
		}
		if len(p.OutboundFleet) > 0 && len(built.Actual.Fleet) == 0 {
			return industryplan.PlanetActual{}, fmt.Errorf("星球 %d 的实际蓝图缺少出口货物的逐塔配船记录", p.PlanetID)
		}
		actual := built.Actual
		if n := len(detail.RelayZones); n > 0 {
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
