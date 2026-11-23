package saveanalysis

// PlanetEnergyEnvironment 职责：返回当前版本主题确认的风力和熔岩条件；使用：能源选址；边界：不推测日照连续性。
func (s *Snapshot) PlanetEnergyEnvironment(id int64) (wind float64, lava bool) {
	theme := s.themes[id]
	// 数据源：构建 25955035 的 ThemeProtoSet，按主题 ID 排列。
	winds := [...]float64{0, 1, 0, 0, 0, 0, 1.5, .4, 1, .7, .7, 0, .8, .8, 1, 1.1, 1.1, 1.1, 1, 1.6, .7, 0, 1.1, 1.5, 1.3, 1}
	if theme.ID > 0 && theme.ID < len(winds) {
		wind = winds[theme.ID]
	}
	return wind, theme.ID > 0 && theme.WaterItemID == -1
}

// PlanetHasDrySurface 根据已识别的主题核对整星是否没有液面，供制造选址排序；具体矿点、地形和障碍仍由落位核验。
func (s *Snapshot) PlanetHasDrySurface(id int64) bool {
	theme, ok := s.themes[id]
	return ok && theme.ID > 0 && theme.WaterItemID == 0
}
