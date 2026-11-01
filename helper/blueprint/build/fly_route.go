package build

// 文件职责：在配送接驳路径提交前检查局部带占位和真实静态建造盒。
// 使用：mode2/5 端点装配；边界：只审核当前不可分割机器行，最终球面规则仍须完整执行。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// validateFlyPathCandidate 职责：拒绝占用已有物流或穿过建筑的接驳候选；使用：byPath 创建实体之前；边界：只允许首段复用同一个真实 link，分流器实体接驳由独立端口规则核验。
func (b *builder) validateFlyPathCandidate(points []vector, link, splitter *placed, existing []*placed) error {
	if err := b.checkGeneration("配送路径预提交"); err != nil {
		return err
	}
	if len(points) < 2 || len(points) > 4096 {
		return &blueprint.Error{Code: "GEN_E_ROUTE", Message: "配送接驳路径为空或超过单路4096格候选预算"}
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	encoding := newPlanarRuleEncoding()
	world := func(p vector) placement.Vector {
		return placement.Vector{p.x * planarGridLength, p.z*1.3333333 + .4, p.y * planarGridLength}
	}
	low, high := points[0], points[0]
	for _, p := range points {
		low.x = math.Min(low.x, p.x)
		low.y = math.Min(low.y, p.y)
		high.x = math.Max(high.x, p.x)
		high.y = math.Max(high.y, p.y)
	}
	for index, p := range existing {
		if index%64 == 0 {
			if err := b.checkGeneration("配送路径占位检查"); err != nil {
				return err
			}
		}
		if p == splitter {
			continue
		}
		if surfaceIsBelt(p.record.ItemID) {
			end := p
			if p.output != nil && surfaceIsBelt(p.output.record.ItemID) {
				end = p.output
			}
			if math.Max(p.a.x, end.a.x) < low.x-1 || math.Min(p.a.x, end.a.x) > high.x+1 || math.Max(p.a.y, end.a.y) < low.y-1 || math.Min(p.a.y, end.a.y) > high.y+1 {
				continue
			}
			old := surfaceBeltSpan{ends: [2]placement.Vector{world(p.a), world(end.a)}}
			for i := 1; i < len(points); i++ {
				if i%64 == 0 {
					if err := b.checkGeneration("配送路径交叉检查"); err != nil {
						return err
					}
				}
				if i == 1 && (p == link || end == link) {
					continue
				}
				candidate := surfaceBeltSpan{ends: [2]placement.Vector{world(points[i-1]), world(points[i])}}
				if surfaceSpanDistanceSquared(candidate, old) < 4*.23*.23-1e-6 {
					return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("产线 %d 的配送接驳与已占用带段相交，候选点(%.2f,%.2f)", link.owner, points[i].x, points[i].y)}
				}
			}
			continue
		}
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			continue
		}
		if p.a.x < low.x-10 || p.a.x > high.x+10 || p.a.y < low.y-10 || p.a.y > high.y+10 {
			continue
		}
		shapes, err := catalog.Shapes(p.record.ItemID, p.record.ModelIndex)
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for _, shape := range shapes {
			origin := world(p.a)
			origin[1] -= .2
			box, err := encoding.colliderBox(vector{origin[0], origin[1], origin[2]}, p.record.Yaw, shape)
			if err != nil {
				return err
			}
			for i := 1; i < len(points); i++ {
				if i%64 == 0 {
					if err := b.checkGeneration("配送路径建造盒检查"); err != nil {
						return err
					}
				}
				if box.SegmentDistanceSquared(world(points[i-1]), world(points[i])) < .23*.23 {
					return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("产线 %d 的配送接驳格(%.2f,%.2f)侵占物品 %d 于(%.2f,%.2f,%.2f)的建造盒", link.owner, points[i].x, points[i].y, p.record.ItemID, p.a.x, p.a.y, p.a.z)}
				}
			}
		}
	}
	return nil
}
