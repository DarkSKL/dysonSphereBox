package saveanalysis

// 文件职责：保留分拣器实载和投放点的直接观测。
// 使用：由厂区布局读取器恢复保存时刻的带口状态；边界：不沿下游追踪或给出产线建议。

// DropObservation 职责：保存持货分拣器与投放观测；使用：厂区解码；边界：不追查下游受阻原因。
type DropObservation struct {
	// Source 职责：保存取货实体；使用：关联分拣器连接；边界：仅当前厂区有效。
	Source int64 `json:"source"`
	// Sorter 职责：保存分拣器实体；使用：索引观测；边界：非组件池编号。
	Sorter int64 `json:"sorter"`
	// Target 职责：保存放货实体；使用：定位投放端；边界：不证明已送达。
	Target int64 `json:"target"`
	// Stage 职责：保存工作阶段；使用：判定普通投放条件；边界：不推断持续时间。
	Stage int64 `json:"stage"`
	// Belt 职责：记录目标带段编号；使用：关联带口；边界：未关联时为零。
	Belt int64 `json:"belt"`
	// ItemID 职责：保存实载物品；使用：解释货物；边界：不表示过滤配置。
	ItemID int64 `json:"itemId"`
	// Count 职责：保存实载数量；使用：解释货物；边界：不是历史流量。
	Count int64 `json:"count"`
	// Refusal 职责：记录直接可证的拒收条件；使用：普通正向投放观测；边界：空值不保证可投放。
	Refusal string `json:"refusal,omitempty"`
}

// dropRefusal 职责：识别普通投放窗口的拒收条件；使用：分拣器区段解码；边界：未知、压缩或堆叠情形返回空值。
func dropRefusal(buffer []byte, pos int64) string {
	if pos < 4 || pos+5 >= int64(len(buffer)) {
		return ""
	}
	end := pos + 5
	for end > pos && buffer[end] != 0 {
		end--
	}
	if end == pos {
		return "occupied-window"
	}
	center := end - 5
	if end+6 < int64(len(buffer)) {
		for j := end + 1; j <= end+5; j++ {
			if buffer[j] != 0 {
				center = j - 6
				break
			}
		}
	}
	if center < 4 {
		return "path-boundary"
	}
	return ""
}
