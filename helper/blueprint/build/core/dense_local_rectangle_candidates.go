package build

// 文件职责：为密铺直连大厂提供不绕圈的有限独立矩形布局候选。
// 使用：球面规格枚举及frame纯边界估计；边界：不修改选区、中心、实体预算、碰撞或共享形状规则，不用于极区形状豁免。

import (
	"DysonSphereBox/helper/blueprint"
	"fmt"
	"math"
)

// denseLocalRectangleAspectKey 职责：传递当前候选布局比例；使用：装箱宽度预算；边界：必须随geometryContext冻结，不以外部ctx覆盖已有候选。
type denseLocalRectangleAspectKey struct{}

// denseLocalRectangleTurnKey 职责：传递独立长行竖放策略；使用：原生格距补偿之前；边界：随候选冻结，不作为形状豁免。
type denseLocalRectangleTurnKey struct{}

// denseLocalRectangleRotateRows 职责：对封闭单行单元作完整刚性旋转以比较竖向装箱。
// 使用：原生格距补偿前的独立候选；边界：保留真实指针、双端姿态和槽位，跨单元链接/共享行不旋转，完整物理检查仍由既有装箱和终检执行。
func denseLocalRectangleRotateRows(units []denseSpatialUnit, height int) int {
	uses := map[*placed]int{}
	rows := map[*transportRow]int{}
	for _, u := range units {
		for _, p := range u.entities {
			uses[p]++
		}
		if u.row != nil {
			rows[u.row]++
		}
	}
	rotated := 0
	for _, u := range units {
		if u.row == nil || rows[u.row] != 1 || u.row.line == nil || len(u.rows) > 0 {
			continue
		}
		lo, hi := bounds(u.entities)
		width, h := hi.x-lo.x, hi.y-lo.y
		if width <= h*1.5 || width > float64(height-9) {
			continue
		}
		members := map[*placed]bool{}
		safe := true
		for _, p := range u.entities {
			members[p] = true
			if uses[p] != 1 {
				safe = false
			}
		}
		for _, p := range u.entities {
			if p.input != nil && !members[p.input] || p.output != nil && !members[p.output] {
				safe = false
			}
		}
		for _, p := range u.row.line.all {
			if !members[p] {
				safe = false
			}
		}
		if !safe {
			continue
		}
		rotatePlacedQuarter(u.entities, 1)
		next, rotatedHigh := bounds(u.entities)
		// 旋转后的原始种子也须位于有效纬向窗口，不能让尚未装箱的种子先映射到窗口外或极点外。
		seedY := max(0., min(lo.y, float64(height-1)-(rotatedHigh.y-next.y)))
		shift(u.entities, lo.x-next.x, seedY-next.y, 0)
		u.row.quarterTurns = (u.row.quarterTurns + 1) % 4
		rotated++
	}
	return rotated
}

// denseLocalRectangleAspects 职责：按显式配置给非极区扩带密铺直连大厂提供独立矩形候选。
// 使用：原窗口及方向已经枚举后；边界：普通、极区、单带和小厂保持原候选，不能用原绕圈结果绕过最终新守卫。
func denseLocalRectangleAspects(req Request, selection densityLatitudeSelection, options SurfaceSearchOptions) []float64 {
	if req.Mode == 1 && req.Density == densityDense && req.Plan.MachineCount >= denseNativeLargeMachineThreshold && selection.tier < 5 && options.ExpandAdjacent {
		if len(options.LocalRectangleAspects) > 0 {
			return append([]float64(nil), options.LocalRectangleAspects...)
		}
	}
	return []float64{0}
}

// deduplicateDenseLocalRectangleBounds 职责：按正式frame和同一纯预算计算去除被半圈clamp后的等价布局。
// 使用：单depth候选枚举末端；边界：原aspect0后备保留，方向/镜像/窗口/深度身份不同不合并，不用宽度推断物理可行。
func deduplicateDenseLocalRectangleBounds(req Request, specs []SurfaceCandidateSpec) ([]SurfaceCandidateSpec, error) {
	if req.SurfaceLayout == nil {
		return specs, nil
	}
	type key struct {
		spec  SurfaceCandidateSpec
		width uint64
		start int
	}
	seen := map[key]bool{}
	out := make([]SurfaceCandidateSpec, 0, len(specs))
	for _, spec := range specs {
		if spec.Strategy != "native-window" || spec.LocalRectangleAspect <= 0 && !spec.OffsetWindow {
			out = append(out, spec)
			continue
		}
		selection, _ := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
		selection = densePolarMirrorSelection(selection, spec.Mirror)
		bands := denseLatitudeAllowedBands(selection, spec.ExpandAdjacent, spec.AdjacentDepth)
		frame, err := newDensePolarLayoutFrame(selection, bands, spec.ExtentRows, spec.ExpandAdjacent, spec.Mirror, true, spec.OffsetWindow)
		if err != nil {
			return nil, err
		}
		if spec.OffsetWindow && !frame.offsetWindow {
			continue
		}
		if spec.LocalRectangleAspect <= 0 {
			out = append(out, spec)
			continue
		}
		frame.longitudeBudget = denseLocalRectangleLongitudeBudget(frame, spec.LocalRectangleAspect)
		identity := spec
		identity.LocalRectangleAspect = 0
		identity.OffsetWindow = false
		k := key{identity, math.Float64bits(frame.longitudeLimit()), frame.start}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, spec)
	}
	return out, nil
}

// denseLocalRectangleLongitudeBudget 职责：把实际纬向高度转换为保守的经向搜索宽度。
// 使用：非极区独立候选；边界：使用最靠赤道的真实格宽、预留两侧接线空间，并限制为半圈内；这里只缩小布局搜索，不接受或伪造实体形状。
func denseLocalRectangleLongitudeBudget(frame *densePolarFrame, aspect float64) float64 {
	if frame == nil || frame.height < 2 || !finitePositive(aspect) {
		return 0
	}
	first := float64(float32(frame.start) * frame.latitudeStep)
	last := float64(float32(frame.start)*frame.latitudeStep + float32(frame.height-1)*frame.latitudeStep)
	near := math.Min(math.Abs(first), math.Abs(last))
	if first <= 0 && last >= 0 {
		near = 0
	}
	// 一行余量覆盖原点和单元模型向赤道外伸，最终仍由全部实体和角点规则决定。
	near = math.Max(0, near-float64(frame.latitudeStep))
	heightMeters := 200.4 * float64(frame.height-1) * float64(frame.latitudeStep)
	metersPerEastGrid := 200.4 * math.Cos(near) * float64(frame.longitudeStep)
	if !finitePositive(metersPerEastGrid) {
		return 0
	}
	width := math.Floor(heightMeters*aspect/metersPerEastGrid) - 16
	halfCircle := math.Floor(math.Pi/float64(frame.longitudeStep)) - 16
	return math.Max(1, math.Min(width, halfCircle))
}

// denseLocalRectanglePreflight 职责：只证明刚性单元的端点跨度在任一合法纬度都无法放入当前布局宽度。
// 使用：窄经向候选正式装箱前；边界：端点包络是必要条件而非碰撞证明，未知缩放保留，不改变实体或物理规则。
func denseLocalRectanglePreflight(frame *densePolarFrame, units []denseSpatialUnit) error {
	limit := frame.longitudeLimit()
	for i, unit := range units {
		lo, hi := vector{math.Inf(1), math.Inf(1), 0}, vector{math.Inf(-1), math.Inf(-1), 0}
		scale := 0.
		for _, p := range unit.entities {
			if scale == 0 && p.nativeUnitScale > 0 {
				scale = p.nativeUnitScale
			}
			for _, v := range []vector{p.a, p.b} {
				lo.x = min(lo.x, v.x)
				lo.y = min(lo.y, v.y)
				hi.x = max(hi.x, v.x)
				hi.y = max(hi.y, v.y)
			}
		}
		if !finitePositive(scale) || math.IsInf(lo.x, 0) {
			continue
		}
		height := hi.y - lo.y
		if height > float64(frame.height-1)+1e-6 {
			return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("局部矩形单元%d端点高度%.3f超过窗口%d行", i, height, frame.height)}
		}
		bestLow := max(0., min(float64(frame.height-1)-height, -float64(frame.start)-height/2))
		minimumScale := frame.unitScale(unit, vector{y: bestLow}, vector{y: bestLow + height})
		width := (hi.x - lo.x) * minimumScale / scale
		if width >= limit+1e-6 {
			owner, count, recipe := -1, 0, 0
			if unit.row != nil {
				owner, count, recipe = unit.row.owner, unit.row.count, unit.row.info.RecipeID
			}
			return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("局部矩形单元%d最优纬度下端点宽度下界%.3f仍超过经向预算%.3f；原端点宽%.3f/高度%.3f，owner%d/count%d/recipe%d，rows%d/buffer%v/entities%d", i, width, limit, hi.x-lo.x, height, owner, count, recipe, len(unit.rows), unit.buffer != nil, len(unit.entities))}
		}
	}
	return nil
}
