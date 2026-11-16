package build

// 文件职责：穷举比较三项局部路由优化与优化前的查询；使用：仅运行微型内存几何；边界：不执行全厂生成、A*或改变任何预算。

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// routeMinimalFrames 职责：覆盖平面、赤道及南北球面；使用：差分矩阵；边界：保持原生 float32 变换。
func routeMinimalFrames() []*densePolarFrame {
	frames := []*densePolarFrame{nil}
	for _, start := range []int{0, 180, -180} {
		frames = append(frames, &densePolarFrame{start: start, height: 100, segments: 200, latitudeStep: float32(math.Pi / 500), longitudeStep: float32(math.Pi / 500)})
	}
	return frames
}

// routeMinimalIndex 职责：创建独立微型路由索引；使用：差分；边界：不加载量化或全厂布局。
func routeMinimalIndex(frame *densePolarFrame) *denseRoutePortIndex {
	return &denseRoutePortIndex{polarFrame: frame, strictBelts: true,
		cells: map[[3]int][]int{}, nodes: map[[3]int][]*placed{}, previous: map[*placed][]*placed{},
		edges: map[[2]vector]bool{}, registered: map[*placed]bool{}, beltCells: map[[3]int][]int{}, beltEdges: map[[2]*placed]bool{}}
}

// routeMinimalNode 职责：创建带节点；使用：直接登记；边界：只使用内存坐标与原型身份。
func routeMinimalNode(p vector) *placed {
	return &placed{a: p, record: blueprint.Building{ItemID: 2001}}
}

// TestDenseRouteMinimalBeltDifferential 职责：穷举带段宽相位前后结果；使用：多方向、跨桶、退化段及阈值邻点；边界：保留原端点豁免、顺序和精确公式。
func TestDenseRouteMinimalBeltDifferential(t *testing.T) {
	threshold := math.Sqrt(.46*.46-1e-6) / planarGridLength
	offsets := []float64{-1, -.5, 0, math.Nextafter(threshold, 0), threshold, math.Nextafter(threshold, math.Inf(1)), .5, 1}
	directions := []vector{{}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}, {1, -1, 0}, {0, 0, 1}, {1, 0, 1}}
	checks, blocked, clear := 0, 0, 0
	for frameID, frame := range routeMinimalFrames() {
		for _, origin := range []vector{{}, {3.99, 3.99, 0}, {-4.01, -4.01, .5}} {
			for _, direction := range directions {
				index := routeMinimalIndex(frame)
				first := routeMinimalNode(origin)
				first.output = routeMinimalNode(vector{origin.x + direction.x, origin.y + direction.y, origin.z + direction.z})
				index.addBeltSpan(first)
				index.addBeltSpan(first)
				if len(index.belts) != 1 || len(index.beltBounds) != 1 {
					t.Fatal("重复登记或包围盒不同步")
				}
				for _, dx := range offsets {
					for _, dy := range offsets {
						for _, dz := range []float64{0, .25, .5, 1} {
							for _, delta := range directions {
								a := vector{origin.x + dx, origin.y + dy, origin.z + dz}
								b := vector{a.x + delta.x, a.y + delta.y, a.z + delta.z}
								want, got := index.routeMinimalLegacyBeltSpanClear(a, b), index.beltSpanClear(a, b)
								if got != want {
									t.Fatalf("frame=%d existing=%v→%v query=%v→%v got=%v want=%v", frameID, origin, first.output.a, a, b, got, want)
								}
								checks++
								if got {
									clear++
								} else {
									blocked++
								}
							}
						}
					}
				}
			}
		}
	}
	if blocked == 0 || clear == 0 {
		t.Fatal("差分矩阵未覆盖允许和阻塞")
	}
	t.Logf("带段差分%d次，阻塞%d，允许%d", checks, blocked, clear)
}

// TestDenseRouteMinimalNodeDifferential 职责：穷举节点缓存与旧桶扫描；使用：预存及外部直接登记、身份豁免、增量登记和坐标更新；边界：节点桶及原判定顺序保持不变。
func TestDenseRouteMinimalNodeDifferential(t *testing.T) {
	threshold := math.Sqrt(.35*.35-1e-9) / planarGridLength
	offsets := []float64{-1, -.5, -threshold, 0, math.Nextafter(threshold, 0), threshold, math.Nextafter(threshold, math.Inf(1)), .5, 1}
	checks := 0
	for frameID, frame := range routeMinimalFrames() {
		for _, direct := range []bool{false, true} {
			index := routeMinimalIndex(frame)
			nodes := []*placed{}
			for _, x := range []float64{-4.01, 0, 3.99} {
				for _, y := range []float64{-4.01, 0, 3.99} {
					for _, z := range []float64{0, .5} {
						node := routeMinimalNode(vector{x, y, z})
						nodes = append(nodes, node)
						if direct {
							cell := denseRouteNodeCell(index.world(node.a))
							index.nodes[cell] = append(index.nodes[cell], node)
						} else {
							index.addNode(node)
							index.addNode(node)
						}
					}
				}
			}
			for _, node := range nodes {
				for _, dx := range offsets {
					for _, dy := range offsets {
						for _, dz := range []float64{0, .1, .25, .5} {
							point := vector{node.a.x + dx, node.a.y + dy, node.a.z + dz}
							for _, reused := range []*placed{nil, node, routeMinimalNode(node.a)} {
								want, got := index.routeMinimalLegacyNodeClear(point, reused), index.nodeClear(point, reused)
								if got != want {
									t.Fatalf("frame=%d direct=%v node=%v point=%v got=%v want=%v", frameID, direct, node.a, point, got, want)
								}
								checks++
							}
						}
					}
				}
			}
			// 在同一桶内更新位置，验证缓存位置校验；外部直写桶无需改跨文件类型。
			node := nodes[0]
			old := node.a
			node.a.x += .01
			for _, point := range []vector{old, node.a, {old.x + .5, old.y, old.z}} {
				if index.nodeClear(point, nil) != index.routeMinimalLegacyNodeClear(point, nil) {
					t.Fatal("节点更新后缓存结论不同")
				}
				checks++
			}
			added := routeMinimalNode(vector{1, 1, 1})
			index.addNode(added)
			if index.nodeClear(added.a, nil) || !index.nodeClear(added.a, added) {
				t.Fatal("增量登记或身份豁免变化")
			}
		}
	}
	t.Logf("节点差分%d次", checks)
}

// TestDenseRouteMinimalPortalsDifferential 职责：逐项比较完整 portal 候选、顺序、代价和路径；使用：所有有限组合及原完整高度；边界：不调用 findDenseTransportPath 或 A*。
func TestDenseRouteMinimalPortalsDifferential(t *testing.T) {
	checks, nonempty, empty := 0, 0, 0
	for frameID, frame := range routeMinimalFrames() {
		for _, endpointPoint := range []vector{{}, {.25, .5, 0}, {1, 1, .5}} {
			for _, sourceSide := range []bool{false, true} {
				for _, fixed := range []bool{false, true} {
					for _, preferred := range []vector{{-1, 0, 0}, {0, 1, 0}} {
						for _, height := range []float64{0, 1, 2, 4} {
							for scenario := 0; scenario < 3; scenario++ {
								index := routeMinimalIndex(frame)
								endpoint := routeMinimalNode(endpointPoint)
								index.addNode(endpoint)
								env := &transportEnvironment{densePorts: index, minimum: vector{-3, -3, 0}, maximum: vector{6, 6, transportMaxHeight}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
								if scenario == 1 {
									obstacle := routeMinimalNode(vector{endpointPoint.x - 1, endpointPoint.y, endpointPoint.z})
									index.addNode(obstacle)
									env.reserved[vector{1, 1, 1}] = map[*placed]bool{routeMinimalNode(vector{}): true}
								} else if scenario == 2 {
									index.boundary = &denseBoundaryLayout{minimumX: -3, maximumX: 2, levelLimitX: 0, minimumY: -3, maximumY: 6}
									belt := routeMinimalNode(vector{-1, -1, 0})
									belt.output = routeMinimalNode(vector{-1, 2, 0})
									index.addNode(belt)
									index.addNode(belt.output)
								}
								want := env.routeMinimalLegacyPortals(endpoint, endpoint, endpoint, preferred, sourceSide, fixed, height)
								got := env.densePortals(endpoint, endpoint, endpoint, preferred, sourceSide, fixed, height)
								if !reflect.DeepEqual(got, want) {
									t.Fatalf("frame=%d endpoint=%v source=%v fixed=%v preferred=%v height=%v scenario=%d\ngot=%v\nwant=%v", frameID, endpointPoint, sourceSide, fixed, preferred, height, scenario, got, want)
								}
								checks++
								if len(got) == 0 {
									empty++
								} else {
									nonempty++
								}
							}
						}
					}
				}
			}
		}
		index := routeMinimalIndex(frame)
		endpoint := routeMinimalNode(vector{.25, .5, 0})
		index.addNode(endpoint)
		env := &transportEnvironment{densePorts: index, minimum: vector{-3, -3, 0}, maximum: vector{6, 6, transportMaxHeight}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
		if !reflect.DeepEqual(env.densePortals(endpoint, endpoint, endpoint, vector{-1, 0, 0}, true, false), env.routeMinimalLegacyPortals(endpoint, endpoint, endpoint, vector{-1, 0, 0}, true, false)) {
			t.Fatalf("frame=%d 完整32层候选变化", frameID)
		}
		checks++
	}
	if nonempty == 0 || empty == 0 {
		t.Fatal("portal矩阵未覆盖空候选和非空候选")
	}
	t.Logf("完整portal候选差分%d组，有限矩阵非空%d组，空%d组", checks, nonempty, empty)
}

// 以下三个参考函数保留冻结源码7cf85b67中的未优化查询；仅改测试名称及参考节点调用，不跟随优化入口重写几何判定。
func (index *denseRoutePortIndex) routeMinimalLegacyBeltSpanClear(first, second vector) bool {
	if !index.strictBelts && !index.boundarySegment(first, second) {
		return true
	}
	a, c := index.world(first), index.world(second)
	candidate := beltPackingShape(a, c)
	index.beltQuery.begin(len(index.belts))
	clear := packingShapeCells(candidate, func(cell [3]int) bool {
		for _, number := range index.beltCells[cell] {
			if index.beltQuery.seen(number) {
				continue
			}
			existing := index.belts[number]
			if a == existing.ends[0] || a == existing.ends[1] || c == existing.ends[0] || c == existing.ends[1] {
				continue
			}
			if surfaceSpanDistanceSquared(candidate.span, existing) < .46*.46-1e-6 {
				return false
			}
		}
		return true
	})
	return clear
}

func (index *denseRoutePortIndex) routeMinimalLegacyNodeClear(p vector, reused *placed) bool {
	world := index.world(p)
	padding := placement.Vector{denseRouteNodeClearance, denseRouteNodeClearance, denseRouteNodeClearance}
	low, high := denseRouteNodeCell(world.Sub(padding)), denseRouteNodeCell(world.Add(padding))
	for x := low[0]; x <= high[0]; x++ {
		for y := low[1]; y <= high[1]; y++ {
			for z := low[2]; z <= high[2]; z++ {
				for _, existing := range index.nodes[[3]int{x, y, z}] {
					if existing == reused && existing.a == p {
						continue
					}
					delta := world.Sub(index.world(existing.a))
					if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
						return false
					}
				}
			}
		}
	}
	return true
}

func (env *transportEnvironment) routeMinimalLegacyPortals(endpoint, source, target *placed, preferred vector, sourceSide, fixedGroundStem bool, heightLimits ...float64) []denseRoutePortal {
	index := env.densePorts
	maximumHeight := float64(transportMaxHeight)
	if len(heightLimits) > 0 {
		maximumHeight = math.Min(maximumHeight, heightLimits[0])
	}
	outward := index.outward(endpoint, preferred, sourceSide)
	linked := index.linkedPortBuildings(endpoint, sourceSide)
	base := []vector{endpoint.a}
	if fixedGroundStem {
		stem := vector{endpoint.a.x + 1, endpoint.a.y, endpoint.a.z}
		if !index.routeMinimalLegacyNodeClear(stem, nil) || !index.spanClear(endpoint.a, stem) || env.reservedByOther(stem, source, target) {
			return nil
		}
		base = append(base, stem)
		outward = vector{x: 1}
	}
	inside := func(p vector) bool {
		return p.x >= env.minimum.x && p.x <= env.maximum.x && p.y >= env.minimum.y && p.y <= env.maximum.y && p.z >= 0 && p.z <= maximumHeight
	}
	clearNew := func(path []vector, p vector) bool {
		permissions := densePortalLinkPermissions(linked, len(path)-1, len(path))
		if !inside(p) || !index.routeMinimalLegacyNodeClear(p, nil) || env.reservedByOther(p, source, target) || !index.spanClearWithLinks(path[len(path)-1], p, permissions) {
			return false
		}
		for _, previous := range path {
			delta := index.world(previous).Sub(index.world(p))
			if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
				return false
			}
		}
		return true
	}
	byExit := map[vector]denseRoutePortal{}
	appendCandidate := func(path []vector, p vector) {
		if !denseIntegerNode(p) || !clearNew(path, p) {
			return
		}
		if len(path) == 1 && (p.x-endpoint.a.x)*outward.x+(p.y-endpoint.a.y)*outward.y < -1e-6 {
			return
		}
		delta := denseRouteWorld(p).Sub(denseRouteWorld(path[len(path)-1]))
		candidate := denseRoutePortal{cost: denseRouteLength(path) + math.Sqrt(delta.Dot(delta)), height: p.z}
		for _, point := range path {
			candidate.height = math.Max(candidate.height, point.z)
		}
		old, exists := byExit[p]
		if !exists || candidate.cost < old.cost-1e-9 || math.Abs(candidate.cost-old.cost) <= 1e-9 && len(path)+1 < len(old.points) {
			// 先比较完整代价，仅为真正获选的出口分配一次路径。
			candidate.points = make([]vector, len(path)+1)
			copy(candidate.points, path)
			candidate.points[len(path)] = p
			byExit[p] = candidate
		}
	}
	last := base[len(base)-1]
	// 老整数口优先保持原有短接，分数口允许邻近整格，但不能退回低轨内部。
	appendCandidate(base, vector{math.Round(last.x + outward.x), math.Round(last.y + outward.y), math.Round(last.z)})
	stems := [][]vector{base}
	if !fixedGroundStem {
		for _, length := range []float64{.5, 1} {
			stem := vector{last.x + outward.x*length, last.y + outward.y*length, last.z}
			if clearNew(base, stem) {
				stems = append(stems, append(append([]vector{}, base...), stem))
			}
		}
	}
	for _, stem := range stems {
		origin := stem[len(stem)-1]
		reachable := [][]vector{stem}
		for z := math.Ceil(origin.z*2) / 2; z <= maximumHeight; z += .5 {
			path := stem
			if z > origin.z+1e-9 {
				point := vector{origin.x, origin.y, z}
				path = nil
				for i := len(reachable) - 1; i >= 0; i-- {
					previous := reachable[i]
					if z-previous[len(previous)-1].z > denseRouteMaximumSpan/1.3333333 {
						break
					}
					if clearNew(previous, point) && (path == nil || len(previous)+1 < len(path)) {
						path = append(append([]vector{}, previous...), point)
					}
				}
				if path == nil {
					continue
				}
				reachable = append(reachable, path)
			}
			point := path[len(path)-1]
			if denseIntegerNode(point) && len(path) > 1 {
				candidate := denseRoutePortal{points: path, cost: denseRouteLength(path), height: point.z}
				if old, exists := byExit[point]; !exists || candidate.cost < old.cost {
					byExit[point] = candidate
				}
			}
			for dx := -1.; dx <= 1; dx++ {
				for dy := -1.; dy <= 1; dy++ {
					appendCandidate(path, vector{math.Round(point.x) + dx, math.Round(point.y) + dy, math.Round(z)})
				}
			}
		}
	}
	portals := make([]denseRoutePortal, 0, len(byExit))
	for _, portal := range byExit {
		portals = append(portals, portal)
	}
	sort.Slice(portals, func(i, j int) bool {
		a, c := portals[i], portals[j]
		if a.height != c.height {
			return a.height < c.height
		}
		if math.Abs(a.cost-c.cost) > 1e-9 {
			return a.cost < c.cost
		}
		if len(a.points) != len(c.points) {
			return len(a.points) < len(c.points)
		}
		x, y := a.points[len(a.points)-1], c.points[len(c.points)-1]
		// 外供接口同距离先保留直出，避免被两条侧移挤掉形成 U 形绕路；厂内接驳及目标端保留原有候选次序。
		if sourceSide && index.boundarySegment(endpoint.a, endpoint.a) {
			forwardA := (x.x-endpoint.a.x)*outward.x + (x.y-endpoint.a.y)*outward.y
			forwardC := (y.x-endpoint.a.x)*outward.x + (y.y-endpoint.a.y)*outward.y
			if math.Abs(forwardA-forwardC) > 1e-6 {
				return forwardA > forwardC
			}
		}
		if x.x != y.x {
			return x.x < y.x
		}
		if x.y != y.y {
			return x.y < y.y
		}
		return x.z < y.z
	})
	limited := make([]denseRoutePortal, 0, 2*(transportMaxHeight+1))
	levels := map[int]int{}
	for _, portal := range portals {
		level := int(math.Ceil(portal.height))
		if levels[level] < 2 {
			limited = append(limited, portal)
			levels[level]++
		}
	}
	return limited
}
