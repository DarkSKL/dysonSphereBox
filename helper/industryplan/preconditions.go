package industryplan

// 文件职责：量化与选址无关的必需配套物料；使用：资源调查后的提前缺口检查；边界：不把可选发电燃料或辅助产物当作用户交付目标。

import (
	"context"
	"math"
	"slices"

	"DysonSphereBox/helper/quant"
)

// mandatoryInputsLowerBound 职责：将光子采集必需的最低透镜消耗并入原目标量化；使用：分工前核对当前配方的原料供给；边界：只给出最优连续率下的接收机下限，不代替实际落位、戴森功率或能源闭合。
func mandatoryInputsLowerBound(ctx context.Context, req Request, base quant.Result) (quant.Result, bool, error) {
	if !req.EnableFuel || !enabled(req.EnableMining, true) || req.Dyson.Mode != "sphere" || !req.Dyson.Lenses || slices.Contains(base.ExternalInputItemIDs, 1208) {
		return base, false, nil
	}
	photons, uptime := 0.0, 0.0
	for _, raw := range base.RawInputs {
		if raw.ItemID == 1208 {
			photons += raw.RatePerMinute
		}
	}
	for _, supply := range req.Dyson.Supplies {
		uptime = max(uptime, supply.Uptime)
	}
	if photons <= 0 || uptime <= 0 {
		return base, false, nil
	}
	q := req.Quant
	q.Targets = append(slices.Clone(req.Quant.Targets), quant.Target{ItemID: 1209, RatePerMinute: math.Ceil(photons / (12 * uptime))})
	result, err := quant.Solve(ctx, q)
	return result, true, err
}
