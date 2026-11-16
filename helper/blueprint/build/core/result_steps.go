package build

// 文件职责：在编码完成后分别计算最终实体统计、运输升档说明并组合结果。
// 使用：生产工作流三个明确结果节点和独立核心对照；边界：不编码、不读装配器、不发布预览、不重校原请求或改变已编码实体。

import (
	"context"
	"crypto/sha256"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/factory/statistics"
)

// ResultStatistics 职责：封闭保存同一编码内容的实体统计；使用：统计节点交给结果组合；边界：私有报告创建后只读，零值和其它编码的报告不能组合。
type ResultStatistics struct {
	// encodedID/planID 职责：绑定编码内容和可信需求；使用：防止跨任务拼接；边界：不代替生成前的输入规则。
	encodedID [32]byte
	planID    string
	// report/bytes 职责：持有独立统计及保守保留空间；使用：不可变产物共享；边界：不暴露可变指针，不声称进程峰值。
	report *statistics.Report
	bytes  int64
}

// EstimatedBytes 职责：返回统计产物保守保留空间；使用：工作流内存预算；边界：零值返回零，不是RSS。
func (value ResultStatistics) EstimatedBytes() int64 { return value.bytes }

// ResultTransportUpgrade 职责：封闭保存原始规范档位与实际档位的说明；使用：运输说明节点交给组合；边界：无升档仍有内容身份，不修改设备或复制生成逻辑。
type ResultTransportUpgrade struct {
	// encodedID/planID 职责：限定说明所属编码及需求；使用：组合核对；边界：不能把其它候选说明附加到当前结果。
	encodedID [32]byte
	planID    string
	// warnings/checks/bytes 职责：保存独立文案、规则说明和保留空间；使用：只读组合；边界：提示不是新的物理验证证明。
	warnings []string
	checks   []LogisticsCheck
	bytes    int64
}

// EstimatedBytes 职责：返回运输说明保守空间；使用：实例预算；边界：零值无效，不代表没有完成说明步骤。
func (value ResultTransportUpgrade) EstimatedBytes() int64 { return value.bytes }

// resultStepError 职责：分类缺失或不匹配的结果产物；使用：三个纯步骤的前置检查；边界：不将内核产物故障伪装成布局无解。
func resultStepError(message string) error {
	return &blueprint.Error{Code: "GEN_E_RESULT", Message: message}
}

// encodedResultIdentity 职责：绑定已编码内容与来源；使用：结果步骤交接；边界：不重新编码或解码，封闭EncodedSnapshot由编码器创建。
func encodedResultIdentity(encoded EncodedSnapshot) ([32]byte, error) {
	result := encoded.result
	if result.Blueprint == "" || result.SourcePlanID == "" || result.BuildingCount <= 0 || result.LogisticsSummary == nil {
		return [32]byte{}, resultStepError("结果步骤缺少完整编码产物")
	}
	return sha256.Sum256([]byte(result.Blueprint)), nil
}

// CalculateResultStatistics 职责：依据获选规范需求和最终编码实体计算完整统计；使用：stats.calculate@1；边界：不改输入、无预览副作用、不重新求解或编码。
func CalculateResultStatistics(ctx context.Context, input *PreparedInput, encoded EncodedSnapshot) (ResultStatistics, error) {
	if err := ctx.Err(); err != nil {
		return ResultStatistics{}, err
	}
	id, err := encodedResultIdentity(encoded)
	if err != nil {
		return ResultStatistics{}, err
	}
	if input == nil || input.request.Plan.PlanID == "" || input.request.Plan.PlanID != encoded.result.SourcePlanID {
		return ResultStatistics{}, resultStepError("统计需求与编码来源不一致")
	}
	result := encoded.Result()
	if err := attachFactoryStatistics(ctx, input.request.Plan, &result); err != nil {
		if ctx.Err() != nil {
			return ResultStatistics{}, ctx.Err()
		}
		return ResultStatistics{}, &blueprint.Error{Code: "GEN_E_STATISTICS", Message: err.Error()}
	}
	if err := ctx.Err(); err != nil {
		return ResultStatistics{}, err
	}
	c := newFinalizationCloner()
	report := c.statisticsReport(*result.Statistics)
	return ResultStatistics{encodedID: id, planID: result.SourcePlanID, report: &report, bytes: c.retainedBytes(256)}, nil
}

// CalculateResultTransportUpgrade 职责：比较补默认后的初始用户档位与获选实际档位；使用：result.transport-upgrade@1；边界：只读既有输入，不重跑normalizeSetting或容量校验，不遗漏初次最低档位升档。
func CalculateResultTransportUpgrade(ctx context.Context, base, selected *PreparedInput, encoded EncodedSnapshot) (ResultTransportUpgrade, error) {
	if err := ctx.Err(); err != nil {
		return ResultTransportUpgrade{}, err
	}
	id, err := encodedResultIdentity(encoded)
	if err != nil {
		return ResultTransportUpgrade{}, err
	}
	if base == nil || selected == nil || base.request.Plan.PlanID == "" || base.request.Plan.PlanID != selected.request.Plan.PlanID || selected.request.Plan.PlanID != encoded.result.SourcePlanID || base.request.Plan.DataVersion != selected.request.Plan.DataVersion {
		return ResultTransportUpgrade{}, resultStepError("运输说明的原始需求、获选需求与编码来源不一致")
	}
	original, actual := base.requestedSetting, selected.request.Setting
	if original.BeltLv < 1 || original.InserterLv < 1 || actual.BeltLv < original.BeltLv || actual.InserterLv < original.InserterLv || encoded.result.LogisticsSummary.BeltLevel != actual.BeltLv {
		return ResultTransportUpgrade{}, resultStepError("运输说明缺少初始规范档位或与编码实际档位不符")
	}
	value := ResultTransportUpgrade{encodedID: id, planID: encoded.result.SourcePlanID, bytes: 256}
	if original.BeltLv != actual.BeltLv || original.InserterLv != actual.InserterLv {
		message := transportUpgradeMessage(original, actual)
		value.warnings = []string{message}
		value.checks = []LogisticsCheck{{RuleID: "automatic-transport-upgrade", Status: "passed", Message: message + "；已按实际设备通过完整终检"}}
		value.bytes += int64(len(message)+len(value.checks[0].Message)+128) * 2
	}
	return value, nil
}

// ComposeResult 职责：把同一编码的统计和运输说明组成独立最终结果；使用：result.compose@1；边界：不补跑缺失步骤、不重复编码或统计，克隆全部可写容器。
func ComposeResult(ctx context.Context, encoded EncodedSnapshot, computed ResultStatistics, upgrade ResultTransportUpgrade) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	id, err := encodedResultIdentity(encoded)
	if err != nil {
		return Result{}, err
	}
	if computed.report == nil || computed.encodedID != id || upgrade.encodedID != id || computed.planID != encoded.result.SourcePlanID || upgrade.planID != encoded.result.SourcePlanID {
		return Result{}, resultStepError("最终结果缺少同一编码内容的统计或运输说明")
	}
	result := encoded.Result()
	c := newFinalizationCloner()
	report := c.statisticsReport(*computed.report)
	result.Statistics = &report
	result.Warnings = append(result.Warnings, upgrade.warnings...)
	for _, check := range upgrade.checks {
		result.LogisticsSummary.Checks = append(result.LogisticsSummary.Checks, c.buildLogisticsCheck(check))
	}
	result.statisticsRecords = nil
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return result, nil
}

// ResultEstimatedBytes 职责：以完整强类型复制图估计最终结果保留空间；使用：结果产物预算；边界：包含未导出预览载荷，不用JSON漏掉内部容器，不代表峰值RSS。
func ResultEstimatedBytes(result Result) int64 {
	c := newFinalizationCloner()
	_ = c.resultCopy(result)
	return c.retainedBytes(4096)
}
