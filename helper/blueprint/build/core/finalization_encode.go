package build

// 文件职责：只读编码同内容已验证快照并复制结果投影。
// 使用：最终编码工具；边界：不修复实体、不调用生成器、不持有任务实例或发布获选预览。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"DysonSphereBox/helper/blueprint"
)

// Encode 职责：复核验证内容与版本后执行严格蓝图编码；使用：Validate 产物的唯一编码入口；边界：输入只读，每次返回独立统计记录、参数及报告，失败不返回部分串。
func Encode(ctx context.Context, snapshot VerifiedSnapshot) (Result, error) {
	if _, err := FreezeValidatedGeometry(ctx, snapshot.geometry, snapshot.validation); err != nil {
		return Result{}, err
	}
	data, validation := snapshot.geometry.data, snapshot.validation.data
	profile, err := verifiedRuleProfile()
	if err != nil {
		return Result{}, err
	}
	// 规则报告可带密铺覆盖后缀；物理规则来源及全部来源指纹仍必须精确匹配。
	actual := validation.audit.Profile
	if actual.GameVersion != profile.GameVersion || actual.SteamBuildID != profile.SteamBuildID || actual.AssemblySHA256 != profile.AssemblySHA256 || actual.ResourcesSHA256 != profile.ResourcesSHA256 || actual.Version != profile.Version && actual.Version != profile.Version+"-dense-single-paste" {
		return Result{}, &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "最终规则来源版本已经变化，必须重新验证"}
	}
	reportProgress(ctx, "encode", "正在编码并校验蓝图", 0, 0)
	c := newFinalizationCloner()
	document := c.blueprintDocument(data.document)
	encoded, err := blueprint.Encode(document, 1)
	if err != nil {
		return Result{}, err
	}
	if err := checkFinalGeometry(ctx, snapshot.geometry); err != nil {
		return Result{}, err
	}
	warnings := append([]string(nil), validation.warnings...)
	if len(document.Buildings) > 6000 {
		warnings = append(warnings, "建筑超过 6000，预览可能较慢，可折叠预览后复制导出")
	}
	density := copyFinalizationPointer(c, validation.density, c.buildDensityReport)
	digest := sha256.Sum256([]byte(encoded.Blueprint))
	density.BlueprintSHA256 = hex.EncodeToString(digest[:])
	return Result{
		statisticsRecords: document.Buildings, DensityReport: density,
		Blueprint: encoded.Blueprint, BuildingCount: len(document.Buildings), Areas: len(document.Areas),
		Warnings: warnings, SourcePlanID: data.request.Plan.PlanID,
		Bindings:              copyFinalizationSlice(c, data.bindings, c.buildProductionBinding),
		SharedBuildingIndices: append([]int(nil), data.shared...),
		Zones:                 copyFinalizationSlice(c, data.zones, c.buildZone),
		SurfacePlan:           copyFinalizationPointer(c, validation.surfacePlan, c.buildSurfacePlan),
		LogisticsSummary:      copyFinalizationPointer(c, validation.logistics, c.buildLogisticsSummary),
		RuleAudit:             copyFinalizationPointer(c, validation.audit, c.buildRuleAudit),
		previewScene:          c.previewCopy(data.previewScene),
	}, nil
}
