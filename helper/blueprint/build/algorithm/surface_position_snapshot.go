package build

// 文件职责：在明确只读的规则检查阶段共享世界端点，避免重复球面映射和整份记录复制。
// 使用：动态规则入口建立快照，退出时恢复；边界：不得跨越锚点、区域或记录变更，不跨候选共享。

import "DysonSphereBox/helper/blueprint"

// surfacePositionSnapshot 职责：绑定一份当前只读记录的世界端点；使用：同阶段多个规则；边界：身份和长度必须同时相符，端点切片只能读取。
type surfacePositionSnapshot struct {
	firstRecord   *blueprint.Building
	count         int
	first, second []vector
}

// matches 职责：判断是否为本只读阶段的同一记录切片；使用：快照命中；边界：内容修改须先退出只读阶段。
func (snapshot *surfacePositionSnapshot) matches(records []blueprint.Building) bool {
	return snapshot != nil && len(records) > 0 && snapshot.count == len(records) && snapshot.firstRecord == &records[0]
}

// beginPositionSnapshot 职责：在只读校验区间内固定世界端点；使用：调用方defer恢复；边界：不作为可变记录的长期缓存，嵌套检查保留外层快照。
func (encoding *surfaceEncoding) beginPositionSnapshot(records []blueprint.Building) (func(), error) {
	if len(records) == 0 || encoding.positionSnapshot.matches(records) {
		return func() {}, nil
	}
	points, err := encoding.surfaceWorldPositionsFor(records, false)
	if err != nil {
		return nil, err
	}
	previous := encoding.positionSnapshot
	encoding.positionSnapshot = &surfacePositionSnapshot{firstRecord: &records[0], count: len(records), first: points}
	return func() { encoding.positionSnapshot = previous }, nil
}

// surfaceWorldPositions 职责：读取当前锚点的第一端点；使用：所有原有几何检查；边界：只读阶段以外每次重算。
func (encoding *surfaceEncoding) surfaceWorldPositions(records []blueprint.Building) ([]vector, error) {
	if encoding.positionSnapshot.matches(records) {
		return encoding.positionSnapshot.first, nil
	}
	return encoding.surfaceWorldPositionsFor(records, false)
}

// surfaceSecondPositions 职责：直接映射第二端点而不复制整份建筑结构；使用：分拣器；边界：保留原Area及float32运算顺序。
func (encoding *surfaceEncoding) surfaceSecondPositions(records []blueprint.Building) ([]vector, error) {
	snapshot := encoding.positionSnapshot
	if snapshot.matches(records) && snapshot.second != nil {
		return snapshot.second, nil
	}
	points, err := encoding.surfaceWorldPositionsFor(records, true)
	if err == nil && snapshot.matches(records) {
		snapshot.second = points
	}
	return points, err
}
