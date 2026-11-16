package build

// 文件职责：集中维护所有生产模式最终候选的形状验收与稳定排序。
// 使用：完整终检后的配送、物流塔、直连和跨带归约共用此规则。
// 边界：不生成候选、不修改实体或指标；形状达标不等于物流、碰撞或游戏验收通过。

import "math"

const (
	// layoutAcceptedAspect 职责：定义近方形验收上限；使用：所有模式共同分档；边界：仅对真实完整包络度量生效，不能补空白凑比例。
	layoutAcceptedAspect = 1.8
	// layoutPreferredAspect 职责：定义达标档内的近方形软目标；使用：统一面积与最长边代价；边界：1.4不是验收线，也不随密度模式改变。
	layoutPreferredAspect = 1.4
)

// LayoutShapeAccepted 职责：判断真实完整包络是否满足近方形验收；使用：最终归约及验收记录；边界：无效、缺失或非有限的面积/比例均不达标，不替代完整规则证明。
func LayoutShapeAccepted(metrics LayoutMetrics) bool {
	return validLayoutShape(metrics) && metrics.EnvelopeAspectRatio <= layoutAcceptedAspect
}

// validLayoutShape 职责：拒绝缺失或非有限的形状度量；使用：共用排序入口；边界：零面积和零长宽比不能冒充方形成功结果。
func validLayoutShape(metrics LayoutMetrics) bool {
	return metrics.EnvelopeArea > 0 && !math.IsNaN(metrics.EnvelopeArea) && !math.IsInf(metrics.EnvelopeArea, 0) &&
		metrics.EnvelopeAspectRatio >= 1 && !math.IsNaN(metrics.EnvelopeAspectRatio) && !math.IsInf(metrics.EnvelopeAspectRatio, 0)
}

// layoutShapeCost 职责：以面积和最长边共同约束近方形软目标；使用：普通与密铺共同评分；边界：固定最长边时增加空白不会降低代价，无效输入返回正无穷。
func layoutShapeCost(area, aspect float64) float64 {
	if !validLayoutShape(LayoutMetrics{EnvelopeArea: area, EnvelopeAspectRatio: aspect}) {
		return math.Inf(1)
	}
	return area * math.Max(1, aspect/layoutPreferredAspect)
}

// validLayoutCandidateCost 职责：核对排序会使用的真实设施和线路代价；使用：拒绝NaN、负数和无穷大绕过比较；边界：不校验实体或推测缺失线路。
func validLayoutCandidateCost(metrics LayoutMetrics, facilities int) bool {
	return facilities >= 0 && validLayoutShape(metrics) && !math.IsInf(layoutShapeObjective(metrics), 0) &&
		metrics.TotalBeltLengthMeters >= 0 && !math.IsNaN(metrics.TotalBeltLengthMeters) && !math.IsInf(metrics.TotalBeltLengthMeters, 0) &&
		metrics.InternalBeltLength >= 0 && !math.IsNaN(metrics.InternalBeltLength) && !math.IsInf(metrics.InternalBeltLength, 0) &&
		metrics.BeltCount >= 0 && metrics.CrossLayerConnections >= 0 && metrics.CrossAreaConnections >= 0 && metrics.ExternalInputCount >= 0
}

// BetterLayoutCandidate 职责：按近方形达标档、真实设施数、共同形状代价、面积和线路稳定排序；使用：各生产模式完整终检后的唯一比较器，无设施差异时两侧均传0。
// 边界：不读取密度、拓扑或缓存ShapeObjective，不奖励填带降低BlankRatio；双方均未达标时只选较优失败形状，调用方须用LayoutShapeAccepted单独记录验收。
// 逻辑流：1. 排除无效度量 => 2. 优先aspect<=1.8 => 3. 同档节省真实设施 => 4. 比较面积/最长边和实际带长 => 5. 同分保留原候选。
func BetterLayoutCandidate(a LayoutMetrics, facilitiesA int, b LayoutMetrics, facilitiesB int) bool {
	if !validLayoutCandidateCost(a, facilitiesA) {
		return false
	}
	if !validLayoutCandidateCost(b, facilitiesB) {
		return true
	}
	if acceptedA, acceptedB := LayoutShapeAccepted(a), LayoutShapeAccepted(b); acceptedA != acceptedB {
		return acceptedA
	}
	if facilitiesA != facilitiesB {
		return facilitiesA < facilitiesB
	}
	// 不用成对epsilon判等，避免A≈B、B≈C但A不≈C造成归约顺序改变胜者。
	for _, cost := range [][2]float64{
		{layoutShapeObjective(a), layoutShapeObjective(b)},
		{a.EnvelopeArea, b.EnvelopeArea},
		{a.TotalBeltLengthMeters, b.TotalBeltLengthMeters},
		{float64(a.BeltCount), float64(b.BeltCount)},
		{a.InternalBeltLength, b.InternalBeltLength},
		{float64(a.CrossLayerConnections), float64(b.CrossLayerConnections)},
		{float64(a.CrossAreaConnections), float64(b.CrossAreaConnections)},
		{float64(a.ExternalInputCount), float64(b.ExternalInputCount)},
		{a.EnvelopeAspectRatio, b.EnvelopeAspectRatio},
	} {
		if cost[0] != cost[1] {
			return cost[0] < cost[1]
		}
	}
	return false
}
