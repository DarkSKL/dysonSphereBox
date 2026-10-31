package build

// 文件职责：对所有布局执行版本化、可取消的最终记录规则审计。
// 使用：finish 在 Area 适配和统一编号后、编码前调用；边界：硬失败不返回半成品，离线未知环境明确标记未验证。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
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
	profile, err := verifiedRuleProfile()
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	if err := validateFinalReferences(ctx, records); err != nil {
		return nil, err
	}
	audit := &RuleAudit{Profile: profile, QuantDataVersion: req.Plan.DataVersion, CoordinateSpace: "planet-surface", Coverage: "partial-numeric", Checks: []LogisticsCheck{}, Unverified: []string{"既有世界实体与真实地形", "Unity Physics、运行时 Mod 与科技环境", "外部原料、增产剂、机队与电力供应", "实际游戏放置与连续十分钟产量"}}
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
		audit.CoordinateSpace = "equatorial-tangent"
		audit.Unverified = append(audit.Unverified, "未指定球面模式的纬圈收缩与跨 Area 粘贴")
	}
	// 球面候选已执行的同一最终记录规则不重复计算；平面不能再绕过这些硬约束。
	if !encoding.plan.BeltConnectionsChecked {
		if err := encoding.validateBeltDistancesContext(ctx, records); err != nil {
			return nil, err
		}
	}
	if !encoding.plan.StaticCollidersChecked {
		if err := encoding.validateStaticColliders(ctx, records); err != nil {
			return nil, err
		}
	}
	if encoding.plan.CollisionCoverage != "staticBuildColliders+powerNodeSpacing+stationSpacing+beltCrossings+beltBuildingOccupancy+spraycoaterChannels+inserterSlotPoses+inserterBuildColliders" {
		if err := encoding.validateDynamicFacilities(ctx, records); err != nil {
			return nil, err
		}
	}
	if err := validateStationPorts(ctx, records); err != nil {
		return nil, err
	}
	for _, check := range []LogisticsCheck{
		{RuleID: "final-references", Status: "passed", Message: "最终 float32 坐标、编号和引用闭合"},
		{RuleID: "static-build-colliders", Status: "passed", Message: "已核对原型 OBB 与三维空间索引，无未知静态模型"},
		{RuleID: "belt-distance-occupancy", Status: "passed", Message: "带连接距离、无连接交叉、建筑占用与喷涂通道"},
		{RuleID: "inserter-slot-geometry", Status: "passed", Message: "分拣器槽位修正、方向、长度、跨格和动态 OBB"},
		{RuleID: "power-node-spacing", Status: "passed", Message: "按真实电力节点类型检查独立最小间距"},
		{RuleID: "station-spacing-ports", Status: "passed", Message: "物流塔独立最小间距、物品槽、容量与唯一带端口"},
	} {
		audit.Checks = append(audit.Checks, check)
	}
	if req.Setting.NeedPower == nil || *req.Setting.NeedPower {
		audit.PowerNetworkCount, err = validateFinalPowerCoverage(ctx, encoding, records)
		if err != nil {
			return nil, err
		}
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "power-coverage", Status: "passed", Message: fmt.Sprintf("全部消费者在感应塔覆盖内；%d 个网络仍需玩家连接充足电源", audit.PowerNetworkCount)})
	} else {
		audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "power-coverage", Status: "unverified", Message: "用户关闭供电设施生成，必须在游戏中补齐电网"})
	}
	audit.Checks = append(audit.Checks, LogisticsCheck{RuleID: "game-placement-runtime", Status: "unverified", Message: "数值预检为部分覆盖，不等于实际放置、完整 Unity Physics 或连续运行产量验收"})
	return audit, nil
}
