package build

// 文件职责：承载蓝图生成前的参数规范化与产线拆列计算。
// 使用：由统一生成入口和各模式布局阶段复用，保证所有模式使用同一份输入边界与带宽约束。
// 边界：不选择具体模式、不放置建筑、不装配物流网络，也不发布预览事件。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"fmt"
	"math"
	"reflect"
)

// normalizeSetting 职责：补齐参考默认值并限制生成规模；使用：布局前；边界：不截断请求机器总数。
func normalizeSetting(s Setting) (Setting, error) {
	defaults := Setting{MaxMachineInALine: 15, BeltLv: 1, InserterLv: 1, MaxLays: 3, SmelterMaxCount: 15, WorkbenchMaxCount: 15, RefineryMaxCount: 15, ChemicalMaxCount: 15, ColliderMaxCount: 15, MatrixMaxCount: 999, MergeType: 2, BlockWidthLimit: 50, LocalStationHitRadius: 7}
	v, d := reflect.ValueOf(&s).Elem(), reflect.ValueOf(defaults)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Int:
			if field.Int() < 0 || field.Int() > 10000 {
				return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "布局整数参数超限"}
			}
			if field.Int() == 0 {
				field.Set(d.Field(i))
			}
		case reflect.Float64:
			n := field.Float()
			if n < 0 || n > 1000000 || math.IsNaN(n) || math.IsInf(n, 0) {
				return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "布局数值无效"}
			}
			if n == 0 {
				field.Set(d.Field(i))
			}
		}
	}
	if s.NeedPower == nil {
		value := true
		s.NeedPower = &value
	}
	if s.Mix2 == nil {
		value := true
		s.Mix2 = &value
	}
	if s.BeltLv > 3 || s.InserterLv > 4 || s.MaxLays > 15 || s.MergeType > 2 || s.BlockWidthLimit < 10 {
		return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "带/分拣器等级、堆叠或布局方式无效"}
	}
	// 旧工作区的 BeltSpeed 只作读取兼容，不能覆盖所选等级的单带能力。
	capacity, err := transport.BeltCapacityPerMinute(s.BeltLv)
	if err != nil {
		return s, err
	}
	s.BeltSpeed = capacity
	return s, nil
}

// EstimateFactoryAccessories 按施工档位的机器行、底层机位和物料口估算分拣器及喷涂机数量。
// 使用：多星分工初排电源。边界：布局可能拆成更多行、启用不同轨道或升档，最终功率始终按实物结算。
func EstimateFactoryAccessories(data *quant.Dataset, rows []quant.Production, belt, inserter int) (arms, coaters int) {
	return estimateFactoryRows(data, rows, belt, inserter, nil)
}

// EstimateFactoryGroundSpans 按普通机行的水平间距累计各工区所需的地面行长，以一个熔炉机位为单位。
// 使用：大单初排时比较不同设备的占地；边界：整块未能估算时按原机器数分担，施工仍核验完整布局。
func EstimateFactoryGroundSpans(data *quant.Dataset, rows []quant.Production, belt, inserter int) []float64 {
	spans := make([]float64, len(rows))
	for i, row := range rows {
		spans[i] = float64(row.MachineCount)
	}
	estimateFactoryRows(data, rows, belt, inserter, spans)
	return spans
}

// estimateFactoryRows 共用真实拆行、端口和堆叠规则。需要初排土地份额时才记录地面行长。
func estimateFactoryRows(data *quant.Dataset, rows []quant.Production, belt, inserter int, spans []float64) (arms, coaters int) {
	setting, err := normalizeSetting(Setting{BeltLv: belt, InserterLv: inserter})
	if err != nil {
		return 0, 0
	}
	facts, err := loadAssemblyFacts()
	if err != nil {
		return 0, 0
	}
	ordinary := stationRowPlans()[0]
	for i, row := range rows {
		index, ok := data.RecipeByID[row.RecipeID]
		if !ok || row.MachineType < 1 || row.MachineType > 6 {
			continue
		}
		recipe := data.Recipes[index]
		counts, err := lineCounts(row, recipe, setting, data.ByID[row.MachineID].Speed)
		if err != nil {
			continue
		}
		// 同一带宽组可能拆成多行。每行分别配喷涂机，研究站按该行实际层数配底层分拣器。
		span := ordinary.spec.MaxSpan
		if recipeHasMachineRecycle(recipe) {
			span = ordinary.recycleSpan
		}
		counts, err = singleRowCounts(row, recipe, setting, counts, span)
		if err != nil {
			continue
		}
		ground, measured := 0, 0
		for _, count := range counts {
			ports := planMachineInserters(row, recipe, setting, count)
			if ports.capacityError != nil {
				continue
			}
			bases := (count + ports.layers - 1) / ports.layers
			ground += bases
			measured += count
			for _, p := range ports.inputs {
				arms += bases * len(p.slots)
			}
			for _, p := range ports.outputs {
				arms += bases * len(p.slots)
			}
			if productionSprayItem(row) != 0 {
				coaters += len(recipe.Items)
			}
		}
		if spans != nil && measured == row.MachineCount && ground > 0 {
			pitch := machineSpacing(row.MachineType, facts.geometry[row.MachineType], setting.extraSpace(row.MachineType))
			spans[i] = float64(ground) * pitch / facts.geometry[1].Space
		}
	}
	return arms, coaters
}

// lineCounts 职责：按单机满负荷毛流、真实带宽和列上限均匀拆列；使用：每条量化产线；边界：不把期望速率均摊到整数机器当作已降速，不丢机器。
func lineCounts(p quant.Production, r quant.Recipe, s Setting, speed float64) ([]int, error) {
	return lineCountsWithStrategy(p, r, s, speed, false)
}

// lineCountsWithStrategy 职责：在同一真实容量和行上限内选择均衡或优先填满分组；使用：mode1密铺独立候选；边界：默认仍均衡，策略不改变机器数、每台能力或请求设置。
func lineCountsWithStrategy(p quant.Production, r quant.Recipe, s Setting, speed float64, capacityFill bool) ([]int, error) {
	capacity, err := transport.BeltCapacityPerMinute(s.BeltLv)
	if err != nil {
		return nil, err
	}
	if p.MachineCount <= 0 || p.MachineCount > 10000 || r.TimeSpend <= 0 || !finitePositive(speed) {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "机器数、配方周期或设备速度无效"}
	}
	maxUse := 0.
	for _, n := range r.ItemCounts {
		maxUse = math.Max(maxUse, n)
	}
	for _, n := range r.ResultCounts {
		maxUse = math.Max(maxUse, n)
	}
	acc := 1.
	if p.ProductionMode == 1 && p.IncreaseLv > 0 {
		rates := []float64{0, .25, .5, 1}
		acc += rates[p.IncreaseLv]
	}
	perMachine := 60 / (r.TimeSpend / 60) * speed * maxUse * acc
	if !finitePositive(perMachine) {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 没有有效的单机物料流量", r.ID)}
	}
	theory := int(math.Floor((capacity + 1e-6) / perMachine))
	limit := s.maxCount(p.MachineType)
	if s.MaxMachineInALine > 0 && p.MachineType != 6 {
		limit = min(limit, s.MaxMachineInALine)
	}
	limit = min(limit, theory)
	if labSevenPorts(p, r) {
		ports := planLabSevenInserters(p, r, s, p.MachineCount)
		if ports.capacityError != nil {
			return nil, ports.capacityError
		}
		limit = min(limit, ports.layers)
	}
	// 喷涂及自定义倍率以可信计划的单机毛流为准；每种物品独立占带，
	// 不能累加不同产物，也不能用期望流量/取整机器数低估整行满载需求。
	inputs, outputs, err := machineInserterRates(p, r)
	if err != nil {
		return nil, err
	}
	for _, rate := range inputs {
		perMachine = math.Max(perMachine, rate)
	}
	for _, rate := range outputs {
		perMachine = math.Max(perMachine, rate)
	}
	denominator := p.ExactMachineCount
	if denominator <= 0 {
		denominator = float64(p.MachineCount)
	}
	for _, rate := range p.Inputs {
		if p.Capacity != nil && rate.ItemID == productionSprayItem(p) && productionRecipeInputCount(r, rate.ItemID) > 0 {
			// 同物品的原料与供剂走两条真实带；已验证的配置毛流分别约束容量，不能重新合计到一条机器带。
			for _, role := range []productionInputRole{recipeInputRole, sprayInputRole} {
				configured, err := configuredRoleInputRate(p, r, rate.ItemID, role)
				if err != nil {
					return nil, err
				}
				perMachine = math.Max(perMachine, configured/float64(p.MachineCount))
			}
			continue
		}
		perMachine = math.Max(perMachine, rate.RatePerMinute/denominator)
	}
	for _, rate := range p.Outputs {
		perMachine = math.Max(perMachine, rate.RatePerMinute/denominator)
	}
	limit = min(limit, int(math.Floor((capacity+1e-6)/perMachine)))
	if limit <= 0 {
		return nil, &blueprint.Error{Code: "GEN_E_OVERFLOW", Message: "带速不足以支持一台机器，请升级传送带"}
	}
	lines := int(math.Ceil(float64(p.MachineCount) / float64(limit)))
	balanced := int(math.Ceil(float64(p.MachineCount) / float64(lines)))
	if capacityFill {
		balanced = limit
	}
	result := []int{}
	for left := p.MachineCount; left > 0; {
		n := min(left, balanced)
		result = append(result, n)
		left -= n
	}
	return result, nil
}
