package build

// 文件职责：按真实净补料需求选择回流入口，并独立核验实体链。
// 使用：机器回流装配及终检；边界：零净输入才可省去新料合流门，启动种子仍由玩家预充。

import (
	"DysonSphereBox/helper/blueprint"
	"math"
)

// prepareRecycleIntake 职责：密铺零补料直接回机，其余情况创建回料优先的新料合流门；使用：回流出口装配前；边界：不增加规范外供或降低回收量。
func (b *builder) prepareRecycleIntake(entry *machineRecycle, stub []*placed, x float64) error {
	if b.denseSpatial && b.adapter != nil && b.adapter.density == densityDense && entry.netInput <= 1e-8 {
		entry.intake = stub
		entry.fresh, entry.returned = stub[0], stub[0]
		entry.fresh.record.Parameters = []int32{int32(entry.item), 0}
		return nil
	}
	var err error
	entry.inputGate = b.recycleGate(x, stub[0].a.y, 3, entry.item)
	entry.freshPath, err = b.buildRecyclePort(entry.inputGate, 2, true)
	if err != nil {
		return err
	}
	entry.fresh = entry.freshPath[0]
	entry.fresh.record.Parameters = []int32{int32(entry.item), int32(math.Ceil(entry.netInput))}
	entry.returnedPath, err = b.buildRecyclePort(entry.inputGate, 3, true)
	if err != nil {
		return err
	}
	entry.returned = entry.returnedPath[0]
	entry.intake, err = b.buildRecyclePort(entry.inputGate, 0, false)
	if err != nil {
		return err
	}
	middle := b.straight(x+3, stub[0].a.y, int(math.Round(stub[0].a.x-x-3)), 2)
	entry.intake = append(entry.intake, middle...)
	entry.intake = append(entry.intake, stub...)
	for i := 0; i+1 < len(entry.intake); i++ {
		connect(entry.intake[i], entry.intake[i+1])
	}
	return nil
}

// validateRecycleIntake 职责：证明回料实际到达本物品机器带，存在新料时仍验证优先合流；使用：完整回流终检；边界：不能以空合流门字段绕过净补料与端口身份检查。
func validateRecycleIntake(entry *machineRecycle, indices map[*placed]int) error {
	fail := func(reason string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "机器回流入口无效：" + reason}
	}
	if len(entry.intake) == 0 || !hasPortItem(entry.fresh, entry.item) {
		return fail("缺少实际回机或预充带")
	}
	if entry.inputGate == nil {
		if entry.netInput > 1e-8 || entry.fresh != entry.returned || entry.returned != entry.intake[0] || entry.row.inputs[entry.item] != entry.returned || len(entry.freshPath) != 0 || len(entry.returnedPath) != 0 {
			return fail("直接回机只适用于零净补料的同一真实输入链")
		}
		for _, rate := range entry.row.info.Inputs {
			if rate.ItemID == entry.item && rate.RatePerMinute > 1e-8 {
				return fail("存在净输入，不能省去新料优先合流门")
			}
		}
	} else {
		if err := validateRecycleGate(entry.inputGate, 3, entry.item, entry.row.quarterTurns, indices); err != nil {
			return err
		}
		if !recycleInputAttached(entry.returnedPath, entry.returned, entry.inputGate, 3) || !recycleInputAttached(entry.freshPath, entry.fresh, entry.inputGate, 2) || entry.fresh == entry.returned {
			return fail("新料或回料优先槽已断开")
		}
		for _, path := range [][]*placed{entry.freshPath, entry.returnedPath} {
			if err := validateBusFabricChain(path, indices); err != nil {
				return err
			}
		}
		if entry.intake[0].input != entry.inputGate || entry.intake[0].record.InputFromSlot != 0 {
			return fail("合流门未接回机器")
		}
	}
	if _, ok := indices[entry.fresh]; !ok {
		return fail("预充入口不属于蓝图")
	}
	if err := validateBusFabricChain(entry.intake, indices); err != nil {
		return err
	}
	for i, item := range entry.row.recipe.Items {
		if item == entry.item && len(entry.row.line.inputs[i]) > 0 && entry.intake[len(entry.intake)-1].output == entry.row.line.inputs[i][0] {
			return nil
		}
	}
	return fail("回料链未接到对应的机器原料带")
}
