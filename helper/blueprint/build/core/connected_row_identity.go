package build

// 文件职责：按真实运输分行与街道能力消除等价直连候选，并预筛缺少必要行数的折行。
// 使用：完整规格归一后、预览和工作区创建前；边界：只改变身份键，返回首个原规格；不同输入或surface分别枚举，不跨预算复用。

import (
	"context"
	"sort"
	"strconv"

	"DysonSphereBox/helper/blueprint/build/candidates"
)

// connectedRowPlanIdentity 职责：保存当前输入中一次实际分行的不可变摘要；使用：跨度与容量探针的请求内缓存；边界：不保存实体、不证明最终可行。
type connectedRowPlanIdentity struct {
	// rows 职责：按真实运输行顺序保存生产归属、配方、设备和机器数；使用：精确等价键；边界：不以机器总数或面积替代分行。
	rows string
	// street 职责：保存各分组策略实际拥有的街道身份；使用：跨容量开关比较；边界：空键不能证明存在可合并街道。
	street map[int]string
	// multipleRows 职责：标记是否存在至少两行的同一生产者；使用：折行必要条件；边界：true仍须完整几何求解。
	multipleRows bool
	// rowCount 职责：记录真实拆行工作量；使用：大厂先尝试较少拆行的完整候选；边界：只影响尝试顺序，不删除任何不同策略。
	rowCount int
}

// connectedExecutionRows 职责：用真实运输规划生成行序及街道摘要；使用：语义去重和折行必要条件预筛；边界：探针独占，无预览，不运行几何或共享可变状态。
func connectedExecutionRows(ctx context.Context, input *PreparedInput, span float64, capacity bool) (connectedRowPlanIdentity, error) {
	probe, err := input.newAssembly(ctx, nil)
	if err != nil {
		return connectedRowPlanIdentity{}, err
	}
	probe.denseSpatial, probe.denseMaxSpan, probe.denseCapacityFill = true, span, capacity
	rows, err := probe.planTransportRows(ctx, input.request, input.dataset)
	if err != nil {
		return connectedRowPlanIdentity{}, err
	}
	result := connectedRowPlanIdentity{street: denseConnectedStreetPlanKeys(rows, probe.setting), rowCount: len(rows)}
	counts := map[int]int{}
	encoded := make([]byte, 0, len(rows)*20)
	for _, row := range rows {
		counts[row.owner]++
		result.multipleRows = result.multipleRows || counts[row.owner] >= 2
		for _, value := range []int{row.owner, row.recipe.ID, row.info.MachineID, row.count} {
			encoded = strconv.AppendInt(encoded, int64(value), 10)
			encoded = append(encoded, ',')
		}
		encoded = append(encoded, ';')
	}
	result.rows = string(encoded)
	return result, nil
}

// deduplicateConnectedRows 职责：合并只有跨度、容量或街道分组身份不同的真实等价规格；使用：直连枚举；边界：其余策略完整保留，折行仅在必要条件确定不成立时排除。
func deduplicateConnectedRows(ctx context.Context, input *PreparedInput, specs []ConnectedCandidateSpec, flowAwareRowThreshold int) ([]ConnectedCandidateSpec, error) {
	type rowPlanKey struct {
		span     float64
		capacity bool
	}
	type candidateKey struct {
		spec   ConnectedCandidateSpec
		rows   string
		street string
	}
	plans := make(map[rowPlanKey]connectedRowPlanIdentity)
	identities := make(map[ConnectedCandidateSpec]candidateKey, len(specs))
	eligible := make([]ConnectedCandidateSpec, 0, len(specs))
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		identity := candidateKey{spec: spec}
		if input.request.Density != densityDense && !spec.SpatialRows && input.geometry.Strategy != "native-window" {
			identities[spec] = identity
			eligible = append(eligible, spec)
			continue
		}
		key := rowPlanKey{spec.DenseSpan, spec.CapacityFill && input.request.Mode == 1 && input.request.Density == densityDense}
		plan, known := plans[key]
		if !known {
			var err error
			plan, err = connectedExecutionRows(ctx, input, key.span, key.capacity)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				// 规划失败没有等价证明；保留原规格交给正常候选失败路径，不能因此终止其它策略。
				identities[spec] = identity
				eligible = append(eligible, spec)
				continue
			}
			plans[key] = plan
		}
		// 两个同生产者行单元是L/U的必要条件；填洞上下文允许无折行原样返回，不能在此排除。
		holes, _ := ctx.Value(denseSpatialHolesKey{}).(float64)
		if spec.BentRows && input.request.Mode == 1 && input.request.Density == densityDense && !plan.multipleRows && holes <= 0 {
			continue
		}
		identity.spec.DenseSpan = 0
		// 全场装箱最终按同一真实纬向范围截断高度；超过该范围的软高度不形成新的可执行布局。
		identity.spec.Limit = denseLatitudeHeightLimit(ctx, spec.Limit)
		identity.rows = plan.rows
		if !spec.Street {
			// 容量开关的几何作用已由实际分行完整表达；诊断和装配仍消费首个原规格。
			identity.spec.CapacityFill = false
		} else if street, valid := plan.street[spec.StreetGroupLimit]; valid && street != "" {
			// 必须同时具有相同行序及owner/counts/perStreet，才消除两项策略字段。
			identity.spec.CapacityFill, identity.spec.StreetGroupLimit = false, 0
			identity.street = street
		}
		identities[spec] = identity
		eligible = append(eligible, spec)
	}
	unique := candidates.StableUnique(eligible, func(spec ConnectedCandidateSpec) candidateKey { return identities[spec] })
	if input.request.Plan.MachineCount >= 256 && (input.request.Density == densityDense || input.geometry.Strategy == "native-window") {
		// 有限预算下先求较小的真实行图，避免短跨度把大厂拆成成千个独立装箱/接线路径后才尝试长行。
		count := func(spec ConnectedCandidateSpec) int {
			if plan, ok := plans[rowPlanKey{spec.DenseSpan, spec.CapacityFill && input.request.Mode == 1}]; ok {
				return plan.rowCount
			}
			return int(^uint(0) >> 1)
		}
		complex := func(spec ConnectedCandidateSpec) bool {
			return spec.Street || spec.BentRows || spec.Refine || spec.TranslateCompact || spec.ProducerSplits
		}
		// 只衡量当前优先的基础候选；尚未进入的街道/折行可能合并大量行，不能借它们的
		// 探针下界把基础分行已经很大的工厂误判为小工作量。
		unknown := int(^uint(0) >> 1)
		minimumRows := unknown
		for _, spec := range unique {
			if !complex(spec) {
				minimumRows = min(minimumRows, count(spec))
			}
		}
		preferFlowAware := input.request.Density == densityDense && minimumRows != unknown && flowAwareRowThreshold > 0 && minimumRows >= flowAwareRowThreshold
		sort.SliceStable(unique, func(i, j int) bool {
			// 普通大厂优先真实短行阵；它已按完整段净空形成矩形，旧连续长排仍保留为后备。
			if input.request.Density == densityStandard && unique[i].SpatialRows != unique[j].SpatialRows {
				return unique[i].SpatialRows
			}
			if complex(unique[i]) != complex(unique[j]) {
				return !complex(unique[i])
			}
			if count(unique[i]) != count(unique[j]) {
				return count(unique[i]) < count(unique[j])
			}
			// 仅在真实分行后仍有大量单元时优先物料图布局；其它同工作量场景保留配置顺序，
			// 避免仅按机器数推迟仍可用的基础布局。阈值只作用于排序，不缩减候选集合。
			if preferFlowAware {
				return unique[i].DenseOrder >= 2 && unique[j].DenseOrder < 2
			}
			return false
		})
	}
	recordCandidatePlans(ctx, "connected-machine-rows", len(specs), len(unique), 0)
	return unique, nil
}
