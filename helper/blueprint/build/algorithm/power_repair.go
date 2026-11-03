package build

// 文件职责：在最终球面坐标固定后补齐消费者覆盖并连接内部电网。
// 使用：全部模式在删冗余塔之前调用；边界：只增加合法地面塔，失败或取消撤回全部新增实体，不代表外部电源已经接入。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/transport"
)

// newFinalPowerProbe 职责：从最终记录恢复独立装配库存供候选补电；使用：装箱预审；边界：保留真实引用和双端，不修改原 builder 的实体、区间或图。
func (b *builder) newFinalPowerProbe(encoding *surfaceEncoding, records []blueprint.Building) (*builder, error) {
	probe := &builder{setting: b.setting, models: b.models, owner: -1}
	byIndex := make(map[int32]*placed, len(records))
	for _, record := range records {
		area := int(record.AreaIndex)
		if area < 0 || area >= len(encoding.areas) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "供电候选预审遇到无效 Area"}
		}
		point := func(local blueprint.Point) vector {
			x := float64(local.X)
			if len(encoding.longitudeScales) > 0 {
				x /= encoding.longitudeScales[area]
			}
			return vector{x, float64(local.Y) + float64(encoding.latitudeOffsets[area]), float64(local.Z)}
		}
		entity := &placed{record: record, a: point(record.LocalOffset), b: point(record.LocalOffset2), owner: -1}
		probe.all = append(probe.all, entity)
		byIndex[record.Index] = entity
		encoding.byBuilding[entity] = area
	}
	for _, entity := range probe.all {
		entity.input = byIndex[entity.record.InputObjIdx]
		entity.output = byIndex[entity.record.OutputObjIdx]
	}
	return probe, nil
}

// repairFinalPower 职责：原子修复最终坐标下的覆盖遗漏和断开的电塔网络；使用：共同编码出口及装箱候选预审；边界：不移动消费者、改变 Area 或放宽碰撞、半径与塔距。
func (b *builder) repairFinalPower(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (added int, err error) {
	if b.setting.NeedPower != nil && !*b.setting.NeedPower {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := finalPowerPruneCancelled(ctx); err != nil {
		return 0, err
	}
	rules := encoding
	if rules == nil {
		rules = newPlanarRuleEncoding()
	}
	networks, coverageErr := validateFinalPowerCoverage(ctx, rules, records)
	if coverageErr == nil && networks <= 1 {
		return 0, nil
	}
	if coverageErr != nil {
		failure, valid := coverageErr.(*blueprint.Error)
		if !valid || failure.Code != "GEN_E_POWER_COVERAGE" {
			return 0, coverageErr
		}
	}
	begin, previousEncoding := len(b.all), b.connectedPowerEncoding
	owner, previousContext := b.owner, b.generationCtx
	var previousAreas map[*placed]int
	if encoding != nil {
		previousAreas = make(map[*placed]int, len(encoding.byBuilding))
		for entity, area := range encoding.byBuilding {
			previousAreas[entity] = area
		}
	}
	b.owner, b.generationCtx, b.connectedPowerEncoding = -1, ctx, encoding
	defer func() {
		b.owner, b.generationCtx, b.connectedPowerEncoding = owner, previousContext, previousEncoding
		if err != nil {
			b.all = b.all[:begin]
			if encoding != nil {
				encoding.byBuilding = previousAreas
			}
			added = 0
		}
	}()
	if coverageErr != nil {
		if err = b.completeConnectedPower(ctx); err != nil {
			return 0, err
		}
	}
	index, err := newConnectedPowerIndexForEncoding(ctx, b, encoding)
	if err != nil {
		return 0, err
	}
	if err = b.bridgeFinalPower(index); err != nil {
		return 0, err
	}
	final, _, _, err := b.records()
	if err != nil {
		return 0, err
	}
	if encoding != nil {
		if err = encoding.applyRecords(b.all, final); err != nil {
			return 0, err
		}
	}
	networks, err = validateFinalPowerCoverage(ctx, rules, final)
	if err != nil {
		return 0, err
	}
	if networks > 1 {
		return 0, &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: fmt.Sprintf("最终供电修复后仍有 %d 个互不连通的内部电网", networks)}
	}
	if err = finalPowerPruneCancelled(ctx); err != nil {
		return 0, err
	}
	added = len(b.all) - begin
	if encoding != nil && added > 0 {
		// 新塔改变占位集合，必须撤销新增之前的预检标记，再由共同终检完整检查。
		encoding.plan.StaticCollidersChecked = false
		encoding.plan.CollisionCoverage = "none"
		encoding.plan.ColliderGameVersion = ""
		counts := make(map[int]int)
		for _, area := range encoding.byBuilding {
			counts[area]++
		}
		for i := range encoding.plan.Bands {
			encoding.plan.Bands[i].BuildingCount = counts[encoding.plan.Bands[i].AreaIndex]
		}
	}
	return added, nil
}

// finalPowerComponents 职责：按原版连接距离建立现存感应塔分量；使用：桥接每个独立网络；边界：顺序由建筑库存决定，不使用随机 map 遍历选路。
func finalPowerComponents(index *connectedPowerIndex, distance float64) ([][]*placed, error) {
	poles := make([]*placed, 0, len(index.poleWorlds))
	for _, entity := range index.builder.all {
		if entity.record.ItemID == 2201 {
			poles = append(poles, entity)
		}
	}
	parents := make([]int, len(poles))
	for i := range parents {
		parents[i] = i
	}
	root := func(i int) int {
		for parents[i] != i {
			parents[i] = parents[parents[i]]
			i = parents[i]
		}
		return i
	}
	for i, pole := range poles {
		if err := finalPowerPruneCancelled(index.ctx); err != nil {
			return nil, err
		}
		for j := 0; j < i; j++ {
			if finalPowerWorldDistanceSquared(index.poleWorlds[pole], index.poleWorlds[poles[j]]) <= distance*distance {
				parents[root(i)] = root(j)
			}
		}
	}
	groups, byRoot := make([][]*placed, 0), make(map[int]int)
	for i, pole := range poles {
		r := root(i)
		group, exists := byRoot[r]
		if !exists {
			group = len(groups)
			byRoot[r] = group
			groups = append(groups, nil)
		}
		groups[group] = append(groups[group], pole)
	}
	return groups, nil
}

// finalPowerWorldDistanceSquared 职责：复用最终记录的单精度电网距离规则；使用：桥接边和分量；边界：不能以装配平面距离代替球面弦长。
func finalPowerWorldDistanceSquared(a, b placement.Vector) float64 {
	return float64(surfacePowerDistanceSquared(vector{a[0], a[1], a[2]}, vector{b[0], b[1], b[2]}))
}

// bridgeFinalPower 职责：选择最近的断开网络并增加合法连接链；使用：真实覆盖补漏之后；边界：有界搜索无路则整个修复失败，不返回需要玩家补内部连接的成功结果。
func (b *builder) bridgeFinalPower(index *connectedPowerIndex) error {
	distance, err := transport.TeslaPowerConnectDistance()
	if err != nil {
		return err
	}
	for {
		groups, err := finalPowerComponents(index, distance)
		if err != nil || len(groups) <= 1 {
			return err
		}
		var first, second *placed
		best := math.Inf(1)
		for i, group := range groups {
			if err := finalPowerPruneCancelled(index.ctx); err != nil {
				return err
			}
			for _, a := range group {
				for j := 0; j < i; j++ {
					for _, c := range groups[j] {
						d := finalPowerWorldDistanceSquared(index.poleWorlds[a], index.poleWorlds[c])
						if d < best {
							best, first, second = d, a, c
						}
					}
				}
			}
		}
		if first == nil || second == nil {
			return &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: "无法解析需要桥接的内部电网"}
		}
		if err := b.bridgeFinalPowerPair(index, first, second, distance); err != nil {
			return err
		}
	}
}

// bridgeFinalPowerPair 职责：在两根已确定的塔之间搜索地面连接链；使用：最近网络桥接；边界：每步严格推进且服从原塔距、静态盒、带臂净空、Area 与建筑总数预算。
func (b *builder) bridgeFinalPowerPair(index *connectedPowerIndex, first, target *placed, distance float64) error {
	current, targetWorld := first, index.poleWorlds[target]
	offsets := connectedPowerCandidateOffsets(index.coverRadius)
	for step := 0; step < 512; step++ {
		if err := finalPowerPruneCancelled(index.ctx); err != nil {
			return err
		}
		currentWorld := index.poleWorlds[current]
		remaining := math.Sqrt(finalPowerWorldDistanceSquared(currentWorld, targetWorld))
		if remaining <= distance {
			return nil
		}
		t := math.Min(.5, (distance-3)/remaining)
		ideal := vector{current.a.x + (target.a.x-current.a.x)*t, current.a.y + (target.a.y-current.a.y)*t, 0}
		if !index.encoding.planar {
			idealWorld := currentWorld.Scale(1 - t).Add(targetWorld.Scale(t))
			var err error
			ideal, err = index.finalPowerPointNear(idealWorld)
			if err != nil {
				return err
			}
		}
		ideal.x, ideal.y = math.Round(ideal.x*2)/2, math.Round(ideal.y*2)/2
		var selected vector
		best := remaining*remaining - 1e-4
		found := false
		for number, offset := range offsets {
			if number%64 == 0 {
				if err := finalPowerPruneCancelled(index.ctx); err != nil {
					return err
				}
			}
			point := vector{ideal.x + offset.x, ideal.y + offset.y, 0}
			if !index.insideArea(point) {
				continue
			}
			world, err := index.world(point)
			if err != nil {
				return err
			}
			if finalPowerWorldDistanceSquared(world, currentWorld) > (distance-.01)*(distance-.01) {
				continue
			}
			score := finalPowerWorldDistanceSquared(world, targetWorld)
			if score >= best {
				continue
			}
			available, err := index.available(point)
			if err != nil {
				return err
			}
			if available {
				selected, best, found = point, score, true
			}
		}
		if !found {
			return &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: fmt.Sprintf("内部电网桥接无合法地面路径：塔(%.2f,%.2f)至(%.2f,%.2f)，剩余%.2f米；未放宽碰撞、覆盖或塔距", current.a.x, current.a.y, target.a.x, target.a.y, remaining)}
		}
		if len(b.all) >= blueprint.MaxBuildingCount {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "内部电网桥接超过建筑总数预算"}
		}
		current = b.make(2201, selected.x, selected.y, 0)
		if err := index.add(current); err != nil {
			return err
		}
	}
	return &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: "内部电网桥接超过有限搜索预算"}
}

// finalPowerPointNear 职责：把短球面弦上的目标反算到实际 Area 网格；使用：高纬或跨纬带桥接；边界：不直接插值跨 Area 的装配 X，邻圈没有可编码点时明确失败。
func (index *connectedPowerIndex) finalPowerPointNear(world placement.Vector) (vector, error) {
	latitude := math.Atan2(world[1], math.Hypot(world[0], world[2]))
	longitude := math.Atan2(world[0], -world[2])
	best, selected := math.Inf(1), vector{}
	for areaNumber, area := range index.encoding.areas {
		origins, err := index.encoding.surfaceWorldPositions([]blueprint.Building{{AreaIndex: int8(areaNumber)}})
		if err != nil {
			return vector{}, err
		}
		origin := origins[0]
		originLatitude := math.Atan2(origin.y, math.Hypot(origin.x, origin.z))
		originLongitude := math.Atan2(origin.x, -origin.z)
		xStep := 2 * math.Pi / float64(int(area.AreaSegments)*5)
		y := math.Max(0, math.Min(float64(area.Height-1), (latitude-originLatitude)/(2*math.Pi/1000)))
		for _, turn := range []float64{0, -2 * math.Pi, 2 * math.Pi} {
			x := (longitude - originLongitude + turn) / xStep
			if x < -1e-3 || x > float64(area.Width-1)+1e-3 {
				continue
			}
			x = math.Max(0, math.Min(float64(area.Width-1), x))
			if len(index.encoding.longitudeScales) > 0 {
				x /= index.encoding.longitudeScales[areaNumber]
			}
			point := vector{x, y + float64(index.encoding.latitudeOffsets[areaNumber]), 0}
			candidate, err := index.world(point)
			if err != nil {
				return vector{}, err
			}
			delta := candidate.Sub(world)
			if d := delta.Dot(delta); d < best {
				best, selected = d, point
			}
		}
	}
	if math.IsInf(best, 1) {
		return vector{}, &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: "球面连接目标落在全部可编码 Area 之外"}
	}
	return selected, nil
}
