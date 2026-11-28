package build

// 文件职责：保护已规划接口通道的连续几何，而不只是离散端点。
// 使用：现有原料出口预留与所有路由搜索共用；边界：只拒绝外来路线穿过未完成接口，当前端点拥有者继续执行原物理检查。

// transportReservedSpan 职责：保存一个固定短通道及其真实所有者；使用：候选环境私有宽相位；边界：不创建实际传送带。
type transportReservedSpan struct {
	owner *placed
	shape packingShape
}

// reserveTransportSpan 职责：把预定短通道登记到共同空间桶；使用：原料头外两步出口；边界：登记后不随路线试探变化。
func (env *transportEnvironment) reserveTransportSpan(first, second vector, owner *placed) {
	if env.densePorts == nil {
		return
	}
	shape := beltPackingShape(env.densePorts.world(first), env.densePorts.world(second))
	if env.reservedSpanCells == nil {
		env.reservedSpanCells = map[[3]int][]int{}
	}
	number := len(env.reservedSpans)
	env.reservedSpans = append(env.reservedSpans, transportReservedSpan{owner, shape})
	packingShapeCells(shape, func(cell [3]int) bool {
		env.reservedSpanCells[cell] = append(env.reservedSpanCells[cell], number)
		return true
	})
}

// reservedSpanClear 职责：核对整段不会横穿其他接口的连续出口；使用：分数portal、整数A*、提交复核。
// 边界：仍使用原带段半径及端点身份；未启用预留或没有密铺索引时直接沿用原路径。
func (env *transportEnvironment) reservedSpanClear(first, second vector, source, target *placed) bool {
	if len(env.reservedSpans) == 0 || env.densePorts == nil {
		return true
	}
	if slot, shift := env.densePorts.integerEdges.slot(first, second); slot >= 0 && uint64(len(env.reservedSpans)) < uint64(manyReservedOwners) {
		return env.integerReservedSpanClear(first, second, uint32(slot), integerEdgeDirection(first, second, shift), source, target)
	}
	return env.visitReservedIntersections(first, second, func(number int) bool {
		owner := env.reservedSpans[number].owner
		return owner == source || owner == target
	})
}

// visitReservedIntersections 在真实球面或平面上执行原有带段距离核验；visit 返回 false 时提前结束。
func (env *transportEnvironment) visitReservedIntersections(first, second vector, visit func(int) bool) bool {
	shape := beltPackingShape(env.densePorts.world(first), env.densePorts.world(second))
	env.reservedSpanQuery.begin(len(env.reservedSpans))
	return packingShapeCells(shape, func(cell [3]int) bool {
		for _, number := range env.reservedSpanCells[cell] {
			if env.reservedSpanQuery.seen(number) {
				continue
			}
			other := env.reservedSpans[number].shape
			if shape.high[0] < other.low[0] || other.high[0] < shape.low[0] || shape.high[1] < other.low[1] || other.high[1] < shape.low[1] || shape.high[2] < other.low[2] || other.high[2] < shape.low[2] {
				continue
			}
			if surfaceSpanDistanceSquared(shape.span, other.span) < .46*.46-1e-6 && !visit(number) {
				return false
			}
		}
		return true
	})
}
