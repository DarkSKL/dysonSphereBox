package build

// 文件职责：集中登记模式适配器及其阶段钩子。
// 使用：统一生成入口在公共准备完成后解析并调用；边界：物流塔布局来自具体组合包，旧实体装配和公共编码逐步迁移。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/adapterprofile"
	mode1Dense "DysonSphereBox/helper/blueprint/build/adapters/mode1/dense"
	mode1Standard "DysonSphereBox/helper/blueprint/build/adapters/mode1/standard"
	mode2Dense "DysonSphereBox/helper/blueprint/build/adapters/mode2/dense"
	mode2Standard "DysonSphereBox/helper/blueprint/build/adapters/mode2/standard"
	mode3Dense "DysonSphereBox/helper/blueprint/build/adapters/mode3/dense"
	mode3Standard "DysonSphereBox/helper/blueprint/build/adapters/mode3/standard"
	mode4Dense "DysonSphereBox/helper/blueprint/build/adapters/mode4/dense"
	mode4Standard "DysonSphereBox/helper/blueprint/build/adapters/mode4/standard"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// modeAdapter 职责：组合单个模式的生成和收尾阶段；使用：统一入口解析后注入 builder；边界：公共校验、蓝图编码与结果发布仍由 pipeline 负责。
type modeAdapter struct {
	// mode 职责：标识生成拓扑；使用：适配器校验和诊断；边界：不作为分散的算法分支开关。
	mode int
	// density 职责：标识布局密度组合；使用：组合注册与调试；边界：不得跨组合共享可变适配器对象。
	density string
	// generate 职责：执行组合生成阶段；使用：统一入口；边界：只返回最终候选，不负责最终事件发布。
	generate func(context.Context, *builder, Request, *quant.Dataset) (Result, error)
	// prepareFinish 职责：执行收尾前的模式专属几何准备；使用：通用 finish 入口；边界：nil 表示无需额外步骤。
	prepareFinish func(context.Context, *builder, Request) error
	// retrySurfaceAnchor 职责：处理模式专属球面锚点重试；使用：通用收尾的可恢复失败分支；边界：必须保留未处理的错误。
	retrySurfaceAnchor func(context.Context, *builder, Request, *surfaceEncoding, []blueprint.Building, error) (*surfaceEncoding, []blueprint.Building, []ProductionBinding, []int, error)
	// finalizeRecords 职责：绑定模式专属物流证据；使用：记录编号和通用规则检查后；边界：不得替代共享规则校验。
	finalizeRecords func(context.Context, *builder, []blueprint.Building) error
	// logistics 职责：选择公共物流摘要中的模式校验策略；使用：最终指标汇总；边界：只描述策略，不执行校验。
	logistics logisticsFinalization
	// supportsMachineRecycle 职责：声明模式是否具有机器内回流物理方案；使用：公共容量预检；边界：不证明该模式候选一定连接成功。
	supportsMachineRecycle bool
	// flySurfacePairs 职责：声明配送模式是否包含球面反向组合候选；使用：布局候选生成；边界：只影响候选集合。
	flySurfacePairs bool
	// connectedRows 职责：选择共享机器行装配能力；使用：直连/总线路径；边界：不包含寻路代价或终检策略。
	connectedRows connectedRowsCapabilities
	// lineAssembly 职责：选择共享机器行物理差异；使用：机器行装配；边界：实体规则仍由统一预检验证。
	lineAssembly lineAssemblyCapabilities
	// station 职责：保存由具体组合目录提供的物流塔阶段；使用：统一编排；边界：非物流塔模式为空，不在注册器选择内部布局。
	station *contract.StationAdapter
}

const (
	// densityStandard 职责：标识当前稳定布局实现；使用：默认生成请求和兼容旧工作区；边界：保持现有生成行为。
	densityStandard = "standard"
	// densityDense 职责：标识预留的高密度布局组合；使用：未来替换各阶段适配器；边界：密集机器行使用独立局部几何，完整双排与外围密度优化仍在实现。
	densityDense = "dense"
)

// modeAdapterKey 职责：唯一标识模式与密度组合；使用：适配器注册表；边界：不携带用户方案或运行时实体状态。
type modeAdapterKey = adapterprofile.Key

// publicModeProfiles 职责：从模式目录汇总公开模式/密度组合；使用：适配器注册表构建；边界：内部兼容模式单独注册，不扩展前端公开目录。
var publicModeProfiles = []adapterprofile.Definition{
	mode1Standard.Definition(), mode1Dense.Definition(),
	mode2Standard.Definition(), mode2Dense.Definition(),
	mode3Standard.Definition(), mode3Dense.Definition(),
	mode4Standard.Definition(), mode4Dense.Definition(),
}

// flyModeAdapter 职责：保存单个配送设备模式的适配配置；使用：mode 2/5 各自注册；边界：可共享配送布局原语，但模式标识不可混用。
type flyModeAdapter struct {
	mode int
}

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

// logisticsFinalization 职责：描述统一物流收尾中的模式差异；使用：由模式适配器提供给公共证据汇总器；边界：不包含路线规划或建筑装配。
type logisticsFinalization struct {
	preserveStationLanes     bool
	validateConnectedRoutes  bool
	validateSelfSprays       bool
	validateLeftovers        bool
	validateEdgeConservation bool
	validateBusTransport     bool
}

// modeAdapters 职责：集中登记现有生成模式实现；使用：统一生成入口按 mode 查询；边界：未知模式明确失败，不隐式落入其它模式。
var modeAdapters = buildModeAdapters()

// buildModeAdapters 职责：登记公开 profile 和内部兼容模式；使用：进程启动时构建只读组合表；边界：外围编排暂共享，机器行按组合密度选择独立几何。
func buildModeAdapters() map[modeAdapterKey]*modeAdapter {
	standard := map[int]*modeAdapter{
		1: {mode: 1, density: densityStandard, generate: generateConnectedMode, retrySurfaceAnchor: retryConnectedSurfaceAnchor, supportsMachineRecycle: true, connectedRows: connectedRowsCapabilities{prepareConnectedSelfSpray: true}, lineAssembly: lineAssemblyCapabilities{reverseOutputRows: true, writeInputParameters: true, useLocalLinePower: true}, logistics: logisticsFinalization{validateConnectedRoutes: true, validateSelfSprays: true, validateLeftovers: true, validateEdgeConservation: true}},
		2: {mode: 2, density: densityStandard, generate: generateFlyMode2, finalizeRecords: finalizeFlyMode, flySurfacePairs: true, lineAssembly: lineAssemblyCapabilities{useFlyTransport: true}, logistics: logisticsFinalization{validateEdgeConservation: true}},
		3: {mode: 3, density: densityStandard, generate: generateStationMode, finalizeRecords: finalizeStationMode, lineAssembly: lineAssemblyCapabilities{reverseOutputRows: true, useLocalLinePower: true}, logistics: logisticsFinalization{preserveStationLanes: true}},
		4: {mode: 4, density: densityStandard, generate: generateStationMode, finalizeRecords: finalizeStationMode, lineAssembly: lineAssemblyCapabilities{reverseOutputRows: true, useLocalLinePower: true}, logistics: logisticsFinalization{preserveStationLanes: true}},
		5: {mode: 5, density: densityStandard, generate: generateFlyMode5, finalizeRecords: finalizeFlyMode, lineAssembly: lineAssemblyCapabilities{useFlyTransport: true}, logistics: logisticsFinalization{validateEdgeConservation: true}},
		6: {mode: 6, density: densityStandard, generate: generateBusMode, prepareFinish: prepareBusFinish, connectedRows: connectedRowsCapabilities{reservePerRouteInputStubs: true, preferFreshRecycleInput: true, keepAssignedRouteEnd: true}, lineAssembly: lineAssemblyCapabilities{useLocalLinePower: true, useBusPowerOffset: true}, logistics: logisticsFinalization{validateBusTransport: true}, supportsMachineRecycle: true},
	}
	result := make(map[modeAdapterKey]*modeAdapter, len(publicModeProfiles)+4)
	for _, profile := range publicModeProfiles {
		base, ok := standard[profile.Mode]
		if !ok {
			continue
		}
		adapter := *base
		adapter.density = profile.Density
		if profile.Density == densityDense && profile.Mode >= 2 && profile.Mode <= 4 {
			// 仅密铺黑盒具备本地优先回流门；各模式对这类请求强制只走黑盒候选，不能退回旧远程净流端口。
			adapter.supportsMachineRecycle = true
		}
		if profile.Station != nil {
			stages := *profile.Station
			adapter.station = &stages
		}
		result[modeAdapterKey{Mode: profile.Mode, Density: profile.Density}] = &adapter
	}
	// mode 5/6 仅为引擎内部兼容入口保留，不属于当前四个公开模式目录。
	for _, mode := range []int{5, 6} {
		base := standard[mode]
		for _, density := range []string{densityStandard, densityDense} {
			adapter := *base
			adapter.density = density
			result[modeAdapterKey{Mode: mode, Density: density}] = &adapter
		}
	}
	return result
}

// generateConnectedMode 职责：调用直连带生成算法；使用：mode 1 适配器；边界：不接管共享收尾。
func generateConnectedMode(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset) (Result, error) {
	return builder.generateConnected(ctx, request, dataset)
}

// generateFlyMode2 职责：运行物流配送器模式阶段；使用：mode 2 适配器；边界：请求模式不匹配时拒绝，不回退为另一个配送拓扑。
func generateFlyMode2(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset) (Result, error) {
	return (flyModeAdapter{mode: 2}).generate(ctx, builder, request, dataset)
}

// generateFlyMode5 职责：运行四向分流配送模式阶段；使用：mode 5 适配器；边界：请求模式不匹配时拒绝，不回退为普通配送拓扑。
func generateFlyMode5(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset) (Result, error) {
	return (flyModeAdapter{mode: 5}).generate(ctx, builder, request, dataset)
}

// generate 职责：运行指定配送模式的共享阶段算法；使用：mode 2/5 专属配置适配器；边界：设备差异由明确的 mode 传递给各领域步骤。
func (adapter flyModeAdapter) generate(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset) (Result, error) {
	if request.Mode != adapter.mode {
		return Result{}, &blueprint.Error{Code: "GEN_E_MODE", Message: "配送适配器与请求模式不匹配"}
	}
	return builder.generateFlyLayout(ctx, request, dataset)
}

// generateBusMode 职责：调用总线生成算法；使用：mode 6 适配器；边界：不处理其它模式的端口拓扑。
func generateBusMode(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset) (Result, error) {
	return builder.generateBus(ctx, request, dataset)
}

// resolveModeAdapter 职责：解析唯一的模式适配器；使用：统一入口和注册表契约测试；边界：不回退到默认模式。
func resolveModeAdapter(mode int, density string) (*modeAdapter, error) {
	if density != densityStandard && density != densityDense {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY", Message: "没有可用的布局密度适配器"}
	}
	adapter, ok := modeAdapters[modeAdapterKey{Mode: mode, Density: density}]
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_MODE", Message: "没有可用的蓝图生成适配器"}
	}
	return adapter, nil
}

// prepareBusFinish 职责：为球面总线模式折叠环路布局；使用：统一收尾开始时调用；边界：非球面请求保持原平面收尾路径。
func prepareBusFinish(ctx context.Context, builder *builder, request Request) error {
	if request.SurfaceLayout == nil {
		return nil
	}
	reportProgress(ctx, "surface", "正在折叠球面总线布局", 0, 0)
	return builder.foldSurfaceBus(ctx, request.SurfaceLayout)
}

// retryConnectedSurfaceAnchor 职责：为直连模式重建可扩展球面接线候选；使用：初次锚点规则可恢复失败时调用；边界：只接管原直连特殊重试条件。
func retryConnectedSurfaceAnchor(ctx context.Context, builder *builder, request Request, surface *surfaceEncoding, records []blueprint.Building, cause error) (*surfaceEncoding, []blueprint.Building, []ProductionBinding, []int, error) {
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

// finalizeModeRecords 职责：执行当前模式的记录收尾；使用：builder.finish 的共同终检阶段；边界：无专属逻辑时为空操作。
func finalizeModeRecords(ctx context.Context, builder *builder, records []blueprint.Building) error {
	if builder.adapter == nil || builder.adapter.finalizeRecords == nil {
		return nil
	}
	return builder.adapter.finalizeRecords(ctx, builder, records)
}

// finalizeModeLogistics 职责：以当前适配器的规则生成物流摘要；使用：统一收尾完成实体绑定后调用；边界：适配器只提供策略，不复制公共指标计算。
func finalizeModeLogistics(builder *builder, records []blueprint.Building, areas []blueprint.Area) (*LogisticsSummary, error) {
	if builder.adapter == nil {
		return nil, &blueprint.Error{Code: "GEN_E_MODE", Message: "生成收尾缺少模式适配器"}
	}
	return builder.finalizeLogistics(builder.adapter.logistics, records, areas)
}

// finalizeStationMode 职责：核验物流塔模式的实体端口与规范流边绑定；使用：mode 3/4 收尾；边界：失败时不继续生成物流摘要。
func finalizeStationMode(ctx context.Context, builder *builder, records []blueprint.Building) error {
	return builder.validateStationTransport(ctx, records)
}

// finalizeFlyMode 职责：登记配送模式的设施运输关联；使用：mode 2/5 收尾；边界：不重新规划或修改物流路线。
func finalizeFlyMode(_ context.Context, builder *builder, _ []blueprint.Building) error {
	if builder.logistics == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送模式缺少规范流图"}
	}
	return builder.validateFlyTransport(builder.adapter.mode)
}
