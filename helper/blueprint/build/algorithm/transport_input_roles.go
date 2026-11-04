package build

import "sort"

// 文件职责：维护直连输入的配方/供剂实体身份。
// 使用：普通mode1与街道附件共同绑定；边界：同物料的两个用途保持独立带头，不复制规范需求。

// inputEndpoint 职责：按规范lane用途返回真实消费者入口；使用：最终端口绑定；边界：未带用途元数据的兼容路线仍读原inputs。
func (row *transportRow) inputEndpoint(item int, role productionInputRole) *placed {
	if role == sprayInputRole && row.sprayInput != nil {
		return row.sprayInput
	}
	return row.inputs[item]
}

// allInputEndpoints 职责：返回去重的全部输入实体；使用：接驳预留和出货过滤区避让；边界：配方与供剂同item不因map键相同而丢失。
func (row *transportRow) allInputEndpoints() []*placed {
	result := []*placed{}
	seen := map[*placed]bool{}
	items := make([]int, 0, len(row.inputs))
	for item := range row.inputs {
		items = append(items, item)
	}
	sort.Ints(items)
	for _, item := range items {
		input := row.inputs[item]
		if input != nil && !seen[input] {
			seen[input] = true
			result = append(result, input)
		}
	}
	if row.sprayInput != nil && !seen[row.sprayInput] {
		result = append(result, row.sprayInput)
	}
	return result
}

// prepareConnectedInputStub 职责：为指定输入用途保留原有连续接驳与逐lane端口；使用：普通mode1及内部兼容入口；边界：只给匹配用途分配此实体头，不复制需求。
func (b *builder) prepareConnectedInputStub(row *transportRow, item int, role productionInputRole, endpoint *placed) []*placed {
	length := 5
	if b.denseSpatial && row.info.IncreasePoints == 0 && !recipeHasMachineRecycle(row.recipe) {
		length = 1
	}
	if b.adapter != nil && b.adapter.connectedRows.reservePerRouteInputStubs {
		count := 0
		for _, route := range b.transportRoutes {
			if route.to == row && route.edge.ItemID == item && route.inputRole == role {
				count++
			}
		}
		length = max(length, count*3+2)
	}
	points := []vector{}
	for x := endpoint.a.x - float64(length); x < endpoint.a.x; x++ {
		points = append(points, vector{x, endpoint.a.y, endpoint.a.z})
	}
	stub := b.byPath(points)
	connect(stub[len(stub)-1], endpoint)
	next := 0
	for _, route := range b.transportRoutes {
		if route.to != row || route.edge.ItemID != item || route.inputRole != role {
			continue
		}
		route.end = stub[0]
		if b.adapter != nil && b.adapter.connectedRows.reservePerRouteInputStubs {
			route.end = stub[next*3+1]
			next++
		}
	}
	return stub
}

// bindSprayInput 职责：登记供剂头并保留旧非双用途map兼容；使用：普通喷涂准备和独立附件；边界：已存在的机器原料头不被替换。
func (row *transportRow) bindSprayInput(item int, head *placed) {
	row.sprayInput = head
	if row.inputs[item] == nil {
		row.inputs[item] = head
	}
}
