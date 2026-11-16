package factory

// 文件职责：公开工作流后台只读接口；使用：未来工作流可视化及诊断调用已绑定工厂服务；边界：不新增页面，不启动生成，不暴露实体或内部能力。

import (
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/factory/workflowjournal"
)

// DescribeWorkflow 职责：查询当前配置的冻结生产方案图；使用：后台工作流定义查询；边界：配置错误明确返回，已运行任务图请调用 GetWorkflowTask。
func (s *Service) DescribeWorkflow() workflowjournal.Description {
	graph, err := build.DescribeWorkflow()
	return workflowjournal.Describe(graph, err)
}

// GetWorkflowTask 职责：查询指定任务状态及其实际绑定图；使用：执行中或完成后恢复；边界：未知、过期与关闭明确区分，不返回当前配置替代图。
func (s *Service) GetWorkflowTask(taskID string) workflowjournal.Snapshot {
	return s.workflow.Snapshot(taskID)
}

// ReadWorkflowEvents 职责：按独立日志游标查询真实增量事件；使用：执行中轮询及完成后恢复；边界：分页大小一至一百二十八，超前游标拒绝，旧游标缺口显式返回。
func (s *Service) ReadWorkflowEvents(taskID string, after uint64, limit int) workflowjournal.Page {
	return s.workflow.Read(taskID, after, limit)
}
