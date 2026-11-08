package build

// 文件职责：保留 f64 装配坐标及建筑引用；使用：各布局策略；边界：仅在最终编码前转 f32，索引在重排后统一赋值。
// 几何与原型事实核对 dsp-calc data/config.ts、itemsData.js 和 BuleprintMaker.ts。

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
	// densityCargo 职责：保存参考臂实际取放目标与独立物料轨道；使用：最终货物采样重绑；边界：值类型元数据及只读轨道切片可随候选快照安全复制。
	densityCargo denseCargoTarget
	// densityThirdInput 职责：标识参考单元派生的第三输入臂；使用：目标纬度局部端口适配；边界：仅内部装配元数据，不写入蓝图。
	densityThirdInput bool
	// densityPattern 职责：记录实际机器局部布局来源；使用：获选候选的密度报告；边界：仅生成器写入，不能由请求自证。
	densityPattern string
	// previewID 职责：保存可选任务内对象身份；使用：临时预览增量；边界：不参与记录编号、连接或几何判断。
	previewID string
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
	// productionRate 职责：保留真实机器带的双速率标注；使用：最终编号后输出诊断；边界：不参与期望净流账，也不改变连接。
	productionRate *ProductionRateAnnotation
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
	// sprayRecycle 职责：保存自产供剂的机器行证据；使用：同塔库存真实出带的独立毛流核验；边界：不作为规范净流需求或外供。
	sprayRecycle *line
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
	// flowEdgeID 职责：限定独立副产物回收端点所属规范边；使用：共享产物拆分后的稳定绑定；边界：空值仍按普通供需端点分配，不改变规范边或产量。
	flowEdgeID string
	// leftoverAnchor 职责：保存实际外供箱体所在原料区的空间锚点；使用：回收端点布局验证；边界：仅表示邻近，不建立供给连接或循环消费。
	leftoverAnchor *placed
	// leftoverAnchorMatched 职责：标记回收锚点是否拥有同物品的真实输入路线；使用：边界摘要区分可复用入口与仅空间归集；边界：false 时不得宣称已回流或已消费。
	leftoverAnchorMatched bool
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
	// selfSpray 职责：保存自产供剂的行、过滤臂和真实回流链；使用：独立重算本地出货毛流；边界：rate 始终保留规范净流。
	selfSpray *flySelfSpray
}

// builder 职责：收集单次装配；使用：六种模式；边界：不做文件写入。
type builder struct {
	// preview 职责：共享任务内旁路通知器；使用：真实提交和回退；边界：普通生成为空，不参与算法决策。
	preview *generationPreview
	// previewActive 职责：区分真实候选与内部几何探测；使用：决定检查点是否可见；边界：新建试装 Builder 默认不发布。
	previewActive bool
	// adapter 职责：保存当前生成组合的阶段实现；使用：统一收尾调用模式钩子；边界：每个 builder 生命周期只绑定一个适配器。
	adapter *modeAdapter
	// denseSpatial 职责：启用全场实体占位搜索；使用：超密度及普通模式共享设施候选；边界：具体碰撞与间距仍按适配器密度决定。
	denseSpatial bool
	// denseSpatialOrder 职责：选择全场装配顺序；使用：有界密铺比较；边界：只改变空间顺序，不改变物料份额。
	denseSpatialOrder int
	// denseMaxSpan 职责：限制当前密铺候选的机器长轴跨度；使用：保容量均衡细分；边界：零值不额外细分。
	denseMaxSpan float64
	// denseLatitudeSpan 职责：限制独立大厂候选的工作格纬向高度；使用：窄纬长矩形与真实共享物料相邻落位；边界：零保留旧预算，单元自身不得撑大该上限。
	denseLatitudeSpan float64
	// denseStationSprayNetwork 职责：保存塔黑盒外供剂的真实串供与规范份额映射；使用：独立大厂候选；边界：不混入配方原料或自产回流，不假设自喷节省。
	denseStationSprayNetwork *denseStationSprayPlan
	// denseRefine 职责：在整数全场候选附近继续搜索四分之一格接触位置；使用：独立优化候选；边界：仍需真实短接驳和完整终检。
	denseRefine bool
	// connectedBestObjective 职责：保留本次请求已完整验证的最优矩形指标；使用：固定Area的供电前严格劣解剪枝；边界：不作为通过证明，不影响普通模式。
	connectedBestObjective *LayoutMetrics
	// denseStreetCandidate/denseStreetRotation 职责：启用连续机器街道完整候选及整场四向比较；使用：mode1独立装配；边界：不改变普通或其它密铺候选。
	denseStreetCandidate bool
	denseStreetRotation  int
	// denseStreetGroupLimit 职责：为完整候选指定每条双排街道的容量段数；使用：补电或接线失败后的几何比较；边界：零使用核心候选、负数保持所有组同行。
	denseStreetGroupLimit int
	// densePairedOutputs 职责：启用真实多去向双侧过滤候选；使用：缩短单侧长干线；边界：每个抓取点及所选等级过滤臂仍独立。
	densePairedOutputs bool
	// directBoundaryUnits 职责：为内部直连的黑盒候选提供真实边界设施；使用：机器端口建成后、全场占位前；边界：普通模式为空，回调必须给每条外部边分配真实端点。
	directBoundaryUnits func(context.Context, []*transportRow) ([]denseSpatialUnit, error)
	// directBoundaryCommit 职责：在内部共享网络恢复规范路线后提交独立边界网络；使用：真实全图路由完成且未裁尾/补电前；边界：只能改其拥有的规范份额，不覆盖其它网络恢复结果。
	directBoundaryCommit func(context.Context) error
	// directBoundaryAttach 职责：把已生成边界辅助节点归入真实刚性集合；使用：边界准备后、裁尾及轮廓估算前；边界：只补实际归属，不创建虚拟实体或改变排序hook合同。
	directBoundaryAttach func([]denseSpatialUnit) ([]denseSpatialUnit, error)
	// directBoundaryLaneLimit 职责：限制黑盒单个外部物理接口的净流量；使用：容量拆分；边界：零沿用带宽，不限制内部带或增加所选分拣器能力。
	directBoundaryLaneLimit float64
	// directBoundaryExtentScale 职责：按实际核心/附件规模给黑盒有限占位预算；使用：设施成形并裁尾后；边界：零保留调用方预算，候选仍用真实轮廓占位及最终包络评分。
	directBoundaryExtentScale float64
	// denseMaterialNetworks 职责：保存同物料共享消费轨的规范/物理映射；使用：独立密铺网络候选；边界：不能替代最终逐机、容量或真实路线证明。
	denseMaterialNetworks *denseMaterialNetworkPlan
	// denseUnitOrder 职责：为独立候选重排完整实体集合的落位顺序；使用：机器、辅助网络和边界设施全部准备后；边界：只允许重排，不改变集合实体、owner或规范生产顺序。
	denseUnitOrder func(context.Context, []denseSpatialUnit) ([]denseSpatialUnit, error)
	// denseFlyBlackbox 职责：登记密铺配送黑盒的真实边界库存及分支；使用：独立候选的最终设施证明；边界：内部运输必须由transportRoutes逐条直连，不生成虚拟配送端点。
	denseFlyBlackbox *denseFlyBlackboxState
	// denseConnectedStreets/denseStreetRoutes 职责：保存独立附件及其实际内部接线；使用：mode1全场落位后提交；边界：辅助线不重复记入规范净流边。
	denseConnectedStreets []*denseConnectedStreetWork
	denseStreetRoutes     []*transportRoute
	// generationCtx 职责：保存本次生成的取消和总截止时间；使用：局部装配、占位和补电的长循环；边界：纯几何测试未设置时不附加截止时间，不跨请求复用。
	generationCtx context.Context
	// all 职责：最终建筑顺序；使用：编码；边界：每个指针只出现一次。
	all []*placed
	// machineLines 职责：保留实际局部生产拓扑；使用：全部重排/重绑后逐台最终核验；边界：不能仅凭早期passed摘要发布产物。
	machineLines []*line
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
	// flyLayoutBlocks 职责：保留配送闭合块身份；使用：副产物本组导回及球面独立装箱；边界：平面也登记，不能跨块连接实体回收带。
	flyLayoutBlocks []surfaceBlockRange
	// flyLayoutHeight 职责：保存当前配送候选列高；使用：副产物扩展后按同一候选重新装箱；边界：不能把所有候选重置成配置的旧长条列高。
	flyLayoutHeight float64
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
	// connectedHorizontalPacking 职责：在自动球面水平行架中启用真实轮廓空槽回填；使用：独立有界候选；边界：保留原矩形及竖向轮廓退路，不在全局接线后移动机器。
	connectedHorizontalPacking bool
	// connectedGlobalPower 职责：在独立候选中延后电塔落位；使用：机器与路线固定后统一补电；边界：不改变公开供电开关，候选仍须完整覆盖与碰撞审计。
	connectedGlobalPower bool
	// connectedPowerEncoding 职责：固定补电候选预检使用的球面坐标系；使用：球面全局供电索引；边界：新增塔复用同一映射，最终 records 与完整规则仍重新审计。
	connectedPowerEncoding *surfaceEncoding
	// logistics 职责：积累本次生成的物流与空间证据；使用：finish；边界：硬错误不随结果返回部分蓝图。
	logistics *LogisticsSummary
	// stationLinks 职责：保存每条塔路线；使用：模式3/4按FlowGraph核验；边界：不跨请求复用。
	stationLinks []stationTransportLink
	// stationLeftovers 职责：保存不重复记账的塔内存货实体回收出带；使用：剩余边最终边界与闭合块校验；边界：不作为另一条生产供给端点。
	stationLeftovers []stationLeftoverReturn
	// flyEndpoints 职责：保存 mode2/5 每条产线端点；使用：按 FlowGraph 逐边绑定；边界：没有端点的规范边必须硬失败。
	flyEndpoints []*flyTransportEndpoint
	// flyCompact 职责：启用配送候选的紧凑本地端点余量；使用：有界布局候选；边界：基线保持原几何，紧凑方案仍经过同一预提交和最终规则。
	flyCompact bool
	// flySurfacePairs 职责：标识本候选使用交错配送组合块；使用：球面宽度及完整占位预筛；边界：不影响旧单行候选。
	flySurfacePairs bool
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

// newCandidateBuilder 职责：从当前生成上下文创建候选 Builder 并继承阶段适配器；使用：布局候选、试装和优化候选；边界：候选实体、物流摘要和可变状态必须由调用方独立初始化。
func (b *builder) newCandidateBuilder() (*builder, error) {
	candidate, err := newBuilder(b.setting)
	if err != nil {
		return nil, err
	}
	candidate.adapter = b.adapter
	candidate.preview = b.preview.fork()
	return candidate, nil
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
