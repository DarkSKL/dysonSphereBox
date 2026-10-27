package factory

// 文件职责：对蓝图制造配方做只读理论反算；使用：factory Inspect；边界：没有已喷涂点数信息，不能反推实际运行产能或物流瓶颈。

import (
	"DysonSphereBox/helper/blueprint"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	"fmt"
)

// Inspect 职责：聚合配方理论产出与净物料；使用：导入回放；边界：不伪造 QuantResult，不修改资源文件。
func Inspect(req contract.InspectRequest) (contract.Inspection, error) {
	out := contract.Inspection{Machines: []contract.InspectedMachine{}, Outputs: []quant.ItemRate{}, NetRates: []quant.ItemRate{}, UnknownIDs: []int{}, Warnings: []string{"蓝图不保存已喷涂物品的增产点数，以下按未喷涂、额定速度计算，不包含带速、堵带或供电损失"}}
	d, err := blueprint.Decode(req.Blueprint, true)
	if err != nil {
		return out, err
	}
	dataset, err := quant.LoadDataset(req.Dataset)
	if err != nil {
		return out, err
	}
	out.Summary = d.Summary
	unknown := map[int]bool{}
	outputs, net := map[int]float64{}, map[int]float64{}
	for index, b := range d.Buildings {
		item, ok := dataset.ByID[int(b.ItemID)]
		if !ok {
			if !unknown[int(b.ItemID)] {
				unknown[int(b.ItemID)] = true
				out.UnknownIDs = append(out.UnknownIDs, int(b.ItemID))
			}
			continue
		}
		if b.RecipeID <= 0 {
			continue
		}
		ri, ok := dataset.RecipeByID[int(b.RecipeID)]
		if !ok {
			out.Warnings = append(out.Warnings, fmt.Sprintf("建筑 %d 的配方 %d 不在当前数据集", index, b.RecipeID))
			continue
		}
		r := dataset.Recipes[ri]
		allowed := false
		for _, id := range r.Factories {
			if id == int(b.ItemID) {
				allowed = true
			}
		}
		if !allowed {
			out.Warnings = append(out.Warnings, fmt.Sprintf("建筑 %d 的设施与配方不匹配，已跳过反算", index))
			continue
		}
		if b.ItemID == 2901 || b.ItemID == 2902 {
			if len(b.Parameters) > 0 && b.Parameters[0] != 1 {
				continue
			}
		}
		speed := item.Speed
		if speed <= 0 {
			speed = 1
		}
		times := 60 / (r.TimeSpend / 60) * speed
		m := contract.InspectedMachine{BuildingIndex: index, RecipeID: r.ID, MachineID: item.ID, Outputs: []quant.ItemRate{}}
		for i, id := range r.Results {
			rate := r.ResultCounts[i] * times
			outputs[id] += rate
			net[id] += rate
			m.Outputs = append(m.Outputs, quant.ItemRate{ItemID: id, Name: dataset.ByID[id].Name, RatePerMinute: rate})
		}
		for i, id := range r.Items {
			net[id] -= r.ItemCounts[i] * times
		}
		out.Machines = append(out.Machines, m)
	}
	for _, item := range dataset.Items {
		if rate := outputs[item.ID]; rate > 0 {
			out.Outputs = append(out.Outputs, quant.ItemRate{ItemID: item.ID, Name: item.Name, RatePerMinute: rate})
		}
		if rate := net[item.ID]; rate != 0 {
			out.NetRates = append(out.NetRates, quant.ItemRate{ItemID: item.ID, Name: item.Name, RatePerMinute: rate})
		}
	}
	return out, nil
}
