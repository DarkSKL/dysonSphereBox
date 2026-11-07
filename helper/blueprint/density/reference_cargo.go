package density

// 文件职责：把参考单元的双向外部引用转换为实际带边界，防止裁切片段的缺失前驱被当作自然带头。
// 使用：创建参考 CargoSampler 时填入 CargoOptions.Boundaries；边界：仅保留带到带引用，不改变原始记录。

import "fmt"

// ReferenceBoundary 职责：保存研究文件中的一条跨裁切引用；使用：辨认缺失带邻居；边界：direction字段不是物料流方向，以Input/Output字段决定真实带向。
type ReferenceBoundary struct {
	// RecordIndex 职责：保存引用发起记录；使用：定位内外侧；边界：可以位于当前记录集合之外。
	RecordIndex int32 `json:"recordIndex"`
	// Field 职责：保存引用字段名；使用：区分引用方向和实际带方向；边界：仅接受inputObjIdx/outputObjIdx。
	Field string `json:"field"`
	// TargetIndex 职责：保存被引用的建筑编号；使用：形成实际邻接；边界：不是切片下标。
	TargetIndex int32 `json:"targetIndex"`
	// ExternalItemID 职责：保存裁切外侧的原型；使用：过滤非带关系；边界：不能仅据编号猜测类型。
	ExternalItemID int16 `json:"externalItemId"`
}

// CargoBoundaries 职责：将跨界引用转换为去重的有向带边；使用：还原原图货物采样；边界：节点两端必须恰有一个在当前单元内，非带引用不参与CargoPath。
func (unit ReferenceUnit) CargoBoundaries() ([]CargoBoundary, error) {
	items := map[int32]int16{}
	for _, record := range unit.Records {
		items[record.Index] = record.ItemID
	}
	isBelt := func(item int16) bool { return item >= 2001 && item <= 2003 }
	result := []CargoBoundary{}
	seen := map[[2]int32]bool{}
	for _, reference := range unit.BoundaryReferences {
		fromItem, fromInside := items[reference.RecordIndex]
		toItem, toInside := items[reference.TargetIndex]
		if fromInside == toInside {
			return nil, fmt.Errorf("%w：参考引用%d→%d的内外边界不一致", ErrCargoGeometry, reference.RecordIndex, reference.TargetIndex)
		}
		if !fromInside {
			fromItem = reference.ExternalItemID
		}
		if !toInside {
			toItem = reference.ExternalItemID
		}
		if !isBelt(fromItem) || !isBelt(toItem) {
			continue
		}
		from, to := reference.RecordIndex, reference.TargetIndex
		switch reference.Field {
		case "inputObjIdx":
			from, to = to, from
		case "outputObjIdx":
		default:
			return nil, fmt.Errorf("%w：未知参考带字段%s", ErrCargoGeometry, reference.Field)
		}
		key := [2]int32{from, to}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, CargoBoundary{FromIndex: from, ToIndex: to})
	}
	return result, nil
}
