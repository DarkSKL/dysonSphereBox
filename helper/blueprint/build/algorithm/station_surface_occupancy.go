package build

// 文件职责：集中塔组球面带占位探测的固定预算及拒绝计数。
// 使用：完整块装箱预筛；边界：始终执行最终带占位规则，缓存仍由装箱入口管理。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// stationOccupancyProbeLimit 职责：固定昂贵球面占位探测上限；使用：完整塔组装箱；边界：预算耗尽不代表几何无解，也不允许未经验证的落点。
const stationOccupancyProbeLimit = 4096

// stationSurfaceOccupancyProbe 职责：累计同次装箱的探测和拒绝次数；使用：硬预算及最终诊断；边界：不保存或修改候选建筑。
type stationSurfaceOccupancyProbe struct {
	probes, rejected int
}

// check 职责：先检查固定预算再运行真实几何规则；使用：每个未缓存的塔组落点；边界：第二返回值表示预算耗尽，调用方不得继续接受其它落点。
func (p *stationSurfaceOccupancyProbe) check(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (error, bool) {
	if p.probes >= stationOccupancyProbeLimit {
		return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("物流塔球面带占位探测达到固定预算 %d，拒绝未完成几何验证的落点", stationOccupancyProbeLimit)}, true
	}
	p.probes++
	err := encoding.validateBeltBuildingOccupancy(ctx, records)
	if err != nil {
		p.rejected++
	}
	return err, false
}
