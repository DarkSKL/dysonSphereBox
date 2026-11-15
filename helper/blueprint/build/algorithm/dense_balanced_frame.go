package build

// 文件职责：为大型球面塔产线提供保留供电空间的均衡矩形候选。
// 使用：与既有候选独立比较；边界：不改设备、配方、地区或最终碰撞规则，不使用跨请求共享状态。

import (
	"context"
	"fmt"
	"math"
)

// denseBalancedFrameVariantOffset 职责：区分均衡矩形与既有共享塔候选；使用：内部候选编码；边界：不是公开模式编号。
const denseBalancedFrameVariantOffset = 200

// denseBalancedFrameKey 职责：启用当前候选的完整纬向装箱与精确供电净空；使用：候选私有上下文；边界：既有候选保持原路径。
type denseBalancedFrameKey struct{}

// denseUnitPowerReserveKey 职责：缩小预排供电半径以保留球面变换余量；使用：单元预置电塔；边界：正式覆盖验证仍使用真实半径。
type denseUnitPowerReserveKey struct{}

// denseUnitPowerRequestedKey 职责：保存用户原始供电选项；使用：局部装配暂时关闭供电时预置电塔；边界：不把临时禁用误认为用户选择，也不覆盖用户明确关闭。
type denseUnitPowerRequestedKey struct{}

// denseBalancedFrameEnabled 职责：读取候选私有策略；使用：装箱与供电索引；边界：空上下文或未启用时返回假。
func denseBalancedFrameEnabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(denseBalancedFrameKey{}).(bool)
	return enabled
}

// denseBalancedFrameEligible 职责：限制新增搜索到非极区大型原生候选；使用：公开候选登记；边界：小厂、普通模式与极点沿用已有路径。
func denseBalancedFrameEligible(ctx context.Context, req Request) bool {
	root, _ := ctx.Value(densityPolarRootKey{}).(bool)
	if !root || req.Density != densityDense || req.SurfaceLayout == nil || req.Plan.MachineCount < denseNativeLargeMachineThreshold {
		return false
	}
	selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	return ok && selection.tier < 5
}

// denseBalancedFrameContext 职责：给宽纬带保留五分之四高度、窄纬带保留完整预算；使用：同一候选装配与终检；边界：幂等且只缩小内部预算，不借用其它档位。
func denseBalancedFrameContext(ctx context.Context) context.Context {
	if denseBalancedFrameEnabled(ctx) {
		return ctx
	}
	if band, ok := ctx.Value(densityNativeBandKey{}).(surfacePackedBand); ok {
		height := band.high - band.low + 1
		if height > 32 {
			height = int(math.Ceil(float64(height) * .8))
		}
		band.high = band.low + height - 1
		ctx = context.WithValue(ctx, densityNativeBandKey{}, band)
	}
	return context.WithValue(ctx, denseBalancedFrameKey{}, true)
}

// reserveDenseUnitPower 职责：在各刚性单元内预置真实电塔并纳入装箱；使用：球面变换前；边界：不强开供电，不修改已有实体，正式球面覆盖与冗余裁剪仍在收尾执行。
// 逻辑流：1. 为含耗电实体的单元独立求解 => 2. 记录真实原型占地 => 3. 合并新电塔与单元；失败由整个候选回滚。
func (b *builder) reserveDenseUnitPower(ctx context.Context, units []denseSpatialUnit) error {
	requested, _ := ctx.Value(denseUnitPowerRequestedKey{}).(bool)
	if !requested {
		return nil
	}
	localContext := context.WithValue(ctx, denseUnitPowerReserveKey{}, true)
	total := 0
	for i := range units {
		if err := ctx.Err(); err != nil {
			return err
		}
		needed := false
		for _, p := range units[i].entities {
			needed = needed || finalPowerConsumer(p.record)
		}
		if !needed {
			continue
		}
		local, err := b.newCandidateBuilder()
		if err != nil {
			return err
		}
		local.connectedGlobalPower, local.denseSpatial = true, true
		enabled := true
		local.setting.NeedPower = &enabled
		local.generationCtx, local.preview = localContext, nil
		local.all = append([]*placed(nil), units[i].entities...)
		begin := len(local.all)
		if err := local.completeConnectedPower(localContext); err != nil {
			return fmt.Errorf("球面单元%d预置供电失败：%w", i, err)
		}
		poles := local.all[begin:]
		for _, p := range poles {
			p.layoutFootprint, err = prototypeLayoutFootprint(p.record.ItemID, p.record.ModelIndex)
			if err != nil {
				return err
			}
		}
		b.all = append(b.all, poles...)
		units[i].entities = append(append([]*placed(nil), units[i].entities...), poles...)
		total += len(poles)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-balanced-frame", Status: "passed", Message: fmt.Sprintf("%d个真实电力节点随单元参与球面装箱；预排使用90%%覆盖半径，最终仍检查真实覆盖、实体净空与设施间距", total)})
	return nil
}
