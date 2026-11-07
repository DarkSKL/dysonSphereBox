package factory

import "DysonSphereBox/helper/factory/progressjournal"

// ReadGenerationProgress 职责：分页补发本实例的真实候选过程；使用：前端序号缺口或完成事件缺失时恢复；边界：只读受控日志，不启动或改变计算。
func (s *Service) ReadGenerationProgress(taskID string, after uint64) progressjournal.Page {
	return s.progress.Read(taskID, after)
}
