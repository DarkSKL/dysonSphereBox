package build

// 文件职责：逐项执行真实最终记录和物理规则；使用：独立规则工具及候选内部几何预检；边界：固定Go分派，不组合最终工作流，不降低已核对阈值。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
)

// executePlanetRule 职责：执行单个物理职责；使用：同一权威规则实现；边界：不以旧通过标记代替检查，不执行其它规则链。
func executePlanetRule(ctx context.Context, kind RuleKind, req Request, encoding *surfaceEncoding, records []blueprint.Building) (int, error) {
	switch kind {
	case RuleReferences:
		return 0, validateFinalReferences(ctx, records)
	case RulePastedSlots:
		return 0, validatePastedConnections(ctx, records)
	case RuleBeltDistance:
		return 0, encoding.validateBeltDistancesContext(ctx, records)
	case RuleStaticColliders:
		return 0, encoding.validateStaticColliders(ctx, records)
	case RuleTankPorts:
		return 0, encoding.validateTankBufferPorts(ctx, records)
	case RulePowerSpacing:
		return 0, encoding.validateSurfacePowerNodeSpacing(ctx, records)
	case RuleStationSpacing:
		return 0, encoding.validateSurfaceStationSpacing(ctx, records)
	case RuleBeltCrossings:
		return 0, encoding.validateBeltCrossings(ctx, records)
	case RuleBeltBuildingOccupancy:
		return 0, encoding.validateBeltBuildingOccupancy(ctx, records)
	case RuleInserterGeometry:
		return 0, encoding.validateInserterGeometry(ctx, records)
	case RuleSplitterClearance:
		return 0, encoding.validateSplitterClearance(ctx, records)
	case RuleStationPorts:
		return 0, validateStationPorts(ctx, records)
	case RuleStorageRoof:
		if req.Density == densityDense {
			return 0, encoding.validateStorageRoofClearance(ctx, records)
		}
		return 0, nil
	case RulePowerNetwork:
		if req.Setting.NeedPower != nil && !*req.Setting.NeedPower {
			return 0, nil
		}
		count, err := validateFinalPowerCoverage(ctx, encoding, records)
		if err != nil {
			return 0, err
		}
		if count > 1 {
			return 0, &blueprint.Error{Code: "GEN_E_POWER_NETWORK", Message: fmt.Sprintf("已生成感应塔分为 %d 个互不连通的内部电网，不能仅凭覆盖范围返回成功蓝图", count)}
		}
		return count, nil
	default:
		return 0, ruleProofError("不是已登记物理规则")
	}
}

// validateInserterGeometry 职责：执行真实槽位、修正跨度、方向与动态盒冲突；使用：独立分拣器规则和几何预检；边界：不检查其它动态设施，不改实体。
func (encoding *surfaceEncoding) validateInserterGeometry(ctx context.Context, records []blueprint.Building) error {
	boxes, err := encoding.surfaceInserterBoxes(ctx, records)
	if err != nil {
		return err
	}
	left, right, statistics, err := placement.FirstConflictWithStatistics(ctx, boxes)
	recordPerformanceCollision(ctx, "inserterBuildColliders", statistics.CandidatePairs, statistics.NarrowPhaseChecks)
	if err != nil {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "分拣器动态占用检查已取消或超时"}
		}
		return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	if left >= 0 {
		return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("分拣器动态建造盒冲突：建筑 %d 与建筑 %d", left, right)}
	}
	return nil
}
