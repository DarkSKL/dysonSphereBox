package build

// 文件职责：为直连生产行建立自身喷涂剂的实体毛流回路。
// 使用：已完成过滤分支的机器行；边界：不增加规范流边或外供，不绕过容量和最终几何规则。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// connectedSelfSpray 职责：保存本行自产剂回流的实体证据；使用：装配与最终核验；边界：毛流与规范净流分开核算。
type connectedSelfSpray struct {
	row         *transportRow
	item        int
	rate        float64
	source, arm *placed
	path        []*placed
}

// prepareConnectedSelfSpray 职责：从机器出货带过滤取料接回本行喷涂入口；使用：mode1 机器行落位前；边界：仅处理净供剂输入为零的自产剂并验证有限真实分拣器能力。
func (b *builder) prepareConnectedSelfSpray(row *transportRow, begin int) error {
	item := productionSprayItem(row.info)
	if item == 0 || row.line == nil || row.line.coaterInput == nil || productionItemRate(row.info.Inputs, item) > 0 {
		return nil
	}
	output := row.line.outputForItem(item)
	if len(output) == 0 {
		return nil
	}
	capacity := row.info.Capacity
	if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || !finitePositive(capacity.OutputPerMinute) || row.info.MachineCount <= 0 {
		return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "直连自产增产剂缺少可信配置毛流"}
	}
	rate := productionItemRate(capacity.GrossInputs, item) * row.info.OutputCount / capacity.OutputPerMinute * float64(row.count) / float64(row.info.MachineCount)
	if !finitePositive(rate) || rate > b.logistics.BeltCapacityPerMinute+1e-6 {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "直连自产增产剂回流毛流无效或超过单带能力"}
	}
	ctx := b.generationCtx
	if ctx == nil {
		ctx = context.Background()
	}
	target := row.inputs[item]
	if target == nil {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "直连自产剂缺少本行供剂入口"}
	}
	sources := []*placed{}
	visited := map[*placed]bool{}
	lastFilter := (*placed)(nil)
	for _, route := range row.outputs {
		if route.edge.ItemID == item && len(route.filters) > 0 {
			lastFilter = route.filters[len(route.filters)-1].input
		}
	}
	for _, seed := range output {
		if seed.productionRate == nil {
			continue
		}
		for source := seed; source != nil && !visited[source] && surfaceIsBelt(source.record.ItemID); source = source.output {
			visited[source] = true
			sources = append(sources, source)
			if source == lastFilter {
				break
			}
		}
	}
	for index := len(sources) - 1; index >= 0; index-- {
		source := sources[index]
		for _, delta := range []vector{{0, -1, 0}, {0, 1, 0}, {-1, 0, 0}, {1, 0, 0}} {
			start := vector{source.a.x + delta.x, source.a.y + delta.y, source.a.z}
			points, err := b.flySelfSprayPath(start, target, begin)
			if err != nil {
				if ctx.Err() != nil {
					return err
				}
				continue
			}
			if err := b.validateFlyPathEndpoints(points, source, target, nil, b.all[begin:]); err != nil {
				if ctx.Err() != nil {
					return err
				}
				continue
			}
			checkpoint := len(b.all)
			path := b.byPath(points[:len(points)-1])
			arm, err := b.beltTransfer(source, path[0], b.setting.InserterLv, item)
			if err != nil {
				return err
			}
			actual, err := flyEntityInserterCapacity(arm)
			if err != nil {
				return err
			}
			if actual+1e-6 < rate {
				return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "直连自产剂回流超过真实单臂能力"}
			}
			connect(path[len(path)-1], target)
			path = append(path, target)
			local := &builder{all: b.all[begin:]}
			records, _, _, err := local.records()
			if err != nil {
				return err
			}
			boxes, err := newPlanarRuleEncoding().surfaceInserterBoxes(ctx, records)
			if err != nil {
				return err
			}
			left, _, err := placement.FirstConflict(ctx, boxes)
			if err != nil {
				return err
			}
			if left >= 0 {
				b.all = b.all[:checkpoint]
				continue
			}
			row.selfSpray = &connectedSelfSpray{row: row, item: item, rate: rate, source: source, arm: arm, path: path}
			if err := b.addFlyPower(arm.a, begin); err != nil {
				return err
			}
			b.warnings = append(b.warnings, fmt.Sprintf("产线%d的自产增产剂%d经真实过滤臂回流，启动须预充库存；离线不验证实际启动和持续运行", row.owner, item))
			return nil
		}
	}
	return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("产线%d自产增产剂没有满足占位和有界预算的回流路径", row.owner)}
}

// validateConnectedSelfSprays 职责：按真实过滤臂、连续带及共享干线核验自产剂回流；使用：最终物流收尾；边界：不得漏计毛流或把回流当成外供。
func (b *builder) validateConnectedSelfSprays(indices map[*placed]int, loads map[*placed]float64) error {
	seen := map[*transportRow]bool{}
	for _, route := range b.transportRoutes {
		for _, row := range []*transportRow{route.from, route.to} {
			if row == nil || seen[row] || row.selfSpray == nil {
				continue
			}
			seen[row] = true
			r := row.selfSpray
			if r.row != row || r.arm == nil || r.source == nil || len(r.path) < 2 || r.arm.input != r.source || r.arm.output != r.path[0] || int(r.arm.record.FilterID) != r.item || r.path[len(r.path)-1] != row.inputs[r.item] {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "自产剂回流过滤臂或真实供剂端点断开"}
			}
			if _, ok := indices[r.arm]; !ok {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "自产剂回流臂不属于最终蓝图"}
			}
			if err := flyInserterSpanMatchesGeometry(r.arm); err != nil {
				return err
			}
			capacity, err := flyEntityInserterCapacity(r.arm)
			if err != nil {
				return err
			}
			if capacity+1e-6 < r.rate {
				return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "自产剂回流过滤臂能力不足"}
			}
			if err := validateBusFabricChain(r.path, indices); err != nil {
				return err
			}
			for _, p := range r.path {
				loads[p] += r.rate
			}
			output := row.line.outputForItem(r.item)
			found := false
			if len(output) == 0 {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "自产剂缺少真实生产带"}
			}
			visited := map[*placed]bool{}
			for p := output[len(output)-1]; p != nil && !visited[p]; p = p.output {
				if _, ok := indices[p]; !ok || !surfaceIsBelt(p.record.ItemID) {
					break
				}
				visited[p] = true
				loads[p] += r.rate
				if p == r.source {
					found = true
					break
				}
			}
			if !found {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "自产剂过滤臂未连接真实生产带"}
			}
		}
	}
	if len(seen) > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "connected-self-spray-recycle", Status: "passed", Message: fmt.Sprintf("%d 条自产剂回流以真实过滤臂和有向带接回喷涂入口，回流毛流计入共享带容量；启动库存及游戏运行未验证", len(seen))})
	}
	return nil
}
