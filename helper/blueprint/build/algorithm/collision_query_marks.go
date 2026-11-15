package build

// 文件职责：复用空间查询的按索引去重标记，消除热点中的临时哈希表。
// 使用：供电与运输各自私有的串行宽相位查询；边界：不缓存碰撞结果，不跨候选或并发查询共享。

// collisionQueryMarks 职责：按连续形状编号记录本次查询已访问项；使用：一次开始后顺序调用seen；边界：外部形状只能追加，回绕时清空旧标记，嵌套查询须使用不同实例。
type collisionQueryMarks struct {
	// marks 职责：保存每项最后访问序号；使用：常数时间去重；边界：长度随所属索引增长。
	marks []uint32
	// serial 职责：标识当前查询；使用：免清空地切换；边界：零不表示有效查询。
	serial uint32
}

// begin 职责：开启容量足够的新查询；使用：每轮宽相位枚举之前；边界：回绕清空全部标记，任何新增形状都会得到未访问状态。
func (query *collisionQueryMarks) begin(size int) {
	if len(query.marks) < size {
		query.marks = append(query.marks, make([]uint32, size-len(query.marks))...)
	}
	query.serial++
	if query.serial == 0 {
		clear(query.marks)
		query.serial = 1
	}
}

// seen 职责：登记编号并返回是否已在本轮访问；使用：空间桶返回同一形状时去重；边界：编号必须来自begin声明的索引范围，不调整枚举顺序。
func (query *collisionQueryMarks) seen(number int) bool {
	if query.marks[number] == query.serial {
		return true
	}
	query.marks[number] = query.serial
	return false
}
