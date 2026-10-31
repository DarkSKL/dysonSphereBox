package factory

// 文件职责：构造蓝图导出审计事实；使用：factory 服务确认导出成功后；边界：不记录整串内容，不负责文件写入。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/helper/blueprint/build"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordExport 职责：记录用户确认的文本导出；使用：ExportBlueprint；边界：仅记录长度和结果，不包含蓝图和完整路径。
func RecordExport(length int) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{Namespace: "factory.blueprint.export", Source: "backend.factory", Handler: "User", Level: logging.Info, Action: "蓝图文本已导出", Reason: "用户通过桌面保存对话框选择文件，编码已严格校验", Data: map[string]interface{}{"length": length, "result": "success"}})
}

// RecordWorkspaceExport 职责：记录方案 JSON 已落盘；使用：服务写入成功后；边界：不记录具体方案内容和路径。
func RecordWorkspaceExport(targets int) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{Namespace: "factory.workspace.export", Source: "backend.factory", Handler: "User", Level: logging.Info, Action: "计算方案已导出", Reason: "重新求解与前端快照一致，用户已选择 JSON 文件", Data: map[string]interface{}{"targets": targets, "result": "success"}})
}

// RecordGeneration 职责：为已经通过物流及蓝图自检的生成记录有限审计元数据。
// 使用：服务确认任务仍为最新且全部校验成功后调用；边界：不记录蓝图串、完整方案、用户路径或真实游戏放置保证。
func RecordGeneration(req build.Request, result build.Result) {
	data := map[string]interface{}{"planId": req.Plan.PlanID, "mode": req.Mode, "beltLevel": req.Setting.BeltLv, "machines": req.Plan.MachineCount, "buildings": result.BuildingCount, "areas": result.Areas, "result": "success"}
	if req.Plan.FlowGraph != nil {
		data["flowGraphDigest"] = req.Plan.FlowGraph.Digest
	}
	systemHelper.RecordLogSilently(systemHelper.LogEntry{Namespace: "factory.blueprint.generate", Source: "backend.factory", Handler: "User", Level: logging.Info, Action: "蓝图生成完成", Reason: "已完成当前抽象规则覆盖内的物流和编码检查；不代表游戏内放置验收", Data: data})
}
