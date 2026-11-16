package build

// 文件职责：提供共享喷涂供剂、密度归一和成功产物发布辅助。
// 使用：领域装配与旁路观察适配；边界：不包含生成入口、候选调度或装配器收尾桥。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"math"
	"sort"
)

// coatSupply 职责：连接同组喷涂机供剂带；使用：有喷涂的布局；边界：默认暴露供给口，不伪造已存在的增产剂。
func (b *builder) coatSupply(lines []*line) error {
	created := false
	for _, l := range lines {
		if err := b.checkGeneration("喷涂供剂干线"); err != nil {
			return err
		}
		if l == nil {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂供剂缺少已装配机器行"}
		}
		if len(l.coaters) == 0 {
			continue
		}
		if l.coaterInput != nil {
			continue
		}
		if len(l.machines) == 0 {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的喷涂机器行没有任何真实机器，拒绝输出装饰喷涂网络", l.recipe.ID)}
		}
		// 原料切片保持配方逻辑顺序，底侧物理槽位已反转；供剂干线
		// 必须按喷涂机真实Y排序，不能把逻辑首尾当作递增几何区间。
		coaters := append([]*placed(nil), l.coaters...)
		for _, coater := range coaters {
			if coater == nil {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂机器行存在空喷涂端点"}
			}
		}
		sort.SliceStable(coaters, func(i, j int) bool { return coaters[i].a.y < coaters[j].a.y })
		first, last := coaters[0], coaters[len(coaters)-1]
		for _, coater := range coaters {
			if math.Abs(coater.a.x-first.a.x) > 1e-6 {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 的喷涂口横坐标不一致，当前局部供剂干线无法连接", l.recipe.ID)}
			}
		}
		// 入口降到地面，再用一个水平格加一层的真实斜坡进入供剂轨；
		// 各模式可以统一连接地面端点，不要求箱体臂跨层斜插。
		startY := first.a.y - 1
		for _, row := range append(append([][]*placed{}, l.inputs...), l.outputRows()...) {
			if len(row) > 0 {
				startY = math.Min(startY, row[0].a.y-1)
			}
		}
		points := []vector{{first.a.x - 3, startY, 0}, {first.a.x - 2, startY, 0}}
		for y := startY; y <= last.a.y; y++ {
			points = append(points, vector{first.a.x - 1, y, 1})
		}
		owner := b.owner
		b.owner = l.machines[0].owner
		belts := b.byPath(points)
		b.owner = owner
		// 塔接驳从l.all读取占位；供剂入口必须同步登记到本行独立切片，
		// 避免在共享底层数组上append覆盖随后产线或让斜坡成为隐形障碍。
		l.all = append(append([]*placed(nil), l.all...), belts...)
		id := productionSprayItem(l.info)
		if id == 0 {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的喷涂设施没有合法增产剂等级", l.recipe.ID)}
		}
		belts[0].record.Parameters = []int32{int32(id), 0}
		l.coaterInput = belts[0]
		b.coaterLinks[id] = append(b.coaterLinks[id], belts[0])
		created = true
	}
	if created {
		b.warnings = append(b.warnings, "喷涂机供剂已按物流诊断中的物料来源接入；标记为外部供给的端口需持续提供增产剂，供剂不足会降低产能")
	}
	return nil
}

// normalizeDensity 职责：规范可信输入密度键；使用：输入准备与测试；边界：空值兼容普通模式，未知值由装配能力检查拒绝。
func normalizeDensity(value string) string {
	if value == "" {
		return densityStandard
	}
	return value
}

// publishFinalBlueprintProgress 职责：只为最终成功蓝图发布预览事件；使用：Generate 各模式选择完成后；边界：候选布局和失败结果永不泄露为动画结果。
func publishFinalBlueprintProgress(ctx context.Context, request Request, dataset *quant.Dataset, result Result, generationErr error) (Result, error) {
	if generationErr != nil {
		return result, generationErr
	}
	if err := commitGenerationPreview(ctx, result); err != nil {
		return Result{}, err
	}
	if result.previewScene != nil && result.previewScene.session != nil {
		result.WinnerCandidateID = result.previewScene.session.candidateID
	}
	result.previewScene = nil
	if err := emitBlueprintProgress(ctx, request, result, dataset); err != nil {
		return Result{}, err
	}
	return result, nil
}
