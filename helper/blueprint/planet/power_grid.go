package planet

// 文件职责：连通原版火电与新建设施；使用：整星装配前；边界：无法连通则返回具体用电设备。

import (
	"DysonSphereBox/helper/industryplan/search"
	"container/heap"
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/transport"
)

// powerNode 职责：保存已接节点的球面位姿和实际原型能力；使用：供电路由；边界：只包含火电和特斯拉塔。
type powerNode struct {
	position       Position
	point          placement.Vector
	connect, cover float64
}

// wireEntry 职责：缓存在原版网格上的搜索路径及代价；使用：故障绕路；边界：父索引由同一搜索产生。
type wireEntry struct {
	node          powerNode
	parent, depth int
	score         float64
}

// wireQueue 职责：按估计余程选候选；使用：有界路径搜索；边界：平分时按登记顺序确定。
type wireQueue struct {
	entries *[]wireEntry
	order   []int
}

// Len 职责：汇报候选数；使用：heap；边界：不含已访问节点。
func (q wireQueue) Len() int { return len(q.order) }

// Less 职责：比较路径优先级；使用：heap；边界：得分相同用登记序。
func (q wireQueue) Less(i, j int) bool {
	a, b := q.order[i], q.order[j]
	left, right := (*q.entries)[a].score, (*q.entries)[b].score
	return left < right || left == right && a < b
}

// Swap 职责：交换堆元素；使用：heap；边界：路径祖先索引不变。
func (q wireQueue) Swap(i, j int) { q.order[i], q.order[j] = q.order[j], q.order[i] }

// Push 职责：将登记索引放入堆；使用：heap；边界：输入必须为 int。
func (q *wireQueue) Push(value any) { q.order = append(q.order, value.(int)) }

// Pop 职责：返回堆尾候选；使用：heap；边界：调用时不允许空堆。
func (q *wireQueue) Pop() any {
	i := len(q.order) - 1
	value := q.order[i]
	q.order = q.order[:i]
	return value
}

// connectPowerGrid 职责：为离散设备布置真实连通电力节点；使用：已有额定火电；边界：依据原版电力塔半径和连接距离。
func connectPowerGrid(ctx context.Context, req *Request, catalog *placement.Catalog, deviceFacts *facts.Catalog) error {
	cover, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return err
	}
	reach, err := transport.TeslaPowerConnectDistance()
	if err != nil {
		return err
	}
	point := powerGridPoint
	var network []powerNode
	// 原版 25955035 resources.assets 的 PowerDesc，单位游戏米。
	radii := map[int16][2]float64{2201: {22.5, 10.5}, 2203: {16.5, 7.7}, 2204: {11.75, 4.5}, 2205: {6.75, 0}, 2206: {4.2, 0}, 2208: {22.5, 10.5}, 2209: {15.5, 7}, 2210: {15, 5}, 2211: {11.75, 4.5}, 2213: {22, 0}}
	var targets []Object
	for _, object := range req.Objects {
		fact, ok := deviceFacts.Lookup(int(object.Building.ItemID))
		if !ok || fact.Power == nil || !fact.Power.Node {
			continue
		}
		source := fact.Power.Generator || fact.Power.Exchanger && len(object.Building.Parameters) > 0 && object.Building.Parameters[0] == -1
		if fact.Power.Gamma && len(object.Building.Parameters) > 0 && object.Building.Parameters[0] == 1208 {
			source = false
		}
		if source && len(network) == 0 {
			radius := radii[object.Building.ItemID]
			network = append(network, powerNode{object.First, point(object.First), radius[0], radius[1]})
		} else {
			targets = append(targets, object)
		}
	}
	if len(network) == 0 {
		if !req.externalGrid {
			return fmt.Errorf("未生成可供电节点")
		}
		if len(targets) > 0 {
			root := targets[0]
			radius := radii[root.Building.ItemID]
			network = append(network, powerNode{root.First, point(root.First), radius[0], radius[1]})
			targets = targets[1:]
		} else {
			p, ok := standaloneSite(extendOccupancy(*req, catalog), req.wet, catalog)
			if !ok {
				return fmt.Errorf("没有可用的已有电网接入点落位")
			}
			req.Objects = append(req.Objects, Object{First: p, Second: p, Module: "电网接入", Building: blueprint.Building{Index: nextObjectIndex(req.Objects), ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}})
			network = append(network, powerNode{p, point(p), reach, cover})
		}
	}
	fixedNodes := len(targets)
	for _, object := range req.Objects {
		fact, ok := deviceFacts.Lookup(int(object.Building.ItemID))
		if ok && fact.Power != nil && fact.Power.Work > 0 && !fact.Power.Node {
			targets = append(targets, object)
		}
	}
	occupied := newObstacleIndex(append(append([]Obstacle(nil), req.Obstacles...), req.minerals...))
	nodeSpacing := obstacleIndex{}
	poleBuilding := blueprint.Building{ItemID: 2201, ModelIndex: 44, InputObjIdx: -1, OutputObjIdx: -1}
	poleShapes, err := catalog.Shapes(poleBuilding.ItemID, poleBuilding.ModelIndex)
	if err != nil {
		return err
	}
	// 原版杆体从地面延伸到盒顶。带高于盒顶的部分按实际垂直余量判断。
	poleHeight := float64(poleShapes[0].Position[1]+poleShapes[0].Extents[1]) / float64(geometry.LayerPitch)
	for _, object := range req.Objects {
		radius := groundRadius(catalog, object.Building)
		first, second := object.First, object.Second
		if object.Building.ItemID >= 2001 && object.Building.ItemID <= 2003 {
			first.Height -= min(poleHeight, max(0, first.Height))
			second.Height -= min(poleHeight, max(0, second.Height))
		}
		occupied.add(Obstacle{Position: first, Radius: radius})
		if second != first {
			occupied.add(Obstacle{Position: second, Radius: radius})
		}
		if fact, ok := deviceFacts.Lookup(int(object.Building.ItemID)); ok && fact.Power != nil && fact.Power.Node {
			nodeSpacing.add(Obstacle{Position: object.First, Radius: 3.55})
		}
	}
	index := nextObjectIndex(req.Objects)
	poleRadius := groundRadius(catalog, poleBuilding)
	var joined map[Position]bool
	for ordinal, target := range targets {
		if err := search.Checkpoint(ctx); err != nil {
			return err
		}
		targetPoint := point(target.First)
		fact, _ := deviceFacts.Lookup(int(target.Building.ItemID))
		isNode := fact.Power != nil && fact.Power.Node
		if isNode && joined[target.First] {
			continue
		}
		targetReach, targetCover := 0.0, 0.0
		if radius, ok := radii[target.Building.ItemID]; ok {
			targetReach, targetCover = radius[0], radius[1]
		}
		available := func(p Position) bool {
			w, err := world(p)
			if err != nil || occupied.conflicts(w, poleRadius) || nodeSpacing.conflicts(w, 0) {
				return false
			}
			probe := Object{Building: poleBuilding, First: p, Second: p}
			return !terrainConflict([]Object{probe}, req.wet, catalog)
		}
		path, err := findPowerPath(ctx, network, targetPoint, targetReach, reach, cover, available, point)
		if err != nil && isNode && targetReach > 0 && ctx.Err() == nil {
			// 设备登记顺序与电线连通无关，工区内部已摆好的杆可能从另一端接入。
			var fixed []powerNode
			for _, other := range targets[ordinal:fixedNodes] {
				if r, ok := radii[other.Building.ItemID]; ok {
					fixed = append(fixed, powerNode{other.First, point(other.First), r[0], r[1]})
				}
			}
			if component := connectedFixedNodes(ctx, network, fixed); len(component) > 0 {
				network = append(network, component...)
				if joined == nil {
					joined = map[Position]bool{}
				}
				for _, node := range component {
					joined[node.position] = true
				}
				continue
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return &Error{PlanetID: req.PlanetID, Item: target.Item, Module: "电网", Code: "PLANET_POWER_GRID", Message: fmt.Sprintf("无法将设备 %d（原型 %d，经纬 %.4f, %.4f）接入火电：%v", target.Building.Index, target.Building.ItemID, target.First.Longitude, target.First.Latitude, err)}
		}
		for _, n := range path {
			pole := Object{Building: poleBuilding, First: n.position, Second: n.position, Item: target.Item, Module: "电网"}
			pole.Building.Index = index
			index++
			req.Objects = append(req.Objects, pole)
			occupied.add(Obstacle{Position: n.position, Radius: groundRadius(catalog, poleBuilding)})
			nodeSpacing.add(Obstacle{Position: n.position, Radius: 3.55})
			network = append(network, n)
		}
		if isNode {
			network = append(network, powerNode{target.First, targetPoint, targetReach, targetCover})
		}
	}
	return nil
}

// connectedFixedNodes 从首个待接节点沿固定电杆查找，只有实际连接到带电网络的分量才可以整组接入。
func connectedFixedNodes(ctx context.Context, network, fixed []powerNode) []powerNode {
	if len(fixed) == 0 {
		return nil
	}
	linked := func(a, b powerNode) bool {
		r := float32(max(a.connect, b.connect))
		return powerGridDistanceSquared(a.point, b.point) <= r*r
	}
	component := []powerNode{fixed[0]}
	remaining := fixed[1:]
	connected := false
	for i := 0; i < len(component); i++ {
		if ctx.Err() != nil {
			return nil
		}
		current := component[i]
		if !connected {
			for _, source := range network {
				if linked(source, current) {
					connected = true
					break
				}
			}
		}
		next := remaining[:0]
		for _, node := range remaining {
			if linked(current, node) {
				component = append(component, node)
			} else {
				next = append(next, node)
			}
		}
		remaining = next
	}
	if connected {
		return component
	}
	return nil
}

// findPowerPath 职责：寻找可绕过矿区或液面的原版节点链；使用：电网组件连接；边界：按真实半径及单精度平方距离判断连接与覆盖。
func findPowerPath(ctx context.Context, starts []powerNode, target placement.Vector, targetReach, reach, cover float64,
	available func(Position) bool, point func(Position) placement.Vector) ([]powerNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := func(node powerNode) float32 {
		value := node.cover
		if targetReach > 0 {
			value = max(node.connect, targetReach)
		}
		return float32(value)
	}
	covered := func(node powerNode) bool {
		radius := limit(node)
		return powerGridDistanceSquared(node.point, target) <= radius*radius
	}
	margin := func(node powerNode) float64 {
		return math.Sqrt(float64(powerGridDistanceSquared(node.point, target))) - float64(limit(node))
	}
	targetDirection := unitVector(target)
	estimate := func(p placement.Vector) float64 {
		// 球面弧长用于远距离引导，弦长在接近背面时几乎不给搜索方向。
		cosine := max(-1, min(1, unitVector(p).Dot(targetDirection)))
		return math.Acos(cosine) * (200.0 / 20.0) * 2.5
	}
	var entries []wireEntry
	queue := &wireQueue{entries: &entries}
	best := math.Inf(1)
	seen := map[Position]bool{}
	for _, start := range starts {
		gap := margin(start)
		if covered(start) {
			return nil, nil
		}
		best = min(best, gap)
	}
	for _, start := range starts {
		quality := margin(start)
		if quality > best+30 {
			continue
		}
		entries = append(entries, wireEntry{node: start, parent: -1, score: estimate(start.point)})
		heap.Push(queue, len(entries)-1)
		seen[start.position] = true
	}
	for visited := 0; queue.Len() > 0 && visited < 500000; visited++ {
		if err := search.Checkpoint(ctx); err != nil {
			return nil, err
		}
		i := heap.Pop(queue).(int)
		current := entries[i]
		if covered(current.node) {
			path := make([]powerNode, current.depth)
			for index, parent := len(path)-1, i; index >= 0; index-- {
				path[index] = entries[parent].node
				parent = entries[parent].parent
			}
			return path, nil
		}
		up := unitVector(current.node.point)
		dir := target.Sub(current.node.point)
		along := unitVector(dir.Sub(up.Scale(dir.Dot(up))))
		if along == (placement.Vector{}) {
			along = unitVector(up.Cross(placement.Vector{0, 1, 0}))
		}
		across := along.Cross(up)
		for _, radius := range []float64{20, 15, 10, 6, 4.5} {
			for _, angle := range []float64{0, -.3, .3, -.65, .65, -1, 1, -1.57, 1.57, -2.25, 2.25, math.Pi} {
				s, c := math.Sincos(angle)
				p := snapToGround(current.node.point.Add(along.Scale(radius * c)).Add(across.Scale(radius * s)))
				if seen[p] || !available(p) {
					continue
				}
				v := point(p)
				connection := float32(max(current.node.connect, reach))
				if gap := powerGridDistanceSquared(v, current.node.point); gap < 3.55*3.55 || gap > connection*connection {
					continue
				}
				clear := true
				for parent := i; parent >= 0; parent = entries[parent].parent {
					if powerGridDistanceSquared(v, entries[parent].node.point) < 3.55*3.55 {
						clear = false
						break
					}
				}
				if !clear {
					continue
				}
				next := powerNode{p, v, reach, cover}
				left := margin(next)
				best = min(best, left)
				seen[p] = true
				entries = append(entries, wireEntry{node: next, parent: i, depth: current.depth + 1, score: float64(current.depth+1) + estimate(v)})
				heap.Push(queue, len(entries)-1)
			}
		}
	}
	return nil, fmt.Errorf("绕行后仍差 %.2f 游戏米的连接或覆盖", max(best, 0))
}

// powerGridPoint 职责：复现原版电网按实体高度归一到地表的接电点；使用：节点与消费者距离；边界：仅供电判断归一，不改变建筑、碰撞或分拣器端点。
func powerGridPoint(position Position) placement.Vector {
	w, _ := world(position)
	x, y, z := float32(w.X), float32(w.Y), float32(w.Z)
	altitude := float32(math.Sqrt(float64(x*x + y*y + z*z)))
	scale := altitude / (200 + float32(geometry.BaseLift))
	return placement.Vector{float64(x / scale), float64(y / scale), float64(z / scale)}
}

// powerGridDistanceSquared 职责：复用原版逐分量单精度距离运算；使用：供电边界；边界：输入为同一行星的已映射接电点，不附加容差。
func powerGridDistanceSquared(left, right placement.Vector) float32 {
	frame := geometry.PlanetWorldFrame()
	value, _ := geometry.DistanceSquared32(
		geometry.WorldPoint{Frame: frame, X: geometry.WorldLength(left[0]), Y: geometry.WorldLength(left[1]), Z: geometry.WorldLength(left[2])},
		geometry.WorldPoint{Frame: frame, X: geometry.WorldLength(right[0]), Y: geometry.WorldLength(right[1]), Z: geometry.WorldLength(right[2])})
	return value
}

// unitVector 职责：获取稳定方向；使用：电网节点搜索；边界：零长度向量保持零。
func unitVector(v placement.Vector) placement.Vector {
	if length := math.Sqrt(v.Dot(v)); length > 0 {
		return v.Scale(1 / length)
	}
	return v
}

// snapToGround 职责：将世界候选吸附原版地表网格；使用：电力塔节点；边界：不选不可放置极点。
func snapToGround(point placement.Vector) Position {
	unit := unitVector(point)
	latStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	row := min(249, max(-249, math.Round(math.Asin(unit[1])/float64(latStep))))
	lat := float64(float32(row) * float32(latStep))
	lonStep, _ := geometry.PasteLongitudeStep32(geometry.Radians(lat), geometry.Vanilla200)
	lon := math.Round(math.Atan2(unit[0], -unit[2])/float64(lonStep)) * float64(lonStep)
	return Position{Latitude: lat, Longitude: lon}
}
