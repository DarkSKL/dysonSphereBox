package saveanalysis

// 文件职责：保存采集、分馏、矩阵站与能量枢纽的存档字段。
// 使用：由厂区布局读取器校验实体并保留设备记录；边界：不判定停机原因或计算配方需求。

// DeviceState 职责：保存设备身份及存档字段；使用：布局读取器填充设备记录；边界：不包含产线诊断结论。
type DeviceState struct {
	// BufferItemID 职责：记录缓冲物品；使用：识别矿机旧产物；边界：可与当前 ProductID 不同。
	BufferItemID int64 `json:"bufferItemId,omitempty"`
	// Kind 职责：区分组件类别；使用：选择对应字段；边界：不表达工作状态。
	Kind string `json:"kind"`
	// ComponentID 职责：保存组件池编号；使用：校验记录有效性；边界：非实体编号。
	ComponentID int64 `json:"componentId"`
	// EntityID 职责：定位厂区实体；使用：关联标志与电力；边界：仅在本星球唯一。
	EntityID int64 `json:"entityId"`
	// Building 职责：保存建筑原型；使用：识别设备；边界：来自实体池。
	Building int64 `json:"building"`
	// ProductID 职责：记录当前产物；使用：保存设备配置；边界：未知或无产物时为零。
	ProductID int64 `json:"productId"`
	// RecipeID 职责：保留配方编号；使用：矩阵站记录；边界：不加载配方表。
	RecipeID int64 `json:"recipeId,omitempty"`
	// TechID 职责：保留科研编号；使用：科研站记录；边界：不计算科研需求。
	TechID int64 `json:"techId,omitempty"`
	// NextLabID 职责：保留堆叠引用；使用：矩阵站记录；边界：零表示无下一层。
	NextLabID int64 `json:"nextLabId,omitempty"`
	// Active 职责：保留工作标志；使用：有该字段的组件；边界：nil 表示未记录。
	Active *bool `json:"active,omitempty"`
	// Sign 职责：保存实体标志；使用：直接观测；边界：不映射为诊断结论。
	Sign int `json:"sign"`
	// Power 职责：保留电力动画值；使用：还原实体状态；边界：不是历史供电统计。
	Power float32 `json:"power"`
	// Progress 职责：保存组件进度；使用：还原缓冲状态；边界：单位由组件类型决定。
	Progress int64 `json:"progress"`
	// Cycle 职责：保存组件周期；使用：采集记录；边界：不从配方表补算。
	Cycle int64 `json:"cycle,omitempty"`
	// Buffered 职责：保存成品缓冲数量；使用：设备库存记录；边界：不是历史产量。
	Buffered int64 `json:"buffered"`
	// Capacity 职责：保存缓冲上限；使用：分馏组件；边界：未记录时为零。
	Capacity int64 `json:"capacity,omitempty"`
	// FluidID 职责：保存分馏原料编号；使用：区分输入物品；边界：零表示未确定。
	FluidID int64 `json:"fluidId,omitempty"`
	// FluidInput 职责：保存分馏输入缓冲；使用：组件还原；边界：不计算供给速率。
	FluidInput int64 `json:"fluidInput"`
	// FluidReturn 职责：保存分馏回流缓冲；使用：组件还原；边界：不追踪回流线路。
	FluidReturn int64 `json:"fluidReturn"`
	// ReturnCapacity 职责：保存回流容量；使用：分馏组件；边界：未记录时为零。
	ReturnCapacity int64 `json:"returnCapacity,omitempty"`
	// Veins 职责：保存矿脉编号；使用：矿机记录；边界：编号属于当前厂区。
	Veins []int64 `json:"veins,omitempty"`
	// ResourceRemaining 职责：合计已关联矿脉余量；使用：矿机记录；边界：不推断未来产能。
	ResourceRemaining int64 `json:"resourceRemaining,omitempty"`
	// MatrixPoints 职责：保存科研缓冲点数；使用：科研站记录；边界：每个完整矩阵为 3600 点。
	MatrixPoints []int64 `json:"matrixPoints,omitempty"`
	// Ports 职责：保存设备物理端口；使用：关联带段；边界：连接不代表实际运输。
	Ports []DevicePort `json:"ports,omitempty"`
}

// DevicePort 职责：保存设备带口或矿机接收实体；使用：设备解码关联端口；边界：只记录物理连接。
type DevicePort struct {
	// Role 职责：区分输入、成品或回流；使用：还原端口配置；边界：不是运输结论。
	Role string `json:"role"`
	// ItemID 职责：标识端口物品；使用：关联设备配置；边界：零表示未确定。
	ItemID int64 `json:"itemId"`
	// Belt 职责：保存带段池编号；使用：查询厂区带段；边界：不是实体编号。
	Belt int64 `json:"belt"`
	// Entity 职责：保存连接实体；使用：定位厂区建筑；边界：仅在本星球有效。
	Entity int64 `json:"entity"`
}

// veinSource 职责：保存矿脉物品与余量；使用：矿机解码关联；边界：不生成矿脉。
type veinSource struct{ item, amount int64 }

// deviceRecord 职责：保留设备公共字段及组件专属数组；使用：厂区解码内部；边界：不判读生产约束。
type deviceRecord struct {
	speedDamper float32
	working     bool
	DeviceState
	minerType, currentVein, target, speed int64
	belts                                 [3]int64 // 分馏器三个带口的池编号。
	out                                   [3]bool
	requires, served, products, produced  []int64
}

// deviceHeader 职责：读取池编号、实体并消费电力引用；使用：组件解码入口；边界：实体有效性由 keepDevice 校验。
func (r *reader) deviceHeader(kind string) deviceRecord {
	d := deviceRecord{DeviceState: DeviceState{Kind: kind, ComponentID: r.number(4, true), EntityID: r.number(4, true)}}
	r.skip(4)
	return d
}

// deviceArray 职责：读取带长度前缀的设备数组；使用：矩阵站解码；边界：非法长度或短读终止解析。
func (r *reader) deviceArray(width int64) []int64 {
	return r.lineValues(r.length(r.number(4, true)), width)
}

// keepDevice 职责：校验实体并保留设备与端口；使用：组件读取完成后调用；边界：空池槽忽略，无效实体终止解析。
// 逻辑流：1. 验证组件与实体 => 2. 关联实体标志和电力 => 3. 恢复设备端口 => 4. 保存记录。
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
