package quant

// 文件职责：公开多产物配方的实际供需与持续过剩，区分缓冲和消耗。
// 使用：量化结果及界面物料平衡说明；边界：所有数值来自已守恒净流，不把有限库存当作永久去向。

import "sort"

// ByproductBalance 职责：汇总耦合产物的期望供需；使用：界面展示内部利用与必须外运量；边界：不代表所有整数设备同时满负荷。
type ByproductBalance struct {
	// ItemID 职责：标识物料；使用：图节点联动；边界：必须存在于原型数据。
	ItemID int `json:"itemId"`
	// Name 职责：提供物料名称；使用：结果提示；边界：不作为实体身份。
	Name string `json:"name"`
	// ProducedPerMinute 职责：汇总本厂净产出；使用：扣除机器内回收后的供给；边界：包含各配方来源。
	ProducedPerMinute float64 `json:"producedPerMinute"`
	// ConsumedPerMinute 职责：汇总厂内净消耗；使用：说明优先利用份额；边界：包含生产及喷涂需求。
	ConsumedPerMinute float64 `json:"consumedPerMinute"`
	// ImportedPerMinute 职责：记录补足内部缺口的外供；使用：区分联产与外部原料；边界：不是额外副产物。
	ImportedPerMinute float64 `json:"importedPerMinute"`
	// TargetPerMinute 职责：记录已经分配的目标交付；使用：从过剩中排除显式产品；边界：目标本身仍需取走。
	TargetPerMinute float64 `json:"targetPerMinute"`
	// SurplusPerMinute 职责：记录必须持续排出的剩余；使用：出口、缓存与运行提示；边界：缓存不能令该数值消失。
	SurplusPerMinute float64 `json:"surplusPerMinute"`
}

// byproductBalances 职责：从已验证供需图的输入输出汇总联产物料；使用：Solve收尾；边界：不修改方案或虚增下游消耗。
func byproductBalances(d *Dataset, plan Result) []ByproductBalance {
	items := map[int]bool{}
	for _, p := range plan.Productions {
		index, exists := d.RecipeByID[p.RecipeID]
		if exists && len(d.Recipes[index].Results) > 1 {
			for _, id := range d.Recipes[index].Results {
				items[id] = true
			}
		}
	}
	ids := make([]int, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]ByproductBalance, 0, len(ids))
	for _, id := range ids {
		balance := ByproductBalance{ItemID: id, Name: d.ByID[id].Name}
		for _, p := range plan.Productions {
			for _, rate := range p.Outputs {
				if rate.ItemID == id {
					balance.ProducedPerMinute += rate.RatePerMinute
				}
			}
			for _, rate := range p.Inputs {
				if rate.ItemID == id {
					balance.ConsumedPerMinute += rate.RatePerMinute
				}
			}
		}
		for _, rate := range plan.RawInputs {
			if rate.ItemID == id {
				balance.ImportedPerMinute += rate.RatePerMinute
			}
		}
		for _, rate := range plan.Targets {
			if rate.ItemID == id {
				balance.TargetPerMinute += rate.RatePerMinute
			}
		}
		for _, rate := range plan.Leftovers {
			if rate.ItemID == id {
				balance.SurplusPerMinute += rate.RatePerMinute
			}
		}
		out = append(out, balance)
	}
	return out
}
