package build

// 文件职责：从最终实际图重建独立交付链、来源贡献、毛负载及原料隔离。
// 使用：公共物流和producer尾链保护之前；边界：不信任成员path自身声明的终点，只有登记且独立验证的边才能共享。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

// validateMode1Deliveries 职责：从真实output图重建每份完整交付，复核独立左边界、来源和实际带段毛负载；使用：producer与共同物流终检之前；边界：所有新来源、混料、倒带、丢末端和伪造份额都拒绝。
func (b *builder) validateMode1Deliveries() error {
	p := b.mode1Deliveries
	if p == nil {
		return nil
	}
	p.verified = false
	fail := func(s string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "左侧独立交付验证失败：" + s}
	}
	if !p.committed {
		return fail("未完成交付尾链提交")
	}
	alive := map[*placed]bool{}
	for _, node := range b.all {
		alive[node] = true
	}
	present := map[*transportRoute]bool{}
	raw := map[*placed]bool{}
	for _, r := range b.transportRoutes {
		present[r] = true
		if r.from == nil {
			for _, nodes := range [][]*placed{r.path, r.sharedPath} {
				for _, n := range nodes {
					if surfaceIsBelt(n.record.ItemID) {
						raw[n] = true
					}
				}
			}
		}
	}
	edges := map[[2]*placed]bool{}
	colors := map[*placed]int{}
	allowed := map[*placed]bool{}
	arms := map[*placed]bool{}
	loads := map[*placed]map[*transportRow]float64{}
	seenRoutes := map[*transportRoute]bool{}
	step := 1.
	if b.nativeLatitudeScaled && b.nativeLongitudeScale > 0 {
		step = b.nativeLongitudeScale
	}
	for _, g := range p.groups {
		if len(g.members) == 0 || !finitePositive(g.planned) || !finitePositive(g.maximum) {
			return fail("交付组没有正的真实规范份额或配置毛能力")
		}
		if !alive[g.head] || !alive[g.end] || g.head == g.end || g.head.output != g.end || g.end.output != nil || math.Abs(g.head.a.x-g.end.a.x-step) > 1e-5 || math.Abs(g.head.a.y-g.end.a.y) > 1e-5 || math.Abs(g.head.a.z) > 1e-5 || math.Abs(g.end.a.z) > 1e-5 {
			return fail("预定尾链不再是独立地面向西交付")
		}
		for _, marker := range []*placed{g.head, g.end} {
			if len(marker.record.Parameters) != 2 || int(marker.record.Parameters[0]) != g.item || int(marker.record.Parameters[1]) != int(math.Ceil(g.planned)) {
				return fail("合并交付尾链缺少聚合净流标注")
			}
		}
		for _, input := range p.inputs {
			if !alive[input] || input == g.end || math.Abs(input.a.x-g.end.a.x) > 1e-5 || math.Abs(input.a.y-g.end.a.y) < 2-1e-5 {
				return fail("交付口没有与原料入口保持独立同侧边界")
			}
		}
		fulls := map[*transportRow]float64{}
		net, maximum := 0., 0.
		for _, r := range g.members {
			id, ok := p.identities[r]
			if !ok || !present[r] || seenRoutes[r] || p.byRoute[r] != g || r.edge != id.edge || r.rate != id.rate || r.from != id.from || r.to != id.to || r.inputRole != id.role || r.start != id.start || r.edge.ItemID != g.item || r.end != g.end || r.boundary != g.end {
				return fail("canonical份额或真实交付边界被改变")
			}
			seenRoutes[r] = true
			actual, e := denseMaterialDirectedPath(id.first, g.end, alive)
			if e != nil || !sameDenseMaterialNodes(actual, r.path) {
				return fail("成员路径不是从原生产接带到预定交付口的完整真实链")
			}
			hasStart, hasHead := false, false
			for i, node := range actual {
				hasStart = hasStart || node == id.start
				hasHead = hasHead || node == g.head
				if i > 0 {
					edges[[2]*placed{actual[i-1], node}] = true
				}
			}
			if !hasStart || !hasHead {
				return fail("成员未经过自己的物理起点或预定合流头")
			}
			full, e := b.stationOutputMaximum(r)
			if e != nil {
				return e
			}
			if _, known := fulls[r.from]; !known {
				fulls[r.from] = full
				maximum += full
			}
			net += r.rate
			for _, node := range append(append([]*placed{}, actual...), r.sharedPath...) {
				if !alive[node] {
					return fail("份额引用蓝图外实体")
				}
				if !surfaceIsBelt(node.record.ItemID) {
					continue
				}
				if raw[node] {
					return fail("成品交付带混入原料带")
				}
				if item, known := colors[node]; known && item != g.item {
					return fail("不同交付物品混用带段")
				}
				colors[node] = g.item
				allowed[node] = true
				if loads[node] == nil {
					loads[node] = map[*transportRow]float64{}
				}
				loads[node][r.from] = full
			}
			for _, arm := range r.filters {
				arms[arm] = true
			}
			if r.from.line != nil {
				for _, arm := range r.from.line.all {
					if arm.record.ItemID >= 2011 && arm.record.ItemID <= 2014 && arm.input != nil && arm.input.record.RecipeID > 0 && allowed[arm.output] {
						arms[arm] = true
					}
				}
			}
		}
		if len(fulls) != len(g.sources) || math.Abs(net-g.planned) > math.Max(1e-6, net*1e-9) || math.Abs(maximum-g.maximum) > math.Max(1e-6, maximum*1e-9) || maximum > b.logistics.BeltCapacityPerMinute+1e-6 || net > maximum+1e-6 {
			return fail("来源配置毛能力或净份额不守恒/超带宽")
		}
		for row, full := range fulls {
			if math.Abs(g.sources[row]-full) > 1e-6 {
				return fail("配置来源与计划不一致")
			}
		}
	}
	for _, r := range b.transportRoutes {
		if r.from != nil && r.to == nil && !seenRoutes[r] {
			return fail("对外份额缺少独立交付登记")
		}
	}
	for node, sources := range loads {
		full := 0.
		for _, v := range sources {
			full += v
		}
		capacity, e := transport.BeltCapacityPerMinute(int(node.record.ItemID) - 2000)
		if e != nil || int(node.record.ItemID) != 2000+b.setting.BeltLv || node.record.ModelIndex != node.record.ItemID-1966 {
			return fail("真实交付带等级或模型与请求不一致")
		}
		if full > capacity+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("交付物品%d实际带段毛负载%.6f超过真实容量%.6f/min", colors[node], full, capacity)}
		}
	}
	for _, node := range b.all {
		if allowed[node.output] && !allowed[node] && !arms[node] {
			return fail("交付带被无关实体追加来源")
		}
	}
	p.edges = edges
	p.verified = true
	return nil
}
