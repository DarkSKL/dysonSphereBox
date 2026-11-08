package build

// 文件职责：把跨产线物料端口按真实物品槽、供需角色和十二带口约束归入共享物流塔。
// 使用：密铺塔全场装配；边界：不跨角色混用同物品槽，不改机器数、净流或自产供剂毛流。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// denseStationRow 职责：保存全场排布的局部功能实体和塔端口；使用：机器装配至统一路由；边界：外包矩形不作为独占占位。
type denseStationRow struct {
	// line 职责：保存真实局部物料拓扑；使用：机器与供剂能力证明；边界：不拥有独立地块。
	line *line
	// entities 职责：保存本行必要实体；使用：保持相对姿态；边界：不含全局后补带。
	entities []*placed
	// routes 职责：索引本行全部塔接线需求；使用：库存分配和朝向代价；边界：引用候选内路线表。
	routes []int
	// turns 职责：记录四向旋转；使用：端口净空解释；边界：不改变槽位编号。
	turns int
	// unit 职责：保留街道组之间已经验证的相对位置；使用：整体真实轮廓落位；边界：普通行为空，不把每个容量组再次独立移动。
	unit *denseStationUnit
}

// denseStationUnit 职责：保存存在必要相对约束的机器集合；使用：塔间真实空隙搜索；边界：集合矩形不独占，只登记实际轮廓与连续端口。
type denseStationUnit struct {
	entities []*placed
	rows     []*denseStationRow
	ports    []denseStreetPort
	// accessories 职责：保留可独立落位的真实附件；使用：全场轮廓搜索；边界：只在核心单元登记一次。
	accessories []*denseStationUnit
	// assembly/links 职责：关联共享附件契约及真实待接线端点；使用：放置代价和最终统一提交；边界：不把内部连接写入规范塔流量账。
	assembly *denseStreetAssembly
	links    []*denseStreetAccessoryLink
}

// denseStationTower 职责：保存一个跨产线共享的物品角色库存；使用：塔数量、实际带口和全场位置规划；边界：不允许同物品供给/需求在同一槽冲突。
type denseStationTower struct {
	// items 职责：保存物品和供需角色；使用：真实塔参数；边界：同物品不重复。
	items []stationItem
	// routes 职责：保存归入此塔的实际路线；使用：十二口预算；边界：副产物出带另计。
	routes []int
	// leftovers 职责：预留规范剩余流的物理出口数；使用：塔端口预算；边界：不增加生产份额。
	leftovers int
	// tower 职责：引用实际库存实体；使用：全场布线；边界：候选间不共享。
	tower *placed
	// reserved 职责：保存副产物独占的真实塔口；使用：后续回收带；边界：普通路线不可占用。
	reserved []int
}

// denseStationRoute 职责：保留机器行与真实塔口之间的连接需求；使用：全场统一布线；边界：剩余出口独立占口。
type denseStationRoute struct {
	// stationRoute 职责：保存物料、方向与机器端点；使用：与现有物流证据共用；边界：不是虚拟连线。
	stationRoute
	// row 职责：关联所属局部功能实体；使用：全场位置选择；边界：不限定空间分区。
	row *denseStationRow
	// leftover 职责：标记还需真实库存出带的规范剩余物品；使用：端口预留；边界：不代表全部产物均为剩余。
	leftover bool
}

// assembleDenseStationRows 职责：装配已验证机器数的实际行及供剂，暂不建局部庭院或固定塔；使用：密铺全场候选；边界：临时延后供电，最终必须全场补齐。
func (b *builder) assembleDenseStationRows(ctx context.Context, input contract.StationLayoutInput, plan contract.StationLayoutPlan, config contract.StationConfig, maxSpan float64, useStreets bool) ([]*denseStationRow, []denseStationRoute, error) {
	rows, routes := []*denseStationRow{}, []denseStationRoute{}
	previousPower := b.setting.NeedPower
	disabled := false
	b.setting.NeedPower = &disabled
	defer func() { b.setting.NeedPower = previousPower }()
	for _, zone := range plan.Zones {
		for _, work := range zone.Productions {
			production, recipe := input.Plan.Productions[work.Index], input.Productions[work.Index].Recipe
			leftovers := b.stationLeftoverKinds(work.Index)
			counts, err := b.layoutDenseSpatialRows(production, recipe, work.Counts, maxSpan)
			if err != nil {
				return nil, nil, err
			}
			attach := func(l *line, entities []*placed) error {
				row := &denseStationRow{line: l, entities: append([]*placed(nil), entities...)}
				share := float64(len(l.machines)) / float64(production.MachineCount)
				appendRoute := func(route stationRoute, leftover bool) {
					row.routes = append(row.routes, len(routes))
					routes = append(routes, denseStationRoute{stationRoute: route, row: row, leftover: leftover})
				}
				for i, belts := range l.inputs {
					item := recipe.Items[i]
					rate, err := productionRoleInputRate(production, recipe, item, recipeInputRole)
					if err != nil {
						return err
					}
					appendRoute(stationRoute{head: belts[0], item: item, rate: rate * share}, false)
				}
				if item := productionSprayItem(production); item > 0 {
					if l.coaterInput == nil {
						return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "密铺塔机器行缺少实际供剂入口"}
					}
					rate, err := productionRoleInputRate(production, recipe, item, sprayInputRole)
					if err != nil {
						return err
					}
					route := stationRoute{head: l.coaterInput, item: item, rate: rate * share}
					if route.rate == 0 && productionItemRate(production.Outputs, item) > 0 {
						route.sprayRecycle = l
					}
					appendRoute(route, false)
				}
				for _, item := range uniqueStationItems(recipe.Results) {
					head, err := b.stationOutputHead(l, item)
					if err != nil {
						return err
					}
					appendRoute(stationRoute{head: head, item: item, output: true, rate: productionItemRate(production.Outputs, item) * share}, leftovers[item])
				}
				rows = append(rows, row)
				return nil
			}
			b.owner = work.Index
			if useStreets && b.denseStationStreetEligible(production, recipe) {
				// 第一候选保留原单容量组，减少没有运输需要的额外塔口；第二候选仍比较细分后的折排形状。
				if len(counts) == 1 && maxSpan >= 32 {
					counts = []int{(counts[0] + 1) / 2, counts[0] / 2}
				}
				assembly, err := b.buildDenseStreetAccessories(ctx, production, recipe, counts)
				if err != nil {
					return nil, nil, err
				}
				if err := b.registerDenseStationAssembly(assembly, production, &rows, &routes); err != nil {
					return nil, nil, err
				}
				continue
			}
			for _, count := range counts {
				if err := b.checkGeneration("密铺塔机器端口装配"); err != nil {
					return nil, nil, err
				}
				begin := len(b.all)
				l, err := b.buildLine(production, recipe, count, config.Mode, 0, true)
				if err != nil {
					return nil, nil, err
				}
				if err = b.coatSupply([]*line{l}); err != nil {
					return nil, nil, err
				}
				if err := attach(l, b.all[begin:]); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	if err := b.trimDenseStationTails(rows, routes); err != nil {
		return nil, nil, err
	}
	return rows, routes, nil
}

// denseStationStreetEligible 职责：选择已具备真实连续端口来源的塔街道；使用：独立街道候选；边界：喷涂/监测由共享附件构造及真实接驳证明，不改变可信生产配置。
func (b *builder) denseStationStreetEligible(production quant.Production, recipe quant.Recipe) bool {
	return production.MachineCount > 1 && b.referenceLowTrackEligible(production, recipe)
}

// trimDenseStationTails 职责：在空间搜索前删除实际取放点之外的无用局部带尾；使用：避免预留不存在于最终产物的轮廓；边界：保留喷涂、监测器、机器臂和所有塔入口。
func (b *builder) trimDenseStationTails(rows []*denseStationRow, routes []denseStationRoute) error {
	required, removed := map[*placed]bool{}, map[*placed]bool{}
	preserveDenseCargoNeighborhoods(b.all, required)
	for _, entity := range b.all {
		required[entity.input] = true
		if !surfaceIsBelt(entity.record.ItemID) {
			required[entity.output] = true
		}
	}
	for _, route := range routes {
		required[route.head] = true
	}
	trim := func(belts []*placed) ([]*placed, error) {
		last := -1
		for i, p := range belts {
			if required[p] {
				last = i
			}
		}
		if last < 0 {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "密铺塔局部物料带缺少实际机器或设施接点"}
		}
		for _, p := range belts[last+1:] {
			removed[p] = true
		}
		return belts[:last+1], nil
	}
	for _, row := range rows {
		if row.unit != nil && row.unit.assembly != nil {
			// 共享附件构造已经按实际cargo邻域裁剪，并保存了commit所需的核心数组；此处不能再次裁断未接回的模块契约。
			continue
		}
		for i, belts := range row.line.inputs {
			kept, err := trim(belts)
			if err != nil {
				return err
			}
			row.line.inputs[i] = kept
		}
		for i, belts := range row.line.outputRows() {
			kept, err := trim(belts)
			if err != nil {
				return err
			}
			row.line.outputs[i] = kept
			if i == 0 {
				row.line.output = kept
			}
		}
	}
	for _, p := range b.all {
		if !removed[p] && removed[p.output] {
			p.output = nil
			p.record.OutputToSlot = 0
		}
	}
	keep := func(all []*placed) []*placed {
		kept := make([]*placed, 0, len(all))
		for _, p := range all {
			if !removed[p] {
				kept = append(kept, p)
			}
		}
		return kept
	}
	units := map[*denseStationUnit]bool{}
	for _, row := range rows {
		row.entities = keep(row.entities)
		row.line.all = keep(row.line.all)
		if row.unit != nil && !units[row.unit] {
			row.unit.entities = keep(row.unit.entities)
			units[row.unit] = true
		}
	}
	b.all = keep(b.all)
	return nil
}

// planDenseStationTowers 职责：优先把整个机器行端口放入兼容的跨产线库存，再按物品角色拆塔；使用：真实塔容量预算；边界：自产供剂出入口同一库存，副产物预留独立带口。
func planDenseStationTowers(ctx context.Context, rows []*denseStationRow, routes []denseStationRoute, slots int) ([]denseStationTower, error) {
	check := func() error {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺共享塔库存规划已取消或超时"}
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	towers := []denseStationTower{}
	role := func(route denseStationRoute) stationItem {
		return stationItem{id: route.item, provide: route.output || route.sprayRecycle != nil}
	}
	need := func(indices []int) int {
		ports := len(indices)
		for _, i := range indices {
			if routes[i].leftover {
				ports++
			}
		}
		return ports
	}
	newKinds := func(tower denseStationTower, indices []int) (int, bool) {
		items := append([]stationItem(nil), tower.items...)
		for _, i := range indices {
			item := role(routes[i])
			found := false
			for _, existing := range items {
				if existing.id == item.id {
					if existing.provide != item.provide {
						return 0, false
					}
					found = true
				}
			}
			if !found {
				items = append(items, item)
			}
		}
		return len(items) - len(tower.items), len(items) <= slots && len(tower.routes)+tower.leftovers+need(indices) <= 12
	}
	assign := func(indices []int) error {
		chosen, score := -1, math.MaxInt
		for i, tower := range towers {
			if i%32 == 0 {
				if err := check(); err != nil {
					return err
				}
			}
			added, ok := newKinds(tower, indices)
			if !ok {
				continue
			}
			candidate := added*100 + 12 - len(tower.routes) - tower.leftovers - need(indices)
			if candidate < score {
				chosen, score = i, candidate
			}
		}
		if chosen < 0 {
			if _, ok := newKinds(denseStationTower{}, indices); !ok {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "密铺塔角色组超过物品槽或十二带口容量"}
			}
			towers = append(towers, denseStationTower{})
			chosen = len(towers) - 1
		}
		tower := &towers[chosen]
		for _, index := range indices {
			item := role(routes[index])
			found := false
			for _, existing := range tower.items {
				found = found || existing.id == item.id
			}
			if !found {
				tower.items = append(tower.items, item)
			}
			tower.routes = append(tower.routes, index)
			if routes[index].leftover {
				tower.leftovers++
			}
			routes[index].tower = chosen
		}
		return nil
	}
	for _, row := range rows {
		if err := check(); err != nil {
			return nil, err
		}
		whole := false
		for i, tower := range towers {
			if i%32 == 0 {
				if err := check(); err != nil {
					return nil, err
				}
			}
			if _, ok := newKinds(tower, row.routes); ok {
				whole = true
				break
			}
		}
		if _, ok := newKinds(denseStationTower{}, row.routes); ok && len(towers) == 0 {
			whole = true
		}
		if whole {
			if err := assign(row.routes); err != nil {
				return nil, err
			}
			continue
		}
		groups := [][]int{}
		keys := []stationItem{}
		for _, index := range row.routes {
			item := role(routes[index])
			group := -1
			for i, key := range keys {
				if key == item {
					group = i
					break
				}
			}
			if group < 0 {
				keys = append(keys, item)
				groups = append(groups, []int{})
				group = len(groups) - 1
			}
			groups[group] = append(groups[group], index)
		}
		for _, indices := range groups {
			if err := assign(indices); err != nil {
				return nil, err
			}
		}
	}
	for _, tower := range towers {
		if len(tower.items) > slots || len(tower.routes)+tower.leftovers > 12 {
			return nil, fmt.Errorf("密铺塔内部预算不一致")
		}
	}
	if err := check(); err != nil {
		return nil, err
	}
	return towers, nil
}
