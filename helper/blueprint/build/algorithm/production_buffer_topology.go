package build

// 文件职责：构造连续主带上的箱体取放及储液罐溢流旁路。
// 使用：缓存模块局部装配；边界：不通过库存串联主物流，不改变主带等级。

// joinBufferBelts 职责：用短曼哈顿带连接两个现存链；使用：行末主带和储液罐支路；边界：两端必须同层且互不重合。
func (b *builder) joinBufferBelts(first, second []*placed, firstX bool) []*placed {
	start, end := first[len(first)-1], second[0]
	points := manhattan(start.a, end.a, firstX)
	middle := b.byPath(points[1 : len(points)-1])
	path := append(append(append([]*placed{}, first...), middle...), second...)
	for i := len(first); i <= len(first)+len(middle); i++ {
		connect(path[i-1], path[i])
	}
	return path
}

// buildBoxSideBuffer 职责：在连续主带旁放置同侧取放箱臂；使用：固体缓存；边界：单臂只影响缓存速率，主带可直接绕过箱体。
func (b *builder) buildBoxSideBuffer(buffer *productionBuffer) error {
	buffer.store = b.make(2101, 0, 0, 0)
	buffer.store.record.Yaw, buffer.store.record.Yaw2 = 0, 0
	buffer.store.record.Parameters = make([]int32, 110)
	reach := 3
	if b.compactBufferLayout() {
		reach = 2
	}
	buffer.main = b.byPath(manhattan(vector{float64(reach), -2, 0}, vector{-float64(reach), -2, 0}, true))
	buffer.intake = buffer.main[:reach+1]
	buffer.outlet = buffer.main[reach+1:]
	for i, slot := range []int{6, 8} {
		arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
		belt := buffer.main[reach-1]
		if i == 1 {
			belt = buffer.main[reach+1]
		}
		if err := flyArmGeometry(arm, buffer.store, belt, slot, i == 1); err != nil {
			return err
		}
		arm.record.FilterID = int16(buffer.item)
		if i == 0 {
			buffer.inArms = []*placed{arm}
		} else {
			buffer.outArms = []*placed{arm}
		}
	}
	return nil
}

// buildTankSideBuffer 职责：以优先直行分流门和主输入优先合流门连接储液罐；使用：流体缓存；边界：主带畅通时不强制入罐，满罐或关闭支路不能卡住主带。
func (b *builder) buildTankSideBuffer(buffer *productionBuffer) error {
	first := b.recycleGate(0, 0, 2, buffer.item)
	gap, height := 8., 5.
	if b.compactBufferLayout() {
		gap, height = 6, 4
	}
	second := b.recycleGate(-gap, 0, 0, buffer.item)
	buffer.gates = []*placed{first, second}
	input, err := b.buildRecyclePort(first, 0, true)
	if err != nil {
		return err
	}
	straight, err := b.buildRecyclePort(first, 2, false)
	if err != nil {
		return err
	}
	overflow, err := b.buildRecyclePort(first, 3, false)
	if err != nil {
		return err
	}
	merge, err := b.buildRecyclePort(second, 0, true)
	if err != nil {
		return err
	}
	output, err := b.buildRecyclePort(second, 2, false)
	if err != nil {
		return err
	}
	returned, err := b.buildRecyclePort(second, 3, true)
	if err != nil {
		return err
	}
	between := b.joinBufferBelts(straight, merge, true)
	buffer.intake, buffer.outlet = input, output
	buffer.main = append(append(append(append(append([]*placed{}, input...), first), between...), second), output...)
	buffer.store = b.make(2106, -gap/2, height, 0)
	buffer.store.record.Yaw, buffer.store.record.Yaw2 = 0, 0
	buffer.store.record.Parameters = []int32{1, 1}
	tankIn := b.byPath([]vector{{-gap/2 + 2.5, height, 0}, {-gap/2 + 1.7, height, 0}})
	tankIn[len(tankIn)-1].output = buffer.store
	tankIn[len(tankIn)-1].record.OutputToSlot = 1
	buffer.storeIntake = b.joinBufferBelts(overflow, tankIn, false)
	tankOut := b.byPath([]vector{{-gap/2 - 1.7, height, 0}, {-gap/2 - 2.5, height, 0}})
	tankOut[0].input = buffer.store
	tankOut[0].record.InputFromSlot = 3
	buffer.storeOutlet = b.joinBufferBelts(tankOut, returned, true)
	return nil
}
