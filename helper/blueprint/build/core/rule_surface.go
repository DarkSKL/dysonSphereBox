package build

// 文件职责：复制最终规则需要的坐标元数据，隔离已经结束的装配阶段；使用：每项规则建立独占视图；边界：不改变记录、区域、锚点和物理规则。

// copyRuleSurface 职责：只复制正式记录校验依赖的坐标状态；使用：newRuleEnvironment；边界：建筑到区域的装配指针表、旧重定位源记录和寻锚候选不进入终检视图，仍保留在完整几何快照中。
func copyRuleSurface(c *finalizationCloner, source *surfaceEncoding) *surfaceEncoding {
	if source == nil {
		return newPlanarRuleEncoding()
	}
	view := *source
	view.byBuilding = nil
	view.anchorCandidates = nil
	view.relocation = nil
	view.positionSnapshot = nil
	copy := c.buildSurfaceEncoding(view)
	return &copy
}
