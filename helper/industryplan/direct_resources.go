package industryplan

// 文件职责：从存档核对玩家指定的直接采集来源。规划入口调用，产能与实际落位由资源调查和蓝图生成复核。

import (
	"fmt"

	"DysonSphereBox/helper/quant"
)

// validateDirectResources 核对显式采集选择在所选星系有可验证的固体或液体来源，具体产能交由已有资源预算与逐星校验。
func validateDirectResources(req Request, planets map[int64]Planet, canResolve bool) error {
	items := req.Quant.Settings.DirectResourceItemIDs
	if len(items) == 0 {
		return nil
	}
	if !enabled(req.EnableMining, true) {
		return fmt.Errorf("直接采集星球资源需要启用新建采矿设施")
	}
	dataset, err := quant.LoadDataset(req.Quant.Dataset)
	if err != nil {
		return err
	}
	sources := map[int]bool{}
	for _, planet := range planets {
		if !planet.Buildable {
			continue
		}
		for _, res := range planet.Resources {
			if res.Kind != "solid" && res.Kind != "liquid" {
				continue
			}
			candidate := !planet.ResourcesKnown && res.Source == "candidate" && canResolve
			known := res.Source != "candidate" && (res.Kind == "solid" && res.Count > 0 || res.Kind == "liquid" && res.UnitRatePerMinute > 0)
			if candidate || known {
				sources[res.ItemID] = true
			}
		}
	}
	for _, id := range items {
		if !sources[id] {
			return fmt.Errorf("%s在所选存档没有已知或可精算的固体矿点或可泵液体", dataset.ByID[id].Name)
		}
	}
	return nil
}
