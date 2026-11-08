package build

// 文件职责：在全局路线落位后补齐直连模式独立原料入口的流速监测器。
// 使用：mode1 完成全图路由和带尾裁剪后、finish 之前调用；边界：不修改带链、不合并物料，不在局部块尚会平移时借用其它块的坐标或供电。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// completeBoundaryFlowMonitors 职责：保证每条独立外供 lane 的合流前原料带存在一个合法监测器；使用：直连候选全部落位后调用；边界：其它模式复用本地机器带监测，不创建额外外部带；失败撤回本函数新增设施。
// 逻辑流：1. 核对真实入口和独立带链 => 2. 复用已有监测器或按入口向内选点 => 3. 用全布局补电 => 4. 执行全局碰撞和供电复核；任一路无解即拒绝候选。
func (b *builder) completeBoundaryFlowMonitors(ctx context.Context, mode int) (err error) {
	if !b.setting.EnableFlowMonitor || mode != 1 && b.directBoundaryUnits == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	originalLength, originalOwner, originalContext := len(b.all), b.owner, b.generationCtx
	if err = b.checkGeneration("原料入口流速监测"); err != nil {
		return err
	}
	if originalContext != nil {
		combined, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(originalContext, cancel)
		defer cancel()
		defer stop()
		ctx = combined
	}
	b.generationCtx = ctx
	b.owner = -1
	defer func() {
		b.owner, b.generationCtx = originalOwner, originalContext
		if err != nil {
			b.all = b.all[:originalLength]
		}
	}()
	if err = b.checkGeneration("原料入口流速监测"); err != nil {
		return err
	}
	present := make(map[*placed]bool, len(b.all))
	incoming := make(map[*placed]int, len(b.all))
	existing := map[vector][]*placed{}
	items := map[*placed]map[int]bool{}
	routes := b.uniqueMaterialBoundaryRoutes(b.denseBoundaryMonitorRoutes())
	for index, entity := range b.all {
		if index%64 == 0 {
			if err = b.checkGeneration("原料入口监测占位索引"); err != nil {
				return err
			}
		}
		present[entity] = true
		if entity.output != nil && surfaceIsBelt(entity.record.ItemID) {
			incoming[entity.output]++
		}
		if entity.record.ItemID == 2030 {
			existing[entity.a] = append(existing[entity.a], entity)
		}
	}
	for _, route := range routes {
		if route == nil {
			return boundaryFlowMonitorError(nil, "物料路线为空，无法证明独立原料入口")
		}
		for index, host := range route.path {
			if index%64 == 0 {
				if err = b.checkGeneration("原料入口监测物品索引"); err != nil {
					return err
				}
			}
			if host == nil || !surfaceIsBelt(host.record.ItemID) {
				continue
			}
			if items[host] == nil {
				items[host] = map[int]bool{}
			}
			items[host][route.edge.ItemID] = true
		}
	}
	lanes := 0
	claimed := map[*placed]bool{}
	for _, route := range routes {
		if err = b.checkGeneration("原料入口流速监测"); err != nil {
			return err
		}
		if route.edge.FlowKind != "external-input" {
			continue
		}
		lanes++
		if claimed[route.boundary] {
			return boundaryFlowMonitorError(route, "不同独立原料 lane 重用了同一个入口实体")
		}
		claimed[route.boundary] = true
		hosts, hostErr := boundaryFlowMonitorHosts(ctx, route, present, incoming, items)
		if hostErr != nil {
			return hostErr
		}
		var selected *placed
		for index, host := range hosts {
			if index%64 == 0 {
				if err = b.checkGeneration("原料入口已有监测检索"); err != nil {
					return err
				}
			}
			for _, monitor := range existing[host.a] {
				if boundaryFlowMonitorMatches(host, monitor) {
					if !b.boundaryFlowMonitorExistingClear(host, monitor) {
						if err = b.checkGeneration("原料入口已有监测净空"); err != nil {
							return err
						}
						return boundaryFlowMonitorError(route, "已有监测器与全布局其它实体的建造净空冲突，不能将其视为合格入口监测")
					}
					selected = host
					break
				}
			}
			if selected != nil {
				break
			}
		}
		if selected != nil {
			if err = b.coverBoundaryFlowMonitor(selected); err != nil {
				return fmt.Errorf("原料入口监测器（物品 %d、边 %s）补电失败：%w", route.edge.ItemID, route.edge.ID, err)
			}
			continue
		}
		var powerErr error
		if selected == nil {
			for _, host := range hosts {
				if err = b.checkGeneration("原料入口监测选点"); err != nil {
					return err
				}
				if !b.flowMonitorNodeClear(host) {
					continue
				}
				begin := len(b.all)
				monitor := b.make(2030, host.a.x, host.a.y, host.a.z)
				monitor.record.Yaw, monitor.record.Yaw2 = host.record.Yaw, host.record.Yaw
				if powerErr = b.coverBoundaryFlowMonitor(host); powerErr != nil {
					b.all = b.all[:begin]
					if err = b.checkGeneration("原料入口监测补电"); err != nil {
						return err
					}
					continue
				}
				existing[monitor.a] = append(existing[monitor.a], monitor)
				selected = host
				break
			}
		}
		if selected == nil {
			if err = b.checkGeneration("原料入口监测选点"); err != nil {
				return err
			}
			if powerErr != nil {
				return fmt.Errorf("原料入口监测器（物品 %d、边 %s）没有同时满足净空与感应塔覆盖的节点：%w", route.edge.ItemID, route.edge.ID, powerErr)
			}
			return boundaryFlowMonitorError(route, "独立原料带在合流前没有满足全布局建造盒与分拣器净空的监测节点")
		}
	}
	if lanes == 0 {
		return nil
	}
	if err = b.validateBoundaryFlowMonitorLayout(ctx); err != nil {
		return fmt.Errorf("原料入口监测器全布局复核失败：%w", err)
	}
	return nil
}

// boundaryFlowMonitorExistingClear 职责：复用同点监测器时仍检查其它全局实体净空；使用：已有监测器合格性判断；边界：仅从临时视图排除监测器自身，保留宿主带、其它监测器和所有产线占位。
func (b *builder) boundaryFlowMonitorExistingClear(host, monitor *placed) bool {
	view := *b
	view.all = make([]*placed, 0, len(b.all)-1)
	for _, entity := range b.all {
		if entity != monitor {
			view.all = append(view.all, entity)
		}
	}
	return view.flowMonitorNodeClear(host)
}

// coverBoundaryFlowMonitor 职责：在全布局中复用感应塔或补齐入口监测供电；使用：原料节点候选与已有监测器；边界：显式关闭供电时不新增电塔，nil 采用归一配置的启用语义。
func (b *builder) coverBoundaryFlowMonitor(host *placed) error {
	if b.setting.NeedPower != nil && !*b.setting.NeedPower {
		return nil
	}
	// 坐标在此阶段已全局固定，begin=0 才允许复用其它产线的真实电塔。
	view := *b
	enabled := true
	view.setting.NeedPower = &enabled
	if err := view.addFlyPower(host.a, 0); err != nil {
		return err
	}
	b.all = view.all
	return nil
}

// boundaryFlowMonitorHosts 职责：提取外供入口至首个真实合流之前的独立带节点；使用：监测器按入口向内稳定选点；边界：入口缺失、断链、跨物品共带及路线尚未落位均明确失败。
func boundaryFlowMonitorHosts(ctx context.Context, route *transportRoute, present map[*placed]bool, incoming map[*placed]int, items map[*placed]map[int]bool) ([]*placed, error) {
	if route.from != nil || route.boundary == nil || route.start != route.boundary || len(route.path) == 0 || route.path[0] != route.boundary || !present[route.boundary] || !surfaceIsBelt(route.boundary.record.ItemID) {
		return nil, boundaryFlowMonitorError(route, "外供原料入口尚未形成真实带链，请在全图路由和裁剪后布设监测器")
	}
	if len(route.boundary.record.Parameters) == 0 || int(route.boundary.record.Parameters[0]) != route.edge.ItemID {
		return nil, boundaryFlowMonitorError(route, "原料入口物品标记与规范 lane 不一致")
	}
	hosts := make([]*placed, 0, len(route.path))
	seen := map[*placed]bool{}
	for index, host := range route.path {
		if index%64 == 0 && ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "原料入口独立带段检查已取消或超时"}
		}
		if host == nil || !present[host] || !surfaceIsBelt(host.record.ItemID) || seen[host] {
			return nil, boundaryFlowMonitorError(route, "原料 lane 包含失效、重复或非带节点")
		}
		if index > 0 && route.path[index-1].output != host {
			return nil, boundaryFlowMonitorError(route, "原料 lane 的实际传送带输出不连续")
		}
		if len(items[host]) != 1 || !items[host][route.edge.ItemID] {
			return nil, boundaryFlowMonitorError(route, "原料 lane 与其它物品共用传送带，拒绝安装误导性监测器")
		}
		if incoming[host] > 1 {
			break
		}
		seen[host] = true
		hosts = append(hosts, host)
	}
	if len(hosts) == 0 {
		return nil, boundaryFlowMonitorError(route, "原料入口没有合流前的独立监测带段")
	}
	return hosts, nil
}

// boundaryFlowMonitorAt 职责：识别独立带节点已有的同点同向无连接监测器；使用：重复调用及本地监测器复用；边界：不把错向或带拓扑连接的 2030 当作合格监测器。
func (b *builder) boundaryFlowMonitorAt(host *placed) *placed {
	for _, entity := range b.all {
		if boundaryFlowMonitorMatches(host, entity) {
			return entity
		}
	}
	return nil
}

// boundaryFlowMonitorMatches 职责：核对监测器与真实宿主的空间和方向一致性；使用：索引复用及回归查询；边界：不接受错向、跨 Area 或附带带链连接的监测实体。
func boundaryFlowMonitorMatches(host, entity *placed) bool {
	return entity.record.ItemID == 2030 && entity.record.AreaIndex == host.record.AreaIndex && entity.a == host.a && entity.b == host.a && math.Abs(math.Remainder(float64(entity.record.Yaw-host.record.Yaw), 360)) < 1e-3 && math.Abs(math.Remainder(float64(entity.record.Yaw2-host.record.Yaw), 360)) < 1e-3 && entity.input == nil && entity.output == nil && entity.record.InputObjIdx < 0 && entity.record.OutputObjIdx < 0
}

// validateBoundaryFlowMonitorLayout 职责：对监测器和补电后的全局实体重新执行静态、动态及供电规则；使用：入口监测收尾；边界：切平面预检不能替代 finish 的实际球面和游戏放置审计。
func (b *builder) validateBoundaryFlowMonitorLayout(ctx context.Context) error {
	records, _, _, err := b.records()
	if err != nil {
		return err
	}
	encoding := newPlanarRuleEncoding()
	if err := encoding.validateStaticColliders(ctx, records); err != nil {
		return err
	}
	if err := encoding.validateDynamicFacilities(ctx, records); err != nil {
		return err
	}
	if b.setting.NeedPower == nil || *b.setting.NeedPower {
		_, err = validateFinalPowerCoverage(ctx, encoding, records)
	}
	return err
}

// boundaryFlowMonitorError 职责：提供可定位原料物品、边和流量的布设失败；使用：入口监测硬失败；边界：不返回静默缺失或部分成功蓝图。
func boundaryFlowMonitorError(route *transportRoute, reason string) error {
	if route == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "原料入口流速监测器布设失败：" + reason}
	}
	return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: fmt.Sprintf("原料入口流速监测器布设失败（物品 %d、边 %s、%.3f/min）：%s", route.edge.ItemID, route.edge.ID, route.rate, reason)}
}
