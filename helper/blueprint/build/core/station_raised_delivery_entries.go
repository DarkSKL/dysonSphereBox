package build

// 文件职责：在全场寻路前为普通大厂的独立入塔交付口提交真实降层带链。
// 使用：station实体边界准备已有固定布局和共同路由索引时；边界：保留原塔槽、原短带、物料与带宽，不处理共享端点或非物流塔接口。
import "context"

// raiseStationDeliveryEntries 职责：把真实独立交付头延伸到合法高层，防止先行路线封死未来的入塔短通道。
// 使用：布局固定后、生产源与配方入口准备前；边界：只提交原portal及完整densePathClear已通过的实体链，候选取消由装配事务丢弃，未找到合法链保持原状。
// 逻辑流：1. 核对唯一route与真实stationLink => 2. 按原高度范围比较合法portal => 3. 创建有向降层链并同步head/path/end => 4. 增量登记原净距索引。
func (b *builder) raiseStationDeliveryEntries(ctx context.Context, env *transportEnvironment, routes []*transportRoute) error {
	uses := map[*placed]int{}
	for _, r := range routes {
		if r != nil && r.end != nil {
			uses[r.end]++
		}
	}
	links := map[*placed]*stationTransportLink{}
	for i := range b.stationLinks {
		link := &b.stationLinks[i]
		if link.output && link.sprayRecycle == nil {
			links[link.head] = link
		}
	}
	owner := b.owner
	defer func() { b.owner = owner }()
	b.owner = -1
	for _, r := range routes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if r == nil || r.from == nil || r.to != nil || r.start == nil || r.end == nil || r.boundary == nil || !surfaceIsStation(r.boundary.record.ItemID) {
			continue
		}
		old := r.end
		link := links[old]
		if uses[old] != 1 || link == nil || link.tower != r.boundary || link.item != r.edge.ItemID || old.input != nil || old.a.z >= 8 || len(link.path) == 0 || old.output != link.path[0] {
			continue
		}
		source := r.transportSource()
		found := false
		for _, height := range []float64{8, 16, transportMaxHeight} {
			for _, portal := range env.densePortals(old, source, old, vector{x: -1}, false, false, height) {
				// 至少包含旧端点和一个新点，才可读取出口并提交非空降层链。
				if len(portal.points) < 2 || portal.points[0] != old.a {
					continue
				}
				last := portal.points[len(portal.points)-1]
				if portal.height < 8 || last.z < 8 {
					continue
				}
				points := make([]vector, len(portal.points))
				for i, p := range portal.points {
					points[len(points)-1-i] = p
				}
				virtual := &placed{a: last, record: old.record}
				if !env.densePathClear(points, virtual, old) || len(b.all)+len(points)-1 > generationBuildingBudget(ctx) {
					continue
				}
				begin := len(b.all)
				nodes := b.byPath(points[:len(points)-1])
				connect(nodes[len(nodes)-1], old)
				head := nodes[0]
				head.record.Parameters = append([]int32(nil), old.record.Parameters...)
				link.path = append(append(append([]*placed(nil), nodes[1:]...), old), link.path...)
				link.head, r.end = head, head
				for _, p := range nodes {
					env.densePorts.blockNode(env.blocked, p.a)
					env.densePorts.addNode(p)
				}
				b.previewRouteAppend(begin, old)
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	return nil
}
