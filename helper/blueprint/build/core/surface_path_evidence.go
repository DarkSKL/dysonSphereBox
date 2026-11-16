package build

// 文件职责：为原生球面和多Area共用原始带边替换后的路径证据更新。
// 使用：实际补点后同步所有有向及反向保存的路径；边界：只插入桥接表声明的原相邻边，不按坐标猜连接或变更物料。

// expandSurfaceBeltPath 职责：同步有向或反向保存的原始带序；使用：机器带及设施路径；边界：仅扩展原本相邻的两个实体，不按坐标猜连接。
func expandSurfaceBeltPath(path []*placed, bridges map[*placed]connectedSurfaceBridge) []*placed {
	result := make([]*placed, 0, len(path))
	for i, p := range path {
		result = append(result, p)
		if i+1 == len(path) {
			continue
		}
		next := path[i+1]
		if bridge, ok := bridges[p]; ok && bridge.target == next {
			result = append(result, bridge.belts...)
		} else if bridge, ok := bridges[next]; ok && bridge.target == p {
			for j := len(bridge.belts) - 1; j >= 0; j-- {
				result = append(result, bridge.belts[j])
			}
		}
	}
	return result
}
