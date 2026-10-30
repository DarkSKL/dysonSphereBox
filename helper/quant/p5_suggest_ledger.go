package quant

// 文件职责：P5a 请求隔离与物理可判定门禁；使用：候选搜索内部；边界：只检查 Go 投影闭合，不替代真实游戏或完整设施模型。

import "math"

// suggestCloneMap 职责：保留 nil 与显式零值复制 map；使用：请求快照；边界：值类型不得包含可变引用。
func suggestCloneMap[K comparable, V any](in map[K]V) map[K]V {
	if in == nil {
		return nil
	}
	out := make(map[K]V, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// suggestCloneSlice 职责：保留 nil 与空切片差异复制值元素；使用：请求快照；边界：元素为无引用结构。
func suggestCloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

// suggestClonePointer 职责：独立复制可选标量；使用：保留 0/false；边界：不通过默认值或 JSON 重建。
func suggestClonePointer[T any](in *T) *T {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// suggestCloneRequest 职责：逐字段隔离所有请求引用；使用：每一次基线/候选 Solve；边界：调用者仍须避免并发修改同一原请求。
func suggestCloneRequest(req Request) Request {
	req.Targets = suggestCloneSlice(req.Targets)
	req.Scheme.ItemRecipeChoices = suggestCloneMap(req.Scheme.ItemRecipeChoices)
	req.Scheme.SchemeForRecipe = suggestCloneSlice(req.Scheme.SchemeForRecipe)
	req.Scheme.CostWeight.Space = suggestClonePointer(req.Scheme.CostWeight.Space)
	req.Scheme.CostWeight.BuildingCost = suggestCloneMap(req.Scheme.CostWeight.BuildingCost)
	req.Scheme.CostWeight.ItemExtraCost = suggestCloneMap(req.Scheme.CostWeight.ItemExtraCost)
	req.Settings.FixedNum = suggestClonePointer(req.Settings.FixedNum)
	req.Settings.ProliferateItself = suggestClonePointer(req.Settings.ProliferateItself)
	req.Settings.MineralizeList = suggestCloneMap(req.Settings.MineralizeList)
	req.Settings.NaturalProductionLine = suggestCloneSlice(req.Settings.NaturalProductionLine)
	return req
}

// suggestAssess 职责：检查有限值、制造计数与逐物品守恒；使用：基线及全部成功候选推荐门禁；边界：固定线 net 只扣一次，严格小于逐物品相对容差才通过。
func suggestAssess(d *Dataset, req Request, result Result) bool {
	if !finiteResult(result) || len(result.Warnings) > 0 || result.MachineCount < 0 {
		return false
	}
	sum := 0
	maxInt := int(^uint(0) >> 1)
	for _, p := range result.Productions {
		if p.MachineCount < 0 || p.ExactMachineCount < 0 || p.ExactMachineCount-float64(p.MachineCount) > 1e-9 || p.MachineCount > maxInt-sum {
			return false
		}
		sum += p.MachineCount
	}
	if sum != result.MachineCount {
		return false
	}
	residual, scale := map[int]float64{}, map[int]float64{}
	add := func(id int, amount float64) bool {
		item, ok := d.ByID[id]
		if !ok || item.ID != id || !finite(amount) {
			return false
		}
		residual[id] += amount
		scale[id] = math.Max(scale[id], math.Abs(amount))
		return finite(residual[id])
	}
	for _, target := range result.Targets {
		if !add(target.ItemID, -target.RatePerMinute) {
			return false
		}
	}
	for _, raw := range result.RawInputs {
		if !add(raw.ItemID, raw.RatePerMinute) {
			return false
		}
	}
	for _, p := range result.Productions {
		for _, v := range p.Outputs {
			if !add(v.ItemID, v.RatePerMinute) {
				return false
			}
		}
		for _, v := range p.Inputs {
			if !add(v.ItemID, -v.RatePerMinute) {
				return false
			}
		}
	}
	for _, left := range result.Leftovers {
		if !add(left.ItemID, -left.RatePerMinute) {
			return false
		}
	}
	settings, err := normalizeSettings(req.Settings)
	if err != nil || !suggestNaturalIDs(d, settings.NaturalProductionLine) {
		return false
	}
	// 每条固定线独立重建净项，避免多条同向叠加放大最大单项容差，或反向抵消缩小容差。
	for _, line := range settings.NaturalProductionLine {
		lineSettings := settings
		lineSettings.NaturalProductionLine = []NaturalProduction{line}
		g := &graph{dataset: d, settings: lineSettings}
		needs := newRates()
		if err := g.balanceNatural(needs); err != nil {
			return false
		}
		for _, name := range needs.keys() {
			item, ok := d.ByName[name]
			if !ok || !add(item.ID, -needs.values[name]) {
				return false
			}
		}
	}
	for id, value := range residual {
		if !(math.Abs(value) < 1e-6*math.Max(1, scale[id])) {
			return false
		}
	}
	return true
}

// suggestNaturalIDs 职责：在固定产线重建前验证原型反查；使用：阻止缺失 ID 被 map 零值掩盖；边界：不修复或写入共享 Dataset。
func suggestNaturalIDs(d *Dataset, lines []NaturalProduction) bool {
	validItem := func(id int) bool {
		item, ok := d.ByID[id]
		back, found := d.ByName[item.Name]
		return ok && found && item.ID == id && back.ID == id
	}
	for _, line := range lines {
		choices := d.Choices[line.TargetItem]
		if line.RecipeChoice < 1 || line.RecipeChoice > len(choices) {
			return false
		}
		index := choices[line.RecipeChoice-1]
		if index < 0 || index >= len(d.Recipes) {
			return false
		}
		r := d.Recipes[index]
		if line.Building < 0 || line.Building >= len(r.Factories) || !validItem(r.Factories[line.Building]) || len(r.Items) != len(r.ItemCounts) || len(r.Results) != len(r.ResultCounts) {
			return false
		}
		for _, ids := range [][]int{r.Items, r.Results} {
			for _, id := range ids {
				if !validItem(id) {
					return false
				}
			}
		}
	}
	return true
}
