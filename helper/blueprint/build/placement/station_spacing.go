package placement

// 文件职责：复算固态星球上原版物流设施的独立建造间距。使用：选址及整图粘贴终检。
// 边界：模型碰撞另查，轨道采集器不参加固态星球施工。

// HasStationSpacing 标记参与原版物流站距离检查的陆上设备，普通矿机只检查模型碰撞。
func HasStationSpacing(item int16) bool {
	return item == 2103 || item == 2104 || item == 2316
}

// StationSpacingSquared 返回候选与另一座陆上物流设施的最小距离平方。
// 使用：两座同时粘贴时原版从两端各查一次；面对存档实体或已有预设时只从新建端判断。
// 边界：两台大型采矿机之间没有专用站距，恰好等于阈值允许。
func StationSpacingSquared(candidate, other int16, bothPlanned bool) float32 {
	switch {
	case !HasStationSpacing(candidate) || !HasStationSpacing(other), candidate == 2316 && other == 2316:
		return 0
	case candidate == 2104 || other == 2104:
		return 841
	case candidate == 2316 || bothPlanned && other == 2316:
		return 625
	default:
		return 225
	}
}
