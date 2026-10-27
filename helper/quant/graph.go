package quant

// 文件职责：建立保序名称图与剥离启发式；使用：量化成本和 LP；边界：不改共享数据，不用 map 迭代驱动算法。
// 算法对照 dsp-calc global_state.jsx；独立 Go 实现，来源见 NOTICE.md。

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

// rates 职责：维护 JS 对象等价的插入顺序；使用：图边与历史足迹；边界：删除不改变其他键顺序。
type rates struct {
	// order 职责：键顺序；使用：保序计算；边界：可含已删除键。
	order []string
	// values 职责：数值；使用：键查询；边界：不直接遍历。
	values map[string]float64
}

// newRates 职责：创建有序值集；使用：每个局部图；边界：不共享可变 map。
func newRates() *rates { return &rates{values: map[string]float64{}} }

// set 职责：保序写值；使用：构图；边界：已有键位置不变。
func (r *rates) set(k string, v float64) {
	if _, ok := r.values[k]; !ok {
		r.order = append(r.order, k)
	}
	r.values[k] = v
}

// add 职责：累计量；使用：成本/需求展开；边界：零值仍保留键。
func (r *rates) add(k string, v float64) { r.set(k, r.values[k]+v) }

// keys 职责：返回存活键；使用：确定性遍历；边界：调用方不能改变原始顺序。
func (r *rates) keys() []string {
	out := make([]string, 0, len(r.values))
	for _, k := range r.order {
		if _, ok := r.values[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

// clone 职责：复制可变边；使用：剥离图；边界：不共享底层数据。
func (r *rates) clone() *rates {
	out := newRates()
	for _, k := range r.keys() {
		out.set(k, r.values[k])
	}
	return out
}

// node 职责：保存单位净产出关系；使用：成本、求解和取整；边界：速率单位为每秒每机器倍率前。
type node struct {
	// invalid 职责：保存不可执行配方原因；使用：实际需要该节点时报告；边界：未使用的 Mod 占位配方不阻塞其它目标。
	invalid string
	// inputs 职责：单位原料；使用：展开；边界：含喷涂。
	inputs *rates
	// consumers 职责：反向边；使用：剥离；边界：保持构图时序。
	consumers *rates
	// sides 职责：副产物；使用：LP；边界：主产物之外。
	sides *rates
	// rate 职责：净产出倍率；使用：机器换算；边界：未乘工厂速度。
	rate float64
	// recipe 职责：选择的配方序号；使用：结果桥梁；边界：原矿化为 -1。
	recipe int
	// factory 职责：选择的设施；使用：速度与功率；边界：只读数据副本。
	factory Item
	// scheme 职责：归一化配方配置；使用：喷涂；边界：不自动优化。
	scheme RecipeScheme
	// raw 职责：外部原料标记；使用：结果分类；边界：不生成机器。
	raw bool
	// runInputs 职责：单次执行的毛原料（含喷涂）；使用：产线物料流；边界：不参与求解。
	runInputs *rates
	// runOutputs 职责：单次执行的毛产物（含额外产出）；使用：产线物料流；边界：不参与求解。
	runOutputs *rates
	// runNet 职责：单次执行的主产物净产出；使用：把求解量换算为执行次数；边界：非正值时节点已标记 invalid。
	runNet float64
}

// graph 职责：汇总物品图和顺序；使用：一次求解；边界：不跨请求共享。
type graph struct {
	// nodes 职责：节点索引；使用：名称查询；边界：遍历用 order。
	nodes map[string]*node
	// order 职责：JS 产物插入顺序；使用：构图；边界：不排序。
	order []string
	// sequence 职责：剥离产物序列；使用：前向成本；边界：关键节点先隔断。
	sequence []string
	// keys 职责：循环关键节点；使用：LP；边界：采用最大出入度而非 SCC。
	keys []string
	// multi 职责：多来源产物顺序；使用：LP 变量；边界：含未使用支路。
	multi *rates
	// settings 职责：归一设置；使用：倍率；边界：请求局部。
	settings Settings
	// dataset 职责：只读数据；使用：配方反查；边界：不可修改。
	dataset *Dataset
	// scheme 职责：成本设置；使用：价格；边界：不改原请求。
	scheme Scheme
}

// normalizeSettings 职责：补齐参考默认值；使用：构图前；边界：显式无效值报错，不更改系统配置。
func normalizeSettings(s Settings) (Settings, error) {
	defaults := Settings{MiningSpeedOil: 3, MiningSpeedHydrogen: 1, MiningSpeedDeuterium: .05, MiningSpeedGasHydrate: .8, MiningSpeedHelium: .02, MiningSpeedAmmonia: .3, MiningSpeedNitrogen: 1.2, MiningSpeedOxygen: .6, MiningSpeedCarbonDioxide: .4, MiningSpeedSulfurDioxide: .6, CoveredVeinsSmall: 8, CoveredVeinsLarge: 16, MiningEfficiencyLarge: 3, MiningSpeedMultiple: 1, EnemyDropMultiple: 1, IcarusManufacturingSpeed: 1, FractionatingSpeed: 30, StackResearchLab: 15, AccRate: 1, IncRate: 1}
	value := reflect.ValueOf(&s).Elem()
	def := reflect.ValueOf(defaults)
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).Kind() == reflect.Float64 {
			n := value.Field(i).Float()
			if n == 0 {
				value.Field(i).SetFloat(def.Field(i).Float())
			} else if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
				return s, qerror("Q_E_BAD_REQUEST", "量化倍率必须为有限正数")
			}
		}
	}
	if s.FixedNum == nil {
		v := 2
		s.FixedNum = &v
	}
	if *s.FixedNum < 0 || *s.FixedNum > 8 {
		return s, qerror("Q_E_BAD_REQUEST", "小数位需为 0 到 8")
	}
	if s.ProliferateItself == nil {
		v := true
		s.ProliferateItself = &v
	}
	return s, nil
}

// effects 职责：取得 0 到 10 点喷涂表；使用：量化和功率；边界：等级与点数分别处理。
func effects(points int) (increase, speed, power float64) {
	if points == 0 {
		return 1, 1, 1
	}
	if points == 1 {
		return 1.125, 1.25, 1.3
	}
	return 1.15 + .025*float64(points), 1 + .25*float64(points), .9 + .4*float64(points)
}

// sprayCost 职责：选择可用增产剂；使用：单位喷涂原料；边界：1/2/4 点采用原版喷涂次数，其他点需数据支持。
func sprayCost(d *Dataset, points int, self bool) (string, float64, error) {
	id, times := 1141, 12.
	if points == 2 {
		id, times = 1142, 24
	}
	if points >= 4 {
		id, times = 1143, 60
	}
	if points == 10 && strings.Contains(d.Info.Key, "FractionateEverything") {
		times = 24
	}
	if points != 1 && points != 2 && points != 4 && !(points == 10 && strings.Contains(d.Info.Key, "FractionateEverything")) {
		return "", 0, qerror("Q_E_BAD_REQUEST", "当前数据集没有对应点数的增产剂，支持 1、2、4 点")
	}
	item, ok := d.ByID[id]
	if !ok {
		return "", 0, qerror("Q_E_BAD_REQUEST", "数据集缺少增产剂")
	}
	inc, _, _ := effects(points)
	if self {
		times = math.Floor(times*inc - 1 + 1e-6)
	}
	return item.Name, 1 / times, nil
}

// multiplier 职责：按原型应用采集倍率；使用：图速率；边界：Mod 专用原型退回名称兼容并在文档记录。
func multiplier(out float64, f Item, item string, s Settings) float64 {
	switch f.ID {
	case 2301:
		return out * s.MiningSpeedMultiple * s.CoveredVeinsSmall
	case 2316:
		return out * s.MiningSpeedMultiple * s.CoveredVeinsLarge * s.MiningEfficiencyLarge
	case 2307:
		return out * s.MiningSpeedMultiple * s.MiningSpeedOil
	case 2306:
		return out * s.MiningSpeedMultiple
	case 2105:
		rate := 1.
		switch item {
		case "氢":
			rate = s.MiningSpeedHydrogen
		case "重氢":
			rate = s.MiningSpeedDeuterium
		case "可燃冰":
			rate = s.MiningSpeedGasHydrate
		case "氦":
			rate = s.MiningSpeedHelium
		case "氨":
			rate = s.MiningSpeedAmmonia
		}
		return out * s.MiningSpeedMultiple * rate
	}
	if strings.HasSuffix(f.Name, "分馏塔") {
		return out * s.FractionatingSpeed
	}
	if f.Name == "聚束液体汲取设施" {
		return out * s.MiningSpeedMultiple
	}
	if f.Name == "行星基地" {
		return out * s.EnemyDropMultiple
	}
	if f.Name == "伊卡洛斯" {
		return out * s.IcarusManufacturingSpeed
	}
	if f.Name == "大气采集站" {
		rate := 1.
		switch item {
		case "氮":
			rate = s.MiningSpeedNitrogen
		case "氧":
			rate = s.MiningSpeedOxygen
		case "二氧化硫":
			rate = s.MiningSpeedCarbonDioxide
		case "二氧化碳":
			rate = s.MiningSpeedSulfurDioxide
		}
		return out * s.MiningSpeedMultiple * rate
	}
	return out
}

// makeGraph 职责：构建与参考相同的名称图；使用：一次 Solve；边界：缺失来源或非法喷涂立即失败，不猜测配方。
func makeGraph(d *Dataset, scheme Scheme, settings Settings) (*graph, error) {
	s, err := normalizeSettings(settings)
	if err != nil {
		return nil, err
	}
	g := &graph{nodes: map[string]*node{}, order: append([]string{}, d.Order...), multi: newRates(), settings: s, dataset: d, scheme: scheme}
	for _, item := range d.Items {
		if s.MineralizeList[item.Name] && len(d.Choices[item.Name]) == 0 {
			g.order = append(g.order, item.Name)
		}
	}
	for _, name := range g.order {
		g.nodes[name] = &node{inputs: newRates(), consumers: newRates(), sides: newRates(), runInputs: newRates(), runOutputs: newRates(), recipe: -1}
	}
	for _, name := range g.order {
		n := g.nodes[name]
		if s.MineralizeList[name] {
			n.raw = true
			n.rate = math.Pow(100000000, float64(*s.FixedNum+1))
			continue
		}
		choice := scheme.ItemRecipeChoices[name]
		if choice == 0 {
			choice = 1
		}
		if choice < 1 || choice > len(d.Choices[name]) {
			return nil, qerror("Q_E_NO_RECIPE", fmt.Sprintf("物品 %d %s 的配方选项 %d 不存在", d.ByName[name].ID, name, choice))
		}
		index := d.Choices[name][choice-1]
		r := d.Recipes[index]
		rs := RecipeScheme{}
		if index < len(scheme.SchemeForRecipe) {
			rs = scheme.SchemeForRecipe[index]
		}
		if r.Proliferator == 8 && rs.ProductionMode == 0 {
			rs.ProductionMode = 4
		}
		if rs.ProductionMode > 0 && rs.IncreasePoints == 0 {
			rs.IncreasePoints = 4
			if strings.Contains(d.Info.Key, "FractionateEverything") {
				rs.IncreasePoints = 10
			}
		}
		if rs.Building < 0 || rs.Building >= len(r.Factories) || rs.ProductionMode < 0 || rs.ProductionMode > 4 || rs.IncreasePoints < 0 || rs.IncreasePoints > 10 {
			return nil, qerror("Q_E_BAD_REQUEST", fmt.Sprintf("配方 %d 的设施或喷涂配置无效", r.ID))
		}
		if rs.ProductionMode > 0 && r.Proliferator&(1<<uint(rs.ProductionMode-1)) == 0 {
			return nil, qerror("Q_E_BAD_REQUEST", fmt.Sprintf("配方 %d 不支持所选增产模式", r.ID))
		}
		n.recipe = index
		n.factory = d.ByID[r.Factories[rs.Building]]
		n.scheme = rs
		output := 0.
		products := newRates()
		for i, id := range r.Results {
			products.set(d.ByID[id].Name, r.ResultCounts[i])
			if d.ByID[id].Name == name {
				output = r.ResultCounts[i]
			}
		}
		if strings.Contains(d.Info.Key, "TheyComeFromVoid") && s.BlueBuff && len(r.Items) >= 2 {
			products.set(d.ByID[r.Items[0]].Name, r.ResultCounts[0])
		}
		total := 0.
		for i, id := range r.Items {
			v := r.ItemCounts[i] / output
			n.inputs.set(d.ByID[id].Name, v)
			n.runInputs.add(d.ByID[id].Name, r.ItemCounts[i])
			total += v
		}
		n.rate = output
		produce := 1.
		if rs.ProductionMode > 0 && rs.IncreasePoints > 0 {
			spray, cost, err := sprayCost(d, rs.IncreasePoints, *s.ProliferateItself)
			if err != nil {
				return nil, err
			}
			n.inputs.add(spray, total*cost)
			n.runInputs.add(spray, total*output*cost)
			inc, speed, _ := effects(rs.IncreasePoints)
			switch rs.ProductionMode {
			case 1:
				n.rate *= speed * s.AccRate
			case 2:
				produce = inc * s.IncRate
				n.rate *= produce
			case 3:
				produce = speed * s.AccRate
				n.rate *= produce
			case 4:
				n.rate *= float64(rs.IncreasePoints) / 10
			}
		}
		for _, k := range n.inputs.keys() {
			n.inputs.values[k] /= produce
		}
		for _, k := range products.keys() {
			n.runOutputs.add(k, products.values[k]*produce)
		}
		n.runNet = output*produce - n.runInputs.values[name]
		n.rate /= r.TimeSpend / 60
		selfCost := 0.
		if v, ok := n.inputs.values[name]; ok {
			if v >= 1 {
				n.invalid = fmt.Sprintf("配方 %d 对 %s 无净产出", r.ID, name)
			}
			used := 1 / (1 - v)
			n.rate /= used
			selfCost = used - 1
			delete(n.inputs.values, name)
			for _, k := range n.inputs.keys() {
				n.inputs.values[k] *= used
			}
		}
		for _, k := range n.inputs.keys() {
			material, ok := g.nodes[k]
			if !ok {
				return nil, qerror("Q_E_NO_RECIPE", "原料没有来源: "+k)
			}
			material.consumers.set(name, 1/n.inputs.values[k])
		}
		for _, product := range products.keys() {
			if product == name {
				continue
			}
			side := products.values[product] / (output - selfCost)
			if material, ok := n.inputs.values[product]; ok {
				if math.Min(side, material) == material {
					n.sides.set(product, side-material)
					n.inputs.values[product] = 0
					g.multi.set(product, 1)
				} else {
					n.inputs.values[product] -= side
				}
			} else {
				n.sides.set(product, side)
				g.multi.set(product, 1)
			}
		}
		n.rate = multiplier(n.rate, n.factory, name, s)
		if n.rate <= 0 || math.IsNaN(n.rate) || math.IsInf(n.rate, 0) {
			n.invalid = "净产出无效: " + name
		}
	}
	g.peel()
	return g, nil
}

// peel 职责：按剥离及最大出入度切环；使用：成本前向序列；边界：并列度数保留首次出现顺序。
func (g *graph) peel() {
	inputs, outputs := map[string]*rates{}, map[string]*rates{}
	for _, name := range g.order {
		inputs[name] = g.nodes[name].inputs.clone()
		outputs[name] = g.nodes[name].consumers.clone()
	}
	g.sequence = make([]string, len(g.order))
	left, right := 0, len(g.order)-1
	remove := func(name string) {
		for _, k := range inputs[name].keys() {
			if outputs[k] != nil {
				delete(outputs[k].values, name)
			}
		}
		for _, k := range outputs[name].keys() {
			if inputs[k] != nil {
				delete(inputs[k].values, name)
			}
		}
		delete(inputs, name)
		delete(outputs, name)
	}
	var find func(string, bool)
	find = func(name string, production bool) {
		in, out := inputs[name], outputs[name]
		if in == nil {
			return
		}
		if !production && len(in.values) == 0 {
			next := out.keys()
			remove(name)
			g.sequence[left] = name
			left++
			for _, k := range next {
				find(k, false)
			}
		} else if production && len(out.values) == 0 {
			next := in.keys()
			remove(name)
			g.sequence[right] = name
			right--
			for _, k := range next {
				find(k, true)
			}
		}
	}
	for len(inputs) > 0 {
		for _, name := range g.order {
			if inputs[name] == nil {
				continue
			}
			if len(inputs[name].values) == 0 {
				find(name, false)
			} else if len(outputs[name].values) == 0 {
				find(name, true)
			}
		}
		if len(inputs) == 0 {
			break
		}
		best := ""
		count := -1
		for _, name := range g.order {
			if inputs[name] != nil {
				degree := len(inputs[name].values) + len(outputs[name].values)
				if degree > count {
					best, count = name, degree
				}
			}
		}
		g.keys = append(g.keys, best)
		g.sequence[left] = best
		left++
		remove(best)
	}
}
