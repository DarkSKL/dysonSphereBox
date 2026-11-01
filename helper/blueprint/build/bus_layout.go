package build

// 文件职责：在寻路前规划混带环路的容量分组及邻近落位。
// 使用：mode6 的预装配阶段比较固定数量的几何候选；边界：仅估计接驳带长，不替代真实寻路、逐圈容量与最终碰撞规则。

import (
	"math"
	"sort"
)

// busFabricPlacement 职责：保存一个环路在未提交候选中的落位；使用：容量分组确定后的实体装配；边界：origin 为闭环左下角，不包含接驳和电杆的外围净空。
type busFabricPlacement struct {
	bin    int
	origin vector
}

// busFabricLayout 职责：保存一个固定分组及落位候选；使用：按实体带长下界选择寻路起点；边界：estimatedLength 不宣称是有障碍的实际带长或产能证明。
type busFabricLayout struct {
	name            string
	bins            []busFabricBin
	placements      []busFabricPlacement
	estimatedLength float64
}

// betterBusFabricCandidate 职责：只接纳不增加实际建筑数的空间优化结果；使用：完整环路候选比较；边界：两侧必须已有严格最终规则和物流摘要，不能仅凭估计面积掩盖线路实体膨胀。
func betterBusFabricCandidate(candidate, baseline Result) bool {
	return candidate.LogisticsSummary != nil && baseline.LogisticsSummary != nil && candidate.BuildingCount <= baseline.BuildingCount && betterTransportCandidate(candidate.LogisticsSummary.Metrics, baseline.LogisticsSummary.Metrics)
}

// planBusFabricLayout 职责：比较基线、紧邻落位与供需邻近分组；使用：mode6 装配前的有限候选；边界：球面先保留已核验的单排纬向预算，不移动机器或改变规范 lane。
func (b *builder) planBusFabricLayout(pending []*busFabricRoute, minimum, maximum vector, surface bool) (busFabricLayout, error) {
	bins, err := b.packBusFabric(pending, false)
	if err != nil {
		return busFabricLayout{}, err
	}
	baseline := busFabricLayout{name: "capacity-first-north", bins: bins, placements: baselineBusFabricPlacements(bins, minimum, maximum, surface)}
	baseline.estimatedLength = estimateBusFabricLength(baseline)
	if surface || len(bins) == 0 {
		return baseline, nil
	}
	best := baseline
	compact := busFabricLayout{name: "capacity-first-neighbors", bins: bins, placements: neighborBusFabricPlacements(bins, minimum, maximum)}
	compact.estimatedLength = estimateBusFabricLength(compact)
	if compact.estimatedLength < best.estimatedLength {
		best = compact
	}
	nearbyBins, err := b.packBusFabric(pending, true)
	if err != nil {
		return busFabricLayout{}, err
	}
	nearby := busFabricLayout{name: "endpoint-neighbors", bins: nearbyBins, placements: neighborBusFabricPlacements(nearbyBins, minimum, maximum)}
	nearby.estimatedLength = estimateBusFabricLength(nearby)
	if nearby.estimatedLength < best.estimatedLength {
		best = nearby
	}
	return best, nil
}

// packBusFabric 职责：在单带容量和实际端口高度内分组；使用：流量首适应基线与邻近候选；边界：邻近分组允许增加真实环路，不能把远端小流量的连接成本当作零。
func (b *builder) packBusFabric(pending []*busFabricRoute, nearby bool) ([]busFabricBin, error) {
	bins := []busFabricBin{}
	for _, entry := range pending {
		if err := b.checkGeneration("混带环路容量与邻近分组"); err != nil {
			return nil, err
		}
		arms, err := b.busFabricArmCount(entry.route.rate)
		if err != nil {
			return nil, err
		}
		portHeight := busFabricPortHeight(entry, arms)
		entry.portHeight = portHeight
		chosen := -1
		// 新圈至少增加两侧净空与闭环本体；只有减少的接驳下界超过
		// 这项真实成本才新开圈，避免为了极小空间差异复制大量环路。
		bestExtra := busFabricSpread([]*busFabricRoute{entry}) + float64(2*(portHeight+4+6)+24)
		for i := range bins {
			if bins[i].load+entry.route.rate > b.logistics.BeltCapacityPerMinute+1e-6 || bins[i].height+portHeight > 60 {
				continue
			}
			if !nearby {
				chosen = i
				break
			}
			joined := append(append([]*busFabricRoute{}, bins[i].routes...), entry)
			extra := busFabricSpread(joined) - busFabricSpread(bins[i].routes)
			if extra < bestExtra {
				chosen, bestExtra = i, extra
			}
		}
		if chosen < 0 {
			bins = append(bins, busFabricBin{height: 4})
			chosen = len(bins) - 1
		}
		bins[chosen].load += entry.route.rate
		bins[chosen].height += portHeight
		bins[chosen].routes = append(bins[chosen].routes, entry)
	}
	return bins, nil
}

// busFabricPortHeight 职责：返回真实并联端口的纵向跨度；使用：分组和逐圈装配；边界：原料回收入口需要第二组分拣器及供料闭环。
func busFabricPortHeight(entry *busFabricRoute, arms int) int {
	if entry.route.from == nil {
		return arms*2 + 7
	}
	return arms + 4
}

// busFabricSpread 职责：求同圈实际供需端点到二维中位点的距离下界；使用：邻近分组比较；边界：每根实体连接器等权，不按流量忽略小流量长距离接驳。
func busFabricSpread(entries []*busFabricRoute) float64 {
	xs, ys := []float64{}, []float64{}
	for _, entry := range entries {
		if entry.route.from != nil {
			xs, ys = append(xs, entry.route.start.a.x), append(ys, entry.route.start.a.y)
		}
		xs, ys = append(xs, entry.route.end.a.x), append(ys, entry.route.end.a.y)
	}
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	sort.Float64s(ys)
	x, y, distance := xs[len(xs)/2], ys[len(ys)/2], 0.
	for i := range xs {
		distance += math.Abs(xs[i]-x) + math.Abs(ys[i]-y)
	}
	return distance
}

// baselineBusFabricPlacements 职责：保留原容量优先的北侧固定节距候选；使用：已通过的球面布局及邻近改进对照；边界：不在函数内写入实体或 lane 归属。
func baselineBusFabricPlacements(bins []busFabricBin, minimum, maximum vector, surface bool) []busFabricPlacement {
	order := make([]int, len(bins))
	centers := make([]float64, len(bins))
	for index, bin := range bins {
		order[index] = index
		for _, entry := range bin.routes {
			center := entry.route.end.a.x
			if entry.route.start != nil {
				center = (center + entry.route.start.a.x) / 2
			}
			centers[index] += center * entry.route.rate / bin.load
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return centers[order[i]] < centers[order[j]] })
	columns := max(1, min(len(bins), int(math.Ceil((maximum.x-minimum.x+1)/22))))
	if surface && len(bins)*22 <= 900 {
		columns = max(1, len(bins))
	}
	rows := max(1, int(math.Ceil(float64(len(bins))/float64(columns))))
	columns = max(1, int(math.Ceil(float64(len(bins))/float64(rows))))
	baseX := math.Floor((minimum.x + maximum.x - float64((columns-1)*22) - 6) / 2)
	placements := make([]busFabricPlacement, 0, len(bins))
	for rank, index := range order {
		placements = append(placements, busFabricPlacement{bin: index, origin: vector{baseX + float64((rank/rows)*22), math.Floor(minimum.y) - float64(bins[index].height) - 12 - float64((rank%rows)*72), 0}})
	}
	return placements
}

// neighborBusFabricPlacements 职责：把端口多的圈优先放在供需附近的南北空闲列；使用：平面邻近候选；边界：列间22格、圈间12格净空保持，按实际环高推进而非一律预留72格。
func neighborBusFabricPlacements(bins []busFabricBin, minimum, maximum vector) []busFabricPlacement {
	placements, _ := neighborBusFabricPlacementsWithinDepth(bins, minimum, maximum, 0)
	return placements
}

// neighborBusFabricPlacementsWithinDepth 职责：在不改变环路净空下限制南北最远落位；使用：平面紧凑候选；边界：零表示无附加深度限制，排不下返回失败而不缩小环路或提高容量。
func neighborBusFabricPlacementsWithinDepth(bins []busFabricBin, minimum, maximum vector, depth float64) ([]busFabricPlacement, bool) {
	columns := max(1, int(math.Ceil((maximum.x-minimum.x+1)/22)))
	baseX := math.Floor((minimum.x + maximum.x - float64((columns-1)*22) - 6) / 2)
	north, south := make([]float64, columns), make([]float64, columns)
	for i := range north {
		north[i], south[i] = math.Floor(minimum.y), math.Ceil(maximum.y)
	}
	order := make([]int, len(bins))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		if len(bins[order[i]].routes) != len(bins[order[j]].routes) {
			return len(bins[order[i]].routes) > len(bins[order[j]].routes)
		}
		return bins[order[i]].height > bins[order[j]].height
	})
	placements := make([]busFabricPlacement, 0, len(bins))
	for _, index := range order {
		best, bestCost, bestColumn, bestSouth := vector{}, math.Inf(1), 0, false
		for column := 0; column < columns; column++ {
			left := baseX + float64(column*22)
			for _, below := range []bool{false, true} {
				low := north[column] - float64(bins[index].height) - 12
				if below {
					low = south[column] + 12
				}
				if depth > 0 && (!below && math.Floor(minimum.y)-low > depth || below && low+float64(bins[index].height)-math.Ceil(maximum.y) > depth) {
					continue
				}
				origin := vector{left, low, 0}
				cost := busFabricConnectorLength(bins[index], origin)
				if cost < bestCost {
					best, bestCost, bestColumn, bestSouth = origin, cost, column, below
				}
			}
		}
		if math.IsInf(bestCost, 1) {
			return nil, false
		}
		placements = append(placements, busFabricPlacement{bin: index, origin: best})
		if bestSouth {
			south[bestColumn] = best.y + float64(bins[index].height)
		} else {
			north[bestColumn] = best.y
		}
	}
	return placements, true
}

// tightenBusFabricLayout 职责：用三个固定深度上界把热点列向邻近空列溢出；使用：成功基线之后的一个完整可行性候选；边界：仅降低预估矩形面积，调用方必须独立寻路和收尾才允许选用。
func tightenBusFabricLayout(layout busFabricLayout, minimum, maximum vector) (busFabricLayout, bool) {
	if len(layout.bins) == 0 {
		return busFabricLayout{}, false
	}
	columns := max(1, int(math.Ceil((maximum.x-minimum.x+1)/22)))
	total, tallest := 0., 0.
	for _, bin := range layout.bins {
		height := float64(bin.height + 12)
		total += height
		tallest = math.Max(tallest, height)
	}
	best, bestArea, found := busFabricLayout{}, busFabricLayoutArea(layout, minimum, maximum), false
	for _, factor := range []float64{1.1, 1.25, 1.5} {
		depth := math.Max(tallest, math.Ceil(total/float64(columns*2)*factor))
		placements, ok := neighborBusFabricPlacementsWithinDepth(layout.bins, minimum, maximum, depth)
		if !ok {
			continue
		}
		candidate := busFabricLayout{name: layout.name + "-bounded-depth", bins: layout.bins, placements: placements}
		candidate.estimatedLength = estimateBusFabricLength(candidate)
		area := busFabricLayoutArea(candidate, minimum, maximum)
		if area < bestArea || area == bestArea && found && candidate.estimatedLength < best.estimatedLength {
			best, bestArea, found = candidate, area, true
		}
	}
	return best, found
}

// busFabricLayoutArea 职责：计算机器区与真实环路外围的保守矩形估计；使用：深度候选筛选；边界：未包含有障碍接驳的最终包络，不能代替最终 Metrics。
func busFabricLayoutArea(layout busFabricLayout, minimum, maximum vector) float64 {
	for _, placement := range layout.placements {
		minimum.x = math.Min(minimum.x, placement.origin.x-6)
		maximum.x = math.Max(maximum.x, placement.origin.x+12)
		minimum.y = math.Min(minimum.y, placement.origin.y)
		maximum.y = math.Max(maximum.y, placement.origin.y+float64(layout.bins[placement.bin].height))
	}
	return (maximum.x - minimum.x + 1) * (maximum.y - minimum.y + 1)
}

// busFabricConnectorLength 职责：按每条 lane 的真实feed/take位置估计接驳带长；使用：落位候选比较；边界：不计算障碍绕行或把外供原料当作另一条内部接驳。
func busFabricConnectorLength(bin busFabricBin, origin vector) float64 {
	y, distance := origin.y+2, 0.
	for _, entry := range bin.routes {
		route := entry.route
		if route.from != nil {
			distance += math.Abs(route.start.a.x-(origin.x+9)) + math.Abs(route.start.a.y-y) + math.Abs(route.start.a.z)
		}
		distance += math.Abs(route.end.a.x-(origin.x-3)) + math.Abs(route.end.a.y-y) + math.Abs(route.end.a.z)
		y += float64(entry.portHeight)
	}
	return distance
}

// estimateBusFabricLength 职责：比较实体环路长度与全部接驳的无障碍下界；使用：固定布局候选选择；边界：最终选择仍必须通过真实有界寻路及最终规则。
func estimateBusFabricLength(layout busFabricLayout) float64 {
	length := 0.
	for _, placement := range layout.placements {
		bin := layout.bins[placement.bin]
		length += float64(2*(bin.height+6)) + busFabricConnectorLength(bin, placement.origin)
	}
	return length
}
