package build

// 文件职责：在所有真实配送设施成形后，安全排除设施数量严格更多的候选。
// 使用：全场占位和寻路之前；边界：只以完整终检成功的矩形方案为上界，不按预计端口数剪枝，不跳过同数或更少设施的比较。

import (
	"context"
	"errors"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// denseFlyFacilityBoundKey 职责：隔离单个完整生成请求的已验证上界；使用：候选context；边界：不缓存其它请求或失败候选的指标。
type denseFlyFacilityBoundKey struct{}

// denseFlyFacilityBound 职责：保存已经通过finish的实际设施数和形状；使用：严格劣解证明；边界：长条 incumbent 不能淘汰潜在矩形。
type denseFlyFacilityBound struct {
	count  int
	aspect float64
}

// denseFlyFacilityPruneDisabledKey 职责：保留完整枚举的独立审计入口；使用：同请求等价回归；边界：私有context键，不改变公开设置或默认行为。
type denseFlyFacilityPruneDisabledKey struct{}

// rejectDenseFlyFacilityDominance 职责：用已构造且不会被后续裁尾/供电清理删除的配送器数量证明严格劣解；使用：两类配送候选全部设施构造后；边界：黑盒临时对面箱删除以前禁止调用，设施少或同数始终继续。
func (b *builder) rejectDenseFlyFacilityDominance(ctx context.Context) error {
	if ctx == nil || !b.denseSpatial || b.adapter == nil || b.adapter.mode != 2 || b.adapter.density != densityDense {
		return nil
	}
	if disabled, _ := ctx.Value(denseFlyFacilityPruneDisabledKey{}).(bool); disabled {
		return nil
	}
	bound, ok := ctx.Value(denseFlyFacilityBoundKey{}).(denseFlyFacilityBound)
	if !ok || bound.count <= 0 || math.IsNaN(bound.aspect) || math.IsInf(bound.aspect, 0) || bound.aspect < 1 || bound.aspect > 2+1e-9 {
		return nil
	}
	actual := denseFlyFacilityCount(b.all)
	if actual <= bound.count {
		return nil
	}
	return &blueprint.Error{Code: "GEN_E_DENSITY_FACILITIES", Message: fmt.Sprintf("已构造%d个真实配送器，严格多于完整终检通过的矩形方案%d个；后续带尾裁剪及供电清理不会删除配送器，故不再执行该候选的占位、接线和终检；本结论不代表该候选几何不可行", actual, bound.count)}
}

// denseFlyFacilityDominated 职责：区分优化剪枝与实际装配失败；使用：诊断阶段及几何重试条件；边界：只有指定的严格设施下界错误属于优化结论。
func denseFlyFacilityDominated(err error) bool {
	var failure *blueprint.Error
	return errors.As(err, &failure) && failure.Code == "GEN_E_DENSITY_FACILITIES"
}
