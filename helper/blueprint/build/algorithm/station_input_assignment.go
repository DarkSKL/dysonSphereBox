package build

// 文件职责：在塔及机器落位后，把原料份额匹配到同塔已有的最近合法带口。
// 使用：独立的塔接线候选、全局寻路之前；边界：不移动建筑、不增加塔口，不改变物料、容量及规范份额，所有路线仍完整终检。

import (
	"context"
	"fmt"
	"math"
	"math/bits"
)

// stationInputAssignmentKey 职责：选择落位后就近分配塔口的独立候选；使用：塔候选上下文；边界：原固定槽序候选继续保留。
type stationInputAssignmentKey struct{}

// minimumStationPortAssignment 职责：在最多十二个已有端口中求最小总接驳距离的双射；使用：同塔输入槽分配；边界：非法距离返回nil，完整带路优劣由路由后指标决定。
func minimumStationPortAssignment(costs [][]float64) []int {
	n := len(costs)
	if n == 0 || n > 12 {
		return nil
	}
	for _, row := range costs {
		if len(row) != n {
			return nil
		}
		for _, cost := range row {
			if math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
				return nil
			}
		}
	}
	size := 1 << n
	dp := make([]float64, size)
	choice := make([]int, size)
	for mask := 1; mask < size; mask++ {
		dp[mask] = math.Inf(1)
	}
	for mask := 0; mask < size-1; mask++ {
		row := bits.OnesCount(uint(mask))
		for slot := 0; slot < n; slot++ {
			if mask&(1<<slot) != 0 {
				continue
			}
			next := mask | 1<<slot
			if value := dp[mask] + costs[row][slot]; value < dp[next]-1e-9 {
				dp[next], choice[next] = value, slot
			}
		}
	}
	if math.IsInf(dp[size-1], 1) {
		return nil
	}
	result := make([]int, n)
	mask := size - 1
	for row := n - 1; row >= 0; row-- {
		result[row] = choice[mask]
		mask ^= 1 << result[row]
	}
	return result
}

// assignStationInputPorts 职责：将普通配方原料与同塔输入短带重新绑定；使用：布局固定后、任何跨产线路由之前；边界：供剂附件、输出、多用途及非完整三节点端口保持原样。
func (b *builder) assignStationInputPorts(ctx context.Context) error {
	enabled, _ := ctx.Value(stationInputAssignmentKey{}).(bool)
	if !enabled || b.adapter == nil || b.adapter.station == nil || b.directBoundaryUnits == nil {
		return nil
	}
	byHead := map[*placed][]*transportRoute{}
	for _, route := range b.transportRoutes {
		if route.from == nil {
			byHead[route.start] = append(byHead[route.start], route)
		}
	}
	towers := []*placed{}
	grouped := map[*placed][]int{}
	for i, link := range b.stationLinks {
		routes := byHead[link.head]
		if link.output || link.sprayRecycle != nil || len(link.path) != 2 || len(routes) != 1 {
			continue
		}
		route := routes[0]
		if route.inputRole != recipeInputRole || route.end == nil || len(route.path) != 1 || route.path[0] != link.head || route.boundary != link.tower {
			continue
		}
		if len(grouped[link.tower]) == 0 {
			towers = append(towers, link.tower)
		}
		grouped[link.tower] = append(grouped[link.tower], i)
	}
	changed, before, after := 0, 0., 0.
	for _, tower := range towers {
		if err := ctx.Err(); err != nil {
			return err
		}
		indices := grouped[tower]
		n := len(indices)
		if n < 2 {
			continue
		}
		// 先冻结所有实体口，再置换业务绑定，不能在交换过程中读取已经改写的源。
		bundles := make([]stationTransportLink, n)
		routes := make([]*transportRoute, n)
		costs := make([][]float64, n)
		stocks := make([]int, n)
		for i, index := range indices {
			bundles[i] = b.stationLinks[index]
			routes[i] = byHead[bundles[i].head][0]
		}
		valid := true
		for i, link := range bundles {
			stocks[i] = -1
			for slot := 0; slot < b.adapter.station.Config.ItemSlots; slot++ {
				if tower.record.Parameters[slot*6] == int32(link.item) {
					stocks[i] = slot
					break
				}
			}
			if stocks[i] < 0 {
				valid = false
				break
			}
			costs[i] = make([]float64, n)
			for j, port := range bundles {
				a, c := port.head.a, routes[i].end.a
				costs[i][j] = math.Abs(a.x-c.x) + math.Abs(a.y-c.y) + math.Abs(a.z-c.z)
			}
		}
		if !valid {
			continue
		}
		assignment := minimumStationPortAssignment(costs)
		if len(assignment) != n {
			continue
		}
		oldCost, newCost := 0., 0.
		for i, j := range assignment {
			oldCost += costs[i][i]
			newCost += costs[i][j]
		}
		if newCost >= oldCost-1e-6 {
			continue
		}
		before, after = before+oldCost, after+newCost
		for i, j := range assignment {
			if i != j {
				changed++
			}
			link, port := &b.stationLinks[indices[i]], bundles[j]
			link.head, link.belt, link.path, link.slot = port.head, port.belt, port.path, port.slot
			routes[i].start, routes[i].path = link.head, []*placed{link.head}
			link.head.record.Parameters = []int32{int32(link.item), int32(math.Ceil(link.rate))}
			link.belt.record.Parameters = []int32{int32(link.item), 0}
			tower.record.Parameters[192+link.slot*4] = 1
			tower.record.Parameters[193+link.slot*4] = int32(stocks[i] + 1)
		}
	}
	if changed > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "station-input-port-distance", Status: "passed", Message: fmt.Sprintf("%d个原料份额就近匹配到同塔已有真实带口，端点网格距离合计%.1f→%.1f；物料、带宽、塔数不变，实际线路另经完整终检", changed, before, after)})
	}
	return nil
}
