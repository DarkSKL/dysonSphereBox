package factory

// 文件职责：复核并序列化离线方案记录；使用：factory 导出方案；边界：不写数据库、不访问资源文件，不信任前端拼装的计算结果。

import (
	"DysonSphereBox/helper/blueprint"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	"context"
	"encoding/json"
	"reflect"
)

// MarshalWorkspace 职责：重新求解后保存版本化 JSON；使用：导出前检查；边界：输入改变或结果篡改立即拒绝，最多 1 MiB 配置记录。
func MarshalWorkspace(req contract.WorkspaceRequest) (string, error) {
	if req.Mode < 1 || req.Mode > 6 {
		return "", &blueprint.Error{Code: "GEN_E_MODE", Message: "方案策略必须为 1 到 6"}
	}
	plan, err := quant.Solve(context.Background(), req.Request)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(plan, req.Plan) {
		return "", &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "方案与当前输入不一致，请重新计算后导出"}
	}
	data, err := json.MarshalIndent(struct {
		// FormatVersion 职责：固定离线记录版本；使用：后续兼容识别；边界：不改变游戏蓝图格式。
		FormatVersion int `json:"formatVersion"`
		// WorkspaceRequest 职责：携带经复核的方案；使用：离线审阅；边界：不包含蓝图串和外部路径。
		contract.WorkspaceRequest
	}{FormatVersion: 1, WorkspaceRequest: req}, "", "  ")
	if err != nil {
		return "", err
	}
	if len(data) > 1<<20 {
		return "", &blueprint.Error{Code: "BP_E_TOO_LARGE", Message: "方案记录超过 1 MiB，请拆分目标"}
	}
	return string(data) + "\n", nil
}
