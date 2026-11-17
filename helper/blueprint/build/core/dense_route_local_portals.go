package build

// 文件职责：在固定半格局部区域中搜索完整端口短链，供源前缀和目标后缀共用。
// 使用：原primary、半格及合流均失败之后；边界：所有预算仅限制新增局部尝试，不扩大主路A*、root时限或物理权限；预算触及不是无解证明。

import (
	"context"
	"math"
	"sort"
)

// 局部预算职责：约束完整路径状态、分层前沿和交接候选；使用：所有端点共同遵守；边界：不是额外主路展开额度。
const (
	// denseLocalPortalNodes 职责：限制完整短链为八点；使用：两端局部状态；边界：不代表原生带口权限可超过原三跳。
	denseLocalPortalNodes = 8
	// denseLocalPortalStates 职责：限制每端局部展开数；使用：有限搜索；边界：不增加主路展开额度。
	denseLocalPortalStates = 512
	// denseLocalPortalFrontier 职责：限制每层完整路径前沿；使用：有限内存调度；边界：裁减表示未知。
	denseLocalPortalFrontier = 64
	// denseLocalPortalPending 职责：限制待排序状态；使用：构造下一层前沿；边界：不按出口合并身份。
	denseLocalPortalPending = 512
	// denseLocalPortalCandidates 职责：限制每端输出候选；使用：主路交接；边界：不同前缀分别占用候选名额。
	denseLocalPortalCandidates = 64
	// denseLocalPortalPerHeight 职责：给四个相对高度组保留机会；使用：候选调度；边界：排序不是可达证明。
	denseLocalPortalPerHeight = 16
	// denseLocalPortalTrials 职责：限制完整前缀试探数；使用：共用备用出口；边界：每次仍走原A*预算。
	denseLocalPortalTrials = 32
	// 局部半格包络职责：XY两格、向下一层和向上四层；使用：相对端点裁界；边界：与全局32高度不同。
	denseLocalPortalHalfXY   = 4
	denseLocalPortalHalfDown = 2
	denseLocalPortalHalfUp   = 8
)

// denseLocalPortalKey 职责：保留完整有序前缀身份；使用：状态和候选精确去重；边界：绝不按最后出口或近似屏蔽集合合并不同路径。
type denseLocalPortalKey struct {
	points [denseLocalPortalNodes]vector
	count  int
}

// denseLocalPortalCell 职责：记录相对真实端点的半格坐标；使用：局部边界与前沿调度；边界：只调度，不作为路径身份键。
type denseLocalPortalCell struct{ x, y, z int }

// denseLocalPortalState 职责：持有一条经过完整逐段守卫的路径；使用：分层有界搜索；边界：同一cell允许多个不同key。
type denseLocalPortalState struct {
	key  denseLocalPortalKey
	cell denseLocalPortalCell
	cost float64
}

// denseLocalPortalStats 职责：区分有限探索结果和真正主路展开；使用：测试与独立报告；边界：不计入或修改主路A*额度。
type denseLocalPortalStats struct {
	Expanded, Checked, Candidates, FrontierPruned, CandidatePruned int
	BudgetHit, Canceled                                            bool
}

// denseLocalPortalKeyLess 职责：为相同代价路径提供确定性次序；使用：有限前沿和候选排序；边界：不以出口相同认定路径等价。
func denseLocalPortalKeyLess(a, b denseLocalPortalKey) bool {
	if a.count != b.count {
		return a.count < b.count
	}
	for i := 0; i < a.count; i++ {
		x, y := a.points[i], b.points[i]
		if x.x != y.x {
			return x.x < y.x
		}
		if x.y != y.y {
			return x.y < y.y
		}
		if x.z != y.z {
			return x.z < y.z
		}
	}
	return false
}

// denseLocalPortalOffsets 职责：给半格格点提供固定邻接；使用：两端同一局部图；边界：整格轴步也必须逐段通过原2.15米和自距检查。
func denseLocalPortalOffsets() []denseLocalPortalCell {
	out := []denseLocalPortalCell{}
	// 局部箱直径至多八个半格；由同一world和原guard筛去过短或超过2.15m的轴步，不能把固定半格等同真实最小带距。
	for step := 1; step <= 8; step++ {
		out = append(out, denseLocalPortalCell{0, 0, step}, denseLocalPortalCell{0, -step, 0}, denseLocalPortalCell{0, step, 0}, denseLocalPortalCell{-step, 0, 0}, denseLocalPortalCell{step, 0, 0}, denseLocalPortalCell{0, 0, -step})
	}
	for z := 1; z >= -1; z-- {
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				if absInt(x)+absInt(y)+absInt(z) > 1 {
					out = append(out, denseLocalPortalCell{x, y, z})
				}
			}
		}
	}
	return out
}

// denseLocalPortals 职责：搜索半格真实短链并保留不同屏蔽身份的有限出口。
// 使用：sourceSide决定原生端口许可方向，fixedGroundStem只沿用旧目标候选；边界：XY仅各两格、向下至多一层/向上四层（并仍受全局32层约束），完整链至多八点；前沿或候选裁减只表示未知。
func (env *transportEnvironment) denseLocalPortals(ctx context.Context, endpoint, source, target *placed, preferred vector, sourceSide, fixedGroundStem bool) ([]denseRoutePortal, denseLocalPortalStats) {
	stats := denseLocalPortalStats{}
	if ctx.Err() != nil {
		stats.Canceled = true
		return nil, stats
	}
	if env.densePorts == nil || endpoint == nil || fixedGroundStem {
		return nil, stats
	}
	idx := env.densePorts
	linked := idx.linkedPortBuildings(endpoint, sourceSide)
	outward := idx.outward(endpoint, preferred, sourceSide)
	base := denseLocalPortalState{key: denseLocalPortalKey{count: 1}}
	base.key.points[0] = endpoint.a
	origin := base.key.points[base.key.count-1]
	offsets := denseLocalPortalOffsets()
	frontier := []denseLocalPortalState{base}
	expanded := map[denseLocalPortalKey]bool{}
	visits := map[denseLocalPortalCell]int{}
	buckets := [4][]denseLocalPortalState{}
	selected := map[denseLocalPortalKey]bool{}
	progress := func(s denseLocalPortalState) float64 {
		p := s.key.points[s.key.count-1]
		d := idx.world(p).Sub(idx.world(endpoint.a))
		return d.Dot(d)
	}
	better := func(a, b denseLocalPortalState) bool {
		pa, pb := progress(a), progress(b)
		if math.Abs(pa-pb) > 1e-9 {
			return pa > pb
		}
		if math.Abs(a.cost-b.cost) > 1e-9 {
			return a.cost < b.cost
		}
		return denseLocalPortalKeyLess(a.key, b.key)
	}
	keep := func(s denseLocalPortalState) {
		if selected[s.key] {
			return
		}
		h := endpoint.a.z
		for _, p := range s.key.points[:s.key.count] {
			h = max(h, p.z)
		}
		bucket := min(3, max(0, int(math.Ceil(h-endpoint.a.z))))
		values := buckets[bucket]
		if len(values) < denseLocalPortalPerHeight {
			buckets[bucket] = append(values, s)
			selected[s.key] = true
			return
		}
		counts := map[vector]int{}
		for _, v := range values {
			counts[v.key.points[v.key.count-1]]++
		}
		worst := 0
		for i := 1; i < len(values); i++ {
			a, b := values[i], values[worst]
			ca, cb := counts[a.key.points[a.key.count-1]], counts[b.key.points[b.key.count-1]]
			if ca > cb || ca == cb && better(b, a) {
				worst = i
			}
		}
		old := values[worst]
		oldPoint, newPoint := old.key.points[old.key.count-1], s.key.points[s.key.count-1]
		if counts[newPoint] < counts[oldPoint] || counts[newPoint] == counts[oldPoint] && better(s, old) {
			delete(selected, old.key)
			values[worst] = s
			selected[s.key] = true
			buckets[bucket] = values
		}
		stats.CandidatePruned++
		stats.BudgetHit = true
	}
	clear := func(s denseLocalPortalState, p vector) bool {
		stats.Checked++
		if s.key.count == base.key.count && !fixedGroundStem && (p.x-origin.x)*outward.x+(p.y-origin.y)*outward.y < -1e-6 {
			return false
		}
		return env.densePortalPointClear(s.key.points[:s.key.count], p, source, target, linked)
	}
	extend := func(s denseLocalPortalState, p vector) denseLocalPortalState {
		d := idx.world(p).Sub(idx.world(s.key.points[s.key.count-1]))
		s.cost += math.Sqrt(d.Dot(d))
		s.key.points[s.key.count] = p
		s.key.count++
		return s
	}
	for len(frontier) > 0 && stats.Expanded < denseLocalPortalStates {
		next := []denseLocalPortalState{}
		for _, s := range frontier {
			if ctx.Err() != nil {
				stats.Canceled = true
				return nil, stats
			}
			if stats.Expanded >= denseLocalPortalStates {
				stats.BudgetHit = true
				break
			}
			if expanded[s.key] {
				continue
			}
			expanded[s.key] = true
			stats.Expanded++
			visits[s.cell]++
			last := s.key.points[s.key.count-1]
			if s.key.count > base.key.count && denseIntegerNode(last) {
				keep(s)
			}
			if s.key.count < denseLocalPortalNodes {
				for z := math.Floor(last.z); z <= min(float64(transportMaxHeight), math.Ceil(last.z)+1); z++ {
					for dx := -1.; dx <= 1; dx++ {
						for dy := -1.; dy <= 1; dy++ {
							if ctx.Err() != nil {
								stats.Canceled = true
								return nil, stats
							}
							p := vector{math.Round(last.x) + dx, math.Round(last.y) + dy, z}
							if clear(s, p) {
								keep(extend(s, p))
							}
						}
					}
				}
			}
		}
		for _, d := range offsets {
			for _, s := range frontier {
				if ctx.Err() != nil {
					stats.Canceled = true
					return nil, stats
				}
				if s.key.count >= denseLocalPortalNodes-1 {
					continue
				}
				c := denseLocalPortalCell{s.cell.x + d.x, s.cell.y + d.y, s.cell.z + d.z}
				if absInt(c.x) > denseLocalPortalHalfXY || absInt(c.y) > denseLocalPortalHalfXY || c.z < -denseLocalPortalHalfDown || c.z > denseLocalPortalHalfUp {
					continue
				}
				p := vector{origin.x + float64(c.x)*.5, origin.y + float64(c.y)*.5, origin.z + float64(c.z)*.5}
				if !clear(s, p) {
					continue
				}
				v := extend(s, p)
				v.cell = c
				if len(next) < denseLocalPortalPending {
					next = append(next, v)
				} else {
					stats.FrontierPruned++
					stats.BudgetHit = true
				}
			}
		}
		sort.SliceStable(next, func(i, j int) bool {
			a, b := next[i], next[j]
			if visits[a.cell] != visits[b.cell] {
				return visits[a.cell] < visits[b.cell]
			}
			pa, pb := absInt(a.cell.x)+absInt(a.cell.y)+max(0, a.cell.z), absInt(b.cell.x)+absInt(b.cell.y)+max(0, b.cell.z)
			if pa != pb {
				return pa > pb
			}
			if math.Abs(a.cost-b.cost) > 1e-9 {
				return a.cost < b.cost
			}
			return denseLocalPortalKeyLess(a.key, b.key)
		})
		if len(next) > denseLocalPortalFrontier {
			stats.FrontierPruned += len(next) - denseLocalPortalFrontier
			stats.BudgetHit = true
			next = next[:denseLocalPortalFrontier]
		}
		frontier = next
	}
	if len(frontier) > 0 {
		stats.BudgetHit = true
	}
	for i := range buckets {
		sort.SliceStable(buckets[i], func(a, b int) bool { return better(buckets[i][a], buckets[i][b]) })
	}
	portals := []denseRoutePortal{}
	for n := 0; n < denseLocalPortalPerHeight; n++ {
		for h := range buckets {
			if n >= len(buckets[h]) {
				continue
			}
			s := buckets[h][n]
			points := append([]vector(nil), s.key.points[:s.key.count]...)
			height := endpoint.a.z
			for _, p := range points {
				height = max(height, p.z)
			}
			portals = append(portals, denseRoutePortal{points: points, height: height, cost: s.cost})
		}
	}
	if ctx.Err() != nil {
		stats.Canceled = true
		return nil, stats
	}
	stats.Candidates = len(portals)
	return portals, stats
}
