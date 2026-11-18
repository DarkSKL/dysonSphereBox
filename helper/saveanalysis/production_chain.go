package saveanalysis

// 文件职责：在一次授权快照查询中，从选定产品的实际生产者沿缺料输入追到可达设备和货槽。
// 使用：返回带索引的证据图与检查顺序；边界：只沿本档物理连接及已载货任务展开。

import (
	"fmt"
	"slices"
)

// ProductionDiagnosis 描述所选产品的生产设备、供应证据和按优先级排序的检查位置。
type ProductionDiagnosis struct {
	Downstream      *OutputDiagnosis `json:"downstream,omitempty"`      // 所选设备的成品积压分支。
	FirstCheckGroup string           `json:"firstCheckGroup,omitempty"` // 指向优先级 1 所在的检查清单。
	SHA256          string           `json:"sha256"`
	Tick            int64            `json:"tick"`
	PlanetID        int64            `json:"planetId"`
	ProductID       int64            `json:"productId"`
	Status          string           `json:"status"` // history-only、blocked、undetermined 或 no-observed-block。
	Roots           []int            `json:"roots"`  // 所选产品的全部当前设备，值为 Nodes 下标。
	Nodes           []ChainNode      `json:"nodes"`
	Checks          []ChainCheck     `json:"checks"`
}

// ChainNode 表示一个星球上的设备产物或货槽，保留判定时的原料、运行或运输证据。
type ChainNode struct {
	PlanetID      int64             `json:"planetId"`
	EntityID      int64             `json:"entityId"`
	ProductID     int64             `json:"productId"`
	Machine       *CircuitMachine   `json:"machine,omitempty"`
	Device        *DeviceState      `json:"device,omitempty"`
	Freight       *StationFreight   `json:"freight,omitempty"`
	Constraints   []ChainConstraint `json:"constraints"`   // 同机不同缺料或不同限制分别记录。
	Inputs        []ChainInput      `json:"inputs"`        // 设备缺料或货槽进塔端的物理连接。
	LoadedSources []int             `json:"loadedSources"` // 在途载货可定位来源槽的节点，逐单证据见 Freight。
}

// ChainConstraint 记录设备此刻的限制或未决状态；物品编号为零表示本机级别判断。
type ChainConstraint struct {
	Status string `json:"status"`
	ItemID int64  `json:"itemId"`
}

// ChainInput 描述一个进料检查口。Sources 指向可达生产设备或出货槽，不分摊共享流量。
type ChainInput struct {
	BlockedFeeds []BlockedFeed `json:"blockedFeeds,omitempty"` // 有投放拒收证据的进料支路。
	ItemID       int64         `json:"itemId"`
	CheckEntity  int64         `json:"checkEntity"`
	Sorters      []int64       `json:"sorters"`
	StartBelts   []int64       `json:"startBelts"`
	Sources      []int         `json:"sources"`
	Shared       bool          `json:"shared"`
	Unresolved   bool          `json:"unresolved"`
}

// BlockedFeed 将可达的供应端拒收点与具体的进料带、取料分拣器对应。每条带单独保留。
type BlockedFeed struct {
	StartBelt int64             `json:"startBelt"`
	Sorters   []int64           `json:"sorters"`
	Drops     []DropObservation `json:"drops"`
}

// ChainCheck 给出可执行的检查动作。RootEntities 是所选星球上可关联的目标设备，
// DirectEntities 是检查点所在星球上直接缺料的设备；两组编号分别去重。
type ChainCheck struct {
	ReceiverEntities   []int64          `json:"receiverEntities,omitempty"` // 下游转查时，跨过的缺料接收机。
	Drop               *DropObservation `json:"drop,omitempty"`             // 此供应设备在已查进料路线上的一个投放拒收点。
	Priority           int              `json:"priority"`                   // 同等检查依据共用名次。
	Evidence           string           `json:"evidence"`                   // 保存时刻观测到的库存、缓冲或本机状态。
	UpstreamConstraint bool             `json:"upstreamConstraint"`         // 该缺料分支还能继续定位到已观测的限制。
	Node               int              `json:"node"`
	Status             string           `json:"status"`
	ItemID             int64            `json:"itemId"`
	DirectEntities     []int64          `json:"directEntities"`
	RootEntities       []int64          `json:"rootEntities"`
	Shared             bool             `json:"shared"`
	Unresolved         bool             `json:"unresolved"`
	Advice             string           `json:"advice"`
}

// chainKey 用于查询内去重；货槽使用本星球站号和零基槽号，设备使用实体和产物。
type chainKey struct {
	planet, entity, product int64
	slot                    int
}

// inputBranches 在一个物品和星球内复用一次寻路结果，只提取当前消费者自己的分支。
type inputBranches struct {
	report   SupplyReport
	byEntity map[int64]SupplyBranch
}

// chainWalk 的索引和队列仅在本次调用内有效。实体不同产物及物流槽使用不同的键。
type chainWalk struct {
	s       *Snapshot
	report  ProductionDiagnosis
	ids     map[chainKey]int
	keys    []chainKey
	inputs  map[[3]int64]inputBranches
	catalog map[[2]int64]map[int64]ChainNode
}

// producers 按保存的实际产物收集设备及其所有限制，未工作原因不明时保留原状态。
func (w *chainWalk) producers(planet, product int64) map[int64]ChainNode {
	k := [2]int64{planet, product}
	if nodes, ok := w.catalog[k]; ok {
		return nodes
	}
	nodes := map[int64]ChainNode{}
	w.catalog[k] = nodes
	f := w.s.lines[planet]
	if f == nil {
		return nodes
	}
	for _, a := range f.machines {
		if !slices.Contains(a.products, product) {
			continue
		}
		m := f.diagnose(a, w.s.gameVersion, product)
		n := ChainNode{PlanetID: planet, EntityID: a.entity, ProductID: product, Machine: &m}
		switch m.Status {
		case "input-missing":
			for _, v := range m.Inputs {
				if v.Required > v.Available {
					n.Constraints = append(n.Constraints, ChainConstraint{m.Status, v.ItemID})
				}
			}
		case "output-blocked":
			for _, v := range m.Outputs {
				if v.Batch > 0 && v.Buffered > v.DeliveryLimit {
					n.Constraints = append(n.Constraints, ChainConstraint{m.Status, v.ItemID})
				}
			}
		case "processing":
		default:
			n.Constraints = append(n.Constraints, ChainConstraint{Status: m.Status})
		}
		nodes[a.entity] = n
	}
	for _, d := range f.devices {
		if d.ProductID != product {
			continue
		}
		v, issues := f.deviceState(d, w.s.gameVersion)
		n := ChainNode{PlanetID: planet, EntityID: d.EntityID, ProductID: product, Device: &v}
		if v.Status == "source-located" {
			n.Constraints = append(n.Constraints, ChainConstraint{Status: "undetermined"})
		}
		for _, issue := range issues {
			n.Constraints = append(n.Constraints, ChainConstraint{issue.status, issue.item})
		}
		nodes[d.EntityID] = n
	}
	return nodes
}

// intern 让共享、汇流和循环引用同一节点，队列中的每个节点只展开一次。
func (w *chainWalk) intern(k chainKey, n ChainNode) int {
	if i, ok := w.ids[k]; ok {
		return i
	}
	if n.Constraints == nil {
		n.Constraints = []ChainConstraint{}
	}
	n.Inputs = []ChainInput{}
	n.LoadedSources = []int{}
	i := len(w.keys)
	w.ids[k] = i
	w.keys = append(w.keys, k)
	w.report.Nodes = append(w.report.Nodes, n)
	return i
}

// branch 复用同一配方缺料查询。图只连接该实体可达的来源，其他同配方设备不会进入这条分支。
func (w *chainWalk) branch(n ChainNode, item int64) (SupplyBranch, SupplyReport, error) {
	key := [3]int64{n.PlanetID, n.ProductID, item}
	view, ok := w.inputs[key]
	if !ok {
		r, err := w.s.InputSupply(n.PlanetID, n.ProductID, item, nil)
		if err != nil {
			return SupplyBranch{}, r, err
		}
		view = inputBranches{r, map[int64]SupplyBranch{}}
		for _, b := range r.Branches {
			for _, id := range b.Consumers {
				view.byEntity[id] = b
			}
		}
		w.inputs[key] = view
	}
	b, ok := view.byEntity[n.EntityID]
	if !ok {
		b = SupplyBranch{CheckEntity: n.EntityID, Unresolved: true}
	}
	return b, view.report, nil
}

// physical 关联一个已筛选的进料口，可复用已有的逐段查询复查相应起始带。
func (w *chainWalk) physical(r SupplyReport, b SupplyBranch) ChainInput {
	l := ChainInput{ItemID: r.ItemID, CheckEntity: b.CheckEntity, Sorters: append([]int64{}, b.Sorters...), StartBelts: []int64{}, Sources: []int{}, Shared: b.Shared, Unresolved: b.Unresolved}
	for _, idx := range b.Routes {
		l.StartBelts = append(l.StartBelts, r.Routes[idx].StartBelt)
	}
	devices := w.producers(r.PlanetID, r.ItemID)
	for _, route := range b.Routes {
		start := r.Routes[route].StartBelt
		feed := BlockedFeed{StartBelt: start, Sorters: b.beltSorters[start]}
		for _, end := range r.Routes[route].Endpoints {
			d := w.s.lines[r.PlanetID].drops[end.SorterEntity]
			if d.Refusal != "" && d.Source == end.EntityID && d.ItemID == r.ItemID && d.Belt == end.Belt {
				if _, ok := devices[end.EntityID]; ok {
					feed.Drops = append(feed.Drops, d)
				}
			}
		}
		if len(feed.Drops) != 0 {
			l.BlockedFeeds = append(l.BlockedFeeds, feed)
		}
	}
	for _, id := range b.ProducerEntities {
		n, ok := devices[id]
		if !ok {
			l.Unresolved = true
			continue
		}
		l.Sources = append(l.Sources, w.intern(chainKey{r.PlanetID, id, r.ItemID, -1}, n))
	}
	for _, slot := range b.SourceSlots {
		l.Sources = append(l.Sources, w.station(slot.PlanetID, slot.StationID, slot.Index, slot.ItemID))
	}
	return l
}

// station 为待查货槽分配节点，下次出队时读取进塔路线和在途订单。
func (w *chainWalk) station(planet, station int64, index int, item int64) int {
	return w.intern(chainKey{planet, station, item, index}, ChainNode{PlanetID: planet, ProductID: item})
}

// expand 沿设备已证实的缺料输入、货槽进塔线与实际载货来源展开。纯配置和取货预约留在运输证据中。
func (w *chainWalk) expand(i int) error {
	k, n := w.keys[i], w.report.Nodes[i]
	if k.slot < 0 {
		for _, c := range n.Constraints {
			if c.Status != "input-missing" {
				continue
			}
			if c.ItemID <= 0 {
				n.Inputs = append(n.Inputs, ChainInput{CheckEntity: n.EntityID, Unresolved: true, Sorters: []int64{}, StartBelts: []int64{}, Sources: []int{}})
				continue
			}
			b, r, err := w.branch(n, c.ItemID)
			if err != nil {
				return err
			}
			n.Inputs = append(n.Inputs, w.physical(r, b))
		}
	} else {
		r, err := w.s.StationSupply(k.planet, k.entity, k.slot)
		if err != nil {
			return err
		}
		n.Freight = r.Freight
		n.EntityID = r.Freight.Slot.EntityID
		for _, b := range r.Branches {
			n.Inputs = append(n.Inputs, w.physical(r, b))
		}
		for _, t := range n.Freight.Tasks {
			if t.Phase != "carrying" || t.SourceIndex < 0 {
				continue
			}
			idx := w.station(t.SourcePlanet, t.SourceStation, t.SourceIndex, k.product)
			if !slices.Contains(n.LoadedSources, idx) {
				n.LoadedSources = append(n.LoadedSources, idx)
			}
		}
	}
	w.report.Nodes[i] = n
	return nil
}

// DiagnoseProduction 职责：按星球与实际产物自动诊断；使用：从快照内目标生产者开始；边界：历史产出单独返回。
func (s *Snapshot) DiagnoseProduction(planet, product int64) (ProductionDiagnosis, error) {
	options, err := s.ProductionItems(planet)
	if err != nil {
		return ProductionDiagnosis{}, err
	}
	if !slices.ContainsFunc(options, func(p ProductionItem) bool { return p.ItemID == product }) {
		return ProductionDiagnosis{}, fmt.Errorf("星球 %d 没有物品 %d 的设备或产出记录", planet, product)
	}
	w := chainWalk{s: s, ids: map[chainKey]int{}, inputs: map[[3]int64]inputBranches{}, catalog: map[[2]int64]map[int64]ChainNode{},
		report: ProductionDiagnosis{SHA256: s.digest, Tick: s.tick, PlanetID: planet, ProductID: product, Status: "history-only", Roots: []int{}, Nodes: []ChainNode{}, Checks: []ChainCheck{}}}
	roots := w.producers(planet, product)
	entities := make([]int64, 0, len(roots))
	for id := range roots {
		entities = append(entities, id)
	}
	slices.Sort(entities)
	if len(entities) > 0 {
		w.report.Status = "no-observed-block"
	}
	for _, id := range entities {
		n := roots[id]
		for _, c := range n.Constraints {
			if c.Status != "undetermined" {
				w.report.Status = "blocked"
			} else if w.report.Status != "blocked" {
				w.report.Status = "undetermined"
			}
		}
		w.report.Roots = append(w.report.Roots, w.intern(chainKey{planet, id, product, -1}, n))
	}
	if err := w.expandAll(); err != nil {
		return ProductionDiagnosis{}, err
	}
	w.report.prioritize()
	w.report.Downstream = s.diagnoseOutputs(w.report)
	if len(w.report.Checks) > 0 {
		w.report.FirstCheckGroup = "checks"
	}
	if out := w.report.Downstream; out != nil && len(out.Checks) > 0 {
		c := out.Checks[0]
		if len(w.report.Checks) == 0 || (c.Status != "check-output" && c.Status != "not-requesting" && len(c.RootEntities) >= len(w.report.Checks[0].RootEntities)) {
			w.report.FirstCheckGroup = "downstream.checks"
		}
		if err := w.linkReceiverSupply(out); err != nil {
			return ProductionDiagnosis{}, err
		}
		if supply := out.Supply; supply != nil && len(supply.Checks) > 0 {
			best := supply.Checks[0]
			covered := len(c.RootEntities)
			if w.report.FirstCheckGroup == "checks" {
				covered = len(w.report.Checks[0].RootEntities)
			}
			target := supply.Nodes[best.Node]
			isReceiver := target.PlanetID == planet && slices.Contains(best.ReceiverEntities, target.EntityID)
			if len(best.RootEntities) >= covered && !isReceiver && best.Status != "undetermined" && best.Status != "stocked-slot" {
				w.report.FirstCheckGroup = "downstream.supply.checks"
			}
		}
	}
	return w.report, nil
}

// expandAll 在同一快照展开新发现的节点；intern 让跨设备及物流回环只处理一次。
func (w *chainWalk) expandAll() error {
	for i := 0; i < len(w.keys); i++ {
		if err := w.expand(i); err != nil {
			return err
		}
	}
	return nil
}
