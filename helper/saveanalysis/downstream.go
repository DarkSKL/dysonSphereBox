package saveanalysis

// 文件职责：从成品交付受阻的设备追踪出料方向。使用：由固定快照的产物诊断调用。
// 边界：带段可达性和某一投放点的占用分别记录，只有接收端自身的状态可支持继续归因。

import (
	"cmp"
	"fmt"
	"slices"
)

// DropObservation 记录实际持货的分拣器。Refusal 只在普通正向投放规则能证明拒收时填写。
type DropObservation struct {
	Source  int64  `json:"source"`
	Sorter  int64  `json:"sorter"`
	Target  int64  `json:"target"`
	Stage   int64  `json:"stage"`
	Belt    int64  `json:"belt"`
	ItemID  int64  `json:"itemId"`
	Count   int64  `json:"count"`
	Refusal string `json:"refusal,omitempty"`
}

// dropRefusal 复现普通带口寻找投放位置时的拒收分支；其后压缩或堆叠的可能性保持未决。
func dropRefusal(buffer []byte, pos int64) string {
	if pos < 4 || pos+5 >= int64(len(buffer)) {
		return ""
	}
	end := pos + 5
	for end > pos && buffer[end] != 0 {
		end--
	}
	if end == pos {
		return "occupied-window"
	}
	center := end - 5
	if end+6 < int64(len(buffer)) {
		for j := end + 1; j <= end+5; j++ {
			if buffer[j] != 0 {
				center = j - 6
				break
			}
		}
	}
	if center < 4 {
		return "path-boundary"
	}
	return ""
}

// OutputDiagnosis 描述成品积压分支。报告内下标指向本报告的设备、路线或接收端。
type OutputDiagnosis struct {
	Supply    *OutputSupply    `json:"supply,omitempty"`
	Sources   []OutputSource   `json:"sources"`
	Routes    []CircuitRoute   `json:"routes"`
	Receivers []OutputReceiver `json:"receivers"`
	Checks    []OutputCheck    `json:"checks"`
}

// OutputSource 保存每台积压设备的出料位置，Roots 为可关联的所选产物实体。
type OutputSource struct {
	EntityID   int64        `json:"entityId"`
	ItemID     int64        `json:"itemId"`
	Buffered   int64        `json:"buffered"`
	Roots      []int64      `json:"roots"`
	Ports      []OutputPort `json:"ports"`
	Shared     bool         `json:"shared"`
	Unresolved bool         `json:"unresolved"`
}

// OutputPort 对应一个出料分拣器或直连带口。路线下标为 -1 表示没有中间传送带。
type OutputPort struct {
	CheckEntity int64            `json:"checkEntity"`
	Sorter      int64            `json:"sorter"`
	Target      int64            `json:"target"`
	Route       int              `json:"route"`
	Drop        *DropObservation `json:"drop,omitempty"`
	Receivers   []int            `json:"receivers"`
}

// OutputReceiver 记录收货实体对该物品的请求、库存或设备状态。Requested 缺省表示无法核实收货意愿。
type OutputReceiver struct {
	SupplyNode *int            `json:"supplyNode,omitempty"` // 本接收机在 Supply.Nodes 中的缺料节点。
	EntityID   int64           `json:"entityId"`
	Building   int64           `json:"building"`
	ItemID     int64           `json:"itemId"`
	Status     string          `json:"status"`
	Requested  *bool           `json:"requested,omitempty"`
	Machine    *CircuitMachine `json:"machine,omitempty"`
	Device     *DeviceState    `json:"device,omitempty"`
	Slot       *SupplySlot     `json:"slot,omitempty"`
	Next       []int           `json:"next"` // 继续追查的积压成品。
}

// OutputCheck 按检查对象及约束合并，保留接收端、直接发货设备和可关联的根设备。
type OutputCheck struct {
	Priority       int     `json:"priority"`
	EntityID       int64   `json:"entityId"`
	ItemID         int64   `json:"itemId"`
	Status         string  `json:"status"`
	Receivers      []int   `json:"receivers"`
	DirectEntities []int64 `json:"directEntities"`
	RootEntities   []int64 `json:"rootEntities"`
	Shared         bool    `json:"shared"`
	Unresolved     bool    `json:"unresolved"`
	Evidence       string  `json:"evidence"`
	Advice         string  `json:"advice"`
}

// outputWalk 为本次查询索引星球内的制造组件、端口和已访问的设备成品。
type outputWalk struct {
	s                          *Snapshot
	g                          deviceGraph
	machines                   map[int64]circuitRecord
	devices                    map[int64]deviceRecord
	stations                   map[int64]*layoutStationComponent
	sources, receivers, routes map[[2]int64]int
	r                          OutputDiagnosis
}

// source 按实体和受阻产物去重，可处理副产物、共享线和成环配方。
func (w *outputWalk) source(entity, item, buffered int64) int {
	k := [2]int64{entity, item}
	if i, ok := w.sources[k]; ok {
		return i
	}
	i := len(w.r.Sources)
	w.sources[k] = i
	w.r.Sources = append(w.r.Sources, OutputSource{EntityID: entity, ItemID: item, Buffered: buffered, Ports: []OutputPort{}})
	return i
}

// receiver 仅在实际配方消费该物品时关联机器。入塔带按物品定位，忽略入带端口的出货槽提示。
func (w *outputWalk) receiver(entity, item int64) int {
	k := [2]int64{entity, item}
	if i, ok := w.receivers[k]; ok {
		return i
	}
	v := OutputReceiver{EntityID: entity, ItemID: item, Building: w.g.f.protos[entity], Status: "unresolved", Next: []int{}}
	if st := w.stations[entity]; st != nil {
		if i := st.uniqueSlot(item); i >= 0 {
			slot := st.slot(i)
			v.Slot = &slot
			want := slot.Inventory < slot.Capacity
			v.Requested = &want
			v.Status = "station-room"
			if !want {
				v.Status = "station-full"
				// 星际站还能向独立的翘曲器缓冲收货；普通槽满不足以判定拒收。
				if item == 1210 && st.isStellar != 0 {
					v.Requested = nil
					v.Status = "unresolved-intake"
				}
			}
		} else {
			v.Status = "unresolved-slot"
		}
	} else if a, ok := w.machines[entity]; ok {
		v.Status = "other-recipe"
		if slices.Contains(a.requires, item) {
			m := w.g.f.diagnose(a, w.s.gameVersion, 0)
			v.Machine = &m
			v.Status = m.Status
			if len(a.needs) > 0 {
				want := slices.Contains(a.needs, item)
				v.Requested = &want
			}
			if v.Requested != nil && !*v.Requested && m.Status == "output-blocked" {
				for _, o := range m.Outputs {
					if o.Batch > 0 && o.Buffered > o.DeliveryLimit {
						v.Next = append(v.Next, w.source(entity, o.ItemID, o.Buffered))
					}
				}
			}
		}
	} else if d, ok := w.devices[entity]; ok {
		state, _ := w.g.f.deviceState(d, w.s.gameVersion)
		v.Device = &state
		// 特殊设备的缓冲与运行状态可读，普通制造台的收货规则不能套用。
		v.Status = "unresolved-intake"
	}
	idx := len(w.r.Receivers)
	w.receivers[k] = idx
	w.r.Receivers = append(w.r.Receivers, v)
	return idx
}

// port 保留起点、终点、转运与开放带端；收货端按实体去重，分拣器持货必须匹配实际物品及投放方向。
func (w *outputWalk) port(src *OutputSource, check, arm, target, belt int64) {
	p := OutputPort{CheckEntity: check, Sorter: arm, Target: target, Route: -1, Receivers: []int{}}
	if d, ok := w.g.f.drops[arm]; ok && d.Target == target && d.ItemID == src.ItemID {
		p.Drop = &d
	}
	add := func(e int64) {
		i := w.receiver(e, src.ItemID)
		if !slices.Contains(p.Receivers, i) {
			p.Receivers = append(p.Receivers, i)
		}
		v := w.r.Receivers[i]
		src.Unresolved = src.Unresolved || v.Status == "unresolved" || v.Status == "unresolved-slot" || v.Status == "unresolved-intake" || v.Status == "undetermined"
	}
	if belt > 0 {
		k := [2]int64{belt, src.ItemID}
		idx, ok := w.routes[k]
		if !ok {
			idx = len(w.r.Routes)
			w.routes[k] = idx
			w.r.Routes = append(w.r.Routes, w.g.f.trace(belt, "out", src.ItemID, w.g.incoming, w.g.attached))
		}
		p.Route = idx
		r := w.r.Routes[idx]
		src.Shared = src.Shared || r.Shared
		src.Unresolved = src.Unresolved || len(r.OpenEnds) > 0
		for _, ep := range r.Endpoints {
			add(ep.EntityID)
		}
	} else if target > 0 {
		add(target)
	} else {
		src.Unresolved = true
	}
	src.Ports = append(src.Ports, p)
}

// expand 检查设备对应物品的出料方向；研究站堆叠及矿机、分馏直连口复用现有设备定位。
func (w *outputWalk) expand(i int) {
	src := w.r.Sources[i]
	check := src.EntityID
	if d, ok := w.devices[check]; ok {
		local := w.g
		local.cache = map[[3]int64]int{}
		var report DeviceReport
		c := local.check(d, deviceIssue{"output-blocked", src.ItemID}, &report)
		src.Shared = c.Shared
		check = c.CheckEntity
		if d.Kind == "miner" || d.Kind == "fractionator" {
			for _, p := range d.Ports {
				if p.Role == "product" && p.ItemID == src.ItemID {
					w.port(&src, d.EntityID, 0, p.Entity, p.Belt)
				}
			}
			if w.g.f.veinStations[d.EntityID] {
				w.port(&src, d.EntityID, 0, d.EntityID, 0)
			}
			src.Unresolved = src.Unresolved || len(src.Ports) == 0
			w.r.Sources[i] = src
			return
		}
	}
	for _, a := range w.g.attached[check] {
		if a.filter != 0 && a.filter != src.ItemID {
			continue
		}
		target := a.drop
		if a.pick != check {
			if !a.reversible {
				continue
			}
			target = a.pick
		}
		src.Shared = src.Shared || a.reversible
		w.port(&src, check, a.entity, target, w.g.f.beltByEntity[target])
	}
	src.Unresolved = src.Unresolved || len(src.Ports) == 0
	w.r.Sources[i] = src
}

// diagnoseOutputs 从目标设备已证实的积压成品开始，每个节点只展开一次。
func (s *Snapshot) diagnoseOutputs(r ProductionDiagnosis) *OutputDiagnosis {
	var w *outputWalk
	roots := map[int64][]int{}
	for _, idx := range r.Roots {
		n := r.Nodes[idx]
		for _, c := range n.Constraints {
			if c.Status != "output-blocked" || c.ItemID <= 0 {
				continue
			}
			if w == nil {
				f := s.lines[r.PlanetID]
				w = &outputWalk{
					s: s, g: newDeviceGraph(f),
					machines: map[int64]circuitRecord{}, devices: map[int64]deviceRecord{},
					stations: map[int64]*layoutStationComponent{},
					sources:  map[[2]int64]int{}, receivers: map[[2]int64]int{}, routes: map[[2]int64]int{},
					r: OutputDiagnosis{Sources: []OutputSource{}, Routes: []CircuitRoute{}, Receivers: []OutputReceiver{}, Checks: []OutputCheck{}},
				}
				for _, m := range f.machines {
					w.machines[m.entity] = m
				}
				for _, d := range f.devices {
					w.devices[d.EntityID] = d
				}
				for _, st := range s.stations {
					if st.planetId == r.PlanetID {
						w.stations[st.entityId] = st
					}
				}
			}
			var buffered int64
			if n.Device != nil {
				buffered = n.Device.Buffered
			} else {
				for _, o := range n.Machine.Outputs {
					if o.ItemID == c.ItemID {
						buffered = o.Buffered
					}
				}
			}
			roots[n.EntityID] = append(roots[n.EntityID], w.source(n.EntityID, c.ItemID, buffered))
		}
	}
	if w == nil {
		return nil
	}
	for i := 0; i < len(w.r.Sources); i++ {
		w.expand(i)
	}
	for entity, start := range roots {
		seen := map[int]bool{}
		queue := slices.Clone(start)
		for i := 0; i < len(queue); i++ {
			j := queue[i]
			if seen[j] {
				continue
			}
			seen[j] = true
			src := &w.r.Sources[j]
			src.Roots = append(src.Roots, entity)
			for _, p := range src.Ports {
				for _, k := range p.Receivers {
					queue = append(queue, w.r.Receivers[k].Next...)
				}
			}
		}
	}
	w.r.propagateRouteLimits()
	w.r.prioritize()
	return &w.r
}

// propagateRouteLimits 将共用带段和开放分支的限制传给后续检查点。循环中的标记只会从无变为有。
func (r *OutputDiagnosis) propagateRouteLimits() {
	users := map[int64]int{}
	for i := range r.Sources {
		src := &r.Sources[i]
		src.Shared = src.Shared || len(src.Roots) > 1
		for _, p := range src.Ports {
			if p.Route < 0 {
				continue
			}
			for _, b := range r.Routes[p.Route].Belts {
				if other, ok := users[b]; ok && other != i {
					src.Shared = true
					r.Sources[other].Shared = true
				} else {
					users[b] = i
				}
			}
		}
	}
	queue := make([]int, len(r.Sources))
	for i := range queue {
		queue[i] = i
	}
	for pos := 0; pos < len(queue); pos++ {
		src := r.Sources[queue[pos]]
		for _, p := range src.Ports {
			for _, receiver := range p.Receivers {
				for _, next := range r.Receivers[receiver].Next {
					target := &r.Sources[next]
					if src.Shared && !target.Shared || src.Unresolved && !target.Unresolved {
						target.Shared = target.Shared || src.Shared
						target.Unresolved = target.Unresolved || src.Unresolved
						queue = append(queue, next)
					}
				}
			}
		}
	}
}

// prioritize 将无需求且受阻的消费者、满槽、拒收投放点分别归因。根设备数用于安排检查，不能作为流量。
func (r *OutputDiagnosis) prioritize() {
	type key struct {
		entity, item int64
		status       string
	}
	checks := map[key]int{}
	add := func(src OutputSource, entity, item int64, status string, receiver int, evidence, advice string, unknown bool) {
		k := key{entity, item, status}
		i, ok := checks[k]
		if !ok {
			i = len(r.Checks)
			checks[k] = i
			r.Checks = append(r.Checks, OutputCheck{EntityID: entity, ItemID: item, Status: status, Evidence: evidence, Advice: advice, Receivers: []int{}})
		}
		c := &r.Checks[i]
		if receiver >= 0 && !slices.Contains(c.Receivers, receiver) {
			c.Receivers = append(c.Receivers, receiver)
		}
		if !slices.Contains(c.DirectEntities, src.EntityID) {
			c.DirectEntities = append(c.DirectEntities, src.EntityID)
		}
		c.RootEntities = append(c.RootEntities, src.Roots...)
		c.Shared = c.Shared || src.Shared || len(src.Roots) > 1
		c.Unresolved = c.Unresolved || src.Unresolved || unknown
	}
	for i := range r.Sources {
		src := &r.Sources[i]
		slices.Sort(src.Roots)
		found := false
		for _, p := range src.Ports {
			if d := p.Drop; d != nil && d.Refusal != "" {
				reason := "五个候选位置均占用"
				if d.Refusal == "path-boundary" {
					reason = "按占用情况调整后的投放位置越过货运路径起点"
				}
				found = true
				add(*src, d.Sorter, src.ItemID, "belt-drop-blocked", -1, fmt.Sprintf("分拣器处于投放阶段，持有 %d 件；带段 %d 的%s", d.Count, d.Belt, reason), "检查该分拣器投放点与带上接收端", false)
			}
			for _, idx := range p.Receivers {
				v := r.Receivers[idx]
				if v.Requested == nil || *v.Requested {
					continue
				}
				found = true
				if v.Slot != nil {
					slot := v.Slot
					add(*src, v.EntityID, v.ItemID, "station-full", idx, fmt.Sprintf("槽内 %d/%d 件，达到进塔收货上限", slot.Inventory, slot.Capacity), fmt.Sprintf("查 %d 号站第 %d 槽的出货或消耗", slot.StationID, slot.Index+1), false)
					continue
				}
				m := v.Machine
				switch m.Status {
				case "input-missing":
					for _, in := range m.Inputs {
						if in.Available < in.Required {
							add(*src, v.EntityID, in.ItemID, "consumer-input-missing", idx, fmt.Sprintf("接收设备未请求该成品，另一原料 %d 只有 %d/%d 件", in.ItemID, in.Available, in.Required), "先查接收设备的缺料输入及进料分拣器", false)
						}
					}
				case "output-blocked":
					for _, o := range m.Outputs {
						if o.Batch > 0 && o.Buffered > o.DeliveryLimit {
							add(*src, v.EntityID, o.ItemID, "consumer-output-blocked", idx, fmt.Sprintf("接收设备未请求该成品，自身成品 %d 缓冲 %d 件，交付门槛 %d 件", o.ItemID, o.Buffered, o.DeliveryLimit), "核对接收设备的出料及后续接收端", false)
						}
					}
				case "low-power":
					add(*src, v.EntityID, v.ItemID, "consumer-low-power", idx, fmt.Sprintf("接收设备未请求此物品，保存的供电比例 %.3f", m.Power), "核对接收设备供电", false)
				default:
					add(*src, v.EntityID, v.ItemID, "not-requesting", idx, "保存时没有请求该物品", "检查接收设备消耗及其进料缓冲", true)
				}
			}
		}
		if !found {
			add(*src, src.EntityID, src.ItemID, "check-output", -1, fmt.Sprintf("设备成品缓冲 %d 件；已追踪端点未证实下游限制", src.Buffered), "核对本机出料口、可达带段和接收端", true)
		}
	}
	for i := range r.Checks {
		c := &r.Checks[i]
		slices.Sort(c.RootEntities)
		c.RootEntities = slices.Compact(c.RootEntities)
		slices.Sort(c.DirectEntities)
		c.Shared = c.Shared || len(c.DirectEntities) > 1
	}
	tier := func(s string) int {
		switch s {
		case "station-full", "consumer-input-missing", "consumer-low-power":
			return 0
		case "consumer-output-blocked", "belt-drop-blocked":
			return 1
		default:
			return 2
		}
	}
	compare := func(a, b OutputCheck) int {
		ta, tb := tier(a.Status), tier(b.Status)
		if (ta == 2) != (tb == 2) {
			return cmp.Compare(ta, tb)
		}
		if n := cmp.Compare(len(b.RootEntities), len(a.RootEntities)); n != 0 {
			return n
		}
		if n := cmp.Compare(ta, tb); n != 0 {
			return n
		}
		return cmp.Compare(len(b.DirectEntities), len(a.DirectEntities))
	}
	slices.SortStableFunc(r.Checks, compare)
	rank := 0
	for i := range r.Checks {
		if i == 0 || compare(r.Checks[i-1], r.Checks[i]) != 0 {
			rank++
		}
		r.Checks[i].Priority = rank
	}
}
