package build

// 文件职责：把同一份守恒流图和真实分行变成独立于摆位的运输份额。本步骤之后，各候选才创建自己的端口和传送带。

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// transportRowSourceKey 标明当前候选的已验证输入，用来核实量化方案和图的实际来源。
type transportRowSourceKey struct{}

// maxRetainedRowAllocations 限定单个共享分配的数组容量，也限制仅按行数推测的初始预留。
const maxRetainedRowAllocations = 8192

// verifiedRowSource 检查当前图、配方数据和生产切片是否来自已经通过生产容量规则的同一快照。
func (b *builder) verifiedRowSource(ctx context.Context, req Request, dataset *quant.Dataset) *PreparedInput {
	source, _ := ctx.Value(transportRowSourceKey{}).(*PreparedInput)
	if source == nil || source.flowFactID == "" || source.graph != b.flowGraph || source.dataset != dataset ||
		len(req.Plan.Productions) == 0 || len(req.Plan.Productions) != len(source.request.Plan.Productions) ||
		&req.Plan.Productions[0] != &source.request.Plan.Productions[0] {
		return nil
	}
	return source
}

// transportRowAllocation 只保存规范边序号、生产行序号和数量；负行号表示工厂边界。
type transportRowAllocation struct {
	edge, from, to int
	rate           float64
	role           productionInputRole
}

// allocateTransportRows 在等价分行上复用完整的份额及容量证明。物料网络、实体、端口预留和布线在各自的候选内建立。
func (b *builder) allocateTransportRows(ctx context.Context, req Request, dataset *quant.Dataset, rows []*transportRow) ([]transportRowAllocation, error) {
	capacity := b.logistics.BeltCapacityPerMinute
	boundary := capacity
	if b.directBoundaryUnits != nil && b.directBoundaryLaneLimit > 0 {
		boundary = math.Min(boundary, b.directBoundaryLaneLimit)
	}
	splitRoles := req.Mode == 1 || b.directBoundaryUnits != nil
	compute := func() ([]transportRowAllocation, error) {
		return planTransportAllocations(ctx, b.flowGraph, rows, capacity, boundary, splitRoles)
	}
	registry, _ := ctx.Value(candidateRegistryKey{}).(*candidateRegistry)
	source := b.verifiedRowSource(ctx, req, dataset)
	if registry == nil || source == nil {
		return compute()
	}
	// 来源指纹绑定全部配方、毛/净流及规范边序。纬带、塔型和跨度对本步的影响已归入实际行序、用途与边界容量。
	key := make([]byte, 0, len(source.flowFactID)+24+len(rows)*3)
	key = binary.AppendUvarint(key, uint64(len(source.flowFactID)))
	key = append(key, source.flowFactID...)
	key = binary.LittleEndian.AppendUint64(key, math.Float64bits(capacity))
	key = binary.LittleEndian.AppendUint64(key, math.Float64bits(boundary))
	if splitRoles {
		key = append(key, 1)
	} else {
		key = append(key, 0)
	}
	for _, row := range rows {
		key = binary.AppendUvarint(key, uint64(row.owner))
		key = binary.AppendUvarint(key, uint64(row.count))
	}
	return registry.rowAllocations.DoRetaining(ctx, sha256.Sum256(key), compute, func(parts []transportRowAllocation) bool {
		// 按底层数组容量限制驻留内存，每任务最多16份；超大工厂仍可使用本次计算。
		return cap(parts) <= maxRetainedRowAllocations
	})
}

// rowFlow 保存某行可供配对的剩余数量；同一节点的行保持生成顺序。
type rowFlow struct {
	row       int
	remaining float64
	role      productionInputRole
}

// transportPortFlows 的队首是该物料尚有用量的第一行。耗尽的份额只出队一次。
type transportPortFlows map[int][]rowFlow

// next 返回本物料的首个可用份额，已耗尽的队首只检查一次。
func (ports transportPortFlows) next(item int) *rowFlow {
	flows := ports[item]
	for len(flows) > 0 && !(flows[0].remaining > 1e-8) {
		flows = flows[1:]
	}
	ports[item] = flows
	if len(flows) == 0 {
		return nil
	}
	return &flows[0]
}

// planTransportAllocations 稳定配对每条规范边的生产行与消费行，并按真实端口累加容量。
// 逻辑流：建立每行供需队列，再按规范边顺序取份额，最后按输入用途和配方产物次序验证。
func planTransportAllocations(ctx context.Context, graph *quant.FlowGraph, rows []*transportRow, capacity, boundary float64, splitRoles bool) ([]transportRowAllocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !finitePositive(capacity) || !finitePositive(boundary) {
		return nil, &blueprint.Error{Code: "GEN_E_BELT_CAPACITY", Message: "运输份额需要有限的正带容量"}
	}
	byOwner := make(map[int][]int)
	for i, row := range rows {
		byOwner[row.owner] = append(byOwner[row.owner], i)
	}
	supplies, demands := map[string]transportPortFlows{}, map[string]transportPortFlows{}
	for _, node := range graph.Nodes {
		if node.Kind != "production" {
			continue
		}
		outputs, inputs := transportPortFlows{}, transportPortFlows{}
		supplies[node.ID], demands[node.ID] = outputs, inputs
		for _, i := range byOwner[node.ProductionIndex] {
			row := rows[i]
			share := float64(row.count) / float64(row.info.MachineCount)
			for _, rate := range row.info.Outputs {
				outputs[rate.ItemID] = append(outputs[rate.ItemID], rowFlow{row: i, remaining: rate.RatePerMinute * share})
			}
			for _, rate := range row.info.Inputs {
				if !splitRoles {
					inputs[rate.ItemID] = append(inputs[rate.ItemID], rowFlow{row: i, remaining: rate.RatePerMinute * share})
					continue
				}
				for _, role := range []productionInputRole{recipeInputRole, sprayInputRole} {
					part, err := productionRoleInputRate(row.info, row.recipe, rate.ItemID, role)
					if err != nil {
						return nil, err
					}
					if part > 0 {
						inputs[rate.ItemID] = append(inputs[rate.ItemID], rowFlow{row: i, remaining: part * share, role: role})
					}
				}
			}
		}
	}

	allocations := make([]transportRowAllocation, 0, max(len(graph.Edges), min(len(rows), maxRetainedRowAllocations)))
	for index, edge := range graph.Edges {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		from, hasFrom := supplies[edge.SourceNodeID]
		to, hasTo := demands[edge.TargetNodeID]
		limit := capacity
		if !hasFrom || !hasTo {
			limit = boundary
		}
		for left := edge.RatePerMinute; left > max(1e-8, edge.RatePerMinute*1e-10); {
			if len(allocations)%1024 == 0 && ctx.Err() != nil {
				return nil, ctx.Err()
			}
			part := transportRowAllocation{edge: index, from: -1, to: -1, rate: math.Min(left, limit)}
			var source, target *rowFlow
			if hasFrom {
				source = from.next(edge.ItemID)
				if source == nil {
					return nil, transportAllocationError(edge, left, "生产输出端口不足")
				}
				part.from, part.rate = source.row, math.Min(part.rate, source.remaining)
			}
			if hasTo {
				target = to.next(edge.ItemID)
				if target == nil {
					return nil, transportAllocationError(edge, left, "消费输入端口不足")
				}
				part.to, part.role, part.rate = target.row, target.role, math.Min(part.rate, target.remaining)
			}
			if source != nil {
				source.remaining = math.Max(0, source.remaining-part.rate)
			}
			if target != nil {
				target.remaining = math.Max(0, target.remaining-part.rate)
			}
			allocations = append(allocations, part)
			left = math.Max(0, left-part.rate)
		}
	}
	if err := verifyTransportAllocationCapacity(graph, rows, allocations, capacity); err != nil {
		return nil, err
	}
	return allocations, nil
}

// verifyTransportAllocationCapacity 逐份额向所属行记账；沿用规范边的浮点累加顺序，每条份额只访问一次。
func verifyTransportAllocationCapacity(graph *quant.FlowGraph, rows []*transportRow, parts []transportRowAllocation, capacity float64) error {
	type port struct {
		row, item int
		role      productionInputRole
		output    bool
	}
	sums := make(map[port]float64, len(rows))
	firstInvalid := len(rows)
	add := func(key port, rate float64) {
		total := sums[key] + rate
		sums[key] = total
		if total > capacity+1e-6 {
			firstInvalid = min(firstInvalid, key.row)
		}
	}
	for _, part := range parts {
		item := graph.Edges[part.edge].ItemID
		if part.to >= 0 {
			add(port{row: part.to, item: item, role: part.role}, part.rate)
		}
		if part.from >= 0 {
			add(port{row: part.from, item: item, output: true}, part.rate)
		}
	}
	if firstInvalid == len(rows) {
		return nil
	}
	// 所有份额均为正；只需解释首个超载行，仍报告该端口完整累计值，按原料用途再按配方产物核对。
	row := rows[firstInvalid]
	for _, input := range row.info.Inputs {
		for _, role := range []productionInputRole{recipeInputRole, sprayInputRole} {
			if rate := sums[port{row: firstInvalid, item: input.ItemID, role: role}]; rate > capacity+1e-6 {
				return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行 %d 物品 %d 的独立输入用途汇入 %.3f/min，超过输入带容量 %.3f/min", row.owner, input.ItemID, rate, capacity)}
			}
		}
	}
	for _, item := range row.recipe.Results {
		if rate := sums[port{row: firstInvalid, item: item, output: true}]; rate > capacity+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行 %d 产物 %d 独立输出 %.3f/min，超过单带容量 %.3f/min", row.owner, item, rate, capacity)}
		}
	}
	return fmt.Errorf("机器行 %d 的超载端口与已核验的配方不符", row.owner)
}
