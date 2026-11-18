package saveanalysis

// 文件职责：保存厂区解码时读取的实体、设备、带线和缓冲记录。
// 使用：由存档布局读取器构建快照；边界：只保存事实，不生成产线诊断或检查建议。

// factoryLine 职责：保存厂区实体连接与设备原始状态；使用：布局读取器逐区段填充；边界：不遍历连接或生成诊断。
type factoryLine struct {
	drops map[int64]DropObservation // 有实载的分拣器保存状态，按实体查询。
	// veinStations 标识矿机物流站实体，供快照保留组件身份，不推断供货。
	veinStations    map[int64]bool
	devices         []deviceRecord
	veins           map[int64]veinSource
	currentTech     int64 // 当前科技编号，与 researchPending 共同保存科技队列状态。
	researchPending bool
	machines        []circuitRecord
	sorters         []lineSorter
	beltByEntity    map[int64]int64 // 实体编号映射到带段池编号，二者不可混用。
	belts           map[int64]lineBelt
	sign            []byte    // 实体 i 的标志位于 i-1
	power           []float32 // 动画池含 0 号位置
	protos          map[int64]int64
	ports           map[int64][]linePort // 按带段池编号保存物流站端口。
}

// linePort 职责：保存端口实体、方向和物品；使用：物流区段关联带段；边界：方向不代表实际流量。
type linePort struct{ entity, dir, item int64 }

// lineBelt 职责：保存带段实体和出口编号；使用：厂区解码建立索引；边界：不沿出口遍历。
type lineBelt struct{ entity, output int64 }

// lineSorter 职责：保存分拣器取放点和过滤配置；使用：厂区解码保留连接；边界：不推断吞吐或停机原因。
type lineSorter struct {
	entity, pick, drop, filter int64
	reversible                 bool
}

// circuitRecord 职责：保存制造台配方和缓冲字段；使用：装配机区段读取器填充；边界：不套用配方表计算需求。
type circuitRecord struct {
	id, entity, recipe, kind int64
	active                   bool
	progress                 int64
	requires, served         []int64
	needs                    []int64 // 保存时的物品请求。
	products, produced       []int64
}

// newFactoryLine 职责：初始化厂区记录容器；使用：读取每个厂区前调用；边界：不共享不同星球的映射。
func newFactoryLine() *factoryLine {
	return &factoryLine{veins: map[int64]veinSource{}, beltByEntity: map[int64]int64{}, belts: map[int64]lineBelt{}, protos: map[int64]int64{}, ports: map[int64][]linePort{}}
}

// addBelt 职责：同时登记带段和实体索引；使用：读取有效带段后调用；边界：不创建连接以外的推断数据。
func (f *factoryLine) addBelt(id, entity, out int64) {
	f.belts[id] = lineBelt{entity, out}
	f.beltByEntity[entity] = id
}

// lineValues 职责：读取指定宽度的有符号数组；使用：调用方先校验元素数；边界：短读沿用 reader 的错误处理。
func (r *reader) lineValues(n, size int64) []int64 {
	v := make([]int64, n)
	for i := range v {
		v[i] = r.number(size, true)
	}
	return v
}
