package saveanalysis

// 文件职责：判读生产设备保存时刻的缓冲与工作状态，追踪本地物料连接。
// 使用：处理快照产线查询；边界：可达端点和检查建议均不能证明物品的实际流量。

import (
	"fmt"
	"slices"
	"sort"
)

// factoryLine 在解析厂区时保存有方向的实体连接以及制造台上次更新的状态。
type factoryLine struct {
	drops           map[int64]DropObservation // 有实载的分拣器保存状态，按实体查询。
	veinStations    map[int64]bool
	devicePorts     map[int64][]linePort // 仅在特殊设备查询的只读视图中建立。
	devices         []deviceRecord
	veins           map[int64]veinSource
	currentTech     int64
	researchPending bool
	machines        []circuitRecord
	sorters         []lineSorter
	beltByEntity    map[int64]int64
	belts           map[int64]lineBelt
	sign            []byte    // 实体 i 的标志位于 i-1
	power           []float32 // 动画池含 0 号位置
	protos          map[int64]int64
	ports           map[int64][]linePort
}
type linePort struct{ entity, dir, item int64 }
type lineBelt struct{ entity, output int64 }
type lineSorter struct {
	entity, pick, drop, filter int64
	reversible                 bool
}
type circuitRecord struct {
	id, entity, recipe, kind int64
	active                   bool
	progress                 int64
	requires, served         []int64
	needs                    []int64 // 保存时的物品请求。
	products, produced       []int64
}

func newFactoryLine() *factoryLine {
	return &factoryLine{veins: map[int64]veinSource{}, beltByEntity: map[int64]int64{}, belts: map[int64]lineBelt{}, protos: map[int64]int64{}, ports: map[int64][]linePort{}}
}
func (f *factoryLine) addBelt(id, entity, out int64) {
	f.belts[id] = lineBelt{entity, out}
	f.beltByEntity[entity] = id
}
func (r *reader) lineValues(n, size int64) []int64 {
	v := make([]int64, n)
	for i := range v {
		v[i] = r.number(size, true)
	}
	return v
}

// CircuitReport 描述所选星球保存时刻的配方生产设备及可达路径。连接仅表示物理可达。
type CircuitReport struct {
	DeviceDiagnostics *DeviceReport    `json:"deviceDiagnostics,omitempty"`
	ItemID            int64            `json:"itemId"`
	Coverage          string           `json:"coverage"`
	SHA256            string           `json:"sha256"`
	Tick              int64            `json:"tick"`
	PlanetID          int64            `json:"planetId"`
	Machines          []CircuitMachine `json:"machines"`
	Findings          []CircuitFinding `json:"findings"`
	Routes            []CircuitRoute   `json:"routes"`
	// Neighbors 保留可达端点中已核实配方的相邻设备证据，按实体去重。
	Neighbors []CircuitMachine `json:"neighbors"`
}

// CircuitMachine 定位单台制造设备，保留保存时刻的缓冲和运行证据。
type CircuitMachine struct {
	Building           int64               `json:"building"`
	RecipeMatched      bool                `json:"recipeMatched"`
	CycleProgressLimit int64               `json:"cycleProgressLimit,omitempty"`
	Outputs            []ProductionOutput  `json:"outputs"`
	AssemblerID        int64               `json:"assemblerId"`
	EntityID           int64               `json:"entityId"`
	RecipeID           int64               `json:"recipeId"`
	Replicating        bool                `json:"replicating"`
	Progress           int64               `json:"progress"`
	Produced           int64               `json:"produced"`
	Sign               int                 `json:"sign"`
	Power              float32             `json:"power"`
	Inputs             []CircuitInput      `json:"inputs"`
	Status             string              `json:"status"`
	Advice             string              `json:"advice"`
	Connections        []CircuitConnection `json:"connections"`
}

// ProductionOutput 保存每种成品的件数，以及按当前配方核实的交付门槛。
type ProductionOutput struct {
	ItemID        int64 `json:"itemId"`
	Buffered      int64 `json:"buffered"`
	Batch         int64 `json:"batch,omitempty"`
	DeliveryLimit int64 `json:"deliveryLimit,omitempty"`
}

// CircuitInput 记录本批配方输入及制造台内的可用数量。
type CircuitInput struct {
	ItemID    int64 `json:"itemId"`
	Available int64 `json:"available"`
	Required  int64 `json:"required,omitempty"`
}

// CircuitConnection 记录直接连接的分拣器，路线索引指向本报告内同物品的可达带段。
type CircuitConnection struct {
	SorterEntity  int64  `json:"sorterEntity"`
	OtherEntity   int64  `json:"otherEntity"`
	Filter        int64  `json:"filter"`
	Direction     string `json:"direction"`
	Reversible    bool   `json:"reversible"`
	Routes        []int  `json:"routes"`
	OtherBuilding int64  `json:"otherBuilding"`
}

// CircuitRoute 记录从一个带段向物料来源或去向追踪的结果。共享带上的端点不能归为专用供应方。
type CircuitRoute struct {
	StartBelt int64             `json:"startBelt"`
	ItemID    int64             `json:"itemId"`
	Direction string            `json:"direction"`
	Belts     []int64           `json:"belts"`
	Endpoints []CircuitEndpoint `json:"endpoints"`
	Transfers []CircuitEndpoint `json:"transfers"`
	Shared    bool              `json:"shared"`
	OpenEnds  []int64           `json:"openEnds"`
}

// CircuitEndpoint 标记可能收发货的邻接实体或中转分拣器，过滤器为零表示未指定物品。
type CircuitEndpoint struct {
	Belt         int64 `json:"belt"`
	SorterEntity int64 `json:"sorterEntity"`
	EntityID     int64 `json:"entityId"`
	Building     int64 `json:"building"`
	Filter       int64 `json:"filter"`
}

func (f *factoryLine) diagnose(a circuitRecord, version string, selected int64) CircuitMachine {
	m := CircuitMachine{AssemblerID: a.id, EntityID: a.entity, Building: f.protos[a.entity], RecipeID: a.recipe, Replicating: a.active, Progress: a.progress, Sign: int(f.sign[a.entity-1]), Power: f.power[a.entity]}
	recipe, ok := productionRecipes[a.recipe]
	m.RecipeMatched = ok && version == productionRecipeVersion && recipe.kind == a.kind && slices.Equal(a.requires, recipe.inputs) && slices.Equal(a.products, recipe.products)
	if m.RecipeMatched {
		m.CycleProgressLimit = recipe.ticks * 10000
	}
	outputFull := false
	for i, id := range a.products {
		v := ProductionOutput{ItemID: id, Buffered: a.produced[i]}
		if id == selected || (selected == 0 && i == 0) {
			m.Produced = v.Buffered
		}
		if m.RecipeMatched {
			v.Batch = recipe.batches[i]
			switch a.kind {
			case 1:
				v.DeliveryLimit = 100 - v.Batch
			case 4:
				v.DeliveryLimit = 9 * v.Batch
			default:
				v.DeliveryLimit = 19 * v.Batch
			}
			outputFull = outputFull || v.Buffered > v.DeliveryLimit
		}
		m.Outputs = append(m.Outputs, v)
	}
	for i, id := range a.requires {
		input := CircuitInput{ItemID: id, Available: a.served[i]}
		if m.RecipeMatched {
			input.Required = recipe.counts[i]
		}
		m.Inputs = append(m.Inputs, input)
	}
	m.Status = "undetermined"
	m.Advice = "检查设备状态及实际配方，现有证据不足以确定限制环节"
	switch {
	case m.Power < 0.1:
		m.Status = "low-power"
		m.Advice = "优先核查本机所属电网的供电"
	case a.active && m.Sign == 0:
		m.Status = "processing"
		m.Advice = "当前已进入加工周期，无需因下批原料不足调整供料"
	case m.Sign == 6 && !a.active && a.progress == 0:
		for _, v := range m.Inputs {
			if v.Required > 0 && v.Available < v.Required {
				m.Status = "input-missing"
				m.Advice = "先查缺料输入的分拣器与上游带段，再核实共享带的实际来料"
				break
			}
		}
	case m.Sign == 6 && !a.active && m.RecipeMatched && a.progress >= m.CycleProgressLimit && outputFull:
		m.Status = "output-blocked"
		m.Advice = "先查出料分拣器和接收端能否继续收货"
	}
	return m
}

// CircuitLine 职责：分析给定星球的电路板产线；使用：查询同一快照；边界：只报告已保存的状态及带段连接。
func (s *Snapshot) CircuitLine(planet int64) (CircuitReport, error) {
	return s.productionLine(planet, 1301)
}

func (s *Snapshot) productionLine(planet, item int64) (CircuitReport, error) {
	result := CircuitReport{ItemID: item, Coverage: "逐机诊断覆盖制造台、熔炉、精炼厂、化工厂及对撞机。共享线路仅证明物理可达。", SHA256: s.digest, Tick: s.tick, PlanetID: planet, Machines: []CircuitMachine{}, Routes: []CircuitRoute{}, Findings: []CircuitFinding{}, Neighbors: []CircuitMachine{}}
	f := s.lines[planet]
	if f == nil || !slices.ContainsFunc(f.machines, func(a circuitRecord) bool { return slices.Contains(a.products, item) }) {
		result.Coverage = "未找到当前配置该产物的已覆盖设备。历史产出可能来自其他设备或已更换的配方，无法定位受阻设备。"
		for _, p := range s.worldView.Planets {
			if p.PlanetID == planet {
				return result, nil
			}
		}
		return CircuitReport{}, fmt.Errorf("未知星球 %d", planet)
	}
	attached := map[int64][]lineSorter{}
	incoming := map[int64][]int64{}
	for _, v := range f.sorters {
		attached[v.pick] = append(attached[v.pick], v)
		if v.drop != v.pick {
			attached[v.drop] = append(attached[v.drop], v)
		}
	}
	for id, b := range f.belts {
		if b.output > 0 {
			incoming[b.output] = append(incoming[b.output], id)
		}
	}
	for _, ids := range incoming {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	}
	cache := map[[3]int64]int{}
	byEntity := make(map[int64]int, len(f.machines))
	for index, a := range f.machines {
		byEntity[a.entity] = index
		if !slices.Contains(a.products, item) {
			continue
		}
		m := f.diagnose(a, s.gameVersion, item)
		for _, v := range attached[a.entity] {
			dest, dir := v.pick, "in"
			if v.pick == a.entity {
				dest, dir = v.drop, "out"
			}
			c := CircuitConnection{SorterEntity: v.entity, OtherEntity: dest, Filter: v.filter, Direction: dir, Reversible: v.reversible, OtherBuilding: f.protos[dest]}
			if belt := f.beltByEntity[dest]; belt > 0 {
				for _, direction := range []string{"in", "out"} {
					if direction != dir && !v.reversible {
						continue
					}
					items, flag := a.requires, int64(0)
					if direction == "out" {
						items, flag = a.products, 1
					}
					for _, item := range items {
						if v.filter > 0 && v.filter != item {
							continue
						}
						key := [3]int64{belt, flag, item}
						index, ok := cache[key]
						if !ok {
							index = len(result.Routes)
							cache[key] = index
							result.Routes = append(result.Routes, f.trace(belt, direction, item, incoming, attached))
						}
						c.Routes = append(c.Routes, index)
					}
				}
			}
			m.Connections = append(m.Connections, c)
		}
		result.Machines = append(result.Machines, m)
	}
	markSharedCircuitRoutes(&result)
	seenPeer := map[int64]bool{}
	addPeer := func(entity int64) {
		if i, ok := byEntity[entity]; ok && !seenPeer[entity] {
			result.Neighbors = append(result.Neighbors, f.diagnose(f.machines[i], s.gameVersion, 0))
			seenPeer[entity] = true
		}
	}
	for _, route := range result.Routes {
		for _, end := range route.Endpoints {
			addPeer(end.EntityID)
		}
	}
	for _, m := range result.Machines {
		for _, c := range m.Connections {
			addPeer(c.OtherEntity)
		}
	}
	result.Findings = prioritizeCircuit(result)
	f.addNeighborEvidence(&result, incoming, attached)
	return result, nil
}
func (f *factoryLine) trace(start int64, direction string, item int64, incoming map[int64][]int64, attached map[int64][]lineSorter) CircuitRoute {
	r := CircuitRoute{StartBelt: start, Direction: direction, ItemID: item}
	queue := []int64{start}
	seen := map[int64]bool{}
	taps := map[int64]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		b, ok := f.belts[id]
		if !ok {
			r.OpenEnds = append(r.OpenEnds, id)
			continue
		}
		r.Belts = append(r.Belts, id)
		ends := 0
		for _, v := range attached[b.entity] {
			if v.filter > 0 && v.filter != item {
				r.Shared = true // 同一物理带段还接有面向另一物品的端口。
				continue
			}
			isSource := v.drop == b.entity
			if (direction == "in") == isSource || v.reversible {
				other := v.drop
				if isSource {
					other = v.pick
				}
				ep := CircuitEndpoint{id, v.entity, other, f.protos[other], v.filter}
				if transfer := f.beltByEntity[other]; transfer > 0 {
					queue = append(queue, transfer)
					r.Transfers = append(r.Transfers, ep)
				} else {
					r.Endpoints = append(r.Endpoints, ep)
				}
				ends++
			}
			if (direction == "in") != isSource || v.reversible {
				other := v.pick
				if !isSource {
					other = v.drop
				}
				taps[other] = true
			}
		}
		for _, port := range f.devicePorts[id] {
			if port.item != item {
				r.Shared = true
				continue
			}
			if (direction == "in") == (port.dir == 1) {
				r.Endpoints = append(r.Endpoints, CircuitEndpoint{id, 0, port.entity, f.protos[port.entity], port.item})
				ends++
			} else {
				taps[port.entity] = true
			}
		}
		for _, station := range f.ports[id] {
			if direction == "in" && (station.dir != 1 || station.item != item) || direction == "out" && station.dir != 2 {
				continue
			}
			r.Endpoints = append(r.Endpoints, CircuitEndpoint{id, 0, station.entity, f.protos[station.entity], station.item})
			ends++
		}
		next := incoming[id]
		if direction == "out" {
			next = nil
			if b.output > 0 {
				next = []int64{b.output}
			}
		}
		if len(incoming[id]) > 1 {
			r.Shared = true
		}
		if len(next) == 0 && ends == 0 {
			r.OpenEnds = append(r.OpenEnds, id)
		}
		queue = append(queue, next...)
	}
	r.Shared = r.Shared || len(taps) > 1 || len(r.Endpoints) > 1

	sort.Slice(r.Endpoints, func(i, j int) bool {
		a, b := r.Endpoints[i], r.Endpoints[j]
		if a.Belt != b.Belt {
			return a.Belt < b.Belt
		}
		return a.SorterEntity < b.SorterEntity
	})
	sort.Slice(r.OpenEnds, func(i, j int) bool { return r.OpenEnds[i] < r.OpenEnds[j] })
	return r
}

// markSharedCircuitRoutes 标记不同设备或候选物品重叠的物理带段，保留混带归属的不确定性。
func markSharedCircuitRoutes(report *CircuitReport) {
	type use struct{ machine, item int64 }
	first := map[int64]use{}
	shared := map[int64]bool{}
	for _, m := range report.Machines {
		for _, c := range m.Connections {
			for _, index := range c.Routes {
				r := &report.Routes[index]
				current := use{m.EntityID, r.ItemID}
				for _, belt := range r.Belts {
					if previous, ok := first[belt]; !ok {
						first[belt] = current
					} else if previous != current {
						shared[belt] = true
					}
				}
			}
		}
	}
	for i := range report.Routes {
		r := &report.Routes[i]
		r.Shared = r.Shared || slices.ContainsFunc(r.Belts, func(belt int64) bool { return shared[belt] })
	}
}

// CircuitFinding 汇集同一物料约束涉及的设备，给出可回查的首个检查点。
// CheckBelt 表示受影响设备最多的共同带段。共享连接仅支持检查顺序，不能证明带段本身故障。
type CircuitFinding struct {
	RouteEvidence          string  `json:"routeEvidence"`
	Status                 string  `json:"status"`
	ItemID                 int64   `json:"itemId"`
	CheckBelt              int64   `json:"checkBelt"`
	Assemblers             []int64 `json:"assemblers"`
	Advice                 string  `json:"advice"`
	RelatedBlockedEntities []int64 `json:"relatedBlockedEntities"`
}

func prioritizeCircuit(report CircuitReport) []CircuitFinding {
	type key struct {
		status     string
		item, belt int64
	}
	candidates := map[key][]int64{}
	var individual []CircuitFinding
	pending := map[[2]int64]bool{}
	// 同样多的机器受影响时，优先检查沿各机线路较早到达的带段。
	visitCost := map[key]int{}
	shared := map[key]bool{}
	for _, m := range report.Machines {
		if m.Status == "processing" {
			continue
		}
		var items []int64
		switch m.Status {
		case "input-missing":
			for _, v := range m.Inputs {
				if v.Required > 0 && v.Available < v.Required {
					items = append(items, v.ItemID)
				}
			}
		case "output-blocked":
			for _, out := range m.Outputs {
				if out.Batch > 0 && out.Buffered > out.DeliveryLimit {
					items = append(items, out.ItemID)
				}
			}
		default:
			items = []int64{0}
		}
		for _, item := range items {
			visited := map[int64]int{}
			for _, c := range m.Connections {
				for _, i := range c.Routes {
					r := report.Routes[i]
					if r.ItemID != item || len(r.Endpoints) == 0 || (m.Status == "output-blocked" && r.Direction != "out") || (m.Status == "input-missing" && r.Direction != "in") {
						continue
					}
					for step, belt := range r.Belts {
						if r.Shared {
							shared[key{m.Status, item, belt}] = true
						}
						if old, ok := visited[belt]; !ok || step < old {
							visited[belt] = step
						}
					}
				}
			}
			if len(visited) == 0 {
				evidence, advice := "machine", m.Advice
				if m.Status == "input-missing" || m.Status == "output-blocked" {
					evidence, advice = sorterEvidence(m, item)
				}
				individual = append(individual, CircuitFinding{RouteEvidence: evidence, Status: m.Status, ItemID: item, Assemblers: []int64{m.AssemblerID}, Advice: advice})
				continue
			}
			pending[[2]int64{m.AssemblerID, item}] = true
			for belt, step := range visited {
				k := key{m.Status, item, belt}
				visitCost[k] += step
				candidates[k] = append(candidates[k], m.AssemblerID)
			}
		}
	}
	findings := []CircuitFinding{}
	for len(pending) > 0 {
		var best key
		var affected []int64
		for k, list := range candidates {
			var ids []int64
			for _, id := range list {
				if pending[[2]int64{id, k.item}] {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				continue
			}
			earlier := visitCost[k] < visitCost[best] || visitCost[k] == visitCost[best] && (k.belt < best.belt || k.belt == best.belt && (k.item < best.item || k.item == best.item && k.status < best.status))
			if len(ids) > len(affected) || len(ids) == len(affected) && earlier {
				best, affected = k, ids
			}
		}
		if len(affected) == 0 {
			break
		}
		evidence := "reachable"
		if shared[best] || len(affected) > 1 {
			evidence = "shared"
		}
		advice := "核查该带的实际来料及对应分拣器是否能完成下一批投料"
		if best.status == "output-blocked" {
			advice = "检查所列设备可达的成品带和接收端，核实下游是否取货，再决定是否调整出料"
		}
		findings = append(findings, CircuitFinding{RouteEvidence: evidence, Status: best.status, ItemID: best.item, CheckBelt: best.belt, Assemblers: affected, Advice: advice})
		for _, id := range affected {
			delete(pending, [2]int64{id, best.item})
		}
	}
	// 无共同带段证据时逐台给出检查位置，已知限制优先于原因未明的设备。
	sort.SliceStable(individual, func(i, j int) bool {
		return individual[i].Status != "undetermined" && individual[j].Status == "undetermined"
	})
	return append(findings, individual...)
}

// sorterEvidence 在追不到收发端时区分缺少相应方向的分拣器、过滤不符和开放线路。
func sorterEvidence(m CircuitMachine, item int64) (string, string) {
	direction, label := "in", "进料"
	if m.Status == "output-blocked" {
		direction, label = "out", "出料"
	}
	hasDirection := false
	for _, c := range m.Connections {
		if c.Direction != direction && !c.Reversible {
			continue
		}
		hasDirection = true
		if c.Filter == 0 || c.Filter == item {
			return "unresolved", "先查本机" + label + "分拣器及最后可达带段，现有连接不足以确认收发端"
		}
	}
	if hasDirection {
		return "filter-mismatch", fmt.Sprintf("本机%s分拣器的过滤条件均不接收物品 %d，先查过滤设置及是否另有转运途径", label, item)
	}
	return "no-direction-sorter", "保存时未发现本机" + label + "分拣器，先检查该方向的接驳"
}

// addNeighborEvidence 把同一条物料路径上的设备状态关联到检查建议，共享线路只报告可达关系。
func (factory *factoryLine) addNeighborEvidence(r *CircuitReport, incoming map[int64][]int64, attached map[int64][]lineSorter) {
	machines := map[int64]CircuitMachine{}
	for _, m := range r.Machines {
		machines[m.AssemblerID] = m
	}
	peers := map[int64]CircuitMachine{}
	for _, p := range r.Neighbors {
		peers[p.EntityID] = p
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		seen := map[int64]bool{}
		link := func(entity int64) {
			p, ok := peers[entity]
			if !ok || seen[entity] {
				return
			}
			if f.Status == "input-missing" && p.Status == "input-missing" && slices.ContainsFunc(p.Outputs, func(o ProductionOutput) bool { return o.ItemID == f.ItemID }) ||
				f.Status == "output-blocked" && p.Status == "output-blocked" && slices.ContainsFunc(p.Inputs, func(in CircuitInput) bool { return in.ItemID == f.ItemID }) {
				seen[entity] = true
				f.RelatedBlockedEntities = append(f.RelatedBlockedEntities, entity)
			}
		}
		direction := "in"
		if f.Status == "output-blocked" {
			direction = "out"
		} else if f.Status != "input-missing" {
			continue
		}
		if f.CheckBelt != 0 {
			// 从建议检查点重走对应方向，排除只在前一分支可达的设备。
			route := factory.trace(f.CheckBelt, direction, f.ItemID, incoming, attached)
			for _, ep := range route.Endpoints {
				link(ep.EntityID)
			}
		} else {
			for _, id := range f.Assemblers {
				for _, c := range machines[id].Connections {
					if (c.Reversible || c.Direction == direction) && (c.Filter == 0 || c.Filter == f.ItemID) && factory.beltByEntity[c.OtherEntity] == 0 && c.OtherBuilding > 0 {
						f.RouteEvidence = "direct"
						f.Advice = "先核查该物品直接连接的分拣器和相邻设备的缓冲"
						link(c.OtherEntity)
					}
				}
			}
		}
		sort.Slice(f.RelatedBlockedEntities, func(i, j int) bool { return f.RelatedBlockedEntities[i] < f.RelatedBlockedEntities[j] })
		if count := len(seen); count > 0 {
			if f.Status == "output-blocked" {
				f.Advice = fmt.Sprintf("可达的 %d 台下游设备需要该物品，且其自身出料受阻。先检查这些设备能否送走成品。共享线路还需核实实际流向。", count)
			} else {
				f.Advice = fmt.Sprintf("可达的 %d 台上游设备生产该物品，且其自身缺料。先检查这些设备的进料，共享线路还需核实实际供货。", count)
			}
		}
	}
}
