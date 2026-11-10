package build

// 文件职责：把容量切片外供装配为真实分流器树与消费者独立合流缓冲。
// 使用：超密度直连带消费网络装配及规范份额展开；边界：按配置毛需求分桶，不升级设备，不伪造普通带分叉，多个来源只在无机器臂的缓冲区合流。

import (
	"DysonSphereBox/helper/quant"
	"fmt"
)

// denseInputSplitNetwork 职责：保存一种原料的容量切片和全部真实支路；使用：刚性排布及终检；边界：每个切片只有一个外供口，消费者可由多个切片共同供给。
type denseInputSplitNetwork struct {
	gates     []*placed
	branches  []*denseInputSplitBranch
	slices    []*denseInputSlice
	bySource  map[*transportRoute]*denseInputSlice
	consumers []*denseInputConsumer
}

// denseInputSlice 职责：保存不超过所选单带毛容量的一个实际入口；使用：分流树装配及配置负载核验；边界：source仍是净流记录，configured另外保留毛需求。
type denseInputSlice struct {
	source     *transportRoute
	configured float64
	branches   []*denseInputSplitBranch
	head       *placed
}

// denseInputConsumer 职责：保存消费者跨入口取料的独立合流缓冲；使用：避免机器cargo邻域直接多路合流；边界：单一来源沿用原入口，不补造无用设施。
type denseInputConsumer struct {
	head   *placed
	buffer []*placed
	route  *transportRoute
}

// denseInputSplitBranch 职责：记录一个容量切片对指定消费者的真实贡献；使用：净流展开及配置毛负荷守恒；边界：每条规范碎片绑定原始边，不把同消费者的多个来源当成独立需求。
type denseInputSplitBranch struct {
	target              int
	planned, configured float64
	source              *transportRoute
	route               *transportRoute
	prefix              []*placed
	contributions       map[*transportRoute]*transportRoute
}

// prepareDenseInputSplitters 职责：给多源消费者创建独立合流区，给多去向入口创建Model38分流树；使用：机器轨道固定后、全场排布前；边界：所有设施及端口进入真实建造盒排布，单去向入口保留普通单向带。
func (b *builder) prepareDenseInputSplitters(group *denseMaterialNetwork, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	owner := b.owner
	b.owner = -1
	defer func() { b.owner = owner }()
	network := group.splitInput
	counts := make([]int, len(group.rows))
	for _, branch := range network.branches {
		counts[branch.target]++
	}
	for target, row := range group.rows {
		consumer := &denseInputConsumer{head: group.entries[target]}
		if counts[target] > 1 {
			begin := len(b.all)
			consumer.buffer = b.byPath([]vector{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 0}})
			consumer.head = consumer.buffer[0]
			proxy := &transportRow{owner: -1, inputs: map[int]*placed{group.item: consumer.head}}
			consumer.route = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("external-merge:%d:%d", group.id, target), ItemID: group.item, FlowKind: "internal"}, rate: group.planned[target], from: proxy, to: row, start: consumer.buffer[3], end: group.entries[target], path: []*placed{consumer.buffer[3]}}
			b.denseMaterialNetworks.joins = append(b.denseMaterialNetworks.joins, consumer.route)
			group.intake = append(group.intake, consumer.buffer...)
			units = append(units, denseSpatialUnit{entities: b.all[begin:], ports: []denseStreetPort{{item: group.item, input: true, head: consumer.head, inner: consumer.buffer[1]}, {item: group.item, head: consumer.buffer[3], inner: consumer.buffer[2]}}})
		}
		network.consumers = append(network.consumers, consumer)
	}
	for _, slice := range network.slices {
		if len(slice.branches) == 1 {
			branch := slice.branches[0]
			consumer := network.consumers[branch.target]
			slice.head = consumer.head
			slice.source.to = &transportRow{owner: -1, inputs: map[int]*placed{group.item: consumer.head}}
			branch.route = slice.source
			continue
		}
		begin := len(b.all)
		intake := b.byPath([]vector{{-3, 0, 0}, {-2, 0, 0}, {-1, 0, 0}})
		slice.head = intake[0]
		group.intake = append(group.intake, intake...)
		proxy := &transportRow{owner: -1, inputs: map[int]*placed{group.item: slice.head}}
		slice.source.to = proxy
		ports := []denseStreetPort{{item: group.item, input: true, head: intake[0], inner: intake[1]}}
		prefix := append([]*placed{}, intake...)
		previous := intake[len(intake)-1]
		for index := 0; index < len(slice.branches)-1; index++ {
			x := float64(index * 4)
			gate := b.make(2020, x, 0, 0)
			gate.record.ModelIndex = 38
			gate.record.Parameters = make([]int32, 6)
			previous.output, previous.record.OutputToSlot = gate, 2
			network.gates = append(network.gates, gate)
			prefix = append(prefix, gate)
			appendBranch := func(part int, side float64, slot int8) {
				branch := slice.branches[part]
				track := b.byPath([]vector{{x, side, 0}, {x, side * 2, 0}, {x, side * 3, 0}})
				track[0].input, track[0].record.InputFromSlot = gate, slot
				branch.prefix = append([]*placed{}, prefix...)
				branch.route = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("external-split:%d:%d:%d", group.id, len(units), part), ItemID: group.item, FlowKind: "internal"}, rate: branch.planned, from: proxy, to: group.rows[branch.target], start: track[len(track)-1], end: network.consumers[branch.target].head, path: track}
				b.denseMaterialNetworks.joins = append(b.denseMaterialNetworks.joins, branch.route)
				ports = append(ports, denseStreetPort{item: group.item, head: branch.route.start, inner: track[len(track)-2]})
			}
			appendBranch(index, 1, 3)
			if index == len(slice.branches)-2 {
				appendBranch(index+1, -1, 1)
				break
			}
			continuation := b.byPath([]vector{{x + 1, 0, 0}, {x + 2, 0, 0}, {x + 3, 0, 0}})
			continuation[0].input, continuation[0].record.InputFromSlot = gate, 0
			prefix = append(prefix, continuation...)
			group.intake = append(group.intake, continuation...)
			previous = continuation[len(continuation)-1]
		}
		units = append(units, denseSpatialUnit{entities: b.all[begin:], ports: ports})
	}
	return units, nil
}

// denseInputSplitConsumerPath 职责：展开某入口份额经实际合流缓冲到消费者的连续带；使用：规范提交及最终复核；边界：不穿分流器，也不按坐标补链。
func denseInputSplitConsumerPath(group *denseMaterialNetwork, branch *denseInputSplitBranch) []*placed {
	path := append([]*placed{}, branch.route.path...)
	consumer := group.splitInput.consumers[branch.target]
	if len(consumer.buffer) > 0 {
		path = append(path, consumer.buffer[1:]...)
		path = append(path, consumer.route.path[1:]...)
	}
	return path
}

// denseInputSplitSharedPath 职责：逐份额计入实际分流前缀及本消费者机器输入轨；使用：规范带负载汇总；边界：单去向入口已在route.path计量，合流缓冲也只计一次。
func denseInputSplitSharedPath(source *transportRoute, group *denseMaterialNetwork, branchIndex int) []*placed {
	branch := group.splitInput.branches[branchIndex]
	path := []*placed{}
	if len(branch.prefix) > 0 {
		path = append(path, source.path...)
		path = append(path, branch.prefix[1:]...)
	}
	return append(path, group.tracks[branch.target][1:]...)
}
