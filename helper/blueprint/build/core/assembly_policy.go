package build

// 文件职责：从可信模式和密度建立无回调的装配能力及记录验证策略。
// 使用：输入准备、单候选装配与最终记录校验；边界：不登记生成函数、不派发候选、不保存全局组合注册表。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
	"context"
)

// assemblyPolicy 职责：保存单次装配所需的纯值能力；使用：输入快照和局部装配器按值复制；边界：不含生成、收尾或重试函数指针。
type assemblyPolicy struct {
	// mode/density 职责：标识可信拓扑与物理密度；使用：共享设备算法；边界：不决定工作流或候选调度。
	mode    int
	density string
	// records 职责：选择具名记录证明；使用：冻结前记录绑定；边界：不是可执行回调。
	records recordStrategy
	// foldSurfaceBus/connectedSurfaceAnchor 职责：声明单次几何适配能力；使用：定稿阶段明确函数调用；边界：不另起生产候选。
	foldSurfaceBus         bool
	connectedSurfaceAnchor bool
	// logistics 职责：保存真实物流证明要求；使用：共享终检；边界：不能代替实体守恒检查。
	logistics logisticsFinalization
	// supportsMachineRecycle 职责：声明本地毛流回收能力；使用：准备阶段资格检查；边界：不证明具体候选可行。
	supportsMachineRecycle bool
	// flySurfacePairs 职责：声明交错配送组合能力；使用：显式候选资格；边界：不自动添加候选。
	flySurfacePairs bool
	// connectedRows/lineAssembly 职责：保存局部机器行装配能力；使用：共享几何；边界：不改变真实设备事实。
	connectedRows connectedRowsCapabilities
	lineAssembly  lineAssemblyCapabilities
	// station 职责：保存真实塔设备配置；使用：塔候选读取槽位及实体参数；边界：不持有规划器或注册回调。
	station *contract.StationPolicy
}

// 密度常量职责：统一可信输入与硬几何口径；使用：能力与规则；边界：不注册模式组合。
const (
	// densityStandard 职责：标识普通实体规则；使用：规范输入；边界：不放宽碰撞。
	densityStandard = "standard"
	// densityDense 职责：标识整图密铺实体规则；使用：规范输入；边界：仍需完整真实校验。
	densityDense = "dense"
)

// recordStrategy 职责：声明装配完成后的实体绑定证明种类；使用：明确函数分派；边界：不包含函数指针或生成入口。
type recordStrategy uint8

// 记录策略职责：保留各真实拓扑的不同证明；使用：定稿；边界：不能用编号存在代替真实连通。
const (
	// recordConnected 职责：选择直接物流证明；使用：直连与总线；边界：额外守恒由物流策略核验。
	recordConnected recordStrategy = iota
	// recordFly 职责：选择逐行配送证明；使用：配送实体；边界：不复用黑盒证明。
	recordFly
	// recordStation 职责：选择共享塔场端口证明；使用：塔场实体；边界：不跳过逐边绑定。
	recordStation
	// recordFlyBlackbox 职责：选择对外箱体的黑盒证明；使用：配送黑盒；边界：保留真实箱臂及内部路线。
	recordFlyBlackbox
	// recordStationBlackbox 职责：选择对外塔的黑盒证明；使用：塔黑盒；边界：保留共享塔端口及真实净流。
	recordStationBlackbox
)

// connectedRowsCapabilities 职责：声明通用机器行装配需要的模式特有行为；使用：直连带和共享总线装配；边界：不包含几何计算和路线搜索策略。
type connectedRowsCapabilities struct {
	reservePerRouteInputStubs bool
	preferFreshRecycleInput   bool
	prepareConnectedSelfSpray bool
	keepAssignedRouteEnd      bool
}

// lineAssemblyCapabilities 职责：声明机器行装配的模式差异；使用：buildLine 的共享装配器；边界：只描述实体准备策略，不替代路线和最终规则。
type lineAssemblyCapabilities struct {
	reverseOutputRows    bool
	writeInputParameters bool
	useFlyTransport      bool
	useLocalLinePower    bool
	useBusPowerOffset    bool
}

// logisticsFinalization 职责：描述统一物流收尾中的模式差异；使用：纯装配能力提供给公共证据汇总器；边界：不包含路线规划或建筑装配。
type logisticsFinalization struct {
	preserveStationLanes     bool
	validateConnectedRoutes  bool
	validateSelfSprays       bool
	validateLeftovers        bool
	validateEdgeConservation bool
	validateBusTransport     bool
}

// newAssemblyPolicy 职责：把可信设备模式映射为纯能力值；使用：输入准备一次调用；边界：不保存全局注册表，未知模式或密度明确失败。
func newAssemblyPolicy(mode int, density string) (*assemblyPolicy, error) {
	if density != densityStandard && density != densityDense {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY", Message: "没有可用的布局密度策略"}
	}
	policy := &assemblyPolicy{mode: mode, density: density}
	switch mode {
	case 1:
		policy.supportsMachineRecycle, policy.connectedSurfaceAnchor = true, true
		policy.connectedRows = connectedRowsCapabilities{prepareConnectedSelfSpray: true}
		policy.lineAssembly = lineAssemblyCapabilities{reverseOutputRows: true, writeInputParameters: true, useLocalLinePower: true}
		policy.logistics = logisticsFinalization{validateConnectedRoutes: true, validateSelfSprays: true, validateLeftovers: true, validateEdgeConservation: true}
	case 2:
		policy.records, policy.supportsMachineRecycle, policy.flySurfacePairs = recordFly, true, true
		policy.lineAssembly = lineAssemblyCapabilities{useFlyTransport: true}
		policy.logistics = logisticsFinalization{validateEdgeConservation: true}
	case 3, 4:
		policy.records, policy.supportsMachineRecycle = recordStation, true
		policy.lineAssembly = lineAssemblyCapabilities{reverseOutputRows: true, useLocalLinePower: true}
		policy.logistics = logisticsFinalization{preserveStationLanes: true}
		rows, strategy := 0, "grouped"
		if density == densityDense {
			rows, strategy = 1, "field"
		}
		config, err := stationlayout.Configuration(2100+mode, rows)
		if err != nil {
			return nil, err
		}
		policy.station = &contract.StationPolicy{Config: config, Strategy: strategy}
	case 5:
		policy.records = recordFly
		policy.lineAssembly = lineAssemblyCapabilities{useFlyTransport: true}
		policy.logistics = logisticsFinalization{validateEdgeConservation: true}
	case 6:
		policy.supportsMachineRecycle, policy.foldSurfaceBus = true, true
		policy.connectedRows = connectedRowsCapabilities{reservePerRouteInputStubs: true, preferFreshRecycleInput: true, keepAssignedRouteEnd: true}
		policy.lineAssembly = lineAssemblyCapabilities{useLocalLinePower: true, useBusPowerOffset: true}
		policy.logistics = logisticsFinalization{validateBusTransport: true}
	default:
		return nil, &blueprint.Error{Code: "GEN_E_MODE", Message: "没有可用的蓝图装配策略"}
	}
	return policy, nil
}

// prepareBusGeometry 职责：为球面总线模式折叠环路布局；使用：统一收尾开始时调用；边界：非球面请求保持原平面收尾路径。
func prepareBusGeometry(ctx context.Context, builder *builder, request Request) error {
	if request.SurfaceLayout == nil {
		return nil
	}
	reportProgress(ctx, "surface", "正在折叠球面总线布局", 0, 0)
	return builder.foldSurfaceBus(ctx, request.SurfaceLayout)
}

// prepareConnectedSurfaceAnchor 职责：为直连模式重建可扩展球面接线候选；使用：初次锚点规则可恢复失败时调用；边界：只接管原直连特殊重试条件。
func prepareConnectedSurfaceAnchor(ctx context.Context, builder *builder, request Request, surface *surfaceEncoding, records []blueprint.Building, cause error) (*surfaceEncoding, []blueprint.Building, []ProductionBinding, []int, error) {
	if request.SurfaceLayout == nil || !surfaceAnchorRetryable(cause) || surface.packed || len(builder.transportRoutes) == 0 || (request.SurfaceLayout.Mode != "autoTropic" && !request.SurfaceLayout.ExpandAdjacent) {
		return surface, records, nil, nil, cause
	}
	reportProgress(ctx, "surface-retry", "正在保留物料连接并扩展相邻纬带", 0, 0)
	next, err := builder.prepareConnectedSurfaceEncoding(ctx, request.SurfaceLayout)
	if err != nil {
		return surface, records, nil, nil, err
	}
	nextRecords, bindings, shared, err := builder.records()
	if err == nil {
		err = next.applyRecords(builder.all, nextRecords)
	}
	if err == nil {
		err = next.validateAnchorCandidates(ctx, nextRecords)
	}
	return next, nextRecords, bindings, shared, err
}
