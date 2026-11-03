package build

// 文件职责：将配方同物品的毛输入/输出抵消量落实为本地优先回流。
// 使用：直连带和总线模式在机器行装箱前创建、全图寻路后复核；边界：不改变规范净流，不伪造启动种子或持续供料。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// machineRecycle 职责：保存单行单物品的优先回收证据；使用：路由和最终实体核验；边界：route是局部毛流，不属于规范FlowGraph边。
type machineRecycle struct {
	row                   *transportRow
	item                  int
	rate, netInput        float64
	netOutput             float64
	source, surplus       []*placed
	intake                []*placed
	outputGate, inputGate *placed
	fresh, returned       *placed
	outputSlot            int8
	route                 *transportRoute
}

// recipeHasMachineRecycle 职责：识别配方层面同物品输入与输出；使用：选择支持本地回流的模式；边界：不把不同生产节点的SCC误作设备内部循环。
func recipeHasMachineRecycle(recipe quant.Recipe) bool {
	for _, input := range recipe.Items {
		for _, output := range recipe.Results {
			if input == output {
				return true
			}
		}
	}
	return false
}

// prepareMachineRecycles 职责：为同物品创建先回收后出货、先回料后补新的双优先门；使用：机器行完成输入stub之后；边界：两个门都使用已核对Model38地面端口，不通过普通T接假设优先级。
func (b *builder) prepareMachineRecycles(row *transportRow) error {
	if !recipeHasMachineRecycle(row.recipe) {
		return nil
	}
	flows, err := productionMaterialFlows(row.info, row.recipe)
	if err != nil {
		return err
	}
	share := float64(row.count) / float64(row.info.MachineCount)
	inputCursor := math.Inf(1)
	for _, input := range row.inputs {
		inputCursor = math.Min(inputCursor, input.a.x-2)
	}
	// 普通产物先在机器旁完成过滤，回流门及其西向回收口再放到整个有限
	// 过滤区之外；否则外轨产物横带会穿过邻近内轨回收门的真实建造盒。
	outputCursor := inputCursor - 4*float64(len(flows.recycle)) - 4
	for _, route := range row.outputs {
		if flows.recycle[route.edge.ItemID] <= 0 {
			capacity := inserterCapacityPerMinute(b.setting.InserterLv, 1)
			outputCursor -= float64(max(1, int(math.Ceil((route.rate-1e-8)/capacity))) + 3)
		}
	}
	for index, item := range row.recipe.Results {
		rate := flows.recycle[item] * share
		if rate <= 0 {
			continue
		}
		inputIndex := -1
		for i, candidate := range row.recipe.Items {
			if candidate == item {
				inputIndex = i
				break
			}
		}
		stub := row.inputStubs[item]
		output := row.line.outputForItem(item)
		if inputIndex < 0 || len(stub) == 0 || len(output) == 0 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("配方%d回流物品%d缺少独立机器原料/出货带", row.recipe.ID, item)}
		}
		grossIn, grossOut := flows.inputs[inputIndex]*share, flows.outputs[index]*share
		if grossIn > b.logistics.BeltCapacityPerMinute+1e-6 || grossOut > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("配方%d回流物品%d机器行毛输入/输出%.3f/%.3f超过单带容量%.3f/min", row.recipe.ID, item, grossIn, grossOut, b.logistics.BeltCapacityPerMinute)}
		}
		entry := &machineRecycle{row: row, item: item, rate: rate, netInput: math.Max(0, grossIn-grossOut), netOutput: math.Max(0, grossOut-grossIn)}
		// 输入优先门也必须声明真实物品。使用 0 会让粘贴后的游戏过滤器
		// 失去物品语义，且无法和规范 lane 的物品诊断对应。
		entry.inputGate = b.recycleGate(inputCursor, stub[0].a.y, 3, item)
		entry.fresh = b.make(2000+b.setting.BeltLv, inputCursor-1, stub[0].a.y, 0)
		entry.fresh.output, entry.fresh.record.OutputToSlot = entry.inputGate, 2
		entry.fresh.record.Parameters = []int32{int32(item), int32(math.Ceil(entry.netInput))}
		entry.returned = b.make(2000+b.setting.BeltLv, inputCursor, stub[0].a.y+1, 0)
		entry.returned.output, entry.returned.record.OutputToSlot = entry.inputGate, 3
		entry.intake = b.straight(inputCursor+1, stub[0].a.y, int(math.Round(stub[0].a.x-inputCursor-1)), 2)
		entry.intake[0].input, entry.intake[0].record.InputFromSlot = entry.inputGate, 0
		connect(entry.intake[len(entry.intake)-1], stub[0])
		entry.intake = append(entry.intake, stub...)
		row.inputs[item] = entry.fresh
		// 出货门放在所有原料接口西侧，避免其上下优先口落到邻近原料stub上。
		x := outputCursor
		for _, previous := range row.recycles {
			x = math.Min(x, previous.outputGate.a.x-4)
		}
		trunk := output[0]
		for trunk.a.x > x+1 {
			belt := b.make(2000+b.setting.BeltLv, trunk.a.x-1, trunk.a.y, 0)
			connect(trunk, belt)
			entry.source = append(entry.source, belt)
			trunk = belt
		}
		direction := 1.
		surplusSlot := int8(1)
		if trunk.a.y < row.line.machines[0].a.y {
			direction, surplusSlot = -1, 3
		}
		// 优先回流沿原出货带向西继续，净产品向机器侧引出；不能把北侧
		// 优先端放在下一种外轨产物的横带上（真实配方58的石墨轨）。
		entry.outputSlot = 2
		entry.outputGate = b.recycleGate(x, trunk.a.y, entry.outputSlot, item)
		trunk.output, trunk.record.OutputToSlot = entry.outputGate, 0
		start := b.make(2000+b.setting.BeltLv, x-1, trunk.a.y, 0)
		start.input, start.record.InputFromSlot = entry.outputGate, entry.outputSlot
		start.record.Parameters = []int32{int32(item), int32(math.Ceil(rate))}
		if entry.netOutput > 1e-8 {
			belt := b.make(2000+b.setting.BeltLv, x, trunk.a.y-direction, 0)
			belt.input, belt.record.InputFromSlot = entry.outputGate, surplusSlot
			entry.surplus = []*placed{belt}
		}
		entry.route = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("machine-recycle:%d:%d:%d", row.owner, len(b.machineRecycles), item), ItemID: item, FlowKind: "machine-recycle"}, rate: rate, from: row, to: row, start: start, end: entry.returned, path: []*placed{start}}
		row.recycles = append(row.recycles, entry)
		b.machineRecycles = append(b.machineRecycles, entry)
		inputCursor -= 4
		if entry.netInput <= 1e-8 {
			message := fmt.Sprintf("配方%d物品%d使用本地优先回流；净输入为零，启动前需在该行机器或带图标的预充口加入种子物料，稳定后不得持续灌入额外物料；离线未验证启动库存和运行时序", row.recipe.ID, item)
			if !containsExact(b.warnings, message) {
				b.warnings = append(b.warnings, message)
			}
		} else {
			message := fmt.Sprintf("配方%d物品%d毛输入%.3f/min、毛回收%.3f/min；净补料%.3f/min通过本地优先门接入，持续供料与启动时序未在游戏内验证", row.recipe.ID, item, flows.inputs[inputIndex]*share, rate, entry.netInput)
			if !containsExact(b.warnings, message) {
				b.warnings = append(b.warnings, message)
			}
		}
	}
	return nil
}

// recycleGate 职责：创建Model38、Yaw90的地面四向优先门；使用：回流产出与新料合流；边界：slot0东/1南/2西/3北，优先参数来自已核实BuildingParameters六参数协议。
func (b *builder) recycleGate(x, y float64, priority int8, item int) *placed {
	gate := b.make(2020, x, y, 0)
	gate.record.ModelIndex = 38
	gate.record.Parameters = make([]int32, 6)
	gate.record.Parameters[priority] = 1
	gate.record.Parameters[4] = 1 << priority
	gate.record.Parameters[5] = int32(item)
	gate.record.FilterID = int16(item)
	return gate
}

// validateMachineRecycles 职责：从实际实体重建双优先门、回料带和机器毛流能力；使用：mode1/6最终物流出口；边界：回流负载叠加规范净流，不能只验证一条孤立回流链或伪造规范边。
func (b *builder) validateMachineRecycles(indices map[*placed]int, beltLoads map[*placed]float64) error {
	for _, entry := range b.machineRecycles {
		if err := b.checkGeneration("机器内部回流核验"); err != nil {
			return err
		}
		if entry == nil || entry.row == nil || entry.row.line == nil || entry.route == nil || !finitePositive(entry.rate) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器内部回流缺少实际行、物料或实体路线"}
		}
		flows, err := productionMaterialFlows(entry.row.info, entry.row.recipe)
		if err != nil {
			return err
		}
		want := flows.recycle[entry.item] * float64(entry.row.count) / float64(entry.row.info.MachineCount)
		if !finitePositive(want) || math.Abs(entry.rate-want) > math.Max(1e-6, want*1e-8) {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "机器回流份额与毛输入/输出的共同部分不守恒"}
		}
		if err := validateRecycleGate(entry.inputGate, 3, entry.item, indices); err != nil {
			return err
		}
		if err := validateRecycleGate(entry.outputGate, entry.outputSlot, entry.item, indices); err != nil {
			return err
		}
		route := entry.route
		if len(route.path) < 2 || route.start != route.path[0] || route.end != entry.returned || route.path[len(route.path)-1] != entry.returned || route.start.input != entry.outputGate || route.start.record.InputFromSlot != entry.outputSlot {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流带没有从优先出货口连接到优先回料口"}
		}
		if err := validateBusFabricChain(route.path, indices); err != nil {
			return err
		}
		if entry.returned.output != entry.inputGate || entry.returned.record.OutputToSlot != 3 || entry.fresh.output != entry.inputGate || entry.fresh.record.OutputToSlot != 2 || entry.fresh == entry.returned || !hasPortItem(entry.fresh, entry.item) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流优先入口或独立新料/预充入口已断开"}
		}
		if _, exists := indices[entry.fresh]; !exists {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流预充入口不属于当前蓝图"}
		}
		if len(entry.intake) == 0 || entry.intake[0].input != entry.inputGate || entry.intake[0].record.InputFromSlot != 0 {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流合流门没有连接机器原料带"}
		}
		if err := validateBusFabricChain(entry.intake, indices); err != nil {
			return err
		}
		var input []*placed
		for index, item := range entry.row.recipe.Items {
			if item == entry.item {
				input = entry.row.line.inputs[index]
			}
		}
		output := entry.row.line.outputForItem(entry.item)
		producer := append(append([]*placed{}, output...), entry.source...)
		for i, j := 0, len(output)-1; i < j; i, j = i+1, j-1 {
			producer[i], producer[j] = producer[j], producer[i]
		}
		if len(producer) == 0 || producer[len(producer)-1].output != entry.outputGate || producer[len(producer)-1].record.OutputToSlot != 0 {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器毛出货带没有抵达回流优先门"}
		}
		if err := validateBusFabricChain(producer, indices); err != nil {
			return err
		}
		surplusSlot := int8(1)
		if entry.outputGate.a.y < entry.row.line.machines[0].a.y {
			surplusSlot = 3
		}
		if entry.netOutput > 1e-8 && (len(entry.surplus) == 0 || entry.surplus[0].input != entry.outputGate || entry.surplus[0].record.InputFromSlot != surplusSlot) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流物品的净产物没有独立出口"}
		}
		// 回流链、共同毛出货段和共同机器进料段各叠加一次；新料与净出口按独立容量
		// 核算，不能以未承诺的逐帧速度限制假装库存和启动已经验证。
		seen := map[*placed]bool{}
		parts := append(append(append(append([]*placed{}, producer...), route.path...), entry.intake...), input...)
		for _, belt := range parts {
			if !seen[belt] {
				beltLoads[belt] += entry.rate
				seen[belt] = true
			}
		}
		if entry.rate+entry.netOutput > b.logistics.BeltCapacityPerMinute+1e-6 || entry.rate+entry.netInput > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "机器回流物品毛输入或毛输出超过所选单带能力"}
		}
	}
	if len(b.machineRecycles) > 0 {
		checks := b.logistics.Checks[:0]
		for _, check := range b.logistics.Checks {
			if check.RuleID != "machine-local-recycle" && check.RuleID != "machine-recycle-startup" {
				checks = append(checks, check)
			}
		}
		b.logistics.Checks = append(checks, LogisticsCheck{RuleID: "machine-local-recycle", Status: "passed", Message: fmt.Sprintf("%d条机器内部毛流已接入本地双优先回收门；规范净边不变，毛输入/输出和实际共享带独立核算", len(b.machineRecycles))}, LogisticsCheck{RuleID: "machine-recycle-startup", Status: "unverified", Message: "回料优先于新料、循环优先于净出货；零净输入的自催化物品仍需启动预充，离线不验证库存、回压或连续运行时序"})
	}
	return nil
}

// validateRecycleGate 职责：逐参数确认分流器优先槽与预设没有被替换；使用：最终回流核验；边界：不接受仅有图标而缺少优先配置的普通分流器。
func validateRecycleGate(gate *placed, priority int8, item int, indices map[*placed]int) error {
	if gate == nil || gate.record.ItemID != 2020 || gate.record.ModelIndex != 38 || gate.record.Yaw != 90 || len(gate.record.Parameters) != 6 || int(gate.record.FilterID) != item {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流优先门原型、模型、方向或过滤物品不符"}
	}
	if _, exists := indices[gate]; !exists {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流优先门引用蓝图外建筑"}
	}
	for slot := 0; slot < 4; slot++ {
		want := int32(0)
		if int(priority) == slot {
			want = 1
		}
		if gate.record.Parameters[slot] != want {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流优先槽配置缺失或错误"}
		}
	}
	if gate.record.Parameters[4] != 1<<priority || gate.record.Parameters[5] != int32(item) {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流优先预设或物品过滤配置错误"}
	}
	return nil
}
