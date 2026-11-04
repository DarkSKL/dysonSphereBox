package build

// 文件职责：把单进单出密集机器单元装配为相向双排，共用产物带并连续供料。
// 使用：密集局部装配；边界：暂不处理喷涂、多物料和回流，不改变总机器数或端口能力。

import (
	"math"
	"slices"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// canPairDenseLine 职责：判断已实现的双排局部端口是否适用于当前请求；使用：机器装配前；边界：不把未覆盖配方假装成可共带的单物料。
func (b *builder) canPairDenseLine(p quant.Production, recipe quant.Recipe, count int) bool {
	return b.adapter != nil && b.adapter.density == densityDense && count > 1 && densePairRecipe(p, recipe) && densePairPortsSupported(p, recipe, b.setting)
}

// densePairPortsSupported 职责：限定现有相向共带结构只能使用已经验证的两侧单臂；使用：双排选择和拆列预算；边界：必要并联臂在相向排会发生动态碰撞，必须由通用密集单排真实实现，不压低流量。
func densePairPortsSupported(p quant.Production, recipe quant.Recipe, setting Setting) bool {
	ports := planMachineInserters(p, recipe, setting, 1)
	return ports.capacityError == nil && len(ports.inputs) == 1 && len(ports.outputs) == 1 && !ports.inputs[0].top && ports.outputs[0].top && len(ports.inputs[0].slots) == 1 && len(ports.outputs[0].slots) == 1
}

// densePairRecipe 职责：声明双排局部拓扑支持范围；使用：容量拆分与实体装配共同判断；边界：不代表最终布局或游戏验收通过。
func densePairRecipe(p quant.Production, recipe quant.Recipe) bool {
	return (p.MachineType == 1 || p.MachineType == 2 || p.MachineType == 4) && p.IncreasePoints == 0 && len(recipe.Items) == 1 && len(recipe.Results) == 1 && recipe.Items[0] != recipe.Results[0]
}

// layoutLineCounts 职责：按局部单元的实际排数拆分容量；使用：四种模式的布局准备；边界：每排仍遵守用户机器数上限，共用物料带容量不翻倍。
func layoutLineCounts(p quant.Production, recipe quant.Recipe, setting Setting, speed float64, density string) ([]int, error) {
	if density == densityDense && (densePairRecipe(p, recipe) && densePairPortsSupported(p, recipe, setting) || referenceLowRecipe(p, recipe) && referenceSingleArmsSufficient(p, recipe, setting.InserterLv)) {
		setting.MaxMachineInALine = min(10000, setting.MaxMachineInALine*2)
		setting.SmelterMaxCount = min(10000, setting.SmelterMaxCount*2)
		setting.WorkbenchMaxCount = min(10000, setting.WorkbenchMaxCount*2)
		setting.ChemicalMaxCount = min(10000, setting.ChemicalMaxCount*2)
	}
	return lineCounts(p, recipe, setting, speed)
}

// completeDensePair 职责：增加相向机器排与连续 U 形输入，共用已存在的独立产物带；使用：首排机器臂创建之后；边界：旋转机器与臂后按真实坐标重绑带引用，最后由完整终检验证。
func (b *builder) completeDensePair(l *line, count int, conf geometry, ports machineInserterPlan, spacing, pre float64) error {
	l.densePaired = true
	if !ports.outputs[0].top || ports.inputs[0].top {
		return &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "当前双排要求输入和输出位于机器相反两侧"}
	}
	lower, middle := l.inputs[0], l.outputs[0]
	axisY := middle[0].a.y
	positions := make([]float64, len(lower))
	for index, belt := range lower {
		positions[index] = belt.a.x
	}
	upper := b.denseTrack(positions, 2*axisY-lower[0].a.y)
	for index := 0; index < count; index++ {
		if err := b.checkGeneration("密集双排机器与端口"); err != nil {
			return err
		}
		original := l.machines[index]
		begin := len(b.all)
		machine := b.make(int(original.record.ItemID), original.a.x, original.a.y, 0)
		machine.record = original.record
		machine.densityPattern = original.densityPattern
		machine.record.Parameters = append([]int32(nil), original.record.Parameters...)
		if err := b.buildMachineInserterPort(machine, lower, ports.inputs[0], conf, index, spacing, pre, false, 0); err != nil {
			return err
		}
		if err := b.buildMachineInserterPort(machine, middle, ports.outputs[0], conf, index, spacing, pre, true, 0); err != nil {
			return err
		}
		centerX := machine.a.x
		entities := b.all[begin:]
		rotatePlacedQuarter(entities, 2)
		shift(entities, 2*centerX, 2*axisY, 0)
		for _, arm := range entities[1:] {
			track := upper
			input := arm.output == machine
			if !input {
				track = middle
			}
			// 带侧原始端点保留了机器槽位外形修正；索引绑定使用旋转后的原带节点坐标。
			old := arm.input
			if !input {
				old = arm.output
			}
			x := 2*centerX - old.a.x
			anchorIndex := sort.Search(len(track), func(i int) bool { return track[i].a.x >= x-1e-6 })
			if anchorIndex == len(track) || math.Abs(track[anchorIndex].a.x-x) > 1e-6 {
				return &blueprint.Error{Code: "GEN_E_DENSITY_PORT", Message: "密集双排镜像槽位缺少真实带节点"}
			}
			anchor := track[anchorIndex]
			if input {
				arm.input = anchor
			} else {
				arm.output = anchor
			}
		}
		l.machines = append(l.machines, machine)
	}
	// 输入路径从下排右端进入，绕左侧转向，再沿上排到达右端；所有机器共用同一物料且总量已受单带预算限制。
	reverse(lower)
	slices.Reverse(lower)
	end := lower[len(lower)-1]
	points := manhattan(end.a, vector{end.a.x - 2, end.a.y, 0}, true)
	points = append(points, manhattan(points[len(points)-1], vector{end.a.x - 2, upper[0].a.y, 0}, false)[1:]...)
	points = append(points, manhattan(points[len(points)-1], upper[0].a, true)[1:]...)
	bridge := b.byPath(points[1 : len(points)-1])
	connect(end, bridge[0])
	connect(bridge[len(bridge)-1], upper[0])
	l.inputs[0] = append(append(lower, bridge...), upper...)
	if b.adapter.mode == 1 {
		// 直连模式以西向输入短桩接入；增加独立南侧回接，避免短桩覆盖已经反向运行的下排物料带。
		head := l.inputs[0][0]
		points := manhattan(vector{0, head.a.y - 2, 0}, vector{head.a.x + 2, head.a.y - 2, 0}, true)
		points = append(points, manhattan(points[len(points)-1], vector{head.a.x + 2, head.a.y, 0}, false)[1:]...)
		points = append(points, manhattan(points[len(points)-1], head.a, true)[1:]...)
		stub := b.byPath(points[:len(points)-1])
		connect(stub[len(stub)-1], head)
		l.inputs[0] = append(stub, l.inputs[0]...)
		// 共用产物带跨过西侧 U 形输入时使用真实低桥，过滤端置于整个块外；不从第二排机器中抽出过滤臂。
		outputHead := middle[0]
		x, y := outputHead.a.x, outputHead.a.y
		points = []vector{{x - 5, y, 0}, {x - 4, y, 0}, {x - 3, y, 1}, {x - 2, y, 1}, {x - 1, y, 1}}
		outlet := b.byPath(points)
		connect(outlet[len(outlet)-1], outputHead)
		l.outputs[0] = append(outlet, middle...)
		l.output = l.outputs[0]
	}
	return nil
}
