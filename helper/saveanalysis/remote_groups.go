package saveanalysis

// FreeRemoteGroup 职责：选择本档没有任何塔占用的一位；使用：新建星际塔规划；边界：只用可由原版蓝图还原并匹配的单个位，无可用位或无有效快照返回零。
func (s *Snapshot) FreeRemoteGroup() uint64 {
	if s == nil || s.digest == "" {
		return 0
	}
	var occupied uint64
	for _, station := range s.stations {
		occupied |= station.remoteGroupMask
	}
	// 原版建筑参数在两个独立的非负 Int32 槽恢复掩码，符号位不能单独承载可配对的组。
	const representable = uint64(0x7FFFFFFF7FFFFFFF)
	free := representable &^ occupied
	return free & -free
}

// RemoteAstroRules 职责：报告物品在两颗星球之间可能匹配的原版天体航线与禁运；使用：已配置航线的干扰诊断；边界：匹配星球对和恒星对，原版键无方向，有禁运时实际先拒绝。
func (s *Snapshot) RemoteAstroRules(fromPlanetID, toPlanetID, itemID int64) (routed, banned bool) {
	if s == nil || fromPlanetID <= 0 || toPlanetID <= 0 || itemID <= 0 || fromPlanetID == toPlanetID {
		return false, false
	}
	key := astroRouteKey(fromPlanetID, toPlanetID, itemID)
	stars := astroRouteKey(fromPlanetID/100*100, toPlanetID/100*100, itemID)
	return s.astroRoutes[key] || s.astroRoutes[stars], s.astroBans[key] || s.astroBans[stars]
}

// astroRouteKey 职责：按原版天体行程编码折叠方向；使用：查询只读规则；边界：参数是原版天体和物品编号。
func astroRouteKey(from, to, item int64) uint64 {
	return uint64(min(from, to)) | uint64(max(from, to))<<22 | uint64(item)<<44
}
