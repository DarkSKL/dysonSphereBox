package build

// 文件职责：维护普通与密铺、直连与设施布局共同的轮廓几何及精确冲突规则。
// 使用：模式适配器提交世界坐标形状；边界：不决定模式、候选或最终规则调度，窄相位阈值保持原物理含义。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
)

// packingShape 职责：保存候选机器行的真实静态盒、修正分拣器盒、带段和端口通道；使用：轮廓靠拢宽窄相位；边界：不裁小游戏碰撞体，不把包围盒重叠直接当作实体冲突。
type packingShape struct {
	// gridPort 职责：把连续位姿下的路由出口柱对齐到真正整数接驳点；使用：密铺微调；边界：不移动实际端口或偷改机器坐标。
	gridPort bool
	// kind 职责：区分静态、分拣器、带、供电间距和端口净空；使用：应用不同碰撞规则；边界：未知类别不被当作安全。
	kind string
	// box 职责：保存静态/动态盒或端口预留柱；使用：窄相位；边界：带段使用 span。
	box placement.Box
	// span 职责：保存真实有向带段两端；使用：带交叉和建筑占用；边界：不会在候选阶段合并不同物品路线。
	span surfaceBeltSpan
	// item 职责：保存电力节点原型；使用：调用版本化节点间距规则；边界：其它类别只作诊断。
	item int16
	// low/high 职责：缓存米制包围盒；使用：空间哈希；边界：只用作宽相位。
	low, high placement.Vector
}

// packingContour 职责：保存一个尚未全局接线的局部产线轮廓；使用：每行增量装箱；边界：所有建筑指针由原 builder 持有，不复制机器。
type packingContour struct {
	shapes []packingShape
}

// boxPackingShape 职责：缓存有效定向盒的宽相位范围；使用：轮廓建模；边界：不修改半尺寸或旋转。
func boxPackingShape(kind string, box placement.Box) packingShape {
	low, high := box.Bounds()
	return packingShape{kind: kind, box: box, low: low, high: high}
}

// beltPackingShape 职责：用最终带半径保存实际线段；使用：带与带/建筑的跨块避碰；边界：末端可退化成球，不以无连接接触作为合法汇流。
func beltPackingShape(first, second placement.Vector) packingShape {
	low, high := first, first
	for i := range low {
		low[i] = math.Min(first[i], second[i]) - .23
		high[i] = math.Max(first[i], second[i]) + .23
	}
	return packingShape{kind: "belt", span: surfaceBeltSpan{ends: [2]placement.Vector{first, second}}, low: low, high: high}
}

// shiftPackingShape 职责：产生候选平移后的几何视图；使用：试探位置而不移动真实建筑；边界：只做刚体平移，不改变连接和朝向。
func shiftPackingShape(shape packingShape, dx, dy float64) packingShape {
	delta := placement.Vector{dx * planarGridLength, 0, dy * planarGridLength}
	shape.box.Center = shape.box.Center.Add(delta)
	shape.span.ends[0] = shape.span.ends[0].Add(delta)
	shape.span.ends[1] = shape.span.ends[1].Add(delta)
	shape.low, shape.high = shape.low.Add(delta), shape.high.Add(delta)
	if shape.gridPort {
		shape.box.Center[0] = math.Round(shape.box.Center[0]/planarGridLength) * planarGridLength
		shape.box.Center[2] = math.Round(shape.box.Center[2]/planarGridLength) * planarGridLength
		shape.low, shape.high = shape.box.Bounds()
	}
	return shape
}

// packingShapesConflict 职责：按静态/动态/带/电力规则区分轮廓冲突；使用：空间哈希窄相位；边界：最终检查的连接豁免只用于块内，跨块不允许借用别的物品端点。
func packingShapesConflict(a, c *packingShape) bool {
	if a.kind == "splitter-selection" || c.kind == "splitter-selection" {
		return splitterSelectionPackingConflict(a, c)
	}
	known := func(kind string) bool {
		return kind == "static" || kind == "inserter" || kind == "belt" || kind == "power" || kind == "port"
	}
	if !known(a.kind) || !known(c.kind) {
		return true
	}
	if a.kind == "power" || c.kind == "power" {
		if a.kind != "power" || c.kind != "power" {
			return false
		}
		delta := a.span.ends[0].Sub(c.span.ends[0])
		return delta.Dot(delta) < float64(surfacePowerDistanceLimit(a.item, c.item))
	}
	for i := range a.low {
		if a.high[i] < c.low[i] || c.high[i] < a.low[i] {
			return false
		}
	}
	if a.kind == "inserter" || c.kind == "inserter" {
		if a.kind == "inserter" && c.kind == "inserter" {
			return a.box.Overlaps(c.box)
		}
		if a.kind == "port" || c.kind == "port" {
			return a.box.Overlaps(c.box)
		}
		return false
	}
	if a.kind == "belt" && c.kind == "belt" {
		return surfaceSpanDistanceSquared(a.span, c.span) < .46*.46-1e-6
	}
	if a.kind == "belt" {
		return c.box.SegmentDistanceSquared(a.span.ends[0], a.span.ends[1]) < .23*.23-1e-6
	}
	if c.kind == "belt" {
		return a.box.SegmentDistanceSquared(c.span.ends[0], c.span.ends[1]) < .23*.23-1e-6
	}
	return a.box.Overlaps(c.box)
}

// packingShapeCells 职责：遍历米制四米哈希桶；使用：候选轮廓宽相位；边界：所有登记与查询共用此尺度，不改变实际碰撞阈值。
func packingShapeCells(shape packingShape, visit func([3]int) bool) bool {
	return worldSpatialBounds(shape.low, shape.high, 4).each(visit)
}
