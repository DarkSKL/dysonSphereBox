package build

// 文件职责：把独立核对的分流器槽位射线接入寻路端口例外。
// 使用：已有真实短接头与新增portal边相接时；边界：不能仅因三跳内有分流器就豁免任意方向的穿壳带段。

import "DysonSphereBox/helper/blueprint/build/placement"

// splitterRoutePosition 职责：取得索引坐标系中的实体原点；使用：与真实槽口比较；边界：不包含带碰撞球的额外0.2米抬高。
func (index *denseRoutePortIndex) splitterRoutePosition(point vector) placement.Vector {
	if index.polarFrame != nil {
		return index.polarFrame.world(point, 0)
	}
	return placement.Vector{point.x * planarGridLength, point.z*1.3333333 + .2, point.y * planarGridLength}
}

// splitterRoutePort 职责：在当前平面或原生球面中解释指定槽位；使用：端点身份及边净空；边界：形态、朝向和层高均来自真实实体与唯一profile。
func (index *denseRoutePortIndex) splitterRoutePort(gate *placed, slot int8) (placement.Vector, placement.Vector, error) {
	encoding := newPlanarRuleEncoding()
	if index.polarFrame != nil {
		encoding = &surfaceEncoding{}
	}
	origin := index.splitterRoutePosition(gate.a)
	return encoding.splitterWorldPort(gate.record, vector{origin[0], origin[1], origin[2]}, slot)
}

// splitterLinkedEndpoint 职责：验证真实端点确由正确槽位上的至多两跳短接头接入；使用：生成portal许可前；边界：错误模型、槽位、方向或高度一律不产生例外。
func (index *denseRoutePortIndex) splitterLinkedEndpoint(endpoint, direct, gate *placed, source bool, hops int) bool {
	if hops > 2 {
		return false
	}
	slot := direct.record.OutputToSlot
	if source {
		slot = direct.record.InputFromSlot
	}
	port, forward, err := index.splitterRoutePort(gate, slot)
	return err == nil && splitterPortProbesAllowed(index.splitterRoutePosition(gate.a), port, forward, []splitterPortProbe{{index.splitterRoutePosition(direct.a), true}, {index.splitterRoutePosition(endpoint.a), true}}, index.polarFrame != nil)
}

// splitterRouteSpanAllowed 职责：把已有槽位许可限定为同一外向直线上的真实短接头；使用：每条候选portal边；边界：离开选择体积的长段可继续向外，但不允许折返到本体或绕到其它槽。
func (index *denseRoutePortIndex) splitterRouteSpanAllowed(gate *placed, first, second vector, allowed [2]bool) bool {
	if !allowed[0] && !allowed[1] {
		return false
	}
	for slot := int8(0); slot < 4; slot++ {
		port, forward, err := index.splitterRoutePort(gate, slot)
		if err != nil {
			return false
		}
		a, c := index.splitterRoutePosition(first), index.splitterRoutePosition(second)
		for endpoint, permitted := range allowed {
			if !permitted {
				continue
			}
			probes := []splitterPortProbe{{a, endpoint == 0}, {c, endpoint == 1}}
			if splitterPortProbesAllowed(index.splitterRoutePosition(gate.a), port, forward, probes, index.polarFrame != nil) {
				return true
			}
		}
	}
	return false
}
