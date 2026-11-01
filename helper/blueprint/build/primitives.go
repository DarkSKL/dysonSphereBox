package build

// 文件职责：保留 f64 装配坐标及建筑引用；使用：各布局策略；边界：仅在最终编码前转 f32，索引在重排后统一赋值。
// 几何与原型事实核对 dsp-calc data/config.ts、itemsData.js 和 BuleprintMaker.ts，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	_ "embed"
	"encoding/json"
	"math"
)

// modelData 职责：内置模型原型映射；使用：创建建筑；边界：纯数值事实，不含游戏美术。
//
//go:embed models.json
var modelData []byte

// vector 职责：保存装配期间 f64 坐标；使用：平移和几何；边界：不提前截断。
type vector struct{ x, y, z float64 }

// placed 职责：以指针维护建筑连接；使用：合并重排；边界：最终 index 与数组映射独立赋值。
type placed struct {
	// record 职责：字段与参数；使用：最终转换；边界：坐标由 a/b 覆盖。
	record blueprint.Building
	// a 职责：首端坐标；使用：布局；边界：f64。
	a vector
	// b 职责：次端坐标；使用：分拣器；边界：f64。
	b vector
	// input 职责：输入引用；使用：最终编号；边界：nil 表示无连接。
	input *placed
	// output 职责：输出引用；使用：最终编号；边界：nil 表示无连接。
	output *placed
	// owner 职责：产线归属；使用：bindings；边界：-1 为公共设施。
	owner int
}

// stationTransportLink 职责：保存物流塔路线与规范物料的实体绑定；使用：mode3/4收尾诊断；边界：仅描述生成器已装配的塔端口，不宣称机队吞吐。
type stationTransportLink struct {
	item   int
	output bool
	tower  *placed
	belt   *placed
	slot   int
	head   *placed
	path   []*placed
	owner  int
	rate   float64
}

// busTransportLink 职责：保存混带总线实体端点、共享环路和过滤分拣器；使用：mode6 流边收尾核验；边界：不模拟总线逐帧物料位置。
type busTransportLink struct {
	// kind 职责：区分生产输入、生产回流和外部边界；使用：按 FlowGraph 边配对；边界：未知类型拒绝核验。
	kind string
	// owner 职责：保存生产节点原始索引；使用：绑定 FlowGraph production 节点；边界：外部端点为 -1。
	owner int
	// item 职责：保存该实体路线的物品；使用：过滤和供需匹配；边界：零值不代表任意物品。
	item int
	// layer 职责：保存所属共享环路编号；使用：合计每圈带负载；边界：未分配为 -1。
	layer int
	// rate 职责：保存该生产行计划速率或边界流量；使用：路线覆盖核对；边界：不扩张为机器额定能力。
	rate float64
	// endpoint 职责：保存生产带或外部边界带；使用：结果 BuildingIndices/BoundaryPort；边界：必须属于 builder.all。
	endpoint *placed
	// ring 职责：保存共享总线上的实际带；使用：确认环路接入；边界：仅代表一个实体带节点。
	ring *placed
	// ringPath 职责：保存共享总线闭环的有向带序；使用：lane 建筑索引和闭环验证；边界：不宣称游戏内物品时序。
	ringPath []*placed
	// parts 职责：保存该路线创建的分拣器及附加带；使用：实体索引和端点核对；边界：不代表未创建的隐含路线。
	parts []*placed
	// limiterParts 职责：保存产物返回线上的实际门控分拣器；使用：累计外输容量；边界：输出带未设置专用 item filter。
	limiterParts []*placed
	// gateFeed 职责：保存输出带到回流门控臂的实际带链；使用：验证限速拓扑；边界：只包含本产线支路。
	gateFeed []*placed
	// gateReturn 职责：保存回流门控臂之后的实际带链；使用：验证方向和实体索引；边界：不含共享环路。
	gateReturn []*placed
	// gateEndpoint 职责：保存回流限速线接回产线后端的带端点；使用：验证门控带链方向；边界：与进入总线的另一端分开记录。
	gateEndpoint *placed
	// itemRates 职责：保存回流支路按物品拆分的计划流量；使用：多产物混带逐边匹配；边界：不改变量化结果。
	itemRates []quant.ItemRate
	// rateLimit 职责：保存总线装配器设定的回流上限；使用：限制通过门控线的核验流量；边界：不是机器额定产出。
	rateLimit float64
	// row 职责：保存产线物品带上的真实方向链；使用：确认被过滤带接入生产行；边界：不包含共享环路全体节点。
	row []*placed
}

// flyTransportEndpoint 职责：登记配送器/分流器模式的一组真实设施端点；使用：把规范 FlowGraph 边映射到箱体、配送器和接驳带；边界：机队或远程分流的运行时吞吐仍由游戏负责。
type flyTransportEndpoint struct {
	// owner 职责：保存所属量化产线索引；使用：映射 production 节点；边界：沿用 builder.owner，不从坐标猜测。
	owner int
	// item 职责：保存端点过滤物品；使用：匹配规范边；边界：一个端点只承载一个物品。
	item int
	// input 职责：区分原料配送入口和产物配送出口；使用：供需方向验证；边界：true 表示物料进入机器行。
	input bool
	// rate 职责：保存该机器行端点按计划应承载的每分钟流量；使用：逐边容量分配；边界：不是配送器额定吞吐承诺。
	rate float64
	// link 职责：保存机器行一侧的真实带端点；使用：诊断接驳；边界：不代表远程网络已连通。
	link *placed
	// belts 职责：保存端点与机器行之间的实体带链；使用：最终索引和带宽核验；边界：方向由 placed.output/input 关系决定。
	belts []*placed
	// box 职责：保存 2101 箱体；使用：确认配送器实际挂接；边界：箱容量不等于持续供给。
	box *placed
	// delivery 职责：保存 2107 配送器；使用：规范边的实体端口；边界：FilterID 必须与 item 一致。
	delivery *placed
	// arm 职责：保存 mode2 箱体与接驳带之间的分拣器；使用：验证端口跨度和过滤；边界：mode5 由分流器带链替代。
	arm *placed
	// arms 职责：保存同一箱体并联的全部真实分拣器；使用：容量拆分与最终索引；边界：arm 仅保留首个兼容引用。
	arms []*placed
	// filters 职责：保存配送端点前的物品过滤分拣器；使用：多产物逐物品路径；边界：每个对象必须有对应 FilterID。
	filters []*placed
	// splitter 职责：保存 mode5 四向分流器；使用：确认分流模式端点；边界：mode2 必须为空。
	splitter *placed
}

// builder 职责：收集单次装配；使用：六种模式；边界：不做文件写入。
type builder struct {
	// generationCtx 职责：保存本次生成的取消和总截止时间；使用：局部装配、占位和补电的长循环；边界：纯几何测试未设置时不附加截止时间，不跨请求复用。
	generationCtx context.Context
	// all 职责：最终建筑顺序；使用：编码；边界：每个指针只出现一次。
	all []*placed
	// models 职责：原型模型表；使用：创建；边界：未知 ID 拒绝。
	models map[string]int16
	// owner 职责：当前装配的产线；使用：归属登记；边界：跨块不依赖 recipeId 推测。
	owner int
	// setting 职责：归一设置；使用：速率/尺寸；边界：只在请求内使用。
	setting Setting
	// warnings 职责：非阻断告警；使用：返回 UI；边界：不得静默丢产线。
	warnings []string
	// coaterLinks 职责：登记每条喷涂供剂带及剂种；使用：可选网络；边界：指针随块一起平移。
	coaterLinks map[int][]*placed
	// zones 职责：多目标分区定义；使用：结果标注；边界：总线模式为空。
	zones []Zone
	// zoneRanges 职责：分区在 all 中的连续区间；使用：收尾计算边界；边界：喷涂网络等公共设施追加在区间之外。
	zoneRanges []zoneRange
	// surfaceBlocks 职责：保存标准模式中不可拆分的产线块范围；使用：球面多纬带装箱；边界：仅记录连续切片，不改变建筑连接。
	surfaceBlocks []surfaceBlockRange
	// surfaceBusEncoding 职责：保存共享总线跨纬带重接后的区域坐标系；使用：统一编码出口；边界：仅成功完成全部物理迁移与接驳后设置，不能再按平面原点归一化。
	surfaceBusEncoding *surfaceEncoding
	// flowGraph 职责：保留可信字段重建的守恒流图；使用：全部模式的顺序、容量和诊断；边界：不信任请求自带的边。
	flowGraph *quant.FlowGraph
	// transportRoutes 职责：保存 lane 与真实建筑指针的关联；使用：重排后的最终连通检查；边界：索引只在统一编号后生成。
	transportRoutes []*transportRoute
	// machineRecycles 职责：保存机器内部毛流回收的物理证据；使用：直连/总线局部路由和共享带复核；边界：不加入规范净流账或额外外供份额。
	machineRecycles []*machineRecycle
	// connectedCompactBelts 职责：启用完整直连路线的闲置带尾裁剪；使用：生成候选与同输入基线对照；边界：不参与装箱或占位判断，必须在路由之后执行以保留原空间布局。
	connectedCompactBelts bool
	// connectedLegacyInputs 职责：为就近入口无可行候选时保留原集中入口退路；使用：mode1完整重建的第二组候选；边界：不得复用失败候选的路径、实体或诊断，mode6仍使用自己的原入口。
	connectedLegacyInputs bool
	// logistics 职责：积累本次生成的物流与空间证据；使用：finish；边界：硬错误不随结果返回部分蓝图。
	logistics *LogisticsSummary
	// stationLinks 职责：保存每条塔路线；使用：模式3/4按FlowGraph核验；边界：不跨请求复用。
	stationLinks []stationTransportLink
	// flyEndpoints 职责：保存 mode2/5 每条产线端点；使用：按 FlowGraph 逐边绑定；边界：没有端点的规范边必须硬失败。
	flyEndpoints []*flyTransportEndpoint
	// flyCompact 职责：启用配送候选的紧凑本地端点余量；使用：有界布局候选；边界：基线保持原几何，紧凑方案仍经过同一预提交和最终规则。
	flyCompact bool
	// busLinks 职责：保存混带输入、回流及边界的实体引用；使用：mode6 逐边绑定与容量校验；边界：不跨请求复用。
	busLinks []busTransportLink
	// busLayers 职责：保存物品到共享环路的确定映射；使用：检查每圈累计带负载；边界：未调度物品不得默认归入第一圈。
	busLayers map[int]int
	// busLayerBelts 职责：保存各共享环路的实体带；使用：确认环路方向与lane端点；边界：只包括本次生成的建筑。
	busLayerBelts map[int][]*placed
	// busFabric 职责：保存按实际 lane 拆圈的混带物流证据；使用：新版 mode6 收尾；边界：旧三圈装配器没有该数据，仍走原有严格验证。
	busFabric []*busFabricRoute
}

// checkGeneration 职责：统一把本次生成取消转换为可识别超时错误；使用：局部有界循环按固定间隔调用；边界：nil 上下文仅供独立几何调用，不改变服务层任务串行语义。
func (b *builder) checkGeneration(stage string) error {
	if b.generationCtx != nil && b.generationCtx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "蓝图生成已取消或超过总截止时间：" + stage}
	}
	return nil
}

// zoneRange 职责：记录分区建筑区间；使用：分区边界；边界：半开区间 [begin, end)。
type zoneRange struct {
	zone, begin, end int
}

// surfaceBlockRange 职责：描述 builder.all 中一个可整体平移的产线块；使用：球面布局在多个 Area 间装箱；边界：块内连接保持，跨块连接需由收尾阶段单独处理。
type surfaceBlockRange struct {
	begin, end int
}

// newBuilder 职责：创建纯内存装配器；使用：Generate；边界：模型数据损坏时返回错误。
func newBuilder(s Setting) (*builder, error) {
	b := &builder{setting: s, models: map[string]int16{}, warnings: []string{}, coaterLinks: map[int][]*placed{}}
	if err := json.Unmarshal(modelData, &b.models); err != nil {
		return nil, err
	}
	return b, nil
}

// make 职责：建立默认记录；使用：各类型建筑；边界：调用方负责合法原型，最终统一校验。
func (b *builder) make(id int, x, y, z float64) *placed {
	p := &placed{record: blueprint.Building{Num: -102, RecordLayout: "v2", ItemID: int16(id), ModelIndex: b.models[itoa(id)], Yaw: 90, Yaw2: 90, OutputObjIdx: -1, InputObjIdx: -1, InputToSlot: 1}, a: vector{x, y, z}, b: vector{x, y, z}, owner: b.owner}
	b.all = append(b.all, p)
	return p
}

// itoa 职责：将原型整数转换为模型键；使用：查模型；边界：不承担数据解析。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var out [20]byte
	i := len(out)
	for v > 0 {
		i--
		out[i] = byte('0' + v%10)
		v /= 10
	}
	return string(out[i:])
}

// connect 职责：连接有向带并设置待编号的默认输入槽；使用：路径装配；边界：多路汇流的独占输入槽由records统一分配，不写带到带的反向输入字段。
func connect(a, b *placed) { a.output = b; a.record.OutputToSlot = 1 }

// reverse 职责：反转原数组中的带方向；使用：输入/输出转换；边界：保持数组位置稳定。
func reverse(line []*placed) {
	for i, p := range line {
		p.record.Yaw = float32(math.Mod(float64(p.record.Yaw)+180, 360))
		p.record.Yaw2 = p.record.Yaw
		p.output = nil
		p.record.OutputToSlot = 0
		if i > 0 {
			p.output = line[i-1]
			p.record.OutputToSlot = 1
		}
	}
}

// straight 职责：创建整数格直线；使用：机器两侧带；边界：方向 0 保留 -90 原始约定。
func (b *builder) straight(x, y float64, count, direction int) []*placed {
	result := make([]*placed, 0, count)
	for i := 0; i < count; i++ {
		px, py := x, y
		switch direction {
		case 1:
			py += float64(i)
		case 2:
			px += float64(i)
		case 3:
			py -= float64(i)
		case 4:
			px -= float64(i)
		}
		p := b.make(2000+b.setting.BeltLv, px, py, 0)
		p.record.Yaw = float32((direction - 1) * 90)
		p.record.Yaw2 = p.record.Yaw
		if i > 0 {
			connect(result[i-1], p)
		}
		result = append(result, p)
	}
	return result
}

// byPath 职责：按几何点装配带；使用：路由和桥接；边界：路径须由上层验证，不跨层猜测。
func (b *builder) byPath(points []vector) []*placed {
	result := make([]*placed, 0, len(points))
	for _, v := range points {
		p := b.make(2000+b.setting.BeltLv, v.x, v.y, v.z)
		if len(result) > 0 {
			connect(result[len(result)-1], p)
		}
		result = append(result, p)
	}
	return result
}

// shift 职责：整体平移；使用：块合并；边界：两端坐标同时移动，连接引用不变。
func shift(all []*placed, x, y, z float64) {
	for _, p := range all {
		p.a.x += x
		p.a.y += y
		p.a.z += z
		p.b.x += x
		p.b.y += y
		p.b.z += z
	}
}

// footprint 职责：返回固定朝向的原型占地矩形；使用：合并范围及路由避让；边界：数值来自参考碰撞表，不代表游戏最终碰撞判定。
func footprint(id int16) (vector, vector) {
	switch id {
	case 2103, 2104:
		return vector{-3, -3, 0}, vector{3, 3, 0}
	case 2101, 2020:
		return vector{-2, -2, 0}, vector{2, 2, 0}
	}
	typ, _ := quant.MachineKind(int(id))
	switch typ {
	case 1:
		return vector{-1, -1, 0}, vector{1, 1, 0}
	case 2, 6:
		return vector{-2, -2, 0}, vector{2, 2, 0}
	case 3:
		return vector{-2, -1, 0}, vector{3, 1, 0}
	case 4:
		return vector{-3, -1, 0}, vector{3, 2, 0}
	case 5:
		return vector{-4, -2, 0}, vector{3, 2, 0}
	}
	return vector{}, vector{}
}

// bounds 职责：计算含建筑占地的块范围；使用：间距/限宽；边界：包含两端点和原型碰撞表，不替代游戏碰撞检查。
func bounds(all []*placed) (min, max vector) {
	min = vector{math.Inf(1), math.Inf(1), math.Inf(1)}
	max = vector{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range all {
		lo, hi := footprint(p.record.ItemID)
		for _, v := range []vector{p.a, p.b, {p.a.x + lo.x, p.a.y + lo.y, p.a.z}, {p.a.x + hi.x, p.a.y + hi.y, p.a.z}} {
			min.x = math.Min(min.x, v.x)
			min.y = math.Min(min.y, v.y)
			min.z = math.Min(min.z, v.z)
			max.x = math.Max(max.x, v.x)
			max.y = math.Max(max.y, v.y)
			max.z = math.Max(max.z, v.z)
		}
	}
	return
}

// records 职责：统一编号和转换；使用：编码前；边界：bindings 使用数组下标，连接使用独立记录索引。
func (b *builder) records() ([]blueprint.Building, []ProductionBinding, []int, error) {
	out := make([]blueprint.Building, len(b.all))
	indices := map[*placed]int{}
	bindings := map[int][]int{}
	machines := map[int][]int{}
	shared := []int{}
	for i, p := range b.all {
		if _, ok := indices[p]; ok {
			return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑指针重复"}
		}
		indices[p] = i
	}
	for i, p := range b.all {
		r := p.record
		if r.ModelIndex == 0 {
			return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "未知建筑模型: " + itoa(int(r.ItemID))}
		}
		r.Index = int32(i)
		r.LocalOffset = blueprint.Point{X: float32(p.a.x), Y: float32(p.a.y), Z: float32(p.a.z)}
		r.LocalOffset2 = blueprint.Point{X: float32(p.b.x), Y: float32(p.b.y), Z: float32(p.b.z)}
		if p.input != nil {
			index, ok := indices[p.input]
			if !ok {
				return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "输入连接指向蓝图外部"}
			}
			r.InputObjIdx = int32(index)
		}
		if p.output != nil {
			index, ok := indices[p.output]
			if !ok {
				return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "输出连接指向蓝图外部"}
			}
			r.OutputObjIdx = int32(index)
		}
		out[i] = r
		if p.owner < 0 {
			shared = append(shared, i)
		} else {
			bindings[p.owner] = append(bindings[p.owner], i)
			if r.RecipeID > 0 {
				machines[p.owner] = append(machines[p.owner], i)
			}
		}
	}
	if err := assignBeltInputSlots(out); err != nil {
		return nil, nil, nil, err
	}
	result := []ProductionBinding{}
	for owner := 0; owner < len(b.all); owner++ {
		if indices, ok := bindings[owner]; ok {
			result = append(result, ProductionBinding{ProductionIndex: owner, BuildingIndices: indices, MachineIndices: machines[owner]})
		}
	}
	return out, result, shared, nil
}
