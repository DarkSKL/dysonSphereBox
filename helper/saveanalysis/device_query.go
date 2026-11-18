package saveanalysis

// 文件职责：定位特殊生产设备的本地检查点。使用：按星球或产品查询；边界：堆叠链和带段仅表示检查范围。

import (
	"fmt"
	"slices"
	"sort"
)

// DeviceReport 记录采集、分馏与研究站的快照证据，设备间关联统一使用实体编号。
type DeviceReport struct {
	QueuedTechID int64          `json:"queuedTechId"`
	SHA256       string         `json:"sha256"`
	Tick         int64          `json:"tick"`
	PlanetID     int64          `json:"planetId"`
	Devices      []DeviceState  `json:"devices"`
	Checks       []DeviceCheck  `json:"checks"`
	Routes       []CircuitRoute `json:"routes"`
}

// DeviceCheck 按同一限制、物品和检查实体汇集受影响设备；路线编号指向本次报告。
type DeviceCheck struct {
	Status         string  `json:"status"`
	ItemID         int64   `json:"itemId"`
	CheckEntity    int64   `json:"checkEntity"`
	DeviceEntities []int64 `json:"deviceEntities"`
	SorterEntities []int64 `json:"sorterEntities,omitempty"`
	Routes         []int   `json:"routes,omitempty"`
	Shared         bool    `json:"shared"`
	Evidence       string  `json:"evidence"`
	Advice         string  `json:"advice"`
}

// PlanetDevices 返回星球上的三类组件，也覆盖无法归到物品的闲置设备及科研消费者。
func (s *Snapshot) PlanetDevices(planet int64) (DeviceReport, error) {
	if !slices.ContainsFunc(s.worldView.Planets, func(p PlanetLink) bool { return p.PlanetID == planet }) {
		return DeviceReport{}, fmt.Errorf("未知星球 %d", planet)
	}
	return s.deviceReport(planet, 0), nil
}

type deviceGraph struct {
	f        *factoryLine
	attached map[int64][]lineSorter
	incoming map[int64][]int64
	lower    map[int64]*deviceRecord
	cache    map[[3]int64]int
}

// newDeviceGraph 使用同一厂区的有效堆叠指针，遇到异配方、异科技或非唯一链接时停止归属。
func newDeviceGraph(f *factoryLine) deviceGraph {
	view := *f
	view.devicePorts = map[int64][]linePort{}
	for _, d := range f.devices {
		for _, p := range d.Ports {
			if p.Belt <= 0 || p.ItemID <= 0 {
				continue
			}
			dir := int64(1)
			if p.Role == "input" {
				dir = 2
			}
			view.devicePorts[p.Belt] = append(view.devicePorts[p.Belt], linePort{d.EntityID, dir, p.ItemID})
		}
	}
	g := deviceGraph{f: &view, attached: map[int64][]lineSorter{}, incoming: map[int64][]int64{}, lower: map[int64]*deviceRecord{}, cache: map[[3]int64]int{}}
	for _, s := range f.sorters {
		g.attached[s.pick] = append(g.attached[s.pick], s)
		if s.drop != s.pick {
			g.attached[s.drop] = append(g.attached[s.drop], s)
		}
	}
	for id, b := range f.belts {
		if b.output > 0 {
			g.incoming[b.output] = append(g.incoming[b.output], id)
		}
	}
	for _, ids := range g.incoming {
		slices.Sort(ids)
	}
	for i := range f.devices {
		d := &f.devices[i]
		if d.NextLabID > 0 {
			if _, ok := g.lower[d.NextLabID]; ok {
				g.lower[d.NextLabID] = nil
			} else {
				g.lower[d.NextLabID] = d
			}
		}
	}
	return g
}

func (g deviceGraph) check(d deviceRecord, issue deviceIssue, r *DeviceReport) DeviceCheck {
	c := DeviceCheck{Status: issue.status, ItemID: issue.item, CheckEntity: d.EntityID, Evidence: "device"}
	switch issue.status {
	case "low-power":
		c.Advice = "检查本机供电"
		return c
	case "no-source":
		c.Advice = "本机没有绑定矿点，先检查采集覆盖范围"
		return c
	case "undetermined":
		c.Advice = "核实本机配置和工作状态，当前证据不足以定位受阻原因"
		return c
	}
	role, dir := "input", "in"
	switch issue.status {
	case "output-blocked":
		role, dir = "product", "out"
	case "return-blocked":
		role, dir = "return", "out"
	}
	addRoute := func(belt int64) {
		if _, ok := g.f.belts[belt]; !ok {
			return
		}
		flag := int64(0)
		if dir == "out" {
			flag = 1
		}
		key := [3]int64{belt, issue.item, flag}
		idx, ok := g.cache[key]
		if !ok {
			idx = len(r.Routes)
			g.cache[key] = idx
			r.Routes = append(r.Routes, g.f.trace(belt, dir, issue.item, g.incoming, g.attached))
		}
		if !slices.Contains(c.Routes, idx) {
			c.Routes = append(c.Routes, idx)
		}
		c.Shared = c.Shared || r.Routes[idx].Shared
	}
	c.Advice = "检查本机进料口及实际来料"
	if dir == "out" {
		c.Advice = "检查成品输出口及接收端是否继续收货"
	}
	if role == "return" {
		c.Advice = "检查原料回流出口及回流带的接收空间"
	}
	if d.Kind == "miner" || d.Kind == "fractionator" {
		c.Evidence = "unresolved"
		count := 0
		for _, p := range d.Ports {
			if p.Role == role {
				count++
				c.Evidence = "port"
				addRoute(p.Belt)
				if p.Entity > 0 && g.f.protos[p.Entity] > 0 {
					c.CheckEntity = p.Entity
				}
			}
		}
		if count > 1 {
			c.CheckEntity = d.EntityID
			c.Evidence = "multiple-ports"
			c.Advice += "，存在多个可用端口，需逐口确认"
		}
		if d.Kind == "miner" && g.f.veinStations[d.EntityID] {
			c.CheckEntity = d.EntityID
			c.Evidence = "integrated-station"
			c.Advice = "先检查大型矿机的内置物流货槽能否接收本机产物，再检查已连接的出货口"
		}
		if len(c.Routes) == 0 && c.Evidence == "unresolved" {
			c.Advice += "，未找到对应带口"
		}
		return c
	}
	seen := map[int64]bool{}
	for {
		seen[d.ComponentID] = true
		for _, s := range g.attached[d.EntityID] {
			out := s.pick == d.EntityID
			if !s.reversible && out != (dir == "out") || s.filter != 0 && s.filter != issue.item {
				continue
			}
			other := s.pick
			if out {
				other = s.drop
			}
			c.SorterEntities = append(c.SorterEntities, s.entity)
			addRoute(g.f.beltByEntity[other])
		}
		if len(c.SorterEntities) > 0 {
			c.Evidence = "sorter"
			break
		}
		lower := g.lower[d.ComponentID]
		if lower == nil || seen[lower.ComponentID] || lower.Kind != d.Kind || lower.TechID != d.TechID || lower.RecipeID != d.RecipeID {
			c.Evidence = "unresolved"
			break
		}
		d = *lower
		c.CheckEntity = d.EntityID
		c.Shared = true
	}
	if c.Shared {
		c.Advice += "，核实同一堆叠或共享带上的实际转运"
	}
	if c.Evidence == "unresolved" {
		c.Advice += "，未确认该方向的外部收发端"
	}
	return c
}

func (s *Snapshot) deviceReport(planet, item int64) DeviceReport {
	r := DeviceReport{SHA256: s.digest, Tick: s.tick, PlanetID: planet, Devices: []DeviceState{}, Checks: []DeviceCheck{}, Routes: []CircuitRoute{}}
	f := s.lines[planet]
	if f == nil || len(f.devices) == 0 {
		return r
	}
	r.QueuedTechID = f.currentTech
	if item != 0 && !slices.ContainsFunc(f.devices, func(d deviceRecord) bool { return d.ProductID == item }) {
		return r
	}
	g := newDeviceGraph(f)
	type key struct {
		status       string
		item, entity int64
	}
	indices := map[key]int{}
	for _, d := range f.devices {
		if item != 0 && d.ProductID != item {
			continue
		}
		v, issues := f.deviceState(d, s.gameVersion)
		r.Devices = append(r.Devices, v)
		for _, i := range issues {
			c := g.check(d, i, &r)
			k := key{c.Status, c.ItemID, c.CheckEntity}
			idx, ok := indices[k]
			if !ok {
				idx = len(r.Checks)
				indices[k] = idx
				r.Checks = append(r.Checks, c)
			}
			group := &r.Checks[idx]
			group.DeviceEntities = append(group.DeviceEntities, v.EntityID)
			if len(group.DeviceEntities) > 1 {
				group.Shared = true
			}
		}
	}
	sort.SliceStable(r.Checks, func(i, j int) bool {
		a, b := r.Checks[i], r.Checks[j]
		if (a.Status == "undetermined") != (b.Status == "undetermined") {
			return b.Status == "undetermined"
		}
		return len(a.DeviceEntities) > len(b.DeviceEntities)
	})
	return r
}
