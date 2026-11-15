package build

// 文件职责：从最终实体重算容量切片、分流/合流连接及每条规范边的贡献守恒。
// 使用：共同物流终检；边界：配置毛负载和规范净流分别验证，拒绝虚拟分叉、未登记汇入和错误端口，建造盒、货物采样及供电仍走公共终检。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
	"fmt"
	"math"
)

// validateDenseInputSplitters 职责：独立验证每个容量入口、跨入口消费者和原始边贡献；使用：最终规范账重算；边界：对每个真实带段累计配置毛负载，不以较低净流掩盖超载。
func (b *builder) validateDenseInputSplitters(group *denseMaterialNetwork, alive map[*placed]bool) (float64, error) {
	fail := func(reason string) (float64, error) {
		return 0, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享原料容量切片不完整：" + reason}
	}
	network, plan := group.splitInput, b.denseMaterialNetworks
	if b.adapter == nil || b.adapter.mode != 1 || len(group.rows) < 1 || len(network.slices) == 0 || len(network.consumers) != len(group.rows) || len(group.tracks) != len(group.rows) {
		return fail("入口或消费者登记不完整")
	}
	allowed, gates := map[*placed]bool{}, map[*placed]bool{}
	grossLoads := map[*placed]float64{}
	graph := finalMachineMaterialGraph(b.all)
	configured := make([]float64, len(group.rows))
	planned := make([]float64, len(group.rows))
	parts := make([]int, len(group.rows))
	originalTotals := map[*transportRoute]float64{}
	originalSeen := map[*transportRoute]bool{}
	fragments := map[*transportRoute]bool{}
	maxConfigured, totalFull, totalSlices := 0., 0., 0.
	for _, gate := range network.gates {
		if !alive[gate] || gates[gate] || gate.splitterGate == nil {
			return fail("缺少唯一真实共同门保护身份")
		}
		if err := b.validateSplitterGateAssembly(gate.splitterGate, alive); err != nil {
			return 0, err
		}
		gates[gate] = true
		allowed[gate] = true
	}
	expectedGates := 0
	for _, slice := range network.slices {
		source := slice.source
		if source == nil || source.from != nil || source.boundary != source.start || source.edge.FlowKind != "external-input" || source.edge.ItemID != group.item || len(source.path) == 0 || source.path[0] != source.start || source.path[len(source.path)-1] != slice.head || network.bySource[source] != slice {
			return fail("容量切片缺少唯一真实外供头")
		}
		sourcePath, err := denseMaterialDirectedPath(source.start, slice.head, alive)
		if err != nil || !sameDenseMaterialNodes(sourcePath, source.path) {
			return fail("外供带没有到达切片分流头")
		}
		if len(slice.branches) < 1 {
			return fail("容量入口没有消费者")
		}
		expectedGates += len(slice.branches) - 1
		full, net, memberCount := 0., 0., 0
		for _, branch := range slice.branches {
			if branch == nil || branch.source != source || branch.target < 0 || branch.target >= len(group.rows) || branch.route == nil || !finitePositive(branch.configured) || !finitePositive(branch.planned) {
				return fail("容量份额缺少有效消费者或流量")
			}
			target := branch.target
			consumer := network.consumers[target]
			if consumer == nil || consumer.head == nil || len(branch.route.path) == 0 || branch.route.path[len(branch.route.path)-1] != consumer.head {
				return fail("分流支路没有到达消费者合流头")
			}
			physical, err := denseMaterialDirectedPath(branch.route.path[0], consumer.head, alive)
			if err != nil || !sameDenseMaterialNodes(physical, branch.route.path) {
				return fail("实际支路带链中断")
			}
			if len(slice.branches) == 1 {
				if branch.route != source || len(branch.prefix) != 0 {
					return fail("单去向入口没有使用同一条真实供给带")
				}
			} else {
				if len(branch.prefix) < 4 || branch.prefix[0] != slice.head || !gates[branch.prefix[len(branch.prefix)-1]] {
					return fail("多去向入口没有真实四向分流前缀")
				}
				chain := append(append([]*placed{}, branch.prefix...), branch.route.path...)
				for i, node := range chain {
					if !alive[node] {
						return fail("分流前缀引用蓝图外实体")
					}
					if i == 0 {
						continue
					}
					previous := chain[i-1]
					if gates[previous] {
						if node.input != previous || !surfaceIsBelt(node.record.ItemID) {
							return fail("分流出口不是实际带连接")
						}
					} else if previous.output != node {
						return fail("分流前缀实际有向带中断")
					}
				}
			}
			if len(consumer.buffer) > 0 {
				if len(consumer.buffer) != 4 || consumer.head != consumer.buffer[0] || consumer.route == nil || len(consumer.route.path) == 0 || consumer.route.path[0] != consumer.buffer[3] || consumer.route.path[len(consumer.route.path)-1] != group.entries[target] {
					return fail("跨入口消费者没有独立合流缓冲及下游接线")
				}
			} else if consumer.head != group.entries[target] {
				return fail("单来源消费者入口改变")
			}
			path := denseInputSplitConsumerPath(group, branch)
			consumerPath, err := denseMaterialDirectedPath(path[0], group.entries[target], alive)
			if err != nil || !sameDenseMaterialNodes(path, consumerPath) || !finalMaterialReaches(graph, source.start, group.entries[target], group.item) {
				return fail("真实入口不能通过分流和合流到达指定消费者")
			}
			full += branch.configured
			net += branch.planned
			parts[target]++
			configured[target] += branch.configured
			planned[target] += branch.planned
			branchNet := 0.
			branchIndex := -1
			for fragment, original := range branch.contributions {
				binding, ok := plan.bindings[fragment]
				if !ok || original == nil || fragments[fragment] || binding.group != group || binding.source != source || binding.target != target || binding.splitBranch < 0 || binding.splitBranch >= len(network.branches) || network.branches[binding.splitBranch] != branch || fragment.from != nil || fragment.to != group.rows[target] || fragment.edge.ID != original.edge.ID || fragment.edge.ItemID != group.item || fragment.edge.FlowKind != "external-input" || fragment.inputRole != recipeInputRole || original.to != fragment.to || original.inputRole != fragment.inputRole {
					return fail("规范碎片失去原始边或实际容量份额")
				}
				expected := original.rate * branch.configured / group.configured[target]
				if math.Abs(fragment.rate-expected) > math.Max(1e-6, expected*1e-9) || fragment.boundary != source.boundary || fragment.start != branch.route.start || fragment.end != group.entries[target] || len(fragment.filters) != 0 || !sameDenseMaterialNodes(fragment.path, path) || !sameDenseMaterialNodes(fragment.sharedPath, denseInputSplitSharedPath(source, group, binding.splitBranch)) {
					return fail("规范碎片比例或真实完整路径不匹配")
				}
				fragments[fragment] = true
				originalSeen[original] = true
				originalTotals[original] += fragment.rate
				branchNet += fragment.rate
				memberCount++
				branchIndex = binding.splitBranch
			}
			if branchIndex < 0 || math.Abs(branchNet-branch.planned) > math.Max(1e-6, branch.planned*1e-9) {
				return fail("支路净流与规范贡献不守恒")
			}
			all := append(append([]*placed{}, path...), denseInputSplitSharedPath(source, group, branchIndex)...)
			seen := map[*placed]bool{}
			for _, node := range all {
				if !alive[node] || seen[node] {
					return fail("份额路径引用无效或重复节点")
				}
				seen[node] = true
				allowed[node] = true
				if surfaceIsBelt(node.record.ItemID) {
					grossLoads[node] += branch.configured
				}
			}
		}
		if math.Abs(full-slice.configured) > math.Max(1e-6, full*1e-9) || math.Abs(net-source.rate) > math.Max(1e-6, net*1e-9) || len(plan.members[source]) != memberCount || full > b.logistics.BeltCapacityPerMinute+1e-6 {
			return fail("容量入口毛负载或净流不守恒、超带宽")
		}
		maxConfigured = math.Max(maxConfigured, full)
		totalSlices += full
		for _, member := range plan.members[source] {
			if !fragments[member] {
				return fail("入口规范份额登记与实际贡献不符")
			}
		}
	}
	if expectedGates != len(gates) {
		return fail("实际分流器数量与分流树不匹配")
	}
	for target, row := range group.rows {
		full, err := configuredRoleInputRate(row.info, row.recipe, group.item, recipeInputRole)
		if err != nil {
			return 0, err
		}
		full *= float64(row.count) / float64(row.info.MachineCount)
		totalFull += full
		if !finitePositive(full) || math.Abs(full-group.configured[target]) > math.Max(1e-6, full*1e-9) || math.Abs(configured[target]-full) > math.Max(1e-6, full*1e-9) || math.Abs(planned[target]-group.planned[target]) > math.Max(1e-6, planned[target]*1e-9) {
			return fail("跨入口消费者的毛需求或净份额不守恒")
		}
		track, err := denseMaterialDirectedPath(group.entries[target], group.exits[target], alive)
		if err != nil || !sameDenseMaterialNodes(track, group.tracks[target]) {
			return fail("消费者原料轨发生改变")
		}
		if (parts[target] > 1) != (len(network.consumers[target].buffer) > 0) {
			return fail("跨入口消费者与实际合流缓冲不匹配")
		}
	}
	if math.Abs(totalFull-totalSlices) > math.Max(1e-6, totalFull*1e-9) || len(network.slices) != int(math.Ceil((totalFull-1e-8)/b.logistics.BeltCapacityPerMinute)) {
		return fail("入口切片总毛需求或最少容量口数不匹配")
	}
	for original := range originalSeen {
		if math.Abs(originalTotals[original]-original.rate) > math.Max(1e-6, original.rate*1e-9) {
			return fail("原始规范边拆份后贡献不守恒")
		}
	}
	for node, load := range grossLoads {
		capacity, err := transport.BeltCapacityPerMinute(int(node.record.ItemID) - 2000)
		if err != nil {
			return fail("实际带段缺少已核对的原型容量")
		}
		if load > capacity+1e-6 {
			return 0, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("外供分流或合流带段物品%d的配置毛负载%.6f/min超过实际原型容量%.6f/min", node.record.ItemID, load, capacity)}
		}
		if int(node.record.ItemID) != 2000+b.setting.BeltLv || node.record.ModelIndex != node.record.ItemID-1966 {
			return fail("实际带段原型或模型与用户所选带等级不一致")
		}
		if load > b.logistics.BeltCapacityPerMinute+1e-6 {
			return 0, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "外供分流或合流的实际带段配置毛负载超过所选带容量"}
		}
	}
	for gate := range gates {
		ports := map[int8]bool{}
		incoming, outgoing := 0, 0
		for _, node := range b.all {
			if node.output == gate || node.input == gate {
				if !allowed[node] || !surfaceIsBelt(node.record.ItemID) {
					return fail("分流器存在未登记连接")
				}
				slot := node.record.InputFromSlot
				if node.output == gate {
					slot = node.record.OutputToSlot
					incoming++
				} else {
					outgoing++
				}
				if slot < 0 || slot > 3 || ports[slot] || !b.denseInputSplitPortMatches(node, gate, slot) {
					return fail("分流器槽位重复或实际几何不匹配")
				}
				ports[slot] = true
			}
		}
		if incoming != 1 || outgoing != 2 {
			return fail("分流器不是一进两出的实际拓扑")
		}
	}
	for _, node := range b.all {
		if (surfaceIsBelt(node.record.ItemID) || node.record.ItemID >= 2011 && node.record.ItemID <= 2014) && allowed[node.output] && !allowed[node] {
			return fail("共享原料带被未登记带或分拣器汇入")
		}
	}
	return maxConfigured, nil
}

// denseInputSplitPortMatches 职责：按共同门的模型端口角色及保护身份核对直接接头；使用：分流树终检；边界：位置、高度和原生比例已由共同profile重算，不按最近方向猜槽。
func (b *builder) denseInputSplitPortMatches(belt, gate *placed, slot int8) bool {
	if gate == nil || gate.splitterGate == nil || slot < 0 || slot > 3 {
		return false
	}
	link := gate.splitterGate.links[slot]
	if link == nil || len(link.nodes) != 3 || belt.splitterPortLink != link {
		return false
	}
	if link.input {
		return belt == link.nodes[len(link.nodes)-1] && belt.output == gate && belt.record.OutputToSlot == slot
	}
	return belt == link.nodes[0] && belt.input == gate && belt.record.InputFromSlot == slot
}
