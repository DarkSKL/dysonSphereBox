package build

// 文件职责：装配物流配送器和四向分流模式；使用：mode 2/5；边界：保留箱体、配送器和端口关系，不声称物流逐帧模拟。
// 行为核对 dsp-calc FlyBlock.ts；依据其算法独立实现，见 ../NOTICE.md。

import (
	"math"
)

// manhattan 职责：生成轴优先整数路径；使用：配送器局部带；边界：不同高度不自动连接。
func manhattan(a, b vector, firstX bool) []vector {
	out := []vector{a}
	p := a
	step := func(xAxis bool) {
		for {
			delta := b.y - p.y
			if xAxis {
				delta = b.x - p.x
			}
			if math.Abs(delta) < 1e-8 {
				break
			}
			if delta > 0 {
				delta = math.Min(1, delta)
			} else {
				delta = math.Max(-1, delta)
			}
			if xAxis {
				p.x += delta
			} else {
				p.y += delta
			}
			out = append(out, p)
		}
	}
	step(firstX)
	step(!firstX)
	return out
}

// flyBlocks 职责：在行末连接逐物品配送块；使用：mode 2/5；边界：原料按下方逆序和上方顺序布置。
func (b *builder) flyBlocks(l *line, mode int) error {
	bottom := len(l.inputs)
	if bottom > 3 {
		bottom = 3
	}
	beginX := l.inputs[0][len(l.inputs[0])-1].a.x + 2
	beginY := l.inputs[0][len(l.inputs[0])-1].a.y + float64(bottom) + 1
	order := []int{}
	for i := bottom - 1; i >= 0; i-- {
		order = append(order, i)
	}
	for i := 3; i < len(l.inputs); i++ {
		order = append(order, i)
	}
	for _, row := range l.inputs {
		reverse(row)
	}
	for depth, i := range order {
		row := l.inputs[i]
		if err := b.fly(l.recipe.Items[i], row[len(row)-1], true, beginX+3*float64(depth), beginY, mode); err != nil {
			return err
		}
	}
	if err := b.fly(l.info.MainItemID, l.output[len(l.output)-1], false, beginX+1+3*float64(len(order)), beginY, mode); err != nil {
		return err
	}
	if *b.setting.NeedPower {
		b.make(2201, beginX-1+3*float64(len(order)), beginY, 0)
	}
	return nil
}

// fly 职责：创建一个物品的箱体/配送器/桥接带；使用：行末连接；边界：生成后容量和运力由上层提示。
func (b *builder) fly(item int, link *placed, input bool, x, y float64, mode int) error {
	var splitter *placed
	if mode == 5 {
		splitter = b.make(2020, x, y, 0)
		splitter.record.ModelIndex = 38
		splitter.record.Parameters = make([]int32, 6)
		splitter.record.OutputToSlot = 14
		splitter.record.InputFromSlot = 15
		splitter.record.OutputFromSlot = 15
		splitter.record.InputToSlot = 14
	}
	z := 0.
	if mode == 5 {
		z = 2
	}
	box := b.make(2101, x, y, z)
	box.record.OutputToSlot = 14
	box.record.InputFromSlot = 15
	box.record.OutputFromSlot = 15
	box.record.InputToSlot = 14
	box.record.Parameters = make([]int32, 110)
	box.record.Parameters[0] = 29
	for _, fluid := range []int{1000, 1007, 1114, 1116, 1120, 1121} {
		if fluid == item {
			box.record.Parameters[0] = 0
		}
	}
	delivery := b.make(2107, x, y, z+1.875)
	delivery.record.Yaw = 0
	delivery.record.Yaw2 = 0
	delivery.input = box
	delivery.record.InputFromSlot = 13
	delivery.record.InputToSlot = 0
	delivery.record.FilterID = int16(item)
	delivery.record.Parameters = make([]int32, 128)
	delivery.record.Parameters[0] = 2
	delivery.record.Parameters[1] = 1
	if input {
		delivery.record.Parameters[1] = 2
	}
	delivery.record.Parameters[2] = 30000
	delivery.record.Parameters[3] = 1
	if mode == 5 {
		box.input = splitter
		box.record.InputFromSlot = 15
		path := manhattan(link.a, splitter.a, true)
		belts := b.byPath(path[1:])
		if input {
			reverse(belts)
			last := belts[len(belts)-1]
			last.input = splitter
			last.record.InputFromSlot = 0
			connect(belts[0], link)
		} else {
			connect(link, belts[0])
			last := belts[len(belts)-1]
			last.output = splitter
			last.record.OutputToSlot = 0
		}
		return nil
	}
	points := []vector{}
	px, py := link.a.x+1, link.a.y
	for px <= x {
		points = append(points, vector{px, py, 0})
		if px == x-1 {
			deep := int(math.Abs(y-py)) - 2
			for j := 0; j < deep; j++ {
				if y > py {
					py++
				} else {
					py--
				}
				points = append(points, vector{px, py, 0})
			}
		}
		px++
	}
	belts := b.byPath(points)
	if input {
		reverse(belts)
		connect(belts[0], link)
	} else {
		connect(link, belts[0])
	}
	end := belts[len(belts)-1]
	arm := b.make(2010+b.setting.InserterLv, x, 0, 0)
	arm.record.InputFromSlot = -1
	arm.record.Parameters = []int32{1}
	isBottom := y > end.a.y
	if input {
		arm.output = end
		arm.input = box
		arm.record.OutputToSlot = -1
		arm.record.InputFromSlot = 10
		arm.record.Yaw = 0
		if isBottom {
			arm.record.InputFromSlot = 4
			arm.record.Yaw = 180
		}
		arm.record.Yaw2 = arm.record.Yaw
		arm.a = vector{x, y + 0.8, 0}
		if isBottom {
			arm.a.y = y - 0.8
		}
		arm.b = end.a
	} else {
		arm.output = box
		arm.input = end
		arm.record.FilterID = int16(item)
		arm.record.OutputToSlot = 10
		arm.record.Yaw = 180
		if isBottom {
			arm.record.OutputToSlot = 4
			arm.record.Yaw = 0
		}
		arm.record.Yaw2 = arm.record.Yaw
		arm.a = end.a
		arm.b = vector{x, y + 0.8, 0}
		if isBottom {
			arm.b.y = y - 0.8
		}
	}
	return nil
}
