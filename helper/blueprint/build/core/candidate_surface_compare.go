package build

// 文件职责：以已验证真实几何比较严格带与跨带结果。
// 使用：引擎只读比较器；边界：不运行候选、不增加设施、不用改变产量换取更好的外观指标。

import "fmt"

// UsesAdjacentSurface 职责：返回该完整结果实际使用的选区许可；使用：相邻带归约；边界：零值结果拒绝，不能把未验证结果当严格基线。
func UsesAdjacentSurface(snapshot VerifiedSnapshot) (bool, error) {
	if snapshot.geometry.data == nil || snapshot.validation.data == nil {
		return false, fmt.Errorf("surface comparison requires verified snapshot")
	}
	layout := snapshot.geometry.data.request.SurfaceLayout
	return layout != nil && layout.ExpandAdjacent, nil
}

// BetterAdjacentCandidate 职责：按共同形状规则比较跨带与严格带结果；使用：完整终检后的选区归约；边界：真实设施数计入同档排序，拒绝缺失或错配验证的快照，不写入诊断或实体。
func BetterAdjacentCandidate(candidate, base VerifiedSnapshot) (bool, error) {
	if candidate.geometry.data == nil || base.geometry.data == nil || candidate.validation.data == nil || base.validation.data == nil ||
		candidate.validation.data.geometry != candidate.geometry.data || base.validation.data.geometry != base.geometry.data ||
		candidate.validation.data.logistics == nil || base.validation.data.logistics == nil {
		return false, fmt.Errorf("surface comparison requires verified snapshots")
	}
	facilities := func(snapshot VerifiedSnapshot) int {
		count := 0
		for _, record := range snapshot.geometry.data.document.Buildings {
			if surfaceIsStation(record.ItemID) || record.ItemID == 2107 {
				count++
			}
		}
		return count
	}
	return BetterLayoutCandidate(candidate.Metrics(), facilities(candidate), base.Metrics(), facilities(base)), nil
}
