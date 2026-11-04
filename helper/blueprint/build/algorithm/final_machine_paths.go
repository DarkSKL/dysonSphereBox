package build

// 文件职责：把最终机器物料链与实际供货/出货设施锚点对应，并证明全部喷涂机有供剂链。
// 使用：逐机能力复核之后；边界：不得经过生产机器凭加工关系补物理带路，也不以同owner任意一台机器代替指定入口。

import (
	"DysonSphereBox/helper/blueprint"
	"fmt"
	"math"
)

// finalMaterialEdge 职责：保存真实带间直连或过滤搬运边；使用：最终物料路径；边界：item为0仅表示同带直连或未过滤。
type finalMaterialEdge struct {
	// target 职责：保存实际下游节点；使用：有向追踪；边界：必须是存活实体。
	target *placed
	// item 职责：限定过滤搬运物品；使用：防止跨物品绕路；边界：不把不同配方的加工关系当带链。
	item int
}

// finalMachineMaterialGraph 职责：从最终实体重建带、地面分流门及带间过滤臂；使用：供料口到机器链的证明；边界：不穿越箱库存、生产过程或远程机队。
func finalMachineMaterialGraph(entities []*placed) map[*placed][]finalMaterialEdge {
	graph := map[*placed][]finalMaterialEdge{}
	alive := map[*placed]bool{}
	for _, entity := range entities {
		alive[entity] = true
	}
	for _, entity := range entities {
		if surfaceIsBelt(entity.record.ItemID) {
			if alive[entity.output] && (surfaceIsBelt(entity.output.record.ItemID) || entity.output.record.ItemID == 2020) {
				graph[entity] = append(graph[entity], finalMaterialEdge{target: entity.output})
			}
			if alive[entity.input] && entity.input.record.ItemID == 2020 {
				graph[entity.input] = append(graph[entity.input], finalMaterialEdge{target: entity})
			}
		}
		if entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 && alive[entity.input] && alive[entity.output] && surfaceIsBelt(entity.input.record.ItemID) && surfaceIsBelt(entity.output.record.ItemID) {
			graph[entity.input] = append(graph[entity.input], finalMaterialEdge{target: entity.output, item: int(entity.record.FilterID)})
		}
	}
	return graph
}

// finalMaterialReaches 职责：在不经过加工/库存的实际链中寻找目标；使用：最终供需锚点核验；边界：过滤器只放行对应物品。
func finalMaterialReaches(graph map[*placed][]finalMaterialEdge, from, to *placed, item int) bool {
	if from == nil || to == nil {
		return false
	}
	queue := []*placed{from}
	seen := map[*placed]bool{from: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == to {
			return true
		}
		for _, edge := range graph[current] {
			if (edge.item == 0 || edge.item == item) && !seen[edge.target] {
				seen[edge.target] = true
				queue = append(queue, edge.target)
			}
		}
	}
	return false
}

// finalMachineTrackEnds 职责：从已确认单链中读取方向；使用：实际供料入口与出货末端比对；边界：调用方先运行完整单链验证。
func finalMachineTrackEnds(row []*placed, incoming map[*placed][]*placed) (entry, exit *placed) {
	members := map[*placed]bool{}
	for _, node := range row {
		members[node] = true
	}
	for _, node := range row {
		hasPrevious := false
		for _, before := range incoming[node] {
			hasPrevious = hasPrevious || members[before]
		}
		if !hasPrevious {
			entry = node
		}
		if !members[node.output] {
			exit = node
		}
	}
	return
}

// validateFinalLineBoundary 职责：证明方向正确的机器物料链与指定运输锚点接通；使用：全模式最终核验；边界：公共四模式都必须提供真实证据，旧内部总线另有专用端到端核验。
func (b *builder) validateFinalLineBoundary(l *line, head *placed, item int, input bool, mode int, graph map[*placed][]finalMaterialEdge) error {
	if mode < 1 || mode > 4 {
		return nil
	}
	owner := l.machines[0].owner
	found := false
	if mode == 1 || b.directBoundaryUnits != nil {
		for _, route := range b.transportRoutes {
			if route.edge.ItemID != item {
				continue
			}
			if input && route.to != nil && route.to.line == l {
				if route.inputRole == sprayInputRole {
					if l.coaterInput == nil || !finalMaterialReaches(graph, route.end, l.coaterInput, item) {
						return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: fmt.Sprintf("产线%d物品%d的供剂份额没有到达实际喷涂入口", owner, item)}
					}
					continue
				}
				if !finalMaterialReaches(graph, route.end, head, item) {
					return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线%d物品%d的一条供料份额没有到达实际输入链", owner, item)}
				}
				found = true
			}
			if !input && route.from != nil && route.from.line == l {
				if !finalMaterialReaches(graph, head, route.start, item) {
					return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线%d物品%d的一条出货份额没有接到实际输出链", owner, item)}
				}
				found = true
			}
		}
		for _, recycle := range b.machineRecycles {
			if recycle.row.line == l && recycle.item == item {
				if input {
					found = found || finalMaterialReaches(graph, recycle.returned, head, item)
				} else {
					found = found || finalMaterialReaches(graph, head, recycle.route.start, item)
				}
			}
		}
	} else if mode == 2 {
		for _, endpoint := range b.flyEndpoints {
			if endpoint.owner != owner || endpoint.item != item || endpoint.input != input {
				continue
			}
			if input {
				found = found || endpoint.link == head
			} else {
				found = found || finalMaterialReaches(graph, head, endpoint.link, item)
			}
		}
	} else {
		for _, link := range b.stationLinks {
			if link.owner == owner && link.item == item && link.output != input && link.head == head {
				found = true
			}
		}
	}
	if !found {
		direction := "出货末端"
		if input {
			direction = "供料入口"
		}
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线%d物品%d的实际%s没有与指定运输端点接通", owner, item, direction)}
	}
	return nil
}

// validateFinalLineCoaters 职责：证明每个实际喷涂机的供剂槽均从本行已登记入口可达；使用：喷涂顺序之外的补充核验；边界：一台有供剂不能代表整行所有喷涂机均可用。
func validateFinalLineCoaters(l *line, entities []*placed, graph map[*placed][]finalMaterialEdge) error {
	if len(l.coaters) == 0 {
		return nil
	}
	item := productionSprayItem(l.info)
	if item == 0 || l.coaterInput == nil {
		return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "最终喷涂机缺少真实供剂入口"}
	}
	for _, coater := range l.coaters {
		angle := (float64(coater.record.Yaw) - 90) * math.Pi / 180
		point := vector{coater.a.x - math.Cos(angle), coater.a.y + math.Sin(angle), coater.a.z + 1}
		var host *placed
		alive := false
		for _, entity := range entities {
			if entity == coater {
				alive = true
			}
			if surfaceIsBelt(entity.record.ItemID) && flySamePoint(entity.a, point) {
				if host != nil {
					return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "喷涂供剂槽对应多个不明确节点"}
				}
				host = entity
			}
		}
		if !alive || host == nil || !finalMaterialReaches(graph, l.coaterInput, host, item) {
			return &blueprint.Error{Code: "GEN_E_SPRAY_ORDER", Message: "存在无法从实际入口取得增产剂的喷涂机"}
		}
	}
	return nil
}
