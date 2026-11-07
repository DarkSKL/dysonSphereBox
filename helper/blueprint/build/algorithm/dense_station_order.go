package build

// 文件职责：让大厂的共享物料消费者与供货核心在真实轮廓搜索中优先相邻落位。
// 使用：独立黑盒布局候选；边界：只重排占位单元，不修改规范图、设备数量、局部机器周期或物料能力。

import (
	"context"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// denseStationMaterialUnitOrder 职责：按实际流路和共享原料亲和度组织核心，再填入其独立附件；使用：大厂枢纽优先/最终消费者优先两个候选；边界：真实连接仍由随后路由和终检完成，亲和度不是通货证明。
func (b *builder) denseStationMaterialUnitOrder(ctx context.Context, units []denseSpatialUnit, consumerFirst bool) ([]denseSpatialUnit, error) {
	n := len(units)
	if n < 2 {
		return append([]denseSpatialUnit(nil), units...), nil
	}
	members := map[*placed]int{}
	core, terminal := make([]bool, n), make([]bool, n)
	areas := make([]float64, n)
	weights := make([]map[int]float64, n)
	for i, unit := range units {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒物料相邻排序已取消"}
		}
		weights[i] = map[int]float64{}
		low, high := bounds(unit.entities)
		areas[i] = math.Max(1, (high.x-low.x+1)*(high.y-low.y+1))
		for _, entity := range unit.entities {
			members[entity] = i
			core[i] = core[i] || entity.record.RecipeID > 0 || surfaceIsStation(entity.record.ItemID)
		}
	}
	add := func(a, c int, weight float64) {
		if a >= 0 && c >= 0 && a != c && finitePositive(weight) {
			weights[a][c] += weight
			weights[c][a] += weight
		}
	}
	indexOf := func(entity *placed) int {
		if i, ok := members[entity]; ok {
			return i
		}
		return -1
	}
	rowCore := func(row *transportRow, fallback *placed) int {
		if row != nil && row.line != nil && len(row.line.machines) > 0 {
			return indexOf(row.line.machines[0])
		}
		return indexOf(fallback)
	}
	consumers := map[int]map[int]float64{}
	routeSets := [][]*transportRoute{b.transportRoutes, b.denseStreetRoutes, b.denseMaterialNetworkRoutes()}
	if b.denseMaterialNetworks != nil {
		routeSets = append(routeSets, b.denseMaterialNetworks.canonical)
	}
	for _, routes := range routeSets {
		for _, route := range routes {
			if route == nil {
				continue
			}
			weight := math.Sqrt(math.Max(1, route.rate))
			add(indexOf(route.start), indexOf(route.end), weight)
			from, to := rowCore(route.from, route.start), rowCore(route.to, route.end)
			add(from, to, weight)
			if route.edge.FlowKind == "target-output" && from >= 0 {
				terminal[from] = true
			}
			if route.to != nil && to >= 0 && route.inputRole == recipeInputRole && route.edge.ItemID > 0 {
				if consumers[route.edge.ItemID] == nil {
					consumers[route.edge.ItemID] = map[int]float64{}
				}
				consumers[route.edge.ItemID][to] += route.rate
			}
		}
	}
	if b.denseMaterialNetworks != nil {
		for _, group := range b.denseMaterialNetworks.groups {
			for i := 1; i < len(group.rows); i++ {
				a, c := rowCore(group.rows[i-1], nil), rowCore(group.rows[i], nil)
				add(a, c, 2*math.Sqrt(math.Max(1, group.configured[i-1]+group.configured[i])))
			}
		}
	}
	// 相同原料的相邻消费者形成稀疏亲和链，避免大规模同物料工厂产生O(n²)邻接库存。
	for _, group := range consumers {
		indices := make([]int, 0, len(group))
		for index := range group {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for i := 1; i < len(indices); i++ {
			a, c := indices[i-1], indices[i]
			add(a, c, math.Sqrt(math.Max(1, math.Min(group[a], group[c]))))
		}
	}
	degree := make([]float64, n)
	for i := range units {
		for _, weight := range weights[i] {
			degree[i] += weight
		}
	}
	placed := make([]bool, n)
	result := make([]denseSpatialUnit, 0, n)
	coreRemaining := 0
	for _, isCore := range core {
		if isCore {
			coreRemaining++
		}
	}
	for len(result) < n {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "黑盒物料邻接排序已取消"}
		}
		best, score := -1, math.Inf(-1)
		for i := range units {
			if placed[i] || coreRemaining > 0 && !core[i] {
				continue
			}
			value := 0.
			for neighbor, weight := range weights[i] {
				if placed[neighbor] {
					value += weight
				}
			}
			if len(result) == 0 {
				value = degree[i]
				if consumerFirst && terminal[i] {
					value += 1e9
				}
			} else {
				value += degree[i] * 1e-4
			}
			value /= math.Sqrt(math.Sqrt(areas[i]))
			if best < 0 || value > score+1e-9 || math.Abs(value-score) <= 1e-9 && areas[i] > areas[best] {
				best, score = i, value
			}
		}
		if best < 0 {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "黑盒物料排序遗漏占位单元"}
		}
		placed[best] = true
		if core[best] {
			coreRemaining--
		}
		result = append(result, units[best])
	}
	return result, nil
}
