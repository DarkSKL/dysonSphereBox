package build

// 文件职责：在联产与其他供给同时进入同一原料口时建立真实优先合流。
// 使用：全场机器装配与最终物流证明；边界：联产先用、外供补缺，不用期望流量假装外部带能自动限速。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// byproductPriorityInput 职责：登记单个消费口的优先门和真实出入链；使用：合流装配及终检；边界：同门只接同一物品。
type byproductPriorityInput struct {
	// row 职责：关联实际消费者；使用：端口与份额复核；边界：不改变制造归属。
	row *transportRow
	// item 职责：锁定合流物料；使用：输入优先参数；边界：不跨物品混流。
	item int
	// routes 职责：保留原规范份额；使用：区分联产与补充来源；边界：不能改写边身份或流量。
	routes []*transportRoute
	// gate 职责：保存真实优先门；使用：参数与端口校验；边界：不以虚拟节点代替实体。
	gate *placed
	// local, fresh, outlet, entities 职责：记录分开的输入、共有出带及附件实体；使用：落位与连通证明；边界：所有链均须存活且有向。
	local, fresh, outlet, entities []*placed
	// feed 职责：保存门到消费者的辅助实线；使用：公共寻路；边界：不另计规范需求。
	feed *transportRoute
}

// coupledProductionSource 职责：识别受其他产物牵制的联产源；使用：优先入口分配；边界：所有多产物来源同等优先，不按配方名称猜测。
func coupledProductionSource(route *transportRoute) bool {
	return route.from != nil && len(route.from.recipe.Results) > 1
}

// needsByproductPriority 职责：识别规范图中同消费者的联产与补充来源；使用：排除无法证明输入优先的旧塔场候选；边界：读取重建图而非请求自带边。
func needsByproductPriority(plan quant.Result, graph *quant.FlowGraph, dataset *quant.Dataset) bool {
	if graph == nil {
		return false
	}
	nodes := map[string]quant.FlowNode{}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	type key struct {
		node string
		item int
	}
	roles := map[key]int{}
	for _, edge := range graph.Edges {
		if nodes[edge.TargetNodeID].Kind != "production" {
			continue
		}
		source := nodes[edge.SourceNodeID]
		role := 2
		if source.Kind == "production" {
			recipe := dataset.Recipes[dataset.RecipeByID[plan.Productions[source.ProductionIndex].RecipeID]]
			if len(recipe.Results) > 1 {
				role = 1
			}
		}
		k := key{edge.TargetNodeID, edge.ItemID}
		roles[k] |= role
		if roles[k] == 3 {
			return true
		}
	}
	return false
}

// byproductPriorityGroups 职责：找出同消费口中联产与补料并存的分组；使用：提前关闭会混合来源的共享轨优化；边界：纯联产或纯外供不新增设施。
func byproductPriorityGroups(routes []*transportRoute) []*byproductPriorityInput {
	type key struct {
		row  *transportRow
		item int
	}
	byKey := map[key]*byproductPriorityInput{}
	ordered := []*byproductPriorityInput{}
	for _, route := range routes {
		if route.to == nil || route.inputRole != recipeInputRole {
			continue
		}
		k := key{route.to, route.edge.ItemID}
		group := byKey[k]
		if group == nil {
			group = &byproductPriorityInput{row: route.to, item: route.edge.ItemID}
			byKey[k] = group
			ordered = append(ordered, group)
		}
		group.routes = append(group.routes, route)
	}
	out := []*byproductPriorityInput{}
	for _, group := range ordered {
		local, fresh := false, false
		for _, route := range group.routes {
			if coupledProductionSource(route) {
				local = true
			} else {
				fresh = true
			}
		}
		if local && fresh {
			out = append(out, group)
		}
	}
	return out
}

// prepareByproductPriorityInputs 职责：创建独立合流附件和接机辅助线；使用：各机器输入固定之后；边界：原消费者及规范物料边保持不变。
func (b *builder) prepareByproductPriorityInputs(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	for index, group := range byproductPriorityGroups(b.transportRoutes) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		begin := len(b.all)
		b.owner = group.row.owner
		group.gate = b.recycleGate(0, 0, 3, group.item)
		var err error
		group.local, err = b.buildRecyclePort(group.gate, 3, true)
		if err != nil {
			return nil, err
		}
		group.fresh, err = b.buildRecyclePort(group.gate, 2, true)
		if err != nil {
			return nil, err
		}
		group.outlet, err = b.buildRecyclePort(group.gate, 0, false)
		if err != nil {
			return nil, err
		}
		total := 0.
		for _, route := range group.routes {
			route.priorityInput = group.fresh[0]
			if coupledProductionSource(route) {
				route.priorityInput = group.local[0]
			}
			route.end = route.priorityInput
			total += route.rate
		}
		end := group.row.inputEndpoint(group.item, recipeInputRole)
		start := group.outlet[len(group.outlet)-1]
		group.feed = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("byproduct-priority:%d", index), ItemID: group.item, FlowKind: "internal"}, rate: total, from: &transportRow{owner: -1}, to: group.row, start: start, end: end, path: []*placed{start}}
		group.entities = append([]*placed(nil), b.all[begin:]...)
		units = append(units, denseSpatialUnit{entities: group.entities, ports: []denseStreetPort{
			{item: group.item, input: true, head: group.local[0], inner: group.local[1]},
			{item: group.item, input: true, head: group.fresh[0], inner: group.fresh[1]},
			{item: group.item, head: start, inner: group.outlet[len(group.outlet)-2]},
		}})
		b.byproductInputs = append(b.byproductInputs, group)
	}
	return units, nil
}

// byproductPriorityRoutes 职责：提供合流门到机器的真实辅助线路；使用：公共空间评分与路由；边界：不重复计入规范需求。
func (b *builder) byproductPriorityRoutes() []*transportRoute {
	out := make([]*transportRoute, 0, len(b.byproductInputs))
	for _, group := range b.byproductInputs {
		out = append(out, group.feed)
	}
	return out
}

// commitByproductPriorityInputs 职责：登记优先合流后的共有带容量；使用：路由完成后；边界：同规范份额只计算一次共享实体。
func (b *builder) commitByproductPriorityInputs() error {
	b.bindByproductPriorityInputs()
	return b.validateByproductPriorityInputs()
}

// bindByproductPriorityInputs 职责：把真实合流链追加到共享路径；使用：独立优先关系提交节点；边界：只绑定，不签发最终优先权检查结果。
func (b *builder) bindByproductPriorityInputs() {
	for _, group := range b.byproductInputs {
		for _, route := range group.routes {
			intake := group.fresh
			if coupledProductionSource(route) {
				intake = group.local
			}
			seen := map[*placed]bool{}
			for _, node := range route.path {
				seen[node] = true
			}
			for _, node := range route.sharedPath {
				seen[node] = true
			}
			for _, path := range [][]*placed{intake, {group.gate}, group.outlet, group.feed.path} {
				for _, node := range path {
					if !seen[node] {
						route.sharedPath = append(route.sharedPath, node)
						seen[node] = true
					}
				}
			}
		}
	}
}

// validateByproductPriorityInputs 职责：从实际参数与带链证明联产具有输入优先权；使用：物流终检；边界：不允许补料绕过门、断接机链或修改规范份额。
func (b *builder) validateByproductPriorityInputs() error {
	expected := byproductPriorityGroups(b.transportRoutes)
	fail := func(message string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "副产物优先合流：" + message}
	}
	if len(expected) != len(b.byproductInputs) {
		return fail("混合来源的消费口缺少真实优先设施")
	}
	indices := map[*placed]int{}
	for i, node := range b.all {
		indices[node] = i
	}
	for _, group := range b.byproductInputs {
		p := group.gate.record.Parameters
		if _, ok := indices[group.gate]; !ok || len(p) != 6 || p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 1 || p[4] != 8 || p[5] != int32(group.item) || group.gate.record.FilterID != int16(group.item) {
			return fail("优先参数与物料不匹配")
		}
		if !recycleInputAttached(group.local, group.local[0], group.gate, 3) || !recycleInputAttached(group.fresh, group.fresh[0], group.gate, 2) || group.outlet[0].input != group.gate || group.outlet[0].record.InputFromSlot != 0 {
			return fail("实际端口方向不匹配")
		}
		for _, path := range [][]*placed{group.local, group.fresh, group.outlet, group.feed.path} {
			if err := validateBusFabricChain(path, indices); err != nil {
				return err
			}
		}
		if group.feed.path[0] != group.outlet[len(group.outlet)-1] || group.feed.path[len(group.feed.path)-1] != group.row.inputEndpoint(group.item, recipeInputRole) {
			return fail("合流门未实际接入消费者")
		}
		total := 0.
		for _, route := range group.routes {
			want := group.fresh[0]
			if coupledProductionSource(route) {
				want = group.local[0]
			}
			if route.to != group.row || route.end != want || route.priorityInput != want || len(route.path) == 0 || route.path[len(route.path)-1] != want {
				return fail("联产或补料绕过其指定入口")
			}
			total += route.rate
		}
		if math.Abs(total-group.feed.rate) > 1e-6 || total > b.logistics.BeltCapacityPerMinute+1e-6 {
			return fail("合流后的真实总负载不守恒或超带宽")
		}
	}
	if len(b.byproductInputs) > 0 {
		for _, check := range b.logistics.Checks {
			if check.RuleID == "byproduct-input-priority" {
				return nil
			}
		}
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "byproduct-input-priority", Status: "passed", Message: fmt.Sprintf("%d个消费口使用真实优先合流门，先消耗厂内联产物，外部或独立生产只补缺；剩余副产物出口仍须持续畅通", len(b.byproductInputs))})
	}
	return nil
}
