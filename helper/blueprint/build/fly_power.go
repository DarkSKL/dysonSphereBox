package build

// 文件职责：为配送模式独立机器行和高层配送器补齐真实供电覆盖。
// 使用：mode2/5 在本地物流装配完成后调用；边界：只放置感应塔，不假定已有电网、发电量或地形允许。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/transport"
)

// flyPowerDistance 职责：把蓝图局部坐标转换为已核对切平面米制距离；使用：补电候选与覆盖；边界：最终球面规则仍需复核。
func flyPowerDistance(a, c vector) float64 {
	x, y, z := (a.x-c.x)*planarGridLength, (a.y-c.y)*planarGridLength, (a.z-c.z)*1.3333333
	return math.Sqrt(x*x + y*y + z*z)
}

// flyPowerCovered 职责：检查当前块中已有感应塔对指定消费者的实际三维覆盖；使用：避免重复补电；边界：高层配送器不能按地面箱体中心估计。
func (b *builder) flyPowerCovered(center vector, existing []*placed) bool {
	radius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return false
	}
	for index, p := range existing {
		if index%64 == 0 && b.checkGeneration("配送供电候选检查") != nil {
			return false
		}
		if p.record.ItemID == 2201 && flyPowerDistance(center, p.a) <= radius-1e-4 {
			return true
		}
	}
	return false
}

// flyPowerCandidateAvailable 职责：在新增电杆前检查最小间距、静态建造盒和物流扫掠净空；使用：固定补电候选；边界：未知原型拒绝候选，最终规则不因此省略。
func (b *builder) flyPowerCandidateAvailable(point vector, existing []*placed) bool {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return false
	}
	encoding := newPlanarRuleEncoding()
	origin := vector{point.x * planarGridLength, .2, point.y * planarGridLength}
	shapes, err := catalog.Shapes(2201, b.models["2201"])
	if err != nil {
		return false
	}
	boxes := make([]placement.Box, 0, len(shapes))
	for _, shape := range shapes {
		box, err := encoding.colliderBox(origin, 90, shape)
		if err != nil {
			return false
		}
		boxes = append(boxes, box)
	}
	world := func(v vector) placement.Vector {
		return placement.Vector{v.x * planarGridLength, v.z*1.3333333 + .2, v.y * planarGridLength}
	}
	for index, p := range existing {
		if index%64 == 0 && b.checkGeneration("配送供电占位检查") != nil {
			return false
		}
		if math.Abs(p.a.x-point.x) > 10 || math.Abs(p.a.y-point.y) > 10 {
			continue
		}
		if surfaceIsPowerNode(p.record.ItemID) && flyPowerDistance(point, p.a) < math.Sqrt(float64(surfacePowerDistanceLimit(2201, p.record.ItemID)))+1e-4 {
			return false
		}
		if surfaceIsBelt(p.record.ItemID) || p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			first, second := world(p.a), world(p.a)
			radius := .25
			if surfaceIsBelt(p.record.ItemID) && p.output != nil && surfaceIsBelt(p.output.record.ItemID) {
				second = world(p.output.a)
			}
			if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
				second = world(p.b)
				radius = .35
			}
			for _, box := range boxes {
				if box.SegmentDistanceSquared(first, second) < radius*radius {
					return false
				}
			}
			continue
		}
		otherShapes, err := catalog.Shapes(p.record.ItemID, p.record.ModelIndex)
		if err != nil {
			return false
		}
		for _, shape := range otherShapes {
			v := world(p.a)
			other, err := encoding.colliderBox(vector{v[0], v[1], v[2]}, p.record.Yaw, shape)
			if err != nil {
				return false
			}
			for _, box := range boxes {
				if box.Overlaps(other) {
					return false
				}
			}
		}
	}
	return true
}

// addFlyPower 职责：按固定顺序选择覆盖消费者的合法感应塔位置；使用：配送端和机器行补电；边界：有限候选无解时失败，不把过远的杆当成覆盖。
func (b *builder) addFlyPower(center vector, begin int) error {
	if err := b.checkGeneration("配送供电补齐"); err != nil {
		return err
	}
	if b.setting.NeedPower == nil || !*b.setting.NeedPower || b.flyPowerCovered(center, b.all[begin:]) {
		return nil
	}
	radius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return err
	}
	for _, offset := range []vector{{4, 4, 0}, {-4, 4, 0}, {4, -4, 0}, {-4, -4, 0}, {0, 4, 0}, {0, -4, 0}, {4, 0, 0}, {-4, 0, 0}, {6, 0, 0}, {-6, 0, 0}, {0, 6, 0}, {0, -6, 0}, {5, 3, 0}, {-5, 3, 0}, {5, -3, 0}, {-5, -3, 0}, {3, 5, 0}, {-3, 5, 0}, {3, -5, 0}, {-3, -5, 0}} {
		if err := b.checkGeneration("配送供电候选"); err != nil {
			return err
		}
		point := vector{center.x + offset.x, center.y + offset.y, 0}
		if flyPowerDistance(center, point) > radius-1e-4 || !b.flyPowerCandidateAvailable(point, b.all[begin:]) {
			continue
		}
		b.make(2201, point.x, point.y, 0)
		return nil
	}
	if err := b.checkGeneration("配送供电候选"); err != nil {
		return err
	}
	return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("产线 %d 的配送消费者(%.2f,%.2f,%.2f)在20个供电候选内无法同时满足%.1f米覆盖、节点间距和建造盒净空", b.owner, center.x, center.y, center.z, radius)}
}

// ensureFlyPower 职责：为独立配送端点复用或创建感应塔；使用：简单几何调用；边界：完整行装配使用带错误返回的 completeFlyPower。
func (b *builder) ensureFlyPower(center vector) { _ = b.addFlyPower(center, 0) }

// completeFlyPower 职责：逐个覆盖本地机器、分拣器和高层配送端；使用：mode2/5 替代跨行复用电杆；边界：不借用随后会独立平移的其它产线块。
func (b *builder) completeFlyPower(begin int) error {
	end := len(b.all)
	for _, p := range b.all[begin:end] {
		if err := b.checkGeneration("配送消费者供电"); err != nil {
			return err
		}
		if !finalPowerConsumer(p.record) {
			continue
		}
		center := p.a
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			center = vector{(p.a.x + p.b.x) / 2, (p.a.y + p.b.y) / 2, (p.a.z + p.b.z) / 2}
		}
		if err := b.addFlyPower(center, begin); err != nil {
			return err
		}
	}
	return nil
}
