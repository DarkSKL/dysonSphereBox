package build

// 文件职责：在直连候选全部落位后按全局真实消费者补齐供电，避免局部电塔提前阻挡装箱。
// 使用：全图路由和裁剪之后、finish 之前；边界：只新增感应塔，不移动设施，不替代最终球面占用与覆盖审计。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/transport"
)

// connectedPowerCellSize 职责：限定米制平面占位桶边长；使用：局部净空和供电查询；边界：高度始终进入三维窄相位，不用二维相邻代替三维覆盖。
const connectedPowerCellSize = 8.

// connectedPowerObstacleCellSize 职责：让细小感应塔净空只枚举邻近障碍；使用：占位桶登记与查询；边界：覆盖和求解索引仍用原八米桶，跨桶形状完整登记。
const connectedPowerObstacleCellSize = 2.

// connectedPowerObstacle 职责：缓存单个实体的精确静态盒或带/分拣器线段；使用：候选塔窄相位；边界：电力节点另按原版三维间距检查。
type connectedPowerObstacle struct {
	entity *placed
	boxes  []placement.Box
	first  placement.Vector
	second placement.Vector
	radius float64
	// nodeProbe 职责：保存正式节点球与缩短胶囊；使用：均衡候选供电净空；边界：不影响既有粗筛候选。
	nodeProbe *surfaceBeltNodeProbe
}

// connectedPowerIndex 职责：索引已固定的全局占位及已登记电塔；使用：避免每个消费者扫描全部实体；边界：新增电塔必须同步登记，线段覆盖其经过的全部平面桶。
type connectedPowerIndex struct {
	builder     *builder
	ctx         context.Context
	catalog     *placement.Catalog
	encoding    *surfaceEncoding
	positions   map[vector]placement.Vector
	obstacles   []connectedPowerObstacle
	cells       map[[2]int][]int
	poles       map[[2]int][]*placed
	poleWorlds  map[*placed]placement.Vector
	poleShapes  []placement.Collider
	coverRadius float64
	clearance   float64
	rejected    map[vector]bool
	// nodeInputs/nodeMembers 职责：按最终记录顺序解析实际带邻居；使用：精确节点几何；边界：仅引用当前实体。
	nodeInputs  map[*placed]*placed
	nodeMembers map[*placed]bool
	// query 职责：复用净空宽相位的形状去重；使用：单候选串行available调用；边界：只去重本轮，不复用动态占位结果。
	query collisionQueryMarks
}

// connectedPowerConsumer 职责：保留真实耗电中心及失败定位身份；使用：补电枚举和有限约束求解；边界：中心不随求解改变，分拣器和堆叠规则由统一解析负责。
type connectedPowerConsumer struct {
	center vector
	world  placement.Vector
	number int
	entity *placed
}

// completeConnectedPower 职责：为坐标已固定的直连候选复用和补建全局感应塔；使用：局部装配临时关闭供电的独立候选；边界：false 不生成，nil 沿用公开配置缺省启用；任一失败撤回本轮新增塔。
// 逻辑流：1. 一次建立全局几何索引 => 2. 解析实体首端和研究站底层接电位置 => 3. 先尝试原有偏移，再搜索覆盖圆内的半格位置 => 4. 登记新塔供后续消费者复用。
func (b *builder) completeConnectedPower(ctx context.Context) (err error) {
	if b.setting.NeedPower != nil && !*b.setting.NeedPower {
		return nil
	}
	if err = b.checkGeneration("直连全局供电"); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	begin, owner, previousContext := len(b.all), b.owner, b.generationCtx
	if previousContext != nil {
		combined, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(previousContext, cancel)
		defer cancel()
		defer stop()
		ctx = combined
	}
	b.owner, b.generationCtx = -1, ctx
	defer func() {
		b.owner, b.generationCtx = owner, previousContext
		if err != nil {
			b.all = b.all[:begin]
		}
	}()
	index, err := newConnectedPowerIndex(ctx, b)
	if err != nil {
		return err
	}
	offsets := connectedPowerCandidateOffsets(index.coverRadius)
	present := make(map[*placed]bool, begin)
	for _, entity := range b.all[:begin] {
		present[entity] = true
	}
	consumers := make([]connectedPowerConsumer, 0, begin/4)
	for number, entity := range b.all[:begin] {
		if err = b.checkGeneration("直连真实消费者供电"); err != nil {
			return err
		}
		if !finalPowerConsumer(entity.record) {
			continue
		}
		center, centerErr := connectedPowerConsumerCenter(entity, present)
		if centerErr != nil {
			return fmt.Errorf("建筑 %d（物品 %d）的全局供电中心无效：%w", number, entity.record.ItemID, centerErr)
		}
		world, worldErr := index.world(center)
		if worldErr != nil {
			return fmt.Errorf("建筑 %d（物品 %d）的球面耗电中心无效：%w", number, entity.record.ItemID, worldErr)
		}
		consumers = append(consumers, connectedPowerConsumer{center: center, world: world, number: number, entity: entity})
	}
	if b.connectedGlobalPower {
		if err = b.solveConnectedPower(index, consumers, offsets); err != nil {
			return err
		}
	} else {
		for _, consumer := range consumers {
			center := consumer.center
			if index.coveredWorld(consumer.world) {
				continue
			}
			placedPole := false
			// 原有偏移优先；密集块的边缝再按半格穷举，半径、净空及电力间距始终保持不变。
			for _, offset := range offsets {
				if err = b.checkGeneration("直连全局电塔候选"); err != nil {
					return err
				}
				point := vector{center.x + offset.x, center.y + offset.y, 0}
				covered, distanceErr := index.withinCover(point, consumer.world)
				if distanceErr != nil {
					return distanceErr
				}
				if !covered {
					continue
				}
				available, candidateErr := index.available(point)
				if candidateErr != nil {
					return candidateErr
				}
				if !available {
					continue
				}
				pole := b.make(2201, point.x, point.y, 0)
				if err = index.add(pole); err != nil {
					return err
				}
				placedPole = true
				break
			}
			if !placedPole {
				return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("全局供电：建筑 %d（物品 %d，产线 %d）的真实耗电中心(%.2f,%.2f,%.2f)在%d个候选内无法同时满足%.1f米覆盖、电力间距与全布局净空", consumer.number, consumer.entity.record.ItemID, consumer.entity.owner, center.x, center.y, center.z, len(offsets), index.coverRadius)}
			}
		}
	}
	for _, consumer := range consumers {
		if err = b.checkGeneration("直连全局覆盖复核"); err != nil {
			return err
		}
		if !index.coveredWorld(consumer.world) {
			return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "全局补电结束后仍有真实消费者未被感应塔覆盖"}
		}
	}
	return nil
}

// connectedPowerCandidateOffsets 职责：给出覆盖圆内确定且有界的补电偏移；使用：原二十位置失败后的边缝搜索；边界：仅枚举地面半格点，真实高度与所有占位仍由调用处审计。
func connectedPowerCandidateOffsets(radius float64) []vector {
	offsets := []vector{{4, 4, 0}, {-4, 4, 0}, {4, -4, 0}, {-4, -4, 0}, {0, 4, 0}, {0, -4, 0}, {4, 0, 0}, {-4, 0, 0}, {6, 0, 0}, {-6, 0, 0}, {0, 6, 0}, {0, -6, 0}, {5, 3, 0}, {-5, 3, 0}, {5, -3, 0}, {-5, -3, 0}, {3, 5, 0}, {-3, 5, 0}, {3, -5, 0}, {-3, -5, 0}}
	seen := make(map[vector]bool, len(offsets))
	for _, offset := range offsets {
		seen[offset] = true
	}
	limit := int(math.Ceil(radius / planarGridLength * 2))
	for ring := 0; ring <= limit; ring++ {
		for x := -ring; x <= ring; x++ {
			for y := -ring; y <= ring; y++ {
				if absInt(x) != ring && absInt(y) != ring {
					continue
				}
				offset := vector{float64(x) / 2, float64(y) / 2, 0}
				if seen[offset] || math.Hypot(offset.x, offset.y)*planarGridLength > radius-1e-4 {
					continue
				}
				seen[offset] = true
				offsets = append(offsets, offset)
			}
		}
	}
	return offsets
}

// connectedPowerConsumerCenter 职责：取得与最终供电审计一致的实体接电位置；使用：全局补电；边界：分拣器与游戏一致使用首端，研究站只沿同物品同配方的有效 input 指针落到底层，拒绝循环或悬空链。
func connectedPowerConsumerCenter(entity *placed, present map[*placed]bool) (vector, error) {
	if entity.record.ItemID != 2901 || entity.record.RecipeID <= 0 {
		return entity.a, nil
	}
	current, seen := entity, map[*placed]bool{}
	for current.input != nil || current.record.InputObjIdx >= 0 {
		next := current.input
		if next == nil || !present[next] || seen[next] || next.record.ItemID != entity.record.ItemID || next.record.RecipeID != entity.record.RecipeID {
			return vector{}, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "堆叠研究站的底层供电连接无效"}
		}
		seen[current] = true
		current = next
	}
	return current.a, nil
}

// newConnectedPowerIndex 职责：预先缓存全局精确占位与电塔；使用：每个独立候选一次；边界：未知模型明确失败，不以缺失几何放行候选。
func newConnectedPowerIndex(ctx context.Context, b *builder) (*connectedPowerIndex, error) {
	return newConnectedPowerIndexForEncoding(ctx, b, b.connectedPowerEncoding)
}

// newConnectedPowerIndexForEncoding 职责：按已确定的最终 Area 建立供电占位索引；使用：共同收尾补漏；边界：不更换锚点，不把多 Area 强制归入主区域。
func newConnectedPowerIndexForEncoding(ctx context.Context, b *builder, encoding *surfaceEncoding) (*connectedPowerIndex, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	shapes, err := catalog.Shapes(2201, b.models["2201"])
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	radius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return nil, err
	}
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	index := &connectedPowerIndex{builder: b, ctx: ctx, catalog: catalog, encoding: encoding, positions: map[vector]placement.Vector{}, cells: map[[2]int][]int{}, poles: map[[2]int][]*placed{}, poleWorlds: map[*placed]placement.Vector{}, rejected: map[vector]bool{}, poleShapes: shapes, coverRadius: radius}
	if reserve, _ := ctx.Value(denseUnitPowerReserveKey{}).(bool); reserve {
		index.coverRadius *= .9
	}
	if b.connectedGlobalPower && encoding.planar {
		// 平面兼容候选保留原额外预排余量；真实球面候选直接执行原型盒，不重复补偿纬向投影。
		index.clearance = .05
	}
	if denseBalancedFrameEnabled(ctx) && usesDenseBeltCollisions(ctx) {
		index.nodeInputs = map[*placed]*placed{}
		index.nodeMembers = map[*placed]bool{}
		for _, p := range b.all {
			if surfaceIsBelt(p.record.ItemID) {
				index.nodeMembers[p] = true
			}
		}
		for _, p := range b.all {
			if index.nodeMembers[p] && index.nodeMembers[p.input] {
				index.nodeInputs[p] = p.input
			}
		}
		for _, p := range b.all {
			if index.nodeMembers[p] && index.nodeMembers[p.output] {
				index.nodeInputs[p.output] = p
			}
		}
	}
	for _, entity := range b.all {
		if err := index.add(entity); err != nil {
			return nil, err
		}
	}
	return index, nil
}

// connectedPowerWorld 职责：转换固定蓝图坐标到切平面米制坐标；使用：无球面编码的合成几何测试与兼容候选；边界：生产球面候选必须使用 surfaceEncoding.surfaceWorldPositions。
func connectedPowerWorld(point vector) placement.Vector {
	return placement.Vector{point.x * planarGridLength, point.z*1.3333333 + .2, point.y * planarGridLength}
}

// world 职责：读取最终候选坐标系中的实体世界位置；使用：供电、碰撞和电网间距窄相位；边界：球面塔点必须留在固定 Area 内，未知或越界坐标失败关闭。
func (index *connectedPowerIndex) world(point vector) (placement.Vector, error) {
	if world, exists := index.positions[point]; exists {
		return world, nil
	}
	if index.encoding == nil || index.encoding.planar {
		world := connectedPowerWorld(point)
		index.positions[point] = world
		return world, nil
	}
	area, exists := index.areaAt(point)
	if !exists {
		return placement.Vector{}, &blueprint.Error{Code: "GEN_E_SURFACE", Message: fmt.Sprintf("全局电塔候选(%.2f,%.2f)超出固定球面区域", point.x, point.y)}
	}
	localX := point.x
	if len(index.encoding.longitudeScales) > 0 {
		localX *= index.encoding.longitudeScales[area]
	}
	localY := point.y - float64(index.encoding.latitudeOffsets[area])
	record := blueprint.Building{AreaIndex: int8(area), LocalOffset: blueprint.Point{X: float32(localX), Y: float32(localY), Z: float32(point.z)}}
	positions, err := index.encoding.surfaceWorldPositions([]blueprint.Building{record})
	if err != nil {
		return placement.Vector{}, err
	}
	world := placement.Vector{positions[0].x, positions[0].y, positions[0].z}
	index.positions[point] = world
	return world, nil
}

// withinCover 职责：以球面三维距离判断塔位是否覆盖实际耗电中心；使用：候选域和普通补电；边界：使用原半径及 1e-4 米余量，不用经纬平面近似替代。
func (index *connectedPowerIndex) withinCover(point vector, center placement.Vector) (bool, error) {
	if !index.insideArea(point) {
		return false, nil
	}
	world, err := index.world(point)
	if err != nil {
		return false, err
	}
	return world.Sub(center).Dot(world.Sub(center)) <= (index.coverRadius-1e-4)*(index.coverRadius-1e-4), nil
}

// insideArea 职责：拒绝会扩张固定球面坐标系的候选塔位；使用：补电域枚举和提交；边界：兼容切平面候选不附加区域限制，实体的非有限数值另由原索引拒绝。
func (index *connectedPowerIndex) insideArea(point vector) bool {
	if index.encoding == nil || index.encoding.planar {
		return true
	}
	_, exists := index.areaAt(point)
	return exists
}

// areaAt 职责：把装配坐标解析到唯一可编码的纬带；使用：最终补电候选与新增塔归属；边界：保留各 Area 经度比例及纬向原点，纬带缝隙不伪造落点。
func (index *connectedPowerIndex) areaAt(point vector) (int, bool) {
	if index.encoding.rootCoordinates {
		return 0, point.x >= 0 && point.y >= 0 && point.x <= float64(index.encoding.plan.LayoutWidth-1) && point.y <= float64(index.encoding.plan.LayoutHeight-1)
	}
	for i, area := range index.encoding.areas {
		x, y := point.x, point.y-float64(index.encoding.latitudeOffsets[i])
		if len(index.encoding.longitudeScales) > 0 {
			x *= index.encoding.longitudeScales[i]
		}
		if x >= 0 && y >= 0 && x <= float64(area.Width-1) && y <= float64(area.Height-1) {
			return i, true
		}
	}
	return 0, false
}

// connectedPowerCell 职责：取得世界平面点的固定网格键；使用：占位和覆盖索引；边界：向负无穷取整以保留负坐标边界。
func connectedPowerCell(x, y float64) [2]int {
	return [2]int{int(math.Floor(x / connectedPowerCellSize)), int(math.Floor(y / connectedPowerCellSize))}
}

// connectedPowerObstacleCell 职责：取得净空专用细桶键；使用：add与available必须成对使用；边界：向负无穷取整，不能用于供电覆盖和求解器的八米桶。
func connectedPowerObstacleCell(x, y float64) [2]int {
	return [2]int{int(math.Floor(x / connectedPowerObstacleCellSize)), int(math.Floor(y / connectedPowerObstacleCellSize))}
}

// add 职责：登记整个静态建造盒或带/分拣器线段经过的桶；使用：首次建索引及每个新增电塔；边界：电力节点的间距范围也并入占位，不只登记首端所在桶。
func (index *connectedPowerIndex) add(entity *placed) error {
	if index.ctx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全局供电占位索引已取消或超时"}
	}
	// 同批预览中分拣器与非分拣器互免；分拣器仍由消费者集合执行真实供电校验。
	if denseBalancedFrameEnabled(index.ctx) && usesDenseBeltCollisions(index.ctx) && entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
		return nil
	}
	obstacle := connectedPowerObstacle{entity: entity}
	first, err := index.world(entity.a)
	if err != nil {
		return err
	}
	low, high := first, first
	if index.nodeInputs != nil && surfaceIsBelt(entity.record.ItemID) {
		before, after := first, first
		previous := index.nodeInputs[entity]
		through := previous != nil && index.nodeMembers[entity.output]
		if through {
			before, err = index.world(previous.a)
			if err != nil {
				return err
			}
			after, err = index.world(entity.output.a)
			if err != nil {
				return err
			}
		}
		probe := index.encoding.beltNodeProbe(first, before, after, through)
		obstacle.nodeProbe = &probe
		for axis := range low {
			low[axis] = probe.center[axis] - surfaceBeltNodeRadius
			high[axis] = probe.center[axis] + surfaceBeltNodeRadius
		}
	} else if surfaceIsBelt(entity.record.ItemID) || entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
		obstacle.first, obstacle.second, obstacle.radius = first, first, .25
		if surfaceIsBelt(entity.record.ItemID) && entity.output != nil && surfaceIsBelt(entity.output.record.ItemID) {
			obstacle.second, err = index.world(entity.output.a)
			if err != nil {
				return err
			}
		}
		if entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
			obstacle.second, obstacle.radius = first, .35
			end, endErr := index.world(entity.b)
			if endErr != nil {
				return endErr
			}
			obstacle.second = end
		}
		for axis := range low {
			low[axis] = math.Min(obstacle.first[axis], obstacle.second[axis]) - obstacle.radius
			high[axis] = math.Max(obstacle.first[axis], obstacle.second[axis]) + obstacle.radius
		}
	} else {
		shapes, err := index.catalog.Shapes(entity.record.ItemID, entity.record.ModelIndex)
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for _, shape := range shapes {
			box, err := index.encoding.colliderBox(vector{first[0], first[1], first[2]}, entity.record.Yaw, shape)
			if err != nil {
				return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
			}
			obstacle.boxes = append(obstacle.boxes, box)
			boxLow, boxHigh := box.Bounds()
			for axis := range low {
				low[axis], high[axis] = math.Min(low[axis], boxLow[axis]), math.Max(high[axis], boxHigh[axis])
			}
		}
	}
	if surfaceIsPowerNode(entity.record.ItemID) {
		padding := math.Sqrt(float64(surfacePowerDistanceLimit(2201, entity.record.ItemID))) + 1e-4
		for _, axis := range []int{0, 2} {
			low[axis], high[axis] = math.Min(low[axis], first[axis]-padding), math.Max(high[axis], first[axis]+padding)
		}
	}
	for axis := range low {
		if math.IsNaN(low[axis]) || math.IsInf(low[axis], 0) || math.IsNaN(high[axis]) || math.IsInf(high[axis], 0) {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: "全局供电索引包含非有限建筑或线段坐标"}
		}
	}
	entry := len(index.obstacles)
	index.obstacles = append(index.obstacles, obstacle)
	minimum, maximum := connectedPowerObstacleCell(low[0], low[2]), connectedPowerObstacleCell(high[0], high[2])
	for x := minimum[0]; x <= maximum[0]; x++ {
		if index.ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全局供电跨桶登记已取消或超时"}
		}
		for y := minimum[1]; y <= maximum[1]; y++ {
			key := [2]int{x, y}
			index.cells[key] = append(index.cells[key], entry)
		}
	}
	if surfaceIsPowerNode(entity.record.ItemID) {
		index.poleWorlds[entity] = first
	}
	if entity.record.ItemID == 2201 {
		key := connectedPowerCell(first[0], first[2])
		index.poles[key] = append(index.poles[key], entity)
	}
	if !index.encoding.planar && index.encoding.byBuilding != nil {
		if _, exists := index.encoding.byBuilding[entity]; !exists {
			area, valid := index.areaAt(entity.a)
			if !valid {
				return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "新增感应塔缺少合法最终区域"}
			}
			if index.encoding.rootCoordinates {
				area = 0
			}
			index.encoding.byBuilding[entity] = area
		}
	}
	return nil
}

// covered 职责：仅查询耗电中心覆盖半径内的已登记电塔；使用：全局共享覆盖及末次复核；边界：保留三维距离和 1e-4 米余量，高层设施不能借用平面半径。
func (index *connectedPowerIndex) covered(center vector) bool {
	world, err := index.world(center)
	return err == nil && index.coveredWorld(world)
}

// coveredWorld 职责：查询真实球面耗电中心附近的已登记感应塔；使用：候选求解和末次覆盖复核；边界：桶只作宽相位，命中必须满足三维米制供电半径。
func (index *connectedPowerIndex) coveredWorld(world placement.Vector) bool {
	minimum := connectedPowerCell(world[0]-index.coverRadius, world[2]-index.coverRadius)
	maximum := connectedPowerCell(world[0]+index.coverRadius, world[2]+index.coverRadius)
	for x := minimum[0]; x <= maximum[0]; x++ {
		for y := minimum[1]; y <= maximum[1]; y++ {
			for _, pole := range index.poles[[2]int{x, y}] {
				poleWorld := index.poleWorlds[pole]
				delta := world.Sub(poleWorld)
				if delta.Dot(delta) <= (index.coverRadius-1e-4)*(index.coverRadius-1e-4) {
					return true
				}
			}
		}
	}
	return false
}

// poleGeometry 职责：按感应塔最终球面姿态建立建造盒；使用：候选静态碰撞与塔间距检查；边界：只扩展预排净空，不代替最终球面终检。
func (index *connectedPowerIndex) poleGeometry(point vector) (placement.Vector, []placement.Box, error) {
	world, err := index.world(point)
	if err != nil {
		return placement.Vector{}, nil, err
	}
	boxes := make([]placement.Box, 0, len(index.poleShapes))
	for _, shape := range index.poleShapes {
		box, boxErr := index.encoding.colliderBox(vector{world[0], world[1], world[2]}, 90, shape)
		if boxErr != nil {
			return placement.Vector{}, nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: boxErr.Error()}
		}
		box.HalfSize[0] += index.clearance
		box.HalfSize[2] += index.clearance
		boxes = append(boxes, box)
	}
	return world, boxes, nil
}

// available 职责：按设施间距、静态盒和当前候选的带臂规则检查塔位；使用：局部空间桶窄相位；边界：既有候选保留粗筛，均衡候选使用正式节点形状和同批分拣器互免，不省略静态盒或真实供电终检。
func (index *connectedPowerIndex) available(point vector) (bool, error) {
	if index.rejected[point] || !index.insideArea(point) {
		return false, nil
	}
	world, boxes, err := index.poleGeometry(point)
	if err != nil {
		return false, err
	}
	low, high := world, world
	for _, box := range boxes {
		boxLow, boxHigh := box.Bounds()
		for axis := range low {
			low[axis], high[axis] = math.Min(low[axis], boxLow[axis]), math.Max(high[axis], boxHigh[axis])
		}
	}
	minimum, maximum := connectedPowerObstacleCell(low[0], low[2]), connectedPowerObstacleCell(high[0], high[2])
	index.query.begin(len(index.obstacles))
	seenCount := 0
	for x := minimum[0]; x <= maximum[0]; x++ {
		for y := minimum[1]; y <= maximum[1]; y++ {
			for _, number := range index.cells[[2]int{x, y}] {
				if index.query.seen(number) {
					continue
				}
				seenCount++
				if seenCount%64 == 0 && index.ctx.Err() != nil {
					return false, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全局供电候选净空检查已取消或超时"}
				}
				obstacle := index.obstacles[number]
				if surfaceIsPowerNode(obstacle.entity.record.ItemID) {
					other := index.poleWorlds[obstacle.entity]
					delta := world.Sub(other)
					minimum := math.Sqrt(float64(surfacePowerDistanceLimit(2201, obstacle.entity.record.ItemID))) + 1e-4
					if delta.Dot(delta) < minimum*minimum {
						index.rejected[point] = true
						return false, nil
					}
				}
				for _, box := range boxes {
					if obstacle.nodeProbe != nil && obstacle.nodeProbe.overlapsBox(box) {
						index.rejected[point] = true
						return false, nil
					}
					if obstacle.radius > 0 && collisionSegmentWithinRadius(box, obstacle.first, obstacle.second, obstacle.radius*obstacle.radius) {
						index.rejected[point] = true
						return false, nil
					}
					for _, other := range obstacle.boxes {
						if box.Overlaps(other) {
							index.rejected[point] = true
							return false, nil
						}
					}
				}
			}
		}
	}
	return true, nil
}
