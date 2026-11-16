package build

// 文件职责：还原参考图的真实货物取放点，并在最终带方向与球面位置确定后重标定分拣偏移。
// 使用：参考臂装配及最终规则前；边界：保留原臂姿态，只在同物料轨道内重绑，未知合流/边界和过大误差明确失败。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/density"
)

// referenceCargoTolerance 职责：限制重新绑定的实际采样误差；使用：约0.06米采样的最近点检查；边界：不放宽机器碰撞或分拣姿态。
const referenceCargoTolerance = .08

// referenceCargoAnchorRadius 职责：限定可能覆盖目标的带段锚点；使用：避免远处汇流口误阻断局部采样；边界：合法带段半长sqrt(5.3)/2、0.15米货物抬升及0.08米误差之和小于1.5米。
const referenceCargoAnchorRadius = 1.5

// denseCargoTarget 职责：保存相对于当前带锚点的物料目标；使用：布局平移、旋转和最终重绑；边界：track在创建后只读，relative按值复制，不能跨物料搜索。
type denseCargoTarget struct {
	// enabled 职责：区分真实参考目标与普通臂；使用：最终入口；边界：普通臂不变。
	enabled bool
	// output 职责：标识机器向带出货；使用：选择OutputOffset或InputOffset；边界：一个参考臂只有一侧连接带。
	output bool
	// relative 职责：保存目标对当前带锚点的装配坐标差；使用：平移自动跟随、旋转同步；边界：不是原始分拣器端点。
	relative vector
	// track 职责：保留该物料轨道的允许锚点；使用：裁剪保护与最终邻域搜索；边界：创建时复制，之后不可改变底层切片。
	track []*placed
	// coater 职责：保留该输入轨道的真实喷涂机；使用：重绑后检查取料仍在喷涂下游；边界：输出及无喷涂臂为空。
	coater *placed
}

// setDenseCargoTarget 职责：记录来源货物点与当前真实锚点的相对关系；使用：原始臂及第三物料变体；边界：不复用源偏移作为新带采样。
func setDenseCargoTarget(arm, anchor *placed, track []*placed, target vector, output bool) {
	arm.densityCargo = denseCargoTarget{enabled: true, output: output, relative: vector{target.x - anchor.a.x, target.y - anchor.a.y, target.z - anchor.a.z}, track: append([]*placed(nil), track...)}
}

// denseCargoLocalTarget 职责：读取随当前绑定锚点移动的实际取放目标；使用：第三槽派生和最终核验；边界：缺失元数据或带端必须失败。
func denseCargoLocalTarget(arm *placed) (vector, error) {
	if !arm.densityCargo.enabled {
		return vector{}, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "参考臂缺少真实货物目标"}
	}
	anchor := arm.input
	if arm.densityCargo.output {
		anchor = arm.output
	}
	if anchor == nil || !surfaceIsBelt(anchor.record.ItemID) {
		return vector{}, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "参考臂当前带锚点无效"}
	}
	d := arm.densityCargo.relative
	return vector{anchor.a.x + d.x, anchor.a.y + d.y, anchor.a.z + d.z}, nil
}

// projectCargo 职责：投影来源实际货物中心而非臂端点；使用：源图采样转换；边界：保留径向高度和0.15米货物抬升，不将其压到节点中心。
func (frame referenceProjector) projectCargo(point density.CargoPoint) vector {
	result := vector{z: (math.Sqrt(point[0]*point[0]+point[1]*point[1]+point[2]*point[2]) - 200.2) / 1.3333333}
	for axis, value := range point {
		result.x += (value - frame.origin[axis]) * frame.east[axis] / planarGridLength
		result.y += (value - frame.origin[axis]) * frame.north[axis] / planarGridLength
	}
	return result
}

// referenceCargoTargets 职责：从裁切参考的真实世界位置和旧offset还原每根机器臂的货物点；使用：参考单元装配；边界：来源锚点采用研究基线0并声明所有跨界带引用。
func referenceCargoTargets(unit density.ReferenceUnit, frame referenceProjector) (map[int32]vector, error) {
	encoding := &surfaceEncoding{areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: 0}}
	world, err := encoding.surfaceWorldPositions(unit.Records)
	if err != nil {
		return nil, err
	}
	positions := make([]density.CargoPoint, len(world))
	byIndex := map[int32]blueprint.Building{}
	for i, p := range world {
		positions[i] = density.CargoPoint{p.x, p.y, p.z}
		byIndex[unit.Records[i].Index] = unit.Records[i]
	}
	boundaries, err := unit.CargoBoundaries()
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: err.Error()}
	}
	sampler, err := density.NewCargoSampler(unit.Records, positions, density.CargoOptions{Boundaries: boundaries})
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: err.Error()}
	}
	targets := map[int32]vector{}
	for _, arm := range unit.Records {
		if arm.ItemID < 2011 || arm.ItemID > 2014 {
			continue
		}
		anchor, offset := arm.InputObjIdx, int(arm.InputOffset)
		if !surfaceIsBelt(byIndex[anchor].ItemID) {
			anchor, offset = arm.OutputObjIdx, int(arm.OutputOffset)
		}
		point, err := sampler.PointAt(anchor, offset)
		if err != nil {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: fmt.Sprintf("参考分拣器%d的货物点无法还原：%v", arm.Index, err)}
		}
		targets[arm.Index] = frame.projectCargo(point)
	}
	return targets, nil
}

// preserveDenseCargoNeighborhoods 职责：保留取放目标附近必要的采样邻域；使用：所有密铺尾带裁剪入口；边界：只保护目标周围及两跳几何，不保留与货物目标无关的长尾。
func preserveDenseCargoNeighborhoods(all []*placed, required map[*placed]bool) {
	alive := make(map[*placed]bool, len(all))
	for _, entity := range all {
		alive[entity] = true
	}
	for _, arm := range all {
		if !arm.densityCargo.enabled {
			continue
		}
		target, err := denseCargoLocalTarget(arm)
		if err != nil {
			continue
		}
		track := arm.densityCargo.track
		nearest, distance := -1, math.Inf(1)
		for i, node := range track {
			if !alive[node] {
				continue
			}
			delta := math.Pow((node.a.x-target.x)*planarGridLength, 2) + math.Pow((node.a.y-target.y)*planarGridLength, 2) + math.Pow((node.a.z-target.z)*1.3333333, 2)
			if delta < distance {
				nearest, distance = i, delta
			}
		}
		if nearest < 0 {
			continue
		}
		for i := max(0, nearest-2); i < min(len(track), nearest+3); i++ {
			required[track[i]] = true
		}
	}
}

// rebindReferenceCargo 职责：从最终记录重建货物曲线并事务式重绑参考臂；使用：供电清理后、最终规则前；边界：不跨物料、截断offset或忽略未知邻域。
func (b *builder) rebindReferenceCargo(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) error {
	if len(records) != len(b.all) {
		return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "货物重绑的实体与记录数量不一致"}
	}
	indices := make(map[*placed]int, len(b.all))
	byRecordIndex := make(map[int32]*placed, len(b.all))
	arms := []int{}
	for i, p := range b.all {
		indices[p] = i
		byRecordIndex[records[i].Index] = p
		if p.densityPattern == "reference-low-arm" && !p.densityCargo.enabled {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "参考密铺臂缺少采样目标"}
		}
		if p.densityCargo.enabled {
			arms = append(arms, i)
		}
	}
	if len(arms) == 0 {
		return nil
	}
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	world, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	positions := make([]density.CargoPoint, len(world))
	for i, p := range world {
		positions[i] = density.CargoPoint{p.x, p.y, p.z}
	}
	options := density.CargoOptions{}
	if encoding.planar {
		up := density.CargoPoint{0, 1, 0}
		options.PlanarUp = &up
	}
	sampler, err := density.NewCargoSampler(records, positions, options)
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: err.Error()}
	}
	targetRecords := make([]blueprint.Building, len(arms))
	targets := make([]vector, len(arms))
	for n, i := range arms {
		arm := b.all[i]
		target, err := denseCargoLocalTarget(arm)
		if err != nil {
			return err
		}
		targets[n] = target
		anchor := arm.input
		if arm.densityCargo.output {
			anchor = arm.output
		}
		index, ok := indices[anchor]
		if !ok {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "参考臂当前锚点已被删除"}
		}
		r := records[index]
		if encoding.relocation != nil {
			if index >= len(encoding.relocation.sourceRecords) {
				return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "球面重定位缺少原始货物锚点记录"}
			}
			// relative属于未改写的内部装配视图，先在来源坐标还原目标，再一起旋转。
			r = encoding.relocation.sourceRecords[index]
		}
		delta := arm.densityCargo.relative
		scale := 1.
		if len(encoding.longitudeScales) > int(r.AreaIndex) {
			scale = encoding.longitudeScales[r.AreaIndex]
		}
		r.LocalOffset.X = float32(float64(r.LocalOffset.X) + delta.x*scale)
		r.LocalOffset.Y = float32(float64(r.LocalOffset.Y) + delta.y)
		r.LocalOffset.Z = float32(float64(r.LocalOffset.Z) + delta.z)
		if encoding.relocation != nil {
			r.LocalOffset, r.Yaw = encoding.relocation.mapPoint(r.LocalOffset, r.Yaw)
		}
		targetRecords[n] = r
	}
	targetWorld, err := encoding.surfaceWorldPositions(targetRecords)
	if err != nil {
		return err
	}
	type decision struct {
		index   int
		anchor  *placed
		binding density.CargoBinding
		target  vector
	}
	decisions := make([]decision, 0, len(arms))
	maximumError := 0.
	for n, i := range arms {
		if err := b.checkGeneration("参考分拣器货物采样重标定"); err != nil {
			return err
		}
		arm := b.all[i]
		goal := targetWorld[n]
		allowed := []int32{}
		for _, node := range arm.densityCargo.track {
			index, alive := indices[node]
			if !alive {
				continue
			}
			p := world[index]
			distance := math.Sqrt((p.x-goal.x)*(p.x-goal.x) + (p.y-goal.y)*(p.y-goal.y) + (p.z-goal.z)*(p.z-goal.z))
			if distance <= referenceCargoAnchorRadius {
				allowed = append(allowed, records[index].Index)
			}
		}
		sort.Slice(allowed, func(i, j int) bool { return allowed[i] < allowed[j] })
		if len(allowed) == 0 {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: fmt.Sprintf("分拣器%d的实际货物目标附近没有存活的同物料锚点", records[i].Index)}
		}
		binding, err := sampler.Nearest(allowed, density.CargoPoint{goal.x, goal.y, goal.z}, referenceCargoTolerance)
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: fmt.Sprintf("分拣器%d在本物料轨道重标定失败：%v", records[i].Index, err)}
		}
		anchor, exists := byRecordIndex[binding.AnchorIndex]
		if !exists {
			return &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "货物采样返回了无效锚点编号"}
		}
		if err := b.validateDenseCargoSprayOrder(arm, anchor); err != nil {
			return err
		}
		maximumError = math.Max(maximumError, binding.Distance)
		decisions = append(decisions, decision{index: i, anchor: anchor, binding: binding, target: targets[n]})
	}
	for _, choice := range decisions {
		arm := b.all[choice.index]
		r := &records[choice.index]
		if arm.densityCargo.output {
			arm.output = choice.anchor
			arm.record.OutputOffset = choice.binding.Offset
			r.OutputObjIdx = choice.binding.AnchorIndex
			r.OutputOffset = choice.binding.Offset
		} else {
			arm.input = choice.anchor
			arm.record.InputOffset = choice.binding.Offset
			r.InputObjIdx = choice.binding.AnchorIndex
			r.InputOffset = choice.binding.Offset
		}
		arm.densityCargo.relative = vector{choice.target.x - choice.anchor.a.x, choice.target.y - choice.anchor.a.y, choice.target.z - choice.anchor.a.z}
	}
	// 带锚点影响游戏对分拣器端点的校正；旧预审覆盖的是重绑前引用，不能复用其动态几何缓存。
	if encoding.plan != nil {
		encoding.plan.CollisionCoverage = ""
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-cargo-offsets", Status: "passed", Message: fmt.Sprintf("%d根参考臂按最终有向物料带重新定位货物采样，最大误差%.4f米（上限%.2f米）；保留臂姿态、等级和同物料约束", len(decisions), maximumError, referenceCargoTolerance)})
	}
	return nil
}

// validateDenseCargoSprayOrder 职责：证明实际取料段在本轨喷涂节点的下游；使用：重标定提交前；边界：至少间隔两个节点，禁止负offset落回喷涂节点前的半段。
func (b *builder) validateDenseCargoSprayOrder(arm, anchor *placed) error {
	coater := arm.densityCargo.coater
	if coater == nil || arm.densityCargo.output {
		return nil
	}
	host, err := b.denseCargoCoaterHost(coater)
	if err != nil {
		return err
	}
	visited := map[*placed]bool{}
	for steps, node := 0, host; node != nil && surfaceIsBelt(node.record.ItemID) && !visited[node]; steps, node = steps+1, node.output {
		if node == anchor && steps >= 2 {
			return nil
		}
		visited[node] = true
	}
	return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "重标定取料段无法证明位于实际喷涂节点下游"}
}

// denseCargoCoaterHost 职责：找到真实喷涂机的唯一存活宿主带；使用：生成余量与原生五样本窗口顺序核验；边界：内部装配坐标仅用于宿主身份，不替代最终货物路径索引。
func (b *builder) denseCargoCoaterHost(coater *placed) (*placed, error) {
	var host *placed
	coaterAlive := false
	for _, entity := range b.all {
		if entity == coater && entity.record.ItemID == 2313 {
			coaterAlive = true
		}
		if !surfaceIsBelt(entity.record.ItemID) || !flySamePoint(entity.a, coater.a) {
			continue
		}
		if host != nil {
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_CARGO", Message: "喷涂节点存在不唯一的重合带"}
		}
		host = entity
	}
	if !coaterAlive || host == nil {
		return nil, &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "参考轨道的喷涂机或宿主带已不属于最终蓝图"}
	}
	return host, nil
}
