package build

// 文件职责：在配送输入带反向前保留监测器宿主身份，并在真实接驳完成后同步方向。
// 使用：flyBlocks 入口及成功出口；边界：不改带链、不推断其它物料宿主，不豁免最终碰撞和附加通道校验。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// flyFlowMonitorBinding 职责：保留本地监测器与原始合法宿主的实体引用；使用：配送接驳反向跨阶段；边界：引用只能来自当前独立机器块，不借用尚未一起移动的其它块。
type flyFlowMonitorBinding struct {
	monitor *placed
	host    *placed
}

// captureFlyFlowMonitorHosts 职责：在配送接驳重写机器输入带方向前验证并记录宿主；使用：flyBlocks 开始处；边界：同坐标多个带、错向或无宿主的监测器明确失败，不通过事后任意重绑修复非法布局。
func (b *builder) captureFlyFlowMonitorHosts(begin int) ([]flyFlowMonitorBinding, error) {
	if !b.setting.EnableFlowMonitor {
		return nil, nil
	}
	if begin < 0 || begin > len(b.all) {
		return nil, &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "配送监测器的独立机器块范围无效"}
	}
	hosts := map[vector][]*placed{}
	for _, entity := range b.all[begin:] {
		if err := b.checkGeneration("配送监测器宿主登记"); err != nil {
			return nil, err
		}
		if surfaceIsBelt(entity.record.ItemID) {
			hosts[entity.a] = append(hosts[entity.a], entity)
		}
	}
	bindings := []flyFlowMonitorBinding{}
	for _, entity := range b.all[begin:] {
		if entity.record.ItemID != 2030 {
			continue
		}
		if err := b.checkGeneration("配送监测器初始宿主核对"); err != nil {
			return nil, err
		}
		var selected *placed
		for _, host := range hosts[entity.a] {
			if entity.record.AreaIndex != host.record.AreaIndex || entity.b != host.a || math.Abs(math.Remainder(float64(entity.record.Yaw-host.record.Yaw), 360)) >= 1e-3 || entity.input != nil || entity.output != nil || entity.record.InputObjIdx >= 0 || entity.record.OutputObjIdx >= 0 {
				continue
			}
			if selected != nil {
				return nil, &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "配送监测器对应多个同点宿主带，无法证明独立物料身份"}
			}
			selected = host
		}
		if selected == nil {
			return nil, &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: fmt.Sprintf("配送监测器(%.2f,%.2f,%.2f)在接驳前没有合法同点同向宿主带", entity.a.x, entity.a.y, entity.a.z)}
		}
		bindings = append(bindings, flyFlowMonitorBinding{monitor: entity, host: selected})
	}
	return bindings, nil
}

// syncFlyFlowMonitorHosts 职责：按已捕获实体身份同步反向后的真实宿主朝向；使用：配送接驳成功出口；边界：只改监测器 yaw，不改任何带坐标、连接、参数或原有严格碰撞规则。
func (b *builder) syncFlyFlowMonitorHosts(bindings []flyFlowMonitorBinding, begin int) error {
	if len(bindings) == 0 {
		return nil
	}
	present := map[*placed]bool{}
	for _, entity := range b.all[begin:] {
		if err := b.checkGeneration("配送监测器终态宿主登记"); err != nil {
			return err
		}
		present[entity] = true
	}
	for _, binding := range bindings {
		if err := b.checkGeneration("配送监测器终态方向同步"); err != nil {
			return err
		}
		monitor, host := binding.monitor, binding.host
		if !present[monitor] || !present[host] || !surfaceIsBelt(host.record.ItemID) || monitor.record.AreaIndex != host.record.AreaIndex || monitor.a != host.a || monitor.b != host.a || monitor.input != nil || monitor.output != nil || monitor.record.InputObjIdx >= 0 || monitor.record.OutputObjIdx >= 0 {
			return &blueprint.Error{Code: "GEN_E_FLOW_MONITOR", Message: "配送接驳破坏了原流速监测器的真实宿主身份、共点位置或无带链连接约束"}
		}
		monitor.record.Yaw, monitor.record.Yaw2 = host.record.Yaw, host.record.Yaw
	}
	return nil
}
