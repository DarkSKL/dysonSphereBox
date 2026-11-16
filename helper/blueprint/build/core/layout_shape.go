package build

// 文件职责：估计有限列高候选，并以最终实体的共同物理包络度量形状。
// 使用：装配前估计列高；终检后统一计算单区、多区和原生root整厂的真实形状。
// 边界：试装不运行最终编码、不改变机器或流量；候选仍需通过原有完整规则，不承诺数学全局最优。

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// layoutHeightCandidates 职责：从本次真实机器、端口和配送设施占地估计矩形候选；使用：有限布局搜索；边界：平面保留旧列高，球面直连保留低纬宽排候选，取消和试装错误不得被忽略。
func (b *builder) layoutHeightCandidates(ctx context.Context, req Request, dataset *quant.Dataset, connected bool) ([]float64, error) {
	probe, err := b.newCandidateBuilder()
	if err != nil {
		return nil, err
	}
	probe.generationCtx, probe.flowGraph = ctx, b.flowGraph
	probe.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	if connected {
		probe.connectedCompactBelts = true
		err = probe.prepareConnectedRows(ctx, req, dataset, math.Inf(1), false)
	} else {
		err = probe.assembleFlyLayout(ctx, req, dataset, flyLayoutVariant{HeightLimit: math.Inf(1)})
	}
	if err != nil {
		return nil, err
	}
	// 同一产线的多行在试装中上下相邻；按产线而不是整个长列求面积，
	// 避免最长机器行把所有较短产线的空白都计成必要占地。
	blocks := make([][]*placed, len(req.Plan.Productions))
	for _, p := range probe.all {
		if p.owner >= 0 && p.owner < len(blocks) {
			blocks[p.owner] = append(blocks[p.owner], p)
		}
	}
	area := 0.
	for _, block := range blocks {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "布局形状估计已取消或超时"}
		}
		if len(block) == 0 {
			continue
		}
		low, high := bounds(block)
		area += (math.Ceil(high.x-low.x) + 2) * (math.Ceil(high.y-low.y) + 2)
	}
	fallback := b.setting.BlockWidthLimit
	if connected && req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
		// 水平行架还须容纳路由扩展，不能用整个赤道纬带高度估算：
		// 行架取其四分之三，为锚点选择及高架曲率留出上下余量。
		for _, band := range surfaceLatitudeBands(packingSegment(req.SurfaceLayout)) {
			if band.low <= 0 && band.high >= 0 {
				usableHeight := math.Max(1, math.Floor(float64(band.high-band.low+1)*.75))
				fallback = math.Max(math.Sqrt(area)*1.6, math.Ceil(area*1.25/usableHeight))
				break
			}
		}
	}
	return rectangularHeightCandidates(area, fallback), nil
}

// rectangularHeightCandidates 职责：提供固定最多四种列高；使用：真实面积估计之后；边界：只为软形状目标，不将旧平面列高作为安全规则或固定最小高度。
func rectangularHeightCandidates(area, legacy float64) []float64 {
	ideal := math.Max(10, math.Ceil(math.Sqrt(math.Max(1, area))))
	values := []float64{ideal, math.Ceil(ideal * .8), math.Ceil(ideal * 1.25), math.Max(60, legacy)}
	result := make([]float64, 0, len(values))
	for _, value := range values {
		duplicate := false
		for _, previous := range result {
			duplicate = duplicate || previous == value
		}
		if !duplicate {
			result = append(result, value)
		}
	}
	return result
}

// updateLayoutShapeMetrics 职责：从Area元数据计算装箱阶段的临时形状；使用：尚无共同世界实体的局部估计；边界：面积加权值不是整厂物理形状，正式终检必须由updatePhysicalLayoutShapeMetrics覆盖，不能据此验收。
func updateLayoutShapeMetrics(summary *LogisticsSummary, areas []blueprint.Area) {
	weightedRatio, total := 0., 0.
	for _, area := range areas {
		width, height := float64(area.Width), float64(area.Height)
		if width <= 0 || height <= 0 {
			continue
		}
		size := width * height
		weightedRatio += size * math.Max(width, height) / math.Min(width, height)
		total += size
	}
	summary.Metrics.EnvelopeAspectRatio = 0
	if total > 0 {
		summary.Metrics.EnvelopeAspectRatio = weightedRatio / total
	}
	summary.Metrics.ShapeObjective = layoutShapeObjective(summary.Metrics)
}

// layoutShapeObjective 职责：将普通布局形状评分交给共同策略；使用：完整候选评分和摘要；边界：软代价不代替1.8验收分档，最终比较须调用BetterLayoutCandidate，缺失度量不能成为最优。
func layoutShapeObjective(metrics LayoutMetrics) float64 {
	return layoutShapeCost(metrics.EnvelopeArea, metrics.EnvelopeAspectRatio)
}

// physicalShapePoint 职责：保存共同米制平面中的实体采样；使用：凸包与最小面积矩形；边界：不是Area局部网格或绘图坐标。
type physicalShapePoint struct{ x, y float64 }

// updatePhysicalLayoutShapeMetrics 职责：从最终实体的共同物理包络重算形状；使用：所有模式最终度量；边界：保留既有格网面积/足迹统计契约，EnvelopeArea不是平方米，形状代价为格面积与物理比例的混合软目标；不使用空Area或根网格伸缩替代实体外形。
// 逻辑流：1. 正式世界映射 => 2. 实体真实建造盒、正式分拣器拉伸盒和节点球轮廓采样 => 3. 球面共同中心等距展开 => 4. 最小面积矩形，拒绝旋转加空白凑方形；不替代碰撞规则。
func updatePhysicalLayoutShapeMetrics(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building, metrics *LayoutMetrics) error {
	if len(records) == 0 || metrics == nil {
		return fmt.Errorf("形状度量缺少最终实体或指标")
	}
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	first, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	second, err := encoding.surfaceSecondPositions(records)
	if err != nil {
		return err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	points := make([]placement.Vector, 0, len(records)*8)
	hasInserters := false
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if surfaceIsBelt(record.ItemID) {
			points = appendPhysicalShapeBelt(points, encoding, first[i])
			continue
		}
		if regionIsInserter(record.ItemID) {
			hasInserters = true
			continue
		}
		shapes, err := catalog.Shapes(record.ItemID, record.ModelIndex)
		if err != nil {
			return err
		}
		for _, shape := range shapes {
			box, err := encoding.colliderBox(first[i], record.Yaw, shape)
			if err != nil {
				return err
			}
			points = appendPhysicalShapeBox(points, box)
		}
	}
	if hasInserters {
		// 复用正式端口修正及拉伸盒；不能用两端加粗球或额外半径让长条虚假变方。
		boxes, err := encoding.surfaceInserterBoxesAtWorld(ctx, records, first, second, nil)
		if err != nil {
			return err
		}
		for _, box := range boxes {
			points = appendPhysicalShapeBox(points, box.Geometry)
		}
	}
	projected, err := physicalShapeProjection(points, encoding.planar)
	if err != nil {
		return err
	}
	width, height, err := physicalShapeRectangle(projected)
	if err != nil {
		return err
	}
	metrics.EnvelopeAspectRatio = math.Max(width, height) / math.Min(width, height)
	metrics.ShapeObjective = layoutShapeObjective(*metrics)
	return ctx.Err()
}

// appendPhysicalShapeBox 职责：取正式实体盒的八个真实顶点；使用：静态建筑和已修正分拣器；边界：不扩大盒或用世界AABB代替定向盒。
func appendPhysicalShapeBox(points []placement.Vector, box placement.Box) []placement.Vector {
	for mask := 0; mask < 8; mask++ {
		point := box.Center
		for axis := 0; axis < 3; axis++ {
			sign := float64(2*((mask>>axis)&1) - 1)
			point = point.Add(box.Axes[axis].Scale(sign * box.HalfSize[axis]))
		}
		points = append(points, point)
	}
	return points
}

// appendPhysicalShapeBelt 职责：按正式节点球采样地表轮廓；使用：物理形状度量；边界：32个点均在实际球面上，不以外包立方体加粗传送带，轮廓离散误差小于球半径的0.5%。
func appendPhysicalShapeBelt(points []placement.Vector, encoding *surfaceEncoding, position vector) []placement.Vector {
	point := placement.Vector{position.x, position.y, position.z}
	up := encoding.localUp(point)
	point = point.Add(up.Scale(.2))
	east := up.Cross(placement.Vector{0, 1, 0})
	if encoding.planar || east.Dot(east) < 1e-12 {
		east = placement.Vector{1, 0, 0}
	} else {
		east = east.Scale(1 / math.Sqrt(east.Dot(east)))
	}
	north := east.Cross(up)
	radius := surfaceBeltNodeRadius
	if !encoding.planar {
		// 球心到地心的视锥切圆：投影轮廓来自节点球本身，不能人为增厚。
		lengthSquared := point.Dot(point)
		factor := 1 - radius*radius/lengthSquared
		point = point.Scale(factor)
		radius *= math.Sqrt(factor)
	}
	for i := 0; i < 32; i++ {
		sine, cosine := math.Sincos(float64(i) * 2 * math.Pi / 32)
		points = append(points, point.Add(east.Scale(radius*cosine)).Add(north.Scale(radius*sine)))
	}
	return points
}

// physicalShapeProjection 职责：统一跨Area实体的米制口径；使用：最终形状采样；边界：平面取东/北，球面归一到地表后方位等距展开；跨半球明确失败，中心仅依赖外包极值，不受填带采样密度影响。
func physicalShapeProjection(points []placement.Vector, planar bool) ([]physicalShapePoint, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("物理形状缺少采样点")
	}
	low, high := placement.Vector{math.Inf(1), math.Inf(1), math.Inf(1)}, placement.Vector{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for i, point := range points {
		for _, value := range point {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("物理形状存在非有限坐标")
			}
		}
		if !planar {
			length := math.Sqrt(point.Dot(point))
			if length < 1 {
				return nil, fmt.Errorf("物理形状缺少有效地表径向位置")
			}
			point = point.Scale(1 / length)
			points[i] = point
		}
		for axis := 0; axis < 3; axis++ {
			low[axis], high[axis] = math.Min(low[axis], point[axis]), math.Max(high[axis], point[axis])
		}
	}
	center := low.Add(high).Scale(.5)
	length := math.Sqrt(center.Dot(center))
	if !planar && length < 1e-6 {
		return nil, fmt.Errorf("物理形状无法建立单一局部球面包络")
	}
	if !planar {
		center = center.Scale(1 / length)
	}
	east := center.Cross(placement.Vector{0, 1, 0})
	if length := math.Sqrt(east.Dot(east)); length > 1e-8 {
		east = east.Scale(1 / length)
	} else {
		east = placement.Vector{1, 0, 0}
	}
	north := east.Cross(center)
	result := make([]physicalShapePoint, len(points))
	for i, point := range points {
		if planar {
			result[i] = physicalShapePoint{point[0], point[2]}
			continue
		}
		cosine := math.Max(-1, math.Min(1, center.Dot(point)))
		if cosine <= 0 {
			return nil, fmt.Errorf("物理形状跨越局部半球，不能伪造近方形包络")
		}
		angle := math.Acos(cosine)
		scale := 200.2
		if angle > 1e-8 {
			scale *= angle / math.Sin(angle)
		}
		result[i] = physicalShapePoint{scale * point.Dot(east), scale * point.Dot(north)}
	}
	return result, nil
}

// physicalShapeRectangle 职责：用凸包及旋转卡壳求最小面积矩形；使用：所有模式的物理形状；边界：仅旋转度量坐标，不移动实体，不能用斜放长条的更大方形外包冒充达标。
func physicalShapeRectangle(points []physicalShapePoint) (float64, float64, error) {
	slices.SortFunc(points, func(a, b physicalShapePoint) int {
		if a.x < b.x || a.x == b.x && a.y < b.y {
			return -1
		}
		if a == b {
			return 0
		}
		return 1
	})
	points = slices.Compact(points)
	if len(points) < 3 {
		return 0, 0, fmt.Errorf("物理形状没有正面积包络")
	}
	cross := func(a, b, c physicalShapePoint) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }
	hull := make([]physicalShapePoint, 0, len(points)*2)
	for _, point := range points {
		for len(hull) >= 2 && cross(hull[len(hull)-2], hull[len(hull)-1], point) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, point)
	}
	lower := len(hull)
	for i := len(points) - 2; i >= 0; i-- {
		for len(hull) > lower && cross(hull[len(hull)-2], hull[len(hull)-1], points[i]) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, points[i])
	}
	hull = hull[:len(hull)-1]
	if len(hull) < 3 {
		return 0, 0, fmt.Errorf("物理形状退化为直线")
	}
	support := [4]int{}
	bestArea, bestWidth, bestHeight := math.Inf(1), 0., 0.
	for i, point := range hull {
		next := hull[(i+1)%len(hull)]
		dx, dy := next.x-point.x, next.y-point.y
		length := math.Hypot(dx, dy)
		u, v := physicalShapePoint{dx / length, dy / length}, physicalShapePoint{-dy / length, dx / length}
		directions := [4]physicalShapePoint{u, v, {-u.x, -u.y}, {-v.x, -v.y}}
		dots := [4]float64{}
		for side, direction := range directions {
			dot := func(index int) float64 { return hull[index].x*direction.x + hull[index].y*direction.y }
			if i == 0 {
				for index := range hull {
					if dot(index) > dot(support[side]) {
						support[side] = index
					}
				}
			} else {
				for step := 0; step < len(hull); step++ {
					next := (support[side] + 1) % len(hull)
					if dot(next) <= dot(support[side]) {
						break
					}
					support[side] = next
				}
			}
			dots[side] = dot(support[side])
		}
		width, height := dots[0]+dots[2], dots[1]+dots[3]
		area := width * height
		if area < bestArea || area == bestArea && math.Max(width, height) < math.Max(bestWidth, bestHeight) {
			bestArea, bestWidth, bestHeight = area, width, height
		}
	}
	if bestWidth <= 0 || bestHeight <= 0 || math.IsNaN(bestArea) || math.IsInf(bestArea, 0) {
		return 0, 0, fmt.Errorf("物理形状包络无效")
	}
	return bestWidth, bestHeight, nil
}
