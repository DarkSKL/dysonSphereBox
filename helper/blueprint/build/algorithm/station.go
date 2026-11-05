package build

// 文件职责：提供共享物流塔参数、端口路径与机器出货端点。使用：mode 3/4 全场布局；边界：旧中央庭院装配已移除。
// 站点参数和微调数值核对 dsp-calc LocalStation/InterstellarStation，算法独立实现。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/transport"
	"fmt"
)

// stationItem 职责：保存站点物品角色；使用：参数装配；边界：不改变求解结果。
type stationItem struct {
	// id 职责：原型；使用：槽位；边界：不能为零。
	id int
	// provide 职责：供给角色；使用：输入回塔与输出供货区分；边界：物品列表不截断。
	provide bool
}

// stationRoute 职责：绑定产线头和塔槽；使用：先寻路后装配；边界：保存实际指针，不反推索引。
type stationRoute struct {
	// head 职责：产线首带；使用：连接；边界：归属不变。
	head *placed
	// item 职责：供给物品；使用：站点物品选择；边界：回塔为主产物。
	item int
	// output 职责：回塔标记；使用：决定带方向；边界：与站点供给角色不同层次。
	output bool
	// tower 职责：站点数组下标；使用：双塔；边界：0 基。
	tower int
	// slot 职责：塔端口；使用：连接；边界：0..11。
	slot int
	// path 职责：外部路由；使用：装配；边界：只有成功路径可写出。
	path []vector
	// rate 职责：记录本行分配的每分钟流量；使用：塔端口容量核验；边界：不以额定机器余量扩大可信需求。
	rate float64
	// sprayRecycle 职责：标记净输入已抵消的自产供剂行；使用：同塔库存回送；边界：必须另行验证毛流与真实产物入塔链。
	sprayRecycle *line
}

// slotOffset 职责：取得参考端口路径；使用：物流塔的整数过渡点与内部微调；边界：原样保留经验小数。
func slotOffset(slot int) []vector {
	paths := [][]vector{{{1, 4, 0}, {1, 2, 0}}, {{0, 5, 0}, {0, 2, 0}}, {{-1, 6, 0}, {-1, 2, 0}}, {{-4, 7, 0}, {-4, 1, 0}, {-2, 1, 0}}, {{-5, 8, 0}, {-5, 0, 0}, {-2, 0, 0}}, {{-6, 9, 0}, {-6, -1, 0}, {-2, -1, 0}}, {{-1, -6, 0}, {-1, -2, 0}}, {{0, -5, 0}, {0, -2, 0}}, {{1, -4, 0}, {1, -2, 0}}, {{4, -1, 0}, {2, -1, 0}}, {{4, 0, 0}, {2, 0, 0}}, {{4, 1, 0}, {2, 1, 0}}}
	return append([]vector{}, paths[slot]...)
}

// stationInner 职责：生成塔内部对齐带点；使用：外部路由之后；边界：微调在 f64 层完成后统一转 f32。
func stationInner(center vector, slot int) []vector {
	main := slotOffset(slot)
	var points []vector
	for i, v := range main {
		v.x += center.x
		v.y += center.y
		main[i] = v
		if i == 0 {
			points = append(points, v)
		} else {
			segment := manhattan(main[i-1], v, true)
			points = append(points, segment[1:]...)
		}
	}
	points = points[1:]
	n := len(points)
	for _, index := range []int{n - 1, n - 2} {
		delta := .0207
		if index == n-1 {
			delta = .1464
		}
		switch {
		case slot <= 2:
			points[index].y += delta
		case slot <= 5:
			points[index].x -= delta
		case slot <= 8:
			points[index].y -= delta
		default:
			points[index].x += delta
		}
		switch slot {
		case 2, 6:
			points[index].x += .0015
		case 0, 8:
			points[index].x -= .0015
		case 3, 11:
			points[index].y -= .0015
		case 5, 9:
			points[index].y += .0015
		}
	}
	return points
}

// createStation 职责：写 2048 字参数和物品角色；使用：已完成寻路后；边界：调用者保证不超过物品槽。
func (b *builder) createStation(center vector, items []stationItem, config contract.StationConfig) *placed {
	id := config.StationID
	capacity, _ := transport.StationItemCapacity(id)
	p := b.make(id, center.x, center.y, 0)
	p.record.Yaw = 0
	p.record.Yaw2 = 0
	p.record.InputToSlot = 0
	p.record.Parameters = make([]int32, 2048)
	parameters := p.record.Parameters
	for i, item := range items {
		role := int32(2)
		if item.provide {
			role = 1
		}
		parameters[i*6] = int32(item.id)
		parameters[i*6+1] = role
		if config.DualRoleFields {
			parameters[i*6+2] = role
		}
		parameters[i*6+3] = int32(capacity)
	}
	for i := 0; i < 12; i++ {
		parameters[192+i*4] = 1
		if i >= 9 {
			parameters[193+i*4] = int32(i - 8)
		}
	}
	parameters[320] = 200000
	if config.LargeCapacity {
		parameters[320] = 1000000
		parameters[331] = 1
	}
	parameters[321] = -100000000
	parameters[322] = 240000000
	parameters[323] = 1
	parameters[324] = 480000
	parameters[325] = 1
	parameters[326] = 10
	parameters[327] = 100
	parameters[330] = 1
	return p
}

// uniqueStationItems 职责：按配方结果顺序去重多产物；使用：独立出货带与塔槽位规划；边界：不排序、不丢副产物。
func uniqueStationItems(items []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(items))
	for _, item := range items {
		if item <= 0 || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

// stationOutputHead 职责：取得机器端已经按物品过滤的独立出货带首端并写入物品图标；使用：mode3/4逐物品塔路；边界：多产物缺少独立带时硬失败，不再从共享干线二次过滤。
func (b *builder) stationOutputHead(l *line, item int) (*placed, error) {
	row := l.outputForItem(item)
	if len(row) == 0 || len(l.recipe.Results) > 1 && len(l.outputs) != len(l.recipe.Results) {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 产物 %d 缺少独立机器出货带", l.recipe.ID, item)}
	}
	members := map[*placed]bool{}
	for _, belt := range row {
		if belt == nil || !surfaceIsBelt(belt.record.ItemID) || members[belt] {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("配方 %d 产物 %d 的塔出货带存在空节点、非带实体或重复引用", l.recipe.ID, item)}
		}
		members[belt] = true
	}
	for _, other := range l.recipe.Results {
		if other == item {
			continue
		}
		for _, belt := range l.outputForItem(other) {
			if members[belt] {
				return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("配方 %d 不同产物共用物流塔出货带", l.recipe.ID)}
			}
		}
	}
	row[0].record.Parameters = []int32{int32(item), 0}
	return row[0], nil
}
