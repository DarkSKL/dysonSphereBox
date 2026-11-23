package industryplan

// 文件职责：按当前分工报告已落位的工作量；使用：桌面任务估算剩余用时；边界：进度不参与容量和导出核验。

import (
	"context"

	"DysonSphereBox/helper/quant"
)

type placedMachinesKey struct{}

// simpleLayout 识别一个标准制造块即可完成的单进单出配方，供首次估时选用较短的起始工时。
func simpleLayout(plan quant.Result) bool {
	if len(plan.Productions) != 1 {
		return false
	}
	p := plan.Productions[0]
	return p.MachineCount <= 32 && len(p.Inputs) == 1 && len(p.Outputs) == 1
}

// ReportPlacedMachines 职责：在一个制造模块成功放置后报告其机器数；使用：逐星装配；边界：直接生成单星蓝图时没有观察者。
func ReportPlacedMachines(ctx context.Context, machines int) {
	if report, ok := ctx.Value(placedMachinesKey{}).(func(int)); ok {
		report(machines)
	}
}

// layoutWork 按需建设的设备数估计布局工作量，另计一次整星编码与核验。
func layoutWork(site PlanetPlan) int {
	work := 1 + site.Plan.MachineCount + site.StationCount
	for _, mine := range site.Mines {
		work += mine.Count
	}
	if site.Photons != nil {
		work += site.Photons.Count
	}
	if p := site.Power; p != nil {
		if len(p.Sources) == 0 {
			work += p.Count
		}
		for _, source := range p.Sources {
			work += source.Count
		}
	}
	return work
}
