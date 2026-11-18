package saveanalysis

// 文件职责：从已经确认缺料的设备或指定物流货槽逆查进货，按实际配方筛选可达生产者。
// 使用：将上一级返回的生产实体传给下一级；边界：共享带上的可达性无法证明单件物品去向。

import (
	"fmt"
	"slices"
	"sort"
)

// SupplyScope 限定上一级可达实体。省略 entities 从全星球开始，显式空数组表示上一级没有可达生产者。
type SupplyScope struct {
	Entities []int64 `json:"entities,omitempty"`
}

// SupplyBranch 把受影响的设备和实际进料口关联到同一份报告内的路线、生产者及出货槽。
type SupplyBranch struct {
	beltSorters      map[int64][]int64 // 起始带对应的实际进料分拣器；直连塔口没有分拣器。
	Consumers        []int64           `json:"consumers"`
	CheckEntity      int64             `json:"checkEntity"`
	Sorters          []int64           `json:"sorters"`
	Routes           []int             `json:"routes"`
	Direct           []int64           `json:"direct"`
	ProducerEntities []int64           `json:"producerEntities"`
	SourceSlots      []SupplySlot      `json:"sourceSlots"`
	Unresolved       bool              `json:"unresolved"`
	Shared           bool              `json:"shared"`
}

// SupplyCheck 汇集同一来源槽可达的受阻设备，数量表示检查覆盖范围。
type SupplyCheck struct {
	Slot      SupplySlot `json:"slot"`
	Consumers []int64    `json:"consumers"`
	Shared    bool       `json:"shared"`
	Advice    string     `json:"advice"`
}

// SupplyReport 提供缺料输入或物流塔进货端的上游证据。路线保留未确认端点和开放带段。
type SupplyReport struct {
	SHA256    string           `json:"sha256"`
	Tick      int64            `json:"tick"`
	PlanetID  int64            `json:"planetId"`
	ProductID int64            `json:"productId,omitempty"`
	ItemID    int64            `json:"itemId"`
	Checks    []SupplyCheck    `json:"checks"`
	Branches  []SupplyBranch   `json:"branches"`
	Routes    []CircuitRoute   `json:"routes"`
	Producers []CircuitMachine `json:"producers"`
	Devices   []DeviceState    `json:"devices"`
	Freight   *StationFreight  `json:"freight,omitempty"`
}

type supplyTrace struct {
	g      deviceGraph
	r      SupplyReport
	byBelt map[int64]int
}

func newSupplyTrace(s *Snapshot, f *factoryLine, planet, product, item int64) *supplyTrace {
	return &supplyTrace{
		g: newDeviceGraph(f), byBelt: map[int64]int{},
		r: SupplyReport{SHA256: s.digest, Tick: s.tick, PlanetID: planet, ProductID: product, ItemID: item,
			Branches: []SupplyBranch{}, Routes: []CircuitRoute{}, Producers: []CircuitMachine{}, Devices: []DeviceState{}, Checks: []SupplyCheck{}},
	}
}
func (t *supplyTrace) belt(b *SupplyBranch, id int64) {
	if id <= 0 {
		return
	}
	idx, ok := t.byBelt[id]
	if !ok {
		idx = len(t.r.Routes)
		t.byBelt[id] = idx
		t.r.Routes = append(t.r.Routes, t.g.f.trace(id, "in", t.r.ItemID, t.g.incoming, t.g.attached))
	}
	if !slices.Contains(b.Routes, idx) {
		b.Routes = append(b.Routes, idx)
	}
	b.Shared = b.Shared || t.r.Routes[idx].Shared
}
func (t *supplyTrace) sorterInputs(b *SupplyBranch) {
	for _, sorter := range t.g.attached[b.CheckEntity] {
		if sorter.filter > 0 && sorter.filter != t.r.ItemID {
			continue
		}
		other := sorter.pick
		if sorter.drop != b.CheckEntity {
			if !sorter.reversible {
				continue
			}
			other = sorter.drop
		}
		b.Sorters = append(b.Sorters, sorter.entity)
		if id := t.g.f.beltByEntity[other]; id > 0 {
			if b.beltSorters == nil {
				b.beltSorters = make(map[int64][]int64)
			}
			b.beltSorters[id] = append(b.beltSorters[id], sorter.entity)
			t.belt(b, id)
		} else {
			b.Direct = append(b.Direct, other)
		}
	}
}

// finish 只在对应的进货路线上匹配配方和出货口，同星球的其他生产线不会混入结果。
func (t *supplyTrace) finish(s *Snapshot) SupplyReport {
	r := &t.r
	f := t.g.f
	producers := map[int64]bool{}
	for _, m := range f.machines {
		producers[m.entity] = slices.Contains(m.products, r.ItemID)
	}
	for _, d := range f.devices {
		producers[d.EntityID] = producers[d.EntityID] || d.ProductID == r.ItemID
	}
	stations := map[int64]*layoutStationComponent{}
	for _, st := range s.stations {
		if st.planetId == r.PlanetID {
			stations[st.entityId] = st
		}
	}
	reached := map[int64]bool{}
	beltUses := map[int64]int{}
	for _, b := range r.Branches {
		seen := map[int64]bool{}
		for _, idx := range b.Routes {
			for _, id := range r.Routes[idx].Belts {
				seen[id] = true
			}
		}
		for id := range seen {
			beltUses[id]++
		}
	}
	for i := range r.Routes {
		route := &r.Routes[i]
		route.Shared = route.Shared || slices.ContainsFunc(route.Belts, func(id int64) bool { return beltUses[id] > 1 })
	}
	for i := range r.Branches {
		b := &r.Branches[i]
		b.ProducerEntities = []int64{}
		b.SourceSlots = []SupplySlot{}
		mark := func(entity int64) {
			if _, known := producers[entity]; !known && stations[entity] == nil {
				b.Unresolved = true
			}
			if producers[entity] && !slices.Contains(b.ProducerEntities, entity) {
				b.ProducerEntities = append(b.ProducerEntities, entity)
				reached[entity] = true
			}
		}
		slot := func(st *layoutStationComponent, i int) {
			if !st.matches(i, r.ItemID) {
				b.Unresolved = true
				return
			}
			v := st.slot(i)
			if !slices.Contains(b.SourceSlots, v) {
				b.SourceSlots = append(b.SourceSlots, v)
			}
		}
		for _, id := range b.Direct {
			mark(id)
			if st := stations[id]; st != nil {
				slot(st, st.uniqueSlot(r.ItemID))
			}
		}
		for _, idx := range b.Routes {
			route := r.Routes[idx]
			b.Unresolved = b.Unresolved || len(route.OpenEnds) > 0
			b.Shared = b.Shared || route.Shared
			for _, e := range route.Endpoints {
				mark(e.EntityID)
				if st := stations[e.EntityID]; st != nil {
					if e.SorterEntity != 0 {
						slot(st, st.uniqueSlot(r.ItemID))
						continue
					}
					for _, p := range st.ports {
						if p.dir == 1 && p.belt == e.Belt {
							slot(st, p.slot)
						}
					}
				}
			}
		}
		b.Unresolved = b.Unresolved || len(b.ProducerEntities)+len(b.SourceSlots) == 0
		slices.Sort(b.ProducerEntities)
		b.Shared = b.Shared || len(b.ProducerEntities)+len(b.SourceSlots) > 1
	}
	for _, m := range f.machines {
		if reached[m.entity] && slices.Contains(m.products, r.ItemID) {
			r.Producers = append(r.Producers, f.diagnose(m, s.gameVersion, r.ItemID))
		}
	}
	for _, d := range f.devices {
		if reached[d.EntityID] && d.ProductID == r.ItemID {
			v, _ := f.deviceState(d, s.gameVersion)
			r.Devices = append(r.Devices, v)
		}
	}
	r.prioritizeSlots()
	return *r
}

// prioritizeSlots 在同一级优先检查覆盖受阻设备较多的货槽，保留共享关系。
func (r *SupplyReport) prioritizeSlots() {
	indices := map[SupplySlot]int{}
	for _, b := range r.Branches {
		if len(b.Consumers) == 0 {
			continue
		}
		for _, slot := range b.SourceSlots {
			i, ok := indices[slot]
			if !ok {
				i = len(r.Checks)
				indices[slot] = i
				advice := "核对该槽的出带和沿线取料分拣器"
				if slot.Inventory == 0 {
					advice = "先核对此空槽的实际来货、空载取货预约和进塔带，再查到货后的出带"
				}
				r.Checks = append(r.Checks, SupplyCheck{Slot: slot, Advice: advice})
			}
			c := &r.Checks[i]
			c.Shared = c.Shared || b.Shared || len(c.Consumers) > 0
			c.Consumers = append(c.Consumers, b.Consumers...)
		}
	}
	for i := range r.Checks {
		c := &r.Checks[i]
		slices.Sort(c.Consumers)
		c.Consumers = slices.Compact(c.Consumers)
	}
	sort.SliceStable(r.Checks, func(i, j int) bool { return len(r.Checks[i].Consumers) > len(r.Checks[j].Consumers) })
}

// InputSupply 查询指定产品已证实的缺料输入，entities 可限制在上一级实际可达的生产实体内。
func (s *Snapshot) InputSupply(planet, product, item int64, entities []int64) (SupplyReport, error) {
	if item <= 0 || product <= 0 {
		return SupplyReport{}, fmt.Errorf("物品编号必须为正数")
	}
	f := s.lines[planet]
	if f == nil {
		return SupplyReport{}, fmt.Errorf("星球 %d 没有可查询的厂区", planet)
	}
	t := newSupplyTrace(s, f, planet, product, item)
	selected := func(id int64) bool { return entities == nil || slices.Contains(entities, id) }
	known := false
	for _, a := range f.machines {
		if !slices.Contains(a.products, product) {
			continue
		}
		known = true
		if !selected(a.entity) {
			continue
		}
		m := f.diagnose(a, s.gameVersion, product)
		if m.Status != "input-missing" || !slices.ContainsFunc(m.Inputs, func(v CircuitInput) bool { return v.ItemID == item && v.Available < v.Required }) {
			continue
		}
		b := SupplyBranch{CheckEntity: a.entity, Consumers: []int64{a.entity}}
		t.sorterInputs(&b)
		t.r.Branches = append(t.r.Branches, b)
	}
	dr := s.deviceReport(planet, product)
	for _, c := range dr.Checks {
		if c.Status != "input-missing" || c.ItemID != item {
			continue
		}
		b := SupplyBranch{CheckEntity: c.CheckEntity, Shared: c.Shared}
		for _, id := range c.DeviceEntities {
			if selected(id) {
				b.Consumers = append(b.Consumers, id)
			}
		}
		if len(b.Consumers) == 0 {
			continue
		}
		if len(c.SorterEntities) > 0 {
			t.sorterInputs(&b)
		} else {
			for _, idx := range c.Routes {
				t.belt(&b, dr.Routes[idx].StartBelt)
			}
		}
		t.r.Branches = append(t.r.Branches, b)
	}
	if !known && len(dr.Devices) == 0 {
		return SupplyReport{}, fmt.Errorf("该星球未定位到生产物品 %d 的设备", product)
	}
	return t.finish(s), nil
}

// StationSupply 从零基货槽查询运输任务和入塔线路。进塔带不指定单一目的货槽，按可达配方匹配。
func (s *Snapshot) StationSupply(planet, station int64, index int) (SupplyReport, error) {
	for _, st := range s.stations {
		if st.planetId != planet || st.id != station {
			continue
		}
		if index < 0 || index >= len(st.storage) || st.storage[index].itemId <= 0 {
			return SupplyReport{}, fmt.Errorf("货槽不存在或未配置物品")
		}
		f := s.lines[planet]
		if f == nil {
			break
		}
		t := newSupplyTrace(s, f, planet, 0, st.storage[index].itemId)
		fr := s.stationFreight(st, index)
		t.r.Freight = &fr
		b := SupplyBranch{CheckEntity: st.entityId}
		t.sorterInputs(&b)
		for _, p := range st.ports {
			if p.dir == 2 {
				t.belt(&b, p.belt)
			}
		}
		t.r.Branches = append(t.r.Branches, b)
		return t.finish(s), nil
	}
	return SupplyReport{}, fmt.Errorf("星球 %d 未找到物流站 %d", planet, station)
}
