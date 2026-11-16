package build

// 文件职责：用实际布局参数描述站点候选族，替代编号和修饰位组合。
// 使用：普通与密铺枚举复制计划并添加拓扑能力；边界：只读值，不持有实体或调度身份。

// stationCandidatePlan 职责：保存尚未结合请求事实的布局意图；使用：归一化后一次解析；边界：回流跨度和纬向基准不会进入装配层。
type stationCandidatePlan struct {
	spec StationCandidateSpec
	// recycleSpan/refineWithRecycle 职责：指定本地回流时的行策略；使用：几何解析；边界：不修改配方与回流量。
	recycleSpan       float64
	refineWithRecycle bool
	// latitudeBase 职责：保存物料布局的纬向基准；使用：按机器规模缩放；边界：零表示普通行布局。
	latitudeBase float64
	// explicitRowSpan 职责：覆盖缓存或回流的规划默认行长；使用：显式普通行策略；边界：不覆盖容量或用户设备事实。
	explicitRowSpan float64
}

// standardStationRowPlans 职责：用有限短行跨度复用基础塔规划；使用：生产YAML显式启用；边界：不复制装配算法，不改变默认候选集合。
func standardStationRowPlans(spans []float64) []stationCandidatePlan {
	plans := make([]stationCandidatePlan, 0, len(spans)*4)
	for _, span := range spans {
		for _, plan := range stationRowPlans() {
			plan.explicitRowSpan = span
			plans = append(plans, plan)
		}
	}
	return plans
}

// stationRowPlans 职责：保留四种基础行布局及原始顺序；使用：普通与密铺塔枚举；边界：旋转、排序与高度都是实际参数，不编码修饰。
func stationRowPlans() []stationCandidatePlan {
	return []stationCandidatePlan{
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 0, MaxSpan: 18}, recycleSpan: 8},
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 1, MaxSpan: 24, Street: true, LinearTowers: true}, recycleSpan: 8, refineWithRecycle: true},
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 2, MaxSpan: 32, Rotation: 1, HeightChoice: 1}, recycleSpan: 8},
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 3, MaxSpan: 24, Street: true, Rotation: 1, LinearTowers: true, HeightChoice: 1}, recycleSpan: 24, refineWithRecycle: true},
	}
}

// stationMaterialPlans 职责：保留紧凑枢纽、消费者优先和整街道三种物料布局；使用：大厂与拓扑扩展；边界：塔列数、分组和高度选择继续独立。
func stationMaterialPlans() []stationCandidatePlan {
	return []stationCandidatePlan{
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 3, MaxSpan: 12, Street: true, MaterialOrder: true, MaterialNetworks: true, HeightChoice: 1}, recycleSpan: 8, latitudeBase: 24},
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 3, MaxSpan: 18, Street: true, Groups: 2, MaterialOrder: true, MaterialNetworks: true, ConsumerFirst: true, LinearTowers: true, HeightChoice: 1}, recycleSpan: 8, refineWithRecycle: true, latitudeBase: 32},
		{spec: StationCandidateSpec{Kind: "blackbox", Order: 3, MaxSpan: 32, Street: true, Groups: -1, MaterialOrder: true, MaterialNetworks: true, HeightChoice: 1}, recycleSpan: 8, latitudeBase: 40},
	}
}

// StationFieldPacking 职责：保存共享塔场实际消费的放置选择；使用：field候选直接交接；边界：不包含普通行阵或不透明变体编号。
type StationFieldPacking struct {
	// Staggered 职责：选择交错塔阵列及长轴朝向；使用：塔距和行距计算；边界：真实间距仍终检。
	Staggered bool `json:"staggered"`
	// NarrowColumns 职责：选择较少塔列与较宽搜索范围；使用：塔阵列和单元排序；边界：不改变容量。
	NarrowColumns bool `json:"narrowColumns"`
}
