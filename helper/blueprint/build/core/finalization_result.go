package build

// 文件职责：封装可跨工作区共享的不可变编码产物与安全结果投影。
// 使用：编码工具提交EncodedSnapshot，服务按需取得独立Result；边界：保留未导出的统计记录与预览身份，不用JSON克隆或共享可写报告。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/candidates"
	"DysonSphereBox/helper/blueprint/build/preview"
	"DysonSphereBox/helper/factory/statistics"
)

// EncodedSnapshot 职责：私有持有完整编码结果；使用：工作流步骤产物；边界：所有可写容器只能通过Result深拷贝读出，零值不表示编码成功。
type EncodedSnapshot struct {
	// result 职责：保存编码结果及内部旁路载荷；使用：只读共享；边界：创建后不再写入。
	result Result
	// bytes 职责：保存保守内存计量；使用：工作区预算；边界：不是进程RSS。
	bytes int64
}

// EncodeSnapshot 职责：编码已验证产物并封装独占结果；使用：生产编码工具；边界：保持Encode错误语义，不发布获选预览或返回半成品。
func EncodeSnapshot(ctx context.Context, verified VerifiedSnapshot) (EncodedSnapshot, error) {
	result, err := Encode(ctx, verified)
	if err != nil {
		return EncodedSnapshot{}, err
	}
	c := newFinalizationCloner()
	result = c.resultCopy(result)
	return EncodedSnapshot{result: result, bytes: c.retainedBytes(4096)}, nil
}

// Result 职责：返回完整且可独立修改的结果投影；使用：服务发布、统计或预览提交；边界：内部统计参数及场景容器也复制，仅旁路会话保留不对外暴露的身份引用。
func (snapshot EncodedSnapshot) Result() Result { return CloneResult(snapshot.result) }

// EstimatedBytes 职责：返回编码产物保守保留大小；使用：工作流内存预算；边界：零值返回零，不计共享旁路会话的任务级所有权。
func (snapshot EncodedSnapshot) EstimatedBytes() int64 { return snapshot.bytes }

// CloneResult 职责：复制全部结果容器及未导出载荷；使用：需要跨步骤隔离Result的调用方；边界：旁路会话仅保留身份，领域记录、参数和展示场景无可变别名。
func CloneResult(source Result) Result { return newFinalizationCloner().resultCopy(source) }

// resultCopy 职责：执行完整结果领域复制并计量；使用：编码产物封装与读取；边界：不序列化结果，保留内部统计和预览数据。
func (c *finalizationCloner) resultCopy(source Result) Result {
	result := source
	result.Blueprint = c.copyString(source.Blueprint)
	result.BlueprintRef = c.copyString(source.BlueprintRef)
	result.SourcePlanID = c.copyString(source.SourcePlanID)
	result.WinnerCandidateID = c.copyString(source.WinnerCandidateID)
	result.CandidateDeduplication = copyFinalizationPointer(c, source.CandidateDeduplication, func(v candidates.Report) candidates.Report { return c.candidatesReport(v) })
	result.Statistics = copyFinalizationPointer(c, source.Statistics, func(v statistics.Report) statistics.Report { return c.statisticsReport(v) })
	result.statisticsRecords = copyFinalizationSlice(c, source.statisticsRecords, c.blueprintBuilding)
	result.DensityReport = copyFinalizationPointer(c, source.DensityReport, c.buildDensityReport)
	result.Warnings = copyFinalizationSlice(c, source.Warnings, c.copyString)
	result.Bindings = copyFinalizationSlice(c, source.Bindings, c.buildProductionBinding)
	result.SharedBuildingIndices = copyFinalizationSlice(c, source.SharedBuildingIndices, func(v int) int { return v })
	result.Zones = copyFinalizationSlice(c, source.Zones, c.buildZone)
	result.SurfacePlan = copyFinalizationPointer(c, source.SurfacePlan, c.buildSurfacePlan)
	result.LogisticsSummary = copyFinalizationPointer(c, source.LogisticsSummary, c.buildLogisticsSummary)
	result.RuleAudit = copyFinalizationPointer(c, source.RuleAudit, c.buildRuleAudit)
	result.Performance = copyFinalizationPointer(c, source.Performance, c.buildGenerationPerformance)
	result.previewScene = c.previewCopy(source.previewScene)
	return result
}

// previewCopy 职责：复制最终场景且保留旁路会话身份；使用：不可变产物投影；边界：会话指针只供原有发布协议使用，不属于实体真源或校验证据。
func (c *finalizationCloner) previewCopy(source *generationPreviewSnapshot) *generationPreviewSnapshot {
	if source == nil {
		return nil
	}
	c.bytes += 128
	return &generationPreviewSnapshot{session: source.session, scene: c.previewScene(source.scene)}
}

// captureFinalizationPreview 职责：在装配器消亡前保留稳定预览身份和最终姿态；使用：定稿冻结；边界：不把未验证候选标为成功、不发布场景，所有领域写入仍在冻结前完成。
func captureFinalizationPreview(b *builder, document blueprint.Document) *generationPreviewSnapshot {
	if b.preview == nil || !b.previewActive {
		return nil
	}
	if len(document.Buildings) != len(b.all) {
		return nil
	}
	ids := make(map[int32]string, len(document.Buildings))
	for i, record := range document.Buildings {
		ids[record.Index] = b.preview.objectID(b.all[i])
	}
	objects := make([]preview.Object, 0, len(document.Buildings))
	for i, record := range document.Buildings {
		building := renderPreviewBuilding(record, b.all[i].owner, false)
		building.InputObjectID, building.OutputObjectID = ids[record.InputObjIdx], ids[record.OutputObjIdx]
		objects = append(objects, preview.Object{ID: ids[record.Index], Building: building})
	}
	return &generationPreviewSnapshot{session: b.preview, scene: preview.Scene{Objects: objects, Frame: preview.Frame{CoordinateSystem: preview.AreaCoordinates, Areas: previewAreas(document.Areas), PrimaryAreaIndex: document.PrimaryAreaIndex}}}
}
