package build

// 文件职责：集中维护所有生产模式最终候选的形状验收与稳定排序。
// 使用：完整终检后的配送、物流塔、直连和跨带归约共用此规则。
// 边界：不生成候选、不修改实体或指标；形状达标不等于物流、碰撞或游戏验收通过。

import (
	"fmt"
	"math"
	"slices"
)

// DefaultLayoutAcceptedAspect 是旧工作流未指定形状验收上限时的默认值。
const DefaultLayoutAcceptedAspect = 2.5

// defaultPreferredAspect 兼容未携带任务策略的历史度量。
const defaultPreferredAspect = 1.4

// DefaultLayoutShapePolicy 返回独立的旧方案默认值，供注册工具与直接准备输入使用。
func DefaultLayoutShapePolicy() LayoutShapePolicy {
	return LayoutShapePolicy{
		MaxAspect: DefaultLayoutAcceptedAspect, PreferredAspect: defaultPreferredAspect,
		PackingPreferredAspect: 1.8, PackingWireWeight: 2,
		HeightScales: []float64{1, .8, 1.25}, AdjacentWindowScales: []float64{1, .8, 1.2},
	}
}

// LayoutShapeAccepted 职责：判断真实完整包络是否满足近方形验收；使用：最终归约及验收记录；边界：无效、缺失或非有限的面积/比例均不达标，不替代完整规则证明。
func LayoutShapeAccepted(metrics LayoutMetrics) bool {
	return validLayoutShape(metrics) && validLayoutAspectLimit(metrics.acceptedAspect) && metrics.EnvelopeAspectRatio <= metrics.acceptedAspect
}

// validLayoutShape 职责：拒绝缺失或非有限的形状度量；使用：共用排序入口；边界：零面积和零长宽比不能冒充方形成功结果。
func validLayoutShape(metrics LayoutMetrics) bool {
	return metrics.EnvelopeArea > 0 && !math.IsNaN(metrics.EnvelopeArea) && !math.IsInf(metrics.EnvelopeArea, 0) &&
		metrics.EnvelopeAspectRatio >= 1 && !math.IsNaN(metrics.EnvelopeAspectRatio) && !math.IsInf(metrics.EnvelopeAspectRatio, 0)
}

// layoutShapeCost 职责：以面积和最长边共同约束近方形软目标；使用：普通与密铺共同评分；边界：固定最长边时增加空白不会降低代价，无效输入返回正无穷。
func layoutShapeCost(preferred, area, aspect float64) float64 {
	if !validLayoutAspectLimit(preferred) || !validLayoutShape(LayoutMetrics{EnvelopeArea: area, EnvelopeAspectRatio: aspect}) {
		return math.Inf(1)
	}
	return area * math.Max(1, aspect/preferred)
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
// 逻辑流：1. 排除无效度量 => 2. 优先验收达标 => 3. 同档节省真实设施 => 4. 比较面积/最长边和实际带长 => 5. 同分保留原候选。
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

// LayoutShapePolicy 职责：定义工作流任务的形状验收策略；使用：输入归一工具；边界：控制形状评分与有限搜索，不改变实体物理规则。
type LayoutShapePolicy struct {
	// MaxAspect 职责：限制真实包络长宽比；使用：验收、择优、剪枝和预览；边界：必须有限且至少为1。
	MaxAspect float64 `json:"maxAspect" min:"1"`
	// PreferredAspect 是全厂形状代价开始按比例增加的软目标。
	PreferredAspect float64 `json:"preferredAspect" min:"1"`
	// PackingPreferredAspect 是密铺单元和塔场局部装箱的软目标。
	PackingPreferredAspect float64 `json:"packingPreferredAspect" min:"1"`
	// PackingWireWeight 规定启用线路代价时每单位估算线长的权重，零表示从局部加权评分中移除线路项。
	PackingWireWeight float64 `json:"packingWireWeight" min:"0"`
	// HeightScales 按声明顺序缩放普通列高，末尾仍保留原列高退路。
	HeightScales []float64 `json:"heightScales" min:"1" max:"16"`
	// AdjacentWindowScales 同时控制居中和偏移跨带窗口的首批高度。
	AdjacentWindowScales []float64 `json:"adjacentWindowScales" min:"1" max:"16"`
}

// packingCost 按同一口径比较塔场和密铺位置，纯面积策略传入零线长。
func (policy *LayoutShapePolicy) packingCost(area, aspect, wire float64) float64 {
	return area*math.Max(1, aspect/policy.PackingPreferredAspect) + wire*policy.PackingWireWeight
}

func validLayoutAspectLimit(value float64) bool {
	return value >= 1 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// Validate 校验冻结前的策略和有序倍率，供编译期与公开配置入口共用。
func (policy LayoutShapePolicy) Validate() error {
	if !validLayoutAspectLimit(policy.MaxAspect) {
		return fmt.Errorf("maxAspect必须是大于等于1的有限数值")
	}
	if !validLayoutAspectLimit(policy.PreferredAspect) || !validLayoutAspectLimit(policy.PackingPreferredAspect) {
		return fmt.Errorf("preferredAspect 和 packingPreferredAspect 必须是大于等于1的有限数值")
	}
	if policy.PackingWireWeight < 0 || math.IsNaN(policy.PackingWireWeight) || math.IsInf(policy.PackingWireWeight, 0) {
		return fmt.Errorf("packingWireWeight 必须是非负有限数值")
	}
	for _, scales := range []struct {
		name   string
		values []float64
	}{
		{"heightScales", policy.HeightScales}, {"adjacentWindowScales", policy.AdjacentWindowScales},
	} {
		if len(scales.values) < 1 || len(scales.values) > 16 {
			return fmt.Errorf("%s 需要 1 至 16 个倍率", scales.name)
		}
		for _, scale := range scales.values {
			if !finitePositive(scale) || scale > 16 {
				return fmt.Errorf("%s 倍率必须在 (0,16] 内", scales.name)
			}
		}
	}
	return nil
}

// ConfigureLayoutShape 将已校验的布局策略冻结在一个请求中。
func ConfigureLayoutShape(request Request, policy LayoutShapePolicy) (Request, error) {
	// 兼容仅设置 MaxAspect 的旧 Go 调用；工作流节点在绑定时已校验全部字段。
	if policy.PreferredAspect == 0 && policy.PackingPreferredAspect == 0 && policy.PackingWireWeight == 0 && policy.HeightScales == nil && policy.AdjacentWindowScales == nil {
		maxAspect := policy.MaxAspect
		policy = DefaultLayoutShapePolicy()
		policy.MaxAspect = maxAspect
	}
	if err := policy.Validate(); err != nil {
		return Request{}, err
	}
	// 策略从此只读，各快照共享；外部持有的切片不能修改已冻结的搜索顺序。
	policy.HeightScales = slices.Clone(policy.HeightScales)
	policy.AdjacentWindowScales = slices.Clone(policy.AdjacentWindowScales)
	request.shape = &policy
	return request, nil
}

// LayoutAspectLimit 职责：读取候选冻结的验收上限；使用：错误说明；边界：未绑定策略的度量不能通过形状验收。
func LayoutAspectLimit(metrics LayoutMetrics) float64 { return metrics.acceptedAspect }
