package build

// 文件职责：为普通塔大厂的真实配方入口构造实体降层短带，保留机器端原带链。
// 使用：生产出口升层完成后、主路搜索之前；边界：只延长真实配方输入stub，不改变机器带、原料需求、带容量或物理规则。
import "context"

// raiseStationConsumerEntries 职责：将独立配方入口延长为通过原校验的实体降层链。
// 使用：普通塔实验性边界准备中、主路布线之前；边界：只处理未接入且有完整stub的真实配方入口，回流及供剂入口保持原样，仍须完整终检。
func (b *builder) raiseStationConsumerEntries(ctx context.Context, env *transportEnvironment, routes []*transportRoute) error {
	seen := map[*placed]bool{}
	owner := b.owner
	defer func() { b.owner = owner }()
	for _, r := range routes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.to == nil || r.to.line == nil || r.end == nil || r.start == nil || r.inputRole != recipeInputRole || recipeHasMachineRecycle(r.to.recipe) || seen[r.end] {
			continue
		}
		old := r.end
		seen[old] = true
		stub := r.to.inputStubs[r.edge.ItemID]
		if len(stub) == 0 || stub[0] != old || r.to.inputEndpoint(r.edge.ItemID, recipeInputRole) != old || old.input != nil || old.a.z >= 8 {
			continue
		}
		source := r.transportSource()
		found := false
		for _, height := range []float64{8, 16, transportMaxHeight} {
			for _, portal := range env.densePortals(old, source, old, denseRotateVector(vector{x: -1}, r.to.quarterTurns), false, false, height) {
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
				virtualSource := &placed{a: last, record: old.record}
				if !env.densePathClear(points, virtualSource, old) || len(b.all)+len(points)-1 > generationBuildingBudget(ctx) {
					continue
				}
				begin := len(b.all)
				b.owner = r.to.owner
				nodes := b.byPath(points[:len(points)-1])
				connect(nodes[len(nodes)-1], old)
				head := nodes[0]
				r.to.inputs[r.edge.ItemID] = head
				r.to.inputStubs[r.edge.ItemID] = append(append([]*placed(nil), nodes...), stub...)
				for _, other := range routes {
					if other.to == r.to && other.end == old && other.edge.ItemID == r.edge.ItemID && other.inputRole == recipeInputRole {
						other.end = head
					}
				}
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
