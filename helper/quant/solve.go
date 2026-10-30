package quant

// 文件职责：校验量化输入并形成产线桥梁；使用：factory Solve 和对拍；边界：只计算，不写数据或代替游戏验收。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// MachineKind 职责：按原型返回参考机器类型和等级；使用：量化到生成桥梁；边界：不支持的设施返回 -1，不伪造可生成产线。
func MachineKind(id int) (int, int) {
	groups := [][]int{{2302, 2315, 2319}, {2303, 2304, 2305, 2318}, {2308}, {2309, 2317}, {2310}, {2901, 2902}}
	for typ, group := range groups {
		for lv, value := range group {
			if value == id {
				return typ + 1, lv + 1
			}
		}
	}
	return -1, 0
}

// Solve 职责：执行有界量化；使用：工作区目标请求；边界：最多 5 秒，失败不返回部分结果。
// 逻辑流：1. 校验目标和版本 => 2. 构图/剥离 => 3. 前向成本及循环 LP => 4. 取整、电力与稳定指纹。
func Solve(parent context.Context, req Request) (Result, error) {
	out := Result{Productions: []Production{}, RawInputs: []ItemRate{}, Leftovers: []ItemRate{}, KeyItems: []int{}, Warnings: []string{}}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	if len(req.Targets) == 0 || len(req.Targets) > 100 {
		return out, qerror("Q_E_BAD_REQUEST", "请设置 1 到 100 个正产量目标")
	}
	d, err := LoadDataset(req.Dataset)
	if err != nil {
		return out, err
	}
	targetRates := map[string][]float64{}
	for _, target := range req.Targets {
		if target.RatePerMinute <= 0 || target.RatePerMinute > 1e9 || math.IsNaN(target.RatePerMinute) || math.IsInf(target.RatePerMinute, 0) {
			return out, qerror("Q_E_BAD_REQUEST", "目标每分钟产量必须为有限正数，且不超过 10 亿")
		}
		item, ok := d.ByID[target.ItemID]
		if !ok {
			return out, qerror("Q_E_NO_RECIPE", fmt.Sprintf("物品 %d 不在数据集中", target.ItemID))
		}
		if len(d.Choices[item.Name]) == 0 && !req.Settings.MineralizeList[item.Name] {
			return out, qerror("Q_E_NO_RECIPE", fmt.Sprintf("物品 %d %s 没有配方", item.ID, item.Name))
		}
		targetRates[item.Name] = append(targetRates[item.Name], target.RatePerMinute)
	}
	g, err := makeGraph(d, req.Scheme, req.Settings)
	if err != nil {
		return out, err
	}
	if ctx.Err() != nil {
		return out, qerror("Q_E_TIMEOUT", "量化超过 5 秒")
	}
	// 目标输入按数据集顺序归一，避免同一物理请求因目标排列改变 LP 变量顺序。
	needs := newRates()
	targetTotals := make(map[string]float64, len(targetRates))
	for name, values := range targetRates {
		sort.Float64s(values)
		for _, value := range values {
			targetTotals[name] += value
		}
	}
	seen := map[string]bool{}
	for _, name := range d.Order {
		if amount, ok := targetTotals[name]; ok {
			needs.add(name, amount)
			seen[name] = true
		}
	}
	remaining := make([]string, 0, len(targetTotals))
	for name := range targetTotals {
		if !seen[name] {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		needs.add(name, targetTotals[name])
	}
	amounts, surplus, cost, err := g.calculate(ctx, needs)
	if err != nil {
		return out, err
	}
	if !finiteRates(amounts) || !finiteRates(surplus) || !finite(cost) {
		return Result{}, qerror("Q_E_BAD_REQUEST", "量化结果包含非有限数值")
	}
	out.Targets = append([]Target{}, req.Targets...)
	out.Dataset = d.Info.Key
	out.DataVersion = d.Info.DataVersion
	out.GameVersion = d.Info.GameVersion
	out.LPCost = cost
	out.Amounts = amounts.values
	req.Dataset = d.Info.Key
	req.Settings = g.settings
	canonical, err := json.Marshal(struct {
		Request Request
		Version string
	}{req, d.Info.DataVersion})
	if err != nil {
		return Result{}, qerror("Q_E_BAD_REQUEST", "配置包含非有限值")
	}
	sum := sha256.Sum256(canonical)
	out.PlanID = hex.EncodeToString(sum[:16])
	for _, name := range g.keys {
		out.KeyItems = append(out.KeyItems, d.ByName[name].ID)
	}
	for _, name := range g.order {
		amount := amounts.values[name]
		if amount < -1e-7 {
			return Result{}, qerror("Q_E_INFEASIBLE", "存在负产出: "+name)
		}
		if amount <= 1e-8 {
			continue
		}
		n := g.nodes[name]
		if n.invalid != "" {
			return Result{}, qerror("Q_E_NO_RECIPE", n.invalid)
		}
		typ, lv := MachineKind(n.factory.ID)
		if n.raw || typ < 0 {
			out.RawInputs = append(out.RawInputs, ItemRate{ItemID: d.ByName[name].ID, Name: name, RatePerMinute: amount})
			if !n.raw && len(d.Recipes[n.recipe].Items) > 0 {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s 的设施 %s 不在六类可生成制造设备中，需外部供应；其功耗未计入制造功耗", name, n.factory.Name))
			}
			continue
		}
		speed := n.factory.Speed
		if speed <= 0 {
			speed = 1
		}
		exact := amount / 60 / n.rate / speed
		if exact > 1000000 {
			return Result{}, qerror("Q_E_BAD_REQUEST", "单条产线机器数超过 100 万，请拆分目标")
		}
		// 设备数量必须覆盖精确需求；仅吸附极小浮点误差，不能按展示小数位预舍入。
		ceilExact := exact
		if math.Abs(exact-math.Round(exact)) <= 1e-9 {
			ceilExact = math.Round(exact)
		}
		count := int(math.Ceil(ceilExact))
		if count < 1 {
			count = 1
		}
		level := 0
		if n.scheme.IncreasePoints > 0 {
			level = 1
			if n.scheme.IncreasePoints >= 2 {
				level = 2
			}
			if n.scheme.IncreasePoints >= 4 {
				level = 3
			}
		}
		r := d.Recipes[n.recipe]
		p := Production{RecipeID: r.ID, MachineCount: count, ExactMachineCount: exact, RoundingMargin: float64(count) - exact, MachineType: typ, MachineLv: lv, BeltLv: 1, MachineID: n.factory.ID, OutputCount: amount, MainItemID: d.ByName[name].ID, ProductionMode: blueprintMode(n.scheme.ProductionMode), ProliferatorMode: n.scheme.ProductionMode, IncreaseLv: level, IncreasePoints: n.scheme.IncreasePoints}
		power := 1.
		if n.scheme.ProductionMode > 0 {
			_, _, power = effects(n.scheme.IncreasePoints)
		}
		p.PowerMW = exact * n.factory.WorkEnergyPerTick * .00006 * power
		p.Inputs, p.Outputs = flows(d, n, amount)
		out.Productions = append(out.Productions, p)
		out.MachineCount += count
		out.PowerMW += p.PowerMW
	}
	attributeTargets(&out)
	for _, name := range surplus.keys() {
		if surplus.values[name] > 1e-8 {
			out.Leftovers = append(out.Leftovers, ItemRate{ItemID: d.ByName[name].ID, Name: name, RatePerMinute: surplus.values[name]})
		}
	}
	if ctx.Err() != nil {
		return Result{}, qerror("Q_E_TIMEOUT", "量化超过 5 秒")
	}
	if !finiteResult(out) {
		return Result{}, qerror("Q_E_BAD_REQUEST", "量化结果包含非有限数值")
	}
	return out, nil
}

// finite 职责：判断浮点值可安全进入响应；使用：结果和成本边界校验；边界：不负责业务范围验证。
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// finiteRates 职责：校验有序流量集合；使用：LP 回填前阻断溢出；边界：不判断负值语义。
func finiteRates(values *rates) bool {
	if values == nil {
		return false
	}
	for _, name := range values.keys() {
		if !finite(values.values[name]) {
			return false
		}
	}
	return true
}

// finiteResult 职责：校验量化响应中的所有数值；使用：Solve 返回前；边界：不改变既有错误码或投影边界。
func finiteResult(result Result) bool {
	if !finite(result.PowerMW) || !finite(result.LPCost) {
		return false
	}
	for _, value := range result.Amounts {
		if !finite(value) {
			return false
		}
	}
	for _, list := range [][]ItemRate{result.RawInputs, result.Leftovers} {
		for _, rate := range list {
			if !finite(rate.RatePerMinute) {
				return false
			}
		}
	}
	for _, production := range result.Productions {
		if !finite(production.ExactMachineCount) || !finite(production.RoundingMargin) || !finite(production.OutputCount) || !finite(production.PowerMW) {
			return false
		}
		for _, list := range [][]ItemRate{production.Inputs, production.Outputs} {
			for _, rate := range list {
				if !finite(rate.RatePerMinute) {
					return false
				}
			}
		}
	}
	return true
}

// attributeTargets 职责：沿物料流反向追溯每个目标依赖的产线；使用：Solve 末尾；边界：只读产线 Inputs/Outputs，循环由 visited 截断。
func attributeTargets(out *Result) {
	producers := map[int][]int{}
	for i, p := range out.Productions {
		out.Productions[i].TargetItemIDs = []int{}
		for _, v := range p.Outputs {
			producers[v.ItemID] = append(producers[v.ItemID], i)
		}
	}
	seen := map[int]bool{}
	for _, target := range out.Targets {
		if seen[target.ItemID] {
			continue
		}
		seen[target.ItemID] = true
		visited := map[int]bool{}
		queue := append([]int{}, producers[target.ItemID]...)
		for len(queue) > 0 {
			index := queue[0]
			queue = queue[1:]
			if visited[index] {
				continue
			}
			visited[index] = true
			out.Productions[index].TargetItemIDs = append(out.Productions[index].TargetItemIDs, target.ItemID)
			for _, v := range out.Productions[index].Inputs {
				queue = append(queue, producers[v.ItemID]...)
			}
		}
	}
}

// flows 职责：把单次执行物料换算为每分钟净流量；使用：节点图边；边界：同一物品出入相抵，主产物净值与 amount 一致。
func flows(d *Dataset, n *node, amount float64) ([]ItemRate, []ItemRate) {
	inputs, outputs := []ItemRate{}, []ItemRate{}
	if n.runNet <= 0 {
		return inputs, outputs
	}
	runs := amount / n.runNet
	for _, name := range n.runInputs.keys() {
		if v := (n.runInputs.values[name] - n.runOutputs.values[name]) * runs; v > 1e-9 {
			inputs = append(inputs, ItemRate{ItemID: d.ByName[name].ID, Name: name, RatePerMinute: v})
		}
	}
	for _, name := range n.runOutputs.keys() {
		if v := (n.runOutputs.values[name] - n.runInputs.values[name]) * runs; v > 1e-9 {
			outputs = append(outputs, ItemRate{ItemID: d.ByName[name].ID, Name: name, RatePerMinute: v})
		}
	}
	return inputs, outputs
}

// blueprintMode 职责：转换人工喷涂为游戏参数枚举；使用：量化桥梁；边界：无喷涂为 -1，额外产出为 0，加速为 1。
func blueprintMode(mode int) int {
	if mode == 0 {
		return -1
	}
	if mode == 1 || mode == 3 {
		return 1
	}
	return 0
}
