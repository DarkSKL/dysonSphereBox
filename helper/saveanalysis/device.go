package saveanalysis

// 文件职责：判读采集、分馏、矩阵生产及科研组件；使用：只读快照查询；边界：保存时的直接限制不能换算为历史停机时长。

import "slices"

// DeviceState 保留星球内的实体定位和本机证据。科研缓冲按游戏矩阵点数计，每个完整矩阵为 3600 点。
type DeviceState struct {
	BufferItemID      int64          `json:"bufferItemId,omitempty"`
	RequiredPoints    []int64        `json:"requiredPoints,omitempty"`
	Kind              string         `json:"kind"`
	ComponentID       int64          `json:"componentId"`
	EntityID          int64          `json:"entityId"`
	Building          int64          `json:"building"`
	ProductID         int64          `json:"productId"`
	RecipeID          int64          `json:"recipeId,omitempty"`
	TechID            int64          `json:"techId,omitempty"`
	NextLabID         int64          `json:"nextLabId,omitempty"`
	Active            *bool          `json:"active,omitempty"`
	Sign              int            `json:"sign"`
	Power             float32        `json:"power"`
	Progress          int64          `json:"progress"`
	Cycle             int64          `json:"cycle,omitempty"`
	Buffered          int64          `json:"buffered"`
	Capacity          int64          `json:"capacity,omitempty"`
	FluidID           int64          `json:"fluidId,omitempty"`
	FluidInput        int64          `json:"fluidInput"`
	FluidReturn       int64          `json:"fluidReturn"`
	ReturnCapacity    int64          `json:"returnCapacity,omitempty"`
	Veins             []int64        `json:"veins,omitempty"`
	ResourceRemaining int64          `json:"resourceRemaining,omitempty"`
	MatrixPoints      []int64        `json:"matrixPoints,omitempty"`
	Inputs            []CircuitInput `json:"inputs,omitempty"`
	Ports             []DevicePort   `json:"ports,omitempty"`
	Status            string         `json:"status"`
}

// DevicePort 是设备本身的带口或矿机的接收实体。回流口输送剩余原料。
type DevicePort struct {
	Role   string `json:"role"`
	ItemID int64  `json:"itemId"`
	Belt   int64  `json:"belt"`
	Entity int64  `json:"entity"`
}
type veinSource struct{ item, amount int64 }
type deviceRecord struct {
	speedDamper float32
	working     bool
	DeviceState
	minerType, currentVein, target, speed int64
	belts                                 [3]int64
	out                                   [3]bool
	requires, served, products, produced  []int64
}

// deviceHeader 读取三种组件共有的池编号、实体及电力组件引用。
func (r *reader) deviceHeader(kind string) deviceRecord {
	d := deviceRecord{DeviceState: DeviceState{Kind: kind, ComponentID: r.number(4, true), EntityID: r.number(4, true)}}
	r.skip(4)
	return d
}
func (r *reader) deviceArray(width int64) []int64 {
	return r.lineValues(r.length(r.number(4, true)), width)
}
func (r *reader) keepDevice(d deviceRecord) {
	if d.ComponentID <= 0 {
		return
	}
	f := r.line
	if d.EntityID <= 0 || d.EntityID > int64(len(f.sign)) || d.EntityID >= int64(len(f.power)) || f.protos[d.EntityID] == 0 {
		r.fail("生产组件引用了无效实体")
	}
	if d.Kind == "matrix-lab" || d.Kind == "research-lab" || d.Kind == "fractionator" {
		active := d.working
		d.Active = &active
	}
	d.Sign = int(f.sign[d.EntityID-1])
	d.Power = f.power[d.EntityID]
	d.Building = f.protos[d.EntityID]
	switch d.Kind {
	case "miner":
		if d.target > 0 {
			item := d.ProductID
			if d.Buffered > 0 && d.BufferItemID > 0 {
				item = d.BufferItemID
			}
			d.Ports = []DevicePort{{"product", item, f.beltByEntity[d.target], d.target}}
		}
	case "fractionator":
		if d.FluidID == 0 && d.Buffered == 0 && d.FluidReturn == 0 && d.FluidInput == 0 {
			d.ProductID = 0
		}
		for i, belt := range d.belts {
			if belt <= 0 {
				continue
			}
			role, item := "input", d.FluidID
			if i == 0 {
				if !d.out[i] {
					continue
				}
				role, item = "product", d.ProductID
			} else if d.out[i] {
				role = "return"
			}
			d.Ports = append(d.Ports, DevicePort{role, item, belt, f.belts[belt].entity})
		}
	}
	f.devices = append(f.devices, d)
}

type deviceIssue struct {
	status string
	item   int64
}

// deviceState 按组件自身运行规则给出直接限制；同机多项约束均保留在检查项中。
func (f *factoryLine) deviceState(d deviceRecord, version string) (DeviceState, []deviceIssue) {
	v := d.DeviceState
	v.Veins = slices.Clone(v.Veins)
	v.MatrixPoints = slices.Clone(v.MatrixPoints)
	v.Ports = slices.Clone(v.Ports)
	if v.Active != nil {
		active := *v.Active
		v.Active = &active
	}
	status := "undetermined"
	var issues []deviceIssue
	switch {
	case d.Kind == "power-exchanger":
		status = "source-located"
	case version != productionRecipeVersion:
	case d.Kind == "research-lab":
		points, ok := researchPoints[d.TechID]
		if ok && d.TechID == f.currentTech {
			v.RequiredPoints = points[:]
		}
		switch {
		case d.TechID <= 0:
			status = "idle"
		case !ok || !f.researchPending || d.TechID != f.currentTech || len(d.MatrixPoints) != 6:
		case v.Power < 0.1:
			status = "low-power"
		case d.working && v.Sign == 0:
			status = "processing"
		case !d.working && v.Sign == 6:
			for i, minimum := range points {
				if minimum > 0 && d.MatrixPoints[i] < minimum {
					issues = append(issues, deviceIssue{"research-input-missing", 6001 + int64(i)})
				}
			}
		}
	case d.Kind == "matrix-lab":
		recipe, ok := productionRecipes[d.RecipeID]
		matched := ok && recipe.kind == 15 && slices.Equal(recipe.inputs, d.requires) && slices.Equal(recipe.products, d.products) && len(d.products) == 1
		for i, item := range d.requires {
			in := CircuitInput{ItemID: item, Available: d.served[i]}
			if matched {
				in.Required = recipe.counts[i]
			}
			v.Inputs = append(v.Inputs, in)
		}
		if matched {
			v.Cycle = recipe.ticks * 10000
			v.Capacity = 10 * ((d.speed + 9999) / 10000)
		}
		switch {
		case d.RecipeID <= 0:
			status = "idle"
		case v.Power < 0.1:
			status = "low-power"
		case d.working && v.Sign == 0:
			status = "processing"
		case matched && !d.working && v.Sign == 6 && d.Progress >= v.Cycle && d.Buffered+recipe.batches[0] > v.Capacity:
			issues = append(issues, deviceIssue{"output-blocked", d.ProductID})
		case matched && !d.working && v.Sign == 6 && d.Progress == 0:
			for _, i := range v.Inputs {
				if i.Available < i.Required {
					issues = append(issues, deviceIssue{"input-missing", i.ItemID})
				}
			}
		}
	case d.Kind == "miner":
		v.Capacity = 50
		switch {
		case d.minerType < 1 || d.minerType > 3:
		case d.minerType != 1 && len(d.Veins) == 0:
			status = "no-source"
		case v.Power < 0.1:
			status = "low-power"
		case v.ProductID <= 0 || d.Cycle <= 0 || (d.minerType != 1 && d.ResourceRemaining <= 0):
		case d.Progress >= d.Cycle && d.Buffered >= v.Capacity:
			item := d.BufferItemID
			if item == 0 {
				item = d.ProductID
			}
			issues = append(issues, deviceIssue{"output-blocked", item})
		case d.minerType == 2 && d.Progress >= d.Cycle && d.BufferItemID > 0 && d.BufferItemID != d.ProductID:
			issues = append(issues, deviceIssue{"output-blocked", d.BufferItemID})
		case d.Progress < d.Cycle && d.speed > 0 && d.speedDamper > 0:
			status = "processing"
		}
	case d.Kind == "fractionator":
		switch {
		case v.Power < 0.1:
			status = "low-power"
		case d.FluidInput > 0 && d.working && d.Buffered < d.Capacity && d.FluidReturn < d.ReturnCapacity:
			status = "processing"
		case !d.working:
			if d.FluidInput == 0 {
				issues = append(issues, deviceIssue{"input-missing", d.FluidID})
			}
			if d.Capacity > 0 && d.Buffered >= d.Capacity {
				issues = append(issues, deviceIssue{"output-blocked", d.ProductID})
			}
			if d.ReturnCapacity > 0 && d.FluidReturn >= d.ReturnCapacity {
				issues = append(issues, deviceIssue{"return-blocked", d.FluidID})
			}
		}
	}
	if len(issues) > 0 {
		status = issues[0].status
	} else if status != "processing" && status != "idle" && status != "source-located" {
		issues = []deviceIssue{{status, 0}}
	}
	v.Status = status
	return v, issues
}
