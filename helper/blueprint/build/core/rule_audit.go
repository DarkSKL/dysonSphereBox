package build

// 文件职责：对所有布局执行版本化、可取消的最终记录规则审计。
// 使用：finish 在 Area 适配和统一编号后、编码前调用；边界：硬失败不返回半成品，离线未知环境明确标记未验证。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// validateFinalReferences 职责：拒绝非有限端点、重复编号及外部悬空引用；使用：最终规则入口；边界：允许带和合法边界无输入或输出，不以闭合索引冒充物料连通。
func validateFinalReferences(ctx context.Context, records []blueprint.Building) error {
	byIndex := make(map[int32]bool, len(records))
	for _, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终记录检查已取消或超时"}
		}
		if record.Index < 0 || byIndex[record.Index] {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "最终建筑编号重复或为负数"}
		}
		byIndex[record.Index] = true
		for _, value := range []float32{record.LocalOffset.X, record.LocalOffset.Y, record.LocalOffset.Z, record.LocalOffset2.X, record.LocalOffset2.Y, record.LocalOffset2.Z, record.Yaw, record.Pitch, record.Tilt, record.Yaw2, record.Pitch2, record.Tilt2} {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("建筑 %d 的最终坐标或旋转无效", record.Index)}
			}
		}
	}
	for _, record := range records {
		for _, connection := range []int32{record.InputObjIdx, record.OutputObjIdx} {
			if connection < -1 || connection >= 0 && (connection == record.Index || !byIndex[connection]) {
				return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("建筑 %d 连接 %d 无效或指向蓝图外部", record.Index, connection)}
			}
		}
	}
	return nil
}

// validateFinalPlanetRules 职责：以最终数据验证索引、建造盒、动态占用、塔端口和供电覆盖；使用：全部模式共同编码出口；边界：平面为切平面检查，完整游戏状态永不伪装为通过。
func validateFinalPlanetRules(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) (*RuleAudit, error) {
	return reuseFinalPlanetRules(ctx, req, encoding, records)
}

// checkFinalPlanetRules 职责：供候选几何预检复用相同逐项纯规则；使用：几何搜索内部预检；边界：不是最终工作流批准，不签发RuleProof。
func checkFinalPlanetRules(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) (*RuleAudit, error) {
	profile, err := verifiedRuleProfile()
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	original := encoding
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	network := 0
	for _, kind := range []RuleKind{RuleReferences, RulePastedSlots, RuleBeltDistance, RuleStaticColliders, RuleTankPorts, RulePowerSpacing, RuleStationSpacing, RuleBeltCrossings, RuleBeltBuildingOccupancy, RuleInserterGeometry, RuleSplitterClearance, RuleStationPorts, RuleStorageRoof, RulePowerNetwork} {
		count, err := executePlanetRule(ctx, kind, req, encoding, records)
		if err != nil {
			return nil, err
		}
		if kind == RulePowerNetwork {
			network = count
		}
	}
	gameVersion := profile.GameVersion
	if req.Plan.Dataset != "" {
		dataset, err := quant.LoadDataset(req.Plan.Dataset)
		if err != nil {
			return nil, err
		}
		gameVersion = dataset.Info.GameVersion
	}
	return composePlanetRuleAudit(ctx, profile, req, original, gameVersion, network), nil
}

// composePlanetRuleAudit 职责：仅从已执行规则的来源及数值结果组装既有报告；使用：完整证明汇合和几何预检出口；边界：不检查实体、不读取外部配置或代跑规则。
func composePlanetRuleAudit(ctx context.Context, profile RuleProfile, req Request, encoding *surfaceEncoding, quantGameVersion string, networkCount int) *RuleAudit {
	audit := &RuleAudit{Profile: profile, QuantDataVersion: req.Plan.DataVersion, CoordinateSpace: "planet-surface", Coverage: "partial-numeric", Checks: []LogisticsCheck{}, Unverified: []string{"既有世界实体与真实地形", "Unity Physics、运行时 Mod 与科技环境", "外部原料、增产剂、机队与电力供应", "实际游戏放置与连续十分钟产量"}}
	if req.Plan.Dataset != "" && quantGameVersion != profile.GameVersion {
		message := fmt.Sprintf("量化配方版本 %s 与几何规则版本 %s 不同；分别锁定来源，跨版本一致性尚未在游戏中校准", quantGameVersion, profile.GameVersion)
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "quant-rule-game-version", Status: "unverified", Message: message})
		audit.Unverified = append(audit.Unverified, message)
	}
	if encoding == nil {
		audit.CoordinateSpace = "equatorial-tangent"
		audit.Unverified = append(audit.Unverified, "未指定球面模式的纬圈收缩与跨 Area 粘贴")
	}
	if req.Density == densityDense {
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "storage-roof-clearance", Status: "passed", Message: "箱体和储液罐顶部在生成器走线高度范围内保留竖向扩建空间，按最终球面带段独立核验"})
	}
	if usesDenseBeltCollisions(ctx) {
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "dense-blueprint-belt-nodes", Status: "passed", Message: "密铺按原版整图单次粘贴处理同批带间豁免，并检查重叠节点合并；未将整段路径粗管或假想节点盒作为额外限制"})
		audit.Unverified = append(audit.Unverified, "密铺整图单次粘贴以外的拖拽复制、既有线路合并与建造顺序；请在游戏中按完整蓝图验证")
	}
	beltCoverage := "带连接距离、无连接交叉、建筑占用与喷涂通道"
	if usesDenseBeltCollisions(ctx) {
		beltCoverage = "带连接距离、整图单次粘贴的带间豁免及节点合并检查、建筑占用与喷涂通道"
		audit.Profile.Version += "-dense-single-paste"
	}
	for _, check := range []LogisticsCheck{
		{RuleID: "splitter-conservative-clearance", Status: "passed", Message: "三形态分流器在原生细分建造盒之外独立保留完整选择体积，检查静态设施、实际带段及经槽位修正和长度拉伸的分拣器OBB；仅正确槽位、方向和高度的短接头开放，属于保守生成约束"},
		{RuleID: "final-references", Status: "passed", Message: "最终 float32 坐标、编号和引用闭合"},
		{RuleID: "pasted-connection-slots", Status: "passed", Message: "按游戏粘贴顺序重放独占连接槽，所有声明的带、分拣器和设施连接在后续写入后仍存续"},
		{RuleID: "static-build-colliders", Status: "passed", Message: "已核对原型 OBB 与三维空间索引，无未知静态模型"},
		{RuleID: "belt-distance-occupancy", Status: "passed", Message: beltCoverage},
		{RuleID: "inserter-slot-geometry", Status: "passed", Message: "分拣器槽位修正、方向、长度、跨格和动态 OBB"},
		{RuleID: "power-node-spacing", Status: "passed", Message: "按真实电力节点类型检查独立最小间距"},
		{RuleID: "station-spacing-ports", Status: "passed", Message: "物流塔独立最小间距、物品槽、容量与唯一带端口"},
	} {
		audit.Checks = append(audit.Checks, check)
	}
	if req.Setting.NeedPower == nil || *req.Setting.NeedPower {
		audit.PowerNetworkCount = networkCount
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "power-coverage", Status: "passed", Message: "全部消费者接电位置在感应塔覆盖内，内部电塔已连成单网；仍需接入充足外部电源"})
	} else {
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "power-coverage", Status: "unverified", Message: "用户关闭供电设施生成，必须在游戏中补齐电网"})
	}
	audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "game-placement-runtime", Status: "unverified", Message: "数值预检为部分覆盖，不等于实际放置、完整 Unity Physics 或连续运行产量验收"})
	return audit
}
