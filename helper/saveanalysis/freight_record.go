package saveanalysis

// 文件职责：保存物流站带口、活动订单与运输实载记录。
// 使用：由物流区段读取器填充；边界：不推断供需关系或追查上游来源。

// stationBelt 职责：保存物流站带口方向及槽位；使用：站点解码；边界：槽位从零开始。
type stationBelt struct {
	dir, belt int64
	slot      int
}

// freightOrder 职责：保存活动物流订单字段；使用：物流运输解码；边界：预约数量不等于实载。
type freightOrder struct {
	other                           int64
	thisIndex, otherIndex           int
	item, thisOrdered, otherOrdered int64
}

// freightRecord 职责：保存无人机或运输船的活动记录；使用：站点运输区段解码；边界：不归因来源或供需。
type freightRecord struct {
	ship                    bool
	planetA, planetB, other int64 // other 属于无人机本地站号或运输船全局站号。
	direction               float32
	item, count             int64
	order                   freightOrder
}
