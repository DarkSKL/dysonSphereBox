package build

// 文件职责：把机器行沿中央塔组四边组织为闭合环绕布局。
// 使用：塔端口寻路前；边界：只刚性变换完整行，保留连接、槽位、数量和双端高度。

import "math"

// surroundStationRows 职责：按东南西北四边布置机器行并保留中央接驳庭院；使用：创建塔端口之前；边界：单行不补机器，双塔按整体中央区域预留空间。
func surroundStationRows(lines []*line, towerCount int) {
	centerX := float64(towerCount-1) * 15
	if len(lines) == 2 && lines[0].densePaired && lines[1].densePaired {
		// 两个双排单元已包含四排机器；相对置于塔两侧，避免再次沿四角展开产生空象限。
		for index, row := range lines {
			if index == 1 {
				rotatePlacedQuarter(row.all, 2)
			}
			low, high := bounds(row.all)
			x := centerX - math.Floor((low.x+high.x)/2)
			y := math.Ceil(4 - low.y)
			if index == 1 {
				y = math.Floor(-4 - high.y)
			}
			shift(row.all, x, y, 0)
		}
		return
	}
	radius := 7. + float64(towerCount-1)*15
	// 多物料端口需要独立穿出机器带与塔口预留区；按真实轨数扩大庭院，不修改机器间距。
	for _, row := range lines {
		tracks := len(row.inputs) + len(row.outputs)
		if row.coaterInput != nil {
			tracks++
		}
		if tracks > 2 {
			radius = math.Max(radius, float64(5+tracks*2)+float64(towerCount-1)*15)
		}
	}
	for i, row := range lines {
		low, _ := bounds(row.all)
		// 每行从庭院角点沿切线向前，四边呈风车形，不将端点藏在另一条机器行后方。
		// 分拣器端点可能超出整数边界；整块只平移整数格，保证地面外围端口仍可被格点寻路到达。
		shift(row.all, math.Ceil(-radius+1-low.x), math.Ceil(radius-low.y), 0)
		rotatePlacedQuarter(row.all, i%4)
		shift(row.all, centerX, 0, 0)
	}
}

// rotatePlacedQuarter 职责：以原点顺时针刚性旋转整块实体；使用：环绕设施布局；边界：不改槽位编号、引用与高度，双端姿态同步，重复指针只处理一次。
func rotatePlacedQuarter(buildings []*placed, turns int) {
	turns = (turns%4 + 4) % 4
	seen := make(map[*placed]bool, len(buildings))
	for _, building := range buildings {
		if seen[building] {
			continue
		}
		seen[building] = true
		for step := 0; step < turns; step++ {
			building.a.x, building.a.y = building.a.y, -building.a.x
			building.b.x, building.b.y = building.b.y, -building.b.x
			building.densityCargo.relative.x, building.densityCargo.relative.y = building.densityCargo.relative.y, -building.densityCargo.relative.x
		}
		building.record.Yaw = float32(math.Mod(float64(building.record.Yaw)+float64(turns)*90, 360))
		building.record.Yaw2 = float32(math.Mod(float64(building.record.Yaw2)+float64(turns)*90, 360))
	}
}
