package build

// 文件职责：让密铺宽相位跳过规则明确互略的对象类别，保留相同的窄相位结果。
// 使用：候选私有空间索引；边界：不是新的碰撞规则，不跨请求共享查询状态。

// denseShapeKind 职责：归一内部类别键；使用：索引未知类别；边界：未知形状仍参与所有保守查询。
func denseShapeKind(kind string) string {
	switch kind {
	case "static", "splitter-selection", "inserter", "belt", "port", "power", "dense-node", "dense-monitor", "dense-monitor-arm":
		return kind
	default:
		return "unknown"
	}
}

// indexDenseKinds 职责：为新增形状追加分类桶；使用：形状库存只追加时的延迟索引；边界：不改实体顺序或已有空间桶。
func (packing *connectedPacking) indexDenseKinds() {
	if packing.kindCells == nil {
		packing.kindCells = map[string]map[[3]int][]int{}
	}
	for packing.kindCount < len(packing.shapes) {
		index := packing.kindCount
		shape := packing.shapes[index]
		kind := denseShapeKind(shape.kind)
		if packing.kindCells[kind] == nil {
			packing.kindCells[kind] = map[[3]int][]int{}
		}
		bucket := packing.kindCells[kind]
		connectedShapeCells(shape, func(cell [3]int) bool { bucket[cell] = append(bucket[cell], index); return true })
		packing.kindCount++
	}
}

// denseQueryBuckets 职责：按既有denseSpatialConflict分派可能冲突类别；使用：缩小宽相位枚举；边界：未知查询回退完整索引。
func (packing *connectedPacking) denseQueryBuckets(kind string) (buckets [6]map[[3]int][]int, count int) {
	var kinds []string
	switch kind {
	case "dense-monitor":
		kinds = []string{"dense-monitor-arm"}
	case "dense-monitor-arm":
		kinds = []string{"dense-monitor"}
	case "dense-node":
		kinds = []string{"dense-node"}
	case "power":
		kinds = []string{"power", "unknown"}
	case "static":
		kinds = []string{"static", "splitter-selection", "belt", "port", "unknown"}
	case "splitter-selection":
		kinds = []string{"static", "splitter-selection", "inserter", "belt", "port", "unknown"}
	case "inserter":
		kinds = []string{"inserter", "splitter-selection", "port", "unknown"}
	case "belt":
		kinds = []string{"static", "splitter-selection", "port", "unknown"}
	case "port":
		kinds = []string{"static", "splitter-selection", "inserter", "belt", "port", "unknown"}
	default:
		buckets[0] = packing.cells
		return buckets, 1
	}
	for _, key := range kinds {
		if bucket := packing.kindCells[key]; len(bucket) > 0 {
			buckets[count] = bucket
			count++
		}
	}
	return
}
