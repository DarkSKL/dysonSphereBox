package planet

// 文件职责：把同星供需相连的工序装配为带路直连的制造块；使用：整星制造优先尝试；边界：组合未通过时交回逐块装配。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// directModules 按星球现有规范物料边记录相邻配方和真实每分钟流量，供布局挑选直连组合。
type directModules []map[int]float64

const directModuleMachines = 4 * 32

type directSiteSearchKey struct{}

// directSiteSearch 记录大型星球同一任务中连续失败的新直连形状。命中已有模板仍能尝试落位，真实成功后再开放新组合。
func directSiteSearch(ctx context.Context, input ProductionRequest) *atomic.Int32 {
	if input.Plan.Plan.MachineCount <= directModuleMachines {
		return nil
	}
	count := &atomic.Int32{}
	if sites, ok := ctx.Value(directSiteSearchKey{}).(*sync.Map); ok {
		value, _ := sites.LoadOrStore(input.Plan.PlanetID, count)
		return value.(*atomic.Int32)
	}
	return count
}

func newDirectModules(ctx context.Context, plan quant.Result) directModules {
	if len(plan.Productions) < 2 {
		return nil
	}
	graph := plan.FlowGraph
	if graph == nil {
		rebuilt, err := quant.BuildFlowGraphContext(ctx, plan)
		if err != nil {
			return nil
		}
		graph = &rebuilt
	}
	indices := make(map[string]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.Kind == "production" && node.ProductionIndex >= 0 && node.ProductionIndex < len(plan.Productions) {
			indices[node.ID] = node.ProductionIndex
		}
	}
	adjacent := make(directModules, len(plan.Productions))
	for _, edge := range graph.Edges {
		a, from := indices[edge.SourceNodeID]
		b, to := indices[edge.TargetNodeID]
		if edge.FlowKind != "internal" || !from || !to || a == b {
			continue
		}
		if adjacent[a] == nil {
			adjacent[a] = map[int]float64{}
		}
		if adjacent[b] == nil {
			adjacent[b] = map[int]float64{}
		}
		adjacent[a][b] += edge.RatePerMinute
		adjacent[b][a] += edge.RatePerMinute
	}
	return adjacent
}

// groups 从当前实例沿最高流量边扩展，最多装入四个常规 32 台制造块的设备量。大组合无落点时仍尝试最紧密的一对。
func (adjacent directModules) groups(first int, placed []bool, rows []quant.Production) [][]int {
	if first >= len(adjacent) || len(adjacent[first]) == 0 {
		return nil
	}
	selected := map[int]bool{first: true}
	frontier := map[int]float64{}
	group := []int{first}
	machines := rows[first].MachineCount
	for {
		for neighbor, rate := range adjacent[group[len(group)-1]] {
			if !placed[neighbor] && !selected[neighbor] {
				frontier[neighbor] += rate
			}
		}
		best := -1
		for neighbor, rate := range frontier {
			if machines+rows[neighbor].MachineCount <= directModuleMachines && (best < 0 || rate > frontier[best] || rate == frontier[best] && neighbor < best) {
				best = neighbor
			}
		}
		if best < 0 {
			break
		}
		delete(frontier, best)
		group = append(group, best)
		selected[best] = true
		machines += rows[best].MachineCount
	}
	if len(group) == 1 {
		return nil
	}
	groups := [][]int{group}
	if len(group) > 2 {
		groups = append(groups, append([]int(nil), group[:2]...))
	}
	for _, members := range groups {
		sort.Ints(members)
	}
	return groups
}

// directTemplateInput 使用完整生产行和选优设置标识一个直连组合，纯模板与星面占地分别核验。
func directTemplateInput(ctx context.Context, input ProductionRequest, group []int, parallel int, region build.DensityLatitudeOption) (quant.Result, string, error) {
	rows := make([]quant.Production, len(group))
	for i, member := range group {
		rows[i] = input.Plan.Plan.Productions[member]
	}
	material, err := manufacturingMaterial(ctx, input.Plan.Plan, rows, group[0])
	if err != nil {
		return quant.Result{}, "", ctx.Err()
	}
	identity, err := json.Marshal(struct {
		Rows                                      []quant.Production
		Setting                                   build.Setting
		Dataset, DataVersion, GameVersion, Region string
		Parallel                                  int
	}{rows, input.Options.Setting, material.Dataset, material.DataVersion, material.GameVersion, region.Value, parallel})
	if err != nil {
		return quant.Result{}, "", err
	}
	key := "direct/" + string(identity)
	if input.Options.firstVerifiedLayout {
		key = "relocate/" + key
	} else if input.Plan.SinglePlanet {
		key = "single/" + key
	}
	return material, key, nil
}

func generateDirectTemplate(ctx context.Context, input ProductionRequest, material quant.Result, parallel int, region build.DensityLatitudeOption) moduleTemplate {
	buildCtx, cancel := context.WithTimeout(search.WithControl(ctx, nil), 12*time.Second)
	defer cancel()
	template, err := generateManufacturingTemplate(buildCtx, input.Options, material, parallel, region, input.Plan.SinglePlanet && !input.Options.firstVerifiedLayout)
	if err == nil && !template.hasDirectRoutes(material.FlowGraph) {
		err = fmt.Errorf("组合缺少已核验的内部传送带或边界塔口")
	}
	if err != nil {
		return moduleTemplate{err: err}
	}
	return template
}

// directTemplateAhead 只在本次整星试摆中提前计算一个后续组合。主循环选到完全相同的组合且有生成机会时才使用成功结果。
type directTemplateAhead struct {
	ctx      context.Context
	key      string
	first    int
	done     chan struct{}
	template moduleTemplate
}

func (a *directTemplateAhead) take(key string) (moduleTemplate, bool) {
	if a == nil || a.done == nil || key != a.key {
		return moduleTemplate{}, false
	}
	<-a.done
	template := a.template
	a.done, a.template = nil, moduleTemplate{}
	return template, template.err == nil
}

// prepare 遇到已能直连的大工区后，让后一个候选与下一个工区的主线程计算重叠。前一组合的落位不会由预测决定。
func (a *directTemplateAhead) prepare(input ProductionRequest, direct directModules, placed []bool, start, parallel int, region build.DensityLatitudeOption, cache *sync.Map) {
	if a.done != nil {
		if start <= a.first {
			return
		}
		select {
		case <-a.done:
			a.done, a.template = nil, moduleTemplate{}
		default:
			return
		}
	}
	predicted := slices.Clone(placed)
	for i, groupsFound := start, 0; i < len(predicted); i++ {
		if predicted[i] {
			continue
		}
		groups := direct.groups(i, predicted, input.Plan.Plan.Productions)
		if len(groups) == 0 {
			continue
		}
		group := groups[0]
		for _, member := range group {
			predicted[member] = true
		}
		groupsFound++
		if groupsFound < 2 {
			continue
		}
		material, key, err := directTemplateInput(a.ctx, input, group, parallel, region)
		if err != nil || key == "" {
			return
		}
		if _, cached := cache.Load(key); cached {
			continue
		}
		a.key, a.first, a.done = key, group[0], make(chan struct{})
		go func() {
			defer close(a.done)
			a.template = generateDirectTemplate(a.ctx, input, material, parallel, region)
		}()
		return
	}
}

// placeDirectModules 用与独立块相同的塔生成器和球面投影核验组合；一次机会限时，组合不可用不消耗任何落点、产能或端口。
func placeDirectModules(ctx context.Context, req Request, input ProductionRequest, group []int, parallel int, region build.DensityLatitudeOption, occupied obstacleIndex, catalog *placement.Catalog, templates *sync.Map, cursors map[string]*moduleSiteCursor, misses *atomic.Int32, ahead *directTemplateAhead) ([]Object, moduleTemplate, error) {
	material, key, err := directTemplateInput(ctx, input, group, parallel, region)
	if err != nil || key == "" {
		return nil, moduleTemplate{}, err
	}
	var template moduleTemplate
	cached, ok := templates.Load(key)
	if ok {
		template = cached.(moduleTemplate)
	} else {
		// 大厂的高流量组合及双工序后备均未成功时，先把时间留给独立块和其它纬带。
		if misses != nil && misses.Load() >= 2 {
			return nil, moduleTemplate{}, nil
		}
		var prepared bool
		template, prepared = ahead.take(key)
		if !prepared {
			template = generateDirectTemplate(ctx, input, material, parallel, region)
		}
		if ctx.Err() != nil {
			return nil, moduleTemplate{}, ctx.Err()
		}
		templates.Store(key, template)
	}
	if template.err != nil {
		if !ok && misses != nil {
			misses.Add(1)
		}
		return nil, moduleTemplate{}, nil
	}
	objects, template := placeDirectShapes(ctx, template, func(shape moduleTemplate, alternate bool) ([]Object, bool) {
		siteKey := key
		if alternate {
			siteKey = "alternate/" + key
		}
		cursor := cursors[siteKey]
		if cursor == nil {
			cursor = &moduleSiteCursor{}
			cursors[siteKey] = cursor
		}
		recordPrefix := resumeModuleSite(templates, req, input.Resources, shape, group[0], cursor)
		placeCtx, cancel := context.WithTimeout(search.WithControl(ctx, nil), 3*time.Second)
		defer cancel()
		objects, _ := placeManufacturingTemplate(placeCtx, req, shape.document, shape.generated.SurfacePlan.AnchorLatitudeGrid, region, fmt.Sprintf("直连制造模块 %d", group[0]+1), cursor, occupied, catalog)
		if recordPrefix != nil && ctx.Err() == nil {
			recordPrefix(len(objects) > 0)
		}
		return objects, cursor.exhausted && placeCtx.Err() == nil
	})
	if misses != nil {
		if len(objects) > 0 {
			misses.Store(0)
		} else if !ok && ctx.Err() == nil {
			misses.Add(1)
		}
	}
	return objects, template, ctx.Err()
}

// hasDirectRoutes 核对规范内部边的全部份额和实体索引，以及外部端口对应的库存。塔间运输候选没有直连证据。
func (template moduleTemplate) hasDirectRoutes(graph *quant.FlowGraph) bool {
	summary := template.generated.LogisticsSummary
	if summary == nil || template.generated.SurfacePlan == nil {
		return false
	}
	buildings := template.document.Buildings
	for _, port := range summary.ExternalPorts {
		if port.BuildingIndex < 0 || port.BuildingIndex >= len(buildings) {
			return false
		}
		b := &buildings[port.BuildingIndex]
		if b.ItemID != 2104 || len(b.Parameters) != 2048 || stationSlot(b, port.ItemID) < 0 {
			return false
		}
	}
	rates := map[string]float64{}
	for _, lane := range summary.Lanes {
		if lane.FlowKind != "internal" {
			continue
		}
		if !lane.InternalConnected || lane.RatePerMinute <= 0 || lane.RatePerMinute > lane.CapacityPerMinute+quant.FlowTolerance(lane.RatePerMinute, lane.CapacityPerMinute) {
			return false
		}
		belt := false
		for _, index := range lane.BuildingIndices {
			if index < 0 || index >= len(buildings) || buildings[index].ItemID == 2104 || buildings[index].ItemID == 2103 {
				return false
			}
			id := buildings[index].ItemID
			belt = belt || id >= 2001 && id <= 2003
		}
		if !belt {
			return false
		}
		rates[lane.EdgeID] += lane.RatePerMinute
	}
	for _, edge := range graph.Edges {
		if edge.FlowKind == "internal" && math.Abs(rates[edge.ID]-edge.RatePerMinute) > quant.FlowTolerance(rates[edge.ID], edge.RatePerMinute) {
			return false
		}
	}
	return len(rates) != 0
}
