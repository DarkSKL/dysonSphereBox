package generator

// 文件职责：把实际工具轨迹投影到现有候选诊断契约。
// 使用：整体成功且编码完成之后；边界：报告不会改变实体版本，不把仅完成装配的步骤视为可行候选。

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// recordSpecification 职责：保存本次实际具名规格而非只有列表下标；使用：Solve工具入口；边界：纯诊断不修改领域内容版本。
func recordSpecification[T any](step *workflow.StepContext, family string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return step.Diagnostic("specification:" + family + ":" + string(data))
}

// candidatePath 职责：取得完整叶候选作用域；使用：汇合求解与验证轨迹；边界：只解释引擎固定路径，不据此控制运行。
func candidatePath(node string) string {
	index := strings.LastIndex(node, "/candidate:")
	if index < 0 {
		return ""
	}
	end := index + len("/candidate:")
	for end < len(node) && node[end] >= '0' && node[end] <= '9' {
		end++
	}
	return node[:end]
}

// attachWorkflowDiagnostics 职责：恢复公开候选报告及有界评估计数；使用：获选编码投影；边界：所有状态以真实调用结果为依据，不把未运行项当无解。
func attachWorkflowDiagnostics(result *core.Result, trace *workflow.RunResult) {
	if result.LogisticsSummary == nil || trace == nil {
		return
	}
	records := map[string]*core.SearchCandidateDiagnostic{}
	for _, step := range trace.Trace {
		key := candidatePath(step.Node)
		if key == "" {
			continue
		}
		isSolve := strings.HasSuffix(step.Tool, ".assemble@1") || step.Tool == "station.assemble-spec@1"
		isValidation := step.Tool == "rules.complete@1"
		if !isSolve && !isValidation && step.Failure == "" {
			continue
		}
		record := records[key]
		if record == nil {
			record = &core.SearchCandidateDiagnostic{Key: key, Scope: step.Tool, Stage: "assembly", Status: "pruned", Reason: "该优化候选未完成最终验证"}
			records[key] = record
		}
		if isSolve {
			record.Parameters = string(step.EffectiveParameters)
			for _, message := range step.Diagnostics {
				if strings.HasPrefix(message, "specification:") {
					record.Parameters = message
					break
				}
			}
		}
		if isValidation {
			record.Stage = "final-validation"
			if step.Failure == "" {
				record.Status, record.Reason = "passed", "实际实体已通过完整最终验证"
			}
		}
		if step.Failure != "" {
			record.Status = "rejected"
			record.Reason = boundedDiagnostic(step.Failure, 512)
			for _, message := range step.Diagnostics {
				if strings.HasPrefix(message, "failure-code:") {
					record.ErrorCode = strings.TrimPrefix(message, "failure-code:")
				}
			}
			if strings.HasPrefix(record.ErrorCode, "GEN_I_") {
				record.Status = "pruned"
			}
		}
	}
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	diagnostics := make([]core.SearchCandidateDiagnostic, 0, len(keys))
	rejected := 0
	for _, key := range keys {
		record := *records[key]
		diagnostics = append(diagnostics, record)
		if record.Status == "rejected" {
			rejected++
		}
	}
	result.LogisticsSummary.SearchCandidates = diagnostics
	result.LogisticsSummary.Metrics.CandidateBudget = trace.Candidates
	result.LogisticsSummary.Metrics.EvaluatedCandidates = len(diagnostics)
	result.LogisticsSummary.Metrics.RejectedCandidates = rejected
	result.LogisticsSummary.Checks = append(result.LogisticsSummary.Checks, core.LogisticsCheck{RuleID: "workflow-candidate-accounting", Status: "passed", Message: fmt.Sprintf("工作流共声明%d次有界候选尝试，实际进入%d次几何求解或基线复核，其中%d次拒绝；只有完整验证结果参与获选", trace.Candidates, len(diagnostics), rejected)})
}

// boundedDiagnostic 职责：保留Unicode完整字符并限制公开诊断长度；使用：候选拒绝报告；边界：原始错误和工具轨迹保持完整。
func boundedDiagnostic(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
