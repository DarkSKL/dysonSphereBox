package build

// 文件职责：在密铺共同占位场中为全部机器与共享塔槽统一分配端口、布线和登记证据。
// 使用：密铺塔全场几何完成后；边界：任一路线无解则拒绝候选，不借库存配置伪造实体连通。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"context"
	"fmt"
	"math"
	"sort"
)

// connectDenseStationField 职责：按距离分配实际塔口并连接全部需求、供货和自产供剂；使用：密铺塔路由阶段；边界：仍由现有站点逐边审计核验角色、毛流和端口。
func (b *builder) connectDenseStationField(ctx context.Context, rows []*denseStationRow, routes []denseStationRoute, towers []denseStationTower, config contract.StationConfig) error {
	continuous := false
	for _, row := range rows {
		continuous = continuous || row.unit != nil || row.line.info.MachineType == 4 && !row.line.referenceLowTracks
	}
	for towerIndex := range towers {
		tower := &towers[towerIndex]
		used := map[int]bool{}
		indices := append([]int(nil), tower.routes...)
		sort.SliceStable(indices, func(i, j int) bool {
			a, c := routes[indices[i]].head.a, routes[indices[j]].head.a
			return math.Abs(a.x-tower.tower.a.x)+math.Abs(a.y-tower.tower.a.y) > math.Abs(c.x-tower.tower.a.x)+math.Abs(c.y-tower.tower.a.y)
		})
		for _, index := range indices {
			best, score := -1, math.Inf(1)
			for slot := 0; slot < 12; slot++ {
				if used[slot] {
					continue
				}
				point := denseStationOuter(tower.tower.a, slot)
				distance := math.Abs(point.x-routes[index].head.a.x) + math.Abs(point.y-routes[index].head.a.y)
				if distance < score {
					best, score = slot, distance
				}
			}
			if best < 0 {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "共享密铺塔实际带口不足"}
			}
			routes[index].slot = best
			used[best] = true
		}
		for _, slot := range []int{9, 10, 11, 8, 0, 7, 1, 6, 2, 5, 3, 4} {
			if !used[slot] && len(tower.reserved) < tower.leftovers {
				tower.reserved = append(tower.reserved, slot)
			}
		}
	}
	blocked, err := denseTransportObstacles(ctx, b.all)
	if err != nil {
		return err
	}
	low, high := bounds(b.all)
	env := &transportEnvironment{blocked: blocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{math.Floor(low.x) - 6, math.Floor(low.y) - 6, 0}, maximum: vector{math.Ceil(high.x) + 6, math.Ceil(high.y) + 6, transportMaxHeight}, preferCorridor: true, beltCorridors: map[[2]int]bool{}}
	for _, p := range b.all {
		if surfaceIsBelt(p.record.ItemID) {
			blockDenseBeltNode(env.blocked, p.a)
			env.beltCorridors[beltColumn(p.a)] = true
		}
	}
	ends := make([]*placed, len(routes))
	reserve := func(point vector, owner *placed) {
		if continuous {
			return
		}
		for z := 0.; z <= transportMaxHeight; z++ {
			cell := beltCell(vector{point.x, point.y, z})
			if env.reserved[cell] == nil {
				env.reserved[cell] = map[*placed]bool{}
			}
			env.reserved[cell][owner] = true
		}
	}
	for i, route := range routes {
		tower := towers[route.tower].tower
		ends[i] = &placed{a: denseStationOuter(tower.a, route.slot)}
		reserve(route.head.a, route.head)
		reserve(ends[i].a, ends[i])
		for _, point := range denseStationInner(tower.a, route.slot) {
			blockDenseBeltNode(env.blocked, point)
			reserve(point, ends[i])
		}
	}
	for _, tower := range towers {
		for _, slot := range tower.reserved {
			for _, point := range append(denseStationInner(tower.tower.a, slot), denseStationOuter(tower.tower.a, slot)) {
				reserve(point, tower.tower)
				blockDenseBeltNode(env.blocked, point)
			}
		}
	}
	if continuous {
		if err := b.prepareDenseStationPortalIndex(ctx, env, routes, ends, towers); err != nil {
			return err
		}
		if err := b.routeDenseStationAccessories(ctx, env, rows); err != nil {
			return err
		}
		for _, assembly := range denseStationAssemblies(rows) {
			if err := b.commitDenseStreetAccessories(assembly); err != nil {
				return err
			}
		}
	}
	order := make([]int, len(routes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, c := routes[order[i]], routes[order[j]]
		return math.Abs(a.head.a.x-ends[order[i]].a.x)+math.Abs(a.head.a.y-ends[order[i]].a.y) < math.Abs(c.head.a.x-ends[order[j]].a.x)+math.Abs(c.head.a.y-ends[order[j]].a.y)
	})
	for _, index := range order {
		route := &routes[index]
		var path []vector
		var expanded int
		if continuous {
			path, expanded = env.findDenseStationPath(ctx, route, ends[index], towers[route.tower].tower)
		} else {
			path, expanded = env.findTransportPath(ctx, route.head.a, ends[index].a, route.head, ends[index], false)
		}
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺塔全场接线已取消"}
		}
		if len(path) < 2 {
			return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("密铺塔物品%d由%+v到%+v无法接线，展开%d", route.item, route.head.a, ends[index].a, expanded)}
		}
		points := append(append([]vector(nil), path[1:]...), denseStationInner(towers[route.tower].tower.a, route.slot)...)
		b.owner = route.head.owner
		belts := b.byPath(points)
		last := belts[len(belts)-1]
		tower := towers[route.tower].tower
		base := 192 + route.slot*4
		last.record.Parameters = []int32{int32(route.item), 0}
		if route.output {
			connect(route.head, belts[0])
			last.output = tower
			last.record.OutputToSlot = int8(route.slot)
			tower.record.Parameters[base] = 2
			tower.record.Parameters[base+1] = 0
		} else {
			reverse(belts)
			connect(belts[0], route.head)
			last.input = tower
			last.record.InputFromSlot = int8(route.slot)
			tower.record.Parameters[base] = 1
			for itemSlot, item := range towers[route.tower].items {
				if item.id == route.item {
					tower.record.Parameters[base+1] = int32(itemSlot + 1)
					break
				}
			}
		}
		b.stationLinks = append(b.stationLinks, stationTransportLink{item: route.item, output: route.output, tower: tower, belt: last, slot: route.slot, head: route.head, path: belts, owner: route.head.owner, rate: route.rate, sprayRecycle: route.sprayRecycle})
		for _, point := range append(path, points...) {
			blockDenseBeltNode(env.blocked, point)
			env.beltCorridors[beltColumn(point)] = true
		}
		if env.densePorts != nil {
			for _, belt := range belts {
				env.densePorts.addNode(belt)
			}
		}
		env.version++
	}
	actualTowers, reserved := make([]*placed, len(towers)), make([][]int, len(towers))
	for i, tower := range towers {
		actualTowers[i] = tower.tower
		reserved[i] = tower.reserved
	}
	return b.prepareStationLeftovers(ctx, config.EnableLeftovers, 0, 0, actualTowers, reserved)
}

// denseStationLeftoverTarget 职责：把副产物放到最靠近对应原料的矩形边界，而非统一延长西侧长尾；使用：真实塔副产物出带；边界：保持独立地面出口，不接入原料链。
func (b *builder) denseStationLeftoverTarget(raw *placed, low, high vector) vector {
	best := vector{math.Floor(low.x) - 2, math.Round(raw.a.y), 0}
	score := math.Inf(1)
	for _, delta := range []float64{0, 2, -2, 4, -4, 6, -6} {
		x, y := math.Max(math.Ceil(low.x), math.Min(math.Floor(high.x), math.Round(raw.a.x)+delta)), math.Max(math.Ceil(low.y), math.Min(math.Floor(high.y), math.Round(raw.a.y)+delta))
		for _, point := range []vector{{math.Floor(low.x) - 2, y, 0}, {math.Ceil(high.x) + 2, y, 0}, {x, math.Floor(low.y) - 2, 0}, {x, math.Ceil(high.y) + 2, 0}} {
			clear := true
			for _, route := range b.stationLeftovers {
				end := route.path[len(route.path)-1].a
				if math.Hypot(end.x-point.x, end.y-point.y) < 2 {
					clear = false
					break
				}
			}
			value := math.Abs(point.x-raw.a.x) + math.Abs(point.y-raw.a.y)
			if clear && value < score {
				best, score = point, value
			}
		}
	}
	return best
}
