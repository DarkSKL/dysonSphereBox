package build

// 文件职责：在消费者附近构造单物料共享主干和真实过滤取料，保持各行既有喷涂及供剂计量。
// 使用：无法直接串过独立喷涂机的共享组；边界：普通带只前行，分支全部由所选等级分拣器实现，不合并不同物料或改变喷涂消费。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// prepareDenseMaterialTaps 职责：装配共享主干、逐消费者过滤支口及待接线尾；使用：机器行落位之前；边界：主干前四节点使多源汇流远离过滤臂采样，整组仍由实际轮廓参与排布。
func (b *builder) prepareDenseMaterialTaps(group *denseMaterialNetwork, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	begin, originalOwner := len(b.all), b.owner
	b.owner = -1
	defer func() { b.owner = originalOwner }()
	group.intake = b.byPath([]vector{{0, 0, 0}, {-1, 0, 0}, {-2, 0, 0}, {-3, 0, 0}})
	group.supplyHead = group.intake[0]
	proxy := &transportRow{owner: -1, inputs: map[int]*placed{group.item: group.supplyHead}}
	ports := []denseStreetPort{{item: group.item, input: true, head: group.intake[0], inner: group.intake[1]}}
	main := group.intake[len(group.intake)-1]
	capacity := inserterCapacityPerMinute(b.setting.InserterLv, 1)
	if !finitePositive(capacity) {
		return nil, &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: "共享原料取料缺少所选等级分拣器能力"}
	}
	appendMain := func() *placed {
		node := b.make(2000+b.setting.BeltLv, main.a.x-1, main.a.y, main.a.z)
		connect(main, node)
		main = node
		group.intake = append(group.intake, node)
		return node
	}
	for i, row := range group.rows {
		side := 1.
		if i%2 == 1 {
			side = -1
		}
		tap := &denseMaterialTap{}
		branch := []*placed{}
		count := max(1, int(math.Ceil((group.configured[i]-1e-8)/capacity)))
		for j := 0; j < count; j++ {
			pickup := appendMain()
			node := b.make(2000+b.setting.BeltLv, pickup.a.x, pickup.a.y+side, pickup.a.z)
			if len(branch) > 0 {
				connect(branch[len(branch)-1], node)
			}
			branch = append(branch, node)
			arm, err := b.beltTransfer(pickup, node, b.setting.InserterLv, group.item)
			if err != nil {
				return nil, err
			}
			tap.filters = append(tap.filters, arm)
		}
		end := b.make(2000+b.setting.BeltLv, main.a.x-1, main.a.y+side, main.a.z)
		connect(branch[len(branch)-1], end)
		branch = append(branch, end)
		tap.mainPrefix = append([]*placed{}, group.intake...)
		tap.route = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("material-tap:%d:%d", group.id, i), ItemID: group.item, FlowKind: "internal"}, rate: group.planned[i], from: proxy, to: row, start: end, end: group.entries[i], path: branch}
		group.taps = append(group.taps, tap)
		b.denseMaterialNetworks.joins = append(b.denseMaterialNetworks.joins, tap.route)
		ports = append(ports, denseStreetPort{item: group.item, head: end, inner: branch[len(branch)-2]})
		if i%2 == 1 || i == len(group.rows)-1 {
			appendMain()
		}
	}
	for _, source := range b.denseMaterialNetworks.physical {
		members := b.denseMaterialNetworks.members[source]
		if len(members) > 0 && b.denseMaterialNetworks.bindings[members[0]].group == group {
			source.to = proxy
		}
	}
	return append(units, denseSpatialUnit{entities: b.all[begin:], ports: ports}), nil
}

// denseMaterialTapSharedPath 职责：完整计入来源轨、共享主干前缀与本行消费轨；使用：规范份额展开；边界：主干头在source.path已计一次，禁止重复计算同一段。
func denseMaterialTapSharedPath(source *transportRoute, group *denseMaterialNetwork, target int) []*placed {
	shared := append(append([]*placed{}, source.sharedPath...), source.path...)
	shared = append(shared, group.taps[target].mainPrefix[1:]...)
	return append(shared, group.tracks[target][1:]...)
}
