package build

// 文件职责：差分保护原portal及原成功路线，复现朝外取整落入静态盒的半格接驳缺口。
// 使用：只运行微型真实几何和原寻路循环；边界：不改物理阈值、不启动大厂、不覆盖主线边界逻辑。

import (
	"context"
	"math"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// sidePortalTestFrames 职责：提供平面及南北原生球面的独立测试坐标系。
// 使用：portal差分矩阵；边界：只构造内存事实，不依赖临时诊断测试文件。
func sidePortalTestFrames() []*densePolarFrame {
	frames := []*densePolarFrame{nil}
	for _, start := range []int{0, 180, -180} {
		frames = append(frames, &densePolarFrame{start: start, height: 100, segments: 200, latitudeStep: float32(math.Pi / 500), longitudeStep: float32(math.Pi / 500)})
	}
	return frames
}

// sidePortalTestIndex 职责：提供带严格避碰的独立测试索引。
// 使用：差分和几何拒绝回归；边界：不加载数据集或改变生产碰撞参数。
func sidePortalTestIndex(frame *densePolarFrame) *denseRoutePortIndex {
	return &denseRoutePortIndex{polarFrame: frame, strictBelts: true,
		cells: map[[3]int][]int{}, nodes: map[[3]int][]*placed{}, previous: map[*placed][]*placed{},
		edges: map[[2]vector]bool{}, registered: map[*placed]bool{}, beltCells: map[[3]int][]int{}, beltEdges: map[[2]*placed]bool{}}
}

// sidePortalTestNode 职责：构造具有真实带物品身份的测试节点。
// 使用：微型图的占位及连接；边界：不创建游戏或持久化实体。
func sidePortalTestNode(point vector) *placed {
	return &placed{a: point, record: blueprint.Building{ItemID: 2001}}
}

// sidePortalTestEnvironment 职责：创建彼此独立的确定性微型场；使用：旧/新算法差分；边界：端点保持真实实体，所有已放节点照常登记占位。
func sidePortalTestEnvironment(frame *densePolarFrame, sourcePoint, targetPoint vector, obstacle, merge bool) (*transportEnvironment, *transportRoute, []*placed, map[*placed]int) {
	idx := sidePortalTestIndex(frame)
	source, target := sidePortalTestNode(sourcePoint), sidePortalTestNode(targetPoint)
	env := &transportEnvironment{densePorts: idx, minimum: vector{-4, -4, 0}, maximum: vector{9, 9, transportMaxHeight}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}, beltCorridors: map[[2]int]bool{}}
	register := func(p *placed) { idx.addNode(p); idx.blockNode(env.blocked, p.a) }
	register(source)
	register(target)
	if obstacle {
		register(sidePortalTestNode(vector{2, 2, 0}))
	}
	var tail []*placed
	if merge {
		anchor := sidePortalTestNode(vector{math.Round(targetPoint.x) + 1, math.Round(targetPoint.y) + 1, 0})
		connect(anchor, target)
		register(anchor)
		tail = []*placed{anchor, target}
	}
	return env, &transportRoute{start: source, end: target, from: &transportRow{}, to: &transportRow{}}, tail, map[*placed]int{}
}

// TestDenseSidePortalsPreserveSuccessfulRoutes 职责：对冻结旧循环可通案例逐项比较完整结果与搜索计数。
// 使用：平面及南北球面、整数/分数端点、障碍和合流矩阵；边界：原成功路线不能因为存在更便宜的备用而改变。
func TestDenseSidePortalsPreserveSuccessfulRoutes(t *testing.T) {
	checks := 0
	for frameID, frame := range sidePortalTestFrames() {
		for _, endpoint := range []vector{{0, 0, 0}, {.25, .5, 0}, {1, 1, .5}} {
			for _, obstacle := range []bool{false, true} {
				for _, merge := range []bool{false, true} {
					old, oldRoute, oldTail, oldIncoming := sidePortalTestEnvironment(frame, vector{6, 5, 0}, endpoint, obstacle, merge)
					want, wi, we, wm, ws := old.sidePortalLegacyPath(context.Background(), oldRoute, oldTail, oldIncoming, vector{x: -1})
					if len(want) == 0 {
						t.Fatalf("差分夹具原算法不可通 frame%d target%v obstacle%v merge%v", frameID, endpoint, obstacle, merge)
					}
					env, route, tail, incoming := sidePortalTestEnvironment(frame, vector{6, 5, 0}, endpoint, obstacle, merge)
					got, gi, ge, gm, gs := env.findDenseTransportPath(context.Background(), route, tail, incoming, vector{x: -1})
					if !reflect.DeepEqual(got, want) || gi != wi || ge != we || gm != wm || gs != ws || env.searchCalls != old.searchCalls || env.totalExpanded != old.totalExpanded {
						t.Fatalf("原可通结果变化 frame%d target%v obstacle%v merge%v", frameID, endpoint, obstacle, merge)
					}
					checks++
				}
			}
		}
	}
	t.Logf("冻结旧循环成功路线差分%d组：完整路径、合流索引和搜索计数一致", checks)
}

// sidePortalAddTestBox 职责：把米制真实静态盒注册到原宽窄相位；使用：方向缺口和拒绝回归；边界：不改变碰撞查询实现。
func sidePortalAddTestBox(idx *denseRoutePortIndex, center, half placement.Vector) {
	box := placement.Box{Center: center, HalfSize: half, Axes: [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
	shape := boxPackingShape("static", box)
	n := len(idx.boxes)
	idx.boxes = append(idx.boxes, shape)
	packingShapeCells(shape, func(cell [3]int) bool { idx.cells[cell] = append(idx.cells[cell], n); return true })
}

// sidePortalBlockedFixture 职责：复现分数端点向外取整进入机器、原地升层遇到上部盒的真实几何。
// 使用：半格侧移回归；边界：东侧已有真实后缀，不能移动端点、忽略盒或改变方向来让测试通过。
func sidePortalBlockedFixture() (*transportEnvironment, *transportRoute) {
	env, r, _, _ := sidePortalTestEnvironment(nil, vector{5, 4, 0}, vector{.3, 0, 0}, false, false)
	inner := sidePortalTestNode(vector{2, 0, 0})
	connect(r.end, inner)
	env.densePorts.addNode(inner)
	sidePortalAddTestBox(env.densePorts, placement.Vector{-1.5, .6, 0}, placement.Vector{1.5, .6, 3})
	sidePortalAddTestBox(env.densePorts, placement.Vector{.1, 2.1, 0}, placement.Vector{1.5, .6, 3})
	return env, r
}

// TestDenseSidePortalsRecoverFractionalEntry 职责：证明旧目标portal为零而有限半格候选能完成原物理规则下的整条路线。
// 使用：1120/1121缺口的最小几何回归；边界：备用候选数量固定，首末实体身份不变，完整带链还须原终检。
func TestDenseSidePortalsRecoverFractionalEntry(t *testing.T) {
	env, r := sidePortalBlockedFixture()
	oldPortals := env.densePortals(r.end, r.start, r.end, vector{x: -1}, false, false)
	if len(oldPortals) != 0 {
		t.Fatalf("夹具旧portal未封闭:%v", oldPortals)
	}
	backup := env.denseSidePortals(r.end, r.start, r.end, vector{x: -1}, false, false)
	if len(backup) == 0 || len(backup) > 6 {
		t.Fatalf("备用数量无效:%d", len(backup))
	}
	path, _, _, _, _ := env.findDenseTransportPath(context.Background(), r, nil, map[*placed]int{}, vector{x: -1})
	if len(path) == 0 {
		t.Fatal("合法半格侧移未恢复完整寻路")
	}
	full := append([]vector{r.start.a}, path...)
	if path[len(path)-1] != r.end.a || !env.densePathClear(full, r.start, r.end) {
		t.Fatal("备用完整路线未通过原终检或改变端点")
	}
}

// TestDenseSidePortalsRejectReservationsAndCollision 职责：证明侧移仍执行真实净空和连续预留。
// 使用：既有两侧分别封闭及固定地面头回归；边界：备用不能穿静态盒、抢占未来入口或绕过固定直出合同。
func TestDenseSidePortalsRejectReservationsAndCollision(t *testing.T) {
	env, r := sidePortalBlockedFixture()
	if len(env.denseSidePortals(r.end, r.start, r.end, vector{x: -1}, false, true)) != 0 {
		t.Fatal("固定地面头启用了侧移备用")
	}
	owner := sidePortalTestNode(vector{})
	for _, dy := range []float64{-.5, .5} {
		p := vector{r.end.a.x, r.end.a.y + dy, r.end.a.z}
		env.reserveTransportSpan(r.end.a, p, owner)
	}
	if len(env.denseSidePortals(r.end, r.start, r.end, vector{x: -1}, false, false)) != 0 {
		t.Fatal("半格接驳穿过其它入口的连续预留")
	}
	env, r = sidePortalBlockedFixture()
	sidePortalAddTestBox(env.densePorts, placement.Vector{.4, .4, 0}, placement.Vector{.5, .5, 1})
	if len(env.denseSidePortals(r.end, r.start, r.end, vector{x: -1}, false, false)) != 0 {
		t.Fatal("半格接驳穿过真实静态盒")
	}
}

// TestDenseSidePortalsOrdinaryDirections 职责：以普通模式严格带段规则覆盖四向、整数及半格端点。
// 使用：普通case04同类方向缺口的微型几何回归；边界：只旋转真实实体和静态盒，原可通路线保持原结果，补充路线仍经原整链终检，不运行完整case04。
func TestDenseSidePortalsOrdinaryDirections(t *testing.T) {
	primary, fallback := 0, 0
	for turn := 0; turn < 4; turn++ {
		for _, targetX := range []float64{0, .3, .5} {
			delta := targetX - .3
			sourcePoint := denseRotateVector(vector{5 + delta, 4, 0}, turn)
			targetPoint := denseRotateVector(vector{targetX, 0, 0}, turn)
			env, r, _, _ := sidePortalTestEnvironment(nil, sourcePoint, targetPoint, false, false)
			env.minimum, env.maximum = vector{-9, -9, 0}, vector{9, 9, transportMaxHeight}
			r.to.quarterTurns = turn
			inner := sidePortalTestNode(denseRotateVector(vector{2 + delta, 0, 0}, turn))
			connect(r.end, inner)
			env.densePorts.addNode(inner)
			for _, box := range [][2]placement.Vector{{{-1.5, .6, 0}, {1.5, .6, 3}}, {{.1, 2.1, 0}, {1.5, .6, 3}}} {
				center, half := box[0], box[1]
				p := denseRotateVector(vector{center[0] + delta*planarGridLength, center[2], center[1]}, turn)
				if turn%2 != 0 {
					half[0], half[2] = half[2], half[0]
				}
				sidePortalAddTestBox(env.densePorts, placement.Vector{p.x, p.z, p.y}, half)
			}
			preferred := denseRotateVector(vector{x: -1}, turn)
			old, _, _, _, _ := env.sidePortalLegacyPath(context.Background(), r, nil, map[*placed]int{}, preferred)
			path, _, _, _, _ := env.findDenseTransportPath(context.Background(), r, nil, map[*placed]int{}, preferred)
			if len(path) == 0 || !env.densePathClear(append([]vector{r.start.a}, path...), r.start, r.end) {
				t.Fatalf("普通严格模式不通 turn%d targetX%v", turn, targetX)
			}
			if len(old) > 0 {
				primary++
				if !reflect.DeepEqual(old, path) {
					t.Fatalf("普通原可通路线变化 turn%d targetX%v", turn, targetX)
				}
			} else {
				fallback++
			}
		}
	}
	if primary == 0 || fallback == 0 {
		t.Fatal("普通方向矩阵未覆盖原成功和备用成功")
	}
	t.Logf("普通四向/整数半格12组，原成功%d组不变，备用恢复%d组", primary, fallback)
}

// TestDenseSidePortalsRecoverRealTailAnchor 职责：复现实际尾链没有可用直接整数目标时的合流。
// 使用：material-network同类分数锚点缺口；边界：复用真实锚点和尾链索引，饱和锚点仍拒绝，不能将实际端点改为四舍五入的邻格。
func TestDenseSidePortalsRecoverRealTailAnchor(t *testing.T) {
	env, r := sidePortalBlockedFixture()
	tail := []*placed{r.end}
	incoming := map[*placed]int{r.end: 1}
	old, _, _, _, _ := env.sidePortalLegacyPath(context.Background(), r, tail, incoming, vector{x: -1})
	if len(old) != 0 {
		t.Fatal("夹具旧合流未失败")
	}
	path, index, _, _, _ := env.findDenseTransportPath(context.Background(), r, tail, incoming, vector{x: -1})
	if len(path) == 0 || index != 0 || path[len(path)-1] != r.end.a || !env.densePathClear(append([]vector{r.start.a}, path...), r.start, r.end) {
		t.Fatal("备用未通过真实锚点侧接")
	}
	if incoming[r.end] != 1 || tail[0] != r.end {
		t.Fatal("试探修改了尾链或输入计数")
	}
	incoming[r.end] = 3
	path, _, _, _, _ = env.findDenseTransportPath(context.Background(), r, tail, incoming, vector{x: -1})
	if len(path) > 0 {
		t.Fatal("备用绕过了三输入容量")
	}
}

// TestDenseSidePortalsOrdinaryIntegerMerge 职责：在普通严格模式下复现整数锚点直接邻格封闭、半格侧接仍可行。
// 使用：case04低展开合流失败的同规则四向回归；边界：阻塞物均有真实外向带段，不能借跳过孤立节点让测试通过。
func TestDenseSidePortalsOrdinaryIntegerMerge(t *testing.T) {
	for turn := 0; turn < 4; turn++ {
		env, r, _, _ := sidePortalTestEnvironment(nil, denseRotateVector(vector{5, 4, 0}, turn), vector{}, false, false)
		env.minimum, env.maximum = vector{-9, -9, 0}, vector{9, 9, transportMaxHeight}
		for _, d := range []vector{{x: 1}, {x: -1}, {y: 1}, {y: -1}, {z: 1}} {
			a := sidePortalTestNode(denseRotateVector(d, turn))
			b := sidePortalTestNode(denseRotateVector(vector{2 * d.x, 2 * d.y, 2 * d.z}, turn))
			connect(a, b)
			for _, p := range []*placed{a, b} {
				env.densePorts.addNode(p)
				env.densePorts.blockNode(env.blocked, p.a)
			}
		}
		tail := []*placed{r.end}
		incoming := map[*placed]int{r.end: 1}
		preferred := denseRotateVector(vector{x: -1}, turn)
		old, _, _, _, _ := env.sidePortalLegacyPath(context.Background(), r, tail, incoming, preferred)
		if len(old) > 0 {
			t.Fatalf("整数合流夹具原算法未失败 turn%d", turn)
		}
		path, index, _, _, _ := env.findDenseTransportPath(context.Background(), r, tail, incoming, preferred)
		if len(path) == 0 || index != 0 || !env.densePathClear(append([]vector{r.start.a}, path...), r.start, r.end) {
			t.Fatalf("普通整数锚点未恢复 turn%d", turn)
		}
	}
}

// TestDenseSidePortalsKeepFixedGroundMerge 职责：确认已有尾链时仍不从侧面绕过裸地面剩余出口的固定东向末段。
// 使用：公共port向兼容模式回归；边界：其它真实上游锚点仍可合流，不能向裸终点直接新增侧向入边。
func TestDenseSidePortalsKeepFixedGroundMerge(t *testing.T) {
	env, r := sidePortalBlockedFixture()
	r.to = nil
	r.edge.FlowKind = "leftover"
	r.boundary = r.end
	path, _, _, _, _ := env.findDenseTransportPath(context.Background(), r, []*placed{r.end}, map[*placed]int{r.end: 1}, vector{x: -1})
	if len(path) > 0 {
		t.Fatal("合流备用绕过了裸剩余地面出口的固定东向末段")
	}
}
