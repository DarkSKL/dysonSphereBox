package build

// 文件职责：在密铺窄相位独立执行分流器完整选择包络与真实动态臂盒的保守净空。
// 使用：完整单元四向排布、折行接角与平移压缩；边界：不改原生建造盒及其同批次互免，不给选择体积套用臂/静态互免。

// splitterSelectionPackingConflict 职责：让完整选择包络参与实体盒和实际带段净空；使用：分类索引及共同排布窄相位；边界：power/节点间距探针不是实体，物理电塔另有原生静态盒。
func splitterSelectionPackingConflict(a, c *connectedPackingShape) bool {
	selection, other := a, c
	if selection.kind != "splitter-selection" {
		selection, other = c, a
	}
	switch other.kind {
	case "power", "dense-node", "dense-monitor", "dense-monitor-arm":
		return false
	case "belt":
		return selection.box.SegmentDistanceSquared(other.span.ends[0], other.span.ends[1]) < surfaceBeltNodeRadius*surfaceBeltNodeRadius-1e-6
	case "static", "splitter-selection", "inserter", "port":
		return selection.box.Overlaps(other.box)
	default:
		return true
	}
}
